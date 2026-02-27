package udn

import (
	"context"
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
	"github.com/IBM/roks-vpc-network-operator/pkg/annotations"
	"github.com/IBM/roks-vpc-network-operator/pkg/finalizers"
	"github.com/IBM/roks-vpc-network-operator/pkg/vpc"
)

func newTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(s)
	_ = v1alpha1.AddToScheme(s)
	return s
}

// makeTestUDN creates an unstructured UserDefinedNetwork object for testing.
func makeTestUDN(namespace, name, topology string, annots map[string]string) *unstructured.Unstructured {
	udn := &unstructured.Unstructured{}
	udn.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "k8s.ovn.org",
		Version: "v1",
		Kind:    "UserDefinedNetwork",
	})
	udn.SetNamespace(namespace)
	udn.SetName(name)
	if annots != nil {
		udn.SetAnnotations(annots)
	}
	_ = unstructured.SetNestedField(udn.Object, topology, "spec", "topology")
	return udn
}

// localNetAnnotations returns a complete set of required annotations for a LocalNet UDN.
func localNetAnnotations() map[string]string {
	return map[string]string{
		annotations.Zone:             "us-south-1",
		annotations.CIDR:             "10.240.64.0/24",
		annotations.VPCID:            "vpc-123",
		annotations.VLANID:           "100",
		annotations.SecurityGroupIDs: "sg-1,sg-2",
		annotations.ACLID:            "acl-1",
	}
}

// makeBareMetalNode creates a bare metal node with the given name and provider ID.
func makeBareMetalNode(name, providerID string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"node.kubernetes.io/instance-type": "bx2-metal-96x384",
			},
		},
		Spec: corev1.NodeSpec{
			ProviderID: providerID,
		},
	}
}

// --- LocalNet reconciliation tests ---

func TestReconcile_LocalNet_CreatesSubnetAndVLANAttachments(t *testing.T) {
	scheme := newTestScheme()
	annots := localNetAnnotations()
	udn := makeTestUDN("default", "localnet-udn", "LocalNet", annots)

	node1 := makeBareMetalNode("bm-node-1", "ibm://acct/us-south/us-south-1/server-1")
	node2 := makeBareMetalNode("bm-node-2", "ibm://acct/us-south/us-south-1/server-2")

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(udn, node1, node2).
		Build()

	mockVPC := vpc.NewMockClient()
	mockVPC.CreateSubnetFn = func(ctx context.Context, opts vpc.CreateSubnetOptions) (*vpc.Subnet, error) {
		return &vpc.Subnet{
			ID:     "subnet-new-1",
			Name:   opts.Name,
			CIDR:   opts.CIDR,
			Status: "available",
		}, nil
	}
	mockVPC.CreateVLANAttachmentFn = func(ctx context.Context, opts vpc.CreateVLANAttachmentOptions) (*vpc.VLANAttachment, error) {
		return &vpc.VLANAttachment{
			ID:   "vla-" + opts.BMServerID,
			Name: opts.Name,
		}, nil
	}

	r := &Reconciler{
		Client:    fakeClient,
		Scheme:    scheme,
		VPC:       mockVPC,
		ClusterID: "cluster-abc",
	}

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: "default", Name: "localnet-udn"},
	})

	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if result.RequeueAfter != 0 {
		t.Errorf("expected no requeue, got %v", result.RequeueAfter)
	}

	// Verify VPC subnet was created
	if mockVPC.CallCount("CreateSubnet") != 1 {
		t.Errorf("expected CreateSubnet called once, got %d", mockVPC.CallCount("CreateSubnet"))
	}

	// Verify VLAN attachments were created for both bare metal nodes
	if mockVPC.CallCount("CreateVLANAttachment") != 2 {
		t.Errorf("expected CreateVLANAttachment called twice, got %d", mockVPC.CallCount("CreateVLANAttachment"))
	}

	// Verify finalizer was added
	updated := &unstructured.Unstructured{}
	updated.SetGroupVersionKind(udnGVK)
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "localnet-udn"}, updated); err != nil {
		t.Fatalf("Failed to get updated UDN: %v", err)
	}
	if !hasFinalizer(updated, finalizers.UDNCleanup) {
		t.Error("Expected udn-cleanup finalizer to be added")
	}

	// Verify subnet-id annotation was set
	updatedAnnots := updated.GetAnnotations()
	if updatedAnnots[annotations.SubnetID] != "subnet-new-1" {
		t.Errorf("expected subnet-id annotation = 'subnet-new-1', got %q", updatedAnnots[annotations.SubnetID])
	}
}

func TestReconcile_LocalNet_SkipsSubnetIfAlreadyExists(t *testing.T) {
	scheme := newTestScheme()
	annots := localNetAnnotations()
	annots[annotations.SubnetID] = "subnet-existing"
	annots[annotations.SubnetStatus] = "available"
	udn := makeTestUDN("default", "localnet-udn", "LocalNet", annots)

	node := makeBareMetalNode("bm-node-1", "ibm://acct/us-south/us-south-1/server-1")

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(udn, node).
		Build()

	mockVPC := vpc.NewMockClient()
	mockVPC.CreateVLANAttachmentFn = func(ctx context.Context, opts vpc.CreateVLANAttachmentOptions) (*vpc.VLANAttachment, error) {
		return &vpc.VLANAttachment{
			ID:   "vla-new",
			Name: opts.Name,
		}, nil
	}

	r := &Reconciler{
		Client:    fakeClient,
		Scheme:    scheme,
		VPC:       mockVPC,
		ClusterID: "cluster-abc",
	}

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: "default", Name: "localnet-udn"},
	})

	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	// CreateSubnet should NOT be called since subnet-id annotation is already set
	if mockVPC.CallCount("CreateSubnet") != 0 {
		t.Errorf("expected CreateSubnet not to be called, got %d", mockVPC.CallCount("CreateSubnet"))
	}
}

func TestReconcile_LocalNet_MissingAnnotation_ReturnsError(t *testing.T) {
	scheme := newTestScheme()

	// Missing required annotations (only provide some)
	annots := map[string]string{
		annotations.Zone: "us-south-1",
		annotations.CIDR: "10.240.64.0/24",
		// Missing: vpc-id, vlan-id, security-group-ids, acl-id
	}
	udn := makeTestUDN("default", "bad-udn", "LocalNet", annots)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(udn).
		Build()

	mockVPC := vpc.NewMockClient()

	r := &Reconciler{
		Client:    fakeClient,
		Scheme:    scheme,
		VPC:       mockVPC,
		ClusterID: "cluster-abc",
	}

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: "default", Name: "bad-udn"},
	})

	if err == nil {
		t.Fatal("expected error for missing annotations")
	}
	if result.RequeueAfter != 30*time.Second {
		t.Errorf("expected 30s requeue, got %v", result.RequeueAfter)
	}

	// No VPC calls should be made
	if mockVPC.CallCount("CreateSubnet") != 0 {
		t.Error("CreateSubnet should not be called with missing annotations")
	}
}

func TestReconcile_LocalNet_NilAnnotations_ReturnsError(t *testing.T) {
	scheme := newTestScheme()

	// No annotations at all
	udn := makeTestUDN("default", "no-annot-udn", "LocalNet", nil)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(udn).
		Build()

	mockVPC := vpc.NewMockClient()

	r := &Reconciler{
		Client:    fakeClient,
		Scheme:    scheme,
		VPC:       mockVPC,
		ClusterID: "cluster-abc",
	}

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: "default", Name: "no-annot-udn"},
	})

	if err == nil {
		t.Fatal("expected error for nil annotations")
	}
}

// --- Layer2 reconciliation tests ---

func TestReconcile_Layer2_AddsFinalizer_NoVPCCalls(t *testing.T) {
	scheme := newTestScheme()

	udn := makeTestUDN("default", "layer2-udn", "Layer2", nil)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(udn).
		Build()

	mockVPC := vpc.NewMockClient()

	r := &Reconciler{
		Client:    fakeClient,
		Scheme:    scheme,
		VPC:       mockVPC,
		ClusterID: "cluster-abc",
	}

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: "default", Name: "layer2-udn"},
	})

	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if result.Requeue || result.RequeueAfter != 0 {
		t.Error("Layer2 UDN should not requeue")
	}

	// No VPC API calls for Layer2
	if mockVPC.CallCount("CreateSubnet") != 0 {
		t.Error("CreateSubnet should not be called for Layer2 UDN")
	}
	if mockVPC.CallCount("CreateVLANAttachment") != 0 {
		t.Error("CreateVLANAttachment should not be called for Layer2 UDN")
	}

	// Verify finalizer was added
	updated := &unstructured.Unstructured{}
	updated.SetGroupVersionKind(udnGVK)
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "layer2-udn"}, updated); err != nil {
		t.Fatalf("Failed to get updated UDN: %v", err)
	}
	if !hasFinalizer(updated, finalizers.UDNCleanup) {
		t.Error("Expected udn-cleanup finalizer to be added to Layer2 UDN")
	}
}

// --- Deletion tests ---

func TestReconcile_LocalNet_Deletion(t *testing.T) {
	scheme := newTestScheme()

	annots := localNetAnnotations()
	annots[annotations.SubnetID] = "subnet-to-delete"
	annots[annotations.VLANAttachments] = "bm-node-1:vla-1,bm-node-2:vla-2"

	udn := makeTestUDN("default", "deleting-udn", "LocalNet", annots)
	now := metav1.Now()
	udn.SetDeletionTimestamp(&now)
	udn.SetFinalizers([]string{finalizers.UDNCleanup})

	// Create nodes so DeleteVLANAttachments can resolve their BM server IDs
	node1 := makeBareMetalNode("bm-node-1", "ibm://acct/us-south/us-south-1/server-1")
	node2 := makeBareMetalNode("bm-node-2", "ibm://acct/us-south/us-south-1/server-2")

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(udn, node1, node2).
		Build()

	mockVPC := vpc.NewMockClient()
	mockVPC.ListSubnetReservedIPsFn = func(ctx context.Context, subnetID string) ([]vpc.ReservedIP, error) {
		return nil, nil // No active VNIs — allow deletion
	}
	mockVPC.DeleteVLANAttachmentFn = func(ctx context.Context, bmServerID, attachmentID string) error {
		return nil
	}
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
		NamespacedName: types.NamespacedName{Namespace: "default", Name: "deleting-udn"},
	})

	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	// Verify VLAN attachments were deleted
	if mockVPC.CallCount("DeleteVLANAttachment") != 2 {
		t.Errorf("expected DeleteVLANAttachment called twice, got %d", mockVPC.CallCount("DeleteVLANAttachment"))
	}

	// Verify subnet was deleted
	if mockVPC.CallCount("DeleteSubnet") != 1 {
		t.Errorf("expected DeleteSubnet called once, got %d", mockVPC.CallCount("DeleteSubnet"))
	}

	// After finalizer removal with a deletion timestamp, the fake client
	// garbage-collects the object. Verify it is gone (which proves the
	// finalizer was removed).
	updated := &unstructured.Unstructured{}
	updated.SetGroupVersionKind(udnGVK)
	getErr := fakeClient.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "deleting-udn"}, updated)
	if getErr == nil {
		// Object still exists — check finalizer was removed
		if hasFinalizer(updated, finalizers.UDNCleanup) {
			t.Error("Expected udn-cleanup finalizer to be removed after deletion")
		}
	}
	// If getErr is not-found, the object was fully deleted — which is the expected outcome.
}

func TestReconcile_Layer2_Deletion(t *testing.T) {
	scheme := newTestScheme()

	udn := makeTestUDN("default", "layer2-deleting", "Layer2", nil)
	now := metav1.Now()
	udn.SetDeletionTimestamp(&now)
	udn.SetFinalizers([]string{finalizers.UDNCleanup})

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(udn).
		Build()

	mockVPC := vpc.NewMockClient()

	r := &Reconciler{
		Client:    fakeClient,
		Scheme:    scheme,
		VPC:       mockVPC,
		ClusterID: "cluster-abc",
	}

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: "default", Name: "layer2-deleting"},
	})

	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	// No VPC API calls for Layer2 deletion
	if mockVPC.CallCount("DeleteVLANAttachment") != 0 {
		t.Error("DeleteVLANAttachment should not be called for Layer2 deletion")
	}
	if mockVPC.CallCount("DeleteSubnet") != 0 {
		t.Error("DeleteSubnet should not be called for Layer2 deletion")
	}

	// After finalizer removal with a deletion timestamp, the fake client
	// garbage-collects the object. Verify it is gone (which proves the
	// finalizer was removed).
	updated := &unstructured.Unstructured{}
	updated.SetGroupVersionKind(udnGVK)
	getErr := fakeClient.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "layer2-deleting"}, updated)
	if getErr == nil {
		// Object still exists — check finalizer was removed
		if hasFinalizer(updated, finalizers.UDNCleanup) {
			t.Error("Expected udn-cleanup finalizer to be removed after Layer2 deletion")
		}
	}
	// If getErr is not-found, the object was fully deleted — which is the expected outcome.
}

// --- Not found test ---

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
		NamespacedName: types.NamespacedName{Namespace: "default", Name: "nonexistent-udn"},
	})

	if err != nil {
		t.Fatalf("Reconcile() error = %v for not-found resource", err)
	}
	if result.Requeue {
		t.Error("should not requeue for not-found resource")
	}
}

// --- Unknown topology test ---

func TestReconcile_UnknownTopology_Skipped(t *testing.T) {
	scheme := newTestScheme()

	udn := makeTestUDN("default", "routed-udn", "Routed", nil)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(udn).
		Build()

	mockVPC := vpc.NewMockClient()

	r := &Reconciler{
		Client:    fakeClient,
		Scheme:    scheme,
		VPC:       mockVPC,
		ClusterID: "cluster-abc",
	}

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: "default", Name: "routed-udn"},
	})

	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if result.Requeue {
		t.Error("Should not requeue for unknown topology")
	}
	if mockVPC.CallCount("CreateSubnet") != 0 {
		t.Error("No VPC calls should be made for unknown topology")
	}
}

// --- LocalNet deletion with no VLAN attachments annotation ---

func TestReconcile_LocalNet_Deletion_NoAttachments(t *testing.T) {
	scheme := newTestScheme()

	annots := localNetAnnotations()
	annots[annotations.SubnetID] = "subnet-to-delete"
	// No vlan-attachments annotation

	udn := makeTestUDN("default", "del-no-vla", "LocalNet", annots)
	now := metav1.Now()
	udn.SetDeletionTimestamp(&now)
	udn.SetFinalizers([]string{finalizers.UDNCleanup})

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(udn).
		Build()

	mockVPC := vpc.NewMockClient()
	mockVPC.ListSubnetReservedIPsFn = func(ctx context.Context, subnetID string) ([]vpc.ReservedIP, error) {
		return nil, nil // No active VNIs — allow deletion
	}
	mockVPC.DeleteSubnetFn = func(ctx context.Context, subnetID string) error {
		return nil
	}

	r := &Reconciler{
		Client:    fakeClient,
		Scheme:    scheme,
		VPC:       mockVPC,
		ClusterID: "cluster-abc",
	}

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: "default", Name: "del-no-vla"},
	})

	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	// No VLAN attachment deletions needed
	if mockVPC.CallCount("DeleteVLANAttachment") != 0 {
		t.Errorf("expected no DeleteVLANAttachment calls, got %d", mockVPC.CallCount("DeleteVLANAttachment"))
	}

	// Subnet should still be deleted
	if mockVPC.CallCount("DeleteSubnet") != 1 {
		t.Errorf("expected DeleteSubnet called once, got %d", mockVPC.CallCount("DeleteSubnet"))
	}
}

// --- Idempotent finalizer test ---

func TestReconcile_Layer2_FinalizerAlreadyPresent(t *testing.T) {
	scheme := newTestScheme()

	udn := makeTestUDN("default", "layer2-idempotent", "Layer2", nil)
	udn.SetFinalizers([]string{finalizers.UDNCleanup})

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(udn).
		Build()

	r := &Reconciler{
		Client:    fakeClient,
		Scheme:    scheme,
		VPC:       vpc.NewMockClient(),
		ClusterID: "cluster-abc",
	}

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: "default", Name: "layer2-idempotent"},
	})

	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	// Verify only one finalizer (not duplicated)
	updated := &unstructured.Unstructured{}
	updated.SetGroupVersionKind(udnGVK)
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "layer2-idempotent"}, updated); err != nil {
		t.Fatalf("Failed to get updated UDN: %v", err)
	}
	count := 0
	for _, f := range updated.GetFinalizers() {
		if f == finalizers.UDNCleanup {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 udn-cleanup finalizer, got %d", count)
	}
}

// --- Helper ---

func hasFinalizer(obj metav1.Object, fin string) bool {
	for _, f := range obj.GetFinalizers() {
		if f == fin {
			return true
		}
	}
	return false
}
