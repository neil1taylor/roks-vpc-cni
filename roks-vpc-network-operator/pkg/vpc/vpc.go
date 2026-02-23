package vpc

import (
	"context"
	"fmt"

	"github.com/IBM/vpc-go-sdk/vpcv1"
)

// ListVPCs lists all VPCs in the account.
func (c *vpcClient) ListVPCs(ctx context.Context) ([]VPC, error) {
	if err := c.limiter.Acquire(ctx); err != nil {
		return nil, err
	}
	defer c.limiter.Release()

	var allVPCs []VPC
	var start *string

	for {
		listOpts := &vpcv1.ListVpcsOptions{}
		if start != nil {
			listOpts.Start = start
		}

		result, _, err := c.service.ListVpcsWithContext(ctx, listOpts)
		if err != nil {
			return nil, fmt.Errorf("VPC API ListVPCs: %w", err)
		}

		for i := range result.Vpcs {
			allVPCs = append(allVPCs, *vpcFromSDK(&result.Vpcs[i]))
		}

		if result.Next == nil || result.Next.Href == nil {
			break
		}
		start = result.Next.Href
	}

	return allVPCs, nil
}

// GetVPC retrieves a single VPC by ID.
func (c *vpcClient) GetVPC(ctx context.Context, vpcID string) (*VPC, error) {
	if err := c.limiter.Acquire(ctx); err != nil {
		return nil, err
	}
	defer c.limiter.Release()

	result, _, err := c.service.GetVPCWithContext(ctx, &vpcv1.GetVPCOptions{
		ID: &vpcID,
	})
	if err != nil {
		return nil, fmt.Errorf("VPC API GetVPC(%s): %w", vpcID, err)
	}

	return vpcFromSDK(result), nil
}

// ListSubnets lists all subnets, optionally filtered by VPC.
func (c *vpcClient) ListSubnets(ctx context.Context, vpcID string) ([]Subnet, error) {
	if err := c.limiter.Acquire(ctx); err != nil {
		return nil, err
	}
	defer c.limiter.Release()

	var allSubnets []Subnet
	var start *string

	for {
		listOpts := &vpcv1.ListSubnetsOptions{}
		if start != nil {
			listOpts.Start = start
		}

		result, _, err := c.service.ListSubnetsWithContext(ctx, listOpts)
		if err != nil {
			return nil, fmt.Errorf("VPC API ListSubnets: %w", err)
		}

		for i := range result.Subnets {
			s := &result.Subnets[i]
			// Filter by VPC if provided
			if vpcID != "" && s.VPC != nil && derefString(s.VPC.ID) != vpcID {
				continue
			}
			allSubnets = append(allSubnets, *subnetFromSDK(s))
		}

		if result.Next == nil || result.Next.Href == nil {
			break
		}
		start = result.Next.Href
	}

	return allSubnets, nil
}

func vpcFromSDK(v *vpcv1.VPC) *VPC {
	vpc := &VPC{
		ID:     derefString(v.ID),
		Name:   derefString(v.Name),
		Status: derefString(v.Status),
	}
	if v.DefaultSecurityGroup != nil {
		vpc.DefaultSGID = derefString(v.DefaultSecurityGroup.ID)
	}
	if v.DefaultNetworkACL != nil {
		vpc.DefaultACLID = derefString(v.DefaultNetworkACL.ID)
	}
	if v.ResourceGroup != nil {
		vpc.ResourceGroupID = derefString(v.ResourceGroup.ID)
	}
	if v.CreatedAt != nil {
		vpc.CreatedAt = v.CreatedAt.String()
	}
	return vpc
}
