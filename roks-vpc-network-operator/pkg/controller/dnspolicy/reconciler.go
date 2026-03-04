package dnspolicy

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	v1alpha1 "github.com/IBM/roks-vpc-network-operator/api/v1alpha1"
)

const finalizerName = "vpc.roks.ibm.com/dnspolicy-cleanup"

// Reconciler reconciles a VPCDNSPolicy object.
type Reconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// configMapName returns the name of the AdGuard Home ConfigMap for a given policy.
func configMapName(policyName string) string {
	return policyName + "-adguard-config"
}

// Reconcile handles the reconciliation loop for VPCDNSPolicy resources.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling VPCDNSPolicy", "name", req.Name)

	policy := &v1alpha1.VPCDNSPolicy{}
	if err := r.Get(ctx, req.NamespacedName, policy); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !policy.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, policy)
	}

	// Ensure finalizer
	if !controllerutil.ContainsFinalizer(policy, finalizerName) {
		controllerutil.AddFinalizer(policy, finalizerName)
		if err := r.Update(ctx, policy); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Validate router ref exists
	router := &v1alpha1.VPCRouter{}
	if err := r.Get(ctx, types.NamespacedName{Name: policy.Spec.RouterRef, Namespace: policy.Namespace}, router); err != nil {
		if errors.IsNotFound(err) {
			policy.Status.Phase = "Error"
			policy.Status.SyncStatus = "Failed"
			policy.Status.Message = fmt.Sprintf("Router %q not found", policy.Spec.RouterRef)
			meta.SetStatusCondition(&policy.Status.Conditions, metav1.Condition{
				Type:    "Ready",
				Status:  metav1.ConditionFalse,
				Reason:  "RouterNotFound",
				Message: policy.Status.Message,
			})
			_ = r.Status().Update(ctx, policy)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Generate AdGuard Home config
	adguardYAML := generateAdGuardConfig(&policy.Spec)

	// Create or update ConfigMap
	cmName := configMapName(policy.Name)
	cm := &corev1.ConfigMap{}
	err := r.Get(ctx, types.NamespacedName{Name: cmName, Namespace: policy.Namespace}, cm)
	if errors.IsNotFound(err) {
		cm = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cmName,
				Namespace: policy.Namespace,
				Labels: map[string]string{
					"app.kubernetes.io/managed-by": "roks-vpc-network-operator",
					"vpc.roks.ibm.com/dnspolicy":   policy.Name,
				},
			},
			Data: map[string]string{"AdGuardHome.yaml": adguardYAML},
		}
		if err := controllerutil.SetControllerReference(policy, cm, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}
		if err := r.Create(ctx, cm); err != nil {
			return ctrl.Result{}, err
		}
	} else if err != nil {
		return ctrl.Result{}, err
	} else {
		cm.Data["AdGuardHome.yaml"] = adguardYAML
		if err := r.Update(ctx, cm); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Update status
	now := metav1.Now()
	policy.Status.Phase = "Active"
	policy.Status.SyncStatus = "Synced"
	policy.Status.ConfigMapName = cmName
	policy.Status.Message = fmt.Sprintf("DNS policy active for router %q", policy.Spec.RouterRef)
	meta.SetStatusCondition(&policy.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		Reason:             "ConfigApplied",
		Message:            "AdGuard Home config generated",
		LastTransitionTime: now,
	})
	if err := r.Status().Update(ctx, policy); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// reconcileDelete handles cleanup when a VPCDNSPolicy is being deleted.
func (r *Reconciler) reconcileDelete(ctx context.Context, policy *v1alpha1.VPCDNSPolicy) (ctrl.Result, error) {
	// Delete the ConfigMap if it exists
	cmName := configMapName(policy.Name)
	cm := &corev1.ConfigMap{}
	if err := r.Get(ctx, types.NamespacedName{Name: cmName, Namespace: policy.Namespace}, cm); err == nil {
		if err := r.Delete(ctx, cm); err != nil && !errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
	}

	// Remove finalizer
	controllerutil.RemoveFinalizer(policy, finalizerName)
	if err := r.Update(ctx, policy); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Recorder = mgr.GetEventRecorderFor("vpcdnspolicy-controller")
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.VPCDNSPolicy{}).
		Owns(&corev1.ConfigMap{}).
		Complete(r)
}
