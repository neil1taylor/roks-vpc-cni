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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
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
		APIKey:          apiKey,
		Region:          os.Getenv("VPC_REGION"),
		ResourceGroupID: os.Getenv("VPC_RESOURCE_GROUP_ID"),
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

	// Create dynamic client for unstructured resources (CUDN, UDN)
	dynClient, err := dynamic.NewForConfig(k8sConfig)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create dynamic K8s client", "error", err)
		os.Exit(1)
	}

	// Create RBAC checker
	rbacChecker := auth.NewRBACChecker(k8sClient)

	// Enable Bearer token authentication via TokenReview
	auth.SetTokenReviewClient(k8sClient)

	// Determine cluster mode (ROKS vs unmanaged)
	// BFF_CLUSTER_MODE env var overrides auto-detection if explicitly set.
	clusterMode := os.Getenv("BFF_CLUSTER_MODE")
	if clusterMode == "" {
		clusterMode = detectClusterPlatform(ctx, dynClient)
	}

	// Validate VPC_ID at startup — if it fails, fall back to unscoped mode
	vpcID := os.Getenv("VPC_ID")
	if vpcID != "" {
		if _, err := vpcClient.GetVPC(ctx, vpcID); err != nil {
			slog.ErrorContext(ctx, "VPC_ID validation failed — falling back to unscoped mode",
				"vpcID", vpcID, "error", err)
			vpcID = ""
		} else {
			slog.InfoContext(ctx, "VPC_ID validated successfully", "vpcID", vpcID)
		}
	}

	slog.InfoContext(ctx, "cluster info",
		"mode", clusterMode,
		"region", os.Getenv("VPC_REGION"),
		"vpcID", vpcID,
	)
	if os.Getenv("VPC_REGION") == "" {
		slog.WarnContext(ctx, "VPC_REGION is empty — zone listing will fail")
	}

	// Create HTTP server
	mux := http.NewServeMux()
	handler.SetupRoutesWithClusterInfo(mux, vpcClient, rbacChecker, k8sClient, dynClient, handler.ClusterInfo{
		Mode:   clusterMode,
		Region: os.Getenv("VPC_REGION"),
		VPCID:  vpcID,
	}, k8sConfig)

	listenAddr := os.Getenv("BFF_LISTEN_ADDR")
	if listenAddr == "" {
		listenAddr = ":8443"
	}

	server := &http.Server{
		Addr:         listenAddr,
		Handler:      handler.LoggingMiddleware(mux),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// TLS cert/key paths (provided by OpenShift service-serving-cert-signer)
	tlsCert := os.Getenv("BFF_TLS_CERT")
	tlsKey := os.Getenv("BFF_TLS_KEY")

	// Start server in background
	errs := make(chan error, 1)
	go func() {
		if tlsCert != "" && tlsKey != "" {
			slog.InfoContext(ctx, "starting BFF server with TLS", "addr", listenAddr, "cert", tlsCert)
			if err := server.ListenAndServeTLS(tlsCert, tlsKey); err != nil && err != http.ErrServerClosed {
				errs <- err
			}
		} else {
			slog.InfoContext(ctx, "starting BFF server (plain HTTP)", "addr", listenAddr)
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				errs <- err
			}
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

// detectClusterPlatform queries the OpenShift Infrastructure CR to determine
// whether this is a ROKS (IBMCloud) cluster or an unmanaged cluster.
func detectClusterPlatform(ctx context.Context, dynClient dynamic.Interface) string {
	infraGVR := schema.GroupVersionResource{
		Group: "config.openshift.io", Version: "v1", Resource: "infrastructures",
	}
	obj, err := dynClient.Resource(infraGVR).Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		slog.WarnContext(ctx, "could not read Infrastructure object, defaulting to unmanaged", "error", err)
		return "unmanaged"
	}
	platform, _, _ := unstructured.NestedString(obj.Object, "status", "platform")
	if platform == "IBMCloud" {
		slog.InfoContext(ctx, "auto-detected ROKS cluster", "platform", platform)
		return "roks"
	}
	slog.InfoContext(ctx, "detected non-ROKS cluster", "platform", platform)
	return "unmanaged"
}
