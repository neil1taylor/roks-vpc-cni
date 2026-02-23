package floatingip

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

// Reconciler reconciles FloatingIP objects.
type Reconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	Recorder  record.EventRecorder
	VPC       vpc.Client
	ClusterID string
}

// Reconcile handles FloatingIP create/update/delete events.
//
// Creation/Update flow:
//  1. Add finalizer
//  2. Create floating IP via vpc.FloatingIPService.CreateFloatingIP()
//  3. Update status with fipID, address, and lastSyncTime
//
// Deletion flow:
//  1. Delete floating IP via vpc.FloatingIPService.DeleteFloatingIP()
//  2. Remove finalizer
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling FloatingIP", "name", req.Name, "namespace", req.Namespace)

	// Fetch the FloatingIP object
	fip := &v1alpha1.FloatingIP{}
	if err := r.Get(ctx, req.NamespacedName, fip); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to fetch FloatingIP")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !fip.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, fip)
	}

	// Handle creation/update
	return r.reconcileNormal(ctx, fip)
}

// reconcileNormal handles FloatingIP creation and updates.
func (r *Reconciler) reconcileNormal(ctx context.Context, fip *v1alpha1.FloatingIP) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Step 1: Add finalizer
	if err := finalizers.EnsureAdded(ctx, r.Client, fip, "vpc.roks.ibm.com/fip-protection"); err != nil {
		logger.Error(err, "Failed to add finalizer")
		return ctrl.Result{}, err
	}

	// Step 2: Create floating IP (only on initial creation)
	if fip.Status.FIPID == "" {
		// Determine VNI ID
		vniID := fip.Spec.VNIID
		if vniID == "" && fip.Spec.VNIRef != "" {
			// TODO: Look up VNI by reference (VirtualNetworkInterface CR name)
			// For now, assume VNIID is set directly
			vniID = fip.Spec.VNIRef
		}

		// Build floating IP creation options
		createOpts := vpc.CreateFloatingIPOptions{
			Name:   fip.Spec.Name,
			Zone:   fip.Spec.Zone,
			VNIID:  vniID,
		}

		// If no name specified, generate one
		if createOpts.Name == "" {
			createOpts.Name = fmt.Sprintf("roks-%s-%s", r.ClusterID, fip.Name)
		}

		vpcFIP, err := r.VPC.CreateFloatingIP(ctx, createOpts)
		if err != nil {
			logger.Error(err, "Failed to create floating IP")
			operatormetrics.ReconcileOpsTotal.WithLabelValues("floatingip", "create", "error").Inc()
			r.emitEvent(fip, "Warning", "CreateFailed", "Failed to create floating IP: %v", err)
			meta.SetStatusCondition(&fip.Status.Conditions, metav1.Condition{
				Type:    "Ready",
				Status:  metav1.ConditionFalse,
				Reason:  "CreateFailed",
				Message: fmt.Sprintf("VPC API error: %v", err),
			})
			fip.Status.SyncStatus = "Failed"
			fip.Status.Message = fmt.Sprintf("Failed to create floating IP: %v", err)
			if err := r.Status().Update(ctx, fip); err != nil {
				logger.Error(err, "Failed to update status after create failure")
			}
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
		logger.Info("Created floating IP", "fipID", vpcFIP.ID)
		operatormetrics.ReconcileOpsTotal.WithLabelValues("floatingip", "create", "success").Inc()
		r.emitEvent(fip, "Normal", "Created", "Created floating IP %s (%s)", vpcFIP.ID, vpcFIP.Address)

		// Step 3: Update status with floating IP details
		fip.Status.FIPID = vpcFIP.ID
		fip.Status.Address = vpcFIP.Address
		fip.Status.TargetVNIID = vpcFIP.Target
	}

	// Update common status fields
	fip.Status.SyncStatus = "Synced"
	now := metav1.Now()
	fip.Status.LastSyncTime = &now
	fip.Status.Message = fmt.Sprintf("Synced with floating IP %s (%s)", fip.Status.FIPID, fip.Status.Address)
	meta.SetStatusCondition(&fip.Status.Conditions, metav1.Condition{
		Type:    "Ready",
		Status:  metav1.ConditionTrue,
		Reason:  "Synced",
		Message: fmt.Sprintf("Floating IP %s (%s) is ready", fip.Status.FIPID, fip.Status.Address),
	})

	if err := r.Status().Update(ctx, fip); err != nil {
		logger.Error(err, "Failed to update status after create")
		return ctrl.Result{}, err
	}

	logger.Info("Successfully synced FloatingIP", "fipID", fip.Status.FIPID, "address", fip.Status.Address)
	return ctrl.Result{}, nil
}

// reconcileDelete handles FloatingIP deletion.
func (r *Reconciler) reconcileDelete(ctx context.Context, fip *v1alpha1.FloatingIP) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Step 1: Delete floating IP if it exists
	if fip.Status.FIPID != "" {
		if err := r.VPC.DeleteFloatingIP(ctx, fip.Status.FIPID); err != nil {
			logger.Error(err, "Failed to delete floating IP", "fipID", fip.Status.FIPID)
			operatormetrics.ReconcileOpsTotal.WithLabelValues("floatingip", "delete", "error").Inc()
			r.emitEvent(fip, "Warning", "DeleteFailed", "Failed to delete floating IP %s: %v", fip.Status.FIPID, err)
			fip.Status.SyncStatus = "Failed"
			fip.Status.Message = fmt.Sprintf("Failed to delete floating IP: %v", err)
			if err := r.Status().Update(ctx, fip); err != nil {
				logger.Error(err, "Failed to update status after delete failure")
			}
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
		logger.Info("Deleted floating IP", "fipID", fip.Status.FIPID)
		operatormetrics.ReconcileOpsTotal.WithLabelValues("floatingip", "delete", "success").Inc()
		r.emitEvent(fip, "Normal", "Deleted", "Deleted floating IP %s", fip.Status.FIPID)
	}

	// Step 2: Remove finalizer
	if err := finalizers.EnsureRemoved(ctx, r.Client, fip, "vpc.roks.ibm.com/fip-protection"); err != nil {
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

// SetupWithManager registers the FloatingIP reconciler with the controller manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Recorder = mgr.GetEventRecorderFor("floatingip-controller")
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.FloatingIP{}).
		Complete(r)
}
