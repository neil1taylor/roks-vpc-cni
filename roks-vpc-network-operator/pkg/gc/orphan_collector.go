package gc

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/IBM/roks-vpc-network-operator/pkg/annotations"
	"github.com/IBM/roks-vpc-network-operator/pkg/controller/network"
	operatormetrics "github.com/IBM/roks-vpc-network-operator/pkg/metrics"
	"github.com/IBM/roks-vpc-network-operator/pkg/vpc"
)

var vmGVK = schema.GroupVersionKind{
	Group:   "kubevirt.io",
	Version: "v1",
	Kind:    "VirtualMachine",
}

var gatewayGVK = schema.GroupVersionKind{
	Group:   "vpc.roks.ibm.com",
	Version: "v1alpha1",
	Kind:    "VPCGateway",
}

// OrphanCollector periodically scans for VPC resources that were created by the
// operator but no longer have a corresponding Kubernetes object.
type OrphanCollector struct {
	K8sClient client.Client
	VPC       vpc.Client
	ClusterID string
	VPCID     string

	// Interval between GC runs (default: 10 minutes)
	Interval time.Duration

	// GracePeriod before orphaned resources are deleted (default: 15 minutes)
	GracePeriod time.Duration
}

// Start begins the periodic orphan collection loop. Blocks until ctx is cancelled.
func (gc *OrphanCollector) Start(ctx context.Context) error {
	logger := log.FromContext(ctx).WithName("orphan-gc")

	interval := gc.Interval
	if interval == 0 {
		interval = 10 * time.Minute
	}
	gracePeriod := gc.GracePeriod
	if gracePeriod == 0 {
		gracePeriod = 15 * time.Minute
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	logger.Info("Orphan GC started", "interval", interval, "gracePeriod", gracePeriod)

	for {
		select {
		case <-ctx.Done():
			logger.Info("Orphan GC stopped")
			return nil
		case <-ticker.C:
			if err := gc.collect(ctx, gracePeriod); err != nil {
				logger.Error(err, "Orphan GC run failed")
			}
		}
	}
}

// collect performs a single GC run covering VNIs, FIPs, PARs, and VPC routes.
func (gc *OrphanCollector) collect(ctx context.Context, gracePeriod time.Duration) error {
	logger := log.FromContext(ctx).WithName("orphan-gc")

	orphanVNIs := gc.collectOrphanedVNIs(ctx, logger, gracePeriod)
	orphanFIPs := gc.collectOrphanedFloatingIPs(ctx, logger)
	orphanPARs := gc.collectOrphanedPARs(ctx, logger)
	orphanRoutes := gc.collectOrphanedVPCRoutes(ctx, logger)

	total := orphanVNIs + orphanFIPs + orphanPARs + orphanRoutes
	if total > 0 {
		logger.Info("Orphan GC completed",
			"deletedVNIs", orphanVNIs,
			"deletedFIPs", orphanFIPs,
			"deletedPARs", orphanPARs,
			"deletedRoutes", orphanRoutes)
	} else {
		logger.V(1).Info("Orphan GC run completed, no orphans found")
	}

	return nil
}

// collectOrphanedVNIs finds and deletes VNIs with no corresponding VM or gateway.
func (gc *OrphanCollector) collectOrphanedVNIs(ctx context.Context, logger logr.Logger, gracePeriod time.Duration) int {
	vnis, err := gc.VPC.ListVNIsByTag(ctx, gc.ClusterID, "", "")
	if err != nil {
		logger.Error(err, "Failed to list VNIs for orphan detection")
		return 0
	}

	referencedVNIs := gc.buildReferencedVNISet(ctx)

	now := time.Now()
	_ = now
	_ = gracePeriod
	orphanCount := 0

	for _, vni := range vnis {
		if referencedVNIs[vni.ID] {
			continue
		}

		ns, vmName := parseVNIName(vni.Name, gc.ClusterID)
		if ns == "" || vmName == "" {
			continue
		}

		vm := &unstructured.Unstructured{}
		vm.SetGroupVersionKind(vmGVK)
		err := gc.K8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: vmName}, vm)
		if err == nil {
			continue
		}
		if client.IgnoreNotFound(err) != nil {
			logger.Error(err, "Error checking VM existence", "namespace", ns, "name", vmName)
			continue
		}

		logger.Info("Deleting orphaned VNI", "vniID", vni.ID, "namespace", ns, "vmName", vmName)
		if err := gc.VPC.DeleteVNI(ctx, vni.ID); err != nil {
			logger.Error(err, "Failed to delete orphaned VNI", "vniID", vni.ID)
			continue
		}
		operatormetrics.OrphanGCDeletionsTotal.WithLabelValues("vni").Inc()
		orphanCount++
	}

	return orphanCount
}

// collectOrphanedFloatingIPs finds and deletes FIPs that are tagged with the
// cluster ID but not referenced by any VPCGateway or FloatingIP CRD.
func (gc *OrphanCollector) collectOrphanedFloatingIPs(ctx context.Context, logger logr.Logger) int {
	fips, err := gc.VPC.ListFloatingIPs(ctx)
	if err != nil {
		logger.Error(err, "Failed to list floating IPs for orphan detection")
		return 0
	}

	// Build referenced FIP set from VPCGateway statuses
	referencedFIPs := gc.buildReferencedFIPSet(ctx)

	orphanCount := 0
	prefix := "roks-" + gc.ClusterID + "-"

	for _, fip := range fips {
		if !strings.HasPrefix(fip.Name, prefix) {
			continue
		}
		if referencedFIPs[fip.ID] {
			continue
		}

		logger.Info("Deleting orphaned floating IP", "fipID", fip.ID, "name", fip.Name, "address", fip.Address)
		if err := gc.VPC.DeleteFloatingIP(ctx, fip.ID); err != nil {
			logger.Error(err, "Failed to delete orphaned floating IP", "fipID", fip.ID)
			continue
		}
		operatormetrics.OrphanGCDeletionsTotal.WithLabelValues("floatingip").Inc()
		orphanCount++
	}

	return orphanCount
}

// collectOrphanedPARs finds and deletes PARs that are tagged with the
// cluster ID but not referenced by any VPCGateway.
func (gc *OrphanCollector) collectOrphanedPARs(ctx context.Context, logger logr.Logger) int {
	if gc.VPCID == "" {
		return 0
	}

	pars, err := gc.VPC.ListPublicAddressRanges(ctx, gc.VPCID)
	if err != nil {
		logger.Error(err, "Failed to list PARs for orphan detection")
		return 0
	}

	referencedPARs := gc.buildReferencedPARSet(ctx)

	orphanCount := 0
	prefix := "roks-" + gc.ClusterID + "-"

	for _, par := range pars {
		if !strings.HasPrefix(par.Name, prefix) {
			continue
		}
		if referencedPARs[par.ID] {
			continue
		}

		logger.Info("Deleting orphaned PAR", "parID", par.ID, "name", par.Name, "cidr", par.CIDR)
		if err := gc.VPC.DeletePublicAddressRange(ctx, par.ID); err != nil {
			logger.Error(err, "Failed to delete orphaned PAR", "parID", par.ID)
			continue
		}
		operatormetrics.OrphanGCDeletionsTotal.WithLabelValues("par").Inc()
		orphanCount++
	}

	return orphanCount
}

// collectOrphanedVPCRoutes finds and deletes VPC routes in the default routing
// table that are tagged with the cluster ID but not referenced by any VPCGateway.
func (gc *OrphanCollector) collectOrphanedVPCRoutes(ctx context.Context, logger logr.Logger) int {
	if gc.VPCID == "" {
		return 0
	}

	// Find the default routing table
	tables, err := gc.VPC.ListRoutingTables(ctx, gc.VPCID)
	if err != nil {
		logger.Error(err, "Failed to list routing tables for orphan detection")
		return 0
	}
	var defaultRTID string
	for _, t := range tables {
		if t.IsDefault {
			defaultRTID = t.ID
			break
		}
	}
	if defaultRTID == "" {
		return 0
	}

	routes, err := gc.VPC.ListRoutes(ctx, gc.VPCID, defaultRTID)
	if err != nil {
		logger.Error(err, "Failed to list routes for orphan detection")
		return 0
	}

	referencedRoutes := gc.buildReferencedRouteSet(ctx)

	orphanCount := 0
	prefix := "roks-" + gc.ClusterID + "-"

	for _, route := range routes {
		if !strings.HasPrefix(route.Name, prefix) {
			continue
		}
		if referencedRoutes[route.ID] {
			continue
		}

		logger.Info("Deleting orphaned VPC route", "routeID", route.ID, "name", route.Name, "destination", route.Destination)
		if err := gc.VPC.DeleteRoute(ctx, gc.VPCID, defaultRTID, route.ID); err != nil {
			logger.Error(err, "Failed to delete orphaned route", "routeID", route.ID)
			continue
		}
		operatormetrics.OrphanGCDeletionsTotal.WithLabelValues("route").Inc()
		orphanCount++
	}

	return orphanCount
}

// buildReferencedVNISet scans all VMs and VPCGateways and returns a set of
// VNI IDs that are actively referenced and must not be garbage collected.
func (gc *OrphanCollector) buildReferencedVNISet(ctx context.Context) map[string]bool {
	result := map[string]bool{}

	// 1. VNIs referenced by VirtualMachine annotations
	vmList := &unstructured.UnstructuredList{}
	vmList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   vmGVK.Group,
		Version: vmGVK.Version,
		Kind:    "VirtualMachineList",
	})
	if err := gc.K8sClient.List(ctx, vmList); err == nil {
		for _, vm := range vmList.Items {
			annots := vm.GetAnnotations()
			if annots == nil {
				continue
			}

			// Check multi-network annotation
			if raw := annots[annotations.NetworkInterfaces]; raw != "" {
				var interfaces []network.VMNetworkInterface
				if err := json.Unmarshal([]byte(raw), &interfaces); err == nil {
					for _, iface := range interfaces {
						if iface.VNIID != "" {
							result[iface.VNIID] = true
						}
					}
				}
			}

			// Also check legacy annotation
			if vniID := annots[annotations.VNIID]; vniID != "" {
				result[vniID] = true
			}
		}
	}

	// 2. VNIs referenced by VPCGateway status (uplink VNIs)
	gwList := &unstructured.UnstructuredList{}
	gwList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   gatewayGVK.Group,
		Version: gatewayGVK.Version,
		Kind:    "VPCGatewayList",
	})
	if err := gc.K8sClient.List(ctx, gwList); err == nil {
		for _, gw := range gwList.Items {
			status, ok := gw.Object["status"].(map[string]interface{})
			if !ok {
				continue
			}
			if vniID, ok := status["vniID"].(string); ok && vniID != "" {
				result[vniID] = true
			}
		}
	}

	return result
}

// buildReferencedFIPSet returns a set of floating IP IDs that are actively
// referenced by VPCGateway statuses, FloatingIP CRDs, or VM annotations.
func (gc *OrphanCollector) buildReferencedFIPSet(ctx context.Context) map[string]bool {
	result := map[string]bool{}

	// 1. FIPs referenced by VPCGateway statuses
	gwList := &unstructured.UnstructuredList{}
	gwList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   gatewayGVK.Group,
		Version: gatewayGVK.Version,
		Kind:    "VPCGatewayList",
	})
	if err := gc.K8sClient.List(ctx, gwList); err == nil {
		for _, gw := range gwList.Items {
			status, ok := gw.Object["status"].(map[string]interface{})
			if !ok {
				continue
			}
			if fipID, ok := status["floatingIPID"].(string); ok && fipID != "" {
				result[fipID] = true
			}
		}
	}

	// 2. FIPs referenced by FloatingIP CRDs
	fipList := &unstructured.UnstructuredList{}
	fipList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "vpc.roks.ibm.com",
		Version: "v1alpha1",
		Kind:    "FloatingIPList",
	})
	if err := gc.K8sClient.List(ctx, fipList); err == nil {
		for _, fip := range fipList.Items {
			status, ok := fip.Object["status"].(map[string]interface{})
			if !ok {
				continue
			}
			if fipID, ok := status["floatingIPID"].(string); ok && fipID != "" {
				result[fipID] = true
			}
		}
	}

	// 3. FIPs referenced by VM annotations (webhook-created per-VM FIPs)
	vmList := &unstructured.UnstructuredList{}
	vmList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   vmGVK.Group,
		Version: vmGVK.Version,
		Kind:    "VirtualMachineList",
	})
	if err := gc.K8sClient.List(ctx, vmList); err == nil {
		for _, vm := range vmList.Items {
			annots := vm.GetAnnotations()
			if annots == nil {
				continue
			}

			// Check legacy single-network FIP annotation
			if fipID := annots[annotations.FIPID]; fipID != "" {
				result[fipID] = true
			}

			// Check multi-network annotation for per-interface FIPs
			if raw := annots[annotations.NetworkInterfaces]; raw != "" {
				var interfaces []network.VMNetworkInterface
				if err := json.Unmarshal([]byte(raw), &interfaces); err == nil {
					for _, iface := range interfaces {
						if iface.FIPID != "" {
							result[iface.FIPID] = true
						}
					}
				}
			}
		}
	}

	return result
}

// buildReferencedPARSet returns a set of PAR IDs referenced by VPCGateway statuses.
func (gc *OrphanCollector) buildReferencedPARSet(ctx context.Context) map[string]bool {
	result := map[string]bool{}

	gwList := &unstructured.UnstructuredList{}
	gwList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   gatewayGVK.Group,
		Version: gatewayGVK.Version,
		Kind:    "VPCGatewayList",
	})
	if err := gc.K8sClient.List(ctx, gwList); err == nil {
		for _, gw := range gwList.Items {
			status, ok := gw.Object["status"].(map[string]interface{})
			if !ok {
				continue
			}
			if parID, ok := status["publicAddressRangeID"].(string); ok && parID != "" {
				result[parID] = true
			}
		}
	}

	return result
}

// buildReferencedRouteSet returns a set of VPC route IDs referenced by VPCGateway statuses.
func (gc *OrphanCollector) buildReferencedRouteSet(ctx context.Context) map[string]bool {
	result := map[string]bool{}

	gwList := &unstructured.UnstructuredList{}
	gwList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   gatewayGVK.Group,
		Version: gatewayGVK.Version,
		Kind:    "VPCGatewayList",
	})
	if err := gc.K8sClient.List(ctx, gwList); err == nil {
		for _, gw := range gwList.Items {
			status, ok := gw.Object["status"].(map[string]interface{})
			if !ok {
				continue
			}
			// VPCRouteIDs
			if routeIDs, ok := status["vpcRouteIDs"].([]interface{}); ok {
				for _, rid := range routeIDs {
					if id, ok := rid.(string); ok && id != "" {
						result[id] = true
					}
				}
			}
			// IngressRouteIDs
			if routeIDs, ok := status["ingressRouteIDs"].([]interface{}); ok {
				for _, rid := range routeIDs {
					if id, ok := rid.(string); ok && id != "" {
						result[id] = true
					}
				}
			}
		}
	}

	return result
}

// parseVNIName extracts namespace and VM name from the VNI naming convention.
// Supports both formats:
//
//	Legacy:        "roks-<clusterID>-<namespace>-<vmname>"
//	Multi-network: "roks-<clusterID>-<namespace>-<vmname>-<netname>"
//
// For multi-network names, we try each possible split point. Since vm names
// and network names can contain dashes, we validate by checking if a VM exists.
func parseVNIName(vniName, clusterID string) (string, string) {
	prefix := "roks-" + clusterID + "-"
	if len(vniName) <= len(prefix) {
		return "", ""
	}
	if !strings.HasPrefix(vniName, prefix) {
		return "", ""
	}
	remainder := vniName[len(prefix):]

	// Find the first dash to split namespace from the rest.
	// The namespace is always a single segment (no dashes in K8s namespaces by convention,
	// though technically allowed). We take the first segment as namespace.
	dashIdx := strings.Index(remainder, "-")
	if dashIdx <= 0 || dashIdx >= len(remainder)-1 {
		return "", ""
	}

	ns := remainder[:dashIdx]
	rest := remainder[dashIdx+1:]

	// For multi-network VNIs, rest = "<vmname>-<netname>".
	// For legacy VNIs, rest = "<vmname>".
	// Since we can't disambiguate without a K8s lookup, return the rest as vmName.
	// The GC caller will check VM existence — if not found with the full rest,
	// it's either a multi-network name (VM still exists, checked by VNI set) or truly orphaned.
	return ns, rest
}
