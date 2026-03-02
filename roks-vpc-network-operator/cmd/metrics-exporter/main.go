package main

import (
	"log"
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/IBM/roks-vpc-network-operator/cmd/metrics-exporter/collectors"
)

func main() {
	port := os.Getenv("METRICS_PORT")
	if port == "" {
		port = "9100"
	}

	leaseDir := os.Getenv("DNSMASQ_LEASE_DIR")
	if leaseDir == "" {
		leaseDir = "/var/lib/misc"
	}

	reg := prometheus.NewRegistry()
	reg.MustRegister(collectors.NewInterfaceCollector())
	reg.MustRegister(collectors.NewNftablesCollector())
	reg.MustRegister(collectors.NewConntrackCollector())
	reg.MustRegister(collectors.NewDHCPCollector(leaseDir))
	reg.MustRegister(collectors.NewProcessCollector())

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	addr := ":" + port
	log.Printf("metrics-exporter listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
