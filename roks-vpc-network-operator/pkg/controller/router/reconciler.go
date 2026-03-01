package router

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

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

// Reconciler reconciles VPCRouter objects.
type Reconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// Reconcile handles VPCRouter create/update/delete events.
//
// The VPCRouter is a pure K8s resource that does not call VPC APIs.
// It references a VPCGateway, resolves the transit network, builds
// network statuses and advertised routes, and sets status conditions.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling VPCRouter", "name", req.Name, "namespace", req.Namespace)

	// Fetch the VPCRouter object
	router := &v1alpha1.VPCRouter{}
	if err := r.Get(ctx, req.NamespacedName, router); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to fetch VPCRouter")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !router.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, router)
	}

	// Handle creation/update
	return r.reconcileNormal(ctx, router)
}

// reconcileNormal handles VPCRouter creation and updates.
func (r *Reconciler) reconcileNormal(ctx context.Context, router *v1alpha1.VPCRouter) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Step 1: Ensure finalizer is added
	if err := finalizers.EnsureAdded(ctx, r.Client, router, finalizers.RouterCleanup); err != nil {
		logger.Error(err, "Failed to add finalizer")
		return ctrl.Result{}, err
	}

	// Step 2: Lookup VPCGateway by spec.gateway (same namespace)
	gw := &v1alpha1.VPCGateway{}
	gwKey := types.NamespacedName{Name: router.Spec.Gateway, Namespace: router.Namespace}
	if err := r.Get(ctx, gwKey, gw); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Referenced VPCGateway not found, requeueing", "gateway", router.Spec.Gateway)
			r.emitEvent(router, "Warning", "GatewayNotFound",
				"Referenced VPCGateway %q not found", router.Spec.Gateway)
			operatormetrics.ReconcileOpsTotal.WithLabelValues("vpcrouter", "reconcile", "error").Inc()
			return r.setFailedStatus(ctx, router,
				fmt.Sprintf("VPCGateway %q not found", router.Spec.Gateway))
		}
		logger.Error(err, "Failed to get VPCGateway")
		return ctrl.Result{}, err
	}

	// Step 3: Check if gateway is Ready
	if gw.Status.Phase != "Ready" {
		logger.Info("Referenced VPCGateway is not Ready, requeueing",
			"gateway", router.Spec.Gateway, "phase", gw.Status.Phase)
		r.emitEvent(router, "Warning", "GatewayNotReady",
			"Referenced VPCGateway %q is not Ready (phase: %s)", router.Spec.Gateway, gw.Status.Phase)
		operatormetrics.ReconcileOpsTotal.WithLabelValues("vpcrouter", "reconcile", "error").Inc()
		return r.setFailedStatus(ctx, router,
			fmt.Sprintf("VPCGateway %q is not Ready (phase: %s)", router.Spec.Gateway, gw.Status.Phase))
	}

	// Step 4: Auto-resolve transit network from gateway status
	transitNetwork := ""
	if router.Spec.Transit != nil && router.Spec.Transit.Network != "" {
		transitNetwork = router.Spec.Transit.Network
	} else {
		transitNetwork = gw.Status.TransitNetwork
	}
	_ = transitNetwork // Used for informational purposes; could be stored in status later

	// Step 5: Build network statuses from spec.networks
	networkStatuses := make([]v1alpha1.RouterNetworkStatus, 0, len(router.Spec.Networks))
	for _, net := range router.Spec.Networks {
		networkStatuses = append(networkStatuses, v1alpha1.RouterNetworkStatus{
			Name:      net.Name,
			Address:   net.Address,
			Connected: true,
		})
	}

	// Step 6: Build advertisedRoutes from spec.networks when route advertisement is enabled
	var advertisedRoutes []string
	if router.Spec.RouteAdvertisement != nil && router.Spec.RouteAdvertisement.ConnectedSegments {
		for _, n := range router.Spec.Networks {
			cidr := networkCIDR(n.Address)
			advertisedRoutes = append(advertisedRoutes, cidr)
		}
	}

	// Step 7: Extract transit IP (strip prefix length)
	transitIP := ""
	if router.Spec.Transit != nil {
		transitIP = stripPrefix(router.Spec.Transit.Address)
	}

	// Step 8: Update status
	now := metav1.Now()
	router.Status.Phase = "Ready"
	router.Status.TransitIP = transitIP
	router.Status.Networks = networkStatuses
	router.Status.AdvertisedRoutes = advertisedRoutes
	router.Status.SyncStatus = "Synced"
	router.Status.LastSyncTime = &now
	router.Status.Message = fmt.Sprintf("Router connected to gateway %q with %d networks",
		router.Spec.Gateway, len(router.Spec.Networks))

	// Step 9: Set conditions
	meta.SetStatusCondition(&router.Status.Conditions, metav1.Condition{
		Type:               "TransitConnected",
		Status:             metav1.ConditionTrue,
		Reason:             "GatewayReady",
		Message:            fmt.Sprintf("Connected to transit network via gateway %q", router.Spec.Gateway),
		LastTransitionTime: now,
	})
	meta.SetStatusCondition(&router.Status.Conditions, metav1.Condition{
		Type:               "RoutesConfigured",
		Status:             metav1.ConditionTrue,
		Reason:             "RoutesAdvertised",
		Message:            fmt.Sprintf("Advertising %d routes", len(advertisedRoutes)),
		LastTransitionTime: now,
	})
	meta.SetStatusCondition(&router.Status.Conditions, metav1.Condition{
		Type:               "PodReady",
		Status:             metav1.ConditionTrue,
		Reason:             "RouterConfigured",
		Message:            "Router pod configuration is ready",
		LastTransitionTime: now,
	})

	if err := r.Status().Update(ctx, router); err != nil {
		logger.Error(err, "Failed to update VPCRouter status")
		return ctrl.Result{}, err
	}

	operatormetrics.ReconcileOpsTotal.WithLabelValues("vpcrouter", "reconcile", "success").Inc()
	r.emitEvent(router, "Normal", "Synced", "Router %s is ready with %d networks",
		router.Name, len(router.Spec.Networks))

	logger.Info("Successfully reconciled VPCRouter",
		"name", router.Name, "phase", router.Status.Phase,
		"networks", len(router.Spec.Networks))
	return ctrl.Result{}, nil
}

// reconcileDelete handles VPCRouter deletion by removing the finalizer.
func (r *Reconciler) reconcileDelete(ctx context.Context, router *v1alpha1.VPCRouter) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling VPCRouter deletion", "name", router.Name)

	if err := finalizers.EnsureRemoved(ctx, r.Client, router, finalizers.RouterCleanup); err != nil {
		logger.Error(err, "Failed to remove finalizer")
		return ctrl.Result{}, err
	}

	operatormetrics.ReconcileOpsTotal.WithLabelValues("vpcrouter", "delete", "success").Inc()
	r.emitEvent(router, "Normal", "Deleted", "VPCRouter %s finalizer removed", router.Name)

	return ctrl.Result{}, nil
}

// setFailedStatus sets the router status to Error/Failed and requeues after 30 seconds.
func (r *Reconciler) setFailedStatus(ctx context.Context, router *v1alpha1.VPCRouter, message string) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	router.Status.Phase = "Error"
	router.Status.SyncStatus = "Failed"
	router.Status.Message = message

	if err := r.Status().Update(ctx, router); err != nil {
		logger.Error(err, "Failed to update status after failure")
	}

	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

func (r *Reconciler) emitEvent(obj runtime.Object, eventType, reason, messageFmt string, args ...interface{}) {
	if r.Recorder != nil {
		r.Recorder.Eventf(obj, eventType, reason, messageFmt, args...)
	}
}

// networkCIDR converts an address with prefix (e.g., "10.100.0.1/24") to
// the network CIDR (e.g., "10.100.0.0/24") by zeroing the host bits.
func networkCIDR(address string) string {
	ip, ipNet, err := net.ParseCIDR(address)
	if err != nil {
		// Fallback: if parsing fails, return the address as-is
		return address
	}
	_ = ip
	return ipNet.String()
}

// stripPrefix removes the prefix length from an IP address string.
// For example, "172.16.0.2/24" becomes "172.16.0.2".
func stripPrefix(address string) string {
	if idx := strings.IndexByte(address, '/'); idx != -1 {
		return address[:idx]
	}
	return address
}

// SetupWithManager registers the VPCRouter reconciler with the controller manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Recorder = mgr.GetEventRecorderFor("vpcrouter-controller")
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.VPCRouter{}).
		Complete(r)
}
