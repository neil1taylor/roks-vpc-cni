package vni

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	v1alpha1 "github.com/IBM/roks-vpc-network-operator/api/v1alpha1"
	"github.com/IBM/roks-vpc-network-operator/pkg/finalizers"
	operatormetrics "github.com/IBM/roks-vpc-network-operator/pkg/metrics"
	"github.com/IBM/roks-vpc-network-operator/pkg/roks"
	"github.com/IBM/roks-vpc-network-operator/pkg/vpc"
)

// Reconciler reconciles VirtualNetworkInterface objects.
//
// On unmanaged clusters, VNIs are created/deleted via the VPC API (vpc.Client).
// On ROKS clusters, VNIs are managed by the ROKS platform and will be synced
// via the ROKS API (roks.ROKSClient) when it becomes available. Until then,
// the reconciler operates in read-only sync mode on ROKS, only populating
// CRD status from ROKS API responses.
type Reconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	Recorder  record.EventRecorder
	VPC       vpc.Client
	ROKS      roks.ROKSClient // nil on unmanaged clusters
	ClusterID string
	Mode      roks.ClusterMode
}

// Reconcile handles VirtualNetworkInterface create/update/delete events.
//
// On unmanaged clusters:
//   - Creation: Create VPC VNI via vpc.VNIService.CreateVNI()
//   - Deletion: Delete VPC VNI via vpc.VNIService.DeleteVNI() + remove finalizer
//
// On ROKS clusters:
//   - VNIs are managed by ROKS platform; this controller only syncs status
//   - TODO(roks-api): Implement ROKS API sync when available
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling VirtualNetworkInterface", "name", req.Name, "namespace", req.Namespace, "mode", r.Mode)

	// Fetch the VirtualNetworkInterface object
	vni := &v1alpha1.VirtualNetworkInterface{}
	if err := r.Get(ctx, req.NamespacedName, vni); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to fetch VirtualNetworkInterface")
		return ctrl.Result{}, err
	}

	// Route to appropriate handler based on cluster mode
	if r.Mode == roks.ModeROKS {
		return r.reconcileROKS(ctx, vni)
	}

	// Unmanaged cluster: full lifecycle via VPC API
	if !vni.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, vni)
	}
	return r.reconcileNormal(ctx, vni)
}

// reconcileROKS handles VNI reconciliation on ROKS clusters.
// VNIs are managed by the ROKS platform; this controller syncs status from ROKS API.
//
// TODO(roks-api): Implement when the ROKS API for VNI management is available.
// Until then, this method sets a "Pending" status indicating ROKS management.
func (r *Reconciler) reconcileROKS(ctx context.Context, vni *v1alpha1.VirtualNetworkInterface) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Check if ROKS API is available
	if r.ROKS == nil || !r.ROKS.IsAvailable(ctx) {
		// ROKS API not yet available — mark as pending with informative message
		vni.Status.SyncStatus = "Pending"
		vni.Status.Message = "VNI is managed by ROKS platform. ROKS API integration pending."
		now := metav1.Now()
		vni.Status.LastSyncTime = &now

		if err := r.Status().Update(ctx, vni); err != nil {
			logger.Error(err, "Failed to update status for ROKS-managed VNI")
			return ctrl.Result{}, err
		}

		// Requeue periodically to check if ROKS API becomes available
		return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
	}

	// TODO(roks-api): When ROKS API is available, sync VNI status:
	//
	// roksVNI, err := r.ROKS.GetVNIByVM(ctx, vni.Namespace, vni.Spec.VMRef.Name)
	// if err != nil {
	//     // Handle error
	// }
	//
	// vni.Status.VNIID = roksVNI.VPCVNIID
	// vni.Status.MACAddress = roksVNI.MACAddress
	// vni.Status.PrimaryIPv4 = roksVNI.PrimaryIPv4
	// vni.Status.SyncStatus = "Synced"
	// vni.Status.Message = "Synced from ROKS API"

	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

// reconcileNormal handles VNI creation/update on unmanaged clusters via VPC API.
func (r *Reconciler) reconcileNormal(ctx context.Context, vni *v1alpha1.VirtualNetworkInterface) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Step 1: Add finalizer
	if err := finalizers.EnsureAdded(ctx, r.Client, vni, "vpc.roks.ibm.com/vni-protection"); err != nil {
		logger.Error(err, "Failed to add finalizer")
		return ctrl.Result{}, err
	}

	// Step 2: Create VPC VNI (only on initial creation, not on updates)
	if vni.Status.VNIID == "" {
		// Determine subnet ID
		subnetID := vni.Spec.SubnetID
		if subnetID == "" && vni.Spec.SubnetRef != "" {
			// TODO: Look up subnet by reference (VPCSubnet CR name)
			subnetID = vni.Spec.SubnetRef
		}

		// Build VNI creation options
		createOpts := vpc.CreateVNIOptions{
			Name:             vni.Name,
			SubnetID:         subnetID,
			SecurityGroupIDs: vni.Spec.SecurityGroupIDs,
			ClusterID:        vni.Spec.ClusterID,
			Namespace:        vni.Namespace,
			VMName:           "",
		}

		// Extract VM name from reference if available
		if vni.Spec.VMRef != nil {
			createOpts.VMName = vni.Spec.VMRef.Name
		}

		vpcVNI, err := r.VPC.CreateVNI(ctx, createOpts)
		if err != nil {
			logger.Error(err, "Failed to create VPC VNI")
			operatormetrics.ReconcileOpsTotal.WithLabelValues("vni", "create", "error").Inc()
			r.emitEvent(vni, "Warning", "CreateFailed", "Failed to create VNI: %v", err)
			vni.Status.SyncStatus = "Failed"
			vni.Status.Message = fmt.Sprintf("Failed to create VNI: %v", err)
			meta.SetStatusCondition(&vni.Status.Conditions, metav1.Condition{
				Type:    "Ready",
				Status:  metav1.ConditionFalse,
				Reason:  "CreateFailed",
				Message: fmt.Sprintf("VPC API error: %v", err),
			})
			if err := r.Status().Update(ctx, vni); err != nil {
				logger.Error(err, "Failed to update status after create failure")
			}
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
		operatormetrics.ReconcileOpsTotal.WithLabelValues("vni", "create", "success").Inc()
		r.emitEvent(vni, "Normal", "Created", "Created VPC VNI %s", vpcVNI.ID)
		logger.Info("Created VPC VNI", "vniID", vpcVNI.ID)

		// Step 3: Update status with VNI details
		vni.Status.VNIID = vpcVNI.ID
		vni.Status.MACAddress = vpcVNI.MACAddress
		vni.Status.PrimaryIPv4 = vpcVNI.PrimaryIP.Address
		vni.Status.ReservedIPID = vpcVNI.PrimaryIP.ID
	}

	// Update common status fields
	vni.Status.SyncStatus = "Synced"
	now := metav1.Now()
	vni.Status.LastSyncTime = &now
	vni.Status.Message = fmt.Sprintf("Synced with VPC VNI %s", vni.Status.VNIID)
	meta.SetStatusCondition(&vni.Status.Conditions, metav1.Condition{
		Type:    "Ready",
		Status:  metav1.ConditionTrue,
		Reason:  "Synced",
		Message: fmt.Sprintf("VPC VNI %s is ready", vni.Status.VNIID),
	})

	if err := r.Status().Update(ctx, vni); err != nil {
		logger.Error(err, "Failed to update status after create")
		return ctrl.Result{}, err
	}

	logger.Info("Successfully synced VirtualNetworkInterface", "vniID", vni.Status.VNIID)
	return ctrl.Result{}, nil
}

// reconcileDelete handles VNI deletion on unmanaged clusters.
// On ROKS clusters, VNIs are managed by the platform and cannot be deleted directly.
func (r *Reconciler) reconcileDelete(ctx context.Context, vni *v1alpha1.VirtualNetworkInterface) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Step 1: Delete VPC VNI if it exists
	if vni.Status.VNIID != "" {
		if err := r.VPC.DeleteVNI(ctx, vni.Status.VNIID); err != nil {
			logger.Error(err, "Failed to delete VPC VNI", "vniID", vni.Status.VNIID)
			operatormetrics.ReconcileOpsTotal.WithLabelValues("vni", "delete", "error").Inc()
			r.emitEvent(vni, "Warning", "DeleteFailed", "Failed to delete VNI %s: %v", vni.Status.VNIID, err)
			vni.Status.SyncStatus = "Failed"
			vni.Status.Message = fmt.Sprintf("Failed to delete VNI: %v", err)
			if err := r.Status().Update(ctx, vni); err != nil {
				logger.Error(err, "Failed to update status after delete failure")
			}
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
		operatormetrics.ReconcileOpsTotal.WithLabelValues("vni", "delete", "success").Inc()
		r.emitEvent(vni, "Normal", "Deleted", "Deleted VPC VNI %s", vni.Status.VNIID)
		logger.Info("Deleted VPC VNI", "vniID", vni.Status.VNIID)
	}

	// Step 2: Remove finalizer
	if err := finalizers.EnsureRemoved(ctx, r.Client, vni, "vpc.roks.ibm.com/vni-protection"); err != nil {
		logger.Error(err, "Failed to remove finalizer")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// emitEvent records a Kubernetes event if the recorder is configured.
func (r *Reconciler) emitEvent(obj runtime.Object, eventType, reason, messageFmt string, args ...interface{}) {
	if r.Recorder != nil {
		r.Recorder.Eventf(obj, eventType, reason, messageFmt, args...)
	}
}

// SetupWithManager registers the VirtualNetworkInterface reconciler with the controller manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Recorder = mgr.GetEventRecorderFor("vni-controller")
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.VirtualNetworkInterface{}).
		Complete(r)
}
