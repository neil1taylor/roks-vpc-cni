package traceflow

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	v1alpha1 "github.com/IBM/roks-vpc-network-operator/api/v1alpha1"
)

// tracerouteHop represents a single parsed hop from traceroute output.
type tracerouteHop struct {
	HopNum  int
	IP      string
	Latency string // e.g. "0.456ms"
}

// hopRegex matches a traceroute hop line like:
//
//	1  172.16.100.1 (172.16.100.1)  0.456 ms  0.312 ms  0.298 ms
var hopRegex = regexp.MustCompile(`^\s*(\d+)\s+(\d+\.\d+\.\d+\.\d+)\s+\(\d+\.\d+\.\d+\.\d+\)\s+(\d+\.\d+)\s+ms`)

// parseTracerouteOutput parses standard traceroute output into a slice of hops.
// Lines with asterisks (timeouts) are skipped.
func parseTracerouteOutput(output string) []tracerouteHop {
	if output == "" {
		return nil
	}

	var hops []tracerouteHop
	for _, line := range strings.Split(output, "\n") {
		matches := hopRegex.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		hopNum, _ := strconv.Atoi(matches[1])
		hops = append(hops, tracerouteHop{
			HopNum:  hopNum,
			IP:      matches[2],
			Latency: matches[3] + "ms",
		})
	}

	if len(hops) == 0 {
		return nil
	}
	return hops
}

// nftCountersDiff computes the difference between before and after nftables counter
// snapshots. Returns only rules where the packet count increased.
// Key format is "table/chain/rule"; split into chain (first two parts) and rule (last part).
func nftCountersDiff(before, after map[string]int64) []v1alpha1.NFTablesRuleHit {
	var hits []v1alpha1.NFTablesRuleHit

	for key, afterCount := range after {
		beforeCount := before[key]
		diff := afterCount - beforeCount
		if diff <= 0 {
			continue
		}

		// Split "table/chain/rule" into chain="table/chain" and rule="rule"
		lastSlash := strings.LastIndex(key, "/")
		if lastSlash < 0 {
			continue
		}
		chain := key[:lastSlash]
		rule := key[lastSlash+1:]

		hits = append(hits, v1alpha1.NFTablesRuleHit{
			Chain:   chain,
			Rule:    rule,
			Packets: diff,
		})
	}

	if len(hits) == 0 {
		return nil
	}

	// Sort for deterministic output
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Chain != hits[j].Chain {
			return hits[i].Chain < hits[j].Chain
		}
		return hits[i].Rule < hits[j].Rule
	})

	return hits
}

// counterRegex matches "counter packets N bytes M" in nft output, capturing the packet count
// and the trailing action keyword (e.g. masquerade, accept, drop).
var counterRegex = regexp.MustCompile(`counter packets (\d+) bytes \d+\s+(\S+)`)

// tableChainRegex matches "table <family> <name> {" lines.
var tableRegex = regexp.MustCompile(`^table\s+\S+\s+(\S+)\s+\{`)

// chainRegex matches "chain <name> {" lines.
var chainRegex = regexp.MustCompile(`^\s+chain\s+(\S+)\s+\{`)

// parseNftCounters parses `nft list ruleset` output into a map of
// "table/chain/action" -> packet count.
func parseNftCounters(output string) map[string]int64 {
	result := make(map[string]int64)
	if output == "" {
		return result
	}

	var currentTable, currentChain string

	for _, line := range strings.Split(output, "\n") {
		if m := tableRegex.FindStringSubmatch(line); m != nil {
			currentTable = m[1]
			currentChain = ""
			continue
		}
		if m := chainRegex.FindStringSubmatch(line); m != nil {
			currentChain = m[1]
			continue
		}
		if m := counterRegex.FindStringSubmatch(line); m != nil && currentTable != "" && currentChain != "" {
			packets, _ := strconv.ParseInt(m[1], 10, 64)
			action := m[2]
			key := currentTable + "/" + currentChain + "/" + action
			result[key] = packets
		}
	}

	return result
}

// buildProbeCommand builds the command to execute for a network probe.
// TCP/UDP use nping; ICMP uses ping.
func buildProbeCommand(dest string, port int32, protocol string, timeoutSec int) []string {
	switch strings.ToUpper(protocol) {
	case "TCP":
		return []string{"nping", "--tcp", "-p", fmt.Sprintf("%d", port), "-c", "3", "--delay", "1s", dest}
	case "UDP":
		return []string{"nping", "--udp", "-p", fmt.Sprintf("%d", port), "-c", "3", "--delay", "1s", dest}
	default: // ICMP
		return []string{"ping", "-c", "3", "-W", strconv.Itoa(timeoutSec), dest}
	}
}

// buildTracerouteCommand builds the traceroute command for path discovery.
func buildTracerouteCommand(dest string, timeoutSec int) []string {
	return []string{"traceroute", "-n", "-w", strconv.Itoa(timeoutSec), dest}
}
