package router

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"reflect"
	"strings"
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
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

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

	// Step 4: Ensure router pod exists
	podReady, err := r.ensureRouterPod(ctx, router, gw)
	if err != nil {
		logger.Error(err, "Failed to ensure router pod")
		operatormetrics.ReconcileOpsTotal.WithLabelValues("vpcrouter", "ensure_pod", "error").Inc()
		r.emitEvent(router, "Warning", "PodFailed", "Failed to ensure router pod: %v", err)
		return r.setFailedStatus(ctx, router, fmt.Sprintf("Failed to ensure router pod: %v", err))
	}

	// Step 4b: Auto-resolve transit network from gateway status
	transitNetwork := ""
	if router.Spec.Transit != nil && router.Spec.Transit.Network != "" {
		transitNetwork = router.Spec.Transit.Network
	} else {
		transitNetwork = gw.Status.TransitNetwork
	}
	_ = transitNetwork // Used for informational purposes; could be stored in status later

	// Step 5: Build network statuses from spec.networks (including DHCP status)
	networkStatuses := make([]v1alpha1.RouterNetworkStatus, 0, len(router.Spec.Networks))
	anyDHCPEnabled := false
	for _, netSpec := range router.Spec.Networks {
		ns := v1alpha1.RouterNetworkStatus{
			Name:      netSpec.Name,
			Address:   netSpec.Address,
			Connected: true,
		}

		// Populate DHCP status for this network
		cfg := resolvedDHCPConfig(router.Spec.DHCP, netSpec)
		if cfg != nil {
			anyDHCPEnabled = true
			dhcpStatus := &v1alpha1.DHCPNetworkStatus{
				Enabled:          true,
				ReservationCount: int32(len(cfg.Reservations)),
			}
			if cfg.Range != nil {
				dhcpStatus.PoolStart = cfg.Range.Start
				dhcpStatus.PoolEnd = cfg.Range.End
			} else {
				// Auto-computed range
				_, ipNet, err := net.ParseCIDR(netSpec.Address)
				if err == nil {
					start := make(net.IP, len(ipNet.IP))
					copy(start, ipNet.IP)
					start[len(start)-1] += 10
					end := broadcastIP(ipNet)
					end[len(end)-1]--
					dhcpStatus.PoolStart = start.String()
					dhcpStatus.PoolEnd = end.String()
				}
			}
			ns.DHCP = dhcpStatus
		}

		networkStatuses = append(networkStatuses, ns)
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
	podCondStatus := metav1.ConditionTrue
	podCondReason := "PodRunning"
	podCondMessage := "Router pod is running"
	if !podReady {
		podCondStatus = metav1.ConditionFalse
		podCondReason = "PodNotReady"
		podCondMessage = "Router pod is not yet running"
	}
	meta.SetStatusCondition(&router.Status.Conditions, metav1.Condition{
		Type:               "PodReady",
		Status:             podCondStatus,
		Reason:             podCondReason,
		Message:            podCondMessage,
		LastTransitionTime: now,
	})

	// DHCP status condition
	if anyDHCPEnabled {
		dhcpCondStatus := metav1.ConditionTrue
		dhcpCondReason := "DHCPConfigured"
		dhcpCondMessage := "DHCP servers configured"
		if !podReady {
			dhcpCondStatus = metav1.ConditionFalse
			dhcpCondReason = "PodNotReady"
			dhcpCondMessage = "DHCP servers waiting for pod"
		}
		meta.SetStatusCondition(&router.Status.Conditions, metav1.Condition{
			Type:               "DHCPReady",
			Status:             dhcpCondStatus,
			Reason:             dhcpCondReason,
			Message:            dhcpCondMessage,
			LastTransitionTime: now,
		})
	} else {
		meta.RemoveStatusCondition(&router.Status.Conditions, "DHCPReady")
	}

	// IDS/IPS status
	if router.Spec.IDS != nil && router.Spec.IDS.Enabled {
		router.Status.IDSMode = router.Spec.IDS.Mode
		idsCondStatus := metav1.ConditionTrue
		idsCondReason := "SuricataConfigured"
		idsCondMessage := fmt.Sprintf("Suricata %s sidecar is configured", strings.ToUpper(router.Spec.IDS.Mode))
		if !podReady {
			idsCondStatus = metav1.ConditionFalse
			idsCondReason = "PodNotReady"
			idsCondMessage = fmt.Sprintf("Suricata %s sidecar waiting for pod", strings.ToUpper(router.Spec.IDS.Mode))
		}
		meta.SetStatusCondition(&router.Status.Conditions, metav1.Condition{
			Type:               "IDSReady",
			Status:             idsCondStatus,
			Reason:             idsCondReason,
			Message:            idsCondMessage,
			LastTransitionTime: now,
		})
	} else {
		router.Status.IDSMode = ""
		meta.RemoveStatusCondition(&router.Status.Conditions, "IDSReady")
	}

	// Metrics exporter status
	if router.Spec.Metrics != nil && router.Spec.Metrics.Enabled {
		router.Status.MetricsEnabled = true
		metricsCondStatus := metav1.ConditionTrue
		metricsCondReason := "MetricsConfigured"
		metricsCondMessage := "Metrics exporter sidecar is configured"
		if !podReady {
			metricsCondStatus = metav1.ConditionFalse
			metricsCondReason = "PodNotReady"
			metricsCondMessage = "Metrics exporter sidecar waiting for pod"
		}
		meta.SetStatusCondition(&router.Status.Conditions, metav1.Condition{
			Type:               "MetricsReady",
			Status:             metricsCondStatus,
			Reason:             metricsCondReason,
			Message:            metricsCondMessage,
			LastTransitionTime: now,
		})
	} else {
		router.Status.MetricsEnabled = false
		meta.RemoveStatusCondition(&router.Status.Conditions, "MetricsReady")
	}

	if err := r.Status().Update(ctx, router); err != nil {
		logger.Error(err, "Failed to update VPCRouter status")
		return ctrl.Result{}, err
	}

	operatormetrics.ReconcileOpsTotal.WithLabelValues("vpcrouter", "reconcile", "success").Inc()
	r.emitEvent(router, "Normal", "Synced", "Router %s is ready with %d networks",
		router.Name, len(router.Spec.Networks))

	logger.Info("Successfully reconciled VPCRouter",
		"name", router.Name, "phase", router.Status.Phase,
		"networks", len(router.Spec.Networks), "podReady", podReady)

	// Requeue to check pod readiness if not yet running
	if !podReady {
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}
	return ctrl.Result{}, nil
}

// ensureRouterPod creates or validates the router pod.
// Returns true if the pod is Running, false otherwise.
func (r *Reconciler) ensureRouterPod(ctx context.Context, router *v1alpha1.VPCRouter, gw *v1alpha1.VPCGateway) (bool, error) {
	logger := log.FromContext(ctx)
	podName := routerPodName(router)

	// Check if pod already exists
	existing := &corev1.Pod{}
	err := r.Get(ctx, types.NamespacedName{Name: podName, Namespace: router.Namespace}, existing)
	if err == nil {
		// Pod exists — check if it needs recreation
		if r.podNeedsRecreation(existing, router, gw) {
			logger.Info("Router pod spec changed, recreating", "pod", podName)
			if delErr := r.Delete(ctx, existing); delErr != nil && !errors.IsNotFound(delErr) {
				return false, fmt.Errorf("delete stale pod %s: %w", podName, delErr)
			}
			r.emitEvent(router, "Normal", "PodDeleted", "Deleted stale router pod %s for recreation", podName)
			// Fall through to create
		} else {
			// Pod exists and is current — update PodIP in status
			router.Status.PodIP = existing.Status.PodIP
			// Check if it's running
			isRunning := existing.Status.Phase == corev1.PodRunning
			return isRunning, nil
		}
	} else if !errors.IsNotFound(err) {
		return false, fmt.Errorf("get pod %s: %w", podName, err)
	}

	// Create the pod
	pod := buildRouterPod(router, gw)
	if err := r.Create(ctx, pod); err != nil {
		if errors.IsAlreadyExists(err) {
			return false, nil // race — will reconcile again
		}
		return false, fmt.Errorf("create pod %s: %w", podName, err)
	}

	logger.Info("Created router pod", "pod", podName)
	operatormetrics.ReconcileOpsTotal.WithLabelValues("vpcrouter", "create_pod", "success").Inc()
	r.emitEvent(router, "Normal", "PodCreated", "Created router pod %s", podName)
	return false, nil // not running yet — just created
}

// podNeedsRecreation checks if the existing pod's spec differs from what
// would be generated for the current router/gateway spec. Compares Multus
// annotations, container count, images, and env vars for all containers.
func (r *Reconciler) podNeedsRecreation(existing *corev1.Pod, router *v1alpha1.VPCRouter, gw *v1alpha1.VPCGateway) bool {
	desired := buildRouterPod(router, gw)

	// Compare Multus annotations
	existingMultus := existing.Annotations["k8s.v1.cni.cncf.io/networks"]
	desiredMultus := desired.Annotations["k8s.v1.cni.cncf.io/networks"]
	if existingMultus != desiredMultus {
		return true
	}

	// Compare container count (sidecar added/removed)
	if len(existing.Spec.Containers) != len(desired.Spec.Containers) {
		return true
	}

	// Compare each container's image and env vars
	for i := range desired.Spec.Containers {
		if existing.Spec.Containers[i].Image != desired.Spec.Containers[i].Image {
			return true
		}
		existingEnv := envToMap(existing.Spec.Containers[i].Env)
		desiredEnv := envToMap(desired.Spec.Containers[i].Env)
		existingJSON, _ := json.Marshal(existingEnv)
		desiredJSON, _ := json.Marshal(desiredEnv)
		if string(existingJSON) != string(desiredJSON) {
			return true
		}
	}

	// Compare router container resources
	if !reflect.DeepEqual(existing.Spec.Containers[0].Resources, desired.Spec.Containers[0].Resources) {
		return true
	}

	// Compare pod-level scheduling fields
	if !reflect.DeepEqual(existing.Spec.NodeSelector, desired.Spec.NodeSelector) {
		return true
	}
	if !reflect.DeepEqual(existing.Spec.Tolerations, desired.Spec.Tolerations) {
		return true
	}
	if !equalStringPtr(existing.Spec.RuntimeClassName, desired.Spec.RuntimeClassName) {
		return true
	}
	if existing.Spec.PriorityClassName != desired.Spec.PriorityClassName {
		return true
	}

	return false
}

// equalStringPtr compares two *string values for equality.
func equalStringPtr(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

// envToMap converts a slice of EnvVar to a map for comparison.
func envToMap(envs []corev1.EnvVar) map[string]string {
	m := make(map[string]string, len(envs))
	for _, e := range envs {
		m[e.Name] = e.Value
	}
	return m
}

// reconcileDelete handles VPCRouter deletion by cleaning up the pod and removing the finalizer.
func (r *Reconciler) reconcileDelete(ctx context.Context, router *v1alpha1.VPCRouter) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling VPCRouter deletion", "name", router.Name)

	// Explicitly delete the router pod for faster cleanup (OwnerReference would
	// eventually GC it, but explicit deletion is faster).
	podName := routerPodName(router)
	pod := &corev1.Pod{}
	if err := r.Get(ctx, types.NamespacedName{Name: podName, Namespace: router.Namespace}, pod); err == nil {
		if err := r.Delete(ctx, pod); err != nil && !errors.IsNotFound(err) {
			logger.Error(err, "Failed to delete router pod", "pod", podName)
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}
		logger.Info("Deleted router pod", "pod", podName)
		r.emitEvent(router, "Normal", "PodDeleted", "Deleted router pod %s", podName)
	}

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
// Watches VPCGateway changes so that router pods are recreated when a gateway's
// NAT rules, firewall, image, or MAC address change.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Recorder = mgr.GetEventRecorderFor("vpcrouter-controller")
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.VPCRouter{}).
		Owns(&corev1.Pod{}).
		Watches(&v1alpha1.VPCGateway{}, handler.EnqueueRequestsFromMapFunc(
			func(ctx context.Context, obj client.Object) []reconcile.Request {
				gw, ok := obj.(*v1alpha1.VPCGateway)
				if !ok {
					return nil
				}
				var routers v1alpha1.VPCRouterList
				if err := mgr.GetClient().List(ctx, &routers, client.InNamespace(gw.Namespace)); err != nil {
					return nil
				}
				var reqs []reconcile.Request
				for _, rt := range routers.Items {
					if rt.Spec.Gateway == gw.Name {
						reqs = append(reqs, reconcile.Request{
							NamespacedName: types.NamespacedName{
								Name:      rt.Name,
								Namespace: rt.Namespace,
							},
						})
					}
				}
				return reqs
			},
		)).
		Complete(r)
}
