package gateway

import (
	"strings"
	"testing"

	v1alpha1 "github.com/IBM/roks-vpc-network-operator/api/v1alpha1"
)

func TestGenerateNftablesConfig_SNATOnly(t *testing.T) {
	nat := &v1alpha1.GatewayNAT{
		SNAT: []v1alpha1.SNATRule{
			{
				Source:            "10.100.0.0/24",
				TranslatedAddress: "10.240.1.5",
				Priority:          100,
			},
		},
	}

	result := GenerateNftablesConfig(nat, "10.240.1.99")

	if !strings.Contains(result, "table ip nat") {
		t.Error("expected output to contain 'table ip nat'")
	}
	if !strings.Contains(result, "chain postrouting") {
		t.Error("expected output to contain 'chain postrouting'")
	}
	if !strings.Contains(result, "ip saddr 10.100.0.0/24 snat to 10.240.1.5") {
		t.Errorf("expected SNAT rule for 10.100.0.0/24 -> 10.240.1.5, got:\n%s", result)
	}
}

func TestGenerateNftablesConfig_DNATRule(t *testing.T) {
	nat := &v1alpha1.GatewayNAT{
		DNAT: []v1alpha1.DNATRule{
			{
				ExternalPort:    443,
				InternalAddress: "10.100.0.10",
				InternalPort:    8443,
				Protocol:        "tcp",
				Priority:        50,
			},
		},
	}

	result := GenerateNftablesConfig(nat, "10.240.1.5")

	if !strings.Contains(result, "chain prerouting") {
		t.Error("expected output to contain 'chain prerouting'")
	}
	if !strings.Contains(result, "tcp dport 443 dnat to 10.100.0.10:8443") {
		t.Errorf("expected DNAT rule 'tcp dport 443 dnat to 10.100.0.10:8443', got:\n%s", result)
	}
}

func TestGenerateNftablesConfig_NoNATExemption(t *testing.T) {
	nat := &v1alpha1.GatewayNAT{
		NoNAT: []v1alpha1.NoNATRule{
			{
				Source:      "10.100.0.0/24",
				Destination: "10.240.0.0/16",
				Priority:    10,
			},
		},
		SNAT: []v1alpha1.SNATRule{
			{
				Source:            "10.100.0.0/24",
				TranslatedAddress: "10.240.1.5",
				Priority:          100,
			},
		},
	}

	result := GenerateNftablesConfig(nat, "10.240.1.5")

	// No-NAT accept rule must appear BEFORE the SNAT rule in postrouting
	acceptIdx := strings.Index(result, "ip saddr 10.100.0.0/24 ip daddr 10.240.0.0/16 accept")
	snatIdx := strings.Index(result, "ip saddr 10.100.0.0/24 snat to 10.240.1.5")

	if acceptIdx == -1 {
		t.Errorf("expected NoNAT accept rule, got:\n%s", result)
	}
	if snatIdx == -1 {
		t.Errorf("expected SNAT rule, got:\n%s", result)
	}
	if acceptIdx >= snatIdx {
		t.Errorf("expected NoNAT accept rule BEFORE SNAT rule, acceptIdx=%d snatIdx=%d\n%s", acceptIdx, snatIdx, result)
	}
}

func TestGenerateNftablesConfig_AutoTranslation(t *testing.T) {
	nat := &v1alpha1.GatewayNAT{
		SNAT: []v1alpha1.SNATRule{
			{
				Source:            "10.100.0.0/24",
				TranslatedAddress: "", // empty => use VNI IP
				Priority:          100,
			},
		},
	}

	vniIP := "10.240.1.99"
	result := GenerateNftablesConfig(nat, vniIP)

	expected := "ip saddr 10.100.0.0/24 snat to 10.240.1.99"
	if !strings.Contains(result, expected) {
		t.Errorf("expected auto-translated SNAT rule %q, got:\n%s", expected, result)
	}
}

func TestGenerateNftablesConfig_Nil(t *testing.T) {
	result := GenerateNftablesConfig(nil, "10.240.1.5")

	if result != "" {
		t.Errorf("expected empty string for nil NAT, got %q", result)
	}
}
