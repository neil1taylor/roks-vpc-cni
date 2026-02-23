package vm

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/IBM/roks-vpc-network-operator/pkg/annotations"
	"github.com/IBM/roks-vpc-network-operator/pkg/finalizers"
	operatormetrics "github.com/IBM/roks-vpc-network-operator/pkg/metrics"
	"github.com/IBM/roks-vpc-network-operator/pkg/vpc"
)

var vmGVK = schema.GroupVersionKind{
	Group:   "kubevirt.io",
	Version: "v1",
	Kind:    "VirtualMachine",
}

// Reconciler reconciles VirtualMachine objects that have been processed
// by the mutating webhook (identified by operator-managed annotations).
// See DESIGN.md §6.3 for the full specification.
type Reconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	VPC       vpc.Client
	ClusterID string
}

// Reconcile handles VM update and delete events.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling VM", "namespace", req.Namespace, "name", req.Name)

	// Fetch VirtualMachine via unstructured (avoids KubeVirt type import)
	vm := &unstructured.Unstructured{}
	vm.SetGroupVersionKind(vmGVK)
	if err := r.Get(ctx, req.NamespacedName, vm); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Skip VMs without our annotations (not managed by this operator)
	annots := vm.GetAnnotations()
	if annots == nil || annots[annotations.VNIID] == "" {
		return ctrl.Result{}, nil
	}

	// Handle deletion
	if !vm.GetDeletionTimestamp().IsZero() {
		return r.reconcileDelete(ctx, vm)
	}

	// Drift detection — verify VPC resources exist
	return r.reconcileDriftCheck(ctx, vm)
}

// reconcileDelete handles VM deletion — cleans up VPC resources.
func (r *Reconciler) reconcileDelete(ctx context.Context, vm client.Object) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	annots := vm.GetAnnotations()

	// Step 1: Delete floating IP if present
	if fipID := annots[annotations.FIPID]; fipID != "" {
		if err := r.VPC.DeleteFloatingIP(ctx, fipID); err != nil {
			logger.Error(err, "Failed to delete floating IP", "fipID", fipID)
			operatormetrics.ReconcileOpsTotal.WithLabelValues("vm", "delete_fip", "error").Inc()
			// Continue — don't block on FIP deletion failure
		} else {
			operatormetrics.ReconcileOpsTotal.WithLabelValues("vm", "delete_fip", "success").Inc()
			logger.Info("Deleted floating IP", "fipID", fipID)
		}
	}

	// Step 2: Delete VNI (this auto-deletes the reserved IP if auto_delete was true)
	if vniID := annots[annotations.VNIID]; vniID != "" {
		if err := r.VPC.DeleteVNI(ctx, vniID); err != nil {
			logger.Error(err, "Failed to delete VNI", "vniID", vniID)
			operatormetrics.ReconcileOpsTotal.WithLabelValues("vm", "delete_vni", "error").Inc()
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
		operatormetrics.ReconcileOpsTotal.WithLabelValues("vm", "delete_vni", "success").Inc()
		logger.Info("Deleted VNI", "vniID", vniID)
	}

	// Step 3: Remove finalizer
	if err := finalizers.EnsureRemoved(ctx, r.Client, vm, finalizers.VMCleanup); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// reconcileDriftCheck verifies that VPC resources referenced by the VM still exist.
func (r *Reconciler) reconcileDriftCheck(ctx context.Context, vm client.Object) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	annots := vm.GetAnnotations()

	// Check VNI still exists
	if vniID := annots[annotations.VNIID]; vniID != "" {
		_, err := r.VPC.GetVNI(ctx, vniID)
		if err != nil {
			operatormetrics.ReconcileOpsTotal.WithLabelValues("vm", "drift_check", "drift_detected").Inc()
			logger.Error(err, "VNI drift detected — VNI may have been deleted out-of-band",
				"vniID", vniID, "vm", vm.GetName())
		}
	}

	// Requeue for periodic drift checks (every 5 minutes)
	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

// SetupWithManager registers the VM reconciler with the controller manager.
// Uses unstructured watch to avoid importing KubeVirt types.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(vmGVK)
	return ctrl.NewControllerManagedBy(mgr).
		For(u).
		Complete(r)
}
