package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

// startHealthServer starts the HTTP health check server.
func startHealthServer(cfg *Config) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthzHandler)
	mux.HandleFunc("/readyz", readyzHandler)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.HealthPort),
		Handler: mux,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("health server error", "error", err)
		}
	}()

	return srv
}

// healthzHandler checks basic router health:
// - net.ipv4.ip_forward == 1
// - uplink interface is UP
func healthzHandler(w http.ResponseWriter, r *http.Request) {
	// Check IP forwarding
	data, err := os.ReadFile("/proc/sys/net/ipv4/ip_forward")
	if err != nil || strings.TrimSpace(string(data)) != "1" {
		http.Error(w, "ip_forward not enabled", http.StatusServiceUnavailable)
		return
	}

	// Check uplink is UP
	out, err := exec.Command("ip", "link", "show", "uplink").CombinedOutput()
	if err != nil || !strings.Contains(string(out), "UP") {
		http.Error(w, "uplink not UP", http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "ok")
}

// readyzHandler checks router readiness:
// - Default route exists via uplink
// - Uplink has an IP address
func readyzHandler(w http.ResponseWriter, r *http.Request) {
	// Check default route via uplink
	out, err := exec.Command("ip", "route", "show", "default").CombinedOutput()
	if err != nil || !strings.Contains(string(out), "uplink") {
		http.Error(w, "no default route via uplink", http.StatusServiceUnavailable)
		return
	}

	// Check uplink has IP
	out, err = exec.Command("ip", "addr", "show", "dev", "uplink").CombinedOutput()
	if err != nil || !strings.Contains(string(out), "inet ") {
		http.Error(w, "uplink has no IP", http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "ready")
}
