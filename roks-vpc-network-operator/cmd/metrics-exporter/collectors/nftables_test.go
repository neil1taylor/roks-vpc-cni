package collectors

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

const sampleNftJSON = `{
  "nftables": [
    {"metainfo": {"version": "1.0.9"}},
    {
      "rule": {
        "family": "ip",
        "table": "nat",
        "chain": "postrouting",
        "comment": "snat-workload",
        "expr": [
          {"match": {"left": {"meta": {"key": "oifname"}}, "right": "uplink"}},
          {"counter": {"packets": 12345, "bytes": 6789012}},
          {"snat": {"addr": "10.240.1.5"}}
        ]
      }
    },
    {
      "rule": {
        "family": "ip",
        "table": "filter",
        "chain": "forward",
        "comment": "allow-ssh",
        "expr": [
          {"match": {"left": {"payload": {"protocol": "tcp", "field": "dport"}}, "right": 22}},
          {"counter": {"packets": 100, "bytes": 5000}},
          {"accept": null}
        ]
      }
    },
    {
      "rule": {
        "family": "ip",
        "table": "filter",
        "chain": "forward",
        "expr": [
          {"counter": {"packets": 0, "bytes": 0}},
          {"drop": null}
        ]
      }
    }
  ]
}`

func TestParseNftJSON(t *testing.T) {
	rules, err := parseNftJSON([]byte(sampleNftJSON))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(rules) != 3 {
		t.Fatalf("expected 3 rules with counters, got %d", len(rules))
	}

	// First rule: SNAT
	if rules[0].Table != "nat" {
		t.Errorf("rule[0].Table = %q, want 'nat'", rules[0].Table)
	}
	if rules[0].Chain != "postrouting" {
		t.Errorf("rule[0].Chain = %q, want 'postrouting'", rules[0].Chain)
	}
	if rules[0].Comment != "snat-workload" {
		t.Errorf("rule[0].Comment = %q, want 'snat-workload'", rules[0].Comment)
	}
	if rules[0].Packets != 12345 {
		t.Errorf("rule[0].Packets = %v, want 12345", rules[0].Packets)
	}
	if rules[0].Bytes != 6789012 {
		t.Errorf("rule[0].Bytes = %v, want 6789012", rules[0].Bytes)
	}

	// Third rule: unnamed
	if rules[2].Comment != "unnamed" {
		t.Errorf("rule[2].Comment = %q, want 'unnamed'", rules[2].Comment)
	}
}

func TestNftablesCollector_Collect(t *testing.T) {
	c := newNftablesCollector(func() ([]byte, error) {
		return []byte(sampleNftJSON), nil
	})

	ch := make(chan prometheus.Metric, 100)
	c.Collect(ch)
	close(ch)

	// 3 rules * 2 metrics (packets + bytes) = 6 metrics
	count := 0
	for m := range ch {
		count++
		d := &dto.Metric{}
		m.Write(d)
		if d.Counter == nil {
			t.Errorf("expected counter metric")
		}
	}
	if count != 6 {
		t.Errorf("expected 6 metrics, got %d", count)
	}
}

func TestParseNftJSON_InvalidJSON(t *testing.T) {
	_, err := parseNftJSON([]byte("not json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseNftJSON_NoRules(t *testing.T) {
	rules, err := parseNftJSON([]byte(`{"nftables": [{"metainfo": {"version": "1.0"}}]}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rules) != 0 {
		t.Errorf("expected 0 rules, got %d", len(rules))
	}
}
