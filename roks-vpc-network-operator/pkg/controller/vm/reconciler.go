package vm

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/IBM/roks-vpc-network-operator/pkg/annotations"
	"github.com/IBM/roks-vpc-network-operator/pkg/controller/network"
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
	if annots == nil || (annots[annotations.NetworkInterfaces] == "" && annots[annotations.VNIID] == "") {
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

	// Try multi-network annotation first
	if raw := annots[annotations.NetworkInterfaces]; raw != "" {
		var interfaces []network.VMNetworkInterface
		if err := json.Unmarshal([]byte(raw), &interfaces); err != nil {
			logger.Error(err, "Failed to parse network-interfaces annotation")
		} else {
			for _, iface := range interfaces {
				if iface.Topology != "LocalNet" || iface.VNIID == "" {
					continue
				}

				// Delete FIP first
				if iface.FIPID != "" {
					if err := r.VPC.DeleteFloatingIP(ctx, iface.FIPID); err != nil {
						logger.Error(err, "Failed to delete floating IP", "fipID", iface.FIPID, "network", iface.NetworkName)
						operatormetrics.ReconcileOpsTotal.WithLabelValues("vm", "delete_fip", "error").Inc()
					} else {
						operatormetrics.ReconcileOpsTotal.WithLabelValues("vm", "delete_fip", "success").Inc()
						logger.Info("Deleted floating IP", "fipID", iface.FIPID, "network", iface.NetworkName)
					}
				}

				// Delete VNI
				if err := r.VPC.DeleteVNI(ctx, iface.VNIID); err != nil {
					logger.Error(err, "Failed to delete VNI", "vniID", iface.VNIID, "network", iface.NetworkName)
					operatormetrics.ReconcileOpsTotal.WithLabelValues("vm", "delete_vni", "error").Inc()
					return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
				}
				operatormetrics.ReconcileOpsTotal.WithLabelValues("vm", "delete_vni", "success").Inc()
				logger.Info("Deleted VNI", "vniID", iface.VNIID, "network", iface.NetworkName)
			}

			// Remove finalizer after all VNIs deleted
			if err := finalizers.EnsureRemoved(ctx, r.Client, vm, finalizers.VMCleanup); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
	}

	// Fall back to legacy single-annotation cleanup
	if fipID := annots[annotations.FIPID]; fipID != "" {
		if err := r.VPC.DeleteFloatingIP(ctx, fipID); err != nil {
			logger.Error(err, "Failed to delete floating IP", "fipID", fipID)
			operatormetrics.ReconcileOpsTotal.WithLabelValues("vm", "delete_fip", "error").Inc()
		} else {
			operatormetrics.ReconcileOpsTotal.WithLabelValues("vm", "delete_fip", "success").Inc()
			logger.Info("Deleted floating IP", "fipID", fipID)
		}
	}

	if vniID := annots[annotations.VNIID]; vniID != "" {
		if err := r.VPC.DeleteVNI(ctx, vniID); err != nil {
			logger.Error(err, "Failed to delete VNI", "vniID", vniID)
			operatormetrics.ReconcileOpsTotal.WithLabelValues("vm", "delete_vni", "error").Inc()
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
		operatormetrics.ReconcileOpsTotal.WithLabelValues("vm", "delete_vni", "success").Inc()
		logger.Info("Deleted VNI", "vniID", vniID)
	}

	if err := finalizers.EnsureRemoved(ctx, r.Client, vm, finalizers.VMCleanup); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// reconcileDriftCheck verifies that VPC resources referenced by the VM still exist.
// It also renames VNIs that have random-word names (VPC API bug) to the expected
// "roks-..." naming convention.
func (r *Reconciler) reconcileDriftCheck(ctx context.Context, vm client.Object) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	annots := vm.GetAnnotations()

	// Try multi-network annotation first
	if raw := annots[annotations.NetworkInterfaces]; raw != "" {
		var interfaces []network.VMNetworkInterface
		if err := json.Unmarshal([]byte(raw), &interfaces); err == nil {
			for _, iface := range interfaces {
				if iface.Topology != "LocalNet" || iface.VNIID == "" {
					continue
				}
				vni, err := r.VPC.GetVNI(ctx, iface.VNIID)
				if err != nil {
					operatormetrics.ReconcileOpsTotal.WithLabelValues("vm", "drift_check", "drift_detected").Inc()
					logger.Error(err, "VNI drift detected",
						"vniID", iface.VNIID, "network", iface.NetworkName, "vm", vm.GetName())
					continue
				}
				// Backfill rename: if the VNI has a random-word name, rename it
				expectedName := network.TruncateVPCName(fmt.Sprintf("roks-%s-%s-%s-%s",
					r.ClusterID, vm.GetNamespace(), vm.GetName(), iface.NetworkName))
				r.renameVNIIfNeeded(ctx, vni, expectedName)
			}
			return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
		}
	}

	// Fall back to legacy single-VNI check
	if vniID := annots[annotations.VNIID]; vniID != "" {
		vni, err := r.VPC.GetVNI(ctx, vniID)
		if err != nil {
			operatormetrics.ReconcileOpsTotal.WithLabelValues("vm", "drift_check", "drift_detected").Inc()
			logger.Error(err, "VNI drift detected — VNI may have been deleted out-of-band",
				"vniID", vniID, "vm", vm.GetName())
		} else {
			expectedName := network.TruncateVPCName(fmt.Sprintf("roks-%s-%s-%s",
				r.ClusterID, vm.GetNamespace(), vm.GetName()))
			r.renameVNIIfNeeded(ctx, vni, expectedName)
		}
	}

	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

// renameVNIIfNeeded renames a VNI if its current name doesn't match the expected name.
func (r *Reconciler) renameVNIIfNeeded(ctx context.Context, vni *vpc.VNI, expectedName string) {
	if vni.Name == expectedName {
		return
	}
	logger := log.FromContext(ctx)
	logger.Info("Renaming VNI from random name to expected name",
		"vniID", vni.ID, "currentName", vni.Name, "expectedName", expectedName)
	if _, err := r.VPC.UpdateVNI(ctx, vni.ID, expectedName); err != nil {
		logger.Error(err, "Failed to rename VNI", "vniID", vni.ID)
		operatormetrics.ReconcileOpsTotal.WithLabelValues("vm", "rename_vni", "error").Inc()
	} else {
		operatormetrics.ReconcileOpsTotal.WithLabelValues("vm", "rename_vni", "success").Inc()
	}
}

// SetupWithManager registers the VM reconciler with the controller manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(vmGVK)
	return ctrl.NewControllerManagedBy(mgr).
		For(u).
		Complete(r)
}
