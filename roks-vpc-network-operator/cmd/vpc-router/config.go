package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
)

// InterfaceConfig describes a single network interface.
type InterfaceConfig struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

// NetworkConfig is the top-level NETWORK_CONFIG JSON.
type NetworkConfig struct {
	Interfaces []InterfaceConfig `json:"interfaces"`
}

// Config holds all configuration parsed from environment variables.
type Config struct {
	// Network
	Networks NetworkConfig

	// nftables
	NftablesConfig   string
	FirewallConfig   string
	IPSNFQueueConfig string

	// DHCP
	DHCPEnabled bool

	// XDP
	XDPEnabled bool

	// Health
	HealthPort int
}

func parseConfig() (*Config, error) {
	cfg := &Config{
		HealthPort: 8080,
	}

	// Parse NETWORK_CONFIG JSON
	netJSON := os.Getenv("NETWORK_CONFIG")
	if netJSON != "" {
		if err := json.Unmarshal([]byte(netJSON), &cfg.Networks); err != nil {
			return nil, fmt.Errorf("parse NETWORK_CONFIG: %w", err)
		}
	}

	cfg.NftablesConfig = os.Getenv("NFTABLES_CONFIG")
	cfg.FirewallConfig = os.Getenv("FIREWALL_CONFIG")
	cfg.IPSNFQueueConfig = os.Getenv("IPS_NFQUEUE_CONFIG")

	cfg.DHCPEnabled = os.Getenv("DHCP_ENABLED") == "true"
	cfg.XDPEnabled = os.Getenv("XDP_ENABLED") == "true"

	if portStr := os.Getenv("HEALTH_PORT"); portStr != "" {
		p, err := strconv.Atoi(portStr)
		if err != nil {
			return nil, fmt.Errorf("parse HEALTH_PORT: %w", err)
		}
		cfg.HealthPort = p
	}

	return cfg, nil
}
