package vpngateway

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
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1alpha1 "github.com/IBM/roks-vpc-network-operator/api/v1alpha1"
	"github.com/IBM/roks-vpc-network-operator/pkg/finalizers"
	operatormetrics "github.com/IBM/roks-vpc-network-operator/pkg/metrics"
)

// Reconciler reconciles VPCVPNGateway objects.
type Reconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// Reconcile handles VPCVPNGateway create/update/delete events.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling VPCVPNGateway", "name", req.Name, "namespace", req.Namespace)

	// Fetch the VPCVPNGateway object
	vpn := &v1alpha1.VPCVPNGateway{}
	if err := r.Get(ctx, req.NamespacedName, vpn); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to fetch VPCVPNGateway")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !vpn.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, vpn)
	}

	// Handle creation/update
	return r.reconcileNormal(ctx, vpn)
}

// reconcileNormal handles VPCVPNGateway creation and updates.
func (r *Reconciler) reconcileNormal(ctx context.Context, vpn *v1alpha1.VPCVPNGateway) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Step 1: Ensure finalizer is added
	if err := finalizers.EnsureAdded(ctx, r.Client, vpn, finalizers.VPNGatewayCleanup); err != nil {
		logger.Error(err, "Failed to add finalizer")
		return ctrl.Result{}, err
	}

	// Step 2: Lookup VPCGateway by spec.gatewayRef (same namespace)
	gw := &v1alpha1.VPCGateway{}
	gwKey := types.NamespacedName{Name: vpn.Spec.GatewayRef, Namespace: vpn.Namespace}
	if err := r.Get(ctx, gwKey, gw); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Referenced VPCGateway not found, requeueing", "gateway", vpn.Spec.GatewayRef)
			r.emitEvent(vpn, "Warning", "GatewayNotFound",
				"Referenced VPCGateway %q not found", vpn.Spec.GatewayRef)
			operatormetrics.ReconcileOpsTotal.WithLabelValues("vpcvpngateway", "reconcile", "error").Inc()
			return r.setPendingStatus(ctx, vpn,
				fmt.Sprintf("VPCGateway %q not found", vpn.Spec.GatewayRef))
		}
		logger.Error(err, "Failed to get VPCGateway")
		return ctrl.Result{}, err
	}

	// Step 3: Check if gateway is Ready
	if gw.Status.Phase != "Ready" {
		logger.Info("Referenced VPCGateway is not Ready, requeueing",
			"gateway", vpn.Spec.GatewayRef, "phase", gw.Status.Phase)
		r.emitEvent(vpn, "Warning", "GatewayNotReady",
			"Referenced VPCGateway %q is not Ready (phase: %s)", vpn.Spec.GatewayRef, gw.Status.Phase)
		operatormetrics.ReconcileOpsTotal.WithLabelValues("vpcvpngateway", "reconcile", "error").Inc()
		return r.setPendingStatus(ctx, vpn,
			fmt.Sprintf("Waiting for gateway %q to become Ready (current phase: %s)", vpn.Spec.GatewayRef, gw.Status.Phase))
	}

	// Step 4: Validate protocol-specific configuration
	if err := r.validateConfig(vpn); err != nil {
		logger.Info("Invalid VPN configuration", "error", err.Error())
		r.emitEvent(vpn, "Warning", "InvalidConfig", "Invalid configuration: %v", err)
		operatormetrics.ReconcileOpsTotal.WithLabelValues("vpcvpngateway", "reconcile", "error").Inc()
		return r.setErrorStatus(ctx, vpn, err.Error())
	}

	// Step 5: Ensure CRL secret exists for OpenVPN gateways
	if vpn.Spec.Protocol == "openvpn" {
		if err := r.ensureCRLSecret(ctx, vpn); err != nil {
			logger.Error(err, "Failed to ensure CRL secret")
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}
	}

	// Step 6: Build the VPN pod based on protocol
	var desiredPod *corev1.Pod
	switch vpn.Spec.Protocol {
	case "wireguard":
		desiredPod = buildWireGuardPod(vpn, gw)
	case "ipsec":
		desiredPod = buildStrongSwanPod(vpn, gw)
	case "openvpn":
		desiredPod = buildOpenVPNPod(vpn, gw)
	default:
		return r.setErrorStatus(ctx, vpn, fmt.Sprintf("unsupported protocol %q", vpn.Spec.Protocol))
	}

	// Step 7: Ensure pod exists
	podName := vpnPodName(vpn.Name)
	existing := &corev1.Pod{}
	err := r.Get(ctx, types.NamespacedName{Name: podName, Namespace: vpn.Namespace}, existing)
	if err != nil && !errors.IsNotFound(err) {
		logger.Error(err, "Failed to get VPN pod", "pod", podName)
		return ctrl.Result{}, err
	}

	podReady := false
	if errors.IsNotFound(err) {
		// Pod does not exist — create it
		if createErr := r.Create(ctx, desiredPod); createErr != nil {
			if errors.IsAlreadyExists(createErr) {
				return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
			}
			logger.Error(createErr, "Failed to create VPN pod", "pod", podName)
			operatormetrics.ReconcileOpsTotal.WithLabelValues("vpcvpngateway", "create_pod", "error").Inc()
			r.emitEvent(vpn, "Warning", "PodFailed", "Failed to create VPN pod: %v", createErr)
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
		logger.Info("Created VPN pod", "pod", podName)
		operatormetrics.ReconcileOpsTotal.WithLabelValues("vpcvpngateway", "create_pod", "success").Inc()
		r.emitEvent(vpn, "Normal", "PodCreated", "Created VPN pod %s", podName)
	} else {
		// Pod exists — check drift and readiness
		if vpnPodNeedsRecreation(existing, desiredPod) {
			logger.Info("VPN pod needs recreation due to drift", "pod", podName)
			if err := r.Delete(ctx, existing); err != nil && !errors.IsNotFound(err) {
				logger.Error(err, "Failed to delete drifted VPN pod")
				return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
			}
			r.emitEvent(vpn, "Normal", "PodRecreated", "Recreating VPN pod %s due to config drift", podName)
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
		podReady = existing.Status.Phase == corev1.PodRunning
	}

	// Step 8: Collect advertised routes from all tunnels
	advertisedRoutes := collectAdvertisedRoutes(vpn)

	// Step 9: Count issued clients for OpenVPN
	if vpn.Spec.Protocol == "openvpn" {
		count, countErr := r.countIssuedClients(ctx, vpn)
		if countErr != nil {
			logger.Error(countErr, "Failed to count issued clients")
		} else {
			vpn.Status.IssuedClients = count
		}
	}

	// Step 10: Update status
	now := metav1.Now()
	if podReady {
		vpn.Status.Phase = "Ready"
	} else {
		vpn.Status.Phase = "Provisioning"
	}
	vpn.Status.PodName = podName
	vpn.Status.TunnelEndpoint = gw.Status.FloatingIP
	vpn.Status.TotalTunnels = int32(len(vpn.Spec.Tunnels))
	vpn.Status.AdvertisedRoutes = advertisedRoutes
	vpn.Status.SyncStatus = "Synced"
	vpn.Status.LastSyncTime = &now

	if podReady {
		vpn.Status.Message = fmt.Sprintf("VPN gateway %s is ready via %s", vpn.Name, vpn.Spec.Protocol)
	} else {
		vpn.Status.Message = fmt.Sprintf("VPN pod %s is starting", podName)
	}

	// Set conditions
	podCondStatus := metav1.ConditionTrue
	podCondReason := "PodRunning"
	podCondMessage := "VPN pod is running"
	if !podReady {
		podCondStatus = metav1.ConditionFalse
		podCondReason = "PodNotReady"
		podCondMessage = "VPN pod is not yet running"
	}
	meta.SetStatusCondition(&vpn.Status.Conditions, metav1.Condition{
		Type:               "PodReady",
		Status:             podCondStatus,
		Reason:             podCondReason,
		Message:            podCondMessage,
		LastTransitionTime: now,
	})
	meta.SetStatusCondition(&vpn.Status.Conditions, metav1.Condition{
		Type:               "GatewayConnected",
		Status:             metav1.ConditionTrue,
		Reason:             "GatewayReady",
		Message:            fmt.Sprintf("Connected to gateway %q", vpn.Spec.GatewayRef),
		LastTransitionTime: now,
	})

	if err := r.Status().Update(ctx, vpn); err != nil {
		logger.Error(err, "Failed to update VPCVPNGateway status")
		return ctrl.Result{}, err
	}

	operatormetrics.ReconcileOpsTotal.WithLabelValues("vpcvpngateway", "reconcile", "success").Inc()
	r.emitEvent(vpn, "Normal", "Synced", "VPN gateway %s is %s", vpn.Name, vpn.Status.Phase)

	logger.Info("Successfully reconciled VPCVPNGateway",
		"name", vpn.Name, "phase", vpn.Status.Phase, "podReady", podReady,
		"tunnels", len(vpn.Spec.Tunnels), "advertisedRoutes", len(advertisedRoutes))

	if !podReady {
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}
	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

// validateConfig validates protocol-specific configuration on the VPN spec.
func (r *Reconciler) validateConfig(vpn *v1alpha1.VPCVPNGateway) error {
	switch vpn.Spec.Protocol {
	case "wireguard":
		if vpn.Spec.WireGuard == nil {
			return fmt.Errorf("protocol %q requires spec.wireGuard configuration", vpn.Spec.Protocol)
		}
		for _, tunnel := range vpn.Spec.Tunnels {
			if tunnel.PeerPublicKey == "" {
				return fmt.Errorf("WireGuard tunnel %q requires peerPublicKey", tunnel.Name)
			}
		}
	case "ipsec":
		for _, tunnel := range vpn.Spec.Tunnels {
			if tunnel.PresharedKey == nil {
				return fmt.Errorf("IPsec tunnel %q requires presharedKey", tunnel.Name)
			}
		}
	case "openvpn":
		if vpn.Spec.OpenVPN == nil {
			return fmt.Errorf("protocol %q requires spec.openVPN configuration", vpn.Spec.Protocol)
		}
	}
	return nil
}

// collectAdvertisedRoutes gathers all remote network CIDRs from all tunnels.
func collectAdvertisedRoutes(vpn *v1alpha1.VPCVPNGateway) []string {
	seen := map[string]bool{}
	for _, tunnel := range vpn.Spec.Tunnels {
		for _, cidr := range tunnel.RemoteNetworks {
			seen[cidr] = true
		}
	}
	routes := make([]string, 0, len(seen))
	for cidr := range seen {
		routes = append(routes, cidr)
	}
	return routes
}

// reconcileDelete handles VPCVPNGateway deletion by cleaning up the pod and removing the finalizer.
func (r *Reconciler) reconcileDelete(ctx context.Context, vpn *v1alpha1.VPCVPNGateway) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling VPCVPNGateway deletion", "name", vpn.Name)

	// Delete the VPN pod
	podName := vpnPodName(vpn.Name)
	pod := &corev1.Pod{}
	if err := r.Get(ctx, types.NamespacedName{Name: podName, Namespace: vpn.Namespace}, pod); err == nil {
		if err := r.Delete(ctx, pod); err != nil && !errors.IsNotFound(err) {
			logger.Error(err, "Failed to delete VPN pod", "pod", podName)
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}
		logger.Info("Deleted VPN pod", "pod", podName)
		r.emitEvent(vpn, "Normal", "PodDeleted", "Deleted VPN pod %s", podName)
	}

	// Remove finalizer
	if err := finalizers.EnsureRemoved(ctx, r.Client, vpn, finalizers.VPNGatewayCleanup); err != nil {
		logger.Error(err, "Failed to remove finalizer")
		return ctrl.Result{}, err
	}

	operatormetrics.ReconcileOpsTotal.WithLabelValues("vpcvpngateway", "delete", "success").Inc()
	r.emitEvent(vpn, "Normal", "Deleted", "VPCVPNGateway %s finalizer removed", vpn.Name)

	return ctrl.Result{}, nil
}

// setPendingStatus sets the VPN gateway status to Pending and requeues after 30 seconds.
func (r *Reconciler) setPendingStatus(ctx context.Context, vpn *v1alpha1.VPCVPNGateway, message string) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	vpn.Status.Phase = "Pending"
	vpn.Status.SyncStatus = "Pending"
	vpn.Status.Message = message

	if err := r.Status().Update(ctx, vpn); err != nil {
		logger.Error(err, "Failed to update status to Pending")
	}

	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// setErrorStatus sets the VPN gateway status to Error. Does not requeue — the user
// must fix the spec, which will trigger a new reconcile via the watch.
func (r *Reconciler) setErrorStatus(ctx context.Context, vpn *v1alpha1.VPCVPNGateway, message string) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	vpn.Status.Phase = "Error"
	vpn.Status.SyncStatus = "Failed"
	vpn.Status.Message = message

	if err := r.Status().Update(ctx, vpn); err != nil {
		logger.Error(err, "Failed to update status to Error")
	}

	return ctrl.Result{}, nil
}

func (r *Reconciler) emitEvent(obj runtime.Object, eventType, reason, messageFmt string, args ...interface{}) {
	if r.Recorder != nil {
		r.Recorder.Eventf(obj, eventType, reason, messageFmt, args...)
	}
}

// ensureCRLSecret creates the convention-based CRL secret (<vpnName>-crl) if it doesn't exist.
// The secret starts empty and is populated by the BFF when a client is revoked.
func (r *Reconciler) ensureCRLSecret(ctx context.Context, vpn *v1alpha1.VPCVPNGateway) error {
	crlName := vpn.Name + "-crl"
	existing := &corev1.Secret{}
	err := r.Get(ctx, types.NamespacedName{Name: crlName, Namespace: vpn.Namespace}, existing)
	if err == nil {
		return nil // already exists
	}
	if !errors.IsNotFound(err) {
		return err
	}

	isTrue := true
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      crlName,
			Namespace: vpn.Namespace,
			Labels: map[string]string{
				"vpc.roks.ibm.com/vpngateway": vpn.Name,
				"vpc.roks.ibm.com/crl":        "true",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         "vpc.roks.ibm.com/v1alpha1",
					Kind:               "VPCVPNGateway",
					Name:               vpn.Name,
					UID:                vpn.UID,
					Controller:         &isTrue,
					BlockOwnerDeletion: &isTrue,
				},
			},
		},
		Data: map[string][]byte{},
	}

	if createErr := r.Create(ctx, secret); createErr != nil {
		if errors.IsAlreadyExists(createErr) {
			return nil
		}
		return createErr
	}

	log.FromContext(ctx).Info("Created CRL secret", "secret", crlName)
	r.emitEvent(vpn, "Normal", "CRLSecretCreated", "Created CRL secret %s", crlName)
	return nil
}

// countIssuedClients counts Secrets with the client-config label for this VPN gateway.
func (r *Reconciler) countIssuedClients(ctx context.Context, vpn *v1alpha1.VPCVPNGateway) (int32, error) {
	secretList := &corev1.SecretList{}
	err := r.List(ctx, secretList,
		client.InNamespace(vpn.Namespace),
		client.MatchingLabels{
			"vpc.roks.ibm.com/vpngateway": vpn.Name,
		},
		client.HasLabels{"vpc.roks.ibm.com/client-config"},
	)
	if err != nil {
		return 0, err
	}
	return int32(len(secretList.Items)), nil
}

// SetupWithManager registers the VPCVPNGateway reconciler with the controller manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Recorder = mgr.GetEventRecorderFor("vpcvpngateway-controller")
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.VPCVPNGateway{}).
		Owns(&corev1.Pod{}).
		Watches(&v1alpha1.VPCGateway{}, handler.EnqueueRequestsFromMapFunc(
			func(ctx context.Context, obj client.Object) []reconcile.Request {
				gw, ok := obj.(*v1alpha1.VPCGateway)
				if !ok {
					return nil
				}
				// List all VPNGateways that reference this gateway
				var vpnList v1alpha1.VPCVPNGatewayList
				if err := r.Client.List(ctx, &vpnList, client.InNamespace(gw.Namespace)); err != nil {
					return nil
				}
				var requests []reconcile.Request
				for _, vpn := range vpnList.Items {
					if vpn.Spec.GatewayRef == gw.Name {
						requests = append(requests, reconcile.Request{
							NamespacedName: types.NamespacedName{
								Name:      vpn.Name,
								Namespace: vpn.Namespace,
							},
						})
					}
				}
				return requests
			},
		)).
		Watches(&corev1.Secret{}, handler.EnqueueRequestsFromMapFunc(
			func(ctx context.Context, obj client.Object) []reconcile.Request {
				vpnName := obj.GetLabels()["vpc.roks.ibm.com/vpngateway"]
				if vpnName == "" {
					return nil
				}
				return []reconcile.Request{{
					NamespacedName: types.NamespacedName{
						Name:      vpnName,
						Namespace: obj.GetNamespace(),
					},
				}}
			},
		)).
		Complete(r)
}
