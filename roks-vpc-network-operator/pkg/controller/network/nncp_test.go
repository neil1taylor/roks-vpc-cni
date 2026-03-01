package network

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/IBM/roks-vpc-network-operator/pkg/annotations"
)

func makeLocalNetCUDN(name string, annots map[string]string, physicalNetworkName string) *unstructured.Unstructured {
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
	if physicalNetworkName != "" {
		_ = unstructured.SetNestedField(obj.Object, physicalNetworkName, "spec", "network", "localnet", "physicalNetworkName")
	}
	return obj
}

func TestEnsureNNCP_CreatesNNCP(t *testing.T) {
	scheme := newTestScheme()
	obj := makeLocalNetCUDN("localnet-1", map[string]string{}, "localnet-1")

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(obj).
		Build()

	cfg := NNCPConfig{
		Enabled:      true,
		BridgeName:   "br-localnet",
		SecondaryNIC: "bond1",
		NodeSelector: map[string]string{"node-role.kubernetes.io/worker": ""},
	}

	err := EnsureNNCP(context.Background(), fakeClient, obj, "localnet-1", cfg)
	if err != nil {
		t.Fatalf("EnsureNNCP() error = %v", err)
	}

	// Verify NNCP was created
	nncp := &unstructured.Unstructured{}
	nncp.SetGroupVersionKind(nncpGVK)
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "localnet-localnet-1"}, nncp); err != nil {
		t.Fatalf("Expected NNCP to be created, got error: %v", err)
	}

	// Verify labels
	labels := nncp.GetLabels()
	if labels["vpc.roks.ibm.com/managed-by"] != "roks-vpc-network-operator" {
		t.Errorf("expected managed-by label, got %v", labels)
	}
	if labels["vpc.roks.ibm.com/network"] != "localnet-1" {
		t.Errorf("expected network label 'localnet-1', got %q", labels["vpc.roks.ibm.com/network"])
	}

	// Verify the annotation was set on the source object
	updated := &unstructured.Unstructured{}
	updated.SetGroupVersionKind(obj.GroupVersionKind())
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "localnet-1"}, updated); err != nil {
		t.Fatalf("Failed to get updated object: %v", err)
	}
	if updated.GetAnnotations()[annotations.NNCPName] != "localnet-localnet-1" {
		t.Errorf("expected NNCPName annotation = 'localnet-localnet-1', got %q", updated.GetAnnotations()[annotations.NNCPName])
	}

	// Verify desiredState structure
	interfaces, found, _ := unstructured.NestedSlice(nncp.Object, "spec", "desiredState", "interfaces")
	if !found || len(interfaces) != 1 {
		t.Fatalf("expected 1 interface in desiredState, found=%v len=%d", found, len(interfaces))
	}
	iface := interfaces[0].(map[string]interface{})
	if iface["name"] != "br-localnet" {
		t.Errorf("expected bridge name 'br-localnet', got %v", iface["name"])
	}
	if iface["type"] != "ovs-bridge" {
		t.Errorf("expected type 'ovs-bridge', got %v", iface["type"])
	}

	// Verify bridge-mappings
	mappings, found, _ := unstructured.NestedSlice(nncp.Object, "spec", "desiredState", "ovn", "bridge-mappings")
	if !found || len(mappings) != 1 {
		t.Fatalf("expected 1 bridge-mapping, found=%v len=%d", found, len(mappings))
	}
	mapping := mappings[0].(map[string]interface{})
	if mapping["localnet"] != "localnet-1" {
		t.Errorf("expected localnet 'localnet-1', got %v", mapping["localnet"])
	}
	if mapping["bridge"] != "br-localnet" {
		t.Errorf("expected bridge 'br-localnet', got %v", mapping["bridge"])
	}
	if mapping["state"] != "present" {
		t.Errorf("expected state 'present', got %v", mapping["state"])
	}
}

func TestEnsureNNCP_IdempotentSkip(t *testing.T) {
	scheme := newTestScheme()

	// Pre-create the NNCP
	existingNNCP := &unstructured.Unstructured{}
	existingNNCP.SetGroupVersionKind(nncpGVK)
	existingNNCP.SetName("localnet-localnet-1")

	obj := makeLocalNetCUDN("localnet-1", map[string]string{
		annotations.NNCPName: "localnet-localnet-1",
	}, "localnet-1")

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(obj, existingNNCP).
		Build()

	cfg := NNCPConfig{
		Enabled:      true,
		BridgeName:   "br-localnet",
		SecondaryNIC: "bond1",
	}

	err := EnsureNNCP(context.Background(), fakeClient, obj, "localnet-1", cfg)
	if err != nil {
		t.Fatalf("EnsureNNCP() should be idempotent, got error = %v", err)
	}
}

func TestEnsureNNCP_Disabled_Noop(t *testing.T) {
	scheme := newTestScheme()
	obj := makeLocalNetCUDN("localnet-1", map[string]string{}, "")

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(obj).
		Build()

	cfg := NNCPConfig{Enabled: false}

	err := EnsureNNCP(context.Background(), fakeClient, obj, "localnet-1", cfg)
	if err != nil {
		t.Fatalf("EnsureNNCP() error = %v", err)
	}

	// Verify no NNCP was created
	nncp := &unstructured.Unstructured{}
	nncp.SetGroupVersionKind(nncpGVK)
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "localnet-localnet-1"}, nncp); err == nil {
		t.Error("Expected NNCP to NOT be created when disabled")
	}
}

func TestEnsureNNCP_FallbackToObjectName(t *testing.T) {
	scheme := newTestScheme()
	obj := makeLocalNetCUDN("my-network", map[string]string{}, "")

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(obj).
		Build()

	cfg := NNCPConfig{
		Enabled:      true,
		BridgeName:   "br-localnet",
		SecondaryNIC: "bond1",
	}

	// Pass empty physicalNetworkName — should fall back to obj name
	err := EnsureNNCP(context.Background(), fakeClient, obj, "", cfg)
	if err != nil {
		t.Fatalf("EnsureNNCP() error = %v", err)
	}

	nncp := &unstructured.Unstructured{}
	nncp.SetGroupVersionKind(nncpGVK)
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "localnet-my-network"}, nncp); err != nil {
		t.Fatalf("Expected NNCP with fallback name, got error: %v", err)
	}
}

func TestDeleteNNCP_DeletesExisting(t *testing.T) {
	scheme := newTestScheme()

	existingNNCP := &unstructured.Unstructured{}
	existingNNCP.SetGroupVersionKind(nncpGVK)
	existingNNCP.SetName("localnet-localnet-1")

	obj := makeLocalNetCUDN("localnet-1", map[string]string{
		annotations.NNCPName: "localnet-localnet-1",
	}, "")

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(obj, existingNNCP).
		Build()

	DeleteNNCP(context.Background(), fakeClient, obj)

	// Verify NNCP was deleted
	nncp := &unstructured.Unstructured{}
	nncp.SetGroupVersionKind(nncpGVK)
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "localnet-localnet-1"}, nncp); err == nil {
		t.Error("Expected NNCP to be deleted")
	}
}

func TestDeleteNNCP_ToleratesNotFound(t *testing.T) {
	scheme := newTestScheme()
	obj := makeLocalNetCUDN("localnet-1", map[string]string{
		annotations.NNCPName: "localnet-localnet-1",
	}, "")

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(obj).
		Build()

	// Should not panic or error
	DeleteNNCP(context.Background(), fakeClient, obj)
}

func TestDeleteNNCP_NilAnnotations_Noop(t *testing.T) {
	scheme := newTestScheme()
	obj := makeLocalNetCUDN("localnet-1", nil, "")

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(obj).
		Build()

	// Should not panic
	DeleteNNCP(context.Background(), fakeClient, obj)
}

func TestExtractPhysicalNetworkName_FromCUDNSpec(t *testing.T) {
	obj := makeLocalNetCUDN("my-cudn", nil, "custom-localnet")
	got := ExtractPhysicalNetworkName(obj)
	if got != "custom-localnet" {
		t.Errorf("ExtractPhysicalNetworkName() = %q, want 'custom-localnet'", got)
	}
}

func TestExtractPhysicalNetworkName_FallbackToName(t *testing.T) {
	obj := makeLocalNetCUDN("my-cudn", nil, "")
	got := ExtractPhysicalNetworkName(obj)
	if got != "my-cudn" {
		t.Errorf("ExtractPhysicalNetworkName() = %q, want 'my-cudn'", got)
	}
}

func TestEnsureNNCP_DefaultConfig(t *testing.T) {
	scheme := newTestScheme()
	obj := makeLocalNetCUDN("net1", map[string]string{}, "net1")

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(obj).
		Build()

	// Empty BridgeName/SecondaryNIC/NodeSelector should use defaults
	cfg := NNCPConfig{Enabled: true}

	err := EnsureNNCP(context.Background(), fakeClient, obj, "net1", cfg)
	if err != nil {
		t.Fatalf("EnsureNNCP() error = %v", err)
	}

	nncp := &unstructured.Unstructured{}
	nncp.SetGroupVersionKind(nncpGVK)
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "localnet-net1"}, nncp); err != nil {
		t.Fatalf("Expected NNCP with defaults, got error: %v", err)
	}

	// Verify default nodeSelector
	ns, found, _ := unstructured.NestedStringMap(nncp.Object, "spec", "nodeSelector")
	if !found {
		t.Fatal("expected nodeSelector to be set")
	}
	if ns["node-role.kubernetes.io/worker"] != "" {
		t.Errorf("expected default worker nodeSelector, got %v", ns)
	}

	// Verify default bridge name in interfaces
	interfaces, _, _ := unstructured.NestedSlice(nncp.Object, "spec", "desiredState", "interfaces")
	iface := interfaces[0].(map[string]interface{})
	if iface["name"] != "br-localnet" {
		t.Errorf("expected default bridge 'br-localnet', got %v", iface["name"])
	}

	// Verify default secondary NIC in bridge ports
	bridge := iface["bridge"].(map[string]interface{})
	ports := bridge["port"].([]interface{})
	port := ports[0].(map[string]interface{})
	if port["name"] != "bond1" {
		t.Errorf("expected default NIC 'bond1', got %v", port["name"])
	}
}
