package gateway

import (
	"context"
	"fmt"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

func makeCUDN(name, subnetID, vlanID string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "k8s.ovn.org",
		Version: "v1",
		Kind:    "ClusterUserDefinedNetwork",
	})
	obj.SetName(name)
	obj.SetAnnotations(map[string]string{
		"vpc.roks.ibm.com/subnet-id": subnetID,
		"vpc.roks.ibm.com/vlan-id":   vlanID,
	})
	return obj
}

func makeBMNode(name, providerID string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       corev1.NodeSpec{ProviderID: providerID},
	}
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

	uplinkCUDN := makeCUDN("uplink-net", "subnet-uplink-123", "100")
	bmNode := makeBMNode("bm-node-1", "ibm://acct/eu-de/eu-de-1/bm-server-123")

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(gw, uplinkCUDN, bmNode).
		WithStatusSubresource(gw).
		Build()

	mockVPC := vpc.NewMockClient()
	mockVPC.CreateVMAttachmentFn = func(ctx context.Context, opts vpc.CreateVMAttachmentOptions) (*vpc.VMAttachmentResult, error) {
		if opts.BMServerID != "bm-server-123" {
			t.Errorf("expected BMServerID 'bm-server-123', got %q", opts.BMServerID)
		}
		if opts.SubnetID != "subnet-uplink-123" {
			t.Errorf("expected SubnetID 'subnet-uplink-123', got %q", opts.SubnetID)
		}
		if opts.VLANID != 100 {
			t.Errorf("expected VLANID 100, got %d", opts.VLANID)
		}
		return &vpc.VMAttachmentResult{
			AttachmentID: "att-gw-123",
			BMServerID:   "bm-server-123",
			VNI: vpc.VNI{
				ID:         "vni-gw-123",
				Name:       opts.VNIName,
				MACAddress: "fa:16:3e:aa:bb:cc",
				PrimaryIP: vpc.ReservedIP{
					ID:      "rip-gw-1",
					Address: "10.240.1.5",
				},
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
	if result.RequeueAfter != 5*time.Minute {
		t.Errorf("expected 5m requeue for drift detection, got %v", result.RequeueAfter)
	}

	if mockVPC.CallCount("CreateVMAttachment") != 1 {
		t.Errorf("expected CreateVMAttachment to be called once, got %d", mockVPC.CallCount("CreateVMAttachment"))
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
	if updated.Status.AttachmentID != "att-gw-123" {
		t.Errorf("expected AttachmentID = 'att-gw-123', got %q", updated.Status.AttachmentID)
	}
	if updated.Status.BMServerID != "bm-server-123" {
		t.Errorf("expected BMServerID = 'bm-server-123', got %q", updated.Status.BMServerID)
	}
	if updated.Status.MACAddress != "fa:16:3e:aa:bb:cc" {
		t.Errorf("expected MACAddress = 'fa:16:3e:aa:bb:cc', got %q", updated.Status.MACAddress)
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
			VNIID:        "vni-existing",
			AttachmentID: "att-existing",
			BMServerID:   "bm-existing",
			ReservedIP:   "10.240.1.10",
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

	if mockVPC.CallCount("CreateVMAttachment") != 0 {
		t.Errorf("CreateVMAttachment should not be called for existing VNI, got %d calls", mockVPC.CallCount("CreateVMAttachment"))
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

	uplinkCUDN := makeCUDN("uplink-net", "subnet-uplink-123", "100")
	bmNode := makeBMNode("bm-node-1", "ibm://acct/eu-de/eu-de-1/bm-server-123")

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(gw, uplinkCUDN, bmNode).
		WithStatusSubresource(gw).
		Build()

	mockVPC := vpc.NewMockClient()
	mockVPC.CreateVMAttachmentFn = func(ctx context.Context, opts vpc.CreateVMAttachmentOptions) (*vpc.VMAttachmentResult, error) {
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
			VNIID:        "vni-gw-delete",
			AttachmentID: "att-gw-delete",
			BMServerID:   "bm-server-123",
			ReservedIP:   "10.240.1.5",
			VPCRouteIDs:  []string{"route-1", "route-2"},
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
	mockVPC.DeleteVLANAttachmentFn = func(ctx context.Context, bmServerID, attachmentID string) error {
		if bmServerID != "bm-server-123" {
			t.Errorf("expected bmServerID 'bm-server-123', got %q", bmServerID)
		}
		if attachmentID != "att-gw-delete" {
			t.Errorf("expected attachmentID 'att-gw-delete', got %q", attachmentID)
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

	if mockVPC.CallCount("DeleteVLANAttachment") != 1 {
		t.Errorf("expected DeleteVLANAttachment to be called once, got %d", mockVPC.CallCount("DeleteVLANAttachment"))
	}
	if mockVPC.CallCount("DeleteRoute") != 2 {
		t.Errorf("expected DeleteRoute to be called twice, got %d", mockVPC.CallCount("DeleteRoute"))
	}
}

func TestReconcileNormal_WithPAR(t *testing.T) {
	scheme := newTestScheme()

	gw := &v1alpha1.VPCGateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "par-gw",
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
			PublicAddressRange: &v1alpha1.GatewayPublicAddressRange{
				Enabled:      true,
				PrefixLength: 28,
			},
		},
		Status: v1alpha1.VPCGatewayStatus{
			// Pre-populated VNI (skip VNI creation step)
			VNIID:        "vni-par-123",
			AttachmentID: "att-par-123",
			BMServerID:   "bm-server-123",
			MACAddress:   "fa:16:3e:aa:bb:cc",
			ReservedIP:   "10.240.1.5",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(gw).
		WithStatusSubresource(gw).
		Build()

	mockVPC := vpc.NewMockClient()

	// Existing VNI drift check
	mockVPC.GetVNIFn = func(ctx context.Context, vniID string) (*vpc.VNI, error) {
		return &vpc.VNI{
			ID:         "vni-par-123",
			MACAddress: "fa:16:3e:aa:bb:cc",
			PrimaryIP:  vpc.ReservedIP{Address: "10.240.1.5"},
		}, nil
	}

	// Routing table for egress routes
	mockVPC.ListRoutingTablesFn = func(ctx context.Context, vpcID string) ([]vpc.RoutingTable, error) {
		return []vpc.RoutingTable{
			{ID: "rt-default", Name: "default", IsDefault: true},
		}, nil
	}
	mockVPC.ListRoutesFn = func(ctx context.Context, vpcID, routingTableID string) ([]vpc.Route, error) {
		return []vpc.Route{}, nil
	}

	// PAR creation
	mockVPC.CreatePublicAddressRangeFn = func(ctx context.Context, opts vpc.CreatePublicAddressRangeOptions) (*vpc.PublicAddressRange, error) {
		if opts.PrefixLength != 28 {
			t.Errorf("expected PrefixLength 28, got %d", opts.PrefixLength)
		}
		if opts.Zone != "eu-de-1" {
			t.Errorf("expected Zone 'eu-de-1', got %q", opts.Zone)
		}
		return &vpc.PublicAddressRange{
			ID:   "par-123",
			CIDR: "150.240.68.0/28",
			Zone: "eu-de-1",
		}, nil
	}

	// Ingress routing table creation
	mockVPC.CreateRoutingTableFn = func(ctx context.Context, vpcID string, opts vpc.CreateRoutingTableOptions) (*vpc.RoutingTable, error) {
		if !opts.RouteInternetIngress {
			t.Error("expected RouteInternetIngress=true for ingress routing table")
		}
		return &vpc.RoutingTable{
			ID:   "rt-ingress-123",
			Name: opts.Name,
		}, nil
	}

	// Ingress route creation
	var createdIngressRoute bool
	mockVPC.CreateRouteFn = func(ctx context.Context, vpcID, routingTableID string, opts vpc.CreateRouteOptions) (*vpc.Route, error) {
		if routingTableID == "rt-ingress-123" {
			createdIngressRoute = true
			if opts.Destination != "150.240.68.0/28" {
				t.Errorf("expected ingress route destination '150.240.68.0/28', got %q", opts.Destination)
			}
			if opts.NextHopIP != "10.240.1.5" {
				t.Errorf("expected ingress route nextHop '10.240.1.5', got %q", opts.NextHopIP)
			}
			return &vpc.Route{
				ID:          "route-ingress-1",
				Destination: opts.Destination,
				NextHop:     opts.NextHopIP,
			}, nil
		}
		return &vpc.Route{ID: "route-egress-1", Destination: opts.Destination}, nil
	}

	r := &Reconciler{
		Client:    fakeClient,
		Scheme:    scheme,
		VPC:       mockVPC,
		ClusterID: "cluster-abc",
		VPCID:     "vpc-123",
	}

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "par-gw", Namespace: "default"},
	})

	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if result.RequeueAfter != 5*time.Minute {
		t.Errorf("expected 5m requeue, got %v", result.RequeueAfter)
	}

	// Verify PAR was created
	if mockVPC.CallCount("CreatePublicAddressRange") != 1 {
		t.Errorf("expected CreatePublicAddressRange called once, got %d", mockVPC.CallCount("CreatePublicAddressRange"))
	}
	if mockVPC.CallCount("CreateRoutingTable") != 1 {
		t.Errorf("expected CreateRoutingTable called once, got %d", mockVPC.CallCount("CreateRoutingTable"))
	}
	if !createdIngressRoute {
		t.Error("expected ingress route to be created")
	}

	// Verify status
	updated := &v1alpha1.VPCGateway{}
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "par-gw", Namespace: "default"}, updated); err != nil {
		t.Fatalf("Failed to get updated VPCGateway: %v", err)
	}

	if updated.Status.PublicAddressRangeID != "par-123" {
		t.Errorf("expected PublicAddressRangeID 'par-123', got %q", updated.Status.PublicAddressRangeID)
	}
	if updated.Status.PublicAddressRangeCIDR != "150.240.68.0/28" {
		t.Errorf("expected PublicAddressRangeCIDR '150.240.68.0/28', got %q", updated.Status.PublicAddressRangeCIDR)
	}
	if updated.Status.IngressRoutingTableID != "rt-ingress-123" {
		t.Errorf("expected IngressRoutingTableID 'rt-ingress-123', got %q", updated.Status.IngressRoutingTableID)
	}
	if len(updated.Status.IngressRouteIDs) != 1 || updated.Status.IngressRouteIDs[0] != "route-ingress-1" {
		t.Errorf("expected IngressRouteIDs ['route-ingress-1'], got %v", updated.Status.IngressRouteIDs)
	}
	if updated.Status.Phase != "Ready" {
		t.Errorf("expected Phase 'Ready', got %q", updated.Status.Phase)
	}
}

func TestReconcileNormal_WithExternalPAR(t *testing.T) {
	scheme := newTestScheme()

	gw := &v1alpha1.VPCGateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "ext-par-gw",
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
			PublicAddressRange: &v1alpha1.GatewayPublicAddressRange{
				Enabled: true,
				ID:      "par-external-456", // externally managed
			},
		},
		Status: v1alpha1.VPCGatewayStatus{
			VNIID:        "vni-ext-123",
			AttachmentID: "att-ext-123",
			BMServerID:   "bm-server-123",
			MACAddress:   "fa:16:3e:dd:ee:ff",
			ReservedIP:   "10.240.1.10",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(gw).
		WithStatusSubresource(gw).
		Build()

	mockVPC := vpc.NewMockClient()
	mockVPC.GetVNIFn = func(ctx context.Context, vniID string) (*vpc.VNI, error) {
		return &vpc.VNI{
			ID:         "vni-ext-123",
			MACAddress: "fa:16:3e:dd:ee:ff",
			PrimaryIP:  vpc.ReservedIP{Address: "10.240.1.10"},
		}, nil
	}
	mockVPC.ListRoutingTablesFn = func(ctx context.Context, vpcID string) ([]vpc.RoutingTable, error) {
		return []vpc.RoutingTable{{ID: "rt-default", IsDefault: true}}, nil
	}
	mockVPC.ListRoutesFn = func(ctx context.Context, vpcID, routingTableID string) ([]vpc.Route, error) {
		return []vpc.Route{}, nil
	}

	// External PAR adoption (Get instead of Create)
	mockVPC.GetPublicAddressRangeFn = func(ctx context.Context, parID string) (*vpc.PublicAddressRange, error) {
		if parID != "par-external-456" {
			t.Errorf("expected parID 'par-external-456', got %q", parID)
		}
		return &vpc.PublicAddressRange{
			ID:   "par-external-456",
			CIDR: "203.0.113.0/29",
			Zone: "eu-de-1",
		}, nil
	}
	mockVPC.CreateRoutingTableFn = func(ctx context.Context, vpcID string, opts vpc.CreateRoutingTableOptions) (*vpc.RoutingTable, error) {
		return &vpc.RoutingTable{ID: "rt-ingress-ext"}, nil
	}
	mockVPC.CreateRouteFn = func(ctx context.Context, vpcID, routingTableID string, opts vpc.CreateRouteOptions) (*vpc.Route, error) {
		return &vpc.Route{ID: "route-ext-1", Destination: opts.Destination}, nil
	}

	r := &Reconciler{
		Client:    fakeClient,
		Scheme:    scheme,
		VPC:       mockVPC,
		ClusterID: "cluster-abc",
		VPCID:     "vpc-123",
	}

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "ext-par-gw", Namespace: "default"},
	})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	// Should NOT have called CreatePublicAddressRange (external PAR)
	if mockVPC.CallCount("CreatePublicAddressRange") != 0 {
		t.Error("should not create PAR for externally-managed PAR")
	}
	// Should have called GetPublicAddressRange to adopt
	if mockVPC.CallCount("GetPublicAddressRange") != 1 {
		t.Errorf("expected GetPublicAddressRange called once, got %d", mockVPC.CallCount("GetPublicAddressRange"))
	}

	updated := &v1alpha1.VPCGateway{}
	_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: "ext-par-gw", Namespace: "default"}, updated)
	if updated.Status.PublicAddressRangeID != "par-external-456" {
		t.Errorf("expected PublicAddressRangeID 'par-external-456', got %q", updated.Status.PublicAddressRangeID)
	}
	if updated.Status.PublicAddressRangeCIDR != "203.0.113.0/29" {
		t.Errorf("expected CIDR '203.0.113.0/29', got %q", updated.Status.PublicAddressRangeCIDR)
	}
}

func TestReconcileDelete_WithPAR(t *testing.T) {
	scheme := newTestScheme()

	now := metav1.Now()
	gw := &v1alpha1.VPCGateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "delete-par-gw",
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
			PublicAddressRange: &v1alpha1.GatewayPublicAddressRange{
				Enabled:      true,
				PrefixLength: 28,
			},
		},
		Status: v1alpha1.VPCGatewayStatus{
			VNIID:                  "vni-par-del",
			AttachmentID:           "att-par-del",
			BMServerID:             "bm-server-123",
			ReservedIP:             "10.240.1.5",
			VPCRouteIDs:            []string{"route-1"},
			PublicAddressRangeID:   "par-del-123",
			PublicAddressRangeCIDR: "150.240.68.0/28",
			IngressRoutingTableID:  "rt-ingress-del",
			IngressRouteIDs:        []string{"iroute-1"},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(gw).
		WithStatusSubresource(gw).
		Build()

	mockVPC := vpc.NewMockClient()
	mockVPC.ListRoutingTablesFn = func(ctx context.Context, vpcID string) ([]vpc.RoutingTable, error) {
		return []vpc.RoutingTable{{ID: "rt-default", IsDefault: true}}, nil
	}
	mockVPC.DeleteRouteFn = func(ctx context.Context, vpcID, routingTableID, routeID string) error {
		return nil
	}
	mockVPC.DeleteVLANAttachmentFn = func(ctx context.Context, bmServerID, attachmentID string) error {
		return nil
	}
	mockVPC.DeleteRoutingTableFn = func(ctx context.Context, vpcID, routingTableID string) error {
		if routingTableID != "rt-ingress-del" {
			t.Errorf("expected routing table 'rt-ingress-del', got %q", routingTableID)
		}
		return nil
	}
	mockVPC.DeletePublicAddressRangeFn = func(ctx context.Context, parID string) error {
		if parID != "par-del-123" {
			t.Errorf("expected parID 'par-del-123', got %q", parID)
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
		NamespacedName: types.NamespacedName{Name: "delete-par-gw", Namespace: "default"},
	})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	// Verify ingress routes deleted
	if mockVPC.CallCount("DeleteRoute") != 2 { // 1 ingress + 1 egress
		t.Errorf("expected 2 DeleteRoute calls (1 ingress + 1 egress), got %d", mockVPC.CallCount("DeleteRoute"))
	}
	// Verify ingress routing table deleted
	if mockVPC.CallCount("DeleteRoutingTable") != 1 {
		t.Errorf("expected DeleteRoutingTable called once, got %d", mockVPC.CallCount("DeleteRoutingTable"))
	}
	// Verify PAR deleted (not externally managed)
	if mockVPC.CallCount("DeletePublicAddressRange") != 1 {
		t.Errorf("expected DeletePublicAddressRange called once, got %d", mockVPC.CallCount("DeletePublicAddressRange"))
	}
	// Verify VLAN attachment also deleted
	if mockVPC.CallCount("DeleteVLANAttachment") != 1 {
		t.Errorf("expected DeleteVLANAttachment called once, got %d", mockVPC.CallCount("DeleteVLANAttachment"))
	}
}

func TestReconcileDelete_ExternalPARNotDeleted(t *testing.T) {
	scheme := newTestScheme()

	now := metav1.Now()
	gw := &v1alpha1.VPCGateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "delete-ext-par-gw",
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
			PublicAddressRange: &v1alpha1.GatewayPublicAddressRange{
				Enabled: true,
				ID:      "par-external-789", // externally managed
			},
		},
		Status: v1alpha1.VPCGatewayStatus{
			VNIID:                  "vni-ext-del",
			AttachmentID:           "att-ext-del",
			BMServerID:             "bm-server-123",
			ReservedIP:             "10.240.1.10",
			PublicAddressRangeID:   "par-external-789",
			PublicAddressRangeCIDR: "203.0.113.0/29",
			IngressRoutingTableID:  "rt-ingress-ext",
			IngressRouteIDs:        []string{"iroute-ext-1"},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(gw).
		WithStatusSubresource(gw).
		Build()

	mockVPC := vpc.NewMockClient()
	mockVPC.DeleteRouteFn = func(ctx context.Context, vpcID, routingTableID, routeID string) error {
		return nil
	}
	mockVPC.DeleteRoutingTableFn = func(ctx context.Context, vpcID, routingTableID string) error {
		return nil
	}
	mockVPC.DeleteVLANAttachmentFn = func(ctx context.Context, bmServerID, attachmentID string) error {
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
		NamespacedName: types.NamespacedName{Name: "delete-ext-par-gw", Namespace: "default"},
	})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	// PAR should NOT be deleted (externally managed)
	if mockVPC.CallCount("DeletePublicAddressRange") != 0 {
		t.Error("should not delete externally-managed PAR")
	}
	// But ingress routes and routing table should still be deleted
	if mockVPC.CallCount("DeleteRoute") != 1 {
		t.Errorf("expected 1 DeleteRoute call (ingress route), got %d", mockVPC.CallCount("DeleteRoute"))
	}
	if mockVPC.CallCount("DeleteRoutingTable") != 1 {
		t.Errorf("expected DeleteRoutingTable called once, got %d", mockVPC.CallCount("DeleteRoutingTable"))
	}
}

// TestReconcileNormal_WithAdvertisedRoutes tests that the gateway collects
// advertised routes from routers that reference it and creates VPC routes.
func TestReconcileNormal_WithAdvertisedRoutes(t *testing.T) {
	scheme := newTestScheme()

	gw := &v1alpha1.VPCGateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "adv-gw",
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
			// No explicit vpcRoutes — routes come from the router
		},
		Status: v1alpha1.VPCGatewayStatus{
			VNIID:        "vni-adv-123",
			AttachmentID: "att-adv-123",
			BMServerID:   "bm-server-123",
			MACAddress:   "fa:16:3e:aa:bb:cc",
			ReservedIP:   "10.240.1.5",
		},
	}

	// Router with connectedSegments route advertisement
	router := &v1alpha1.VPCRouter{
		ObjectMeta: metav1.ObjectMeta{Name: "rt-adv", Namespace: "default"},
		Spec: v1alpha1.VPCRouterSpec{
			Gateway: "adv-gw",
			Networks: []v1alpha1.RouterNetwork{
				{Name: "l2-app", Address: "10.100.0.1/24"},
				{Name: "l2-db", Address: "10.200.0.1/24"},
			},
			RouteAdvertisement: &v1alpha1.RouteAdvertisement{ConnectedSegments: true},
		},
		Status: v1alpha1.VPCRouterStatus{
			Phase:            "Ready",
			AdvertisedRoutes: []string{"10.100.0.0/24", "10.200.0.0/24"},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(gw, router).
		WithStatusSubresource(gw, router).
		Build()

	mockVPC := vpc.NewMockClient()
	mockVPC.GetVNIFn = func(ctx context.Context, vniID string) (*vpc.VNI, error) {
		return &vpc.VNI{
			ID:         "vni-adv-123",
			MACAddress: "fa:16:3e:aa:bb:cc",
			PrimaryIP:  vpc.ReservedIP{Address: "10.240.1.5"},
		}, nil
	}
	mockVPC.ListRoutingTablesFn = func(ctx context.Context, vpcID string) ([]vpc.RoutingTable, error) {
		return []vpc.RoutingTable{{ID: "rt-default", IsDefault: true}}, nil
	}
	mockVPC.ListRoutesFn = func(ctx context.Context, vpcID, routingTableID string) ([]vpc.Route, error) {
		return []vpc.Route{}, nil
	}

	var createdDests []string
	mockVPC.CreateRouteFn = func(ctx context.Context, vpcID, routingTableID string, opts vpc.CreateRouteOptions) (*vpc.Route, error) {
		createdDests = append(createdDests, opts.Destination)
		return &vpc.Route{
			ID:          fmt.Sprintf("route-%s", sanitizeDestination(opts.Destination)),
			Destination: opts.Destination,
			NextHop:     opts.NextHopIP,
		}, nil
	}

	r := &Reconciler{
		Client:    fakeClient,
		Scheme:    scheme,
		VPC:       mockVPC,
		ClusterID: "cluster-abc",
		VPCID:     "vpc-123",
	}

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "adv-gw", Namespace: "default"},
	})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	// Should have created routes for both advertised CIDRs
	if len(createdDests) != 2 {
		t.Fatalf("expected 2 routes created from advertised routes, got %d: %v", len(createdDests), createdDests)
	}

	updated := &v1alpha1.VPCGateway{}
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "adv-gw", Namespace: "default"}, updated); err != nil {
		t.Fatalf("Failed to get updated VPCGateway: %v", err)
	}
	if len(updated.Status.VPCRouteIDs) != 2 {
		t.Errorf("expected 2 VPCRouteIDs, got %d: %v", len(updated.Status.VPCRouteIDs), updated.Status.VPCRouteIDs)
	}
	if updated.Status.Phase != "Ready" {
		t.Errorf("expected Phase 'Ready', got %q", updated.Status.Phase)
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
