package collectors

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

const sampleProcNetDev = `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
    lo: 1000       10    0    0    0     0          0         0     1000       10    0    0    0     0       0          0
uplink: 123456789 987654    5    2    0     0          0         0 987654321 876543   10    3    0     0       0          0
  net0: 555555    4444    1    0    0     0          0         0  666666    3333    0    1    0     0       0          0
  net1: 111111    2222    0    0    0     0          0         0  222222    1111    0    0    0     0       0          0
`

func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "proc_net_dev")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	return path
}

func TestParseProcNetDev(t *testing.T) {
	path := writeTempFile(t, sampleProcNetDev)
	stats, err := parseProcNetDev(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(stats) != 3 {
		t.Fatalf("expected 3 interfaces (lo excluded), got %d", len(stats))
	}

	// Check uplink
	uplink := stats[0]
	if uplink.Name != "uplink" {
		t.Errorf("expected interface name 'uplink', got %q", uplink.Name)
	}
	if uplink.RxBytes != 123456789 {
		t.Errorf("uplink RxBytes = %v, want 123456789", uplink.RxBytes)
	}
	if uplink.TxBytes != 987654321 {
		t.Errorf("uplink TxBytes = %v, want 987654321", uplink.TxBytes)
	}
	if uplink.RxErrors != 5 {
		t.Errorf("uplink RxErrors = %v, want 5", uplink.RxErrors)
	}
	if uplink.TxDrops != 3 {
		t.Errorf("uplink TxDrops = %v, want 3", uplink.TxDrops)
	}
}

func TestInterfaceCollector_Collect(t *testing.T) {
	path := writeTempFile(t, sampleProcNetDev)
	c := newInterfaceCollector(path)

	ch := make(chan prometheus.Metric, 100)
	c.Collect(ch)
	close(ch)

	// 3 interfaces * 8 metrics each = 24 metrics
	count := 0
	for m := range ch {
		count++
		d := &dto.Metric{}
		m.Write(d)
		if d.Counter == nil {
			t.Errorf("expected counter metric, got %v", d)
		}
	}
	if count != 24 {
		t.Errorf("expected 24 metrics, got %d", count)
	}
}

func TestParseProcNetDev_FileNotFound(t *testing.T) {
	_, err := parseProcNetDev("/nonexistent/path")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}
