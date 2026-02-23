package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/IBM/roks-vpc-network-operator/cmd/bff/internal/auth"
	"github.com/IBM/roks-vpc-network-operator/cmd/bff/internal/credentials"
	"github.com/IBM/roks-vpc-network-operator/cmd/bff/internal/handler"
	"github.com/IBM/roks-vpc-network-operator/pkg/vpc"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func main() {
	ctx := context.Background()

	// Setup structured logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Load VPC credentials
	apiKey, err := credentials.LoadCredentials(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to load credentials", "error", err)
		os.Exit(1)
	}

	// Create VPC client
	vpcClient, err := vpc.NewExtendedClient(vpc.ClientConfig{
		APIKey: apiKey,
		Region: os.Getenv("VPC_REGION"),
	})
	if err != nil {
		slog.ErrorContext(ctx, "failed to create VPC client", "error", err)
		os.Exit(1)
	}

	// Create Kubernetes client
	k8sConfig, err := rest.InClusterConfig()
	if err != nil {
		slog.ErrorContext(ctx, "failed to get in-cluster K8s config", "error", err)
		os.Exit(1)
	}

	k8sClient, err := kubernetes.NewForConfig(k8sConfig)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create K8s client", "error", err)
		os.Exit(1)
	}

	// Create RBAC checker
	rbacChecker := auth.NewRBACChecker(k8sClient)

	// Determine cluster mode (ROKS vs unmanaged)
	// BFF_CLUSTER_MODE is set by the Helm chart from .Values.bff.clusterMode
	clusterMode := os.Getenv("BFF_CLUSTER_MODE")
	if clusterMode == "" {
		clusterMode = "unmanaged"
	}
	slog.InfoContext(ctx, "cluster mode", "mode", clusterMode)

	// Create HTTP server
	mux := http.NewServeMux()
	handler.SetupRoutesWithClusterInfo(mux, vpcClient, rbacChecker, k8sClient, handler.ClusterInfo{
		Mode: clusterMode,
	})

	listenAddr := os.Getenv("BFF_LISTEN_ADDR")
	if listenAddr == "" {
		listenAddr = ":8443"
	}

	server := &http.Server{
		Addr:         listenAddr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in background
	errs := make(chan error, 1)
	go func() {
		slog.InfoContext(ctx, "starting BFF server", "addr", listenAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errs <- err
		}
	}()

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errs:
		slog.ErrorContext(ctx, "server error", "error", err)
		os.Exit(1)
	case sig := <-sigChan:
		slog.InfoContext(ctx, "received signal", "signal", sig)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.ErrorContext(ctx, "shutdown error", "error", err)
			os.Exit(1)
		}
		slog.InfoContext(ctx, "server shutdown successfully")
	}
}
