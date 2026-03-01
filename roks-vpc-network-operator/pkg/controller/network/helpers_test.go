package network

import (
	"context"
	"fmt"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
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

// makeTestObj creates an unstructured object suitable for testing network helpers.
// It uses the CUDN GVK as a stand-in (cluster-scoped).
func makeTestObj(name string, annots map[string]string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "k8s.ovn.org",
		Version: "v1",
		Kind:    "ClusterUserDefinedNetwork",
	})
	obj.SetName(name)
	if annots != nil {
		obj.SetAnnotations(annots)
	}
	return obj
}

// makeNamespacedTestObj creates an unstructured namespace-scoped object (UDN).
func makeNamespacedTestObj(namespace, name string, annots map[string]string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "k8s.ovn.org",
		Version: "v1",
		Kind:    "UserDefinedNetwork",
	})
	obj.SetNamespace(namespace)
	obj.SetName(name)
	if annots != nil {
		obj.SetAnnotations(annots)
	}
	return obj
}

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

func makeVirtualNode(name string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"node.kubernetes.io/instance-type": "bx2-4x16",
			},
		},
	}
}

// ─── EnsureVPCSubnet tests ───

func TestEnsureVPCSubnet_CreatesSubnet(t *testing.T) {
	scheme := newTestScheme()
	annots := map[string]string{
		annotations.VPCID: "vpc-123",
		annotations.Zone:  "us-south-1",
		annotations.CIDR:  "10.240.64.0/24",
		annotations.ACLID: "acl-1",
	}
	obj := makeTestObj("test-cudn", annots)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(obj).
		Build()

	mockVPC := vpc.NewMockClient()
	var capturedOpts vpc.CreateSubnetOptions
	mockVPC.CreateSubnetFn = func(ctx context.Context, opts vpc.CreateSubnetOptions) (*vpc.Subnet, error) {
		capturedOpts = opts
		return &vpc.Subnet{
			ID:     "subnet-new-1",
			Name:   opts.Name,
			CIDR:   opts.CIDR,
			Status: "available",
		}, nil
	}

	created, err := EnsureVPCSubnet(context.Background(), fakeClient, mockVPC, obj, "cluster-abc", "test")
	if err != nil {
		t.Fatalf("EnsureVPCSubnet() error = %v", err)
	}
	if !created {
		t.Error("expected created=true for new subnet")
	}

	// Verify the options passed to CreateSubnet
	if capturedOpts.VPCID != "vpc-123" {
		t.Errorf("expected VPCID='vpc-123', got %q", capturedOpts.VPCID)
	}
	if capturedOpts.Zone != "us-south-1" {
		t.Errorf("expected Zone='us-south-1', got %q", capturedOpts.Zone)
	}
	if capturedOpts.CIDR != "10.240.64.0/24" {
		t.Errorf("expected CIDR='10.240.64.0/24', got %q", capturedOpts.CIDR)
	}
	if capturedOpts.ACLID != "acl-1" {
		t.Errorf("expected ACLID='acl-1', got %q", capturedOpts.ACLID)
	}
	if capturedOpts.ClusterID != "cluster-abc" {
		t.Errorf("expected ClusterID='cluster-abc', got %q", capturedOpts.ClusterID)
	}

	// Verify the subnet name for cluster-scoped object (no namespace)
	expectedName := "roks-cluster-abc-test-cudn"
	if capturedOpts.Name != expectedName {
		t.Errorf("expected subnet name=%q, got %q", expectedName, capturedOpts.Name)
	}

	// Verify annotations were updated on the object
	updated := &unstructured.Unstructured{}
	updated.SetGroupVersionKind(obj.GroupVersionKind())
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "test-cudn"}, updated); err != nil {
		t.Fatalf("Failed to get updated object: %v", err)
	}
	updatedAnnots := updated.GetAnnotations()
	if updatedAnnots[annotations.SubnetID] != "subnet-new-1" {
		t.Errorf("expected subnet-id='subnet-new-1', got %q", updatedAnnots[annotations.SubnetID])
	}
	if updatedAnnots[annotations.SubnetStatus] != "available" {
		t.Errorf("expected subnet-status='available', got %q", updatedAnnots[annotations.SubnetStatus])
	}
}

func TestEnsureVPCSubnet_NamespacedObject_IncludesNamespace(t *testing.T) {
	scheme := newTestScheme()
	annots := map[string]string{
		annotations.VPCID: "vpc-123",
		annotations.Zone:  "us-south-1",
		annotations.CIDR:  "10.240.64.0/24",
		annotations.ACLID: "acl-1",
	}
	obj := makeNamespacedTestObj("my-ns", "test-udn", annots)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(obj).
		Build()

	mockVPC := vpc.NewMockClient()
	var capturedName string
	mockVPC.CreateSubnetFn = func(ctx context.Context, opts vpc.CreateSubnetOptions) (*vpc.Subnet, error) {
		capturedName = opts.Name
		return &vpc.Subnet{ID: "subnet-1", Name: opts.Name, CIDR: opts.CIDR, Status: "available"}, nil
	}

	_, err := EnsureVPCSubnet(context.Background(), fakeClient, mockVPC, obj, "cluster-abc", "test")
	if err != nil {
		t.Fatalf("EnsureVPCSubnet() error = %v", err)
	}

	expectedName := "roks-cluster-abc-my-ns-test-udn"
	if capturedName != expectedName {
		t.Errorf("expected subnet name=%q, got %q", expectedName, capturedName)
	}
}

func TestEnsureVPCSubnet_SkipsIfSubnetIDAlreadySet(t *testing.T) {
	scheme := newTestScheme()
	annots := map[string]string{
		annotations.SubnetID: "subnet-existing",
		annotations.VPCID:    "vpc-123",
		annotations.Zone:     "us-south-1",
		annotations.CIDR:     "10.240.64.0/24",
		annotations.ACLID:    "acl-1",
	}
	obj := makeTestObj("existing-cudn", annots)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(obj).
		Build()

	mockVPC := vpc.NewMockClient()

	created, err := EnsureVPCSubnet(context.Background(), fakeClient, mockVPC, obj, "cluster-abc", "test")
	if err != nil {
		t.Fatalf("EnsureVPCSubnet() error = %v", err)
	}
	if created {
		t.Error("expected created=false when subnet-id already exists")
	}
	if mockVPC.CallCount("CreateSubnet") != 0 {
		t.Error("CreateSubnet should not be called when subnet-id is already set")
	}
}

func TestEnsureVPCSubnet_VPCError(t *testing.T) {
	scheme := newTestScheme()
	annots := map[string]string{
		annotations.VPCID: "vpc-123",
		annotations.Zone:  "us-south-1",
		annotations.CIDR:  "10.240.64.0/24",
		annotations.ACLID: "acl-1",
	}
	obj := makeTestObj("failing-cudn", annots)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(obj).
		Build()

	mockVPC := vpc.NewMockClient()
	mockVPC.CreateSubnetFn = func(ctx context.Context, opts vpc.CreateSubnetOptions) (*vpc.Subnet, error) {
		return nil, fmt.Errorf("VPC API error: quota exceeded")
	}

	created, err := EnsureVPCSubnet(context.Background(), fakeClient, mockVPC, obj, "cluster-abc", "test")
	if err == nil {
		t.Fatal("expected error from VPC API failure")
	}
	if created {
		t.Error("expected created=false on error")
	}
}

func TestEnsureVPCSubnet_NilAnnotations(t *testing.T) {
	scheme := newTestScheme()
	obj := makeTestObj("nil-annot-cudn", nil)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(obj).
		Build()

	mockVPC := vpc.NewMockClient()
	mockVPC.CreateSubnetFn = func(ctx context.Context, opts vpc.CreateSubnetOptions) (*vpc.Subnet, error) {
		return &vpc.Subnet{ID: "subnet-1", Name: opts.Name, Status: "available"}, nil
	}

	created, err := EnsureVPCSubnet(context.Background(), fakeClient, mockVPC, obj, "cluster-abc", "test")
	if err != nil {
		t.Fatalf("EnsureVPCSubnet() error = %v", err)
	}
	// With nil annotations, SubnetID is empty, so it should attempt creation
	if !created {
		t.Error("expected created=true since SubnetID annotation is empty")
	}
}

// ─── EnsureVLANAttachments tests ───

func TestEnsureVLANAttachments_CreateForBareMetalNodes(t *testing.T) {
	scheme := newTestScheme()

	annots := map[string]string{
		annotations.SubnetID: "subnet-123",
		annotations.VLANID:   "100",
	}
	obj := makeTestObj("test-cudn", annots)

	node1 := makeBareMetalNode("bm-node-1", "ibm://acct/us-south/us-south-1/server-1")
	node2 := makeBareMetalNode("bm-node-2", "ibm://acct/us-south/us-south-1/server-2")
	vNode := makeVirtualNode("virtual-node-1")

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(obj, node1, node2, vNode).
		Build()

	mockVPC := vpc.NewMockClient()
	mockVPC.CreateVLANAttachmentFn = func(ctx context.Context, opts vpc.CreateVLANAttachmentOptions) (*vpc.VLANAttachment, error) {
		return &vpc.VLANAttachment{
			ID:   "vla-" + opts.BMServerID,
			Name: opts.Name,
		}, nil
	}

	err := EnsureVLANAttachments(context.Background(), fakeClient, mockVPC, obj, "cluster-abc", "test")
	if err != nil {
		t.Fatalf("EnsureVLANAttachments() error = %v", err)
	}

	// Should create attachments for 2 bare metal nodes only
	if mockVPC.CallCount("CreateVLANAttachment") != 2 {
		t.Errorf("expected 2 CreateVLANAttachment calls, got %d", mockVPC.CallCount("CreateVLANAttachment"))
	}

	// Verify annotations were updated with attachment map
	updated := &unstructured.Unstructured{}
	updated.SetGroupVersionKind(obj.GroupVersionKind())
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "test-cudn"}, updated); err != nil {
		t.Fatalf("Failed to get updated object: %v", err)
	}
	updatedAnnots := updated.GetAnnotations()
	attachStr := updatedAnnots[annotations.VLANAttachments]
	attachments := ParseAttachments(attachStr)
	if len(attachments) != 2 {
		t.Errorf("expected 2 VLAN attachments in annotation, got %d", len(attachments))
	}
	if attachments["bm-node-1"] != "vla-server-1" {
		t.Errorf("expected bm-node-1 attachment = 'vla-server-1', got %q", attachments["bm-node-1"])
	}
	if attachments["bm-node-2"] != "vla-server-2" {
		t.Errorf("expected bm-node-2 attachment = 'vla-server-2', got %q", attachments["bm-node-2"])
	}
}

func TestEnsureVLANAttachments_SkipsExistingAttachments(t *testing.T) {
	scheme := newTestScheme()

	annots := map[string]string{
		annotations.SubnetID:        "subnet-123",
		annotations.VLANID:          "100",
		annotations.VLANAttachments: "bm-node-1:vla-existing",
	}
	obj := makeTestObj("test-cudn", annots)

	node1 := makeBareMetalNode("bm-node-1", "ibm://acct/us-south/us-south-1/server-1")
	node2 := makeBareMetalNode("bm-node-2", "ibm://acct/us-south/us-south-1/server-2")

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(obj, node1, node2).
		Build()

	mockVPC := vpc.NewMockClient()
	mockVPC.CreateVLANAttachmentFn = func(ctx context.Context, opts vpc.CreateVLANAttachmentOptions) (*vpc.VLANAttachment, error) {
		return &vpc.VLANAttachment{
			ID:   "vla-new-" + opts.BMServerID,
			Name: opts.Name,
		}, nil
	}

	err := EnsureVLANAttachments(context.Background(), fakeClient, mockVPC, obj, "cluster-abc", "test")
	if err != nil {
		t.Fatalf("EnsureVLANAttachments() error = %v", err)
	}

	// Should only create for bm-node-2 (bm-node-1 already has one)
	if mockVPC.CallCount("CreateVLANAttachment") != 1 {
		t.Errorf("expected 1 CreateVLANAttachment call (skip existing), got %d", mockVPC.CallCount("CreateVLANAttachment"))
	}
}

func TestEnsureVLANAttachments_NoSubnetID_Noop(t *testing.T) {
	scheme := newTestScheme()

	annots := map[string]string{
		annotations.VLANID: "100",
		// No SubnetID
	}
	obj := makeTestObj("test-cudn", annots)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(obj).
		Build()

	mockVPC := vpc.NewMockClient()

	err := EnsureVLANAttachments(context.Background(), fakeClient, mockVPC, obj, "cluster-abc", "test")
	if err != nil {
		t.Fatalf("EnsureVLANAttachments() error = %v", err)
	}

	if mockVPC.CallCount("CreateVLANAttachment") != 0 {
		t.Error("CreateVLANAttachment should not be called without subnet-id")
	}
}

func TestEnsureVLANAttachments_SkipsNodeWithoutProviderID(t *testing.T) {
	scheme := newTestScheme()

	annots := map[string]string{
		annotations.SubnetID: "subnet-123",
		annotations.VLANID:   "100",
	}
	obj := makeTestObj("test-cudn", annots)

	// A bare metal node without a provider ID
	nodeMissing := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "bm-no-provider",
			Labels: map[string]string{
				"node.kubernetes.io/instance-type": "bx2-metal-96x384",
			},
		},
		// No ProviderID
	}
	nodeOK := makeBareMetalNode("bm-ok", "ibm://acct/us-south/us-south-1/server-ok")

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(obj, nodeMissing, nodeOK).
		Build()

	mockVPC := vpc.NewMockClient()
	mockVPC.CreateVLANAttachmentFn = func(ctx context.Context, opts vpc.CreateVLANAttachmentOptions) (*vpc.VLANAttachment, error) {
		return &vpc.VLANAttachment{ID: "vla-1", Name: opts.Name}, nil
	}

	err := EnsureVLANAttachments(context.Background(), fakeClient, mockVPC, obj, "cluster-abc", "test")
	if err != nil {
		t.Fatalf("EnsureVLANAttachments() error = %v", err)
	}

	// Only the node with a valid provider ID should trigger a VLAN attachment creation
	if mockVPC.CallCount("CreateVLANAttachment") != 1 {
		t.Errorf("expected 1 CreateVLANAttachment call, got %d", mockVPC.CallCount("CreateVLANAttachment"))
	}
}

func TestEnsureVLANAttachments_VPCError_ContinuesOtherNodes(t *testing.T) {
	scheme := newTestScheme()

	annots := map[string]string{
		annotations.SubnetID: "subnet-123",
		annotations.VLANID:   "100",
	}
	obj := makeTestObj("test-cudn", annots)

	node1 := makeBareMetalNode("bm-node-1", "ibm://acct/us-south/us-south-1/server-1")
	node2 := makeBareMetalNode("bm-node-2", "ibm://acct/us-south/us-south-1/server-2")

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(obj, node1, node2).
		Build()

	mockVPC := vpc.NewMockClient()
	callCount := 0
	mockVPC.CreateVLANAttachmentFn = func(ctx context.Context, opts vpc.CreateVLANAttachmentOptions) (*vpc.VLANAttachment, error) {
		callCount++
		if callCount == 1 {
			return nil, fmt.Errorf("VPC API error")
		}
		return &vpc.VLANAttachment{ID: "vla-success", Name: opts.Name}, nil
	}

	err := EnsureVLANAttachments(context.Background(), fakeClient, mockVPC, obj, "cluster-abc", "test")
	if err != nil {
		t.Fatalf("EnsureVLANAttachments() error = %v", err)
	}

	// Both should be attempted (errors continue to next node)
	if mockVPC.CallCount("CreateVLANAttachment") != 2 {
		t.Errorf("expected 2 CreateVLANAttachment calls, got %d", mockVPC.CallCount("CreateVLANAttachment"))
	}
}

// ─── DeleteVLANAttachments tests ───

func TestDeleteVLANAttachments_DeletesAll(t *testing.T) {
	scheme := newTestScheme()

	annots := map[string]string{
		annotations.VLANAttachments: "bm-node-1:vla-1,bm-node-2:vla-2",
	}
	obj := makeTestObj("test-cudn", annots)

	node1 := makeBareMetalNode("bm-node-1", "ibm://acct/us-south/us-south-1/server-1")
	node2 := makeBareMetalNode("bm-node-2", "ibm://acct/us-south/us-south-1/server-2")

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(obj, node1, node2).
		Build()

	mockVPC := vpc.NewMockClient()
	deletedAttachments := map[string]string{}
	mockVPC.DeleteVLANAttachmentFn = func(ctx context.Context, bmServerID, attachmentID string) error {
		deletedAttachments[bmServerID] = attachmentID
		return nil
	}

	err := DeleteVLANAttachments(context.Background(), fakeClient, mockVPC, obj)
	if err != nil {
		t.Fatalf("DeleteVLANAttachments() error = %v", err)
	}

	if mockVPC.CallCount("DeleteVLANAttachment") != 2 {
		t.Errorf("expected 2 DeleteVLANAttachment calls, got %d", mockVPC.CallCount("DeleteVLANAttachment"))
	}

	if deletedAttachments["server-1"] != "vla-1" {
		t.Errorf("expected server-1 -> vla-1, got %q", deletedAttachments["server-1"])
	}
	if deletedAttachments["server-2"] != "vla-2" {
		t.Errorf("expected server-2 -> vla-2, got %q", deletedAttachments["server-2"])
	}
}

func TestDeleteVLANAttachments_NilAnnotations_Noop(t *testing.T) {
	scheme := newTestScheme()
	obj := makeTestObj("test-cudn", nil)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(obj).
		Build()

	mockVPC := vpc.NewMockClient()

	err := DeleteVLANAttachments(context.Background(), fakeClient, mockVPC, obj)
	if err != nil {
		t.Fatalf("DeleteVLANAttachments() error = %v", err)
	}

	if mockVPC.CallCount("DeleteVLANAttachment") != 0 {
		t.Error("DeleteVLANAttachment should not be called with nil annotations")
	}
}

func TestDeleteVLANAttachments_EmptyAttachmentString_Noop(t *testing.T) {
	scheme := newTestScheme()

	annots := map[string]string{
		annotations.VLANAttachments: "",
	}
	obj := makeTestObj("test-cudn", annots)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(obj).
		Build()

	mockVPC := vpc.NewMockClient()

	err := DeleteVLANAttachments(context.Background(), fakeClient, mockVPC, obj)
	if err != nil {
		t.Fatalf("DeleteVLANAttachments() error = %v", err)
	}

	if mockVPC.CallCount("DeleteVLANAttachment") != 0 {
		t.Error("DeleteVLANAttachment should not be called with empty attachment string")
	}
}

func TestDeleteVLANAttachments_VPCError_ReturnsError(t *testing.T) {
	scheme := newTestScheme()

	annots := map[string]string{
		annotations.VLANAttachments: "bm-node-1:vla-1",
	}
	obj := makeTestObj("test-cudn", annots)

	node1 := makeBareMetalNode("bm-node-1", "ibm://acct/us-south/us-south-1/server-1")

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(obj, node1).
		Build()

	mockVPC := vpc.NewMockClient()
	mockVPC.DeleteVLANAttachmentFn = func(ctx context.Context, bmServerID, attachmentID string) error {
		return fmt.Errorf("VPC API error: internal server error")
	}

	err := DeleteVLANAttachments(context.Background(), fakeClient, mockVPC, obj)
	if err == nil {
		t.Fatal("expected error from VPC API failure")
	}
}

func TestDeleteVLANAttachments_NotFound_Tolerates(t *testing.T) {
	scheme := newTestScheme()

	annots := map[string]string{
		annotations.VLANAttachments: "bm-node-1:vla-1",
	}
	obj := makeTestObj("test-cudn", annots)

	node1 := makeBareMetalNode("bm-node-1", "ibm://acct/us-south/us-south-1/server-1")

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(obj, node1).
		Build()

	mockVPC := vpc.NewMockClient()
	mockVPC.DeleteVLANAttachmentFn = func(ctx context.Context, bmServerID, attachmentID string) error {
		return fmt.Errorf("VPC API error: not found")
	}

	err := DeleteVLANAttachments(context.Background(), fakeClient, mockVPC, obj)
	if err != nil {
		t.Fatalf("expected 404 to be tolerated, got error: %v", err)
	}
}

func TestDeleteVLANAttachments_NodeNotFound_SkipsNode(t *testing.T) {
	scheme := newTestScheme()

	annots := map[string]string{
		annotations.VLANAttachments: "missing-node:vla-1,bm-node-1:vla-2",
	}
	obj := makeTestObj("test-cudn", annots)

	// Only create bm-node-1 — missing-node does not exist
	node1 := makeBareMetalNode("bm-node-1", "ibm://acct/us-south/us-south-1/server-1")

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(obj, node1).
		Build()

	mockVPC := vpc.NewMockClient()
	mockVPC.DeleteVLANAttachmentFn = func(ctx context.Context, bmServerID, attachmentID string) error {
		return nil
	}

	err := DeleteVLANAttachments(context.Background(), fakeClient, mockVPC, obj)
	if err != nil {
		t.Fatalf("DeleteVLANAttachments() error = %v", err)
	}

	// missing-node is skipped because resolveNodeBMServerID returns ""
	// Only bm-node-1 triggers a delete
	if mockVPC.CallCount("DeleteVLANAttachment") != 1 {
		t.Errorf("expected 1 DeleteVLANAttachment call (skip missing node), got %d", mockVPC.CallCount("DeleteVLANAttachment"))
	}
}

// ─── DeleteVPCSubnet tests ───

func TestDeleteVPCSubnet_DeletesSubnet(t *testing.T) {
	annots := map[string]string{
		annotations.SubnetID: "subnet-to-delete",
	}
	obj := makeTestObj("test-cudn", annots)

	mockVPC := vpc.NewMockClient()
	var deletedID string
	mockVPC.DeleteSubnetFn = func(ctx context.Context, subnetID string) error {
		deletedID = subnetID
		return nil
	}

	err := DeleteVPCSubnet(context.Background(), mockVPC, obj)
	if err != nil {
		t.Fatalf("DeleteVPCSubnet() error = %v", err)
	}

	if deletedID != "subnet-to-delete" {
		t.Errorf("expected to delete 'subnet-to-delete', got %q", deletedID)
	}
	if mockVPC.CallCount("DeleteSubnet") != 1 {
		t.Errorf("expected 1 DeleteSubnet call, got %d", mockVPC.CallCount("DeleteSubnet"))
	}
}

func TestDeleteVPCSubnet_NilAnnotations_Noop(t *testing.T) {
	obj := makeTestObj("test-cudn", nil)

	mockVPC := vpc.NewMockClient()

	err := DeleteVPCSubnet(context.Background(), mockVPC, obj)
	if err != nil {
		t.Fatalf("DeleteVPCSubnet() error = %v", err)
	}

	if mockVPC.CallCount("DeleteSubnet") != 0 {
		t.Error("DeleteSubnet should not be called with nil annotations")
	}
}

func TestDeleteVPCSubnet_EmptySubnetID_Noop(t *testing.T) {
	annots := map[string]string{
		annotations.SubnetID: "",
	}
	obj := makeTestObj("test-cudn", annots)

	mockVPC := vpc.NewMockClient()

	err := DeleteVPCSubnet(context.Background(), mockVPC, obj)
	if err != nil {
		t.Fatalf("DeleteVPCSubnet() error = %v", err)
	}

	if mockVPC.CallCount("DeleteSubnet") != 0 {
		t.Error("DeleteSubnet should not be called with empty subnet-id")
	}
}

func TestDeleteVPCSubnet_VPCError(t *testing.T) {
	annots := map[string]string{
		annotations.SubnetID: "subnet-failing",
	}
	obj := makeTestObj("test-cudn", annots)

	mockVPC := vpc.NewMockClient()
	mockVPC.DeleteSubnetFn = func(ctx context.Context, subnetID string) error {
		return fmt.Errorf("VPC API error: conflict")
	}

	err := DeleteVPCSubnet(context.Background(), mockVPC, obj)
	if err == nil {
		t.Fatal("expected error from VPC API failure")
	}
}

func TestDeleteVPCSubnet_NotFound_Tolerates(t *testing.T) {
	annots := map[string]string{
		annotations.SubnetID: "subnet-gone",
	}
	obj := makeTestObj("test-cudn", annots)

	mockVPC := vpc.NewMockClient()
	mockVPC.DeleteSubnetFn = func(ctx context.Context, subnetID string) error {
		return fmt.Errorf("VPC API error: subnet not found")
	}

	err := DeleteVPCSubnet(context.Background(), mockVPC, obj)
	if err != nil {
		t.Fatalf("expected 404 to be tolerated, got error: %v", err)
	}
}

// ─── ParseAttachments tests ───

func TestParseAttachments(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  map[string]string
	}{
		{
			name:  "empty string",
			input: "",
			want:  map[string]string{},
		},
		{
			name:  "single entry",
			input: "node1:att-123",
			want:  map[string]string{"node1": "att-123"},
		},
		{
			name:  "multiple entries",
			input: "node1:att-123,node2:att-456,node3:att-789",
			want:  map[string]string{"node1": "att-123", "node2": "att-456", "node3": "att-789"},
		},
		{
			name:  "malformed entry skipped",
			input: "node1:att-123,badentry,node2:att-456",
			want:  map[string]string{"node1": "att-123", "node2": "att-456"},
		},
		{
			name:  "entry with spaces",
			input: " node1 : att-123 , node2 : att-456 ",
			want:  map[string]string{"node1": "att-123", "node2": "att-456"},
		},
		{
			name:  "colon in value",
			input: "node1:att:123",
			want:  map[string]string{"node1": "att:123"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseAttachments(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("ParseAttachments(%q) returned %d entries, want %d", tt.input, len(got), len(tt.want))
				return
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("ParseAttachments(%q)[%q] = %q, want %q", tt.input, k, got[k], v)
				}
			}
		})
	}
}

// ─── SerializeAttachments tests ───

func TestSerializeAttachments(t *testing.T) {
	tests := []struct {
		name  string
		input map[string]string
	}{
		{
			name:  "empty map",
			input: map[string]string{},
		},
		{
			name:  "single entry",
			input: map[string]string{"node1": "att-123"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SerializeAttachments(tt.input)
			if len(tt.input) == 0 {
				if result != "" {
					t.Errorf("SerializeAttachments(empty) = %q, want empty", result)
				}
				return
			}
			// Parse it back and verify roundtrip
			parsed := ParseAttachments(result)
			if len(parsed) != len(tt.input) {
				t.Errorf("roundtrip: got %d entries, want %d", len(parsed), len(tt.input))
			}
			for k, v := range tt.input {
				if parsed[k] != v {
					t.Errorf("roundtrip: [%q] = %q, want %q", k, parsed[k], v)
				}
			}
		})
	}
}

func TestSerializeAttachments_Roundtrip(t *testing.T) {
	input := map[string]string{
		"bm-node-1": "vla-abc",
		"bm-node-2": "vla-def",
		"bm-node-3": "vla-ghi",
	}
	serialized := SerializeAttachments(input)
	parsed := ParseAttachments(serialized)

	if len(parsed) != len(input) {
		t.Fatalf("roundtrip: got %d entries, want %d", len(parsed), len(input))
	}
	for k, v := range input {
		if parsed[k] != v {
			t.Errorf("roundtrip: [%q] = %q, want %q", k, parsed[k], v)
		}
	}

	// Verify the serialized format contains all entries
	for k, v := range input {
		expected := k + ":" + v
		if !strings.Contains(serialized, expected) {
			t.Errorf("serialized %q does not contain %q", serialized, expected)
		}
	}
}

// ─── IsBareMetalNode tests ───

func TestIsBareMetalNode(t *testing.T) {
	tests := []struct {
		name         string
		instanceType string
		want         bool
	}{
		{"bare metal bx2", "bx2-metal-96x384", true},
		{"bare metal cx2d", "cx2d-metal-96x384", true},
		{"virtual bx2", "bx2-4x16", false},
		{"virtual cx2", "cx2-8x16", false},
		{"empty instance type", "", false},
		{"metal keyword only", "metal", true},
		{"contains metal mid-word", "not-a-metallic-node", true}, // "metal" is a substring
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{},
				},
			}
			if tt.instanceType != "" {
				node.Labels["node.kubernetes.io/instance-type"] = tt.instanceType
			}
			got := IsBareMetalNode(node)
			if got != tt.want {
				t.Errorf("IsBareMetalNode(instanceType=%q) = %v, want %v", tt.instanceType, got, tt.want)
			}
		})
	}
}

func TestIsBareMetalNode_NilLabels(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			// No labels
		},
	}
	got := IsBareMetalNode(node)
	if got {
		t.Error("IsBareMetalNode should return false for node without labels")
	}
}

// ─── ExtractBMServerID tests ───

func TestExtractBMServerID(t *testing.T) {
	tests := []struct {
		name       string
		providerID string
		want       string
	}{
		{
			name:       "standard IBM provider ID",
			providerID: "ibm://acct123/us-south/us-south-1/bm-server-abc",
			want:       "bm-server-abc",
		},
		{
			name:       "without ibm:// prefix",
			providerID: "acct123/us-south/us-south-1/bm-server-xyz",
			want:       "bm-server-xyz",
		},
		{
			name:       "empty string",
			providerID: "",
			want:       "",
		},
		{
			name:       "too few parts",
			providerID: "ibm://acct/region",
			want:       "",
		},
		{
			name:       "exactly 3 parts (no server ID)",
			providerID: "ibm://acct/region/zone",
			want:       "",
		},
		{
			name:       "extra parts",
			providerID: "ibm://acct/region/zone/server-id/extra",
			want:       "server-id",
		},
		{
			name:       "server ID with dashes",
			providerID: "ibm://acct-123/us-south/us-south-1/bms-abcd-1234-efgh-5678",
			want:       "bms-abcd-1234-efgh-5678",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractBMServerID(tt.providerID)
			if got != tt.want {
				t.Errorf("ExtractBMServerID(%q) = %q, want %q", tt.providerID, got, tt.want)
			}
		})
	}
}

// ─── resolveNodeBMServerID tests (indirect via DeleteVLANAttachments) ───

func TestResolveNodeBMServerID_ExistingNode(t *testing.T) {
	scheme := newTestScheme()

	node := makeBareMetalNode("test-node", "ibm://acct/us-south/us-south-1/server-xyz")

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(node).
		Build()

	got := resolveNodeBMServerID(context.Background(), fakeClient, "test-node")
	if got != "server-xyz" {
		t.Errorf("resolveNodeBMServerID() = %q, want %q", got, "server-xyz")
	}
}

func TestResolveNodeBMServerID_NodeNotFound(t *testing.T) {
	scheme := newTestScheme()

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	got := resolveNodeBMServerID(context.Background(), fakeClient, "nonexistent")
	if got != "" {
		t.Errorf("resolveNodeBMServerID() = %q, want empty for missing node", got)
	}
}

// ─── EnsureVPCSubnet Public Gateway passthrough tests ───

func TestEnsureVPCSubnet_PassesThroughPublicGatewayID(t *testing.T) {
	scheme := newTestScheme()
	annots := map[string]string{
		annotations.VPCID:           "vpc-123",
		annotations.Zone:            "us-south-1",
		annotations.CIDR:            "10.240.64.0/24",
		annotations.ACLID:           "acl-1",
		annotations.PublicGatewayID: "pgw-abc123",
	}
	obj := makeTestObj("pgw-cudn", annots)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(obj).
		Build()

	mockVPC := vpc.NewMockClient()
	var capturedOpts vpc.CreateSubnetOptions
	mockVPC.CreateSubnetFn = func(ctx context.Context, opts vpc.CreateSubnetOptions) (*vpc.Subnet, error) {
		capturedOpts = opts
		return &vpc.Subnet{
			ID:     "subnet-pgw-1",
			Name:   opts.Name,
			CIDR:   opts.CIDR,
			Status: "available",
		}, nil
	}

	created, err := EnsureVPCSubnet(context.Background(), fakeClient, mockVPC, obj, "cluster-abc", "test")
	if err != nil {
		t.Fatalf("EnsureVPCSubnet() error = %v", err)
	}
	if !created {
		t.Error("expected created=true for new subnet")
	}

	if capturedOpts.PublicGatewayID != "pgw-abc123" {
		t.Errorf("expected PublicGatewayID='pgw-abc123', got %q", capturedOpts.PublicGatewayID)
	}
}

func TestEnsureVPCSubnet_AdoptRejectsCIDRMismatch(t *testing.T) {
	scheme := newTestScheme()
	annots := map[string]string{
		annotations.VPCID: "vpc-123",
		annotations.Zone:  "us-south-1",
		annotations.CIDR:  "10.240.64.0/24",
	}
	obj := makeTestObj("mismatch-cudn", annots)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(obj).
		Build()

	mockVPC := vpc.NewMockClient()
	// ListSubnets returns a subnet with the expected name but a different CIDR
	mockVPC.ListSubnetsFn = func(ctx context.Context, vpcID string) ([]vpc.Subnet, error) {
		return []vpc.Subnet{
			{
				ID:     "subnet-wrong-cidr",
				Name:   "roks-cluster-abc-mismatch-cudn",
				CIDR:   "10.240.99.0/24",
				Status: "available",
			},
		}, nil
	}

	created, err := EnsureVPCSubnet(context.Background(), fakeClient, mockVPC, obj, "cluster-abc", "test")
	if err == nil {
		t.Fatal("expected error for CIDR mismatch on adoption")
	}
	if created {
		t.Error("expected created=false on CIDR mismatch")
	}
	if !strings.Contains(err.Error(), "has CIDR") {
		t.Errorf("expected CIDR mismatch error, got: %v", err)
	}

	// Verify error was written to annotation
	updated := &unstructured.Unstructured{}
	updated.SetGroupVersionKind(obj.GroupVersionKind())
	if getErr := fakeClient.Get(context.Background(), types.NamespacedName{Name: "mismatch-cudn"}, updated); getErr != nil {
		t.Fatalf("Failed to get updated object: %v", getErr)
	}
	updatedAnnots := updated.GetAnnotations()
	if updatedAnnots[annotations.SubnetStatus] != "error" {
		t.Errorf("expected subnet-status='error', got %q", updatedAnnots[annotations.SubnetStatus])
	}
	if !strings.Contains(updatedAnnots[annotations.SubnetError], "10.240.99.0/24") {
		t.Errorf("expected subnet-error to mention existing CIDR, got %q", updatedAnnots[annotations.SubnetError])
	}

	// CreateSubnet should NOT have been called
	if mockVPC.CallCount("CreateSubnet") != 0 {
		t.Error("CreateSubnet should not be called on CIDR mismatch")
	}
}

func TestEnsureVPCSubnet_AdoptMatchingCIDR(t *testing.T) {
	scheme := newTestScheme()
	annots := map[string]string{
		annotations.VPCID: "vpc-123",
		annotations.Zone:  "us-south-1",
		annotations.CIDR:  "10.240.64.0/24",
	}
	obj := makeTestObj("adopt-cudn", annots)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(obj).
		Build()

	mockVPC := vpc.NewMockClient()
	mockVPC.ListSubnetsFn = func(ctx context.Context, vpcID string) ([]vpc.Subnet, error) {
		return []vpc.Subnet{
			{
				ID:     "subnet-existing",
				Name:   "roks-cluster-abc-adopt-cudn",
				CIDR:   "10.240.64.0/24",
				Status: "available",
			},
		}, nil
	}

	created, err := EnsureVPCSubnet(context.Background(), fakeClient, mockVPC, obj, "cluster-abc", "test")
	if err != nil {
		t.Fatalf("EnsureVPCSubnet() error = %v", err)
	}
	if !created {
		t.Error("expected created=true for adopted subnet")
	}

	// Verify adoption wrote the correct subnet ID
	updated := &unstructured.Unstructured{}
	updated.SetGroupVersionKind(obj.GroupVersionKind())
	if getErr := fakeClient.Get(context.Background(), types.NamespacedName{Name: "adopt-cudn"}, updated); getErr != nil {
		t.Fatalf("Failed to get updated object: %v", getErr)
	}
	updatedAnnots := updated.GetAnnotations()
	if updatedAnnots[annotations.SubnetID] != "subnet-existing" {
		t.Errorf("expected subnet-id='subnet-existing', got %q", updatedAnnots[annotations.SubnetID])
	}

	// CreateSubnet should NOT have been called (adopted instead)
	if mockVPC.CallCount("CreateSubnet") != 0 {
		t.Error("CreateSubnet should not be called when adopting existing subnet")
	}
}

func TestEnsureVPCSubnet_OmitsPublicGatewayWhenNotSet(t *testing.T) {
	scheme := newTestScheme()
	annots := map[string]string{
		annotations.VPCID: "vpc-123",
		annotations.Zone:  "us-south-1",
		annotations.CIDR:  "10.240.64.0/24",
		annotations.ACLID: "acl-1",
	}
	obj := makeTestObj("no-pgw-cudn", annots)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(obj).
		Build()

	mockVPC := vpc.NewMockClient()
	var capturedOpts vpc.CreateSubnetOptions
	mockVPC.CreateSubnetFn = func(ctx context.Context, opts vpc.CreateSubnetOptions) (*vpc.Subnet, error) {
		capturedOpts = opts
		return &vpc.Subnet{
			ID:     "subnet-no-pgw-1",
			Name:   opts.Name,
			CIDR:   opts.CIDR,
			Status: "available",
		}, nil
	}

	created, err := EnsureVPCSubnet(context.Background(), fakeClient, mockVPC, obj, "cluster-abc", "test")
	if err != nil {
		t.Fatalf("EnsureVPCSubnet() error = %v", err)
	}
	if !created {
		t.Error("expected created=true for new subnet")
	}

	if capturedOpts.PublicGatewayID != "" {
		t.Errorf("expected PublicGatewayID='', got %q", capturedOpts.PublicGatewayID)
	}
}
