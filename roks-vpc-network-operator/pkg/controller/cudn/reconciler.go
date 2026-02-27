package cudn

import (
	"context"
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/IBM/roks-vpc-network-operator/pkg/annotations"
	"github.com/IBM/roks-vpc-network-operator/pkg/controller/network"
	"github.com/IBM/roks-vpc-network-operator/pkg/finalizers"
	"github.com/IBM/roks-vpc-network-operator/pkg/vpc"
)

var cudnGVK = schema.GroupVersionKind{
	Group:   "k8s.ovn.org",
	Version: "v1",
	Kind:    "ClusterUserDefinedNetwork",
}

// Reconciler reconciles ClusterUserDefinedNetwork objects.
// LocalNet CUDNs get VPC subnet + VLAN attachments; Layer2 CUDNs are tracked with no VPC resources.
type Reconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	VPC       vpc.Client
	ClusterID string
}

// Reconcile handles CUDN create/update/delete events.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling CUDN", "name", req.Name)

	// Fetch the ClusterUserDefinedNetwork via unstructured (avoids OVN type import)
	cudn := &unstructured.Unstructured{}
	cudn.SetGroupVersionKind(cudnGVK)
	if err := r.Get(ctx, req.NamespacedName, cudn); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// OVN stores topology at spec.network.topology with values "Localnet", "Layer2", "Layer3"
	topology, _, _ := unstructured.NestedString(cudn.Object, "spec", "network", "topology")
	normalizedTopology := strings.ToLower(topology)

	// Handle deletion
	if !cudn.GetDeletionTimestamp().IsZero() {
		switch normalizedTopology {
		case "localnet":
			return r.reconcileDeleteLocalNet(ctx, cudn)
		case "layer2":
			return r.reconcileDeleteLayer2(ctx, cudn)
		default:
			return ctrl.Result{}, nil
		}
	}

	// Handle create/update based on topology
	switch normalizedTopology {
	case "localnet":
		return r.reconcileLocalNet(ctx, cudn)
	case "layer2":
		return r.reconcileLayer2(ctx, cudn)
	default:
		// Unknown topology — skip
		logger.Info("skipping CUDN with unknown topology", "topology", topology)
		return ctrl.Result{}, nil
	}
}

// reconcileLocalNet handles LocalNet CUDN creation: validate annotations, create VPC subnet, create VLAN attachments.
func (r *Reconciler) reconcileLocalNet(ctx context.Context, cudn client.Object) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	annots := cudn.GetAnnotations()
	if annots == nil {
		annots = map[string]string{}
	}

	// Validate required annotations for LocalNet
	for _, key := range annotations.RequiredLocalNetAnnotations {
		if _, ok := annots[key]; !ok {
			logger.Error(nil, "Missing required annotation", "annotation", key)
			return ctrl.Result{RequeueAfter: 30 * time.Second}, fmt.Errorf("missing required annotation: %s", key)
		}
	}

	// Add finalizer
	if err := finalizers.EnsureAdded(ctx, r.Client, cudn, finalizers.CUDNCleanup); err != nil {
		return ctrl.Result{}, err
	}

	// Create VPC subnet
	if _, err := network.EnsureVPCSubnet(ctx, r.Client, r.VPC, cudn, r.ClusterID, "cudn"); err != nil {
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Create VLAN attachments on all bare metal nodes
	if err := network.EnsureVLANAttachments(ctx, r.Client, r.VPC, cudn, r.ClusterID, "cudn"); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// reconcileLayer2 handles Layer2 CUDN creation: just add finalizer, no VPC resources needed.
func (r *Reconciler) reconcileLayer2(ctx context.Context, cudn client.Object) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Layer2 CUDN — no VPC resources needed", "name", cudn.GetName())

	if err := finalizers.EnsureAdded(ctx, r.Client, cudn, finalizers.CUDNCleanup); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// reconcileDeleteLocalNet handles LocalNet CUDN deletion: delete VLAN attachments, delete VPC subnet, remove finalizer.
func (r *Reconciler) reconcileDeleteLocalNet(ctx context.Context, cudn client.Object) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	annots := cudn.GetAnnotations()

	// Block deletion if VMs still have VNIs on this subnet
	if subnetID := annots[annotations.SubnetID]; subnetID != "" {
		hasVNIs, count, err := network.SubnetHasActiveVNIs(ctx, r.VPC, subnetID)
		if err != nil {
			logger.Error(err, "Failed to check for active VNIs on subnet", "subnetID", subnetID)
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
		if hasVNIs {
			msg := fmt.Sprintf("Cannot delete: %d VM(s) still have VNIs on subnet %s — delete VMs first", count, subnetID)
			logger.Error(nil, msg, "cudn", cudn.GetName())
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
	}

	// Delete VLAN attachments
	if err := network.DeleteVLANAttachments(ctx, r.Client, r.VPC, cudn); err != nil {
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Delete VPC subnet
	if err := network.DeleteVPCSubnet(ctx, r.VPC, cudn); err != nil {
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Remove finalizer
	if err := finalizers.EnsureRemoved(ctx, r.Client, cudn, finalizers.CUDNCleanup); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// reconcileDeleteLayer2 handles Layer2 CUDN deletion: just remove finalizer.
func (r *Reconciler) reconcileDeleteLayer2(ctx context.Context, cudn client.Object) (ctrl.Result, error) {
	if err := finalizers.EnsureRemoved(ctx, r.Client, cudn, finalizers.CUDNCleanup); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager registers the CUDN reconciler with the controller manager.
// Uses unstructured watch to avoid importing OVN-Kubernetes types.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(cudnGVK)
	return ctrl.NewControllerManagedBy(mgr).
		For(u).
		Complete(r)
}

// isBareMetalNode checks if a node is a bare metal worker.
// Kept as package-private for test compatibility.
func isBareMetalNode(node interface{ GetLabels() map[string]string }) bool {
	instanceType := node.GetLabels()["node.kubernetes.io/instance-type"]
	return strings.Contains(instanceType, "metal")
}

// extractBMServerID extracts the bare metal server ID from a node's provider ID.
// Kept as package-private for test compatibility.
func extractBMServerID(providerID string) string {
	return network.ExtractBMServerID(providerID)
}

// parseAttachments parses "node1:att-id-1,node2:att-id-2" into a map.
// Kept as package-private for test compatibility.
func parseAttachments(s string) map[string]string {
	return network.ParseAttachments(s)
}

// serializeAttachments converts a map to "node1:att-id-1,node2:att-id-2".
// Kept as package-private for test compatibility.
func serializeAttachments(m map[string]string) string {
	return network.SerializeAttachments(m)
}
