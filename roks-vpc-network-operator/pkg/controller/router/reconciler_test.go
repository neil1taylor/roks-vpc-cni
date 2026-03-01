package router

import (
	"context"
	"testing"

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

// TestReconcileNormal_CreateRouter tests the happy path: gateway exists and is Ready,
// router connects to L2 networks, status is updated correctly.
func TestReconcileNormal_CreateRouter(t *testing.T) {
	scheme := newTestScheme()

	gw := &v1alpha1.VPCGateway{
		ObjectMeta: metav1.ObjectMeta{Name: "gw-test", Namespace: "default"},
		Status: v1alpha1.VPCGatewayStatus{
			Phase:          "Ready",
			TransitNetwork: "gw-test-transit",
		},
	}

	router := &v1alpha1.VPCRouter{
		ObjectMeta: metav1.ObjectMeta{Name: "router-test", Namespace: "default"},
		Spec: v1alpha1.VPCRouterSpec{
			Gateway: "gw-test",
			Transit: &v1alpha1.RouterTransit{Address: "172.16.0.2/24"},
			Networks: []v1alpha1.RouterNetwork{
				{Name: "l2-app", Address: "10.100.0.1/24"},
				{Name: "l2-db", Address: "10.200.0.1/24"},
			},
			RouteAdvertisement: &v1alpha1.RouteAdvertisement{ConnectedSegments: true},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(gw, router).
		WithStatusSubresource(gw, router).
		Build()

	r := &Reconciler{
		Client: fakeClient,
		Scheme: scheme,
	}

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "router-test", Namespace: "default"},
	})

	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if result.RequeueAfter != 0 {
		t.Errorf("expected no requeue, got %v", result.RequeueAfter)
	}

	// Verify the updated router status
	updated := &v1alpha1.VPCRouter{}
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "router-test", Namespace: "default"}, updated); err != nil {
		t.Fatalf("Failed to get updated VPCRouter: %v", err)
	}

	// Phase should be Ready
	if updated.Status.Phase != "Ready" {
		t.Errorf("expected Phase = 'Ready', got %q", updated.Status.Phase)
	}

	// SyncStatus should be Synced
	if updated.Status.SyncStatus != "Synced" {
		t.Errorf("expected SyncStatus = 'Synced', got %q", updated.Status.SyncStatus)
	}

	// TransitIP should be the IP stripped of prefix
	if updated.Status.TransitIP != "172.16.0.2" {
		t.Errorf("expected TransitIP = '172.16.0.2', got %q", updated.Status.TransitIP)
	}

	// Networks should have 2 entries, all connected
	if len(updated.Status.Networks) != 2 {
		t.Fatalf("expected 2 network statuses, got %d", len(updated.Status.Networks))
	}
	for i, ns := range updated.Status.Networks {
		if !ns.Connected {
			t.Errorf("expected network %d (%s) to be connected", i, ns.Name)
		}
	}
	if updated.Status.Networks[0].Name != "l2-app" {
		t.Errorf("expected network[0].Name = 'l2-app', got %q", updated.Status.Networks[0].Name)
	}
	if updated.Status.Networks[0].Address != "10.100.0.1/24" {
		t.Errorf("expected network[0].Address = '10.100.0.1/24', got %q", updated.Status.Networks[0].Address)
	}
	if updated.Status.Networks[1].Name != "l2-db" {
		t.Errorf("expected network[1].Name = 'l2-db', got %q", updated.Status.Networks[1].Name)
	}

	// AdvertisedRoutes should contain CIDRs derived from spec.networks addresses
	if len(updated.Status.AdvertisedRoutes) != 2 {
		t.Fatalf("expected 2 advertised routes, got %d", len(updated.Status.AdvertisedRoutes))
	}
	if updated.Status.AdvertisedRoutes[0] != "10.100.0.0/24" {
		t.Errorf("expected advertisedRoutes[0] = '10.100.0.0/24', got %q", updated.Status.AdvertisedRoutes[0])
	}
	if updated.Status.AdvertisedRoutes[1] != "10.200.0.0/24" {
		t.Errorf("expected advertisedRoutes[1] = '10.200.0.0/24', got %q", updated.Status.AdvertisedRoutes[1])
	}

	// Conditions should include TransitConnected, RoutesConfigured, PodReady
	conditionMap := make(map[string]metav1.Condition)
	for _, c := range updated.Status.Conditions {
		conditionMap[c.Type] = c
	}
	for _, cType := range []string{"TransitConnected", "RoutesConfigured", "PodReady"} {
		c, ok := conditionMap[cType]
		if !ok {
			t.Errorf("expected condition %q to be present", cType)
			continue
		}
		if c.Status != metav1.ConditionTrue {
			t.Errorf("expected condition %q status = True, got %v", cType, c.Status)
		}
	}
}

// TestReconcileNormal_GatewayNotReady tests that when the referenced gateway
// is not Ready, the reconciler requeues and sets Failed status.
func TestReconcileNormal_GatewayNotReady(t *testing.T) {
	scheme := newTestScheme()

	gw := &v1alpha1.VPCGateway{
		ObjectMeta: metav1.ObjectMeta{Name: "gw-pending", Namespace: "default"},
		Status: v1alpha1.VPCGatewayStatus{
			Phase: "Pending",
		},
	}

	router := &v1alpha1.VPCRouter{
		ObjectMeta: metav1.ObjectMeta{Name: "router-wait", Namespace: "default"},
		Spec: v1alpha1.VPCRouterSpec{
			Gateway: "gw-pending",
			Transit: &v1alpha1.RouterTransit{Address: "172.16.0.2/24"},
			Networks: []v1alpha1.RouterNetwork{
				{Name: "l2-app", Address: "10.100.0.1/24"},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(gw, router).
		WithStatusSubresource(gw, router).
		Build()

	r := &Reconciler{
		Client: fakeClient,
		Scheme: scheme,
	}

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "router-wait", Namespace: "default"},
	})

	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if result.RequeueAfter == 0 {
		t.Error("expected requeue when gateway is not ready")
	}

	updated := &v1alpha1.VPCRouter{}
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "router-wait", Namespace: "default"}, updated); err != nil {
		t.Fatalf("Failed to get updated VPCRouter: %v", err)
	}

	if updated.Status.SyncStatus != "Failed" {
		t.Errorf("expected SyncStatus = 'Failed', got %q", updated.Status.SyncStatus)
	}
	if updated.Status.Phase != "Error" {
		t.Errorf("expected Phase = 'Error', got %q", updated.Status.Phase)
	}
}

// TestReconcileNormal_GatewayNotFound tests that when the referenced gateway
// doesn't exist, the reconciler requeues and sets Failed status.
func TestReconcileNormal_GatewayNotFound(t *testing.T) {
	scheme := newTestScheme()

	router := &v1alpha1.VPCRouter{
		ObjectMeta: metav1.ObjectMeta{Name: "router-orphan", Namespace: "default"},
		Spec: v1alpha1.VPCRouterSpec{
			Gateway: "gw-missing",
			Transit: &v1alpha1.RouterTransit{Address: "172.16.0.2/24"},
			Networks: []v1alpha1.RouterNetwork{
				{Name: "l2-app", Address: "10.100.0.1/24"},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(router).
		WithStatusSubresource(router).
		Build()

	r := &Reconciler{
		Client: fakeClient,
		Scheme: scheme,
	}

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "router-orphan", Namespace: "default"},
	})

	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if result.RequeueAfter == 0 {
		t.Error("expected requeue when gateway is not found")
	}

	updated := &v1alpha1.VPCRouter{}
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "router-orphan", Namespace: "default"}, updated); err != nil {
		t.Fatalf("Failed to get updated VPCRouter: %v", err)
	}

	if updated.Status.SyncStatus != "Failed" {
		t.Errorf("expected SyncStatus = 'Failed', got %q", updated.Status.SyncStatus)
	}
	if updated.Status.Phase != "Error" {
		t.Errorf("expected Phase = 'Error', got %q", updated.Status.Phase)
	}
}

// TestReconcileDelete tests that when a VPCRouter has a DeletionTimestamp set,
// the reconciler removes the finalizer.
func TestReconcileDelete(t *testing.T) {
	scheme := newTestScheme()

	now := metav1.Now()
	router := &v1alpha1.VPCRouter{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "router-delete",
			Namespace:         "default",
			DeletionTimestamp: &now,
			Finalizers:        []string{"vpc.roks.ibm.com/router-cleanup"},
		},
		Spec: v1alpha1.VPCRouterSpec{
			Gateway: "gw-test",
			Networks: []v1alpha1.RouterNetwork{
				{Name: "l2-app", Address: "10.100.0.1/24"},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(router).
		WithStatusSubresource(router).
		Build()

	r := &Reconciler{
		Client: fakeClient,
		Scheme: scheme,
	}

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "router-delete", Namespace: "default"},
	})

	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if result.Requeue {
		t.Error("should not requeue after successful delete")
	}

	// Verify finalizer was removed.
	// With the fake client, removing the last finalizer from an object with a
	// DeletionTimestamp causes the object to be garbage-collected, so a
	// not-found error is the expected successful outcome.
	updated := &v1alpha1.VPCRouter{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{Name: "router-delete", Namespace: "default"}, updated)
	if err != nil {
		// Object was fully deleted (expected with fake client when last finalizer is removed)
		if !errors.IsNotFound(err) {
			t.Fatalf("unexpected error getting VPCRouter after delete: %v", err)
		}
	} else {
		// If the object still exists, verify the finalizer was removed
		for _, f := range updated.Finalizers {
			if f == "vpc.roks.ibm.com/router-cleanup" {
				t.Error("expected finalizer to be removed after deletion")
			}
		}
	}
}

// TestReconcile_NotFound tests that when the VPCRouter resource is gone,
// the reconciler returns no error and does not requeue.
func TestReconcile_NotFound(t *testing.T) {
	scheme := newTestScheme()

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	r := &Reconciler{
		Client: fakeClient,
		Scheme: scheme,
	}

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "nonexistent", Namespace: "default"},
	})

	if err != nil {
		t.Fatalf("Reconcile() error = %v for not-found resource", err)
	}
	if result.Requeue {
		t.Error("should not requeue for not-found resource")
	}
}
