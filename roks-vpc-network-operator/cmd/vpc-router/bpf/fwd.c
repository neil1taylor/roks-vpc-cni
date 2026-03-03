// SPDX-License-Identifier: GPL-2.0
// XDP L3 forwarding program for VPC Router fast-path mode.
//
// This program handles simple L3 forwarding:
//   packet arrives -> LPM trie route lookup -> XDP_REDIRECT to output interface
//
// Traffic that requires kernel processing is passed through (XDP_PASS):
//   - Non-IPv4 packets
//   - UDP port 67/68 (DHCP)
//   - Packets matching NAT CIDRs
//   - All packets when firewall is enabled (kernel nftables handles filtering)

#include <linux/bpf.h>
#include <linux/if_ether.h>
#include <linux/ip.h>
#include <linux/udp.h>
#include <linux/in.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_endian.h>

// LPM trie key for IPv4 (prefix length + 4-byte address)
struct lpm_key {
    __u32 prefixlen;
    __u32 addr;
};

// Route table: destination CIDR -> output interface index
struct {
    __uint(type, BPF_MAP_TYPE_LPM_TRIE);
    __type(key, struct lpm_key);
    __type(value, __u32);  // ifindex
    __uint(max_entries, 256);
    __uint(map_flags, BPF_F_NO_PREALLOC);
} route_table SEC(".maps");

// NAT CIDRs: packets destined to these CIDRs need kernel NAT processing
struct {
    __uint(type, BPF_MAP_TYPE_LPM_TRIE);
    __type(key, struct lpm_key);
    __type(value, __u8);  // 1 = needs NAT
    __uint(max_entries, 64);
    __uint(map_flags, BPF_F_NO_PREALLOC);
} nat_cidrs SEC(".maps");

// Flags array: index 0 = firewall_enabled
struct {
    __uint(type, BPF_MAP_TYPE_ARRAY);
    __type(key, __u32);
    __type(value, __u32);
    __uint(max_entries, 4);
} flags SEC(".maps");

#define FLAG_FIREWALL_ENABLED 0

SEC("xdp")
int xdp_fwd(struct xdp_md *ctx) {
    void *data = (void *)(long)ctx->data;
    void *data_end = (void *)(long)ctx->data_end;

    // Parse Ethernet header
    struct ethhdr *eth = data;
    if ((void *)(eth + 1) > data_end)
        return XDP_PASS;

    // Only handle IPv4
    if (eth->h_proto != bpf_htons(ETH_P_IP))
        return XDP_PASS;

    // Parse IP header
    struct iphdr *ip = (void *)(eth + 1);
    if ((void *)(ip + 1) > data_end)
        return XDP_PASS;

    // Check firewall flag -- if enabled, pass everything to kernel nftables
    __u32 fw_key = FLAG_FIREWALL_ENABLED;
    __u32 *fw_val = bpf_map_lookup_elem(&flags, &fw_key);
    if (fw_val && *fw_val)
        return XDP_PASS;

    // Check for DHCP traffic (UDP port 67 or 68) -- always pass to kernel
    if (ip->protocol == IPPROTO_UDP) {
        struct udphdr *udp = (void *)ip + (ip->ihl * 4);
        if ((void *)(udp + 1) <= data_end) {
            __u16 dport = bpf_ntohs(udp->dest);
            __u16 sport = bpf_ntohs(udp->source);
            if (dport == 67 || dport == 68 || sport == 67 || sport == 68)
                return XDP_PASS;
        }
    }

    // Check if destination needs NAT -- pass to kernel
    struct lpm_key nat_key = {
        .prefixlen = 32,
        .addr = ip->daddr,
    };
    if (bpf_map_lookup_elem(&nat_cidrs, &nat_key))
        return XDP_PASS;

    // Route lookup -- find output interface
    struct lpm_key route_key = {
        .prefixlen = 32,
        .addr = ip->daddr,
    };
    __u32 *out_ifindex = bpf_map_lookup_elem(&route_table, &route_key);
    if (!out_ifindex)
        return XDP_PASS;  // No route found, let kernel handle

    // Decrement TTL
    if (ip->ttl <= 1)
        return XDP_PASS;  // TTL expired, let kernel send ICMP
    ip->ttl--;

    // Recalculate IP checksum (incremental)
    __u32 csum = (__u32)ip->check + 0x0100;  // TTL decrement by 1
    ip->check = (__u16)(csum + (csum >> 16));

    // Redirect to output interface
    return bpf_redirect(*out_ifindex, 0);
}

char _license[] SEC("license") = "GPL";
