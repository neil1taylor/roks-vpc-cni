package main

import (
	"net"
	"testing"
)

func TestBuildRouteEntries_Empty(t *testing.T) {
	cfg := &Config{}
	entries := buildRouteEntries(cfg)
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestBuildRouteEntries_SingleInterface(t *testing.T) {
	cfg := &Config{
		Networks: NetworkConfig{
			Interfaces: []InterfaceConfig{
				{Name: "net0", Address: "10.100.0.1/24"},
			},
		},
	}
	entries := buildRouteEntries(cfg)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.PrefixLen != 24 {
		t.Errorf("PrefixLen = %d, want 24", e.PrefixLen)
	}
	if e.IfIndex != 3 {
		t.Errorf("IfIndex = %d, want 3 (net0 -> lo=1,uplink=2,net0=3)", e.IfIndex)
	}
	// Network address for 10.100.0.0/24 in network byte order
	expected := ipToUint32(net.ParseIP("10.100.0.0").To4())
	if e.Network != expected {
		t.Errorf("Network = %d, want %d (10.100.0.0)", e.Network, expected)
	}
}

func TestBuildRouteEntries_MultipleInterfaces(t *testing.T) {
	cfg := &Config{
		Networks: NetworkConfig{
			Interfaces: []InterfaceConfig{
				{Name: "net0", Address: "10.100.0.1/24"},
				{Name: "net1", Address: "172.16.50.1/16"},
			},
		},
	}
	entries := buildRouteEntries(cfg)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].IfIndex != 3 {
		t.Errorf("entries[0].IfIndex = %d, want 3", entries[0].IfIndex)
	}
	if entries[1].IfIndex != 4 {
		t.Errorf("entries[1].IfIndex = %d, want 4", entries[1].IfIndex)
	}
	if entries[1].PrefixLen != 16 {
		t.Errorf("entries[1].PrefixLen = %d, want 16", entries[1].PrefixLen)
	}
	// 172.16.0.0 in network byte order
	expected := ipToUint32(net.ParseIP("172.16.0.0").To4())
	if entries[1].Network != expected {
		t.Errorf("entries[1].Network = %d, want %d (172.16.0.0)", entries[1].Network, expected)
	}
}

func TestBuildRouteEntries_InvalidCIDR(t *testing.T) {
	cfg := &Config{
		Networks: NetworkConfig{
			Interfaces: []InterfaceConfig{
				{Name: "net0", Address: "not-a-cidr"},
				{Name: "net1", Address: "10.200.0.1/24"},
			},
		},
	}
	entries := buildRouteEntries(cfg)
	// Should skip invalid and still process the valid one
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry (skip invalid), got %d", len(entries))
	}
	if entries[0].IfIndex != 4 {
		t.Errorf("IfIndex = %d, want 4 (net1 index)", entries[0].IfIndex)
	}
}

func TestExtractNATCIDRs_Empty(t *testing.T) {
	cidrs := extractNATCIDRs("")
	if len(cidrs) != 0 {
		t.Errorf("expected 0 CIDRs, got %d", len(cidrs))
	}
}

func TestExtractNATCIDRs_SingleSNAT(t *testing.T) {
	nft := `table ip nat {
		chain postrouting {
			type nat hook postrouting priority 100;
			ip saddr 172.16.100.0/24 oifname "uplink" masquerade
		}
	}`
	cidrs := extractNATCIDRs(nft)
	if len(cidrs) != 1 {
		t.Fatalf("expected 1 CIDR, got %d", len(cidrs))
	}
	if cidrs[0] != "172.16.100.0/24" {
		t.Errorf("CIDR = %q, want 172.16.100.0/24", cidrs[0])
	}
}

func TestExtractNATCIDRs_MultipleSNAT(t *testing.T) {
	nft := `table ip nat {
		chain postrouting {
			ip saddr 10.100.0.0/24 oifname "uplink" masquerade
			ip saddr 10.200.0.0/16 oifname "uplink" masquerade
		}
	}`
	cidrs := extractNATCIDRs(nft)
	if len(cidrs) != 2 {
		t.Fatalf("expected 2 CIDRs, got %d", len(cidrs))
	}
	if cidrs[0] != "10.100.0.0/24" {
		t.Errorf("CIDR[0] = %q, want 10.100.0.0/24", cidrs[0])
	}
	if cidrs[1] != "10.200.0.0/16" {
		t.Errorf("CIDR[1] = %q, want 10.200.0.0/16", cidrs[1])
	}
}

func TestExtractNATCIDRs_NoMatch(t *testing.T) {
	nft := `table ip filter { chain forward { accept } }`
	cidrs := extractNATCIDRs(nft)
	if len(cidrs) != 0 {
		t.Errorf("expected 0 CIDRs, got %d", len(cidrs))
	}
}

func TestIsKernelVersionSufficient(t *testing.T) {
	tests := []struct {
		version string
		want    bool
	}{
		{"5.8.0", true},
		{"5.8.1", true},
		{"5.10.0-1234-generic", true},
		{"5.15.0", true},
		{"6.1.0", true},
		{"6.0.0-custom", true},
		{"5.7.99", false},
		{"5.7.0-generic", false},
		{"4.19.0", false},
		{"4.18.0-305.el8.x86_64", false},
		{"5.8.0-rc1", true},
		{"", false},
		{"invalid", false},
	}
	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			got := isKernelVersionSufficient(tt.version)
			if got != tt.want {
				t.Errorf("isKernelVersionSufficient(%q) = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}

func TestIPToUint32(t *testing.T) {
	tests := []struct {
		ip   string
		want uint32
	}{
		// Network byte order: 10.0.0.1 -> 0x0a000001
		{"10.0.0.1", 0x0a000001},
		{"192.168.1.1", 0xc0a80101},
		{"255.255.255.255", 0xffffffff},
		{"0.0.0.0", 0x00000000},
		{"172.16.100.0", 0xac106400},
	}
	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			ip := net.ParseIP(tt.ip).To4()
			got := ipToUint32(ip)
			if got != tt.want {
				t.Errorf("ipToUint32(%s) = 0x%08x, want 0x%08x", tt.ip, got, tt.want)
			}
		})
	}
}

func TestIPToUint32_Nil(t *testing.T) {
	got := ipToUint32(nil)
	if got != 0 {
		t.Errorf("ipToUint32(nil) = %d, want 0", got)
	}
}
