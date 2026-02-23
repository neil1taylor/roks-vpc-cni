package node

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
		{
			name:       "extra parts",
			providerID: "ibm://acct/us-south/us-south-1/bm-123/extra",
			want:       "bm-123",
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
			node := &corev1.Node{}
			node.Labels = map[string]string{}
			if tt.instanceType != "" {
				node.Labels["node.kubernetes.io/instance-type"] = tt.instanceType
			}
			got := isBareMetalNode(node)
			if got != tt.want {
				t.Errorf("isBareMetalNode(%q) = %v, want %v", tt.instanceType, got, tt.want)
			}
		})
	}
}

func TestIsNodeReady(t *testing.T) {
	tests := []struct {
		name string
		node *corev1.Node
		want bool
	}{
		{
			name: "ready node",
			node: &corev1.Node{
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
					},
				},
			},
			want: true,
		},
		{
			name: "not ready node",
			node: &corev1.Node{
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{Type: corev1.NodeReady, Status: corev1.ConditionFalse},
					},
				},
			},
			want: false,
		},
		{
			name: "no conditions",
			node: &corev1.Node{},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isNodeReady(tt.node)
			if got != tt.want {
				t.Errorf("isNodeReady() = %v, want %v", got, tt.want)
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
		{"empty", "", map[string]string{}},
		{"single", "node1:att-1", map[string]string{"node1": "att-1"}},
		{"multiple", "node1:att-1,node2:att-2", map[string]string{"node1": "att-1", "node2": "att-2"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseAttachments(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("parseAttachments(%q) len = %d, want %d", tt.input, len(got), len(tt.want))
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("parseAttachments(%q)[%q] = %q, want %q", tt.input, k, got[k], v)
				}
			}
		})
	}
}
