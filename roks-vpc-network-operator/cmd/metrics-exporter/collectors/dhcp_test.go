package collectors

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

const sampleLeaseFile = `1709332800 fa:16:3e:aa:bb:01 10.100.0.11 vm-1 *
1709332801 fa:16:3e:aa:bb:02 10.100.0.12 vm-2 *
1709332802 fa:16:3e:aa:bb:03 10.100.0.13 vm-3 *
`

func TestDHCPCollector_CountLeases(t *testing.T) {
	dir := t.TempDir()

	// Write interface-specific lease files
	os.WriteFile(filepath.Join(dir, "dnsmasq-net0.leases"), []byte(sampleLeaseFile), 0644)
	os.WriteFile(filepath.Join(dir, "dnsmasq-net1.leases"), []byte("1709332800 fa:16:3e:cc:dd:01 10.200.0.11 vm-4 *\n"), 0644)

	c := NewDHCPCollector(dir)
	counts := c.countLeases()

	if counts["net0"] != 3 {
		t.Errorf("net0 leases = %d, want 3", counts["net0"])
	}
	if counts["net1"] != 1 {
		t.Errorf("net1 leases = %d, want 1", counts["net1"])
	}
}

func TestDHCPCollector_GlobalLeaseFile(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "dnsmasq.leases"), []byte(sampleLeaseFile), 0644)

	c := NewDHCPCollector(dir)
	counts := c.countLeases()

	if counts["all"] != 3 {
		t.Errorf("global leases = %d, want 3", counts["all"])
	}
}

func TestDHCPCollector_Collect(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "dnsmasq-net0.leases"), []byte(sampleLeaseFile), 0644)

	// Set pool size env vars
	t.Setenv("DHCP_POOL_NET0_SIZE", "244")

	c := NewDHCPCollector(dir)

	ch := make(chan prometheus.Metric, 20)
	c.Collect(ch)
	close(ch)

	var leaseMetric, poolMetric *dto.Metric
	for m := range ch {
		d := &dto.Metric{}
		m.Write(d)
		if d.Gauge != nil {
			for _, lp := range d.Label {
				if lp.GetName() == "interface" && lp.GetValue() == "net0" {
					if *d.Gauge.Value == 3 {
						leaseMetric = d
					} else if *d.Gauge.Value == 244 {
						poolMetric = d
					}
				}
			}
		}
	}

	if leaseMetric == nil {
		t.Error("expected lease metric for net0 with value 3")
	}
	if poolMetric == nil {
		t.Error("expected pool size metric for net0 with value 244")
	}
}

func TestDHCPCollector_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	c := NewDHCPCollector(dir)
	counts := c.countLeases()
	if len(counts) != 0 {
		t.Errorf("expected 0 lease counts, got %d", len(counts))
	}
}

func TestCountLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.leases")

	// File with blank lines and comments
	os.WriteFile(path, []byte("line1\n\n# comment\nline2\n"), 0644)
	n := countLines(path)
	if n != 2 {
		t.Errorf("countLines = %d, want 2 (excluding blanks and comments)", n)
	}
}
