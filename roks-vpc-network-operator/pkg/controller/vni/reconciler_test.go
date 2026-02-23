package vni

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
	"github.com/IBM/roks-vpc-network-operator/pkg/roks"
	"github.com/IBM/roks-vpc-network-operator/pkg/vpc"
)

func newTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(s)
	_ = v1alpha1.AddToScheme(s)
	return s
}

func TestReconcileNormal_CreateVNI(t *testing.T) {
	scheme := newTestScheme()

	vniObj := &v1alpha1.VirtualNetworkInterface{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-vni",
			Namespace: "default",
		},
		Spec: v1alpha1.VNISpec{
			SubnetRef:        "my-subnet",
			SubnetID:         "subnet-123",
			SecurityGroupIDs: []string{"sg-1"},
			ClusterID:        "cluster-abc",
			VMRef: &v1alpha1.VMReference{
				Namespace: "default",
				Name:      "my-vm",
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(vniObj).
		WithStatusSubresource(vniObj).
		Build()

	mockVPC := vpc.NewMockClient()
	mockVPC.CreateVNIFn = func(ctx context.Context, opts vpc.CreateVNIOptions) (*vpc.VNI, error) {
		return &vpc.VNI{
			ID:         "vni-vpc-id-1",
			Name:       opts.Name,
			MACAddress: "fa:16:3e:aa:bb:cc",
			PrimaryIP: vpc.ReservedIP{
				ID:      "rip-123",
				Address: "10.240.0.5",
			},
		}, nil
	}

	r := &Reconciler{
		Client:    fakeClient,
		Scheme:    scheme,
		VPC:       mockVPC,
		ClusterID: "cluster-abc",
		Mode:      roks.ModeUnmanaged,
	}

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-vni",
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

	// Verify status was updated
	updated := &v1alpha1.VirtualNetworkInterface{}
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "test-vni", Namespace: "default"}, updated); err != nil {
		t.Fatalf("Failed to get updated VNI: %v", err)
	}

	if updated.Status.VNIID != "vni-vpc-id-1" {
		t.Errorf("expected VNIID = 'vni-vpc-id-1', got %q", updated.Status.VNIID)
	}
	if updated.Status.MACAddress != "fa:16:3e:aa:bb:cc" {
		t.Errorf("expected MACAddress = 'fa:16:3e:aa:bb:cc', got %q", updated.Status.MACAddress)
	}
	if updated.Status.PrimaryIPv4 != "10.240.0.5" {
		t.Errorf("expected PrimaryIPv4 = '10.240.0.5', got %q", updated.Status.PrimaryIPv4)
	}
	if updated.Status.SyncStatus != "Synced" {
		t.Errorf("expected SyncStatus = 'Synced', got %q", updated.Status.SyncStatus)
	}
}

func TestReconcileNormal_ExistingVNI(t *testing.T) {
	scheme := newTestScheme()

	vniObj := &v1alpha1.VirtualNetworkInterface{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "existing-vni",
			Namespace:  "default",
			Finalizers: []string{"vpc.roks.ibm.com/vni-protection"},
		},
		Spec: v1alpha1.VNISpec{
			SubnetRef: "my-subnet",
			SubnetID:  "subnet-123",
		},
		Status: v1alpha1.VNIStatus{
			VNIID:      "vni-existing-id",
			MACAddress: "fa:16:3e:11:22:33",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(vniObj).
		WithStatusSubresource(vniObj).
		Build()

	mockVPC := vpc.NewMockClient()

	r := &Reconciler{
		Client:    fakeClient,
		Scheme:    scheme,
		VPC:       mockVPC,
		ClusterID: "cluster-abc",
		Mode:      roks.ModeUnmanaged,
	}

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "existing-vni", Namespace: "default"},
	})

	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	// Should NOT create a new VNI since status.VNIID is already set
	if mockVPC.CallCount("CreateVNI") != 0 {
		t.Errorf("CreateVNI should not be called for existing VNI, got %d calls", mockVPC.CallCount("CreateVNI"))
	}
}

func TestReconcileNormal_VPCError(t *testing.T) {
	scheme := newTestScheme()

	vniObj := &v1alpha1.VirtualNetworkInterface{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "failing-vni",
			Namespace: "default",
		},
		Spec: v1alpha1.VNISpec{
			SubnetRef: "my-subnet",
			SubnetID:  "subnet-123",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(vniObj).
		WithStatusSubresource(vniObj).
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
		Mode:      roks.ModeUnmanaged,
	}

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "failing-vni", Namespace: "default"},
	})

	if err != nil {
		t.Fatalf("Reconcile() should not return error for VPC failures, got %v", err)
	}
	if result.RequeueAfter == 0 {
		t.Error("expected requeue after VPC error")
	}

	updated := &v1alpha1.VirtualNetworkInterface{}
	_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: "failing-vni", Namespace: "default"}, updated)
	if updated.Status.SyncStatus != "Failed" {
		t.Errorf("expected SyncStatus = 'Failed', got %q", updated.Status.SyncStatus)
	}
}

func TestReconcileDelete(t *testing.T) {
	scheme := newTestScheme()

	now := metav1.Now()
	vniObj := &v1alpha1.VirtualNetworkInterface{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "deleting-vni",
			Namespace:         "default",
			DeletionTimestamp: &now,
			Finalizers:        []string{"vpc.roks.ibm.com/vni-protection"},
		},
		Spec: v1alpha1.VNISpec{
			SubnetRef: "my-subnet",
		},
		Status: v1alpha1.VNIStatus{
			VNIID: "vni-to-delete",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(vniObj).
		WithStatusSubresource(vniObj).
		Build()

	mockVPC := vpc.NewMockClient()
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
		Mode:      roks.ModeUnmanaged,
	}

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "deleting-vni", Namespace: "default"},
	})

	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	if mockVPC.CallCount("DeleteVNI") != 1 {
		t.Errorf("expected DeleteVNI to be called once, got %d", mockVPC.CallCount("DeleteVNI"))
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
		Mode:      roks.ModeUnmanaged,
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

func TestReconcileROKS_Pending(t *testing.T) {
	scheme := newTestScheme()

	vniObj := &v1alpha1.VirtualNetworkInterface{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "roks-vni",
			Namespace: "default",
		},
		Spec: v1alpha1.VNISpec{
			SubnetRef: "my-subnet",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(vniObj).
		WithStatusSubresource(vniObj).
		Build()

	r := &Reconciler{
		Client:    fakeClient,
		Scheme:    scheme,
		VPC:       vpc.NewMockClient(),
		ROKS:      roks.NewStubClient("cluster-abc"),
		ClusterID: "cluster-abc",
		Mode:      roks.ModeROKS,
	}

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "roks-vni", Namespace: "default"},
	})

	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if result.RequeueAfter == 0 {
		t.Error("expected requeue for ROKS mode (periodic check)")
	}

	updated := &v1alpha1.VirtualNetworkInterface{}
	_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: "roks-vni", Namespace: "default"}, updated)
	if updated.Status.SyncStatus != "Pending" {
		t.Errorf("expected SyncStatus = 'Pending', got %q", updated.Status.SyncStatus)
	}
}
