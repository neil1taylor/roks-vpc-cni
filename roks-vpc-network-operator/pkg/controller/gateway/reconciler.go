package gateway

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	v1alpha1 "github.com/IBM/roks-vpc-network-operator/api/v1alpha1"
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

	// Step 2: Ensure uplink VNI exists
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

	// Step 3: Ensure VPC routes
	routeIDs, err := r.ensureVPCRoutes(ctx, gw)
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
	return ctrl.Result{}, nil
}

// reconcileDelete handles VPCGateway deletion.
func (r *Reconciler) reconcileDelete(ctx context.Context, gw *v1alpha1.VPCGateway) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Step 1: Delete VPC routes
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

	// Step 2: Delete uplink VNI
	if gw.Status.VNIID != "" {
		if err := r.VPC.DeleteVNI(ctx, gw.Status.VNIID); err != nil {
			logger.Error(err, "Failed to delete uplink VNI", "vniID", gw.Status.VNIID)
			operatormetrics.ReconcileOpsTotal.WithLabelValues("vpcgateway", "delete_vni", "error").Inc()
			r.emitEvent(gw, "Warning", "DeleteVNIFailed", "Failed to delete VNI %s: %v", gw.Status.VNIID, err)
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
		logger.Info("Deleted uplink VNI", "vniID", gw.Status.VNIID)
		operatormetrics.ReconcileOpsTotal.WithLabelValues("vpcgateway", "delete_vni", "success").Inc()
		r.emitEvent(gw, "Normal", "VNIDeleted", "Deleted uplink VNI %s", gw.Status.VNIID)
	}

	// Step 3: Remove finalizer
	if err := finalizers.EnsureRemoved(ctx, r.Client, gw, finalizers.GatewayCleanup); err != nil {
		logger.Error(err, "Failed to remove finalizer")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// ensureVNI creates the uplink VNI for the gateway.
func (r *Reconciler) ensureVNI(ctx context.Context, gw *v1alpha1.VPCGateway) (*vpc.VNI, error) {
	name := fmt.Sprintf("roks-%s-gw-%s", r.ClusterID, gw.Name)

	vni, err := r.VPC.CreateVNI(ctx, vpc.CreateVNIOptions{
		Name:             name,
		SecurityGroupIDs: gw.Spec.Uplink.SecurityGroupIDs,
		ClusterID:        r.ClusterID,
	})
	if err != nil {
		return nil, fmt.Errorf("CreateVNI(%s): %w", name, err)
	}

	return vni, nil
}

// ensureVPCRoutes creates VPC routes for the gateway, using idempotent
// checks against existing routes.
func (r *Reconciler) ensureVPCRoutes(ctx context.Context, gw *v1alpha1.VPCGateway) ([]string, error) {
	if len(gw.Spec.VPCRoutes) == 0 {
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

	existingByDest := make(map[string]string) // destination -> routeID
	for _, route := range existingRoutes {
		existingByDest[route.Destination] = route.ID
	}

	var routeIDs []string
	for _, routeSpec := range gw.Spec.VPCRoutes {
		// Check if route already exists for this destination
		if existingID, ok := existingByDest[routeSpec.Destination]; ok {
			routeIDs = append(routeIDs, existingID)
			continue
		}

		routeName := fmt.Sprintf("roks-%s-gw-%s-%s", r.ClusterID, gw.Name, sanitizeDestination(routeSpec.Destination))
		route, err := r.VPC.CreateRoute(ctx, r.VPCID, rtID, vpc.CreateRouteOptions{
			Name:        routeName,
			Destination: routeSpec.Destination,
			Action:      "deliver",
			NextHopIP:   gw.Status.ReservedIP,
			Zone:        gw.Spec.Zone,
		})
		if err != nil {
			return nil, fmt.Errorf("CreateRoute(%s): %w", routeSpec.Destination, err)
		}
		routeIDs = append(routeIDs, route.ID)
	}

	return routeIDs, nil
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
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Recorder = mgr.GetEventRecorderFor("vpcgateway-controller")
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.VPCGateway{}).
		Complete(r)
}
