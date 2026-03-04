package vpcsubnet

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
	"github.com/IBM/roks-vpc-network-operator/pkg/vpc"
)

// Reconciler reconciles VPCSubnet objects.
type Reconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	Recorder  record.EventRecorder
	VPC       vpc.Client
	ClusterID string
}

// Reconcile handles VPCSubnet create/update/delete events.
//
// Creation/Update flow:
//  1. Add finalizer
//  2. Get or create VPC subnet via vpc.SubnetService
//  3. Update status with subnetID, syncStatus, and lastSyncTime
//
// Deletion flow:
//  1. Delete VPC subnet via vpc.SubnetService.DeleteSubnet()
//  2. Remove finalizer
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling VPCSubnet", "name", req.Name, "namespace", req.Namespace)

	// Fetch the VPCSubnet object
	subnet := &v1alpha1.VPCSubnet{}
	if err := r.Get(ctx, req.NamespacedName, subnet); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to fetch VPCSubnet")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !subnet.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, subnet)
	}

	// Handle creation/update
	return r.reconcileNormal(ctx, subnet)
}

// reconcileNormal handles VPCSubnet creation and updates.
func (r *Reconciler) reconcileNormal(ctx context.Context, subnet *v1alpha1.VPCSubnet) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Step 1: Add finalizer
	if err := finalizers.EnsureAdded(ctx, r.Client, subnet, "vpc.roks.ibm.com/subnet-protection"); err != nil {
		logger.Error(err, "Failed to add finalizer")
		return ctrl.Result{}, err
	}

	// Step 2: Create or get VPC subnet
	subnetName := fmt.Sprintf("roks-%s-%s", r.ClusterID, subnet.Name)

	// Try to get existing subnet first
	var vpcSubnet *vpc.Subnet
	if subnet.Status.SubnetID != "" {
		// Subnet already exists, try to get it
		existing, err := r.VPC.GetSubnet(ctx, subnet.Status.SubnetID)
		if err != nil {
			logger.Error(err, "Failed to get existing subnet", "subnetID", subnet.Status.SubnetID)
			// Continue to try creating a new one
		} else {
			vpcSubnet = existing
		}
	}

	// If no existing subnet, create a new one
	if vpcSubnet == nil {
		createOpts := vpc.CreateSubnetOptions{
			Name:            subnetName,
			VPCID:           subnet.Spec.VPCID,
			Zone:            subnet.Spec.Zone,
			CIDR:            subnet.Spec.IPv4CIDRBlock,
			ACLID:           subnet.Spec.ACLID,
			ResourceGroupID: subnet.Spec.ResourceGroupID,
			ClusterID:       subnet.Spec.ClusterID,
			CUDNName:        subnet.Spec.CUDNName,
			OwnerKind:       "vpcsubnet",
			OwnerName:       subnet.Name,
		}

		var err error
		vpcSubnet, err = r.VPC.CreateSubnet(ctx, createOpts)
		if err != nil {
			logger.Error(err, "Failed to create VPC subnet")
			operatormetrics.ReconcileOpsTotal.WithLabelValues("vpcsubnet", "create", "error").Inc()
			r.emitEvent(subnet, "Warning", "CreateFailed", "Failed to create VPC subnet: %v", err)
			subnet.Status.SyncStatus = "Failed"
			subnet.Status.Message = fmt.Sprintf("Failed to create subnet: %v", err)
			meta.SetStatusCondition(&subnet.Status.Conditions, metav1.Condition{
				Type:    "Ready",
				Status:  metav1.ConditionFalse,
				Reason:  "CreateFailed",
				Message: fmt.Sprintf("VPC API error: %v", err),
			})
			if err := r.Status().Update(ctx, subnet); err != nil {
				logger.Error(err, "Failed to update status after create failure")
			}
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
		operatormetrics.ReconcileOpsTotal.WithLabelValues("vpcsubnet", "create", "success").Inc()
		r.emitEvent(subnet, "Normal", "Created", "Created VPC subnet %s", vpcSubnet.ID)
		logger.Info("Created VPC subnet", "subnetID", vpcSubnet.ID)
	}

	// Step 3: Reconcile flow log collector
	r.reconcileFlowLogs(ctx, subnet)

	// Step 4: Update status
	subnet.Status.SubnetID = vpcSubnet.ID
	subnet.Status.VPCSubnetStatus = vpcSubnet.Status
	subnet.Status.SyncStatus = "Synced"
	now := metav1.Now()
	subnet.Status.LastSyncTime = &now
	subnet.Status.Message = fmt.Sprintf("Synced with VPC subnet %s", vpcSubnet.ID)
	meta.SetStatusCondition(&subnet.Status.Conditions, metav1.Condition{
		Type:    "Ready",
		Status:  metav1.ConditionTrue,
		Reason:  "Synced",
		Message: fmt.Sprintf("VPC subnet %s is ready", vpcSubnet.ID),
	})

	if err := r.Status().Update(ctx, subnet); err != nil {
		logger.Error(err, "Failed to update status after create")
		return ctrl.Result{}, err
	}

	logger.Info("Successfully synced VPCSubnet", "subnetID", vpcSubnet.ID)
	return ctrl.Result{}, nil
}

// reconcileFlowLogs ensures the flow log collector matches the desired spec.
// Errors are logged but do not block subnet reconciliation.
func (r *Reconciler) reconcileFlowLogs(ctx context.Context, subnet *v1alpha1.VPCSubnet) {
	logger := log.FromContext(ctx)

	// If flow logs are not configured or not enabled, clean up any existing collector
	if subnet.Spec.FlowLogs == nil || !subnet.Spec.FlowLogs.Enabled {
		if subnet.Status.FlowLogCollectorID != "" {
			logger.Info("Flow logs disabled, deleting collector", "collectorID", subnet.Status.FlowLogCollectorID)
			if err := r.VPC.DeleteFlowLogCollector(ctx, subnet.Status.FlowLogCollectorID); err != nil {
				logger.Error(err, "Failed to delete flow log collector", "collectorID", subnet.Status.FlowLogCollectorID)
			} else {
				subnet.Status.FlowLogCollectorID = ""
				subnet.Status.FlowLogActive = false
			}
		}
		return
	}

	// Flow logs enabled — need a COS bucket CRN
	if subnet.Spec.FlowLogs.COSBucketCRN == "" {
		logger.Info("Flow logs enabled but no COSBucketCRN specified, skipping")
		return
	}

	// Check if collector already exists
	if subnet.Status.FlowLogCollectorID != "" {
		existing, err := r.VPC.GetFlowLogCollector(ctx, subnet.Status.FlowLogCollectorID)
		if err == nil {
			// Collector exists — update status
			subnet.Status.FlowLogActive = existing.IsActive
			return
		}
		// Collector not found or error — try to create a new one
		logger.Info("Existing flow log collector not found, will recreate", "collectorID", subnet.Status.FlowLogCollectorID, "error", err)
		subnet.Status.FlowLogCollectorID = ""
	}

	// Create the flow log collector
	collectorName := fmt.Sprintf("roks-%s-%s-flowlog", r.ClusterID, subnet.Name)
	collector, err := r.VPC.CreateFlowLogCollector(ctx, vpc.CreateFlowLogCollectorOptions{
		Name:           collectorName,
		TargetSubnetID: subnet.Status.SubnetID,
		COSBucketCRN:   subnet.Spec.FlowLogs.COSBucketCRN,
		IsActive:       true,
		ClusterID:      subnet.Spec.ClusterID,
		OwnerKind:      "vpcsubnet",
		OwnerName:      subnet.Name,
	})
	if err != nil {
		// Log and continue — flow log creation failure should not block subnet reconciliation
		logger.Error(err, "Failed to create flow log collector (stub — will be implemented with VPC SDK)")
		return
	}

	subnet.Status.FlowLogCollectorID = collector.ID
	subnet.Status.FlowLogActive = collector.IsActive
	logger.Info("Created flow log collector", "collectorID", collector.ID)
}

// reconcileDelete handles VPCSubnet deletion.
func (r *Reconciler) reconcileDelete(ctx context.Context, subnet *v1alpha1.VPCSubnet) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Step 0: Delete flow log collector if it exists
	if subnet.Status.FlowLogCollectorID != "" {
		if err := r.VPC.DeleteFlowLogCollector(ctx, subnet.Status.FlowLogCollectorID); err != nil {
			logger.Error(err, "Failed to delete flow log collector during subnet deletion", "collectorID", subnet.Status.FlowLogCollectorID)
			// Continue with subnet deletion — flow log collector cleanup is best-effort
		} else {
			logger.Info("Deleted flow log collector", "collectorID", subnet.Status.FlowLogCollectorID)
		}
	}

	// Step 1: Delete VPC subnet if it exists
	if subnet.Status.SubnetID != "" {
		if err := r.VPC.DeleteSubnet(ctx, subnet.Status.SubnetID); err != nil {
			logger.Error(err, "Failed to delete VPC subnet", "subnetID", subnet.Status.SubnetID)
			operatormetrics.ReconcileOpsTotal.WithLabelValues("vpcsubnet", "delete", "error").Inc()
			r.emitEvent(subnet, "Warning", "DeleteFailed", "Failed to delete VPC subnet %s: %v", subnet.Status.SubnetID, err)
			subnet.Status.SyncStatus = "Failed"
			subnet.Status.Message = fmt.Sprintf("Failed to delete subnet: %v", err)
			if err := r.Status().Update(ctx, subnet); err != nil {
				logger.Error(err, "Failed to update status after delete failure")
			}
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
		operatormetrics.ReconcileOpsTotal.WithLabelValues("vpcsubnet", "delete", "success").Inc()
		r.emitEvent(subnet, "Normal", "Deleted", "Deleted VPC subnet %s", subnet.Status.SubnetID)
		logger.Info("Deleted VPC subnet", "subnetID", subnet.Status.SubnetID)
	}

	// Step 2: Remove finalizer
	if err := finalizers.EnsureRemoved(ctx, r.Client, subnet, "vpc.roks.ibm.com/subnet-protection"); err != nil {
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

// SetupWithManager registers the VPCSubnet reconciler with the controller manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Recorder = mgr.GetEventRecorderFor("vpcsubnet-controller")
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.VPCSubnet{}).
		Complete(r)
}
