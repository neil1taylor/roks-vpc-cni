package gateway

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1alpha1 "github.com/IBM/roks-vpc-network-operator/api/v1alpha1"
	"github.com/IBM/roks-vpc-network-operator/pkg/annotations"
	"github.com/IBM/roks-vpc-network-operator/pkg/controller/network"
	"github.com/IBM/roks-vpc-network-operator/pkg/finalizers"
	operatormetrics "github.com/IBM/roks-vpc-network-operator/pkg/metrics"
	"github.com/IBM/roks-vpc-network-operator/pkg/vpc"
)

// Reconciler reconciles VPCGateway objects.
type Reconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	Recorder  record.EventRecorder
	VPC       vpc.Client
	ClusterID string
	VPCID     string
}

// Reconcile handles VPCGateway create/update/delete events.
//
// Creation/Update flow:
//  1. Add finalizer
//  2. Create uplink VNI via vpc.VNIService.CreateVNI()
//  3. Create VPC routes pointing to VNI reserved IP
//  4. Update status with VNIID, reservedIP, routeIDs, and phase=Ready
//
// Deletion flow:
//  1. Delete VPC routes
//  2. Delete uplink VNI
//  3. Remove finalizer
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling VPCGateway", "name", req.Name, "namespace", req.Namespace)

	// Fetch the VPCGateway object
	gw := &v1alpha1.VPCGateway{}
	if err := r.Get(ctx, req.NamespacedName, gw); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to fetch VPCGateway")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !gw.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, gw)
	}

	// Handle creation/update
	return r.reconcileNormal(ctx, gw)
}

// reconcileNormal handles VPCGateway creation and updates.
func (r *Reconciler) reconcileNormal(ctx context.Context, gw *v1alpha1.VPCGateway) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Step 1: Add finalizer
	if err := finalizers.EnsureAdded(ctx, r.Client, gw, finalizers.GatewayCleanup); err != nil {
		logger.Error(err, "Failed to add finalizer")
		return ctrl.Result{}, err
	}

	// Step 2: Ensure uplink VNI exists (with drift detection)
	if gw.Status.VNIID != "" {
		// Drift detection: verify VNI still exists in VPC
		existingVNI, err := r.VPC.GetVNI(ctx, gw.Status.VNIID)
		if err != nil && isVPCNotFound(err) {
			logger.Info("VNI no longer exists in VPC, will recreate", "vniID", gw.Status.VNIID)
			r.emitEvent(gw, "Warning", "VNIDrift", "Uplink VNI %s no longer exists, recreating", gw.Status.VNIID)
			gw.Status.VNIID = ""
			gw.Status.MACAddress = ""
			gw.Status.ReservedIP = ""
			gw.Status.VPCRouteIDs = nil
		} else if err != nil {
			logger.Error(err, "Failed to verify VNI exists", "vniID", gw.Status.VNIID)
		} else if gw.Status.MACAddress == "" {
			// Backfill MACAddress for gateways created before this field existed
			gw.Status.MACAddress = existingVNI.MACAddress
			logger.Info("Backfilled MACAddress from existing VNI", "mac", existingVNI.MACAddress)
		}
	}

	if gw.Status.VNIID == "" {
		vni, err := r.ensureVNI(ctx, gw)
		if err != nil {
			logger.Error(err, "Failed to create uplink VNI")
			operatormetrics.ReconcileOpsTotal.WithLabelValues("vpcgateway", "create_vni", "error").Inc()
			r.emitEvent(gw, "Warning", "CreateVNIFailed", "Failed to create uplink VNI: %v", err)
			r.setFailedStatus(ctx, gw, "VNICreateFailed", fmt.Sprintf("Failed to create uplink VNI: %v", err))
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
		gw.Status.VNIID = vni.ID
		gw.Status.MACAddress = vni.MACAddress
		gw.Status.ReservedIP = vni.PrimaryIP.Address
		operatormetrics.ReconcileOpsTotal.WithLabelValues("vpcgateway", "create_vni", "success").Inc()
		r.emitEvent(gw, "Normal", "VNICreated", "Created uplink VNI %s (%s)", vni.ID, vni.PrimaryIP.Address)
		meta.SetStatusCondition(&gw.Status.Conditions, metav1.Condition{
			Type:    "VNIReady",
			Status:  metav1.ConditionTrue,
			Reason:  "Created",
			Message: fmt.Sprintf("Uplink VNI %s is ready", vni.ID),
		})
	}

	// Step 3: Collect advertised routes from routers and merge with explicit routes
	desiredRoutes := r.collectDesiredRoutes(ctx, gw)

	// Step 3b: Ensure VPC routes (creates missing, deletes stale)
	routeIDs, err := r.ensureVPCRoutes(ctx, gw, desiredRoutes)
	if err != nil {
		logger.Error(err, "Failed to ensure VPC routes")
		operatormetrics.ReconcileOpsTotal.WithLabelValues("vpcgateway", "create_routes", "error").Inc()
		r.emitEvent(gw, "Warning", "RoutesConfigFailed", "Failed to configure VPC routes: %v", err)
		r.setFailedStatus(ctx, gw, "RoutesConfigFailed", fmt.Sprintf("Failed to configure VPC routes: %v", err))
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}
	gw.Status.VPCRouteIDs = routeIDs
	meta.SetStatusCondition(&gw.Status.Conditions, metav1.Condition{
		Type:    "RoutesConfigured",
		Status:  metav1.ConditionTrue,
		Reason:  "Configured",
		Message: fmt.Sprintf("Configured %d VPC route(s)", len(routeIDs)),
	})

	// Step 4: Ensure Floating IP (if enabled)
	if gw.Spec.FloatingIP != nil && gw.Spec.FloatingIP.Enabled {
		if err := r.ensureFloatingIP(ctx, gw); err != nil {
			logger.Error(err, "Failed to ensure floating IP")
			operatormetrics.ReconcileOpsTotal.WithLabelValues("vpcgateway", "create_fip", "error").Inc()
			r.emitEvent(gw, "Warning", "FIPConfigFailed", "Failed to configure floating IP: %v", err)
			r.setFailedStatus(ctx, gw, "FIPConfigFailed", fmt.Sprintf("Failed to configure floating IP: %v", err))
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
		operatormetrics.ReconcileOpsTotal.WithLabelValues("vpcgateway", "create_fip", "success").Inc()
		meta.SetStatusCondition(&gw.Status.Conditions, metav1.Condition{
			Type:    "FloatingIPReady",
			Status:  metav1.ConditionTrue,
			Reason:  "Configured",
			Message: fmt.Sprintf("Floating IP %s (%s) bound to VNI", gw.Status.FloatingIPID, gw.Status.FloatingIP),
		})
	}

	// Step 5: Ensure Public Address Range (if enabled)
	if gw.Spec.PublicAddressRange != nil && gw.Spec.PublicAddressRange.Enabled {
		if err := r.ensurePAR(ctx, gw); err != nil {
			logger.Error(err, "Failed to ensure PAR")
			operatormetrics.ReconcileOpsTotal.WithLabelValues("vpcgateway", "create_par", "error").Inc()
			r.emitEvent(gw, "Warning", "PARConfigFailed", "Failed to configure PAR: %v", err)
			r.setFailedStatus(ctx, gw, "PARConfigFailed", fmt.Sprintf("Failed to configure PAR: %v", err))
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
		operatormetrics.ReconcileOpsTotal.WithLabelValues("vpcgateway", "create_par", "success").Inc()
		meta.SetStatusCondition(&gw.Status.Conditions, metav1.Condition{
			Type:    "PARReady",
			Status:  metav1.ConditionTrue,
			Reason:  "Configured",
			Message: fmt.Sprintf("PAR %s (%s) with ingress routing", gw.Status.PublicAddressRangeID, gw.Status.PublicAddressRangeCIDR),
		})
	}

	// Update status to Ready
	gw.Status.Phase = "Ready"
	gw.Status.SyncStatus = "Synced"
	now := metav1.Now()
	gw.Status.LastSyncTime = &now
	gw.Status.Message = fmt.Sprintf("Gateway ready with VNI %s (%s)", gw.Status.VNIID, gw.Status.ReservedIP)
	meta.SetStatusCondition(&gw.Status.Conditions, metav1.Condition{
		Type:    "Ready",
		Status:  metav1.ConditionTrue,
		Reason:  "Synced",
		Message: fmt.Sprintf("VPCGateway %s is ready", gw.Name),
	})

	if err := r.Status().Update(ctx, gw); err != nil {
		logger.Error(err, "Failed to update status after reconcile")
		return ctrl.Result{}, err
	}

	logger.Info("Successfully synced VPCGateway", "vniID", gw.Status.VNIID, "reservedIP", gw.Status.ReservedIP)
	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

// reconcileDelete handles VPCGateway deletion.
func (r *Reconciler) reconcileDelete(ctx context.Context, gw *v1alpha1.VPCGateway) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Step 1: Delete PAR ingress routes, ingress routing table, and PAR
	if err := r.deletePAR(ctx, gw); err != nil {
		logger.Error(err, "Failed to delete PAR resources")
		operatormetrics.ReconcileOpsTotal.WithLabelValues("vpcgateway", "delete_par", "error").Inc()
		r.emitEvent(gw, "Warning", "DeletePARFailed", "Failed to delete PAR resources: %v", err)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Step 2: Delete Floating IP
	if gw.Status.FloatingIPID != "" {
		isExternalFIP := gw.Spec.FloatingIP != nil && gw.Spec.FloatingIP.ID != ""
		if !isExternalFIP {
			if err := r.VPC.DeleteFloatingIP(ctx, gw.Status.FloatingIPID); err != nil {
				if isVPCNotFound(err) {
					logger.Info("Floating IP already deleted", "fipID", gw.Status.FloatingIPID)
				} else {
					logger.Error(err, "Failed to delete floating IP", "fipID", gw.Status.FloatingIPID)
					operatormetrics.ReconcileOpsTotal.WithLabelValues("vpcgateway", "delete_fip", "error").Inc()
					r.emitEvent(gw, "Warning", "DeleteFIPFailed", "Failed to delete floating IP %s: %v", gw.Status.FloatingIPID, err)
					return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
				}
			} else {
				logger.Info("Deleted floating IP", "fipID", gw.Status.FloatingIPID, "address", gw.Status.FloatingIP)
			}
			operatormetrics.ReconcileOpsTotal.WithLabelValues("vpcgateway", "delete_fip", "success").Inc()
			r.emitEvent(gw, "Normal", "FIPDeleted", "Deleted floating IP %s (%s)", gw.Status.FloatingIPID, gw.Status.FloatingIP)
		} else {
			// Unbind externally-managed FIP from the VNI but don't delete it
			if _, err := r.VPC.UpdateFloatingIP(ctx, gw.Status.FloatingIPID, vpc.UpdateFloatingIPOptions{}); err != nil {
				if !isVPCNotFound(err) {
					logger.Error(err, "Failed to unbind floating IP", "fipID", gw.Status.FloatingIPID)
				}
			}
			logger.Info("Unbound external floating IP", "fipID", gw.Status.FloatingIPID)
			r.emitEvent(gw, "Normal", "FIPUnbound", "Unbound external floating IP %s", gw.Status.FloatingIPID)
		}
	}

	// Step 3: Delete VPC routes
	if len(gw.Status.VPCRouteIDs) > 0 {
		rtID, err := r.getDefaultRoutingTableID(ctx)
		if err != nil {
			logger.Error(err, "Failed to find default routing table for route deletion")
			operatormetrics.ReconcileOpsTotal.WithLabelValues("vpcgateway", "delete_routes", "error").Inc()
			r.emitEvent(gw, "Warning", "DeleteRoutesFailed", "Failed to find routing table: %v", err)
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
		for _, routeID := range gw.Status.VPCRouteIDs {
			if err := r.VPC.DeleteRoute(ctx, r.VPCID, rtID, routeID); err != nil {
				if isVPCNotFound(err) {
					logger.Info("VPC route already deleted", "routeID", routeID)
					continue
				}
				logger.Error(err, "Failed to delete VPC route", "routeID", routeID)
				operatormetrics.ReconcileOpsTotal.WithLabelValues("vpcgateway", "delete_routes", "error").Inc()
				r.emitEvent(gw, "Warning", "DeleteRouteFailed", "Failed to delete route %s: %v", routeID, err)
				return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
			}
		}
		logger.Info("Deleted VPC routes", "count", len(gw.Status.VPCRouteIDs))
		operatormetrics.ReconcileOpsTotal.WithLabelValues("vpcgateway", "delete_routes", "success").Inc()
		r.emitEvent(gw, "Normal", "RoutesDeleted", "Deleted %d VPC route(s)", len(gw.Status.VPCRouteIDs))
	}

	// Step 4: Delete VLAN attachment (auto-deletes the inline VNI)
	if gw.Status.AttachmentID != "" && gw.Status.BMServerID != "" {
		if err := r.VPC.DeleteVLANAttachment(ctx, gw.Status.BMServerID, gw.Status.AttachmentID); err != nil {
			if isVPCNotFound(err) {
				logger.Info("VLAN attachment already deleted", "attachmentID", gw.Status.AttachmentID)
			} else {
				logger.Error(err, "Failed to delete gateway VLAN attachment", "attachmentID", gw.Status.AttachmentID)
				operatormetrics.ReconcileOpsTotal.WithLabelValues("vpcgateway", "delete_vni", "error").Inc()
				r.emitEvent(gw, "Warning", "DeleteAttachmentFailed", "Failed to delete attachment %s: %v", gw.Status.AttachmentID, err)
				return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
			}
		} else {
			logger.Info("Deleted gateway VLAN attachment", "attachmentID", gw.Status.AttachmentID, "vniID", gw.Status.VNIID)
		}
		operatormetrics.ReconcileOpsTotal.WithLabelValues("vpcgateway", "delete_vni", "success").Inc()
		r.emitEvent(gw, "Normal", "AttachmentDeleted", "Deleted gateway VLAN attachment %s (VNI %s)", gw.Status.AttachmentID, gw.Status.VNIID)
	} else if gw.Status.VNIID != "" {
		// Fallback: old-style floating VNI without attachment
		if err := r.VPC.DeleteVNI(ctx, gw.Status.VNIID); err != nil {
			if !isVPCNotFound(err) {
				logger.Error(err, "Failed to delete uplink VNI", "vniID", gw.Status.VNIID)
				return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
			}
		}
		logger.Info("Deleted legacy uplink VNI", "vniID", gw.Status.VNIID)
	}

	// Step 5: Remove finalizer
	if err := finalizers.EnsureRemoved(ctx, r.Client, gw, finalizers.GatewayCleanup); err != nil {
		logger.Error(err, "Failed to remove finalizer")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// ensureVNI creates the uplink VNI for the gateway via a VLAN attachment with
// inline VNI on a bare metal server — the same approach the webhook uses for VMs.
// This gives the VNI a physical path so VPC can deliver traffic (DHCP, ARP, etc.).
func (r *Reconciler) ensureVNI(ctx context.Context, gw *v1alpha1.VPCGateway) (*vpc.VNI, error) {
	logger := log.FromContext(ctx)

	// Look up the uplink CUDN to get the VPC subnet ID and VLAN ID
	subnetID, vlanID, err := r.getUplinkNetworkInfo(ctx, gw.Spec.Uplink.Network)
	if err != nil {
		return nil, fmt.Errorf("getUplinkNetworkInfo(%s): %w", gw.Spec.Uplink.Network, err)
	}

	// Pick a BM server (same as webhook — allow_to_float handles migration)
	bmServerID, err := network.PickBMServer(ctx, r.Client, r.VPC, r.VPCID)
	if err != nil {
		return nil, fmt.Errorf("pickBMServer: %w", err)
	}

	vniName := network.TruncateVPCName(fmt.Sprintf("roks-%s-gw-%s", r.ClusterID, gw.Name))
	attName := network.TruncateVPCName(fmt.Sprintf("roks-%s-gw-%s-vlan%d", r.ClusterID, gw.Name, vlanID))

	result, err := r.VPC.CreateVMAttachment(ctx, vpc.CreateVMAttachmentOptions{
		BMServerID:       bmServerID,
		Name:             attName,
		VLANID:           vlanID,
		SubnetID:         subnetID,
		VNIName:          vniName,
		SecurityGroupIDs: gw.Spec.Uplink.SecurityGroupIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("CreateVMAttachment(%s): %w", attName, err)
	}

	logger.Info("Created gateway VLAN attachment with inline VNI",
		"attachmentID", result.AttachmentID,
		"bmServerID", result.BMServerID,
		"vniID", result.VNI.ID,
		"mac", result.VNI.MACAddress,
		"ip", result.VNI.PrimaryIP.Address)

	// Store attachment info for cleanup
	gw.Status.AttachmentID = result.AttachmentID
	gw.Status.BMServerID = result.BMServerID

	return &result.VNI, nil
}

// getUplinkNetworkInfo looks up the CUDN by name and reads its subnet-id and vlan-id annotations.
func (r *Reconciler) getUplinkNetworkInfo(ctx context.Context, networkName string) (string, int64, error) {
	cudn := &unstructured.Unstructured{}
	cudn.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "k8s.ovn.org",
		Version: "v1",
		Kind:    "ClusterUserDefinedNetwork",
	})

	if err := r.Get(ctx, client.ObjectKey{Name: networkName}, cudn); err != nil {
		return "", 0, fmt.Errorf("get CUDN %s: %w", networkName, err)
	}

	annots := cudn.GetAnnotations()
	subnetID := annots[annotations.SubnetID]
	if subnetID == "" {
		return "", 0, fmt.Errorf("CUDN %s has no %s annotation", networkName, annotations.SubnetID)
	}

	vlanIDStr := annots[annotations.VLANID]
	if vlanIDStr == "" {
		return "", 0, fmt.Errorf("CUDN %s has no %s annotation", networkName, annotations.VLANID)
	}
	vlanID, err := strconv.ParseInt(vlanIDStr, 10, 64)
	if err != nil {
		return "", 0, fmt.Errorf("CUDN %s has invalid VLAN ID %q: %w", networkName, vlanIDStr, err)
	}

	return subnetID, vlanID, nil
}

// collectDesiredRoutes merges explicit spec.vpcRoutes with advertised routes
// from VPCRouters that reference this gateway.
func (r *Reconciler) collectDesiredRoutes(ctx context.Context, gw *v1alpha1.VPCGateway) []string {
	logger := log.FromContext(ctx)
	seen := map[string]bool{}

	// Explicit routes from spec
	for _, vr := range gw.Spec.VPCRoutes {
		seen[vr.Destination] = true
	}

	// Advertised routes from routers
	var routers v1alpha1.VPCRouterList
	if err := r.Client.List(ctx, &routers, client.InNamespace(gw.Namespace)); err != nil {
		logger.Error(err, "Failed to list VPCRouters for route collection")
		// Continue with explicit routes only
	} else {
		for _, rt := range routers.Items {
			if rt.Spec.Gateway == gw.Name {
				for _, route := range rt.Status.AdvertisedRoutes {
					seen[route] = true
				}
			}
		}
	}

	// Advertised routes from VPN gateways
	var vpnGateways v1alpha1.VPCVPNGatewayList
	if err := r.Client.List(ctx, &vpnGateways, client.InNamespace(gw.Namespace)); err != nil {
		logger.Error(err, "Failed to list VPCVPNGateways for route collection")
	} else {
		for _, vpn := range vpnGateways.Items {
			if vpn.Spec.GatewayRef == gw.Name {
				for _, route := range vpn.Status.AdvertisedRoutes {
					seen[route] = true
				}
			}
		}
	}

	result := make([]string, 0, len(seen))
	for dest := range seen {
		// Normalize: bare IPs (e.g. "10.100.0.1") → CIDR (e.g. "10.100.0.0/24")
		if !strings.Contains(dest, "/") {
			dest = dest + "/24"
		}
		if _, ipNet, err := net.ParseCIDR(dest); err == nil {
			dest = ipNet.String() // zero host bits
		}
		result = append(result, dest)
	}
	return result
}

// ensureVPCRoutes creates VPC routes for the desired destinations using
// idempotent checks. Also deletes stale routes that are no longer desired.
func (r *Reconciler) ensureVPCRoutes(ctx context.Context, gw *v1alpha1.VPCGateway, desiredDests []string) ([]string, error) {
	logger := log.FromContext(ctx)

	if len(desiredDests) == 0 {
		// If there were previously tracked routes, clean them up
		if len(gw.Status.VPCRouteIDs) > 0 {
			if err := r.deleteStaleRoutes(ctx, gw, nil); err != nil {
				logger.Error(err, "Failed to delete stale routes")
			}
		}
		return nil, nil
	}

	rtID, err := r.getDefaultRoutingTableID(ctx)
	if err != nil {
		return nil, fmt.Errorf("getDefaultRoutingTableID: %w", err)
	}

	// List existing routes for idempotency
	existingRoutes, err := r.VPC.ListRoutes(ctx, r.VPCID, rtID)
	if err != nil {
		return nil, fmt.Errorf("ListRoutes: %w", err)
	}

	// Build maps for route lookup
	existingByDest := make(map[string]string) // destination -> routeID
	for _, route := range existingRoutes {
		existingByDest[route.Destination] = route.ID
	}

	desiredSet := make(map[string]bool, len(desiredDests))
	for _, d := range desiredDests {
		desiredSet[d] = true
	}

	// Create missing routes
	var routeIDs []string
	for _, dest := range desiredDests {
		if existingID, ok := existingByDest[dest]; ok {
			routeIDs = append(routeIDs, existingID)
			continue
		}

		routeName := fmt.Sprintf("roks-%s-gw-%s-%s", r.ClusterID, gw.Name, sanitizeDestination(dest))
		route, err := r.VPC.CreateRoute(ctx, r.VPCID, rtID, vpc.CreateRouteOptions{
			Name:        routeName,
			Destination: dest,
			Action:      "deliver",
			NextHopIP:   gw.Status.ReservedIP,
			Zone:        gw.Spec.Zone,
		})
		if err != nil {
			return nil, fmt.Errorf("CreateRoute(%s): %w", dest, err)
		}
		routeIDs = append(routeIDs, route.ID)
	}

	// Delete stale routes (previously tracked but no longer desired)
	if err := r.deleteStaleRoutes(ctx, gw, desiredSet); err != nil {
		logger.Error(err, "Failed to delete stale routes")
		// Non-fatal: routes were created successfully
	}

	return routeIDs, nil
}

// deleteStaleRoutes removes VPC routes that are tracked in status but no longer desired.
func (r *Reconciler) deleteStaleRoutes(ctx context.Context, gw *v1alpha1.VPCGateway, desiredSet map[string]bool) error {
	if len(gw.Status.VPCRouteIDs) == 0 {
		return nil
	}

	rtID, err := r.getDefaultRoutingTableID(ctx)
	if err != nil {
		return fmt.Errorf("getDefaultRoutingTableID: %w", err)
	}

	// Get existing routes to check destinations
	existingRoutes, err := r.VPC.ListRoutes(ctx, r.VPCID, rtID)
	if err != nil {
		return fmt.Errorf("ListRoutes: %w", err)
	}

	trackedSet := make(map[string]bool, len(gw.Status.VPCRouteIDs))
	for _, id := range gw.Status.VPCRouteIDs {
		trackedSet[id] = true
	}

	for _, route := range existingRoutes {
		// Only consider routes we previously tracked
		if !trackedSet[route.ID] {
			continue
		}
		// If this route's destination is no longer desired, delete it
		if desiredSet == nil || !desiredSet[route.Destination] {
			if err := r.VPC.DeleteRoute(ctx, r.VPCID, rtID, route.ID); err != nil {
				if !isVPCNotFound(err) {
					return fmt.Errorf("DeleteRoute(%s): %w", route.ID, err)
				}
			}
		}
	}

	return nil
}

// ensureFloatingIP creates or adopts a floating IP and binds it to the gateway's VNI.
func (r *Reconciler) ensureFloatingIP(ctx context.Context, gw *v1alpha1.VPCGateway) error {
	logger := log.FromContext(ctx)
	fipSpec := gw.Spec.FloatingIP

	if gw.Status.FloatingIPID != "" {
		// Drift detection: verify FIP still exists
		existing, err := r.VPC.GetFloatingIP(ctx, gw.Status.FloatingIPID)
		if err != nil && isVPCNotFound(err) {
			logger.Info("Floating IP no longer exists, will recreate", "fipID", gw.Status.FloatingIPID)
			r.emitEvent(gw, "Warning", "FIPDrift", "Floating IP %s no longer exists", gw.Status.FloatingIPID)
			gw.Status.FloatingIPID = ""
			gw.Status.FloatingIP = ""
			return r.ensureFloatingIP(ctx, gw)
		} else if err != nil {
			return fmt.Errorf("GetFloatingIP(%s): %w", gw.Status.FloatingIPID, err)
		}
		// Backfill address if missing
		if gw.Status.FloatingIP == "" {
			gw.Status.FloatingIP = existing.Address
		}
		return nil
	}

	if fipSpec.ID != "" {
		// Adopt externally-managed FIP — bind it to our VNI
		fip, err := r.VPC.UpdateFloatingIP(ctx, fipSpec.ID, vpc.UpdateFloatingIPOptions{
			TargetID: gw.Status.VNIID,
		})
		if err != nil {
			return fmt.Errorf("UpdateFloatingIP(%s, bind to %s): %w", fipSpec.ID, gw.Status.VNIID, err)
		}
		gw.Status.FloatingIPID = fip.ID
		gw.Status.FloatingIP = fip.Address
		logger.Info("Adopted external floating IP", "fipID", fip.ID, "address", fip.Address)
		r.emitEvent(gw, "Normal", "FIPAdopted", "Adopted external floating IP %s (%s)", fip.ID, fip.Address)
		return nil
	}

	// Create new FIP bound to the gateway's VNI
	fipName := network.TruncateVPCName(fmt.Sprintf("roks-%s-gw-%s-fip", r.ClusterID, gw.Name))
	fip, err := r.VPC.CreateFloatingIP(ctx, vpc.CreateFloatingIPOptions{
		Name:  fipName,
		Zone:  gw.Spec.Zone,
		VNIID: gw.Status.VNIID,
	})
	if err != nil {
		return fmt.Errorf("CreateFloatingIP(%s): %w", fipName, err)
	}

	gw.Status.FloatingIPID = fip.ID
	gw.Status.FloatingIP = fip.Address
	logger.Info("Created floating IP", "fipID", fip.ID, "address", fip.Address)
	r.emitEvent(gw, "Normal", "FIPCreated", "Created floating IP %s (%s) bound to VNI %s", fip.ID, fip.Address, gw.Status.VNIID)
	return nil
}

// ensurePAR creates/adopts the Public Address Range, ingress routing table,
// and ingress route pointing the PAR CIDR to the gateway's VNI reserved IP.
func (r *Reconciler) ensurePAR(ctx context.Context, gw *v1alpha1.VPCGateway) error {
	logger := log.FromContext(ctx)
	par := gw.Spec.PublicAddressRange

	// Step A: Ensure PAR exists
	if gw.Status.PublicAddressRangeID == "" {
		if par.ID != "" {
			// Adopt externally-managed PAR
			existing, err := r.VPC.GetPublicAddressRange(ctx, par.ID)
			if err != nil {
				return fmt.Errorf("GetPublicAddressRange(%s): %w", par.ID, err)
			}
			gw.Status.PublicAddressRangeID = existing.ID
			gw.Status.PublicAddressRangeCIDR = existing.CIDR
			logger.Info("Adopted external PAR", "parID", existing.ID, "cidr", existing.CIDR)
			r.emitEvent(gw, "Normal", "PARAdopted", "Adopted external PAR %s (%s)", existing.ID, existing.CIDR)
		} else {
			// Create new PAR
			prefixLen := par.PrefixLength
			if prefixLen == 0 {
				prefixLen = 32
			}
			parName := network.TruncateVPCName(fmt.Sprintf("roks-%s-gw-%s-par", r.ClusterID, gw.Name))
			created, err := r.VPC.CreatePublicAddressRange(ctx, vpc.CreatePublicAddressRangeOptions{
				Name:         parName,
				VPCID:        r.VPCID,
				Zone:         gw.Spec.Zone,
				PrefixLength: prefixLen,
			})
			if err != nil {
				return fmt.Errorf("CreatePublicAddressRange: %w", err)
			}
			gw.Status.PublicAddressRangeID = created.ID
			gw.Status.PublicAddressRangeCIDR = created.CIDR
			logger.Info("Created PAR", "parID", created.ID, "cidr", created.CIDR)
			r.emitEvent(gw, "Normal", "PARCreated", "Created PAR %s (%s)", created.ID, created.CIDR)
		}
	} else {
		// Drift detection: verify PAR still exists
		existing, err := r.VPC.GetPublicAddressRange(ctx, gw.Status.PublicAddressRangeID)
		if err != nil && isVPCNotFound(err) {
			logger.Info("PAR no longer exists, will recreate", "parID", gw.Status.PublicAddressRangeID)
			r.emitEvent(gw, "Warning", "PARDrift", "PAR %s no longer exists", gw.Status.PublicAddressRangeID)
			gw.Status.PublicAddressRangeID = ""
			gw.Status.PublicAddressRangeCIDR = ""
			gw.Status.IngressRoutingTableID = ""
			gw.Status.IngressRouteIDs = nil
			return r.ensurePAR(ctx, gw)
		} else if err != nil {
			return fmt.Errorf("GetPublicAddressRange(%s): %w", gw.Status.PublicAddressRangeID, err)
		}
		// Backfill CIDR if missing (upgrade path)
		if gw.Status.PublicAddressRangeCIDR == "" {
			gw.Status.PublicAddressRangeCIDR = existing.CIDR
		}
	}

	// Step B: Ensure ingress routing table
	if gw.Status.IngressRoutingTableID == "" {
		rtName := network.TruncateVPCName(fmt.Sprintf("roks-%s-gw-%s-ingress", r.ClusterID, gw.Name))
		rt, err := r.VPC.CreateRoutingTable(ctx, r.VPCID, vpc.CreateRoutingTableOptions{
			Name:                 rtName,
			RouteInternetIngress: true,
		})
		if err != nil {
			return fmt.Errorf("CreateRoutingTable(ingress): %w", err)
		}
		gw.Status.IngressRoutingTableID = rt.ID
		logger.Info("Created ingress routing table", "rtID", rt.ID)
		r.emitEvent(gw, "Normal", "IngressRTCreated", "Created ingress routing table %s", rt.ID)
	}

	// Step C: Ensure ingress route (PAR CIDR → gateway VNI IP)
	if len(gw.Status.IngressRouteIDs) == 0 && gw.Status.PublicAddressRangeCIDR != "" {
		routeName := fmt.Sprintf("roks-%s-gw-%s-par-ingress", r.ClusterID, gw.Name)
		route, err := r.VPC.CreateRoute(ctx, r.VPCID, gw.Status.IngressRoutingTableID, vpc.CreateRouteOptions{
			Name:        network.TruncateVPCName(routeName),
			Destination: gw.Status.PublicAddressRangeCIDR,
			Action:      "deliver",
			NextHopIP:   gw.Status.ReservedIP,
			Zone:        gw.Spec.Zone,
		})
		if err != nil {
			return fmt.Errorf("CreateRoute(ingress, %s): %w", gw.Status.PublicAddressRangeCIDR, err)
		}
		gw.Status.IngressRouteIDs = []string{route.ID}
		logger.Info("Created ingress route", "routeID", route.ID, "dest", gw.Status.PublicAddressRangeCIDR, "nextHop", gw.Status.ReservedIP)
		r.emitEvent(gw, "Normal", "IngressRouteCreated", "Created ingress route %s → %s", gw.Status.PublicAddressRangeCIDR, gw.Status.ReservedIP)
	}

	return nil
}

// deletePAR cleans up PAR resources in reverse order: ingress routes → routing table → PAR.
func (r *Reconciler) deletePAR(ctx context.Context, gw *v1alpha1.VPCGateway) error {
	logger := log.FromContext(ctx)

	// Delete ingress routes
	if len(gw.Status.IngressRouteIDs) > 0 && gw.Status.IngressRoutingTableID != "" {
		for _, routeID := range gw.Status.IngressRouteIDs {
			if err := r.VPC.DeleteRoute(ctx, r.VPCID, gw.Status.IngressRoutingTableID, routeID); err != nil {
				if !isVPCNotFound(err) {
					return fmt.Errorf("DeleteRoute(ingress, %s): %w", routeID, err)
				}
				logger.Info("Ingress route already deleted", "routeID", routeID)
			}
		}
		logger.Info("Deleted ingress routes", "count", len(gw.Status.IngressRouteIDs))
		r.emitEvent(gw, "Normal", "IngressRoutesDeleted", "Deleted %d ingress route(s)", len(gw.Status.IngressRouteIDs))
	}

	// Delete ingress routing table
	if gw.Status.IngressRoutingTableID != "" {
		if err := r.VPC.DeleteRoutingTable(ctx, r.VPCID, gw.Status.IngressRoutingTableID); err != nil {
			if !isVPCNotFound(err) {
				return fmt.Errorf("DeleteRoutingTable(ingress, %s): %w", gw.Status.IngressRoutingTableID, err)
			}
			logger.Info("Ingress routing table already deleted", "rtID", gw.Status.IngressRoutingTableID)
		} else {
			logger.Info("Deleted ingress routing table", "rtID", gw.Status.IngressRoutingTableID)
		}
		r.emitEvent(gw, "Normal", "IngressRTDeleted", "Deleted ingress routing table %s", gw.Status.IngressRoutingTableID)
	}

	// Delete PAR (only if we created it — skip if externally managed via spec.publicAddressRange.id)
	isExternalPAR := gw.Spec.PublicAddressRange != nil && gw.Spec.PublicAddressRange.ID != ""
	if gw.Status.PublicAddressRangeID != "" && !isExternalPAR {
		if err := r.VPC.DeletePublicAddressRange(ctx, gw.Status.PublicAddressRangeID); err != nil {
			if !isVPCNotFound(err) {
				return fmt.Errorf("DeletePublicAddressRange(%s): %w", gw.Status.PublicAddressRangeID, err)
			}
			logger.Info("PAR already deleted", "parID", gw.Status.PublicAddressRangeID)
		} else {
			logger.Info("Deleted PAR", "parID", gw.Status.PublicAddressRangeID)
		}
		operatormetrics.ReconcileOpsTotal.WithLabelValues("vpcgateway", "delete_par", "success").Inc()
		r.emitEvent(gw, "Normal", "PARDeleted", "Deleted PAR %s", gw.Status.PublicAddressRangeID)
	} else if isExternalPAR {
		logger.Info("Skipping PAR deletion (externally managed)", "parID", gw.Status.PublicAddressRangeID)
	}

	return nil
}

// getDefaultRoutingTableID finds the default routing table for the VPC.
func (r *Reconciler) getDefaultRoutingTableID(ctx context.Context) (string, error) {
	tables, err := r.VPC.ListRoutingTables(ctx, r.VPCID)
	if err != nil {
		return "", fmt.Errorf("ListRoutingTables: %w", err)
	}

	for _, table := range tables {
		if table.IsDefault {
			return table.ID, nil
		}
	}

	return "", fmt.Errorf("no default routing table found for VPC %s", r.VPCID)
}

// setFailedStatus updates the gateway status to reflect a failure.
func (r *Reconciler) setFailedStatus(ctx context.Context, gw *v1alpha1.VPCGateway, reason, message string) {
	logger := log.FromContext(ctx)
	gw.Status.Phase = "Error"
	gw.Status.SyncStatus = "Failed"
	gw.Status.Message = message
	meta.SetStatusCondition(&gw.Status.Conditions, metav1.Condition{
		Type:    "Ready",
		Status:  metav1.ConditionFalse,
		Reason:  reason,
		Message: message,
	})
	if err := r.Status().Update(ctx, gw); err != nil {
		logger.Error(err, "Failed to update status after failure")
	}
}

func (r *Reconciler) emitEvent(obj runtime.Object, eventType, reason, messageFmt string, args ...interface{}) {
	if r.Recorder != nil {
		r.Recorder.Eventf(obj, eventType, reason, messageFmt, args...)
	}
}

// isVPCNotFound returns true if the error indicates the VPC resource is already gone.
func isVPCNotFound(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "not found") || strings.Contains(msg, "not_found") || strings.Contains(msg, "404")
}

// sanitizeDestination replaces characters in a CIDR that are invalid in VPC
// resource names (e.g. "/" becomes "-").
func sanitizeDestination(dest string) string {
	result := make([]byte, len(dest))
	for i := range dest {
		switch dest[i] {
		case '/', '.':
			result[i] = '-'
		default:
			result[i] = dest[i]
		}
	}
	return string(result)
}

// SetupWithManager registers the VPCGateway reconciler with the controller manager.
// Watches VPCRouter changes so that advertised routes from routers are collected
// and created as VPC routes automatically.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Recorder = mgr.GetEventRecorderFor("vpcgateway-controller")
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.VPCGateway{}).
		Watches(&v1alpha1.VPCRouter{}, handler.EnqueueRequestsFromMapFunc(
			func(ctx context.Context, obj client.Object) []reconcile.Request {
				rt, ok := obj.(*v1alpha1.VPCRouter)
				if !ok {
					return nil
				}
				if rt.Spec.Gateway == "" {
					return nil
				}
				return []reconcile.Request{{
					NamespacedName: types.NamespacedName{
						Name:      rt.Spec.Gateway,
						Namespace: rt.Namespace,
					},
				}}
			},
		)).
		Watches(&v1alpha1.VPCVPNGateway{}, handler.EnqueueRequestsFromMapFunc(
			func(ctx context.Context, obj client.Object) []reconcile.Request {
				vpn, ok := obj.(*v1alpha1.VPCVPNGateway)
				if !ok || vpn.Spec.GatewayRef == "" {
					return nil
				}
				return []reconcile.Request{{
					NamespacedName: types.NamespacedName{
						Name:      vpn.Spec.GatewayRef,
						Namespace: vpn.Namespace,
					},
				}}
			},
		)).
		Complete(r)
}
