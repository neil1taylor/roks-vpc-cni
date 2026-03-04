package traceflow

import (
	"testing"

	v1alpha1 "github.com/IBM/roks-vpc-network-operator/api/v1alpha1"
)

func TestParseTracerouteOutput(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected []tracerouteHop
	}{
		{
			name: "standard three-hop traceroute",
			output: `traceroute to 8.8.8.8 (8.8.8.8), 30 hops max, 60 byte packets
 1  172.16.100.1 (172.16.100.1)  0.456 ms  0.312 ms  0.298 ms
 2  10.0.0.1 (10.0.0.1)  1.234 ms  1.100 ms  1.050 ms
 3  8.8.8.8 (8.8.8.8)  5.678 ms  5.500 ms  5.400 ms`,
			expected: []tracerouteHop{
				{HopNum: 1, IP: "172.16.100.1", Latency: "0.456ms"},
				{HopNum: 2, IP: "10.0.0.1", Latency: "1.234ms"},
				{HopNum: 3, IP: "8.8.8.8", Latency: "5.678ms"},
			},
		},
		{
			name: "hop with asterisks (timeout)",
			output: `traceroute to 10.0.0.5 (10.0.0.5), 30 hops max, 60 byte packets
 1  172.16.100.1 (172.16.100.1)  0.456 ms  0.312 ms  0.298 ms
 2  * * *
 3  10.0.0.5 (10.0.0.5)  3.210 ms  3.100 ms  3.050 ms`,
			expected: []tracerouteHop{
				{HopNum: 1, IP: "172.16.100.1", Latency: "0.456ms"},
				{HopNum: 3, IP: "10.0.0.5", Latency: "3.210ms"},
			},
		},
		{
			name:     "empty output",
			output:   "",
			expected: nil,
		},
		{
			name: "single hop",
			output: `traceroute to 172.16.100.1 (172.16.100.1), 30 hops max, 60 byte packets
 1  172.16.100.1 (172.16.100.1)  0.123 ms  0.100 ms  0.090 ms`,
			expected: []tracerouteHop{
				{HopNum: 1, IP: "172.16.100.1", Latency: "0.123ms"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hops := parseTracerouteOutput(tt.output)
			if len(hops) != len(tt.expected) {
				t.Fatalf("expected %d hops, got %d", len(tt.expected), len(hops))
			}
			for i, hop := range hops {
				if hop.HopNum != tt.expected[i].HopNum {
					t.Errorf("hop %d: expected HopNum %d, got %d", i, tt.expected[i].HopNum, hop.HopNum)
				}
				if hop.IP != tt.expected[i].IP {
					t.Errorf("hop %d: expected IP %s, got %s", i, tt.expected[i].IP, hop.IP)
				}
				if hop.Latency != tt.expected[i].Latency {
					t.Errorf("hop %d: expected Latency %s, got %s", i, tt.expected[i].Latency, hop.Latency)
				}
			}
		})
	}
}

func TestNftCountersDiff(t *testing.T) {
	tests := []struct {
		name     string
		before   map[string]int64
		after    map[string]int64
		expected []v1alpha1.NFTablesRuleHit
	}{
		{
			name: "single rule incremented",
			before: map[string]int64{
				"nat/postrouting/masquerade": 10,
				"filter/forward/accept":     5,
			},
			after: map[string]int64{
				"nat/postrouting/masquerade": 13,
				"filter/forward/accept":     5,
			},
			expected: []v1alpha1.NFTablesRuleHit{
				{Chain: "nat/postrouting", Rule: "masquerade", Packets: 3},
			},
		},
		{
			name: "multiple rules incremented",
			before: map[string]int64{
				"nat/postrouting/masquerade": 10,
				"filter/forward/accept":     5,
			},
			after: map[string]int64{
				"nat/postrouting/masquerade": 15,
				"filter/forward/accept":     8,
			},
			expected: []v1alpha1.NFTablesRuleHit{
				{Chain: "filter/forward", Rule: "accept", Packets: 3},
				{Chain: "nat/postrouting", Rule: "masquerade", Packets: 5},
			},
		},
		{
			name:     "no changes",
			before:   map[string]int64{"nat/postrouting/masquerade": 10},
			after:    map[string]int64{"nat/postrouting/masquerade": 10},
			expected: nil,
		},
		{
			name:   "new rule in after",
			before: map[string]int64{},
			after:  map[string]int64{"filter/input/drop": 2},
			expected: []v1alpha1.NFTablesRuleHit{
				{Chain: "filter/input", Rule: "drop", Packets: 2},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := nftCountersDiff(tt.before, tt.after)
			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d hits, got %d: %+v", len(tt.expected), len(result), result)
			}
			// Build a map for order-independent comparison
			resultMap := make(map[string]v1alpha1.NFTablesRuleHit)
			for _, r := range result {
				resultMap[r.Chain+"/"+r.Rule] = r
			}
			for _, exp := range tt.expected {
				key := exp.Chain + "/" + exp.Rule
				got, ok := resultMap[key]
				if !ok {
					t.Errorf("expected hit for %s not found", key)
					continue
				}
				if got.Packets != exp.Packets {
					t.Errorf("key %s: expected packets %d, got %d", key, exp.Packets, got.Packets)
				}
			}
		})
	}
}

func TestParseNftCounters(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected map[string]int64
	}{
		{
			name: "basic nft ruleset with counters",
			output: `table ip nat {
	chain postrouting {
		type nat hook postrouting priority srcnat; policy accept;
		counter packets 42 bytes 3456 masquerade
	}
}
table ip filter {
	chain forward {
		type filter hook forward priority filter; policy accept;
		counter packets 100 bytes 8000 accept
		counter packets 3 bytes 180 drop
	}
}`,
			expected: map[string]int64{
				"nat/postrouting/masquerade": 42,
				"filter/forward/accept":     100,
				"filter/forward/drop":       3,
			},
		},
		{
			name:     "empty output",
			output:   "",
			expected: map[string]int64{},
		},
		{
			name: "chain with no counter rules",
			output: `table ip filter {
	chain input {
		type filter hook input priority filter; policy accept;
		accept
	}
}`,
			expected: map[string]int64{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseNftCounters(tt.output)
			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d entries, got %d: %v", len(tt.expected), len(result), result)
			}
			for k, v := range tt.expected {
				got, ok := result[k]
				if !ok {
					t.Errorf("expected key %s not found in result", k)
					continue
				}
				if got != v {
					t.Errorf("key %s: expected %d, got %d", k, v, got)
				}
			}
		})
	}
}

func TestBuildProbeCommand(t *testing.T) {
	tests := []struct {
		name       string
		dest       string
		port       int32
		protocol   string
		timeoutSec int
		expected   []string
	}{
		{
			name:       "TCP probe",
			dest:       "10.0.0.5",
			port:       80,
			protocol:   "TCP",
			timeoutSec: 5,
			expected:   []string{"nping", "--tcp", "-p", "80", "-c", "3", "--delay", "1s", "10.0.0.5"},
		},
		{
			name:       "UDP probe",
			dest:       "10.0.0.5",
			port:       53,
			protocol:   "UDP",
			timeoutSec: 5,
			expected:   []string{"nping", "--udp", "-p", "53", "-c", "3", "--delay", "1s", "10.0.0.5"},
		},
		{
			name:       "ICMP probe",
			dest:       "8.8.8.8",
			port:       0,
			protocol:   "ICMP",
			timeoutSec: 10,
			expected:   []string{"ping", "-c", "3", "-W", "10", "8.8.8.8"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildProbeCommand(tt.dest, tt.port, tt.protocol, tt.timeoutSec)
			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d args, got %d: %v", len(tt.expected), len(result), result)
			}
			for i, arg := range tt.expected {
				if result[i] != arg {
					t.Errorf("arg %d: expected %q, got %q", i, arg, result[i])
				}
			}
		})
	}
}

func TestBuildTracerouteCommand(t *testing.T) {
	tests := []struct {
		name       string
		dest       string
		timeoutSec int
		expected   []string
	}{
		{
			name:       "standard traceroute",
			dest:       "8.8.8.8",
			timeoutSec: 5,
			expected:   []string{"traceroute", "-n", "-w", "5", "8.8.8.8"},
		},
		{
			name:       "traceroute with different timeout",
			dest:       "10.0.0.1",
			timeoutSec: 30,
			expected:   []string{"traceroute", "-n", "-w", "30", "10.0.0.1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildTracerouteCommand(tt.dest, tt.timeoutSec)
			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d args, got %d: %v", len(tt.expected), len(result), result)
			}
			for i, arg := range tt.expected {
				if result[i] != arg {
					t.Errorf("arg %d: expected %q, got %q", i, arg, result[i])
				}
			}
		})
	}
}
