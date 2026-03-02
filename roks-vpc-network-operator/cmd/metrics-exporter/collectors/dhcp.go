package collectors

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

// DHCPCollector parses dnsmasq lease files and exposes DHCP pool metrics.
type DHCPCollector struct {
	activeLeases *prometheus.Desc
	poolSize     *prometheus.Desc

	leaseDir string
}

func NewDHCPCollector(leaseDir string) *DHCPCollector {
	return &DHCPCollector{
		leaseDir: leaseDir,
		activeLeases: prometheus.NewDesc("router_dhcp_active_leases",
			"Number of active DHCP leases", []string{"interface"}, nil),
		poolSize: prometheus.NewDesc("router_dhcp_pool_size",
			"Total DHCP pool size", []string{"interface"}, nil),
	}
}

func (c *DHCPCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.activeLeases
	ch <- c.poolSize
}

func (c *DHCPCollector) Collect(ch chan<- prometheus.Metric) {
	// Count leases per interface from dnsmasq lease files
	// dnsmasq writes to /var/lib/misc/dnsmasq.leases or per-interface files
	leases := c.countLeases()
	for iface, count := range leases {
		ch <- prometheus.MustNewConstMetric(c.activeLeases, prometheus.GaugeValue, float64(count), iface)
	}

	// Pool sizes come from environment variables: DHCP_POOL_<IFACE>_SIZE
	for _, env := range os.Environ() {
		if !strings.HasPrefix(env, "DHCP_POOL_") || !strings.HasSuffix(strings.SplitN(env, "=", 2)[0], "_SIZE") {
			continue
		}
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}
		// Extract interface name from DHCP_POOL_NET0_SIZE
		key := parts[0]
		iface := strings.ToLower(strings.TrimSuffix(strings.TrimPrefix(key, "DHCP_POOL_"), "_SIZE"))
		size, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			continue
		}
		ch <- prometheus.MustNewConstMetric(c.poolSize, prometheus.GaugeValue, size, iface)
	}
}

// countLeases reads dnsmasq lease files and counts active leases per interface.
// dnsmasq lease format: timestamp mac ip hostname client-id
// Interface-specific files: dnsmasq-<iface>.leases
func (c *DHCPCollector) countLeases() map[string]int {
	counts := make(map[string]int)

	// Try interface-specific lease files first
	matches, err := filepath.Glob(filepath.Join(c.leaseDir, "dnsmasq-*.leases"))
	if err == nil {
		for _, f := range matches {
			base := filepath.Base(f)
			iface := strings.TrimPrefix(strings.TrimSuffix(base, ".leases"), "dnsmasq-")
			n := countLines(f)
			if n > 0 {
				counts[iface] = n
			}
		}
	}

	// Also try the single global lease file
	globalFile := filepath.Join(c.leaseDir, "dnsmasq.leases")
	if n := countLines(globalFile); n > 0 {
		if len(counts) == 0 {
			counts["all"] = n
		}
	}

	return counts
}

func countLines(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()

	count := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			count++
		}
	}
	return count
}
