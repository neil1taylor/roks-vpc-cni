package collectors

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// ProcessCollector checks if key processes (dnsmasq, dhclient, suricata) are running.
type ProcessCollector struct {
	running *prometheus.Desc
	uptime  *prometheus.Desc

	startTime time.Time

	// procDir allows overriding for tests.
	procDir string
}

func NewProcessCollector() *ProcessCollector {
	return newProcessCollector("/proc", time.Now())
}

func newProcessCollector(procDir string, startTime time.Time) *ProcessCollector {
	return &ProcessCollector{
		procDir:   procDir,
		startTime: startTime,
		running: prometheus.NewDesc("router_process_running",
			"Whether a process is running (1=yes, 0=no)", []string{"process"}, nil),
		uptime: prometheus.NewDesc("router_uptime_seconds",
			"Uptime of the metrics exporter in seconds", nil, nil),
	}
}

func (c *ProcessCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.running
	ch <- c.uptime
}

var trackedProcesses = []string{"dnsmasq", "dhclient", "suricata"}

func (c *ProcessCollector) Collect(ch chan<- prometheus.Metric) {
	running := c.findRunningProcesses()
	for _, proc := range trackedProcesses {
		val := float64(0)
		if running[proc] {
			val = 1
		}
		ch <- prometheus.MustNewConstMetric(c.running, prometheus.GaugeValue, val, proc)
	}

	ch <- prometheus.MustNewConstMetric(c.uptime, prometheus.GaugeValue,
		time.Since(c.startTime).Seconds())
}

// findRunningProcesses scans /proc/*/comm for tracked process names.
func (c *ProcessCollector) findRunningProcesses() map[string]bool {
	found := make(map[string]bool)

	entries, err := os.ReadDir(c.procDir)
	if err != nil {
		return found
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		// Only check numeric directories (PIDs)
		name := entry.Name()
		if len(name) == 0 || name[0] < '0' || name[0] > '9' {
			continue
		}

		commPath := filepath.Join(c.procDir, name, "comm")
		data, err := os.ReadFile(commPath)
		if err != nil {
			continue
		}

		comm := strings.TrimSpace(string(data))
		for _, proc := range trackedProcesses {
			if comm == proc {
				found[proc] = true
			}
		}
	}

	return found
}
