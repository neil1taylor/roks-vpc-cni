//go:build integration

package integration

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/IBM/roks-vpc-network-operator/pkg/vpc"
)

// TestVPCSubnetLifecycle creates, gets, and deletes a VPC subnet.
// Requires IBMCLOUD_API_KEY, VPC_REGION, VPC_ID, VPC_ZONE env vars.
func TestVPCSubnetLifecycle(t *testing.T) {
	client := newIntegrationClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	subnetName := fmt.Sprintf("integration-test-%d", time.Now().UnixNano())

	// Create
	subnet, err := client.CreateSubnet(ctx, vpc.CreateSubnetOptions{
		Name:      subnetName,
		VPCID:     requireEnv(t, "VPC_ID"),
		Zone:      requireEnv(t, "VPC_ZONE"),
		CIDR:      "10.240.128.0/24",
		ClusterID: "integration-test",
		CUDNName:  "test-cudn",
	})
	if err != nil {
		t.Fatalf("CreateSubnet failed: %v", err)
	}
	t.Logf("Created subnet: %s (%s)", subnet.ID, subnet.Name)

	// Cleanup on exit
	defer func() {
		t.Log("Cleaning up subnet...")
		if err := client.DeleteSubnet(ctx, subnet.ID); err != nil {
			t.Errorf("DeleteSubnet cleanup failed: %v", err)
		}
	}()

	// Get
	got, err := client.GetSubnet(ctx, subnet.ID)
	if err != nil {
		t.Fatalf("GetSubnet failed: %v", err)
	}
	if got.ID != subnet.ID {
		t.Errorf("GetSubnet ID mismatch: got %s, want %s", got.ID, subnet.ID)
	}
	if got.Name != subnetName {
		t.Errorf("GetSubnet name mismatch: got %s, want %s", got.Name, subnetName)
	}
	t.Logf("Verified subnet exists: %s", got.ID)
}

// TestVNILifecycle creates, gets, and deletes a VNI.
// Requires a subnet to already exist. Set VPC_SUBNET_ID env var.
func TestVNILifecycle(t *testing.T) {
	client := newIntegrationClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	subnetID := requireEnv(t, "VPC_SUBNET_ID")

	// Create
	vni, err := client.CreateVNI(ctx, vpc.CreateVNIOptions{
		Name:      fmt.Sprintf("integration-test-vni-%d", time.Now().UnixNano()),
		SubnetID:  subnetID,
		ClusterID: "integration-test",
		Namespace: "default",
		VMName:    "test-vm",
	})
	if err != nil {
		t.Fatalf("CreateVNI failed: %v", err)
	}
	t.Logf("Created VNI: %s (MAC: %s, IP: %s)", vni.ID, vni.MACAddress, vni.PrimaryIP.Address)

	defer func() {
		t.Log("Cleaning up VNI...")
		if err := client.DeleteVNI(ctx, vni.ID); err != nil {
			t.Errorf("DeleteVNI cleanup failed: %v", err)
		}
	}()

	// Get
	got, err := client.GetVNI(ctx, vni.ID)
	if err != nil {
		t.Fatalf("GetVNI failed: %v", err)
	}
	if got.MACAddress == "" {
		t.Error("VNI missing MAC address")
	}
	if got.PrimaryIP.Address == "" {
		t.Error("VNI missing primary IP")
	}
	t.Logf("Verified VNI exists: %s", got.ID)
}

// TestFloatingIPLifecycle creates and deletes a floating IP.
func TestFloatingIPLifecycle(t *testing.T) {
	client := newIntegrationClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	zone := requireEnv(t, "VPC_ZONE")

	fip, err := client.CreateFloatingIP(ctx, vpc.CreateFloatingIPOptions{
		Name: fmt.Sprintf("integration-test-fip-%d", time.Now().UnixNano()),
		Zone: zone,
	})
	if err != nil {
		t.Fatalf("CreateFloatingIP failed: %v", err)
	}
	t.Logf("Created FIP: %s (%s)", fip.ID, fip.Address)

	defer func() {
		t.Log("Cleaning up FIP...")
		if err := client.DeleteFloatingIP(ctx, fip.ID); err != nil {
			t.Errorf("DeleteFloatingIP cleanup failed: %v", err)
		}
	}()

	got, err := client.GetFloatingIP(ctx, fip.ID)
	if err != nil {
		t.Fatalf("GetFloatingIP failed: %v", err)
	}
	if got.Address == "" {
		t.Error("FIP missing address")
	}
	t.Logf("Verified FIP exists: %s (%s)", got.ID, got.Address)
}

func newIntegrationClient(t *testing.T) vpc.Client {
	t.Helper()

	apiKey := requireEnv(t, "IBMCLOUD_API_KEY")
	region := requireEnv(t, "VPC_REGION")

	client, err := vpc.NewClient(vpc.ClientConfig{
		APIKey:        apiKey,
		Region:        region,
		ClusterID:     "integration-test",
		MaxConcurrent: 5,
	})
	if err != nil {
		t.Fatalf("Failed to create VPC client: %v", err)
	}
	return client
}

func requireEnv(t *testing.T, key string) string {
	t.Helper()
	val := os.Getenv(key)
	if val == "" {
		t.Skipf("Skipping: %s not set", key)
	}
	return val
}
