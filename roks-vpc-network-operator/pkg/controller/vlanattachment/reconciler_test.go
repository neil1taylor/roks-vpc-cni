package vlanattachment

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

func TestReconcileNormal_CreateVLANAttachment(t *testing.T) {
	scheme := newTestScheme()

	vla := &v1alpha1.VLANAttachment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-vla",
			Namespace: "default",
		},
		Spec: v1alpha1.VLANAttachmentSpec{
			BMServerID: "bm-server-1",
			VLANID:     100,
			SubnetRef:  "my-subnet",
			SubnetID:   "subnet-123",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(vla).
		WithStatusSubresource(vla).
		Build()

	mockVPC := vpc.NewMockClient()
	mockVPC.CreateVLANAttachmentFn = func(ctx context.Context, opts vpc.CreateVLANAttachmentOptions) (*vpc.VLANAttachment, error) {
		if opts.BMServerID != "bm-server-1" {
			t.Errorf("expected BMServerID 'bm-server-1', got %q", opts.BMServerID)
		}
		if opts.VLANID != 100 {
			t.Errorf("expected VLANID 100, got %d", opts.VLANID)
		}
		return &vpc.VLANAttachment{
			ID:         "att-vpc-id-1",
			Name:       opts.Name,
			VLANID:     opts.VLANID,
			BMServerID: opts.BMServerID,
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
			Name:      "test-vla",
			Namespace: "default",
		},
	})

	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if result.RequeueAfter != 0 {
		t.Errorf("expected no requeue, got %v", result.RequeueAfter)
	}

	if mockVPC.CallCount("CreateVLANAttachment") != 1 {
		t.Errorf("expected CreateVLANAttachment to be called once, got %d", mockVPC.CallCount("CreateVLANAttachment"))
	}

	updated := &v1alpha1.VLANAttachment{}
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "test-vla", Namespace: "default"}, updated); err != nil {
		t.Fatalf("Failed to get updated VLANAttachment: %v", err)
	}

	if updated.Status.AttachmentID != "att-vpc-id-1" {
		t.Errorf("expected AttachmentID = 'att-vpc-id-1', got %q", updated.Status.AttachmentID)
	}
	if updated.Status.SyncStatus != "Synced" {
		t.Errorf("expected SyncStatus = 'Synced', got %q", updated.Status.SyncStatus)
	}
	if updated.Status.AttachmentStatus != "attached" {
		t.Errorf("expected AttachmentStatus = 'attached', got %q", updated.Status.AttachmentStatus)
	}
}

func TestReconcileNormal_ExistingAttachment(t *testing.T) {
	scheme := newTestScheme()

	vla := &v1alpha1.VLANAttachment{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "existing-vla",
			Namespace:  "default",
			Finalizers: []string{"vpc.roks.ibm.com/vlan-protection"},
		},
		Spec: v1alpha1.VLANAttachmentSpec{
			BMServerID: "bm-server-1",
			VLANID:     100,
			SubnetRef:  "my-subnet",
		},
		Status: v1alpha1.VLANAttachmentStatus{
			AttachmentID: "att-existing-id",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(vla).
		WithStatusSubresource(vla).
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
		NamespacedName: types.NamespacedName{Name: "existing-vla", Namespace: "default"},
	})

	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	if mockVPC.CallCount("CreateVLANAttachment") != 0 {
		t.Errorf("CreateVLANAttachment should not be called for existing attachment")
	}
}

func TestReconcileNormal_VPCError(t *testing.T) {
	scheme := newTestScheme()

	vla := &v1alpha1.VLANAttachment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "failing-vla",
			Namespace: "default",
		},
		Spec: v1alpha1.VLANAttachmentSpec{
			BMServerID: "bm-server-1",
			VLANID:     100,
			SubnetRef:  "my-subnet",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(vla).
		WithStatusSubresource(vla).
		Build()

	mockVPC := vpc.NewMockClient()
	mockVPC.CreateVLANAttachmentFn = func(ctx context.Context, opts vpc.CreateVLANAttachmentOptions) (*vpc.VLANAttachment, error) {
		return nil, fmt.Errorf("VPC API error: BM server not found")
	}

	r := &Reconciler{
		Client:    fakeClient,
		Scheme:    scheme,
		VPC:       mockVPC,
		ClusterID: "cluster-abc",
		Mode:      roks.ModeUnmanaged,
	}

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "failing-vla", Namespace: "default"},
	})

	if err != nil {
		t.Fatalf("Reconcile() should not return error for VPC failures, got %v", err)
	}
	if result.RequeueAfter == 0 {
		t.Error("expected requeue after VPC error")
	}

	updated := &v1alpha1.VLANAttachment{}
	_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: "failing-vla", Namespace: "default"}, updated)
	if updated.Status.SyncStatus != "Failed" {
		t.Errorf("expected SyncStatus = 'Failed', got %q", updated.Status.SyncStatus)
	}
}

func TestReconcileDelete(t *testing.T) {
	scheme := newTestScheme()

	now := metav1.Now()
	vla := &v1alpha1.VLANAttachment{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "deleting-vla",
			Namespace:         "default",
			DeletionTimestamp: &now,
			Finalizers:        []string{"vpc.roks.ibm.com/vlan-protection"},
		},
		Spec: v1alpha1.VLANAttachmentSpec{
			BMServerID: "bm-server-1",
			VLANID:     100,
			SubnetRef:  "my-subnet",
		},
		Status: v1alpha1.VLANAttachmentStatus{
			AttachmentID: "att-to-delete",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(vla).
		WithStatusSubresource(vla).
		Build()

	mockVPC := vpc.NewMockClient()
	mockVPC.DeleteVLANAttachmentFn = func(ctx context.Context, bmServerID, attachmentID string) error {
		if bmServerID != "bm-server-1" {
			t.Errorf("expected bmServerID 'bm-server-1', got %q", bmServerID)
		}
		if attachmentID != "att-to-delete" {
			t.Errorf("expected attachmentID 'att-to-delete', got %q", attachmentID)
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
		NamespacedName: types.NamespacedName{Name: "deleting-vla", Namespace: "default"},
	})

	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	if mockVPC.CallCount("DeleteVLANAttachment") != 1 {
		t.Errorf("expected DeleteVLANAttachment to be called once, got %d", mockVPC.CallCount("DeleteVLANAttachment"))
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

	vla := &v1alpha1.VLANAttachment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "roks-vla",
			Namespace: "default",
		},
		Spec: v1alpha1.VLANAttachmentSpec{
			BMServerID: "bm-server-1",
			VLANID:     100,
			SubnetRef:  "my-subnet",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(vla).
		WithStatusSubresource(vla).
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
		NamespacedName: types.NamespacedName{Name: "roks-vla", Namespace: "default"},
	})

	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if result.RequeueAfter == 0 {
		t.Error("expected requeue for ROKS mode")
	}

	updated := &v1alpha1.VLANAttachment{}
	_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: "roks-vla", Namespace: "default"}, updated)
	if updated.Status.SyncStatus != "Pending" {
		t.Errorf("expected SyncStatus = 'Pending', got %q", updated.Status.SyncStatus)
	}
}
