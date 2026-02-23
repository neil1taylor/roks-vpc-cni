package cudn

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestExtractBMServerID(t *testing.T) {
	tests := []struct {
		name       string
		providerID string
		want       string
	}{
		{
			name:       "valid IBM provider ID",
			providerID: "ibm://acct123/us-south/us-south-1/bm-server-abc",
			want:       "bm-server-abc",
		},
		{
			name:       "valid without prefix",
			providerID: "acct123/us-south/us-south-1/bm-server-xyz",
			want:       "bm-server-xyz",
		},
		{
			name:       "empty",
			providerID: "",
			want:       "",
		},
		{
			name:       "too few parts",
			providerID: "ibm://acct/region",
			want:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractBMServerID(tt.providerID)
			if got != tt.want {
				t.Errorf("extractBMServerID(%q) = %q, want %q", tt.providerID, got, tt.want)
			}
		})
	}
}

func TestParseAttachments(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  map[string]string
	}{
		{
			name:  "empty",
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
			input: "node1:att-123,node2:att-456",
			want:  map[string]string{"node1": "att-123", "node2": "att-456"},
		},
		{
			name:  "malformed entry skipped",
			input: "node1:att-123,badentry,node2:att-456",
			want:  map[string]string{"node1": "att-123", "node2": "att-456"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseAttachments(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("parseAttachments(%q) returned %d entries, want %d", tt.input, len(got), len(tt.want))
				return
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("parseAttachments(%q)[%q] = %q, want %q", tt.input, k, got[k], v)
				}
			}
		})
	}
}

func TestSerializeAttachments(t *testing.T) {
	// Single entry for deterministic output
	m := map[string]string{"node1": "att-123"}
	got := serializeAttachments(m)
	if got != "node1:att-123" {
		t.Errorf("serializeAttachments() = %q, want %q", got, "node1:att-123")
	}

	// Empty map
	empty := serializeAttachments(map[string]string{})
	if empty != "" {
		t.Errorf("serializeAttachments(empty) = %q, want empty string", empty)
	}
}

func TestIsBareMetalNode(t *testing.T) {
	tests := []struct {
		name         string
		instanceType string
		want         bool
	}{
		{"bare metal", "bx2-metal-96x384", true},
		{"virtual", "bx2-4x16", false},
		{"empty", "", false},
		{"metal in name", "cx2d-metal-96x384", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := makeTestNode(tt.instanceType)
			got := isBareMetalNode(node)
			if got != tt.want {
				t.Errorf("isBareMetalNode(%q) = %v, want %v", tt.instanceType, got, tt.want)
			}
		})
	}
}

func makeTestNode(instanceType string) *corev1.Node {
	node := &corev1.Node{}
	node.Labels = map[string]string{}
	if instanceType != "" {
		node.Labels["node.kubernetes.io/instance-type"] = instanceType
	}
	return node
}
