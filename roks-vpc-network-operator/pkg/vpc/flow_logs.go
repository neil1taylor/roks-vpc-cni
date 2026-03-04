package vpc

import (
	"context"
	"fmt"

	"github.com/IBM/vpc-go-sdk/vpcv1"
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
func (c *vpcClient) CreateFlowLogCollector(ctx context.Context, opts CreateFlowLogCollectorOptions) (*FlowLogCollector, error) {
	if err := c.limiter.Acquire(ctx); err != nil {
		return nil, err
	}
	defer c.limiter.Release()

	sdkOpts := &vpcv1.CreateFlowLogCollectorOptions{
		StorageBucket: &vpcv1.LegacyCloudObjectStorageBucketIdentityCloudObjectStorageBucketIdentityByName{
			Name: &opts.COSBucketCRN,
		},
		Target: &vpcv1.FlowLogCollectorTargetPrototypeSubnetIdentity{
			ID: &opts.TargetSubnetID,
		},
		Name:   &opts.Name,
		Active: &opts.IsActive,
	}

	result, _, err := c.service.CreateFlowLogCollectorWithContext(ctx, sdkOpts)
	if err != nil {
		return nil, fmt.Errorf("VPC API CreateFlowLogCollector: %w", err)
	}

	// Tag the flow log collector for traceability and orphan GC
	if opts.ClusterID != "" || opts.OwnerKind != "" {
		c.tagResource(ctx, derefString(result.CRN), BuildTags(opts.ClusterID, "flowlog", opts.OwnerKind, opts.OwnerName))
	}

	return flowLogFromSDK(result), nil
}

// GetFlowLogCollector retrieves a VPC flow log collector by ID.
func (c *vpcClient) GetFlowLogCollector(ctx context.Context, id string) (*FlowLogCollector, error) {
	if err := c.limiter.Acquire(ctx); err != nil {
		return nil, err
	}
	defer c.limiter.Release()

	result, _, err := c.service.GetFlowLogCollectorWithContext(ctx, &vpcv1.GetFlowLogCollectorOptions{
		ID: &id,
	})
	if err != nil {
		return nil, fmt.Errorf("VPC API GetFlowLogCollector(%s): %w", id, err)
	}

	return flowLogFromSDK(result), nil
}

// DeleteFlowLogCollector deletes a VPC flow log collector by ID.
func (c *vpcClient) DeleteFlowLogCollector(ctx context.Context, id string) error {
	if err := c.limiter.Acquire(ctx); err != nil {
		return err
	}
	defer c.limiter.Release()

	_, err := c.service.DeleteFlowLogCollectorWithContext(ctx, &vpcv1.DeleteFlowLogCollectorOptions{
		ID: &id,
	})
	if err != nil {
		return fmt.Errorf("VPC API DeleteFlowLogCollector(%s): %w", id, err)
	}

	return nil
}

// ListFlowLogCollectors lists all VPC flow log collectors in the account.
func (c *vpcClient) ListFlowLogCollectors(ctx context.Context) ([]FlowLogCollector, error) {
	if err := c.limiter.Acquire(ctx); err != nil {
		return nil, err
	}
	defer c.limiter.Release()

	var allCollectors []FlowLogCollector
	var start *string

	for {
		listOpts := &vpcv1.ListFlowLogCollectorsOptions{}
		if start != nil {
			listOpts.Start = start
		}

		result, _, err := c.service.ListFlowLogCollectorsWithContext(ctx, listOpts)
		if err != nil {
			return nil, fmt.Errorf("VPC API ListFlowLogCollectors: %w", err)
		}

		for i := range result.FlowLogCollectors {
			allCollectors = append(allCollectors, *flowLogFromSDK(&result.FlowLogCollectors[i]))
		}

		next, err := result.GetNextStart()
		if err != nil || next == nil {
			break
		}
		start = next
	}

	return allCollectors, nil
}

// flowLogFromSDK converts an SDK FlowLogCollector to our domain type.
func flowLogFromSDK(f *vpcv1.FlowLogCollector) *FlowLogCollector {
	if f == nil {
		return nil
	}
	flc := &FlowLogCollector{
		ID:             derefString(f.ID),
		Name:           derefString(f.Name),
		IsActive:       f.Active != nil && *f.Active,
		LifecycleState: derefString(f.LifecycleState),
	}
	if f.Target != nil {
		switch t := f.Target.(type) {
		case *vpcv1.FlowLogCollectorTarget:
			flc.TargetSubnetID = derefString(t.ID)
		}
	}
	if f.StorageBucket != nil {
		flc.COSBucketCRN = derefString(f.StorageBucket.Name)
	}
	return flc
}
