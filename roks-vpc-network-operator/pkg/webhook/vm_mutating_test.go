package webhook

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	v1alpha1 "github.com/IBM/roks-vpc-network-operator/api/v1alpha1"
	"github.com/IBM/roks-vpc-network-operator/pkg/annotations"
	"github.com/IBM/roks-vpc-network-operator/pkg/controller/network"
	"github.com/IBM/roks-vpc-network-operator/pkg/vpc"
)

func TestHandle_NonCreate(t *testing.T) {
	w := &VMMutatingWebhook{
		VPC:       vpc.NewMockClient(),
		ClusterID: "test-cluster",
	}

	resp := w.Handle(context.Background(), admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: "UPDATE",
		},
	})

	if !resp.Allowed {
		t.Errorf("expected non-CREATE request to be allowed, got denied: %s", resp.Result.Message)
	}
}

func TestHandle_NoLocalNetInterfaces(t *testing.T) {
	w := &VMMutatingWebhook{
		VPC:       vpc.NewMockClient(),
		ClusterID: "test-cluster",
	}

	// VM with no multus networks
	vmObj := map[string]interface{}{
		"apiVersion": "kubevirt.io/v1",
		"kind":       "VirtualMachine",
		"metadata": map[string]interface{}{
			"name":      "test-vm",
			"namespace": "default",
		},
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"networks": []interface{}{
						map[string]interface{}{
							"name": "default",
							"pod":  map[string]interface{}{},
						},
					},
				},
			},
		},
	}

	raw, _ := json.Marshal(vmObj)
	resp := w.Handle(context.Background(), admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: "CREATE",
			Name:      "test-vm",
			Namespace: "default",
			Object:    runtime.RawExtension{Raw: raw},
		},
	})

	if !resp.Allowed {
		t.Errorf("expected VM without localnet interfaces to be allowed, got denied: %s", resp.Result.Message)
	}
}

func TestFindLocalNetNetworks(t *testing.T) {
	vmObj := map[string]interface{}{
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"networks": []interface{}{
						map[string]interface{}{
							"name": "default",
							"pod":  map[string]interface{}{},
						},
						map[string]interface{}{
							"name": "localnet-1",
							"multus": map[string]interface{}{
								"networkName": "my-localnet-cudn",
							},
						},
					},
					"domain": map[string]interface{}{
						"devices": map[string]interface{}{
							"interfaces": []interface{}{
								map[string]interface{}{
									"name":       "default",
									"masquerade": map[string]interface{}{},
								},
								map[string]interface{}{
									"name":   "localnet-1",
									"bridge": map[string]interface{}{},
								},
							},
						},
					},
				},
			},
		},
	}

	names, paths := findLocalNetNetworks(vmObj)

	if len(names) != 1 {
		t.Fatalf("expected 1 localnet network, got %d", len(names))
	}
	if names[0] != "my-localnet-cudn" {
		t.Errorf("expected network name 'my-localnet-cudn', got %q", names[0])
	}
	if len(paths) != 1 {
		t.Fatalf("expected 1 interface path, got %d", len(paths))
	}
	// Should point to interfaces[1]
	expectedIdx := "1"
	if paths[0][len(paths[0])-1] != expectedIdx {
		t.Errorf("expected interface index %q, got %q", expectedIdx, paths[0][len(paths[0])-1])
	}
}

func TestFindLocalNetNetworks_WithNamespace(t *testing.T) {
	vmObj := map[string]interface{}{
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"networks": []interface{}{
						map[string]interface{}{
							"name": "net1",
							"multus": map[string]interface{}{
								"networkName": "my-ns/my-cudn",
							},
						},
					},
					"domain": map[string]interface{}{
						"devices": map[string]interface{}{
							"interfaces": []interface{}{
								map[string]interface{}{
									"name":   "net1",
									"bridge": map[string]interface{}{},
								},
							},
						},
					},
				},
			},
		},
	}

	names, _ := findLocalNetNetworks(vmObj)
	if len(names) != 1 || names[0] != "my-cudn" {
		t.Errorf("expected CUDN name 'my-cudn' extracted from 'my-ns/my-cudn', got %v", names)
	}
}

func TestExtractCUDNName(t *testing.T) {
	tests := []struct {
		ref  string
		want string
	}{
		{"my-cudn", "my-cudn"},
		{"default/my-cudn", "my-cudn"},
		{"kube-system/test-net", "test-net"},
	}

	for _, tt := range tests {
		got := extractCUDNName(tt.ref)
		if got != tt.want {
			t.Errorf("extractCUDNName(%q) = %q, want %q", tt.ref, got, tt.want)
		}
	}
}

func TestInjectCloudInitNetworkConfig(t *testing.T) {
	vmObj := map[string]interface{}{
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"volumes": []interface{}{
						map[string]interface{}{
							"name":             "cloudinit",
							"cloudInitNoCloud": map[string]interface{}{},
						},
					},
				},
			},
		},
	}

	injectCloudInitNetworkConfig(vmObj, []localNetIPEntry{{ip: "10.240.64.12", mac: "02:00:01:AA:BB:CC", name: "net1"}})

	volumes, _ := getNestedSlice(vmObj, "spec", "template", "spec", "volumes")
	if len(volumes) != 1 {
		t.Fatal("expected 1 volume")
	}
	vol := volumes[0].(map[string]interface{})
	cloudInit := vol["cloudInitNoCloud"].(map[string]interface{})
	networkData, ok := cloudInit["networkData"].(string)
	if !ok {
		t.Fatal("expected networkData string in cloudInit")
	}

	if networkData == "" {
		t.Error("expected non-empty network data")
	}

	// Verify it contains the IP
	if !strings.Contains(networkData, "10.240.64.12/24") {
		t.Errorf("network data should contain IP, got: %s", networkData)
	}
	if !strings.Contains(networkData, "10.240.64.1") {
		t.Errorf("network data should contain gateway, got: %s", networkData)
	}
}

func TestSplitTrimmed(t *testing.T) {
	tests := []struct {
		input string
		sep   string
		want  []string
	}{
		{"", ",", nil},
		{"a,b,c", ",", []string{"a", "b", "c"}},
		{"  a , b , c  ", ",", []string{"a", "b", "c"}},
		{"sg-1, sg-2", ",", []string{"sg-1", "sg-2"}},
	}

	for _, tt := range tests {
		got := splitTrimmed(tt.input, tt.sep)
		if len(got) != len(tt.want) {
			t.Errorf("splitTrimmed(%q, %q) = %v, want %v", tt.input, tt.sep, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("splitTrimmed(%q, %q)[%d] = %q, want %q", tt.input, tt.sep, i, got[i], tt.want[i])
			}
		}
	}
}

func TestContainsString(t *testing.T) {
	if !containsString([]string{"a", "b"}, "a") {
		t.Error("expected to find 'a'")
	}
	if containsString([]string{"a", "b"}, "c") {
		t.Error("should not find 'c'")
	}
	if containsString(nil, "a") {
		t.Error("should not find in nil slice")
	}
}

// newTestScheme builds a scheme with core k8s types, our CRD types, and
// unstructured OVN CUDN/UDN types for use with the fake client.
func newTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(s)
	_ = v1alpha1.AddToScheme(s)

	// Register unstructured CUDN GVK so the fake client can serve it
	s.AddKnownTypeWithName(
		schema.GroupVersionKind{Group: "k8s.ovn.org", Version: "v1", Kind: "ClusterUserDefinedNetwork"},
		&unstructured.Unstructured{},
	)
	s.AddKnownTypeWithName(
		schema.GroupVersionKind{Group: "k8s.ovn.org", Version: "v1", Kind: "ClusterUserDefinedNetworkList"},
		&unstructured.UnstructuredList{},
	)
	return s
}

// makeLayer2CUDN creates an unstructured Layer2 CUDN for testing.
func makeLayer2CUDN(name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "k8s.ovn.org/v1",
			"kind":       "ClusterUserDefinedNetwork",
			"metadata": map[string]interface{}{
				"name": name,
			},
			"spec": map[string]interface{}{
				"network": map[string]interface{}{
					"topology": "Layer2",
					"layer2": map[string]interface{}{
						"role": "Secondary",
					},
				},
			},
		},
	}
}

// makeL2VMObj creates a VM JSON object with a single Layer2 multus interface.
func makeL2VMObj(vmName, namespace, networkRef string) map[string]interface{} {
	return map[string]interface{}{
		"apiVersion": "kubevirt.io/v1",
		"kind":       "VirtualMachine",
		"metadata": map[string]interface{}{
			"name":      vmName,
			"namespace": namespace,
		},
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"networks": []interface{}{
						map[string]interface{}{
							"name": "default",
							"pod":  map[string]interface{}{},
						},
						map[string]interface{}{
							"name": "l2net",
							"multus": map[string]interface{}{
								"networkName": networkRef,
							},
						},
					},
					"domain": map[string]interface{}{
						"devices": map[string]interface{}{
							"interfaces": []interface{}{
								map[string]interface{}{
									"name":       "default",
									"masquerade": map[string]interface{}{},
								},
								map[string]interface{}{
									"name":   "l2net",
									"bridge": map[string]interface{}{},
								},
							},
						},
					},
				},
			},
		},
	}
}

// extractNetworkInterfacesFromPatch parses the admission response patches to
// extract the network-interfaces annotation value from the mutated VM.
func extractNetworkInterfacesFromPatch(t *testing.T, originalRaw []byte, resp admission.Response) []network.VMNetworkInterface {
	t.Helper()

	// The response Patches field contains typed JSON patch operations.
	// The webhook builds the entire annotations map and sets it at
	// /metadata/annotations. Find that operation.
	for _, op := range resp.Patches {
		if op.Path == "/metadata/annotations" || op.Path == "/metadata/annotations/" {
			// Value is a map of annotations
			annotMap, ok := op.Value.(map[string]interface{})
			if !ok {
				t.Fatalf("expected annotations value to be a map, got %T", op.Value)
			}
			netIfacesStr, ok := annotMap[annotations.NetworkInterfaces].(string)
			if !ok {
				t.Fatalf("expected network-interfaces annotation to be a string, got %T", annotMap[annotations.NetworkInterfaces])
			}
			var netIfaces []network.VMNetworkInterface
			if err := json.Unmarshal([]byte(netIfacesStr), &netIfaces); err != nil {
				t.Fatalf("failed to unmarshal network-interfaces JSON: %v", err)
			}
			return netIfaces
		}
	}

	// If the annotations path was not found as a whole object, look for the
	// specific annotation key path (JSON Pointer with ~ escaping).
	annotKey := strings.ReplaceAll(annotations.NetworkInterfaces, "/", "~1")
	targetPath := "/metadata/annotations/" + annotKey
	for _, op := range resp.Patches {
		if op.Path == targetPath {
			netIfacesStr, ok := op.Value.(string)
			if !ok {
				t.Fatalf("expected annotation value to be string, got %T", op.Value)
			}
			var netIfaces []network.VMNetworkInterface
			if err := json.Unmarshal([]byte(netIfacesStr), &netIfaces); err != nil {
				t.Fatalf("failed to unmarshal network-interfaces JSON: %v", err)
			}
			return netIfaces
		}
	}

	// Debug: print all patch operations
	for i, op := range resp.Patches {
		t.Logf("patch[%d]: op=%s path=%s value=%v", i, op.Operation, op.Path, op.Value)
	}

	t.Fatal("could not find network-interfaces annotation in patch operations")
	return nil
}

func TestFindRouterGateway(t *testing.T) {
	s := newTestScheme()

	// Create a Ready VPCRouter with networks
	router := &v1alpha1.VPCRouter{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-router",
			Namespace: "default",
		},
		Spec: v1alpha1.VPCRouterSpec{
			Gateway: "gw-1",
			Networks: []v1alpha1.RouterNetwork{
				{Name: "my-l2-net", Address: "10.100.0.1/24"},
				{Name: "other-net", Address: "10.200.0.1/16"},
			},
		},
		Status: v1alpha1.VPCRouterStatus{
			Phase: "Ready",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(router).
		WithStatusSubresource(&v1alpha1.VPCRouter{}).
		Build()

	// Update the status via status subresource
	router.Status.Phase = "Ready"
	if err := fakeClient.Status().Update(context.Background(), router); err != nil {
		t.Fatalf("failed to update router status: %v", err)
	}

	w := &VMMutatingWebhook{K8s: fakeClient}

	// Should find the gateway for a matching network
	gw := w.findRouterGateway(context.Background(), "default", "my-l2-net")
	if gw != "10.100.0.1" {
		t.Errorf("expected gateway '10.100.0.1', got %q", gw)
	}

	// Should find gateway for the other network
	gw2 := w.findRouterGateway(context.Background(), "default", "other-net")
	if gw2 != "10.200.0.1" {
		t.Errorf("expected gateway '10.200.0.1', got %q", gw2)
	}

	// Should return empty for unknown network
	gw3 := w.findRouterGateway(context.Background(), "default", "nonexistent")
	if gw3 != "" {
		t.Errorf("expected empty gateway for unknown network, got %q", gw3)
	}

	// Should return empty for wrong namespace
	gw4 := w.findRouterGateway(context.Background(), "other-ns", "my-l2-net")
	if gw4 != "" {
		t.Errorf("expected empty gateway in wrong namespace, got %q", gw4)
	}
}

func TestFindRouterGateway_NotReady(t *testing.T) {
	s := newTestScheme()

	// Create a VPCRouter that is NOT Ready
	router := &v1alpha1.VPCRouter{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pending-router",
			Namespace: "default",
		},
		Spec: v1alpha1.VPCRouterSpec{
			Gateway: "gw-1",
			Networks: []v1alpha1.RouterNetwork{
				{Name: "my-l2-net", Address: "10.100.0.1/24"},
			},
		},
		Status: v1alpha1.VPCRouterStatus{
			Phase: "Pending",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(router).
		WithStatusSubresource(&v1alpha1.VPCRouter{}).
		Build()

	// Update status to Pending
	router.Status.Phase = "Pending"
	_ = fakeClient.Status().Update(context.Background(), router)

	w := &VMMutatingWebhook{K8s: fakeClient}

	gw := w.findRouterGateway(context.Background(), "default", "my-l2-net")
	if gw != "" {
		t.Errorf("expected empty gateway when router is not Ready, got %q", gw)
	}
}

func TestWebhook_L2WithGateway(t *testing.T) {
	s := newTestScheme()

	cudn := makeLayer2CUDN("my-l2-net")

	router := &v1alpha1.VPCRouter{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-router",
			Namespace: "default",
		},
		Spec: v1alpha1.VPCRouterSpec{
			Gateway: "gw-1",
			Networks: []v1alpha1.RouterNetwork{
				{Name: "my-l2-net", Address: "10.100.0.1/24"},
			},
		},
		Status: v1alpha1.VPCRouterStatus{
			Phase: "Ready",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(cudn, router).
		WithStatusSubresource(&v1alpha1.VPCRouter{}).
		Build()

	// Set the router status via status subresource
	router.Status.Phase = "Ready"
	if err := fakeClient.Status().Update(context.Background(), router); err != nil {
		t.Fatalf("failed to update router status: %v", err)
	}

	w := &VMMutatingWebhook{
		VPC:       vpc.NewMockClient(),
		K8s:       fakeClient,
		ClusterID: "test-cluster",
	}

	vmObj := makeL2VMObj("test-vm", "default", "my-l2-net")
	raw, _ := json.Marshal(vmObj)

	resp := w.Handle(context.Background(), admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: "CREATE",
			Name:      "test-vm",
			Namespace: "default",
			Object:    runtime.RawExtension{Raw: raw},
		},
	})

	if !resp.Allowed {
		t.Fatalf("expected VM to be allowed, got denied: %s", resp.Result.Message)
	}

	netIfaces := extractNetworkInterfacesFromPatch(t, raw, resp)

	if len(netIfaces) != 1 {
		t.Fatalf("expected 1 network interface, got %d", len(netIfaces))
	}

	iface := netIfaces[0]
	if iface.NetworkName != "my-l2-net" {
		t.Errorf("expected networkName 'my-l2-net', got %q", iface.NetworkName)
	}
	if iface.Topology != "Layer2" {
		t.Errorf("expected topology 'Layer2', got %q", iface.Topology)
	}
	if iface.Gateway != "10.100.0.1" {
		t.Errorf("expected gateway '10.100.0.1', got %q", iface.Gateway)
	}
}

func TestWebhook_L2WithoutGateway(t *testing.T) {
	s := newTestScheme()

	cudn := makeLayer2CUDN("my-l2-net-no-router")

	fakeClient := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(cudn).
		Build()

	w := &VMMutatingWebhook{
		VPC:       vpc.NewMockClient(),
		K8s:       fakeClient,
		ClusterID: "test-cluster",
	}

	vmObj := makeL2VMObj("test-vm", "default", "my-l2-net-no-router")
	raw, _ := json.Marshal(vmObj)

	resp := w.Handle(context.Background(), admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: "CREATE",
			Name:      "test-vm",
			Namespace: "default",
			Object:    runtime.RawExtension{Raw: raw},
		},
	})

	if !resp.Allowed {
		t.Fatalf("expected VM to be allowed, got denied: %s", resp.Result.Message)
	}

	netIfaces := extractNetworkInterfacesFromPatch(t, raw, resp)

	if len(netIfaces) != 1 {
		t.Fatalf("expected 1 network interface, got %d", len(netIfaces))
	}

	iface := netIfaces[0]
	if iface.NetworkName != "my-l2-net-no-router" {
		t.Errorf("expected networkName 'my-l2-net-no-router', got %q", iface.NetworkName)
	}
	if iface.Topology != "Layer2" {
		t.Errorf("expected topology 'Layer2', got %q", iface.Topology)
	}
	if iface.Gateway != "" {
		t.Errorf("expected empty gateway when no router exists, got %q", iface.Gateway)
	}
}

// containsString2 checks if s contains substr.
// Named with a "2" suffix to avoid collision with the webhook's containsString helper.
func containsString2(s, substr string) bool {
	return strings.Contains(s, substr)
}

// Suppress unused import
var _ = annotations.VNIID
