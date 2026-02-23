package vlanattachment

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
	"github.com/IBM/roks-vpc-network-operator/pkg/roks"
	"github.com/IBM/roks-vpc-network-operator/pkg/vpc"
)

// Reconciler reconciles VLANAttachment objects.
//
// On unmanaged clusters, VLAN attachments are created/deleted via the VPC API.
// On ROKS clusters, VLAN attachments are managed by the ROKS platform and will
// be synced via the ROKS API when it becomes available. Until then, the reconciler
// operates in read-only mode on ROKS.
type Reconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	Recorder  record.EventRecorder
	VPC       vpc.Client
	ROKS      roks.ROKSClient // nil on unmanaged clusters
	ClusterID string
	Mode      roks.ClusterMode
}

// Reconcile handles VLANAttachment create/update/delete events.
//
// On unmanaged clusters:
//   - Full lifecycle via VPC API
//
// On ROKS clusters:
//   - VLAN attachments are managed by ROKS; this controller only syncs status
//   - TODO(roks-api): Implement ROKS API sync when available
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling VLANAttachment", "name", req.Name, "namespace", req.Namespace, "mode", r.Mode)

	// Fetch the VLANAttachment object
	vlanAtt := &v1alpha1.VLANAttachment{}
	if err := r.Get(ctx, req.NamespacedName, vlanAtt); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to fetch VLANAttachment")
		return ctrl.Result{}, err
	}

	// Route to appropriate handler based on cluster mode
	if r.Mode == roks.ModeROKS {
		return r.reconcileROKS(ctx, vlanAtt)
	}

	// Unmanaged cluster: full lifecycle via VPC API
	if !vlanAtt.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, vlanAtt)
	}
	return r.reconcileNormal(ctx, vlanAtt)
}

// reconcileROKS handles VLAN attachment reconciliation on ROKS clusters.
//
// TODO(roks-api): Implement when the ROKS API for VLAN attachment management is available.
func (r *Reconciler) reconcileROKS(ctx context.Context, vlanAtt *v1alpha1.VLANAttachment) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Check if ROKS API is available
	if r.ROKS == nil || !r.ROKS.IsAvailable(ctx) {
		vlanAtt.Status.SyncStatus = "Pending"
		vlanAtt.Status.Message = "VLAN attachment is managed by ROKS platform. ROKS API integration pending."
		now := metav1.Now()
		vlanAtt.Status.LastSyncTime = &now

		if err := r.Status().Update(ctx, vlanAtt); err != nil {
			logger.Error(err, "Failed to update status for ROKS-managed VLAN attachment")
			return ctrl.Result{}, err
		}

		return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
	}

	// TODO(roks-api): When ROKS API is available, sync VLAN attachment status:
	//
	// roksAtt, err := r.ROKS.ListVLANAttachmentsByNode(ctx, vlanAtt.Spec.NodeName)
	// if err != nil {
	//     // Handle error
	// }
	// // Find matching attachment and update status

	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

// reconcileNormal handles VLAN attachment creation/update on unmanaged clusters.
func (r *Reconciler) reconcileNormal(ctx context.Context, vlanAtt *v1alpha1.VLANAttachment) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Step 1: Add finalizer
	if err := finalizers.EnsureAdded(ctx, r.Client, vlanAtt, "vpc.roks.ibm.com/vlan-protection"); err != nil {
		logger.Error(err, "Failed to add finalizer")
		return ctrl.Result{}, err
	}

	// Step 2: Create VLAN attachment (only on initial creation)
	if vlanAtt.Status.AttachmentID == "" {
		subnetID := vlanAtt.Spec.SubnetID
		if subnetID == "" && vlanAtt.Spec.SubnetRef != "" {
			subnetID = vlanAtt.Spec.SubnetRef
		}

		createOpts := vpc.CreateVLANAttachmentOptions{
			BMServerID: vlanAtt.Spec.BMServerID,
			Name:       fmt.Sprintf("roks-%s-%s", r.ClusterID, vlanAtt.Name),
			VLANID:     vlanAtt.Spec.VLANID,
			SubnetID:   subnetID,
		}

		vpcVLANAtt, err := r.VPC.CreateVLANAttachment(ctx, createOpts)
		if err != nil {
			logger.Error(err, "Failed to create VLAN attachment")
			operatormetrics.ReconcileOpsTotal.WithLabelValues("vlanattachment", "create", "error").Inc()
			r.emitEvent(vlanAtt, "Warning", "CreateFailed", "Failed to create VLAN attachment: %v", err)
			meta.SetStatusCondition(&vlanAtt.Status.Conditions, metav1.Condition{
				Type:    "Ready",
				Status:  metav1.ConditionFalse,
				Reason:  "CreateFailed",
				Message: fmt.Sprintf("VPC API error: %v", err),
			})
			vlanAtt.Status.SyncStatus = "Failed"
			vlanAtt.Status.Message = fmt.Sprintf("Failed to create VLAN attachment: %v", err)
			if err := r.Status().Update(ctx, vlanAtt); err != nil {
				logger.Error(err, "Failed to update status after create failure")
			}
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
		logger.Info("Created VLAN attachment", "attachmentID", vpcVLANAtt.ID)
		operatormetrics.ReconcileOpsTotal.WithLabelValues("vlanattachment", "create", "success").Inc()
		r.emitEvent(vlanAtt, "Normal", "Created", "Created VLAN attachment %s", vpcVLANAtt.ID)

		vlanAtt.Status.AttachmentID = vpcVLANAtt.ID
		vlanAtt.Status.AttachmentStatus = "attached"
	}

	// Update common status fields
	vlanAtt.Status.SyncStatus = "Synced"
	meta.SetStatusCondition(&vlanAtt.Status.Conditions, metav1.Condition{
		Type:    "Ready",
		Status:  metav1.ConditionTrue,
		Reason:  "Synced",
		Message: fmt.Sprintf("VLAN attachment %s is ready", vlanAtt.Status.AttachmentID),
	})
	now := metav1.Now()
	vlanAtt.Status.LastSyncTime = &now
	vlanAtt.Status.Message = fmt.Sprintf("Synced with VLAN attachment %s", vlanAtt.Status.AttachmentID)

	if err := r.Status().Update(ctx, vlanAtt); err != nil {
		logger.Error(err, "Failed to update status after create")
		return ctrl.Result{}, err
	}

	logger.Info("Successfully synced VLANAttachment", "attachmentID", vlanAtt.Status.AttachmentID)
	return ctrl.Result{}, nil
}

// reconcileDelete handles VLAN attachment deletion on unmanaged clusters.
func (r *Reconciler) reconcileDelete(ctx context.Context, vlanAtt *v1alpha1.VLANAttachment) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if vlanAtt.Status.AttachmentID != "" {
		if err := r.VPC.DeleteVLANAttachment(ctx, vlanAtt.Spec.BMServerID, vlanAtt.Status.AttachmentID); err != nil {
			logger.Error(err, "Failed to delete VLAN attachment", "attachmentID", vlanAtt.Status.AttachmentID)
			operatormetrics.ReconcileOpsTotal.WithLabelValues("vlanattachment", "delete", "error").Inc()
			r.emitEvent(vlanAtt, "Warning", "DeleteFailed", "Failed to delete VLAN attachment %s: %v", vlanAtt.Status.AttachmentID, err)
			vlanAtt.Status.SyncStatus = "Failed"
			vlanAtt.Status.Message = fmt.Sprintf("Failed to delete VLAN attachment: %v", err)
			if err := r.Status().Update(ctx, vlanAtt); err != nil {
				logger.Error(err, "Failed to update status after delete failure")
			}
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
		logger.Info("Deleted VLAN attachment", "attachmentID", vlanAtt.Status.AttachmentID)
		operatormetrics.ReconcileOpsTotal.WithLabelValues("vlanattachment", "delete", "success").Inc()
		r.emitEvent(vlanAtt, "Normal", "Deleted", "Deleted VLAN attachment %s", vlanAtt.Status.AttachmentID)
	}

	if err := finalizers.EnsureRemoved(ctx, r.Client, vlanAtt, "vpc.roks.ibm.com/vlan-protection"); err != nil {
		logger.Error(err, "Failed to remove finalizer")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *Reconciler) emitEvent(obj runtime.Object, eventType, reason, messageFmt string, args ...interface{}) {
	if r.Recorder != nil {
		r.Recorder.Eventf(obj, eventType, reason, messageFmt, args...)
	}
}

// SetupWithManager registers the VLANAttachment reconciler with the controller manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Recorder = mgr.GetEventRecorderFor("vlanattachment-controller")
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.VLANAttachment{}).
		Complete(r)
}
