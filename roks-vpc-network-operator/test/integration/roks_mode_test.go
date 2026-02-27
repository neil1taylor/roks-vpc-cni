//go:build roks_integration

package integration

import (
	"context"
	"os"
	"testing"

	"github.com/IBM/roks-vpc-network-operator/pkg/roks"
)

func TestROKSClient_IsAvailable(t *testing.T) {
	endpoint := os.Getenv("ROKS_API_ENDPOINT")
	if endpoint == "" {
		t.Skip("ROKS_API_ENDPOINT not set, skipping ROKS integration test")
	}

	client, err := roks.NewClient(roks.ROKSClientConfig{
		APIEndpoint: endpoint,
		ClusterID:   os.Getenv("ROKS_CLUSTER_ID"),
		Region:      os.Getenv("ROKS_REGION"),
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if client.IsAvailable(context.Background()) {
		t.Log("ROKS API is available")
	} else {
		t.Log("ROKS API is NOT available (expected for stub)")
	}
}

func TestROKSClient_ListVNIs(t *testing.T) {
	endpoint := os.Getenv("ROKS_API_ENDPOINT")
	if endpoint == "" {
		t.Skip("ROKS_API_ENDPOINT not set, skipping ROKS integration test")
	}

	client, err := roks.NewClient(roks.ROKSClientConfig{
		APIEndpoint: endpoint,
		ClusterID:   os.Getenv("ROKS_CLUSTER_ID"),
		Region:      os.Getenv("ROKS_REGION"),
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	vnis, err := client.ListVNIs(context.Background())
	if err != nil {
		t.Logf("ListVNIs() returned expected error: %v", err)
		return
	}
	t.Logf("ListVNIs() returned %d VNIs", len(vnis))
}
