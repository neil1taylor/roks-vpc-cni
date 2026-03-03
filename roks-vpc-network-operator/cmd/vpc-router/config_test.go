package main

import (
	"os"
	"testing"
)

func TestParseConfig_Defaults(t *testing.T) {
	// Clear env
	os.Unsetenv("NETWORK_CONFIG")
	os.Unsetenv("NFTABLES_CONFIG")
	os.Unsetenv("FIREWALL_CONFIG")
	os.Unsetenv("DHCP_ENABLED")
	os.Unsetenv("XDP_ENABLED")
	os.Unsetenv("HEALTH_PORT")

	cfg, err := parseConfig()
	if err != nil {
		t.Fatalf("parseConfig() error = %v", err)
	}

	if cfg.HealthPort != 8080 {
		t.Errorf("default HealthPort = %d, want 8080", cfg.HealthPort)
	}
	if cfg.DHCPEnabled {
		t.Error("default DHCPEnabled should be false")
	}
	if cfg.XDPEnabled {
		t.Error("default XDPEnabled should be false")
	}
	if len(cfg.Networks.Interfaces) != 0 {
		t.Errorf("default Networks should be empty, got %d", len(cfg.Networks.Interfaces))
	}
}

func TestParseConfig_WithNetworkConfig(t *testing.T) {
	os.Setenv("NETWORK_CONFIG", `{"interfaces":[{"name":"net0","address":"10.100.0.1/24"},{"name":"net1","address":"10.200.0.1/24"}]}`)
	defer os.Unsetenv("NETWORK_CONFIG")
	os.Setenv("DHCP_ENABLED", "true")
	defer os.Unsetenv("DHCP_ENABLED")
	os.Setenv("XDP_ENABLED", "true")
	defer os.Unsetenv("XDP_ENABLED")
	os.Setenv("HEALTH_PORT", "9090")
	defer os.Unsetenv("HEALTH_PORT")

	cfg, err := parseConfig()
	if err != nil {
		t.Fatalf("parseConfig() error = %v", err)
	}

	if len(cfg.Networks.Interfaces) != 2 {
		t.Fatalf("expected 2 interfaces, got %d", len(cfg.Networks.Interfaces))
	}
	if cfg.Networks.Interfaces[0].Name != "net0" {
		t.Errorf("interface[0].Name = %q, want net0", cfg.Networks.Interfaces[0].Name)
	}
	if cfg.Networks.Interfaces[1].Address != "10.200.0.1/24" {
		t.Errorf("interface[1].Address = %q, want 10.200.0.1/24", cfg.Networks.Interfaces[1].Address)
	}
	if !cfg.DHCPEnabled {
		t.Error("DHCPEnabled should be true")
	}
	if !cfg.XDPEnabled {
		t.Error("XDPEnabled should be true")
	}
	if cfg.HealthPort != 9090 {
		t.Errorf("HealthPort = %d, want 9090", cfg.HealthPort)
	}
}

func TestParseConfig_InvalidJSON(t *testing.T) {
	os.Setenv("NETWORK_CONFIG", "invalid json")
	defer os.Unsetenv("NETWORK_CONFIG")

	_, err := parseConfig()
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseConfig_InvalidPort(t *testing.T) {
	os.Setenv("HEALTH_PORT", "notanumber")
	defer os.Unsetenv("HEALTH_PORT")

	_, err := parseConfig()
	if err == nil {
		t.Error("expected error for invalid port")
	}
}

func TestParseConfig_NftablesConfig(t *testing.T) {
	nft := "table ip nat { chain postrouting { type nat hook postrouting priority 100; masquerade; } }"
	os.Setenv("NFTABLES_CONFIG", nft)
	defer os.Unsetenv("NFTABLES_CONFIG")

	cfg, err := parseConfig()
	if err != nil {
		t.Fatalf("parseConfig() error = %v", err)
	}

	if cfg.NftablesConfig != nft {
		t.Errorf("NftablesConfig mismatch")
	}
}
