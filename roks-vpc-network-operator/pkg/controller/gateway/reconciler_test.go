package gateway

import (
	"context"
	"fmt"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1alpha1 "github.com/IBM/roks-vpc-network-operator/api/v1alpha1"
	"github.com/IBM/roks-vpc-network-operator/pkg/vpc"
)

func newTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(s)
	_ = v1alpha1.AddToScheme(s)
	return s
}

func TestReconcileNormal_CreateGateway(t *testing.T) {
	scheme := newTestScheme()

	gw := &v1alpha1.VPCGateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-gw",
			Namespace: "default",
		},
		Spec: v1alpha1.VPCGatewaySpec{
			Zone: "eu-de-1",
			Uplink: v1alpha1.GatewayUplink{
				Network: "uplink-net",
			},
			Transit: v1alpha1.GatewayTransit{
				Address: "192.168.100.1",
			},
			VPCRoutes: []v1alpha1.VPCRouteSpec{
				{Destination: "10.100.0.0/24"},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(gw).
		WithStatusSubresource(gw).
		Build()

	mockVPC := vpc.NewMockClient()
	mockVPC.CreateVNIFn = func(ctx context.Context, opts vpc.CreateVNIOptions) (*vpc.VNI, error) {
		if opts.Name != "roks-cluster-abc-gw-test-gw" {
			t.Errorf("expected VNI name 'roks-cluster-abc-gw-test-gw', got %q", opts.Name)
		}
		return &vpc.VNI{
			ID:   "vni-gw-123",
			Name: opts.Name,
			PrimaryIP: vpc.ReservedIP{
				ID:      "rip-gw-1",
				Address: "10.240.1.5",
			},
		}, nil
	}
	mockVPC.ListRoutingTablesFn = func(ctx context.Context, vpcID string) ([]vpc.RoutingTable, error) {
		return []vpc.RoutingTable{
			{ID: "rt-default", Name: "default", IsDefault: true},
		}, nil
	}
	mockVPC.ListRoutesFn = func(ctx context.Context, vpcID, routingTableID string) ([]vpc.Route, error) {
		return []vpc.Route{}, nil
	}
	mockVPC.CreateRouteFn = func(ctx context.Context, vpcID, routingTableID string, opts vpc.CreateRouteOptions) (*vpc.Route, error) {
		if opts.Destination != "10.100.0.0/24" {
			t.Errorf("expected route destination '10.100.0.0/24', got %q", opts.Destination)
		}
		if opts.NextHopIP != "10.240.1.5" {
			t.Errorf("expected nextHop '10.240.1.5', got %q", opts.NextHopIP)
		}
		if opts.Action != "deliver" {
			t.Errorf("expected action 'deliver', got %q", opts.Action)
		}
		return &vpc.Route{
			ID:          "route-1",
			Name:        opts.Name,
			Destination: opts.Destination,
			NextHop:     opts.NextHopIP,
			Action:      opts.Action,
		}, nil
	}

	r := &Reconciler{
		Client:    fakeClient,
		Scheme:    scheme,
		VPC:       mockVPC,
		ClusterID: "cluster-abc",
		VPCID:     "vpc-123",
	}

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-gw",
			Namespace: "default",
		},
	})

	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if result.RequeueAfter != 0 {
		t.Errorf("expected no requeue, got %v", result.RequeueAfter)
	}

	if mockVPC.CallCount("CreateVNI") != 1 {
		t.Errorf("expected CreateVNI to be called once, got %d", mockVPC.CallCount("CreateVNI"))
	}
	if mockVPC.CallCount("CreateRoute") != 1 {
		t.Errorf("expected CreateRoute to be called once, got %d", mockVPC.CallCount("CreateRoute"))
	}

	updated := &v1alpha1.VPCGateway{}
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "test-gw", Namespace: "default"}, updated); err != nil {
		t.Fatalf("Failed to get updated VPCGateway: %v", err)
	}

	if updated.Status.VNIID != "vni-gw-123" {
		t.Errorf("expected VNIID = 'vni-gw-123', got %q", updated.Status.VNIID)
	}
	if updated.Status.ReservedIP != "10.240.1.5" {
		t.Errorf("expected ReservedIP = '10.240.1.5', got %q", updated.Status.ReservedIP)
	}
	if updated.Status.SyncStatus != "Synced" {
		t.Errorf("expected SyncStatus = 'Synced', got %q", updated.Status.SyncStatus)
	}
	if updated.Status.Phase != "Ready" {
		t.Errorf("expected Phase = 'Ready', got %q", updated.Status.Phase)
	}
	if len(updated.Status.VPCRouteIDs) != 1 || updated.Status.VPCRouteIDs[0] != "route-1" {
		t.Errorf("expected VPCRouteIDs = ['route-1'], got %v", updated.Status.VPCRouteIDs)
	}
}

func TestReconcileNormal_ExistingVNI(t *testing.T) {
	scheme := newTestScheme()

	gw := &v1alpha1.VPCGateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "existing-gw",
			Namespace:  "default",
			Finalizers: []string{"vpc.roks.ibm.com/gateway-cleanup"},
		},
		Spec: v1alpha1.VPCGatewaySpec{
			Zone: "eu-de-1",
			Uplink: v1alpha1.GatewayUplink{
				Network: "uplink-net",
			},
			Transit: v1alpha1.GatewayTransit{
				Address: "192.168.100.1",
			},
		},
		Status: v1alpha1.VPCGatewayStatus{
			VNIID:      "vni-existing",
			ReservedIP: "10.240.1.10",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(gw).
		WithStatusSubresource(gw).
		Build()

	mockVPC := vpc.NewMockClient()
	mockVPC.ListRoutingTablesFn = func(ctx context.Context, vpcID string) ([]vpc.RoutingTable, error) {
		return []vpc.RoutingTable{
			{ID: "rt-default", Name: "default", IsDefault: true},
		}, nil
	}
	mockVPC.ListRoutesFn = func(ctx context.Context, vpcID, routingTableID string) ([]vpc.Route, error) {
		return []vpc.Route{}, nil
	}

	r := &Reconciler{
		Client:    fakeClient,
		Scheme:    scheme,
		VPC:       mockVPC,
		ClusterID: "cluster-abc",
		VPCID:     "vpc-123",
	}

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "existing-gw", Namespace: "default"},
	})

	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	if mockVPC.CallCount("CreateVNI") != 0 {
		t.Errorf("CreateVNI should not be called for existing VNI, got %d calls", mockVPC.CallCount("CreateVNI"))
	}
}

func TestReconcileNormal_VPCError(t *testing.T) {
	scheme := newTestScheme()

	gw := &v1alpha1.VPCGateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "failing-gw",
			Namespace: "default",
		},
		Spec: v1alpha1.VPCGatewaySpec{
			Zone: "eu-de-1",
			Uplink: v1alpha1.GatewayUplink{
				Network: "uplink-net",
			},
			Transit: v1alpha1.GatewayTransit{
				Address: "192.168.100.1",
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(gw).
		WithStatusSubresource(gw).
		Build()

	mockVPC := vpc.NewMockClient()
	mockVPC.CreateVNIFn = func(ctx context.Context, opts vpc.CreateVNIOptions) (*vpc.VNI, error) {
		return nil, fmt.Errorf("VPC API error: quota exceeded")
	}

	r := &Reconciler{
		Client:    fakeClient,
		Scheme:    scheme,
		VPC:       mockVPC,
		ClusterID: "cluster-abc",
		VPCID:     "vpc-123",
	}

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "failing-gw", Namespace: "default"},
	})

	if err != nil {
		t.Fatalf("Reconcile() should not return error for VPC failures, got %v", err)
	}
	if result.RequeueAfter == 0 {
		t.Error("expected requeue after VPC error")
	}

	updated := &v1alpha1.VPCGateway{}
	_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: "failing-gw", Namespace: "default"}, updated)
	if updated.Status.SyncStatus != "Failed" {
		t.Errorf("expected SyncStatus = 'Failed', got %q", updated.Status.SyncStatus)
	}
	if updated.Status.Phase != "Error" {
		t.Errorf("expected Phase = 'Error', got %q", updated.Status.Phase)
	}
}

func TestReconcileDelete(t *testing.T) {
	scheme := newTestScheme()

	now := metav1.Now()
	gw := &v1alpha1.VPCGateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "deleting-gw",
			Namespace:         "default",
			DeletionTimestamp: &now,
			Finalizers:        []string{"vpc.roks.ibm.com/gateway-cleanup"},
		},
		Spec: v1alpha1.VPCGatewaySpec{
			Zone: "eu-de-1",
			Uplink: v1alpha1.GatewayUplink{
				Network: "uplink-net",
			},
			Transit: v1alpha1.GatewayTransit{
				Address: "192.168.100.1",
			},
		},
		Status: v1alpha1.VPCGatewayStatus{
			VNIID:       "vni-gw-delete",
			ReservedIP:  "10.240.1.5",
			VPCRouteIDs: []string{"route-1", "route-2"},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(gw).
		WithStatusSubresource(gw).
		Build()

	mockVPC := vpc.NewMockClient()
	mockVPC.ListRoutingTablesFn = func(ctx context.Context, vpcID string) ([]vpc.RoutingTable, error) {
		return []vpc.RoutingTable{
			{ID: "rt-default", Name: "default", IsDefault: true},
		}, nil
	}
	mockVPC.DeleteRouteFn = func(ctx context.Context, vpcID, routingTableID, routeID string) error {
		if routingTableID != "rt-default" {
			t.Errorf("expected routing table 'rt-default', got %q", routingTableID)
		}
		return nil
	}
	mockVPC.DeleteVNIFn = func(ctx context.Context, vniID string) error {
		if vniID != "vni-gw-delete" {
			t.Errorf("expected to delete VNI 'vni-gw-delete', got %q", vniID)
		}
		return nil
	}

	r := &Reconciler{
		Client:    fakeClient,
		Scheme:    scheme,
		VPC:       mockVPC,
		ClusterID: "cluster-abc",
		VPCID:     "vpc-123",
	}

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "deleting-gw", Namespace: "default"},
	})

	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	if mockVPC.CallCount("DeleteVNI") != 1 {
		t.Errorf("expected DeleteVNI to be called once, got %d", mockVPC.CallCount("DeleteVNI"))
	}
	if mockVPC.CallCount("DeleteRoute") != 2 {
		t.Errorf("expected DeleteRoute to be called twice, got %d", mockVPC.CallCount("DeleteRoute"))
	}
}

func TestReconcile_NotFound(t *testing.T) {
	scheme := newTestScheme()

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	r := &Reconciler{
		Client:    fakeClient,
		Scheme:    scheme,
		VPC:       vpc.NewMockClient(),
		ClusterID: "cluster-abc",
		VPCID:     "vpc-123",
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
