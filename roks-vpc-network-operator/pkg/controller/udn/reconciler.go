package udn

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

var udnGVK = schema.GroupVersionKind{
	Group:   "k8s.ovn.org",
	Version: "v1",
	Kind:    "UserDefinedNetwork",
}

// Reconciler reconciles namespace-scoped UserDefinedNetwork objects.
// LocalNet UDNs get VPC subnet + VLAN attachments; Layer2 UDNs are tracked with no VPC resources.
type Reconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	VPC       vpc.Client
	ClusterID string
}

// Reconcile handles UDN create/update/delete events.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling UDN", "namespace", req.Namespace, "name", req.Name)

	udn := &unstructured.Unstructured{}
	udn.SetGroupVersionKind(udnGVK)
	if err := r.Get(ctx, req.NamespacedName, udn); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// UDN stores topology at spec.topology with values "Layer2", "Layer3"
	topology, _, _ := unstructured.NestedString(udn.Object, "spec", "topology")
	normalizedTopology := strings.ToLower(topology)

	// Handle deletion
	if !udn.GetDeletionTimestamp().IsZero() {
		switch normalizedTopology {
		case "localnet":
			return r.reconcileDeleteLocalNet(ctx, udn)
		case "layer2":
			return r.reconcileDeleteLayer2(ctx, udn)
		default:
			return ctrl.Result{}, nil
		}
	}

	// Handle create/update
	switch normalizedTopology {
	case "localnet":
		return r.reconcileLocalNet(ctx, udn)
	case "layer2":
		return r.reconcileLayer2(ctx, udn)
	default:
		logger.Info("skipping UDN with unsupported topology", "topology", topology)
		return ctrl.Result{}, nil
	}
}

// reconcileLocalNet handles LocalNet UDN creation: validate annotations, create VPC subnet, create VLAN attachments.
func (r *Reconciler) reconcileLocalNet(ctx context.Context, udn client.Object) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	annots := udn.GetAnnotations()
	if annots == nil {
		annots = map[string]string{}
	}

	for _, key := range annotations.RequiredLocalNetAnnotations {
		if _, ok := annots[key]; !ok {
			logger.Error(nil, "Missing required annotation", "annotation", key)
			return ctrl.Result{RequeueAfter: 30 * time.Second}, fmt.Errorf("missing required annotation: %s", key)
		}
	}

	if err := finalizers.EnsureAdded(ctx, r.Client, udn, finalizers.UDNCleanup); err != nil {
		return ctrl.Result{}, err
	}

	if _, err := network.EnsureVPCSubnet(ctx, r.Client, r.VPC, udn, r.ClusterID, "udn"); err != nil {
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	if err := network.EnsureVLANAttachments(ctx, r.Client, r.VPC, udn, r.ClusterID, "udn"); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// reconcileLayer2 handles Layer2 UDN creation: just add finalizer, no VPC resources needed.
func (r *Reconciler) reconcileLayer2(ctx context.Context, udn client.Object) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Layer2 UDN — no VPC resources needed", "namespace", udn.GetNamespace(), "name", udn.GetName())

	if err := finalizers.EnsureAdded(ctx, r.Client, udn, finalizers.UDNCleanup); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// reconcileDeleteLocalNet handles LocalNet UDN deletion.
func (r *Reconciler) reconcileDeleteLocalNet(ctx context.Context, udn client.Object) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	annots := udn.GetAnnotations()

	// Block deletion if VMs still have VNIs on this subnet
	if subnetID := annots[annotations.SubnetID]; subnetID != "" {
		hasVNIs, count, err := network.SubnetHasActiveVNIs(ctx, r.VPC, subnetID)
		if err != nil {
			logger.Error(err, "Failed to check for active VNIs on subnet", "subnetID", subnetID)
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
		if hasVNIs {
			msg := fmt.Sprintf("Cannot delete: %d VM(s) still have VNIs on subnet %s — delete VMs first", count, subnetID)
			logger.Error(nil, msg, "udn", udn.GetName())
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
	}

	if err := network.DeleteVLANAttachments(ctx, r.Client, r.VPC, udn); err != nil {
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	if err := network.DeleteVPCSubnet(ctx, r.VPC, udn); err != nil {
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	if err := finalizers.EnsureRemoved(ctx, r.Client, udn, finalizers.UDNCleanup); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// reconcileDeleteLayer2 handles Layer2 UDN deletion: just remove finalizer.
func (r *Reconciler) reconcileDeleteLayer2(ctx context.Context, udn client.Object) (ctrl.Result, error) {
	if err := finalizers.EnsureRemoved(ctx, r.Client, udn, finalizers.UDNCleanup); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager registers the UDN reconciler with the controller manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(udnGVK)
	return ctrl.NewControllerManagedBy(mgr).
		For(u).
		Complete(r)
}
