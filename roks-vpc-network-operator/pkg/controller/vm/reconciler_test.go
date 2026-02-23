package vm

import (
	"context"
	"fmt"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1alpha1 "github.com/IBM/roks-vpc-network-operator/api/v1alpha1"
	"github.com/IBM/roks-vpc-network-operator/pkg/annotations"
	"github.com/IBM/roks-vpc-network-operator/pkg/vpc"
)

func newTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(s)
	_ = v1alpha1.AddToScheme(s)
	return s
}

func makeTestVM(name, namespace string, annots map[string]string, deleted bool) *unstructured.Unstructured {
	vm := &unstructured.Unstructured{}
	vm.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "kubevirt.io",
		Version: "v1",
		Kind:    "VirtualMachine",
	})
	vm.SetName(name)
	vm.SetNamespace(namespace)
	vm.SetAnnotations(annots)
	if deleted {
		now := metav1.Now()
		vm.SetDeletionTimestamp(&now)
		vm.SetFinalizers([]string{"vpc.roks.ibm.com/vm-cleanup"})
	}
	return vm
}

func TestReconcile_SkipsUnmanagedVM(t *testing.T) {
	scheme := newTestScheme()

	vm := makeTestVM("my-vm", "default", nil, false)

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

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "my-vm", Namespace: "default"},
	})

	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	// No VPC calls should have been made for an unmanaged VM
	if mockVPC.CallCount("GetVNI") != 0 {
		t.Error("GetVNI should not be called for unmanaged VM")
	}
}

func TestReconcile_DriftCheck(t *testing.T) {
	scheme := newTestScheme()

	vm := makeTestVM("my-vm", "default", map[string]string{
		annotations.VNIID:      "vni-123",
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
		NamespacedName: types.NamespacedName{Name: "my-vm", Namespace: "default"},
	})

	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	if result.RequeueAfter == 0 {
		t.Error("expected periodic requeue for drift check")
	}

	if mockVPC.CallCount("GetVNI") != 1 {
		t.Errorf("expected GetVNI to be called once, got %d", mockVPC.CallCount("GetVNI"))
	}
}

func TestReconcile_DriftDetected(t *testing.T) {
	scheme := newTestScheme()

	vm := makeTestVM("my-vm", "default", map[string]string{
		annotations.VNIID:      "vni-missing",
		annotations.MACAddress: "fa:16:3e:aa:bb:cc",
	}, false)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(vm).
		Build()

	mockVPC := vpc.NewMockClient()
	mockVPC.GetVNIFn = func(ctx context.Context, vniID string) (*vpc.VNI, error) {
		return nil, fmt.Errorf("VNI not found")
	}

	r := &Reconciler{
		Client:    fakeClient,
		Scheme:    scheme,
		VPC:       mockVPC,
		ClusterID: "cluster-abc",
	}

	// Should not error, just log the drift
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "my-vm", Namespace: "default"},
	})

	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
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
	}

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "nonexistent", Namespace: "default"},
	})

	if err != nil {
		t.Fatalf("Reconcile() error = %v for not-found VM", err)
	}
	if result.Requeue {
		t.Error("should not requeue for not-found VM")
	}
}

func TestReconcileDelete(t *testing.T) {
	scheme := newTestScheme()

	vm := makeTestVM("deleting-vm", "default", map[string]string{
		annotations.VNIID: "vni-to-delete",
		annotations.FIPID: "fip-to-delete",
	}, true)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(vm).
		Build()

	mockVPC := vpc.NewMockClient()
	mockVPC.DeleteFloatingIPFn = func(ctx context.Context, fipID string) error {
		if fipID != "fip-to-delete" {
			t.Errorf("expected to delete 'fip-to-delete', got %q", fipID)
		}
		return nil
	}
	mockVPC.DeleteVNIFn = func(ctx context.Context, vniID string) error {
		if vniID != "vni-to-delete" {
			t.Errorf("expected to delete 'vni-to-delete', got %q", vniID)
		}
		return nil
	}

	r := &Reconciler{
		Client:    fakeClient,
		Scheme:    scheme,
		VPC:       mockVPC,
		ClusterID: "cluster-abc",
	}

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "deleting-vm", Namespace: "default"},
	})

	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	if mockVPC.CallCount("DeleteFloatingIP") != 1 {
		t.Errorf("expected DeleteFloatingIP once, got %d", mockVPC.CallCount("DeleteFloatingIP"))
	}
	if mockVPC.CallCount("DeleteVNI") != 1 {
		t.Errorf("expected DeleteVNI once, got %d", mockVPC.CallCount("DeleteVNI"))
	}
}
