package floatingip

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

func TestReconcileNormal_CreateFloatingIP(t *testing.T) {
	scheme := newTestScheme()

	fip := &v1alpha1.FloatingIP{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-fip",
			Namespace: "default",
		},
		Spec: v1alpha1.FloatingIPSpec{
			Zone:  "us-south-1",
			VNIID: "vni-123",
			Name:  "my-fip",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(fip).
		WithStatusSubresource(fip).
		Build()

	mockVPC := vpc.NewMockClient()
	mockVPC.CreateFloatingIPFn = func(ctx context.Context, opts vpc.CreateFloatingIPOptions) (*vpc.FloatingIP, error) {
		if opts.Zone != "us-south-1" {
			t.Errorf("expected Zone 'us-south-1', got %q", opts.Zone)
		}
		if opts.VNIID != "vni-123" {
			t.Errorf("expected VNIID 'vni-123', got %q", opts.VNIID)
		}
		return &vpc.FloatingIP{
			ID:      "fip-vpc-id-1",
			Name:    opts.Name,
			Address: "169.48.100.5",
			Zone:    opts.Zone,
			Target:  opts.VNIID,
		}, nil
	}

	r := &Reconciler{
		Client:    fakeClient,
		Scheme:    scheme,
		VPC:       mockVPC,
		ClusterID: "cluster-abc",
	}

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-fip",
			Namespace: "default",
		},
	})

	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if result.RequeueAfter != 0 {
		t.Errorf("expected no requeue, got %v", result.RequeueAfter)
	}

	if mockVPC.CallCount("CreateFloatingIP") != 1 {
		t.Errorf("expected CreateFloatingIP to be called once, got %d", mockVPC.CallCount("CreateFloatingIP"))
	}

	updated := &v1alpha1.FloatingIP{}
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "test-fip", Namespace: "default"}, updated); err != nil {
		t.Fatalf("Failed to get updated FloatingIP: %v", err)
	}

	if updated.Status.FIPID != "fip-vpc-id-1" {
		t.Errorf("expected FIPID = 'fip-vpc-id-1', got %q", updated.Status.FIPID)
	}
	if updated.Status.Address != "169.48.100.5" {
		t.Errorf("expected Address = '169.48.100.5', got %q", updated.Status.Address)
	}
	if updated.Status.TargetVNIID != "vni-123" {
		t.Errorf("expected TargetVNIID = 'vni-123', got %q", updated.Status.TargetVNIID)
	}
	if updated.Status.SyncStatus != "Synced" {
		t.Errorf("expected SyncStatus = 'Synced', got %q", updated.Status.SyncStatus)
	}
}

func TestReconcileNormal_ExistingFIP(t *testing.T) {
	scheme := newTestScheme()

	fip := &v1alpha1.FloatingIP{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "existing-fip",
			Namespace:  "default",
			Finalizers: []string{"vpc.roks.ibm.com/fip-protection"},
		},
		Spec: v1alpha1.FloatingIPSpec{
			Zone:  "us-south-1",
			VNIID: "vni-123",
		},
		Status: v1alpha1.FloatingIPStatus{
			FIPID:   "fip-existing-id",
			Address: "169.48.100.10",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(fip).
		WithStatusSubresource(fip).
		Build()

	mockVPC := vpc.NewMockClient()

	r := &Reconciler{
		Client:    fakeClient,
		Scheme:    scheme,
		VPC:       mockVPC,
		ClusterID: "cluster-abc",
	}

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "existing-fip", Namespace: "default"},
	})

	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	if mockVPC.CallCount("CreateFloatingIP") != 0 {
		t.Errorf("CreateFloatingIP should not be called for existing FIP")
	}
}

func TestReconcileNormal_AutoNameGeneration(t *testing.T) {
	scheme := newTestScheme()

	fip := &v1alpha1.FloatingIP{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "unnamed-fip",
			Namespace: "default",
		},
		Spec: v1alpha1.FloatingIPSpec{
			Zone:  "us-south-1",
			VNIID: "vni-123",
			// Name is intentionally empty to test auto-generation
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(fip).
		WithStatusSubresource(fip).
		Build()

	var capturedName string
	mockVPC := vpc.NewMockClient()
	mockVPC.CreateFloatingIPFn = func(ctx context.Context, opts vpc.CreateFloatingIPOptions) (*vpc.FloatingIP, error) {
		capturedName = opts.Name
		return &vpc.FloatingIP{
			ID:      "fip-auto-1",
			Name:    opts.Name,
			Address: "169.48.100.20",
			Zone:    opts.Zone,
			Target:  opts.VNIID,
		}, nil
	}

	r := &Reconciler{
		Client:    fakeClient,
		Scheme:    scheme,
		VPC:       mockVPC,
		ClusterID: "cluster-abc",
	}

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "unnamed-fip", Namespace: "default"},
	})

	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	expected := "roks-cluster-abc-unnamed-fip"
	if capturedName != expected {
		t.Errorf("expected auto-generated name %q, got %q", expected, capturedName)
	}
}

func TestReconcileNormal_VPCError(t *testing.T) {
	scheme := newTestScheme()

	fip := &v1alpha1.FloatingIP{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "failing-fip",
			Namespace: "default",
		},
		Spec: v1alpha1.FloatingIPSpec{
			Zone:  "us-south-1",
			VNIID: "vni-123",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(fip).
		WithStatusSubresource(fip).
		Build()

	mockVPC := vpc.NewMockClient()
	mockVPC.CreateFloatingIPFn = func(ctx context.Context, opts vpc.CreateFloatingIPOptions) (*vpc.FloatingIP, error) {
		return nil, fmt.Errorf("VPC API error: quota exceeded")
	}

	r := &Reconciler{
		Client:    fakeClient,
		Scheme:    scheme,
		VPC:       mockVPC,
		ClusterID: "cluster-abc",
	}

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "failing-fip", Namespace: "default"},
	})

	if err != nil {
		t.Fatalf("Reconcile() should not return error for VPC failures, got %v", err)
	}
	if result.RequeueAfter == 0 {
		t.Error("expected requeue after VPC error")
	}

	updated := &v1alpha1.FloatingIP{}
	_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: "failing-fip", Namespace: "default"}, updated)
	if updated.Status.SyncStatus != "Failed" {
		t.Errorf("expected SyncStatus = 'Failed', got %q", updated.Status.SyncStatus)
	}
}

func TestReconcileDelete(t *testing.T) {
	scheme := newTestScheme()

	now := metav1.Now()
	fip := &v1alpha1.FloatingIP{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "deleting-fip",
			Namespace:         "default",
			DeletionTimestamp: &now,
			Finalizers:        []string{"vpc.roks.ibm.com/fip-protection"},
		},
		Spec: v1alpha1.FloatingIPSpec{
			Zone: "us-south-1",
		},
		Status: v1alpha1.FloatingIPStatus{
			FIPID: "fip-to-delete",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(fip).
		WithStatusSubresource(fip).
		Build()

	mockVPC := vpc.NewMockClient()
	mockVPC.DeleteFloatingIPFn = func(ctx context.Context, fipID string) error {
		if fipID != "fip-to-delete" {
			t.Errorf("expected to delete 'fip-to-delete', got %q", fipID)
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
		NamespacedName: types.NamespacedName{Name: "deleting-fip", Namespace: "default"},
	})

	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	if mockVPC.CallCount("DeleteFloatingIP") != 1 {
		t.Errorf("expected DeleteFloatingIP to be called once, got %d", mockVPC.CallCount("DeleteFloatingIP"))
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
		t.Fatalf("Reconcile() error = %v for not-found resource", err)
	}
	if result.Requeue {
		t.Error("should not requeue for not-found resource")
	}
}
