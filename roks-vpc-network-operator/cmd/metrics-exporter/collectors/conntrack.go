package collectors

import (
	"os"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

// ConntrackCollector reads conntrack count and max from /proc/sys/net/netfilter/.
type ConntrackCollector struct {
	entries *prometheus.Desc
	max     *prometheus.Desc

	// basePath allows overriding for tests.
	basePath string
}

func NewConntrackCollector() *ConntrackCollector {
	return newConntrackCollector("/proc/sys/net/netfilter")
}

func newConntrackCollector(basePath string) *ConntrackCollector {
	return &ConntrackCollector{
		basePath: basePath,
		entries: prometheus.NewDesc("router_conntrack_entries",
			"Current number of conntrack entries", nil, nil),
		max: prometheus.NewDesc("router_conntrack_max",
			"Maximum number of conntrack entries", nil, nil),
	}
}

func (c *ConntrackCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.entries
	ch <- c.max
}

func (c *ConntrackCollector) Collect(ch chan<- prometheus.Metric) {
	count := readProcInt(c.basePath + "/nf_conntrack_count")
	max := readProcInt(c.basePath + "/nf_conntrack_max")

	if count >= 0 {
		ch <- prometheus.MustNewConstMetric(c.entries, prometheus.GaugeValue, float64(count))
	}
	if max >= 0 {
		ch <- prometheus.MustNewConstMetric(c.max, prometheus.GaugeValue, float64(max))
	}
}

func readProcInt(path string) int64 {
	data, err := os.ReadFile(path)
	if err != nil {
		return -1
	}
	v, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return -1
	}
	return v
}
