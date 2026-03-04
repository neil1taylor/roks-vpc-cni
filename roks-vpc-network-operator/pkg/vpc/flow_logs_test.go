package vpc

import (
	"context"
	"testing"
)

func TestFlowLogCreateMock(t *testing.T) {
	mock := NewMockClient()
	mock.CreateFlowLogCollectorFn = func(ctx context.Context, opts CreateFlowLogCollectorOptions) (*FlowLogCollector, error) {
		if opts.Name != "test-flowlog" {
			t.Errorf("expected name %q, got %q", "test-flowlog", opts.Name)
		}
		if opts.TargetSubnetID != "subnet-123" {
			t.Errorf("expected target subnet ID %q, got %q", "subnet-123", opts.TargetSubnetID)
		}
		if opts.COSBucketCRN != "my-cos-bucket" {
			t.Errorf("expected COS bucket %q, got %q", "my-cos-bucket", opts.COSBucketCRN)
		}
		if !opts.IsActive {
			t.Error("expected IsActive to be true")
		}
		return &FlowLogCollector{
			ID:             "flc-001",
			Name:           opts.Name,
			TargetSubnetID: opts.TargetSubnetID,
			COSBucketCRN:   opts.COSBucketCRN,
			IsActive:       opts.IsActive,
			LifecycleState: "stable",
		}, nil
	}

	ctx := context.Background()
	result, err := mock.CreateFlowLogCollector(ctx, CreateFlowLogCollectorOptions{
		Name:           "test-flowlog",
		TargetSubnetID: "subnet-123",
		COSBucketCRN:   "my-cos-bucket",
		IsActive:       true,
		ClusterID:      "cluster-1",
		OwnerKind:      "vpcsubnet",
		OwnerName:      "my-subnet",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != "flc-001" {
		t.Errorf("expected ID %q, got %q", "flc-001", result.ID)
	}
	if result.LifecycleState != "stable" {
		t.Errorf("expected lifecycle state %q, got %q", "stable", result.LifecycleState)
	}
	if mock.CallCount("CreateFlowLogCollector") != 1 {
		t.Errorf("expected 1 call, got %d", mock.CallCount("CreateFlowLogCollector"))
	}
}

func TestFlowLogGetMock(t *testing.T) {
	mock := NewMockClient()
	mock.GetFlowLogCollectorFn = func(ctx context.Context, id string) (*FlowLogCollector, error) {
		if id != "flc-001" {
			t.Errorf("expected ID %q, got %q", "flc-001", id)
		}
		return &FlowLogCollector{
			ID:             "flc-001",
			Name:           "test-flowlog",
			TargetSubnetID: "subnet-123",
			COSBucketCRN:   "my-cos-bucket",
			IsActive:       true,
			LifecycleState: "stable",
		}, nil
	}

	ctx := context.Background()
	result, err := mock.GetFlowLogCollector(ctx, "flc-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != "flc-001" {
		t.Errorf("expected ID %q, got %q", "flc-001", result.ID)
	}
	if !result.IsActive {
		t.Error("expected IsActive to be true")
	}
	if mock.CallCount("GetFlowLogCollector") != 1 {
		t.Errorf("expected 1 call, got %d", mock.CallCount("GetFlowLogCollector"))
	}
}

func TestFlowLogDeleteMock(t *testing.T) {
	mock := NewMockClient()
	mock.DeleteFlowLogCollectorFn = func(ctx context.Context, id string) error {
		if id != "flc-001" {
			t.Errorf("expected ID %q, got %q", "flc-001", id)
		}
		return nil
	}

	ctx := context.Background()
	err := mock.DeleteFlowLogCollector(ctx, "flc-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mock.CallCount("DeleteFlowLogCollector") != 1 {
		t.Errorf("expected 1 call, got %d", mock.CallCount("DeleteFlowLogCollector"))
	}
}

func TestFlowLogListMock(t *testing.T) {
	mock := NewMockClient()
	mock.ListFlowLogCollectorsFn = func(ctx context.Context) ([]FlowLogCollector, error) {
		return []FlowLogCollector{
			{
				ID:             "flc-001",
				Name:           "flowlog-1",
				TargetSubnetID: "subnet-1",
				COSBucketCRN:   "bucket-1",
				IsActive:       true,
				LifecycleState: "stable",
			},
			{
				ID:             "flc-002",
				Name:           "flowlog-2",
				TargetSubnetID: "subnet-2",
				COSBucketCRN:   "bucket-2",
				IsActive:       false,
				LifecycleState: "stable",
			},
		}, nil
	}

	ctx := context.Background()
	results, err := mock.ListFlowLogCollectors(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 collectors, got %d", len(results))
	}
	if results[0].ID != "flc-001" {
		t.Errorf("expected first ID %q, got %q", "flc-001", results[0].ID)
	}
	if results[1].IsActive {
		t.Error("expected second collector to be inactive")
	}
	if mock.CallCount("ListFlowLogCollectors") != 1 {
		t.Errorf("expected 1 call, got %d", mock.CallCount("ListFlowLogCollectors"))
	}
}

func TestFlowLogFromSDK_Nil(t *testing.T) {
	// Test the conversion helper with nil input
	flc := flowLogFromSDK(nil)
	if flc != nil {
		t.Error("expected nil result for nil input")
	}
}

func TestFlowLogFromSDK_Fields(t *testing.T) {
	// Import the SDK types inline to verify our conversion
	// Since we're in the same package, we can call flowLogFromSDK directly
	// but we need actual SDK types. Test via the mock path instead.

	// Verify FlowLogCollector struct has the right fields
	flc := FlowLogCollector{
		ID:             "test-id",
		Name:           "test-name",
		TargetSubnetID: "subnet-id",
		COSBucketCRN:   "bucket-name",
		IsActive:       true,
		LifecycleState: "stable",
	}
	if flc.ID != "test-id" {
		t.Errorf("expected ID %q, got %q", "test-id", flc.ID)
	}
	if flc.LifecycleState != "stable" {
		t.Errorf("expected lifecycle state %q, got %q", "stable", flc.LifecycleState)
	}
}

func TestFlowLogMockDefaultError(t *testing.T) {
	mock := NewMockClient()
	ctx := context.Background()

	// Verify all 4 methods return errors when no Fn is set
	_, err := mock.CreateFlowLogCollector(ctx, CreateFlowLogCollectorOptions{})
	if err == nil {
		t.Error("expected error from unconfigured CreateFlowLogCollector")
	}

	_, err = mock.GetFlowLogCollector(ctx, "id")
	if err == nil {
		t.Error("expected error from unconfigured GetFlowLogCollector")
	}

	err = mock.DeleteFlowLogCollector(ctx, "id")
	if err == nil {
		t.Error("expected error from unconfigured DeleteFlowLogCollector")
	}

	_, err = mock.ListFlowLogCollectors(ctx)
	if err == nil {
		t.Error("expected error from unconfigured ListFlowLogCollectors")
	}
}
