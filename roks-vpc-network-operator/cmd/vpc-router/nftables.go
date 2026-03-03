package main

import (
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
)

// applyNftables applies NAT, firewall, and IPS NFQUEUE rules.
func applyNftables(cfg *Config) error {
	if cfg.NftablesConfig != "" {
		slog.Info("applying NAT nftables rules")
		if err := applyNftConfig(cfg.NftablesConfig); err != nil {
			return fmt.Errorf("apply NAT rules: %w", err)
		}
	}

	if cfg.FirewallConfig != "" {
		slog.Info("applying firewall nftables rules")
		if err := applyNftConfig(cfg.FirewallConfig); err != nil {
			return fmt.Errorf("apply firewall rules: %w", err)
		}
	}

	if cfg.IPSNFQueueConfig != "" {
		slog.Info("applying IPS NFQUEUE rules")
		if err := applyNftConfig(cfg.IPSNFQueueConfig); err != nil {
			return fmt.Errorf("apply IPS rules: %w", err)
		}
	}

	return nil
}

// applyNftConfig pipes an nftables configuration to nft -f -.
func applyNftConfig(config string) error {
	cmd := exec.Command("nft", "-f", "-")
	cmd.Stdin = strings.NewReader(config)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("nft: %w (output: %s)", err, string(out))
	}
	return nil
}
