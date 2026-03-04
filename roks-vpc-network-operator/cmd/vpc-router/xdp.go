package main

import (
	"encoding/binary"
	"fmt"
	"log/slog"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
)

// lpmKey matches the BPF map key layout: struct lpm_key { __u32 prefixlen; __u32 addr; }
type lpmKey struct {
	PrefixLen uint32
	Addr      uint32
}

// routeEntry represents a parsed route for populating the BPF route_table map.
type routeEntry struct {
	Network   uint32 // network address in network byte order
	PrefixLen uint32 // CIDR prefix length
	IfIndex   uint32 // output interface index (lo=1, uplink=2, net0=3, net1=4, ...)
}

// natCIDRRegex matches "ip saddr <CIDR>" in nftables config.
var natCIDRRegex = regexp.MustCompile(`ip saddr (\d+\.\d+\.\d+\.\d+/\d+)`)

// buildRouteEntries parses the NETWORK_CONFIG interfaces into BPF route entries.
// Interface indices are assigned as: lo=1, uplink=2, net0=3, net1=4, etc.
func buildRouteEntries(cfg *Config) []routeEntry {
	var entries []routeEntry
	for i, iface := range cfg.Networks.Interfaces {
		_, ipNet, err := net.ParseCIDR(iface.Address)
		if err != nil {
			slog.Warn("skipping interface with invalid CIDR", "name", iface.Name, "address", iface.Address, "error", err)
			continue
		}
		prefixLen, _ := ipNet.Mask.Size()
		entries = append(entries, routeEntry{
			Network:   ipToUint32(ipNet.IP.To4()),
			PrefixLen: uint32(prefixLen),
			IfIndex:   uint32(i + 3), // lo=1, uplink=2, net0=3, net1=4, ...
		})
	}
	return entries
}

// extractNATCIDRs parses SNAT source CIDRs from nftables config.
// Looks for patterns like "ip saddr 172.16.100.0/24".
func extractNATCIDRs(nftConfig string) []string {
	if nftConfig == "" {
		return nil
	}
	matches := natCIDRRegex.FindAllStringSubmatch(nftConfig, -1)
	cidrs := make([]string, 0, len(matches))
	for _, m := range matches {
		cidrs = append(cidrs, m[1])
	}
	return cidrs
}

// isKernelVersionSufficient checks that the kernel version is >= 5.8,
// which is needed for BPF LPM trie + XDP redirect support.
func isKernelVersionSufficient(version string) bool {
	if version == "" {
		return false
	}
	// Strip trailing suffix after digits (e.g., "-generic", "-1234-custom")
	// Parse "major.minor.patch-suffix" or "major.minor.patch"
	parts := strings.SplitN(version, ".", 3)
	if len(parts) < 2 {
		return false
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return false
	}
	// Minor may have suffix like "8-rc1"
	minorStr := parts[1]
	if idx := strings.IndexFunc(minorStr, func(r rune) bool { return r < '0' || r > '9' }); idx > 0 {
		minorStr = minorStr[:idx]
	}
	minor, err := strconv.Atoi(minorStr)
	if err != nil {
		return false
	}
	if major > 5 {
		return true
	}
	if major == 5 && minor >= 8 {
		return true
	}
	return false
}

// ipToUint32 converts a net.IP (IPv4) to a uint32 in network byte order.
func ipToUint32(ip net.IP) uint32 {
	if len(ip) < 4 {
		return 0
	}
	return binary.BigEndian.Uint32(ip[:4])
}

// attachXDP loads and attaches XDP/eBPF programs for L3 fast-path forwarding.
// Returns a cleanup function that detaches the programs on shutdown.
//
// The XDP program handles simple L3 forwarding: packet arrives -> LPM trie
// route lookup -> XDP_REDIRECT to output interface. Non-IPv4, DHCP, NAT-destined,
// and firewall-present traffic falls through to the kernel (XDP_PASS).
//
// If XDP attachment fails (old kernel, missing capabilities), the router
// continues with kernel-only forwarding -- XDP is an optimization, not a requirement.
func attachXDP(cfg *Config) (func(), error) {
	noop := func() {}

	// Step 1: Check kernel version
	kernelVersion, err := readKernelVersion()
	if err != nil {
		slog.Warn("cannot read kernel version, skipping XDP", "error", err)
		return noop, nil
	}
	if !isKernelVersionSufficient(kernelVersion) {
		slog.Warn("kernel too old for XDP (need >= 5.8)", "version", kernelVersion)
		return noop, nil
	}
	slog.Info("kernel version sufficient for XDP", "version", kernelVersion)

	// Step 2: Load eBPF object file
	objPath := findEBPFObject()
	if objPath == "" {
		slog.Warn("eBPF object file not found, skipping XDP — using kernel forwarding")
		return noop, nil
	}

	spec, err := ebpf.LoadCollectionSpec(objPath)
	if err != nil {
		slog.Warn("failed to load eBPF collection spec, skipping XDP", "path", objPath, "error", err)
		return noop, nil
	}

	coll, err := ebpf.NewCollection(spec)
	if err != nil {
		slog.Warn("failed to create eBPF collection, skipping XDP", "error", err)
		return noop, nil
	}

	// Step 3: Populate route_table map
	routeTable := coll.Maps["route_table"]
	if routeTable == nil {
		slog.Warn("route_table map not found in eBPF object, skipping XDP")
		coll.Close()
		return noop, nil
	}
	routes := buildRouteEntries(cfg)
	for _, r := range routes {
		key := lpmKey{PrefixLen: r.PrefixLen, Addr: r.Network}
		val := r.IfIndex
		if err := routeTable.Put(key, val); err != nil {
			slog.Warn("failed to insert route into BPF map", "network", fmt.Sprintf("0x%08x/%d", r.Network, r.PrefixLen), "error", err)
		}
	}
	slog.Info("populated BPF route_table", "entries", len(routes))

	// Step 4: Populate nat_cidrs map
	natMap := coll.Maps["nat_cidrs"]
	if natMap != nil {
		cidrs := extractNATCIDRs(cfg.NftablesConfig)
		for _, cidr := range cidrs {
			_, ipNet, err := net.ParseCIDR(cidr)
			if err != nil {
				slog.Warn("skipping invalid NAT CIDR", "cidr", cidr, "error", err)
				continue
			}
			prefixLen, _ := ipNet.Mask.Size()
			key := lpmKey{PrefixLen: uint32(prefixLen), Addr: ipToUint32(ipNet.IP.To4())}
			val := uint8(1)
			if err := natMap.Put(key, val); err != nil {
				slog.Warn("failed to insert NAT CIDR into BPF map", "cidr", cidr, "error", err)
			}
		}
		slog.Info("populated BPF nat_cidrs", "entries", len(cidrs))
	}

	// Step 5: Set firewall flag
	flagsMap := coll.Maps["flags"]
	if flagsMap != nil && cfg.FirewallConfig != "" {
		key := uint32(0) // FLAG_FIREWALL_ENABLED
		val := uint32(1)
		if err := flagsMap.Put(key, val); err != nil {
			slog.Warn("failed to set firewall flag in BPF map", "error", err)
		}
		slog.Info("BPF firewall flag set")
	}

	// Step 6: Attach XDP to each workload interface
	prog := coll.Programs["xdp_fwd"]
	if prog == nil {
		slog.Warn("xdp_fwd program not found in eBPF object, skipping XDP")
		coll.Close()
		return noop, nil
	}

	var xdpLinks []link.Link
	for _, iface := range cfg.Networks.Interfaces {
		netIf, err := net.InterfaceByName(iface.Name)
		if err != nil {
			slog.Warn("interface not found, skipping XDP attachment", "name", iface.Name, "error", err)
			continue
		}
		l, err := link.AttachXDP(link.XDPOptions{
			Program:   prog,
			Interface: netIf.Index,
		})
		if err != nil {
			slog.Warn("failed to attach XDP to interface, skipping", "name", iface.Name, "error", err)
			continue
		}
		xdpLinks = append(xdpLinks, l)
		slog.Info("XDP attached to interface", "name", iface.Name, "ifindex", netIf.Index)
	}

	if len(xdpLinks) == 0 {
		slog.Warn("no XDP links attached, falling back to kernel forwarding")
		coll.Close()
		return noop, nil
	}

	slog.Info("XDP fast-path forwarding active", "interfaces", len(xdpLinks))

	// Return cleanup function
	cleanup := func() {
		for _, l := range xdpLinks {
			if err := l.Close(); err != nil {
				slog.Warn("failed to close XDP link", "error", err)
			}
		}
		coll.Close()
		slog.Info("XDP programs detached")
	}

	return cleanup, nil
}

// readKernelVersion reads the kernel version from /proc/sys/kernel/osrelease.
func readKernelVersion() (string, error) {
	data, err := os.ReadFile("/proc/sys/kernel/osrelease")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// findEBPFObject locates the compiled eBPF object file.
// Checks container path first (/bpf/fwd_bpfel.o), then development path (bpf/fwd_bpfel.o).
func findEBPFObject() string {
	paths := []string{
		"/bpf/fwd_bpfel.o",     // container
		"bpf/fwd_bpfel.o",      // development
		"/bpf/fwd_bpfeb.o",     // container (big-endian)
		"bpf/fwd_bpfeb.o",      // development (big-endian)
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			slog.Info("found eBPF object", "path", p)
			return p
		}
	}
	return ""
}
