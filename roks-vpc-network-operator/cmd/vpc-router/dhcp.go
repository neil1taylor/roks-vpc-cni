package main

import (
	"fmt"
	"log/slog"
	"net"
	"os/exec"
)

// startDnsmasq starts dnsmasq child processes for each DHCP-enabled network interface.
// Each interface gets its own dnsmasq instance with a DHCP range derived from the
// interface address (pool start at .10, pool end at the last usable host).
func startDnsmasq(cfg *Config) []*exec.Cmd {
	if !cfg.DHCPEnabled {
		return nil
	}

	var cmds []*exec.Cmd
	for _, iface := range cfg.Networks.Interfaces {
		poolStart, poolEnd, mask, err := dhcpRange(iface.Address)
		if err != nil {
			slog.Warn("skipping DHCP for interface", "name", iface.Name, "error", err)
			continue
		}

		leaseTime := "12h"
		args := []string{
			"--interface=" + iface.Name,
			"--bind-interfaces",
			"--no-daemon",
			"--log-dhcp",
			"--no-resolv",
			fmt.Sprintf("--pid-file=/var/run/dnsmasq-%s.pid", iface.Name),
			fmt.Sprintf("--dhcp-range=%s,%s,%s,%s", poolStart, poolEnd, mask, leaseTime),
		}

		cmd := exec.Command("dnsmasq", args...)
		cmd.Stdout = nil
		cmd.Stderr = nil
		if err := cmd.Start(); err != nil {
			slog.Error("failed to start dnsmasq", "interface", iface.Name, "error", err)
			continue
		}

		slog.Info("started dnsmasq", "interface", iface.Name, "range", poolStart+"-"+poolEnd)
		cmds = append(cmds, cmd)
	}

	return cmds
}

// dhcpRange computes the DHCP pool start, end, and subnet mask from a CIDR address.
// Pool starts at .10, ends at the broadcast address - 1.
func dhcpRange(cidr string) (start, end, mask string, err error) {
	ip, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return "", "", "", fmt.Errorf("parse CIDR %s: %w", cidr, err)
	}
	_ = ip

	ones, bits := ipNet.Mask.Size()
	if bits != 32 {
		return "", "", "", fmt.Errorf("only IPv4 supported")
	}

	// Network address as uint32
	netIP := ipNet.IP.To4()
	netUint := uint32(netIP[0])<<24 | uint32(netIP[1])<<16 | uint32(netIP[2])<<8 | uint32(netIP[3])

	// Host count
	hostBits := 32 - ones
	hostCount := uint32(1) << hostBits

	// Pool: .10 to broadcast-1
	startUint := netUint + 10
	endUint := netUint + hostCount - 2

	startIP := net.IPv4(byte(startUint>>24), byte(startUint>>16), byte(startUint>>8), byte(startUint))
	endIP := net.IPv4(byte(endUint>>24), byte(endUint>>16), byte(endUint>>8), byte(endUint))

	maskStr := fmt.Sprintf("%d.%d.%d.%d", ipNet.Mask[0], ipNet.Mask[1], ipNet.Mask[2], ipNet.Mask[3])

	return startIP.String(), endIP.String(), maskStr, nil
}
