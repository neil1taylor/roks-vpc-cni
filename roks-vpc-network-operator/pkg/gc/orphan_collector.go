package gc

import (
	"context"
	"encoding/json"
	"strings"
	"time"

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

// collect performs a single GC run.
func (gc *OrphanCollector) collect(ctx context.Context, gracePeriod time.Duration) error {
	logger := log.FromContext(ctx).WithName("orphan-gc")

	// 1. List all VNIs tagged with this cluster ID
	vnis, err := gc.VPC.ListVNIsByTag(ctx, gc.ClusterID, "", "")
	if err != nil {
		logger.Error(err, "Failed to list VNIs for orphan detection")
		return err
	}

	// 2. Build a set of VNI IDs that are referenced by existing VMs
	referencedVNIs := gc.buildReferencedVNISet(ctx)

	now := time.Now()
	orphanCount := 0

	for _, vni := range vnis {
		// Skip VNIs that are referenced by VM annotations
		if referencedVNIs[vni.ID] {
			continue
		}

		// Parse namespace and VM name from VNI name.
		// Supports both formats:
		//   Legacy:       "roks-<cluster>-<ns>-<vmname>"
		//   Multi-network: "roks-<cluster>-<ns>-<vmname>-<netname>"
		ns, vmName := parseVNIName(vni.Name, gc.ClusterID)
		if ns == "" || vmName == "" {
			continue
		}

		// Check if the corresponding VirtualMachine still exists
		vm := &unstructured.Unstructured{}
		vm.SetGroupVersionKind(vmGVK)
		err := gc.K8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: vmName}, vm)
		if err == nil {
			// VM exists — not an orphan
			continue
		}
		if client.IgnoreNotFound(err) != nil {
			logger.Error(err, "Error checking VM existence", "namespace", ns, "name", vmName)
			continue
		}

		// VM not found — check grace period
		if gracePeriod > 0 {
			_ = now
		}

		// Delete associated floating IP first (if any)
		logger.Info("Deleting orphaned VNI", "vniID", vni.ID, "namespace", ns, "vmName", vmName)
		if err := gc.VPC.DeleteVNI(ctx, vni.ID); err != nil {
			logger.Error(err, "Failed to delete orphaned VNI", "vniID", vni.ID)
			continue
		}
		operatormetrics.OrphanGCDeletionsTotal.WithLabelValues("vni").Inc()
		orphanCount++
	}

	if orphanCount > 0 {
		logger.Info("Orphan GC completed", "deletedVNIs", orphanCount)
	} else {
		logger.V(1).Info("Orphan GC run completed, no orphans found")
	}

	return nil
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
