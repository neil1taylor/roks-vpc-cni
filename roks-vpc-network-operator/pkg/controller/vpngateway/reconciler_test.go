package vpngateway

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1alpha1 "github.com/IBM/roks-vpc-network-operator/api/v1alpha1"
)

func newTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(s)
	_ = v1alpha1.AddToScheme(s)
	return s
}

func TestReconcileNormal_CreateWireGuardVPN(t *testing.T) {
	scheme := newTestScheme()

	gw := &v1alpha1.VPCGateway{
		ObjectMeta: metav1.ObjectMeta{Name: "gw-test", Namespace: "default"},
		Spec: v1alpha1.VPCGatewaySpec{
			Zone:   "eu-de-1",
			Uplink: v1alpha1.GatewayUplink{Network: "uplink-net"},
		},
		Status: v1alpha1.VPCGatewayStatus{
			Phase:      "Ready",
			FloatingIP: "169.48.1.1",
			MACAddress: "fa:16:3e:aa:bb:cc",
			ReservedIP: "10.240.1.5",
		},
	}

	listenPort := int32(51820)
	vpn := &v1alpha1.VPCVPNGateway{
		ObjectMeta: metav1.ObjectMeta{Name: "vpn-test", Namespace: "default"},
		Spec: v1alpha1.VPCVPNGatewaySpec{
			Protocol:   "wireguard",
			GatewayRef: "gw-test",
			WireGuard: &v1alpha1.VPNWireGuardConfig{
				PrivateKey: v1alpha1.SecretKeyRef{Name: "wg-secret", Key: "privateKey"},
				ListenPort: &listenPort,
			},
			Tunnels: []v1alpha1.VPNTunnel{
				{
					Name:                "dc1",
					RemoteEndpoint:      "198.51.100.1",
					RemoteNetworks:      []string{"10.0.0.0/8"},
					PeerPublicKey:       "dGVzdC1wdWJsaWMta2V5",
					TunnelAddressLocal:  "10.99.0.1/30",
					TunnelAddressRemote: "10.99.0.2/30",
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(gw, vpn).
		WithStatusSubresource(gw, vpn).
		Build()

	r := &Reconciler{
		Client: fakeClient,
		Scheme: scheme,
	}

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "vpn-test", Namespace: "default"},
	})

	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if result.RequeueAfter == 0 {
		t.Error("expected requeue after pod creation")
	}

	// Verify status
	updated := &v1alpha1.VPCVPNGateway{}
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "vpn-test", Namespace: "default"}, updated); err != nil {
		t.Fatalf("Failed to get updated VPCVPNGateway: %v", err)
	}

	if updated.Status.Phase != "Provisioning" {
		t.Errorf("Phase = %q, want %q", updated.Status.Phase, "Provisioning")
	}
	if updated.Status.SyncStatus != "Synced" {
		t.Errorf("SyncStatus = %q, want %q", updated.Status.SyncStatus, "Synced")
	}
	if updated.Status.PodName != "vpngw-vpn-test" {
		t.Errorf("PodName = %q, want %q", updated.Status.PodName, "vpngw-vpn-test")
	}
	if updated.Status.TunnelEndpoint != "169.48.1.1" {
		t.Errorf("TunnelEndpoint = %q, want %q", updated.Status.TunnelEndpoint, "169.48.1.1")
	}
	if updated.Status.TotalTunnels != 1 {
		t.Errorf("TotalTunnels = %d, want %d", updated.Status.TotalTunnels, 1)
	}

	// Verify advertised routes
	if len(updated.Status.AdvertisedRoutes) != 1 || updated.Status.AdvertisedRoutes[0] != "10.0.0.0/8" {
		t.Errorf("AdvertisedRoutes = %v, want [10.0.0.0/8]", updated.Status.AdvertisedRoutes)
	}

	// Verify finalizer
	hasFinalizer := false
	for _, f := range updated.Finalizers {
		if f == "vpc.roks.ibm.com/vpngateway-cleanup" {
			hasFinalizer = true
			break
		}
	}
	if !hasFinalizer {
		t.Errorf("expected finalizer, got %v", updated.Finalizers)
	}

	// Verify pod was created
	pod := &corev1.Pod{}
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "vpngw-vpn-test", Namespace: "default"}, pod); err != nil {
		t.Fatalf("Expected VPN pod to be created: %v", err)
	}
	if pod.Labels["app"] != "vpngateway" {
		t.Errorf("pod label app = %q, want %q", pod.Labels["app"], "vpngateway")
	}
}

func TestReconcileNormal_CreateIPsecVPN(t *testing.T) {
	scheme := newTestScheme()

	gw := &v1alpha1.VPCGateway{
		ObjectMeta: metav1.ObjectMeta{Name: "gw-test", Namespace: "default"},
		Spec: v1alpha1.VPCGatewaySpec{
			Zone:   "eu-de-1",
			Uplink: v1alpha1.GatewayUplink{Network: "uplink-net"},
		},
		Status: v1alpha1.VPCGatewayStatus{
			Phase:      "Ready",
			FloatingIP: "169.48.2.2",
		},
	}

	vpn := &v1alpha1.VPCVPNGateway{
		ObjectMeta: metav1.ObjectMeta{Name: "vpn-ipsec", Namespace: "default"},
		Spec: v1alpha1.VPCVPNGatewaySpec{
			Protocol:   "ipsec",
			GatewayRef: "gw-test",
			IPsec:      &v1alpha1.VPNIPsecConfig{},
			Tunnels: []v1alpha1.VPNTunnel{
				{
					Name:           "site-b",
					RemoteEndpoint: "203.0.113.20",
					RemoteNetworks: []string{"192.168.0.0/16"},
					PresharedKey:   &v1alpha1.SecretKeyRef{Name: "psk-site-b", Key: "psk"},
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(gw, vpn).
		WithStatusSubresource(gw, vpn).
		Build()

	r := &Reconciler{Client: fakeClient, Scheme: scheme}

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "vpn-ipsec", Namespace: "default"},
	})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if result.RequeueAfter == 0 {
		t.Error("expected requeue after pod creation")
	}

	// Verify pod was created with correct name
	pod := &corev1.Pod{}
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "vpngw-vpn-ipsec", Namespace: "default"}, pod); err != nil {
		t.Fatalf("Expected IPsec VPN pod to be created: %v", err)
	}
}

func TestReconcileNormal_GatewayNotReady(t *testing.T) {
	scheme := newTestScheme()

	gw := &v1alpha1.VPCGateway{
		ObjectMeta: metav1.ObjectMeta{Name: "gw-pending", Namespace: "default"},
		Status:     v1alpha1.VPCGatewayStatus{Phase: "Pending"},
	}

	listenPort := int32(51820)
	vpn := &v1alpha1.VPCVPNGateway{
		ObjectMeta: metav1.ObjectMeta{Name: "vpn-wait", Namespace: "default"},
		Spec: v1alpha1.VPCVPNGatewaySpec{
			Protocol:   "wireguard",
			GatewayRef: "gw-pending",
			WireGuard:  &v1alpha1.VPNWireGuardConfig{PrivateKey: v1alpha1.SecretKeyRef{Name: "s", Key: "k"}, ListenPort: &listenPort},
			Tunnels:    []v1alpha1.VPNTunnel{{Name: "t1", RemoteEndpoint: "1.2.3.4", RemoteNetworks: []string{"10.0.0.0/8"}, PeerPublicKey: "key"}},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(gw, vpn).
		WithStatusSubresource(gw, vpn).
		Build()

	r := &Reconciler{Client: fakeClient, Scheme: scheme}

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "vpn-wait", Namespace: "default"},
	})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if result.RequeueAfter == 0 {
		t.Error("expected requeue when gateway is not ready")
	}

	updated := &v1alpha1.VPCVPNGateway{}
	_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: "vpn-wait", Namespace: "default"}, updated)
	if updated.Status.Phase != "Pending" {
		t.Errorf("Phase = %q, want %q", updated.Status.Phase, "Pending")
	}
}

func TestReconcileNormal_GatewayNotFound(t *testing.T) {
	scheme := newTestScheme()

	listenPort := int32(51820)
	vpn := &v1alpha1.VPCVPNGateway{
		ObjectMeta: metav1.ObjectMeta{Name: "vpn-orphan", Namespace: "default"},
		Spec: v1alpha1.VPCVPNGatewaySpec{
			Protocol:   "wireguard",
			GatewayRef: "gw-missing",
			WireGuard:  &v1alpha1.VPNWireGuardConfig{PrivateKey: v1alpha1.SecretKeyRef{Name: "s", Key: "k"}, ListenPort: &listenPort},
			Tunnels:    []v1alpha1.VPNTunnel{{Name: "t1", RemoteEndpoint: "1.2.3.4", RemoteNetworks: []string{"10.0.0.0/8"}, PeerPublicKey: "key"}},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(vpn).
		WithStatusSubresource(vpn).
		Build()

	r := &Reconciler{Client: fakeClient, Scheme: scheme}

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "vpn-orphan", Namespace: "default"},
	})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if result.RequeueAfter == 0 {
		t.Error("expected requeue when gateway is not found")
	}

	updated := &v1alpha1.VPCVPNGateway{}
	_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: "vpn-orphan", Namespace: "default"}, updated)
	if updated.Status.Phase != "Pending" {
		t.Errorf("Phase = %q, want %q", updated.Status.Phase, "Pending")
	}
}

func TestReconcileNormal_MissingWireGuardConfig(t *testing.T) {
	scheme := newTestScheme()

	gw := &v1alpha1.VPCGateway{
		ObjectMeta: metav1.ObjectMeta{Name: "gw-test", Namespace: "default"},
		Status:     v1alpha1.VPCGatewayStatus{Phase: "Ready", FloatingIP: "1.2.3.4"},
	}

	vpn := &v1alpha1.VPCVPNGateway{
		ObjectMeta: metav1.ObjectMeta{Name: "vpn-bad", Namespace: "default"},
		Spec: v1alpha1.VPCVPNGatewaySpec{
			Protocol:   "wireguard",
			GatewayRef: "gw-test",
			WireGuard:  nil, // Missing!
			Tunnels:    []v1alpha1.VPNTunnel{{Name: "t1", RemoteEndpoint: "1.2.3.4", RemoteNetworks: []string{"10.0.0.0/8"}, PeerPublicKey: "key"}},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(gw, vpn).
		WithStatusSubresource(gw, vpn).
		Build()

	r := &Reconciler{Client: fakeClient, Scheme: scheme}

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "vpn-bad", Namespace: "default"},
	})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if result.RequeueAfter != 0 || result.Requeue {
		t.Error("should not requeue on config error")
	}

	updated := &v1alpha1.VPCVPNGateway{}
	_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: "vpn-bad", Namespace: "default"}, updated)
	if updated.Status.Phase != "Error" {
		t.Errorf("Phase = %q, want %q", updated.Status.Phase, "Error")
	}
}

func TestReconcileNormal_MissingIPsecPSK(t *testing.T) {
	scheme := newTestScheme()

	gw := &v1alpha1.VPCGateway{
		ObjectMeta: metav1.ObjectMeta{Name: "gw-test", Namespace: "default"},
		Status:     v1alpha1.VPCGatewayStatus{Phase: "Ready", FloatingIP: "1.2.3.4"},
	}

	vpn := &v1alpha1.VPCVPNGateway{
		ObjectMeta: metav1.ObjectMeta{Name: "vpn-bad-ipsec", Namespace: "default"},
		Spec: v1alpha1.VPCVPNGatewaySpec{
			Protocol:   "ipsec",
			GatewayRef: "gw-test",
			IPsec:      &v1alpha1.VPNIPsecConfig{},
			Tunnels: []v1alpha1.VPNTunnel{
				{
					Name:           "t1",
					RemoteEndpoint: "1.2.3.4",
					RemoteNetworks: []string{"10.0.0.0/8"},
					PresharedKey:   nil, // Missing!
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(gw, vpn).
		WithStatusSubresource(gw, vpn).
		Build()

	r := &Reconciler{Client: fakeClient, Scheme: scheme}

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "vpn-bad-ipsec", Namespace: "default"},
	})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if result.RequeueAfter != 0 || result.Requeue {
		t.Error("should not requeue on config error")
	}

	updated := &v1alpha1.VPCVPNGateway{}
	_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: "vpn-bad-ipsec", Namespace: "default"}, updated)
	if updated.Status.Phase != "Error" {
		t.Errorf("Phase = %q, want %q", updated.Status.Phase, "Error")
	}
}

func TestReconcileDelete_CleanupPod(t *testing.T) {
	scheme := newTestScheme()

	now := metav1.Now()
	vpn := &v1alpha1.VPCVPNGateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "vpn-delete",
			Namespace:         "default",
			DeletionTimestamp: &now,
			Finalizers:        []string{"vpc.roks.ibm.com/vpngateway-cleanup"},
		},
		Spec: v1alpha1.VPCVPNGatewaySpec{
			Protocol:   "wireguard",
			GatewayRef: "gw-test",
			Tunnels:    []v1alpha1.VPNTunnel{{Name: "t1", RemoteEndpoint: "1.2.3.4", RemoteNetworks: []string{"10.0.0.0/8"}}},
		},
	}

	isTrue := true
	existingPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vpngw-vpn-delete",
			Namespace: "default",
			Labels:    map[string]string{"vpc.roks.ibm.com/vpngateway": "vpn-delete"},
			OwnerReferences: []metav1.OwnerReference{
				{APIVersion: "vpc.roks.ibm.com/v1alpha1", Kind: "VPCVPNGateway", Name: "vpn-delete", Controller: &isTrue},
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "vpn", Image: "test:latest"}},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(vpn, existingPod).
		WithStatusSubresource(vpn).
		Build()

	r := &Reconciler{Client: fakeClient, Scheme: scheme}

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "vpn-delete", Namespace: "default"},
	})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if result.Requeue {
		t.Error("should not requeue after successful delete")
	}

	// Verify pod was deleted
	pod := &corev1.Pod{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{Name: "vpngw-vpn-delete", Namespace: "default"}, pod)
	if err == nil {
		t.Error("expected VPN pod to be deleted")
	} else if !errors.IsNotFound(err) {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify finalizer removed (object should be fully deleted with fake client)
	updated := &v1alpha1.VPCVPNGateway{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{Name: "vpn-delete", Namespace: "default"}, updated)
	if err != nil {
		if !errors.IsNotFound(err) {
			t.Fatalf("unexpected error: %v", err)
		}
	} else {
		for _, f := range updated.Finalizers {
			if f == "vpc.roks.ibm.com/vpngateway-cleanup" {
				t.Error("expected finalizer to be removed")
			}
		}
	}
}

func TestReconcileNormal_CreateOpenVPNGateway(t *testing.T) {
	scheme := newTestScheme()

	gw := &v1alpha1.VPCGateway{
		ObjectMeta: metav1.ObjectMeta{Name: "gw-test", Namespace: "default"},
		Spec: v1alpha1.VPCGatewaySpec{
			Zone:   "eu-de-1",
			Uplink: v1alpha1.GatewayUplink{Network: "uplink-net"},
		},
		Status: v1alpha1.VPCGatewayStatus{
			Phase:      "Ready",
			FloatingIP: "169.48.3.3",
			MACAddress: "fa:16:3e:cc:dd:ee",
			ReservedIP: "10.240.1.10",
		},
	}

	listenPort := int32(1194)
	vpn := &v1alpha1.VPCVPNGateway{
		ObjectMeta: metav1.ObjectMeta{Name: "test-ovpn", Namespace: "default"},
		Spec: v1alpha1.VPCVPNGatewaySpec{
			Protocol:   "openvpn",
			GatewayRef: "gw-test",
			OpenVPN: &v1alpha1.VPNOpenVPNConfig{
				CA:           v1alpha1.SecretKeyRef{Name: "ovpn-ca", Key: "ca.crt"},
				Cert:         v1alpha1.SecretKeyRef{Name: "ovpn-cert", Key: "server.crt"},
				Key:          v1alpha1.SecretKeyRef{Name: "ovpn-key", Key: "server.key"},
				ListenPort:   &listenPort,
				Proto:        "udp",
				Cipher:       "AES-256-GCM",
				ClientSubnet: "10.8.0.0/24",
			},
			Tunnels: []v1alpha1.VPNTunnel{
				{
					Name:           "to-dc",
					RemoteEndpoint: "203.0.113.1",
					RemoteNetworks: []string{"10.0.0.0/8"},
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(gw, vpn).
		WithStatusSubresource(gw, vpn).
		Build()

	r := &Reconciler{
		Client: fakeClient,
		Scheme: scheme,
	}

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "test-ovpn", Namespace: "default"},
	})

	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if result.RequeueAfter == 0 {
		t.Error("expected requeue after pod creation")
	}

	// Verify pod was created with correct name and image
	pod := &corev1.Pod{}
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "vpngw-test-ovpn", Namespace: "default"}, pod); err != nil {
		t.Fatalf("Expected OpenVPN pod to be created: %v", err)
	}
	if pod.Spec.Containers[0].Image != defaultVPNImage {
		t.Errorf("pod image = %q, want %q", pod.Spec.Containers[0].Image, defaultVPNImage)
	}

	// Verify status
	updated := &v1alpha1.VPCVPNGateway{}
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "test-ovpn", Namespace: "default"}, updated); err != nil {
		t.Fatalf("Failed to get updated VPCVPNGateway: %v", err)
	}

	if updated.Status.Phase != "Provisioning" {
		t.Errorf("Phase = %q, want %q", updated.Status.Phase, "Provisioning")
	}
	if updated.Status.PodName != "vpngw-test-ovpn" {
		t.Errorf("PodName = %q, want %q", updated.Status.PodName, "vpngw-test-ovpn")
	}
	if updated.Status.TunnelEndpoint != "169.48.3.3" {
		t.Errorf("TunnelEndpoint = %q, want %q", updated.Status.TunnelEndpoint, "169.48.3.3")
	}
	if updated.Status.TotalTunnels != 1 {
		t.Errorf("TotalTunnels = %d, want %d", updated.Status.TotalTunnels, 1)
	}

	// Verify advertised routes
	if len(updated.Status.AdvertisedRoutes) != 1 || updated.Status.AdvertisedRoutes[0] != "10.0.0.0/8" {
		t.Errorf("AdvertisedRoutes = %v, want [10.0.0.0/8]", updated.Status.AdvertisedRoutes)
	}
}

func TestReconcileNormal_MissingOpenVPNConfig(t *testing.T) {
	scheme := newTestScheme()

	gw := &v1alpha1.VPCGateway{
		ObjectMeta: metav1.ObjectMeta{Name: "gw-test", Namespace: "default"},
		Status:     v1alpha1.VPCGatewayStatus{Phase: "Ready", FloatingIP: "1.2.3.4"},
	}

	vpn := &v1alpha1.VPCVPNGateway{
		ObjectMeta: metav1.ObjectMeta{Name: "vpn-bad-ovpn", Namespace: "default"},
		Spec: v1alpha1.VPCVPNGatewaySpec{
			Protocol:   "openvpn",
			GatewayRef: "gw-test",
			OpenVPN:    nil, // Missing!
			Tunnels: []v1alpha1.VPNTunnel{
				{
					Name:           "t1",
					RemoteEndpoint: "1.2.3.4",
					RemoteNetworks: []string{"10.0.0.0/8"},
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(gw, vpn).
		WithStatusSubresource(gw, vpn).
		Build()

	r := &Reconciler{Client: fakeClient, Scheme: scheme}

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "vpn-bad-ovpn", Namespace: "default"},
	})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if result.RequeueAfter != 0 || result.Requeue {
		t.Error("should not requeue on config error")
	}

	updated := &v1alpha1.VPCVPNGateway{}
	_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: "vpn-bad-ovpn", Namespace: "default"}, updated)
	if updated.Status.Phase != "Error" {
		t.Errorf("Phase = %q, want %q", updated.Status.Phase, "Error")
	}
}

func TestCollectAdvertisedRoutes(t *testing.T) {
	vpn := &v1alpha1.VPCVPNGateway{
		Spec: v1alpha1.VPCVPNGatewaySpec{
			Tunnels: []v1alpha1.VPNTunnel{
				{Name: "dc1", RemoteNetworks: []string{"10.0.0.0/8", "172.16.0.0/12"}},
				{Name: "dc2", RemoteNetworks: []string{"192.168.0.0/16", "10.0.0.0/8"}}, // duplicate
			},
		},
	}

	routes := collectAdvertisedRoutes(vpn)

	// Should deduplicate
	if len(routes) != 3 {
		t.Errorf("expected 3 unique routes, got %d: %v", len(routes), routes)
	}

	// Verify all expected routes are present
	expected := map[string]bool{"10.0.0.0/8": true, "172.16.0.0/12": true, "192.168.0.0/16": true}
	for _, r := range routes {
		if !expected[r] {
			t.Errorf("unexpected route %q", r)
		}
	}
}
