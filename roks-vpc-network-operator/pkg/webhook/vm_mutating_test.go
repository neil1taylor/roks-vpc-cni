package webhook

import (
	"context"
	"encoding/json"
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/IBM/roks-vpc-network-operator/pkg/annotations"
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
									"name":   "default",
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

	injectCloudInitNetworkConfig(vmObj, []localNetIPEntry{{ip: "10.240.64.12", name: "net1"}})

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
	if !containsString2(networkData, "10.240.64.12/24") {
		t.Errorf("network data should contain IP, got: %s", networkData)
	}
	if !containsString2(networkData, "10.240.64.1") {
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

// helpers - can't reuse strings.Contains to avoid collision with webhook helpers
func containsString2(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Suppress unused import
var _ = annotations.VNIID
