package collectors

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestConntrackCollector_Collect(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "nf_conntrack_count"), []byte("1234\n"), 0644)
	os.WriteFile(filepath.Join(dir, "nf_conntrack_max"), []byte("65536\n"), 0644)

	c := newConntrackCollector(dir)

	ch := make(chan prometheus.Metric, 10)
	c.Collect(ch)
	close(ch)

	metrics := make([]*dto.Metric, 0)
	for m := range ch {
		d := &dto.Metric{}
		m.Write(d)
		metrics = append(metrics, d)
	}

	if len(metrics) != 2 {
		t.Fatalf("expected 2 metrics, got %d", len(metrics))
	}

	// Both should be gauge type
	for _, m := range metrics {
		if m.Gauge == nil {
			t.Error("expected gauge metric")
		}
	}

	if *metrics[0].Gauge.Value != 1234 {
		t.Errorf("conntrack entries = %v, want 1234", *metrics[0].Gauge.Value)
	}
	if *metrics[1].Gauge.Value != 65536 {
		t.Errorf("conntrack max = %v, want 65536", *metrics[1].Gauge.Value)
	}
}

func TestConntrackCollector_MissingFiles(t *testing.T) {
	c := newConntrackCollector("/nonexistent/path")

	ch := make(chan prometheus.Metric, 10)
	c.Collect(ch)
	close(ch)

	count := 0
	for range ch {
		count++
	}
	if count != 0 {
		t.Errorf("expected 0 metrics for missing files, got %d", count)
	}
}

func TestReadProcInt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test")
	os.WriteFile(path, []byte("42\n"), 0644)

	v := readProcInt(path)
	if v != 42 {
		t.Errorf("readProcInt = %d, want 42", v)
	}

	// Non-numeric content
	os.WriteFile(path, []byte("not a number\n"), 0644)
	v = readProcInt(path)
	if v != -1 {
		t.Errorf("readProcInt for non-numeric = %d, want -1", v)
	}
}
