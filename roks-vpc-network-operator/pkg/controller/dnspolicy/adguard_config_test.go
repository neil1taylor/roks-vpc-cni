package dnspolicy

import (
	"strings"
	"testing"

	v1alpha1 "github.com/IBM/roks-vpc-network-operator/api/v1alpha1"
)

func TestGenerateAdGuardConfig_Basic(t *testing.T) {
	spec := &v1alpha1.VPCDNSPolicySpec{
		RouterRef: "my-router",
		Upstream: &v1alpha1.DNSUpstreamConfig{
			Servers: []v1alpha1.DNSUpstreamServer{
				{URL: "https://cloudflare-dns.com/dns-query"},
				{URL: "tls://dns.quad9.net"},
			},
		},
		Filtering: &v1alpha1.DNSFilteringConfig{
			Enabled:    true,
			Blocklists: []string{"https://raw.githubusercontent.com/StevenBlack/hosts/master/hosts"},
		},
		LocalDNS: &v1alpha1.DNSLocalConfig{Enabled: true, Domain: "vm.local"},
	}

	cfg := generateAdGuardConfig(spec)

	checks := []string{"bind_host: 127.0.0.1", "bind_port: 5353", "cloudflare-dns.com", "StevenBlack", "filtering_enabled: true", "local_domain_name: vm.local"}
	for _, c := range checks {
		if !strings.Contains(cfg, c) {
			t.Errorf("config missing %q", c)
		}
	}
}

func TestGenerateAdGuardConfig_NoFiltering(t *testing.T) {
	spec := &v1alpha1.VPCDNSPolicySpec{
		RouterRef: "my-router",
		Upstream:  &v1alpha1.DNSUpstreamConfig{Servers: []v1alpha1.DNSUpstreamServer{{URL: "8.8.8.8"}}},
	}

	cfg := generateAdGuardConfig(spec)
	if !strings.Contains(cfg, "filtering_enabled: false") {
		t.Error("expected filtering disabled")
	}
}

func TestGenerateAdGuardConfig_DefaultUpstream(t *testing.T) {
	spec := &v1alpha1.VPCDNSPolicySpec{RouterRef: "my-router"}
	cfg := generateAdGuardConfig(spec)
	if !strings.Contains(cfg, "8.8.8.8") || !strings.Contains(cfg, "1.1.1.1") {
		t.Error("expected default upstream servers")
	}
}

func TestGenerateAdGuardConfig_AllowDenyLists(t *testing.T) {
	spec := &v1alpha1.VPCDNSPolicySpec{
		RouterRef: "my-router",
		Filtering: &v1alpha1.DNSFilteringConfig{
			Enabled:   true,
			Allowlist: []string{"*.example.com"},
			Denylist:  []string{"tracking.bad.com"},
		},
	}

	cfg := generateAdGuardConfig(spec)
	if !strings.Contains(cfg, "@@||*.example.com^") {
		t.Error("expected allowlist rule")
	}
	if !strings.Contains(cfg, "||tracking.bad.com^") {
		t.Error("expected denylist rule")
	}
}
