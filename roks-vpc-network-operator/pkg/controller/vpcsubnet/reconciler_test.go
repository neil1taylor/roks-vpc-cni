package vpcsubnet

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

func TestReconcileNormal_CreateSubnet(t *testing.T) {
	scheme := newTestScheme()

	subnet := &v1alpha1.VPCSubnet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-subnet",
			Namespace: "default",
		},
		Spec: v1alpha1.VPCSubnetSpec{
			VPCID:         "vpc-123",
			Zone:          "us-south-1",
			IPv4CIDRBlock: "10.240.0.0/24",
			ClusterID:     "cluster-abc",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(subnet).
		WithStatusSubresource(subnet).
		Build()

	mockVPC := vpc.NewMockClient()
	mockVPC.CreateSubnetFn = func(ctx context.Context, opts vpc.CreateSubnetOptions) (*vpc.Subnet, error) {
		return &vpc.Subnet{
			ID:     "subnet-vpc-id-1",
			Name:   opts.Name,
			CIDR:   opts.CIDR,
			Status: "available",
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
			Name:      "test-subnet",
			Namespace: "default",
		},
	})

	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if result.RequeueAfter != 0 {
		t.Errorf("expected no requeue, got %v", result.RequeueAfter)
	}

	// Verify VPC client was called
	if mockVPC.CallCount("CreateSubnet") != 1 {
		t.Errorf("expected CreateSubnet to be called once, got %d", mockVPC.CallCount("CreateSubnet"))
	}

	// Verify status was updated
	updated := &v1alpha1.VPCSubnet{}
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "test-subnet", Namespace: "default"}, updated); err != nil {
		t.Fatalf("Failed to get updated subnet: %v", err)
	}

	if updated.Status.SubnetID != "subnet-vpc-id-1" {
		t.Errorf("expected SubnetID = 'subnet-vpc-id-1', got %q", updated.Status.SubnetID)
	}
	if updated.Status.SyncStatus != "Synced" {
		t.Errorf("expected SyncStatus = 'Synced', got %q", updated.Status.SyncStatus)
	}
}

func TestReconcileNormal_ExistingSubnet(t *testing.T) {
	scheme := newTestScheme()

	subnet := &v1alpha1.VPCSubnet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "existing-subnet",
			Namespace: "default",
			Finalizers: []string{"vpc.roks.ibm.com/subnet-protection"},
		},
		Spec: v1alpha1.VPCSubnetSpec{
			VPCID:         "vpc-123",
			Zone:          "us-south-1",
			IPv4CIDRBlock: "10.240.0.0/24",
		},
		Status: v1alpha1.VPCSubnetStatus{
			SubnetID: "subnet-existing-id",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(subnet).
		WithStatusSubresource(subnet).
		Build()

	mockVPC := vpc.NewMockClient()
	mockVPC.GetSubnetFn = func(ctx context.Context, subnetID string) (*vpc.Subnet, error) {
		return &vpc.Subnet{
			ID:     subnetID,
			Name:   "existing",
			CIDR:   "10.240.0.0/24",
			Status: "available",
		}, nil
	}

	r := &Reconciler{
		Client:    fakeClient,
		Scheme:    scheme,
		VPC:       mockVPC,
		ClusterID: "cluster-abc",
	}

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "existing-subnet", Namespace: "default"},
	})

	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	// Should NOT create a new subnet
	if mockVPC.CallCount("CreateSubnet") != 0 {
		t.Errorf("CreateSubnet should not be called for existing subnet")
	}
}

func TestReconcileNormal_VPCError(t *testing.T) {
	scheme := newTestScheme()

	subnet := &v1alpha1.VPCSubnet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "failing-subnet",
			Namespace: "default",
		},
		Spec: v1alpha1.VPCSubnetSpec{
			VPCID:         "vpc-123",
			Zone:          "us-south-1",
			IPv4CIDRBlock: "10.240.0.0/24",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(subnet).
		WithStatusSubresource(subnet).
		Build()

	mockVPC := vpc.NewMockClient()
	mockVPC.CreateSubnetFn = func(ctx context.Context, opts vpc.CreateSubnetOptions) (*vpc.Subnet, error) {
		return nil, fmt.Errorf("VPC API error: quota exceeded")
	}

	r := &Reconciler{
		Client:    fakeClient,
		Scheme:    scheme,
		VPC:       mockVPC,
		ClusterID: "cluster-abc",
	}

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "failing-subnet", Namespace: "default"},
	})

	// Should requeue on error, not return an error
	if err != nil {
		t.Fatalf("Reconcile() should not return error for VPC failures, got %v", err)
	}
	if result.RequeueAfter == 0 {
		t.Error("expected requeue after VPC error")
	}

	// Verify status is "Failed"
	updated := &v1alpha1.VPCSubnet{}
	_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: "failing-subnet", Namespace: "default"}, updated)
	if updated.Status.SyncStatus != "Failed" {
		t.Errorf("expected SyncStatus = 'Failed', got %q", updated.Status.SyncStatus)
	}
}

func TestReconcileDelete(t *testing.T) {
	scheme := newTestScheme()

	now := metav1.Now()
	subnet := &v1alpha1.VPCSubnet{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "deleting-subnet",
			Namespace:         "default",
			DeletionTimestamp: &now,
			Finalizers:        []string{"vpc.roks.ibm.com/subnet-protection"},
		},
		Spec: v1alpha1.VPCSubnetSpec{
			VPCID: "vpc-123",
			Zone:  "us-south-1",
		},
		Status: v1alpha1.VPCSubnetStatus{
			SubnetID: "subnet-to-delete",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(subnet).
		WithStatusSubresource(subnet).
		Build()

	mockVPC := vpc.NewMockClient()
	mockVPC.DeleteSubnetFn = func(ctx context.Context, subnetID string) error {
		if subnetID != "subnet-to-delete" {
			t.Errorf("expected to delete 'subnet-to-delete', got %q", subnetID)
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
		NamespacedName: types.NamespacedName{Name: "deleting-subnet", Namespace: "default"},
	})

	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	if mockVPC.CallCount("DeleteSubnet") != 1 {
		t.Errorf("expected DeleteSubnet to be called once, got %d", mockVPC.CallCount("DeleteSubnet"))
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
