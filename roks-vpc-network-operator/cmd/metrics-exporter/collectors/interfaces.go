package collectors

import (
	"bufio"
	"os"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

// InterfaceCollector reads /proc/net/dev and exposes per-interface counters.
type InterfaceCollector struct {
	rxBytes   *prometheus.Desc
	txBytes   *prometheus.Desc
	rxPackets *prometheus.Desc
	txPackets *prometheus.Desc
	rxErrors  *prometheus.Desc
	txErrors  *prometheus.Desc
	rxDrops   *prometheus.Desc
	txDrops   *prometheus.Desc

	// procPath allows overriding for tests.
	procPath string
}

func NewInterfaceCollector() *InterfaceCollector {
	return newInterfaceCollector("/proc/net/dev")
}

func newInterfaceCollector(procPath string) *InterfaceCollector {
	return &InterfaceCollector{
		procPath: procPath,
		rxBytes: prometheus.NewDesc("router_interface_rx_bytes_total",
			"Total bytes received on interface", []string{"interface"}, nil),
		txBytes: prometheus.NewDesc("router_interface_tx_bytes_total",
			"Total bytes transmitted on interface", []string{"interface"}, nil),
		rxPackets: prometheus.NewDesc("router_interface_rx_packets_total",
			"Total packets received on interface", []string{"interface"}, nil),
		txPackets: prometheus.NewDesc("router_interface_tx_packets_total",
			"Total packets transmitted on interface", []string{"interface"}, nil),
		rxErrors: prometheus.NewDesc("router_interface_rx_errors_total",
			"Total receive errors on interface", []string{"interface"}, nil),
		txErrors: prometheus.NewDesc("router_interface_tx_errors_total",
			"Total transmit errors on interface", []string{"interface"}, nil),
		rxDrops: prometheus.NewDesc("router_interface_rx_drops_total",
			"Total receive drops on interface", []string{"interface"}, nil),
		txDrops: prometheus.NewDesc("router_interface_tx_drops_total",
			"Total transmit drops on interface", []string{"interface"}, nil),
	}
}

func (c *InterfaceCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.rxBytes
	ch <- c.txBytes
	ch <- c.rxPackets
	ch <- c.txPackets
	ch <- c.rxErrors
	ch <- c.txErrors
	ch <- c.rxDrops
	ch <- c.txDrops
}

func (c *InterfaceCollector) Collect(ch chan<- prometheus.Metric) {
	stats, err := parseProcNetDev(c.procPath)
	if err != nil {
		return
	}

	for _, s := range stats {
		ch <- prometheus.MustNewConstMetric(c.rxBytes, prometheus.CounterValue, s.RxBytes, s.Name)
		ch <- prometheus.MustNewConstMetric(c.txBytes, prometheus.CounterValue, s.TxBytes, s.Name)
		ch <- prometheus.MustNewConstMetric(c.rxPackets, prometheus.CounterValue, s.RxPackets, s.Name)
		ch <- prometheus.MustNewConstMetric(c.txPackets, prometheus.CounterValue, s.TxPackets, s.Name)
		ch <- prometheus.MustNewConstMetric(c.rxErrors, prometheus.CounterValue, s.RxErrors, s.Name)
		ch <- prometheus.MustNewConstMetric(c.txErrors, prometheus.CounterValue, s.TxErrors, s.Name)
		ch <- prometheus.MustNewConstMetric(c.rxDrops, prometheus.CounterValue, s.RxDrops, s.Name)
		ch <- prometheus.MustNewConstMetric(c.txDrops, prometheus.CounterValue, s.TxDrops, s.Name)
	}
}

type interfaceStats struct {
	Name      string
	RxBytes   float64
	TxBytes   float64
	RxPackets float64
	TxPackets float64
	RxErrors  float64
	TxErrors  float64
	RxDrops   float64
	TxDrops   float64
}

// parseProcNetDev parses /proc/net/dev format.
// Format: iface: rx_bytes rx_packets rx_errs rx_drop ... tx_bytes tx_packets tx_errs tx_drop ...
func parseProcNetDev(path string) ([]interfaceStats, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var results []interfaceStats
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		// Skip header lines
		if lineNum <= 2 {
			continue
		}

		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		name := strings.TrimSpace(parts[0])
		// Skip loopback
		if name == "lo" {
			continue
		}

		fields := strings.Fields(parts[1])
		if len(fields) < 16 {
			continue
		}

		s := interfaceStats{Name: name}
		s.RxBytes = parseFloat(fields[0])
		s.RxPackets = parseFloat(fields[1])
		s.RxErrors = parseFloat(fields[2])
		s.RxDrops = parseFloat(fields[3])
		s.TxBytes = parseFloat(fields[8])
		s.TxPackets = parseFloat(fields[9])
		s.TxErrors = parseFloat(fields[10])
		s.TxDrops = parseFloat(fields[11])

		results = append(results, s)
	}

	return results, scanner.Err()
}

func parseFloat(s string) float64 {
	v, _ := strconv.ParseFloat(s, 64)
	return v
}
