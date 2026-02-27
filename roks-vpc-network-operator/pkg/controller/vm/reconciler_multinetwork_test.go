package vm

import (
	"context"
	"encoding/json"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/IBM/roks-vpc-network-operator/pkg/annotations"
	"github.com/IBM/roks-vpc-network-operator/pkg/controller/network"
	"github.com/IBM/roks-vpc-network-operator/pkg/vpc"
)

func TestReconcile_SkipsVMWithNetworkInterfacesAnnotOnly(t *testing.T) {
	scheme := newTestScheme()

	interfaces := []network.VMNetworkInterface{
		{NetworkName: "layer2-net", Topology: "Layer2", InterfaceName: "net1"},
	}
	data, _ := json.Marshal(interfaces)

	vm := makeTestVM("layer2-vm", "default", map[string]string{
		annotations.NetworkInterfaces: string(data),
	}, false)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(vm).
		Build()

	mockVPC := vpc.NewMockClient()

	r := &Reconciler{
		Client:    fakeClient,
		Scheme:    scheme,
		VPC:       mockVPC,
		ClusterID: "cluster-abc",
	}

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "layer2-vm", Namespace: "default"},
	})

	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	// Should requeue for drift check
	if result.RequeueAfter == 0 {
		t.Error("expected periodic requeue for drift check")
	}

	// No VPC calls for Layer2-only VM
	if mockVPC.CallCount("GetVNI") != 0 {
		t.Error("GetVNI should not be called for Layer2-only VM")
	}
}

func TestReconcileDelete_MultiVNI(t *testing.T) {
	scheme := newTestScheme()

	interfaces := []network.VMNetworkInterface{
		{
			NetworkName:   "localnet1",
			Topology:      "LocalNet",
			InterfaceName: "net1",
			VNIID:         "vni-111",
			FIPID:         "fip-111",
		},
		{
			NetworkName:   "layer2net",
			Topology:      "Layer2",
			InterfaceName: "net2",
		},
		{
			NetworkName:   "localnet2",
			Topology:      "LocalNet",
			InterfaceName: "net3",
			VNIID:         "vni-222",
		},
	}
	data, _ := json.Marshal(interfaces)

	vm := &unstructured.Unstructured{}
	vm.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "kubevirt.io",
		Version: "v1",
		Kind:    "VirtualMachine",
	})
	vm.SetName("multi-net-vm")
	vm.SetNamespace("default")
	vm.SetAnnotations(map[string]string{
		annotations.NetworkInterfaces: string(data),
		annotations.VNIID:             "vni-111", // legacy annotation for first LocalNet
	})
	now := metav1.Now()
	vm.SetDeletionTimestamp(&now)
	vm.SetFinalizers([]string{"vpc.roks.ibm.com/vm-cleanup"})

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(vm).
		Build()

	deletedVNIs := map[string]bool{}
	deletedFIPs := map[string]bool{}

	mockVPC := vpc.NewMockClient()
	mockVPC.DeleteVNIFn = func(ctx context.Context, vniID string) error {
		deletedVNIs[vniID] = true
		return nil
	}
	mockVPC.DeleteFloatingIPFn = func(ctx context.Context, fipID string) error {
		deletedFIPs[fipID] = true
		return nil
	}

	r := &Reconciler{
		Client:    fakeClient,
		Scheme:    scheme,
		VPC:       mockVPC,
		ClusterID: "cluster-abc",
	}

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "multi-net-vm", Namespace: "default"},
	})

	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	// Verify both LocalNet VNIs were deleted
	if !deletedVNIs["vni-111"] {
		t.Error("expected VNI vni-111 to be deleted")
	}
	if !deletedVNIs["vni-222"] {
		t.Error("expected VNI vni-222 to be deleted")
	}

	// Verify FIP for first interface was deleted
	if !deletedFIPs["fip-111"] {
		t.Error("expected FIP fip-111 to be deleted")
	}

	// Layer2 interface should not trigger any VPC calls
	if mockVPC.CallCount("DeleteVNI") != 2 {
		t.Errorf("expected 2 DeleteVNI calls, got %d", mockVPC.CallCount("DeleteVNI"))
	}
	if mockVPC.CallCount("DeleteFloatingIP") != 1 {
		t.Errorf("expected 1 DeleteFloatingIP call, got %d", mockVPC.CallCount("DeleteFloatingIP"))
	}
}

func TestReconcile_DriftCheck_MultiVNI(t *testing.T) {
	scheme := newTestScheme()

	interfaces := []network.VMNetworkInterface{
		{
			NetworkName:   "localnet1",
			Topology:      "LocalNet",
			InterfaceName: "net1",
			VNIID:         "vni-111",
		},
		{
			NetworkName:   "localnet2",
			Topology:      "LocalNet",
			InterfaceName: "net2",
			VNIID:         "vni-222",
		},
	}
	data, _ := json.Marshal(interfaces)

	vm := makeTestVM("multi-vm", "default", map[string]string{
		annotations.NetworkInterfaces: string(data),
	}, false)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(vm).
		Build()

	mockVPC := vpc.NewMockClient()
	mockVPC.GetVNIFn = func(ctx context.Context, vniID string) (*vpc.VNI, error) {
		return &vpc.VNI{ID: vniID}, nil
	}

	r := &Reconciler{
		Client:    fakeClient,
		Scheme:    scheme,
		VPC:       mockVPC,
		ClusterID: "cluster-abc",
	}

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "multi-vm", Namespace: "default"},
	})

	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	if result.RequeueAfter == 0 {
		t.Error("expected periodic requeue for drift check")
	}

	// Both VNIs should be checked
	if mockVPC.CallCount("GetVNI") != 2 {
		t.Errorf("expected 2 GetVNI calls for drift check, got %d", mockVPC.CallCount("GetVNI"))
	}
}

func TestReconcile_BackwardsCompatibility_LegacyAnnotations(t *testing.T) {
	scheme := newTestScheme()

	// VM with only legacy annotations (no network-interfaces JSON)
	vm := makeTestVM("legacy-vm", "default", map[string]string{
		annotations.VNIID:      "vni-legacy",
		annotations.MACAddress: "fa:16:3e:aa:bb:cc",
	}, false)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(vm).
		Build()

	mockVPC := vpc.NewMockClient()
	mockVPC.GetVNIFn = func(ctx context.Context, vniID string) (*vpc.VNI, error) {
		return &vpc.VNI{ID: vniID}, nil
	}

	r := &Reconciler{
		Client:    fakeClient,
		Scheme:    scheme,
		VPC:       mockVPC,
		ClusterID: "cluster-abc",
	}

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "legacy-vm", Namespace: "default"},
	})

	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	if result.RequeueAfter == 0 {
		t.Error("expected periodic requeue for drift check")
	}

	// Legacy single VNI should be checked
	if mockVPC.CallCount("GetVNI") != 1 {
		t.Errorf("expected 1 GetVNI call for legacy VM, got %d", mockVPC.CallCount("GetVNI"))
	}
}
