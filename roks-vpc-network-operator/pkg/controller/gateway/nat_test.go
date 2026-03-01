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

func TestGenerateNftablesConfig_PARSNATDefault(t *testing.T) {
	// When a PAR CIDR is provided and SNAT TranslatedAddress is empty,
	// the first PAR IP should be used instead of the VNI IP.
	nat := &v1alpha1.GatewayNAT{
		SNAT: []v1alpha1.SNATRule{
			{
				Source:   "10.100.0.0/24",
				Priority: 100,
			},
		},
	}

	result := GenerateNftablesConfig(nat, "10.240.1.99", "150.240.68.0/28")

	expected := "ip saddr 10.100.0.0/24 snat to 150.240.68.0"
	if !strings.Contains(result, expected) {
		t.Errorf("expected PAR-based SNAT rule %q, got:\n%s", expected, result)
	}
}

func TestGenerateNftablesConfig_PARSNATExplicitOverride(t *testing.T) {
	// When TranslatedAddress is explicitly set, it takes precedence over PAR.
	nat := &v1alpha1.GatewayNAT{
		SNAT: []v1alpha1.SNATRule{
			{
				Source:            "10.100.0.0/24",
				TranslatedAddress: "150.240.68.5",
				Priority:          100,
			},
		},
	}

	result := GenerateNftablesConfig(nat, "10.240.1.99", "150.240.68.0/28")

	expected := "ip saddr 10.100.0.0/24 snat to 150.240.68.5"
	if !strings.Contains(result, expected) {
		t.Errorf("expected explicit SNAT address to override PAR, got:\n%s", result)
	}
}

func TestGenerateNftablesConfig_PAREmptyFallsBackToVNI(t *testing.T) {
	// When PAR CIDR is empty string, fall back to VNI IP.
	nat := &v1alpha1.GatewayNAT{
		SNAT: []v1alpha1.SNATRule{
			{
				Source:   "10.100.0.0/24",
				Priority: 100,
			},
		},
	}

	result := GenerateNftablesConfig(nat, "10.240.1.99", "")

	expected := "ip saddr 10.100.0.0/24 snat to 10.240.1.99"
	if !strings.Contains(result, expected) {
		t.Errorf("expected VNI IP fallback when PAR CIDR is empty, got:\n%s", result)
	}
}

func TestGenerateNftablesConfig_DNATWithExternalAddress(t *testing.T) {
	// DNAT rule with explicit ExternalAddress should include ip daddr match.
	nat := &v1alpha1.GatewayNAT{
		DNAT: []v1alpha1.DNATRule{
			{
				ExternalAddress: "150.240.68.5",
				ExternalPort:    443,
				InternalAddress: "10.100.0.10",
				InternalPort:    8443,
				Protocol:        "tcp",
				Priority:        50,
			},
		},
	}

	result := GenerateNftablesConfig(nat, "10.240.1.5")

	expected := "ip daddr 150.240.68.5 tcp dport 443 dnat to 10.100.0.10:8443"
	if !strings.Contains(result, expected) {
		t.Errorf("expected DNAT with external address match %q, got:\n%s", expected, result)
	}
}

func TestFirstIPFromCIDR(t *testing.T) {
	tests := []struct {
		cidr     string
		expected string
	}{
		{"150.240.68.0/28", "150.240.68.0"},
		{"10.0.0.1/32", "10.0.0.1"},
		{"192.168.1.0/24", "192.168.1.0"},
		{"", ""},
	}

	for _, tt := range tests {
		result := firstIPFromCIDR(tt.cidr)
		if result != tt.expected {
			t.Errorf("firstIPFromCIDR(%q) = %q, want %q", tt.cidr, result, tt.expected)
		}
	}
}
