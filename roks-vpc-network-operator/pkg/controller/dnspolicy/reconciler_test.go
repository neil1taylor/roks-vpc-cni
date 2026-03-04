package dnspolicy

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
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

func TestReconcile_CreatesConfigMap(t *testing.T) {
	scheme := newTestScheme()

	router := &v1alpha1.VPCRouter{
		ObjectMeta: metav1.ObjectMeta{Name: "my-router", Namespace: "default"},
		Spec: v1alpha1.VPCRouterSpec{
			Gateway:  "gw1",
			Networks: []v1alpha1.RouterNetwork{{Name: "net-a", Address: "10.0.0.1/24"}},
		},
	}

	policy := &v1alpha1.VPCDNSPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-dns", Namespace: "default"},
		Spec: v1alpha1.VPCDNSPolicySpec{
			RouterRef: "my-router",
			Upstream:  &v1alpha1.DNSUpstreamConfig{Servers: []v1alpha1.DNSUpstreamServer{{URL: "https://cloudflare-dns.com/dns-query"}}},
			Filtering: &v1alpha1.DNSFilteringConfig{Enabled: true},
		},
	}

	fc := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(router, policy).
		WithStatusSubresource(policy).
		Build()

	r := &Reconciler{Client: fc, Scheme: scheme}
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "test-dns", Namespace: "default"},
	})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	cm := &corev1.ConfigMap{}
	if err := fc.Get(context.Background(), types.NamespacedName{Name: "test-dns-adguard-config", Namespace: "default"}, cm); err != nil {
		t.Fatalf("ConfigMap not created: %v", err)
	}
	if _, ok := cm.Data["AdGuardHome.yaml"]; !ok {
		t.Error("ConfigMap missing AdGuardHome.yaml key")
	}
}

func TestReconcile_InvalidRouterRef(t *testing.T) {
	scheme := newTestScheme()

	policy := &v1alpha1.VPCDNSPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "bad-ref", Namespace: "default"},
		Spec:       v1alpha1.VPCDNSPolicySpec{RouterRef: "nonexistent"},
	}

	fc := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(policy).
		WithStatusSubresource(policy).
		Build()

	r := &Reconciler{Client: fc, Scheme: scheme}
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "bad-ref", Namespace: "default"},
	})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	updated := &v1alpha1.VPCDNSPolicy{}
	_ = fc.Get(context.Background(), types.NamespacedName{Name: "bad-ref", Namespace: "default"}, updated)
	if updated.Status.Phase != "Error" {
		t.Errorf("expected phase Error, got %q", updated.Status.Phase)
	}
}

func TestReconcile_DeleteCleansUp(t *testing.T) {
	scheme := newTestScheme()

	now := metav1.Now()
	policy := &v1alpha1.VPCDNSPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "del-dns", Namespace: "default",
			DeletionTimestamp: &now,
			Finalizers:        []string{finalizerName},
		},
		Spec: v1alpha1.VPCDNSPolicySpec{RouterRef: "my-router"},
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "del-dns-adguard-config", Namespace: "default"},
	}

	fc := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(policy, cm).
		WithStatusSubresource(policy).
		Build()

	r := &Reconciler{Client: fc, Scheme: scheme}
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "del-dns", Namespace: "default"},
	})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	if err := fc.Get(context.Background(), types.NamespacedName{Name: "del-dns-adguard-config", Namespace: "default"}, &corev1.ConfigMap{}); err == nil {
		t.Error("ConfigMap should have been deleted")
	}
}

func TestConfigMapName(t *testing.T) {
	if got := configMapName("my-policy"); got != "my-policy-adguard-config" {
		t.Errorf("configMapName() = %q, want %q", got, "my-policy-adguard-config")
	}
}
