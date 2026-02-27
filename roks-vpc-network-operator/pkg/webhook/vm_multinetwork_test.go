package webhook

import (
	"testing"

	"github.com/IBM/roks-vpc-network-operator/pkg/annotations"
)

func TestFindAllMultusNetworks_Multiple(t *testing.T) {
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
						map[string]interface{}{
							"name": "layer2-1",
							"multus": map[string]interface{}{
								"networkName": "my-layer2-cudn",
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
								map[string]interface{}{
									"name":   "layer2-1",
									"bridge": map[string]interface{}{},
								},
							},
						},
					},
				},
			},
		},
	}

	entries, paths := findAllMultusNetworks(vmObj)

	if len(entries) != 2 {
		t.Fatalf("expected 2 multus networks, got %d", len(entries))
	}

	if entries[0].networkRef != "my-localnet-cudn" {
		t.Errorf("expected first network 'my-localnet-cudn', got %q", entries[0].networkRef)
	}
	if entries[0].ifaceName != "localnet-1" {
		t.Errorf("expected first iface name 'localnet-1', got %q", entries[0].ifaceName)
	}

	if entries[1].networkRef != "my-layer2-cudn" {
		t.Errorf("expected second network 'my-layer2-cudn', got %q", entries[1].networkRef)
	}
	if entries[1].ifaceName != "layer2-1" {
		t.Errorf("expected second iface name 'layer2-1', got %q", entries[1].ifaceName)
	}

	// Verify interface paths point to correct indices
	if paths[0][len(paths[0])-1] != "1" {
		t.Errorf("expected first interface at index 1, got %q", paths[0][len(paths[0])-1])
	}
	if paths[1][len(paths[1])-1] != "2" {
		t.Errorf("expected second interface at index 2, got %q", paths[1][len(paths[1])-1])
	}
}

func TestExtractNetworkName(t *testing.T) {
	tests := []struct {
		ref  string
		want string
	}{
		{"my-cudn", "my-cudn"},
		{"default/my-udn", "my-udn"},
		{"kube-system/test-net", "test-net"},
	}

	for _, tt := range tests {
		got := extractNetworkName(tt.ref)
		if got != tt.want {
			t.Errorf("extractNetworkName(%q) = %q, want %q", tt.ref, got, tt.want)
		}
	}
}

func TestParseFIPNetworks(t *testing.T) {
	tests := []struct {
		name   string
		annots map[string]string
		want   map[string]bool
	}{
		{
			name:   "empty",
			annots: map[string]string{},
			want:   map[string]bool{},
		},
		{
			name: "single network",
			annots: map[string]string{
				annotations.FIPNetworks: "net1",
			},
			want: map[string]bool{"net1": true},
		},
		{
			name: "multiple networks",
			annots: map[string]string{
				annotations.FIPNetworks: "net1, net2, net3",
			},
			want: map[string]bool{"net1": true, "net2": true, "net3": true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseFIPNetworks(tt.annots)
			if len(got) != len(tt.want) {
				t.Errorf("parseFIPNetworks() returned %d entries, want %d", len(got), len(tt.want))
				return
			}
			for k := range tt.want {
				if !got[k] {
					t.Errorf("expected %q in result", k)
				}
			}
		})
	}
}

func TestInjectCloudInitNetworkConfig_MultipleIPs(t *testing.T) {
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

	entries := []localNetIPEntry{
		{ip: "10.240.64.12", name: "net1"},
		{ip: "10.240.65.20", name: "net2"},
	}

	injectCloudInitNetworkConfig(vmObj, entries)

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

	// Verify both IPs are present
	if !containsString2(networkData, "10.240.64.12/24") {
		t.Errorf("network data should contain first IP, got: %s", networkData)
	}
	if !containsString2(networkData, "10.240.65.20/24") {
		t.Errorf("network data should contain second IP, got: %s", networkData)
	}
	// Verify default route is only on first
	if !containsString2(networkData, "10.240.64.1") {
		t.Errorf("network data should contain first gateway, got: %s", networkData)
	}
	// Second interface should have its address but no default route
	if !containsString2(networkData, "enp1s0") {
		t.Errorf("expected first interface name enp1s0, got: %s", networkData)
	}
	if !containsString2(networkData, "enp2s0") {
		t.Errorf("expected second interface name enp2s0, got: %s", networkData)
	}
}

func TestInjectCloudInitNetworkConfig_Empty(t *testing.T) {
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

	// No entries — should not modify cloud-init
	injectCloudInitNetworkConfig(vmObj, nil)

	volumes, _ := getNestedSlice(vmObj, "spec", "template", "spec", "volumes")
	vol := volumes[0].(map[string]interface{})
	cloudInit := vol["cloudInitNoCloud"].(map[string]interface{})
	if _, ok := cloudInit["networkData"]; ok {
		t.Error("expected no networkData when no entries provided")
	}
}
