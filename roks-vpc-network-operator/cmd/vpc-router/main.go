package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))
	slog.Info("vpc-router starting")

	cfg, err := parseConfig()
	if err != nil {
		slog.Error("failed to parse config", "error", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	// Step 1: Configure network interfaces
	if err := configureInterfaces(cfg); err != nil {
		slog.Error("failed to configure interfaces", "error", err)
		os.Exit(1)
	}

	// Step 2: Enable IP forwarding
	if err := enableIPForwarding(); err != nil {
		slog.Error("failed to enable IP forwarding", "error", err)
		os.Exit(1)
	}

	// Step 3: Apply nftables rules
	if err := applyNftables(cfg); err != nil {
		slog.Error("failed to apply nftables", "error", err)
		os.Exit(1)
	}

	// Step 4: Start dnsmasq instances
	dnsmasqProcs := startDnsmasq(cfg)

	// Step 5: Attach XDP programs (if enabled)
	var xdpCleanup func()
	if cfg.XDPEnabled {
		var xdpErr error
		xdpCleanup, xdpErr = attachXDP(cfg)
		if xdpErr != nil {
			slog.Warn("XDP attachment failed, continuing with kernel forwarding", "error", xdpErr)
		} else {
			slog.Info("XDP fast-path forwarding attached")
		}
	}

	// Step 6: Start health server
	healthSrv := startHealthServer(cfg)
	slog.Info("vpc-router ready", "mode", "fast-path", "xdp", cfg.XDPEnabled, "healthPort", cfg.HealthPort)

	// Wait for shutdown signal
	<-ctx.Done()
	slog.Info("shutting down")

	// Cleanup
	if xdpCleanup != nil {
		xdpCleanup()
	}
	for _, proc := range dnsmasqProcs {
		if proc.Process != nil {
			_ = proc.Process.Signal(syscall.SIGTERM)
		}
	}
	if healthSrv != nil {
		_ = healthSrv.Close()
	}

	slog.Info("vpc-router shutdown complete")
}
