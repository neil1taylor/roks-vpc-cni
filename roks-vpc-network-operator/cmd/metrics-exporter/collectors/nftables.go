package collectors

import (
	"encoding/json"
	"os/exec"

	"github.com/prometheus/client_golang/prometheus"
)

// NftablesCollector runs `nft -j list ruleset` and exposes per-rule counters.
type NftablesCollector struct {
	rulePackets *prometheus.Desc
	ruleBytes   *prometheus.Desc

	// execFn allows overriding for tests.
	execFn func() ([]byte, error)
}

func NewNftablesCollector() *NftablesCollector {
	return newNftablesCollector(nil)
}

func newNftablesCollector(execFn func() ([]byte, error)) *NftablesCollector {
	if execFn == nil {
		execFn = func() ([]byte, error) {
			return exec.Command("nft", "-j", "list", "ruleset").Output()
		}
	}
	return &NftablesCollector{
		execFn: execFn,
		rulePackets: prometheus.NewDesc("router_nft_rule_packets_total",
			"Total packets matched by nftables rule", []string{"table", "chain", "comment"}, nil),
		ruleBytes: prometheus.NewDesc("router_nft_rule_bytes_total",
			"Total bytes matched by nftables rule", []string{"table", "chain", "comment"}, nil),
	}
}

func (c *NftablesCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.rulePackets
	ch <- c.ruleBytes
}

func (c *NftablesCollector) Collect(ch chan<- prometheus.Metric) {
	data, err := c.execFn()
	if err != nil {
		return
	}

	rules, err := parseNftJSON(data)
	if err != nil {
		return
	}

	for _, r := range rules {
		ch <- prometheus.MustNewConstMetric(c.rulePackets, prometheus.CounterValue, r.Packets, r.Table, r.Chain, r.Comment)
		ch <- prometheus.MustNewConstMetric(c.ruleBytes, prometheus.CounterValue, r.Bytes, r.Table, r.Chain, r.Comment)
	}
}

type nftRuleCounter struct {
	Table   string
	Chain   string
	Comment string
	Packets float64
	Bytes   float64
}

// parseNftJSON parses the JSON output of `nft -j list ruleset`.
// The format is: {"nftables": [{...}, {"rule": {...}}, ...]}
func parseNftJSON(data []byte) ([]nftRuleCounter, error) {
	var top struct {
		Nftables []json.RawMessage `json:"nftables"`
	}
	if err := json.Unmarshal(data, &top); err != nil {
		return nil, err
	}

	var results []nftRuleCounter

	for _, raw := range top.Nftables {
		var wrapper map[string]json.RawMessage
		if err := json.Unmarshal(raw, &wrapper); err != nil {
			continue
		}

		ruleData, ok := wrapper["rule"]
		if !ok {
			continue
		}

		var rule struct {
			Family string `json:"family"`
			Table  string `json:"table"`
			Chain  string `json:"chain"`
			Expr   []json.RawMessage `json:"expr"`
			Comment string `json:"comment"`
		}
		if err := json.Unmarshal(ruleData, &rule); err != nil {
			continue
		}

		// Find counter expression in rule
		for _, exprRaw := range rule.Expr {
			var exprWrapper map[string]json.RawMessage
			if err := json.Unmarshal(exprRaw, &exprWrapper); err != nil {
				continue
			}

			counterData, ok := exprWrapper["counter"]
			if !ok {
				continue
			}

			var counter struct {
				Packets float64 `json:"packets"`
				Bytes   float64 `json:"bytes"`
			}
			if err := json.Unmarshal(counterData, &counter); err != nil {
				continue
			}

			comment := rule.Comment
			if comment == "" {
				comment = "unnamed"
			}

			results = append(results, nftRuleCounter{
				Table:   rule.Table,
				Chain:   rule.Chain,
				Comment: comment,
				Packets: counter.Packets,
				Bytes:   counter.Bytes,
			})
		}
	}

	return results, nil
}
