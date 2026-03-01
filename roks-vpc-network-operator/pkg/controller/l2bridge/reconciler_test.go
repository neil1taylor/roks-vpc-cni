package l2bridge

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

// TestReconcileNormal_CreateGRETAPBridge tests the happy path: gateway exists
// and is Ready, bridge has valid gretap-wireguard config. Assert that the
// bridge pod is created, phase is Provisioning, finalizer is added, and the
// pod has correct labels and Multus annotation.
func TestReconcileNormal_CreateGRETAPBridge(t *testing.T) {
	scheme := newTestScheme()

	gw := &v1alpha1.VPCGateway{
		ObjectMeta: metav1.ObjectMeta{Name: "gw-test", Namespace: "default"},
		Spec: v1alpha1.VPCGatewaySpec{
			Zone: "eu-de-1",
			Uplink: v1alpha1.GatewayUplink{
				Network: "uplink-net",
			},
		},
		Status: v1alpha1.VPCGatewayStatus{
			Phase:      "Ready",
			FloatingIP: "169.48.1.1",
			VNIID:      "vni-gw-123",
			MACAddress: "fa:16:3e:aa:bb:cc",
			ReservedIP: "10.240.1.5",
		},
	}

	listenPort := int32(51820)
	bridge := &v1alpha1.VPCL2Bridge{
		ObjectMeta: metav1.ObjectMeta{Name: "bridge-test", Namespace: "default"},
		Spec: v1alpha1.VPCL2BridgeSpec{
			Type:       "gretap-wireguard",
			GatewayRef: "gw-test",
			NetworkRef: v1alpha1.BridgeNetworkRef{
				Name: "localnet-1",
				Kind: "ClusterUserDefinedNetwork",
			},
			Remote: v1alpha1.BridgeRemote{
				Endpoint: "203.0.113.10",
				WireGuard: &v1alpha1.BridgeWireGuard{
					PrivateKey: v1alpha1.SecretKeyRef{
						Name: "wg-secret",
						Key:  "privateKey",
					},
					PeerPublicKey:       "dGVzdC1wdWJsaWMta2V5LWJhc2U2NC1lbmNvZGVk",
					ListenPort:          &listenPort,
					TunnelAddressLocal:  "10.99.0.1/30",
					TunnelAddressRemote: "10.99.0.2/30",
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(gw, bridge).
		WithStatusSubresource(gw, bridge).
		Build()

	r := &Reconciler{
		Client: fakeClient,
		Scheme: scheme,
	}

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "bridge-test", Namespace: "default"},
	})

	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	// Pod was just created — should requeue for readiness check
	if result.RequeueAfter == 0 {
		t.Error("expected requeue after pod creation, got 0")
	}

	// Verify the updated bridge status
	updated := &v1alpha1.VPCL2Bridge{}
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "bridge-test", Namespace: "default"}, updated); err != nil {
		t.Fatalf("Failed to get updated VPCL2Bridge: %v", err)
	}

	// Phase should be Provisioning (pod just created, not Running yet)
	if updated.Status.Phase != "Provisioning" {
		t.Errorf("expected Phase = 'Provisioning', got %q", updated.Status.Phase)
	}

	// SyncStatus should be Synced
	if updated.Status.SyncStatus != "Synced" {
		t.Errorf("expected SyncStatus = 'Synced', got %q", updated.Status.SyncStatus)
	}

	// PodName should be set
	if updated.Status.PodName != "l2bridge-bridge-test" {
		t.Errorf("expected PodName = 'l2bridge-bridge-test', got %q", updated.Status.PodName)
	}

	// Finalizer should be present
	hasFinalizer := false
	for _, f := range updated.Finalizers {
		if f == "vpc.roks.ibm.com/l2bridge-cleanup" {
			hasFinalizer = true
			break
		}
	}
	if !hasFinalizer {
		t.Errorf("expected finalizer vpc.roks.ibm.com/l2bridge-cleanup to be present, got %v", updated.Finalizers)
	}

	// Verify the bridge pod was created
	pod := &corev1.Pod{}
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "l2bridge-bridge-test", Namespace: "default"}, pod); err != nil {
		t.Fatalf("Expected bridge pod to be created: %v", err)
	}

	// Verify pod labels
	if pod.Labels["vpc.roks.ibm.com/l2bridge"] != "bridge-test" {
		t.Errorf("expected l2bridge label = 'bridge-test', got %q", pod.Labels["vpc.roks.ibm.com/l2bridge"])
	}
	if pod.Labels["app"] != "l2bridge" {
		t.Errorf("expected app label = 'l2bridge', got %q", pod.Labels["app"])
	}

	// Verify Multus annotation is present
	multusAnnotation := pod.Annotations["k8s.v1.cni.cncf.io/networks"]
	if multusAnnotation == "" {
		t.Error("expected Multus annotation to be present")
	}

	// Verify owner reference
	if len(pod.OwnerReferences) != 1 || pod.OwnerReferences[0].Name != "bridge-test" {
		t.Errorf("expected owner reference to bridge-test, got %v", pod.OwnerReferences)
	}
}

// TestReconcileNormal_GatewayNotReady tests that when the referenced gateway
// is not Ready, the reconciler sets Phase=Pending with a descriptive message
// and requeues.
func TestReconcileNormal_GatewayNotReady(t *testing.T) {
	scheme := newTestScheme()

	gw := &v1alpha1.VPCGateway{
		ObjectMeta: metav1.ObjectMeta{Name: "gw-pending", Namespace: "default"},
		Status: v1alpha1.VPCGatewayStatus{
			Phase: "Pending",
		},
	}

	listenPort := int32(51820)
	bridge := &v1alpha1.VPCL2Bridge{
		ObjectMeta: metav1.ObjectMeta{Name: "bridge-wait", Namespace: "default"},
		Spec: v1alpha1.VPCL2BridgeSpec{
			Type:       "gretap-wireguard",
			GatewayRef: "gw-pending",
			NetworkRef: v1alpha1.BridgeNetworkRef{Name: "localnet-1"},
			Remote: v1alpha1.BridgeRemote{
				Endpoint: "203.0.113.10",
				WireGuard: &v1alpha1.BridgeWireGuard{
					PrivateKey:          v1alpha1.SecretKeyRef{Name: "wg-secret", Key: "privateKey"},
					PeerPublicKey:       "dGVzdC1rZXk=",
					ListenPort:          &listenPort,
					TunnelAddressLocal:  "10.99.0.1/30",
					TunnelAddressRemote: "10.99.0.2/30",
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(gw, bridge).
		WithStatusSubresource(gw, bridge).
		Build()

	r := &Reconciler{
		Client: fakeClient,
		Scheme: scheme,
	}

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "bridge-wait", Namespace: "default"},
	})

	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if result.RequeueAfter == 0 {
		t.Error("expected requeue when gateway is not ready")
	}

	updated := &v1alpha1.VPCL2Bridge{}
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "bridge-wait", Namespace: "default"}, updated); err != nil {
		t.Fatalf("Failed to get updated VPCL2Bridge: %v", err)
	}

	if updated.Status.Phase != "Pending" {
		t.Errorf("expected Phase = 'Pending', got %q", updated.Status.Phase)
	}
	if updated.Status.Message == "" {
		t.Error("expected a non-empty status message")
	}
	// Message should mention gateway
	if !containsString(updated.Status.Message, "gateway") && !containsString(updated.Status.Message, "Gateway") {
		t.Errorf("expected message to mention 'gateway', got %q", updated.Status.Message)
	}
}

// TestReconcileNormal_GatewayNotFound tests that when the referenced gateway
// doesn't exist, the reconciler sets Phase=Pending with a descriptive message
// and requeues.
func TestReconcileNormal_GatewayNotFound(t *testing.T) {
	scheme := newTestScheme()

	listenPort := int32(51820)
	bridge := &v1alpha1.VPCL2Bridge{
		ObjectMeta: metav1.ObjectMeta{Name: "bridge-orphan", Namespace: "default"},
		Spec: v1alpha1.VPCL2BridgeSpec{
			Type:       "gretap-wireguard",
			GatewayRef: "gw-missing",
			NetworkRef: v1alpha1.BridgeNetworkRef{Name: "localnet-1"},
			Remote: v1alpha1.BridgeRemote{
				Endpoint: "203.0.113.10",
				WireGuard: &v1alpha1.BridgeWireGuard{
					PrivateKey:          v1alpha1.SecretKeyRef{Name: "wg-secret", Key: "privateKey"},
					PeerPublicKey:       "dGVzdC1rZXk=",
					ListenPort:          &listenPort,
					TunnelAddressLocal:  "10.99.0.1/30",
					TunnelAddressRemote: "10.99.0.2/30",
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(bridge).
		WithStatusSubresource(bridge).
		Build()

	r := &Reconciler{
		Client: fakeClient,
		Scheme: scheme,
	}

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "bridge-orphan", Namespace: "default"},
	})

	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if result.RequeueAfter == 0 {
		t.Error("expected requeue when gateway is not found")
	}

	updated := &v1alpha1.VPCL2Bridge{}
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "bridge-orphan", Namespace: "default"}, updated); err != nil {
		t.Fatalf("Failed to get updated VPCL2Bridge: %v", err)
	}

	if updated.Status.Phase != "Pending" {
		t.Errorf("expected Phase = 'Pending', got %q", updated.Status.Phase)
	}
	if !containsString(updated.Status.Message, "not found") && !containsString(updated.Status.Message, "Not Found") {
		t.Errorf("expected message to contain 'not found', got %q", updated.Status.Message)
	}
}

// TestReconcileDelete_CleanupPod tests that when a VPCL2Bridge has a
// DeletionTimestamp set, the reconciler deletes the bridge pod and removes
// the finalizer.
func TestReconcileDelete_CleanupPod(t *testing.T) {
	scheme := newTestScheme()

	now := metav1.Now()
	bridge := &v1alpha1.VPCL2Bridge{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "bridge-delete",
			Namespace:         "default",
			DeletionTimestamp: &now,
			Finalizers:        []string{"vpc.roks.ibm.com/l2bridge-cleanup"},
		},
		Spec: v1alpha1.VPCL2BridgeSpec{
			Type:       "gretap-wireguard",
			GatewayRef: "gw-test",
			NetworkRef: v1alpha1.BridgeNetworkRef{Name: "localnet-1"},
			Remote: v1alpha1.BridgeRemote{
				Endpoint: "203.0.113.10",
			},
		},
	}

	// Create an existing bridge pod that should be deleted
	isTrue := true
	existingPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "l2bridge-bridge-delete",
			Namespace: "default",
			Labels: map[string]string{
				"vpc.roks.ibm.com/l2bridge": "bridge-delete",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "vpc.roks.ibm.com/v1alpha1",
					Kind:       "VPCL2Bridge",
					Name:       "bridge-delete",
					Controller: &isTrue,
				},
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "bridge", Image: "registry.access.redhat.com/ubi9/ubi:latest"},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(bridge, existingPod).
		WithStatusSubresource(bridge).
		Build()

	r := &Reconciler{
		Client: fakeClient,
		Scheme: scheme,
	}

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "bridge-delete", Namespace: "default"},
	})

	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if result.Requeue {
		t.Error("should not requeue after successful delete")
	}

	// Verify the bridge pod was deleted
	pod := &corev1.Pod{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{Name: "l2bridge-bridge-delete", Namespace: "default"}, pod)
	if err == nil {
		t.Error("expected bridge pod to be deleted")
	} else if !errors.IsNotFound(err) {
		t.Fatalf("unexpected error checking for deleted pod: %v", err)
	}

	// Verify finalizer was removed.
	// With the fake client, removing the last finalizer from an object with a
	// DeletionTimestamp causes the object to be garbage-collected, so a
	// not-found error is the expected successful outcome.
	updated := &v1alpha1.VPCL2Bridge{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{Name: "bridge-delete", Namespace: "default"}, updated)
	if err != nil {
		// Object was fully deleted (expected with fake client when last finalizer is removed)
		if !errors.IsNotFound(err) {
			t.Fatalf("unexpected error getting VPCL2Bridge after delete: %v", err)
		}
	} else {
		// If the object still exists, verify the finalizer was removed
		for _, f := range updated.Finalizers {
			if f == "vpc.roks.ibm.com/l2bridge-cleanup" {
				t.Error("expected finalizer to be removed after deletion")
			}
		}
	}
}

// TestReconcileNormal_MissingWireGuardConfig tests that when the bridge type
// is gretap-wireguard but spec.remote.wireguard is nil, the reconciler sets
// Phase=Error with a descriptive message and does not requeue.
func TestReconcileNormal_MissingWireGuardConfig(t *testing.T) {
	scheme := newTestScheme()

	gw := &v1alpha1.VPCGateway{
		ObjectMeta: metav1.ObjectMeta{Name: "gw-test", Namespace: "default"},
		Status: v1alpha1.VPCGatewayStatus{
			Phase:      "Ready",
			FloatingIP: "169.48.1.1",
		},
	}

	bridge := &v1alpha1.VPCL2Bridge{
		ObjectMeta: metav1.ObjectMeta{Name: "bridge-bad-config", Namespace: "default"},
		Spec: v1alpha1.VPCL2BridgeSpec{
			Type:       "gretap-wireguard",
			GatewayRef: "gw-test",
			NetworkRef: v1alpha1.BridgeNetworkRef{Name: "localnet-1"},
			Remote: v1alpha1.BridgeRemote{
				Endpoint:  "203.0.113.10",
				WireGuard: nil, // Missing WireGuard config
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(gw, bridge).
		WithStatusSubresource(gw, bridge).
		Build()

	r := &Reconciler{
		Client: fakeClient,
		Scheme: scheme,
	}

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "bridge-bad-config", Namespace: "default"},
	})

	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	// Should not requeue on config error — user must fix the spec
	if result.RequeueAfter != 0 || result.Requeue {
		t.Errorf("expected no requeue on config error, got RequeueAfter=%v Requeue=%v", result.RequeueAfter, result.Requeue)
	}

	updated := &v1alpha1.VPCL2Bridge{}
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "bridge-bad-config", Namespace: "default"}, updated); err != nil {
		t.Fatalf("Failed to get updated VPCL2Bridge: %v", err)
	}

	if updated.Status.Phase != "Error" {
		t.Errorf("expected Phase = 'Error', got %q", updated.Status.Phase)
	}
	if updated.Status.Message == "" {
		t.Error("expected a non-empty error message")
	}
	// Message should mention WireGuard
	if !containsString(updated.Status.Message, "WireGuard") && !containsString(updated.Status.Message, "wireguard") {
		t.Errorf("expected message to mention 'WireGuard', got %q", updated.Status.Message)
	}
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
