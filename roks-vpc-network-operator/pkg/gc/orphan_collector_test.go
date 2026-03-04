package gc

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1alpha1 "github.com/IBM/roks-vpc-network-operator/api/v1alpha1"
	"github.com/IBM/roks-vpc-network-operator/pkg/vpc"
)

func TestParseVNIName(t *testing.T) {
	tests := []struct {
		name      string
		vniName   string
		clusterID string
		wantNS    string
		wantVM    string
	}{
		{
			name:      "valid VNI name",
			vniName:   "roks-cluster-abc-default-my-vm",
			clusterID: "cluster-abc",
			wantNS:    "default",
			wantVM:    "my-vm",
		},
		{
			name:      "empty name",
			vniName:   "",
			clusterID: "cluster-abc",
			wantNS:    "",
			wantVM:    "",
		},
		{
			name:      "name too short",
			vniName:   "roks-cluster-abc-",
			clusterID: "cluster-abc",
			wantNS:    "",
			wantVM:    "",
		},
		{
			name:      "wrong prefix",
			vniName:   "other-prefix-default-my-vm",
			clusterID: "cluster-abc",
			wantNS:    "",
			wantVM:    "",
		},
		{
			name:      "no namespace separator",
			vniName:   "roks-cluster-abc-singleword",
			clusterID: "cluster-abc",
			wantNS:    "",
			wantVM:    "",
		},
		{
			name:      "different cluster",
			vniName:   "roks-other-cluster-ns-vm",
			clusterID: "other-cluster",
			wantNS:    "ns",
			wantVM:    "vm",
		},
		{
			name:      "multi-network VNI name",
			vniName:   "roks-cluster-abc-default-my-vm-localnet1",
			clusterID: "cluster-abc",
			wantNS:    "default",
			wantVM:    "my-vm-localnet1",
		},
		{
			name:      "multi-network VNI with dashes",
			vniName:   "roks-cluster-abc-myns-web-server-prod-net",
			clusterID: "cluster-abc",
			wantNS:    "myns",
			wantVM:    "web-server-prod-net",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ns, vm := parseVNIName(tt.vniName, tt.clusterID)
			if ns != tt.wantNS {
				t.Errorf("parseVNIName(%q, %q) namespace = %q, want %q", tt.vniName, tt.clusterID, ns, tt.wantNS)
			}
			if vm != tt.wantVM {
				t.Errorf("parseVNIName(%q, %q) vmName = %q, want %q", tt.vniName, tt.clusterID, vm, tt.wantVM)
			}
		})
	}
}

func newTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(s)
	return s
}

func TestCollectOrphanedFloatingIPs(t *testing.T) {
	scheme := newTestScheme()

	// Create a gateway that references fip-1
	gw := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "vpc.roks.ibm.com/v1alpha1",
			"kind":       "VPCGateway",
			"metadata": map[string]interface{}{
				"name":      "gw-1",
				"namespace": "default",
			},
			"status": map[string]interface{}{
				"floatingIPID": "fip-1",
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(gw).
		Build()

	mockVPC := vpc.NewMockClient()
	mockVPC.ListFloatingIPsFn = func(ctx context.Context) ([]vpc.FloatingIP, error) {
		return []vpc.FloatingIP{
			{ID: "fip-1", Name: "roks-cluster-abc-gw-gw1-fip", Address: "1.2.3.4"},
			{ID: "fip-orphan", Name: "roks-cluster-abc-gw-old-fip", Address: "5.6.7.8"},
			{ID: "fip-other", Name: "unrelated-fip", Address: "9.9.9.9"}, // not our prefix
		}, nil
	}
	var deletedFIPs []string
	mockVPC.DeleteFloatingIPFn = func(ctx context.Context, fipID string) error {
		deletedFIPs = append(deletedFIPs, fipID)
		return nil
	}

	collector := &OrphanCollector{
		K8sClient: fakeClient,
		VPC:       mockVPC,
		ClusterID: "cluster-abc",
		VPCID:     "vpc-123",
	}

	count := collector.collectOrphanedFloatingIPs(context.Background(), testLogger())
	if count != 1 {
		t.Errorf("expected 1 orphaned FIP deleted, got %d", count)
	}
	if len(deletedFIPs) != 1 || deletedFIPs[0] != "fip-orphan" {
		t.Errorf("expected to delete fip-orphan, got %v", deletedFIPs)
	}
}

func TestCollectOrphanedFloatingIPs_VMFIPsPreserved(t *testing.T) {
	scheme := newTestScheme()

	// VM with legacy fip-id annotation
	vmLegacy := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kubevirt.io/v1",
			"kind":       "VirtualMachine",
			"metadata": map[string]interface{}{
				"name":      "vm-legacy",
				"namespace": "vm-demo",
				"annotations": map[string]interface{}{
					"vpc.roks.ibm.com/fip-id": "fip-vm-legacy",
				},
			},
		},
	}

	// VM with multi-network annotation containing FIP
	vmMulti := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kubevirt.io/v1",
			"kind":       "VirtualMachine",
			"metadata": map[string]interface{}{
				"name":      "vm-multi",
				"namespace": "vm-demo",
				"annotations": map[string]interface{}{
					"vpc.roks.ibm.com/network-interfaces": `[{"networkName":"localnet","fipId":"fip-vm-multi"}]`,
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(vmLegacy, vmMulti).
		Build()

	mockVPC := vpc.NewMockClient()
	mockVPC.ListFloatingIPsFn = func(ctx context.Context) ([]vpc.FloatingIP, error) {
		return []vpc.FloatingIP{
			{ID: "fip-vm-legacy", Name: "roks-cluster-abc-vm-demo-vm-legacy-fip", Address: "1.2.3.4"},
			{ID: "fip-vm-multi", Name: "roks-cluster-abc-vm-demo-vm-multi-fip", Address: "5.6.7.8"},
			{ID: "fip-orphan", Name: "roks-cluster-abc-vm-demo-deleted-vm-fip", Address: "9.9.9.9"},
		}, nil
	}
	var deletedFIPs []string
	mockVPC.DeleteFloatingIPFn = func(ctx context.Context, fipID string) error {
		deletedFIPs = append(deletedFIPs, fipID)
		return nil
	}

	collector := &OrphanCollector{
		K8sClient: fakeClient,
		VPC:       mockVPC,
		ClusterID: "cluster-abc",
		VPCID:     "vpc-123",
	}

	count := collector.collectOrphanedFloatingIPs(context.Background(), testLogger())
	if count != 1 {
		t.Errorf("expected 1 orphaned FIP deleted, got %d", count)
	}
	if len(deletedFIPs) != 1 || deletedFIPs[0] != "fip-orphan" {
		t.Errorf("expected to delete only fip-orphan, got %v", deletedFIPs)
	}
}

func TestCollectOrphanedPARs(t *testing.T) {
	scheme := newTestScheme()

	gw := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "vpc.roks.ibm.com/v1alpha1",
			"kind":       "VPCGateway",
			"metadata": map[string]interface{}{
				"name":      "gw-1",
				"namespace": "default",
			},
			"status": map[string]interface{}{
				"publicAddressRangeID": "par-1",
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(gw).
		Build()

	mockVPC := vpc.NewMockClient()
	mockVPC.ListPublicAddressRangesFn = func(ctx context.Context, vpcID string) ([]vpc.PublicAddressRange, error) {
		return []vpc.PublicAddressRange{
			{ID: "par-1", Name: "roks-cluster-abc-gw-gw1-par", CIDR: "150.240.68.0/28"},
			{ID: "par-orphan", Name: "roks-cluster-abc-gw-old-par", CIDR: "150.240.69.0/28"},
		}, nil
	}
	var deletedPARs []string
	mockVPC.DeletePublicAddressRangeFn = func(ctx context.Context, parID string) error {
		deletedPARs = append(deletedPARs, parID)
		return nil
	}

	collector := &OrphanCollector{
		K8sClient: fakeClient,
		VPC:       mockVPC,
		ClusterID: "cluster-abc",
		VPCID:     "vpc-123",
	}

	count := collector.collectOrphanedPARs(context.Background(), testLogger())
	if count != 1 {
		t.Errorf("expected 1 orphaned PAR deleted, got %d", count)
	}
	if len(deletedPARs) != 1 || deletedPARs[0] != "par-orphan" {
		t.Errorf("expected to delete par-orphan, got %v", deletedPARs)
	}
}

func TestCollectOrphanedVPCRoutes(t *testing.T) {
	scheme := newTestScheme()

	gw := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "vpc.roks.ibm.com/v1alpha1",
			"kind":       "VPCGateway",
			"metadata": map[string]interface{}{
				"name":      "gw-1",
				"namespace": "default",
			},
			"status": map[string]interface{}{
				"vpcRouteIDs": []interface{}{"route-1"},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(gw).
		Build()

	mockVPC := vpc.NewMockClient()
	mockVPC.ListRoutingTablesFn = func(ctx context.Context, vpcID string) ([]vpc.RoutingTable, error) {
		return []vpc.RoutingTable{{ID: "rt-default", IsDefault: true}}, nil
	}
	mockVPC.ListRoutesFn = func(ctx context.Context, vpcID, routingTableID string) ([]vpc.Route, error) {
		return []vpc.Route{
			{ID: "route-1", Name: "roks-cluster-abc-gw-gw1-10-100", Destination: "10.100.0.0/24"},
			{ID: "route-orphan", Name: "roks-cluster-abc-gw-old-10-200", Destination: "10.200.0.0/24"},
			{ID: "route-other", Name: "unrelated-route", Destination: "10.0.0.0/8"}, // not our prefix
		}, nil
	}
	var deletedRoutes []string
	mockVPC.DeleteRouteFn = func(ctx context.Context, vpcID, routingTableID, routeID string) error {
		deletedRoutes = append(deletedRoutes, routeID)
		return nil
	}

	collector := &OrphanCollector{
		K8sClient: fakeClient,
		VPC:       mockVPC,
		ClusterID: "cluster-abc",
		VPCID:     "vpc-123",
	}

	count := collector.collectOrphanedVPCRoutes(context.Background(), testLogger())
	if count != 1 {
		t.Errorf("expected 1 orphaned route deleted, got %d", count)
	}
	if len(deletedRoutes) != 1 || deletedRoutes[0] != "route-orphan" {
		t.Errorf("expected to delete route-orphan, got %v", deletedRoutes)
	}
}

func testLogger() logr.Logger {
	// We need the logr import for the test logger
	return logr.Discard()
}

// Ensure the logr import is available
var _ = schema.GroupVersionKind{}
