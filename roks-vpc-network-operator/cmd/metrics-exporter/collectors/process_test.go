package collectors

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestProcessCollector_FindRunning(t *testing.T) {
	dir := t.TempDir()

	// Create fake /proc/<pid>/comm files
	for _, entry := range []struct {
		pid  string
		comm string
	}{
		{"123", "dnsmasq"},
		{"456", "suricata"},
		{"789", "bash"},
	} {
		pidDir := filepath.Join(dir, entry.pid)
		os.MkdirAll(pidDir, 0755)
		os.WriteFile(filepath.Join(pidDir, "comm"), []byte(entry.comm+"\n"), 0644)
	}

	c := newProcessCollector(dir, time.Now())
	running := c.findRunningProcesses()

	if !running["dnsmasq"] {
		t.Error("expected dnsmasq to be running")
	}
	if !running["suricata"] {
		t.Error("expected suricata to be running")
	}
	if running["dhclient"] {
		t.Error("expected dhclient to not be running")
	}
	if running["bash"] {
		t.Error("bash should not be in tracked processes")
	}
}

func TestProcessCollector_Collect(t *testing.T) {
	dir := t.TempDir()

	// Only dnsmasq running
	pidDir := filepath.Join(dir, "1")
	os.MkdirAll(pidDir, 0755)
	os.WriteFile(filepath.Join(pidDir, "comm"), []byte("dnsmasq\n"), 0644)

	startTime := time.Now().Add(-time.Hour)
	c := newProcessCollector(dir, startTime)

	ch := make(chan prometheus.Metric, 20)
	c.Collect(ch)
	close(ch)

	// 3 tracked processes + 1 uptime = 4 metrics
	count := 0
	for m := range ch {
		count++
		d := &dto.Metric{}
		m.Write(d)
		if d.Gauge == nil {
			t.Error("expected gauge metric")
		}
	}

	if count != 4 {
		t.Errorf("expected 4 metrics, got %d", count)
	}
}

func TestProcessCollector_Uptime(t *testing.T) {
	dir := t.TempDir()
	startTime := time.Now().Add(-2 * time.Hour)
	c := newProcessCollector(dir, startTime)

	ch := make(chan prometheus.Metric, 20)
	c.Collect(ch)
	close(ch)

	// Find the uptime metric (no labels)
	for m := range ch {
		d := &dto.Metric{}
		m.Write(d)
		if len(d.Label) == 0 && d.Gauge != nil {
			uptime := *d.Gauge.Value
			if uptime < 7200 || uptime > 7210 {
				t.Errorf("uptime = %v, expected ~7200", uptime)
			}
			return
		}
	}
	t.Error("uptime metric not found")
}
