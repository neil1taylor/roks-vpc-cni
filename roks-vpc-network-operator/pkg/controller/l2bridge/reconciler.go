package l2bridge

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	v1alpha1 "github.com/IBM/roks-vpc-network-operator/api/v1alpha1"
	"github.com/IBM/roks-vpc-network-operator/pkg/finalizers"
	operatormetrics "github.com/IBM/roks-vpc-network-operator/pkg/metrics"
)

// Reconciler reconciles VPCL2Bridge objects.
type Reconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// Reconcile handles VPCL2Bridge create/update/delete events.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling VPCL2Bridge", "name", req.Name, "namespace", req.Namespace)

	// Fetch the VPCL2Bridge object
	bridge := &v1alpha1.VPCL2Bridge{}
	if err := r.Get(ctx, req.NamespacedName, bridge); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to fetch VPCL2Bridge")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !bridge.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, bridge)
	}

	// Handle creation/update
	return r.reconcileNormal(ctx, bridge)
}

// reconcileNormal handles VPCL2Bridge creation and updates.
func (r *Reconciler) reconcileNormal(ctx context.Context, bridge *v1alpha1.VPCL2Bridge) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Step 1: Ensure finalizer is added
	if err := finalizers.EnsureAdded(ctx, r.Client, bridge, finalizers.L2BridgeCleanup); err != nil {
		logger.Error(err, "Failed to add finalizer")
		return ctrl.Result{}, err
	}

	// Step 2: Lookup VPCGateway by spec.gatewayRef (same namespace)
	gw := &v1alpha1.VPCGateway{}
	gwKey := types.NamespacedName{Name: bridge.Spec.GatewayRef, Namespace: bridge.Namespace}
	if err := r.Get(ctx, gwKey, gw); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Referenced VPCGateway not found, requeueing", "gateway", bridge.Spec.GatewayRef)
			r.emitEvent(bridge, "Warning", "GatewayNotFound",
				"Referenced VPCGateway %q not found", bridge.Spec.GatewayRef)
			operatormetrics.ReconcileOpsTotal.WithLabelValues("vpcl2bridge", "reconcile", "error").Inc()
			return r.setPendingStatus(ctx, bridge,
				fmt.Sprintf("VPCGateway %q not found", bridge.Spec.GatewayRef))
		}
		logger.Error(err, "Failed to get VPCGateway")
		return ctrl.Result{}, err
	}

	// Step 3: Check if gateway is Ready
	if gw.Status.Phase != "Ready" {
		logger.Info("Referenced VPCGateway is not Ready, requeueing",
			"gateway", bridge.Spec.GatewayRef, "phase", gw.Status.Phase)
		r.emitEvent(bridge, "Warning", "GatewayNotReady",
			"Referenced VPCGateway %q is not Ready (phase: %s)", bridge.Spec.GatewayRef, gw.Status.Phase)
		operatormetrics.ReconcileOpsTotal.WithLabelValues("vpcl2bridge", "reconcile", "error").Inc()
		return r.setPendingStatus(ctx, bridge,
			fmt.Sprintf("Waiting for gateway %q to become Ready (current phase: %s)", bridge.Spec.GatewayRef, gw.Status.Phase))
	}

	// Step 4: Validate type-specific configuration
	if err := r.validateConfig(bridge); err != nil {
		logger.Info("Invalid bridge configuration", "error", err.Error())
		r.emitEvent(bridge, "Warning", "InvalidConfig", "Invalid configuration: %v", err)
		operatormetrics.ReconcileOpsTotal.WithLabelValues("vpcl2bridge", "reconcile", "error").Inc()
		return r.setErrorStatus(ctx, bridge, err.Error())
	}

	// Step 5: Build the bridge pod based on type
	var pod *corev1.Pod
	switch bridge.Spec.Type {
	case "gretap-wireguard":
		pod = buildGRETAPPod(bridge, gw)
	case "l2vpn":
		pod = buildL2VPNPod(bridge, gw)
	case "evpn-vxlan":
		pod = buildEVPNPod(bridge, gw)
	default:
		return r.setErrorStatus(ctx, bridge, fmt.Sprintf("unsupported bridge type %q", bridge.Spec.Type))
	}

	// Step 6: Ensure pod exists
	podName := bridgePodName(bridge.Name)
	existing := &corev1.Pod{}
	err := r.Get(ctx, types.NamespacedName{Name: podName, Namespace: bridge.Namespace}, existing)
	if err != nil && !errors.IsNotFound(err) {
		logger.Error(err, "Failed to get bridge pod", "pod", podName)
		return ctrl.Result{}, err
	}

	podReady := false
	if errors.IsNotFound(err) {
		// Pod does not exist — create it
		if createErr := r.Create(ctx, pod); createErr != nil {
			if errors.IsAlreadyExists(createErr) {
				// Race condition — will reconcile again
				return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
			}
			logger.Error(createErr, "Failed to create bridge pod", "pod", podName)
			operatormetrics.ReconcileOpsTotal.WithLabelValues("vpcl2bridge", "create_pod", "error").Inc()
			r.emitEvent(bridge, "Warning", "PodFailed", "Failed to create bridge pod: %v", createErr)
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
		logger.Info("Created bridge pod", "pod", podName)
		operatormetrics.ReconcileOpsTotal.WithLabelValues("vpcl2bridge", "create_pod", "success").Inc()
		r.emitEvent(bridge, "Normal", "PodCreated", "Created bridge pod %s", podName)
	} else {
		// Pod exists — check readiness
		podReady = existing.Status.Phase == corev1.PodRunning
	}

	// Step 7: Update status
	now := metav1.Now()
	if podReady {
		bridge.Status.Phase = "Established"
	} else {
		bridge.Status.Phase = "Provisioning"
	}
	bridge.Status.PodName = podName
	bridge.Status.SyncStatus = "Synced"
	bridge.Status.LastSyncTime = &now

	if podReady {
		bridge.Status.Message = fmt.Sprintf("Bridge %s is established via %s", bridge.Name, bridge.Spec.Type)
	} else {
		bridge.Status.Message = fmt.Sprintf("Bridge pod %s is starting", podName)
	}

	// Set conditions
	podCondStatus := metav1.ConditionTrue
	podCondReason := "PodRunning"
	podCondMessage := "Bridge pod is running"
	if !podReady {
		podCondStatus = metav1.ConditionFalse
		podCondReason = "PodNotReady"
		podCondMessage = "Bridge pod is not yet running"
	}
	meta.SetStatusCondition(&bridge.Status.Conditions, metav1.Condition{
		Type:               "PodReady",
		Status:             podCondStatus,
		Reason:             podCondReason,
		Message:            podCondMessage,
		LastTransitionTime: now,
	})
	meta.SetStatusCondition(&bridge.Status.Conditions, metav1.Condition{
		Type:               "GatewayConnected",
		Status:             metav1.ConditionTrue,
		Reason:             "GatewayReady",
		Message:            fmt.Sprintf("Connected to gateway %q", bridge.Spec.GatewayRef),
		LastTransitionTime: now,
	})

	if err := r.Status().Update(ctx, bridge); err != nil {
		logger.Error(err, "Failed to update VPCL2Bridge status")
		return ctrl.Result{}, err
	}

	operatormetrics.ReconcileOpsTotal.WithLabelValues("vpcl2bridge", "reconcile", "success").Inc()
	r.emitEvent(bridge, "Normal", "Synced", "Bridge %s is %s", bridge.Name, bridge.Status.Phase)

	logger.Info("Successfully reconciled VPCL2Bridge",
		"name", bridge.Name, "phase", bridge.Status.Phase, "podReady", podReady)

	// Requeue to check pod readiness if not yet running
	if !podReady {
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}
	// Requeue after 5 minutes for drift detection
	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

// validateConfig validates type-specific configuration on the bridge spec.
func (r *Reconciler) validateConfig(bridge *v1alpha1.VPCL2Bridge) error {
	switch bridge.Spec.Type {
	case "gretap-wireguard":
		if bridge.Spec.Remote.WireGuard == nil {
			return fmt.Errorf("bridge type %q requires spec.remote.wireGuard configuration", bridge.Spec.Type)
		}
	case "l2vpn":
		if bridge.Spec.Remote.L2VPN == nil {
			return fmt.Errorf("bridge type %q requires spec.remote.l2vpn configuration", bridge.Spec.Type)
		}
	case "evpn-vxlan":
		if bridge.Spec.Remote.EVPN == nil {
			return fmt.Errorf("bridge type %q requires spec.remote.evpn configuration", bridge.Spec.Type)
		}
	}
	return nil
}

// reconcileDelete handles VPCL2Bridge deletion by cleaning up the pod and removing the finalizer.
func (r *Reconciler) reconcileDelete(ctx context.Context, bridge *v1alpha1.VPCL2Bridge) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling VPCL2Bridge deletion", "name", bridge.Name)

	// Delete the bridge pod
	podName := bridgePodName(bridge.Name)
	pod := &corev1.Pod{}
	if err := r.Get(ctx, types.NamespacedName{Name: podName, Namespace: bridge.Namespace}, pod); err == nil {
		if err := r.Delete(ctx, pod); err != nil && !errors.IsNotFound(err) {
			logger.Error(err, "Failed to delete bridge pod", "pod", podName)
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}
		logger.Info("Deleted bridge pod", "pod", podName)
		r.emitEvent(bridge, "Normal", "PodDeleted", "Deleted bridge pod %s", podName)
	}

	// Remove finalizer
	if err := finalizers.EnsureRemoved(ctx, r.Client, bridge, finalizers.L2BridgeCleanup); err != nil {
		logger.Error(err, "Failed to remove finalizer")
		return ctrl.Result{}, err
	}

	operatormetrics.ReconcileOpsTotal.WithLabelValues("vpcl2bridge", "delete", "success").Inc()
	r.emitEvent(bridge, "Normal", "Deleted", "VPCL2Bridge %s finalizer removed", bridge.Name)

	return ctrl.Result{}, nil
}

// setPendingStatus sets the bridge status to Pending and requeues after 30 seconds.
func (r *Reconciler) setPendingStatus(ctx context.Context, bridge *v1alpha1.VPCL2Bridge, message string) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	bridge.Status.Phase = "Pending"
	bridge.Status.SyncStatus = "Pending"
	bridge.Status.Message = message

	if err := r.Status().Update(ctx, bridge); err != nil {
		logger.Error(err, "Failed to update status to Pending")
	}

	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// setErrorStatus sets the bridge status to Error. Does not requeue — the user
// must fix the spec, which will trigger a new reconcile via the watch.
func (r *Reconciler) setErrorStatus(ctx context.Context, bridge *v1alpha1.VPCL2Bridge, message string) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	bridge.Status.Phase = "Error"
	bridge.Status.SyncStatus = "Failed"
	bridge.Status.Message = message

	if err := r.Status().Update(ctx, bridge); err != nil {
		logger.Error(err, "Failed to update status to Error")
	}

	return ctrl.Result{}, nil
}

func (r *Reconciler) emitEvent(obj runtime.Object, eventType, reason, messageFmt string, args ...interface{}) {
	if r.Recorder != nil {
		r.Recorder.Eventf(obj, eventType, reason, messageFmt, args...)
	}
}

// SetupWithManager registers the VPCL2Bridge reconciler with the controller manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Recorder = mgr.GetEventRecorderFor("vpcl2bridge-controller")
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.VPCL2Bridge{}).
		Owns(&corev1.Pod{}).
		Complete(r)
}
