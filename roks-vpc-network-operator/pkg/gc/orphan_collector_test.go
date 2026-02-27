package gc

import (
	"testing"
)

func TestParseVNIName(t *testing.T) {
	tests := []struct {
		name      string
		vniName   string
		clusterID string
		wantNS    string
		wantVM    string
	}{
		{
			name:      "valid VNI name",
			vniName:   "roks-cluster-abc-default-my-vm",
			clusterID: "cluster-abc",
			wantNS:    "default",
			wantVM:    "my-vm",
		},
		{
			name:      "empty name",
			vniName:   "",
			clusterID: "cluster-abc",
			wantNS:    "",
			wantVM:    "",
		},
		{
			name:      "name too short",
			vniName:   "roks-cluster-abc-",
			clusterID: "cluster-abc",
			wantNS:    "",
			wantVM:    "",
		},
		{
			name:      "wrong prefix",
			vniName:   "other-prefix-default-my-vm",
			clusterID: "cluster-abc",
			wantNS:    "",
			wantVM:    "",
		},
		{
			name:      "no namespace separator",
			vniName:   "roks-cluster-abc-singleword",
			clusterID: "cluster-abc",
			wantNS:    "",
			wantVM:    "",
		},
		{
			name:      "different cluster",
			vniName:   "roks-other-cluster-ns-vm",
			clusterID: "other-cluster",
			wantNS:    "ns",
			wantVM:    "vm",
		},
		{
			name:      "multi-network VNI name",
			vniName:   "roks-cluster-abc-default-my-vm-localnet1",
			clusterID: "cluster-abc",
			wantNS:    "default",
			wantVM:    "my-vm-localnet1",
		},
		{
			name:      "multi-network VNI with dashes",
			vniName:   "roks-cluster-abc-myns-web-server-prod-net",
			clusterID: "cluster-abc",
			wantNS:    "myns",
			wantVM:    "web-server-prod-net",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ns, vm := parseVNIName(tt.vniName, tt.clusterID)
			if ns != tt.wantNS {
				t.Errorf("parseVNIName(%q, %q) namespace = %q, want %q", tt.vniName, tt.clusterID, ns, tt.wantNS)
			}
			if vm != tt.wantVM {
				t.Errorf("parseVNIName(%q, %q) vmName = %q, want %q", tt.vniName, tt.clusterID, vm, tt.wantVM)
			}
		})
	}
}
