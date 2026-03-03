package main

import (
	"fmt"
	"log/slog"
	"os/exec"
)

// configureInterfaces sets up the uplink and workload network interfaces.
func configureInterfaces(cfg *Config) error {
	// Configure uplink via DHCP
	slog.Info("configuring uplink interface via DHCP")
	if err := runCmd("ip", "link", "set", "uplink", "up"); err != nil {
		return fmt.Errorf("bring uplink up: %w", err)
	}
	if err := runCmd("dhclient", "-v", "uplink"); err != nil {
		return fmt.Errorf("dhclient uplink: %w", err)
	}

	// Configure workload interfaces
	for _, iface := range cfg.Networks.Interfaces {
		slog.Info("configuring workload interface", "name", iface.Name, "address", iface.Address)
		// ip addr add may fail if already present — ignore
		_ = runCmd("ip", "addr", "add", iface.Address, "dev", iface.Name)
		if err := runCmd("ip", "link", "set", iface.Name, "up"); err != nil {
			return fmt.Errorf("bring %s up: %w", iface.Name, err)
		}
	}

	return nil
}

// enableIPForwarding enables IPv4 forwarding via sysctl.
func enableIPForwarding() error {
	return runCmd("sysctl", "-w", "net.ipv4.ip_forward=1")
}

// runCmd executes a command and returns an error if it fails.
func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %v: %w", name, args, err)
	}
	return nil
}
