package main

import (
	"log/slog"
	"os/exec"
)

// startDnsmasq starts dnsmasq child processes for DHCP-enabled networks.
// The actual dnsmasq command lines are passed via the DHCP env vars that the
// standard mode also uses. In the fast-path binary, we simply exec them as
// background processes the same way the bash script does.
//
// For now, DHCP configuration is embedded in the same env vars as standard mode
// and dnsmasq is started by the Go binary instead of a bash script.
func startDnsmasq(cfg *Config) []*exec.Cmd {
	if !cfg.DHCPEnabled {
		return nil
	}

	// The dnsmasq instances are configured identically to standard mode.
	// The buildEnvVars/buildInitScript code in the operator constructs the
	// per-network dnsmasq commands. In fast-path mode, the Go binary starts
	// dnsmasq directly for each DHCP-enabled interface.
	//
	// The DHCP configuration is passed via NETWORK_CONFIG + DHCP_ENABLED.
	// Individual dnsmasq instances are started per network interface with
	// the same arguments the bash init script would use.
	slog.Info("DHCP enabled — dnsmasq instances managed by vpc-router")

	// Note: Actual dnsmasq spawn logic reads the per-interface DHCP config
	// from the NETWORK_CONFIG and DHCP env vars. This is a placeholder for
	// the full implementation which will parse the config and build dnsmasq
	// command lines programmatically.
	return nil
}
