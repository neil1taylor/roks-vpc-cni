package vpc

import (
	"context"
	"fmt"
)

// CreateFlowLogCollectorOptions holds parameters for creating a VPC flow log collector.
type CreateFlowLogCollectorOptions struct {
	Name           string
	TargetSubnetID string
	COSBucketCRN   string
	IsActive       bool
	ClusterID      string // for tagging
	OwnerKind      string // for tagging: e.g. "vpcsubnet"
	OwnerName      string // for tagging: K8s object name
}

// FlowLogCollector represents a VPC flow log collector resource.
type FlowLogCollector struct {
	ID             string
	Name           string
	TargetSubnetID string
	COSBucketCRN   string
	IsActive       bool
	LifecycleState string
}

// CreateFlowLogCollector creates a VPC flow log collector targeting a subnet.
// Stub implementation — will be wired to the VPC SDK flow log API later.
func (c *vpcClient) CreateFlowLogCollector(ctx context.Context, opts CreateFlowLogCollectorOptions) (*FlowLogCollector, error) {
	if err := c.limiter.Acquire(ctx); err != nil {
		return nil, err
	}
	defer c.limiter.Release()

	// TODO: Wire to VPC SDK vpcv1.CreateFlowLogCollectorOptions
	return nil, fmt.Errorf("CreateFlowLogCollector not yet implemented")
}

// DeleteFlowLogCollector deletes a VPC flow log collector by ID.
// Stub implementation — will be wired to the VPC SDK flow log API later.
func (c *vpcClient) DeleteFlowLogCollector(ctx context.Context, id string) error {
	if err := c.limiter.Acquire(ctx); err != nil {
		return err
	}
	defer c.limiter.Release()

	// TODO: Wire to VPC SDK vpcv1.DeleteFlowLogCollectorOptions
	return fmt.Errorf("DeleteFlowLogCollector not yet implemented")
}

// ListFlowLogCollectors lists all VPC flow log collectors in the account.
// Stub implementation — will be wired to the VPC SDK flow log API later.
func (c *vpcClient) ListFlowLogCollectors(ctx context.Context) ([]FlowLogCollector, error) {
	if err := c.limiter.Acquire(ctx); err != nil {
		return nil, err
	}
	defer c.limiter.Release()

	// TODO: Wire to VPC SDK vpcv1.ListFlowLogCollectorsOptions
	return nil, fmt.Errorf("ListFlowLogCollectors not yet implemented")
}

// GetFlowLogCollector retrieves a VPC flow log collector by ID.
// Stub implementation — will be wired to the VPC SDK flow log API later.
func (c *vpcClient) GetFlowLogCollector(ctx context.Context, id string) (*FlowLogCollector, error) {
	if err := c.limiter.Acquire(ctx); err != nil {
		return nil, err
	}
	defer c.limiter.Release()

	// TODO: Wire to VPC SDK vpcv1.GetFlowLogCollectorOptions
	return nil, fmt.Errorf("GetFlowLogCollector not yet implemented")
}
