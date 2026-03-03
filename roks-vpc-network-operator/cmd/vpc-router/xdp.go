package main

import (
	"log/slog"
)

// attachXDP loads and attaches XDP/eBPF programs for L3 fast-path forwarding.
// Returns a cleanup function that detaches the programs on shutdown.
//
// The XDP program handles simple L3 forwarding: packet arrives → LPM trie
// route lookup → XDP_REDIRECT to output interface. Non-IPv4, DHCP, NAT-destined,
// and firewall-present traffic falls through to the kernel (XDP_PASS).
//
// If XDP attachment fails (old kernel, missing capabilities), the router
// continues with kernel-only forwarding — XDP is an optimization, not a requirement.
func attachXDP(cfg *Config) (func(), error) {
	// TODO: Implement XDP loading via cilium/ebpf once the eBPF program is compiled.
	// For now, log that XDP is not yet available and return no-op.
	//
	// Future implementation:
	// 1. Load the compiled eBPF object (fwd_bpfel.o / fwd_bpfeb.o)
	// 2. Populate route_table BPF map from NETWORK_CONFIG
	// 3. Populate nat_cidrs BPF map from NFTABLES_CONFIG
	// 4. Set firewall_enabled flag if FIREWALL_CONFIG is non-empty
	// 5. Attach XDP program to each network interface
	// 6. Return cleanup function that detaches programs and closes maps

	slog.Info("XDP/eBPF loading not yet compiled — using kernel forwarding")
	return func() {}, nil
}
