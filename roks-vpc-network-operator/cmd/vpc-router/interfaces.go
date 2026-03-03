package main

import (
	"fmt"
	"log/slog"
	"net"
	"os/exec"
	"strings"
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

	// Ensure default route goes via uplink for SNAT/internet-bound traffic.
	// The VPC subnet gateway is always at .1 of the uplink CIDR.
	if gwIP, err := uplinkGatewayIP(); err == nil {
		// Replace pod-network default route with uplink gateway
		_ = runCmd("ip", "route", "replace", "default", "via", gwIP, "dev", "uplink")
		slog.Info("default route set via uplink", "gateway", gwIP)
	} else {
		slog.Warn("could not determine uplink gateway IP", "error", err)
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

// uplinkGatewayIP discovers the VPC subnet gateway IP (always .1) from the uplink interface address.
func uplinkGatewayIP() (string, error) {
	out, err := exec.Command("ip", "-4", "addr", "show", "dev", "uplink").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("ip addr show uplink: %w", err)
	}
	// Parse "inet 172.16.100.34/24" from output
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "inet ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				ip, ipNet, err := net.ParseCIDR(parts[1])
				if err != nil {
					return "", fmt.Errorf("parse CIDR %s: %w", parts[1], err)
				}
				_ = ip
				// Gateway is network address + 1
				gw := make(net.IP, len(ipNet.IP))
				copy(gw, ipNet.IP)
				gw[len(gw)-1] = 1
				return gw.String(), nil
			}
		}
	}
	return "", fmt.Errorf("no IPv4 address found on uplink")
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
