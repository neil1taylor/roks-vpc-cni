package vpc

import (
	"context"
	"fmt"

	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/vpc-go-sdk/vpcv1"
)

// CreateSubnet creates a VPC subnet for a CUDN.
func (c *vpcClient) CreateSubnet(ctx context.Context, opts CreateSubnetOptions) (*Subnet, error) {
	if err := c.limiter.Acquire(ctx); err != nil {
		return nil, err
	}
	defer c.limiter.Release()

	prototype := &vpcv1.SubnetPrototypeSubnetByCIDR{
		Name:          &opts.Name,
		VPC:           &vpcv1.VPCIdentityByID{ID: &opts.VPCID},
		Zone:          &vpcv1.ZoneIdentityByName{Name: &opts.Zone},
		Ipv4CIDRBlock: &opts.CIDR,
	}

	if opts.ACLID != "" {
		prototype.NetworkACL = &vpcv1.NetworkACLIdentityByID{ID: &opts.ACLID}
	}
	if opts.ResourceGroupID != "" {
		prototype.ResourceGroup = &vpcv1.ResourceGroupIdentityByID{ID: &opts.ResourceGroupID}
	}

	result, _, err := c.service.CreateSubnetWithContext(ctx, &vpcv1.CreateSubnetOptions{
		SubnetPrototype: prototype,
	})
	if err != nil {
		return nil, fmt.Errorf("VPC API CreateSubnet: %w", err)
	}

	// Tag the subnet for traceability and orphan GC
	if opts.ClusterID != "" || opts.CUDNName != "" {
		var tagNames []string
		if opts.ClusterID != "" {
			tagNames = append(tagNames, fmt.Sprintf("roks-cluster:%s", opts.ClusterID))
		}
		if opts.CUDNName != "" {
			tagNames = append(tagNames, fmt.Sprintf("roks-cudn:%s", opts.CUDNName))
		}
		c.tagResource(ctx, derefString(result.CRN), tagNames)
	}

	return subnetFromSDK(result), nil
}

// GetSubnet retrieves a VPC subnet by ID.
func (c *vpcClient) GetSubnet(ctx context.Context, subnetID string) (*Subnet, error) {
	if err := c.limiter.Acquire(ctx); err != nil {
		return nil, err
	}
	defer c.limiter.Release()

	result, _, err := c.service.GetSubnetWithContext(ctx, &vpcv1.GetSubnetOptions{
		ID: &subnetID,
	})
	if err != nil {
		return nil, fmt.Errorf("VPC API GetSubnet(%s): %w", subnetID, err)
	}

	return subnetFromSDK(result), nil
}

// DeleteSubnet deletes a VPC subnet.
func (c *vpcClient) DeleteSubnet(ctx context.Context, subnetID string) error {
	if err := c.limiter.Acquire(ctx); err != nil {
		return err
	}
	defer c.limiter.Release()

	_, err := c.service.DeleteSubnetWithContext(ctx, &vpcv1.DeleteSubnetOptions{
		ID: &subnetID,
	})
	if err != nil {
		return fmt.Errorf("VPC API DeleteSubnet(%s): %w", subnetID, err)
	}

	return nil
}

func subnetFromSDK(s *vpcv1.Subnet) *Subnet {
	return &Subnet{
		ID:     derefString(s.ID),
		Name:   derefString(s.Name),
		CIDR:   derefString(s.Ipv4CIDRBlock),
		Status: derefString(s.Status),
	}
}

// Suppress unused import warnings
var _ = core.StringPtr
