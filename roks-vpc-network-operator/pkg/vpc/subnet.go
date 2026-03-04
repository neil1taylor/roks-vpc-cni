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
	if opts.PublicGatewayID != "" {
		prototype.PublicGateway = &vpcv1.PublicGatewayIdentityPublicGatewayIdentityByID{
			ID: &opts.PublicGatewayID,
		}
	}

	result, _, err := c.service.CreateSubnetWithContext(ctx, &vpcv1.CreateSubnetOptions{
		SubnetPrototype: prototype,
	})
	if err != nil {
		return nil, fmt.Errorf("VPC API CreateSubnet: %w", err)
	}

	// Tag the subnet for traceability and orphan GC
	ownerKind := opts.OwnerKind
	ownerName := opts.OwnerName
	if ownerKind == "" && opts.CUDNName != "" {
		ownerKind = "cudn"
		ownerName = opts.CUDNName
	}
	if opts.ClusterID != "" || ownerKind != "" {
		c.tagResource(ctx, derefString(result.CRN), BuildTags(opts.ClusterID, "subnet", ownerKind, ownerName))
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

// ListSubnetReservedIPs lists all reserved IPs in a subnet.
func (c *vpcClient) ListSubnetReservedIPs(ctx context.Context, subnetID string) ([]ReservedIP, error) {
	if err := c.limiter.Acquire(ctx); err != nil {
		return nil, err
	}
	defer c.limiter.Release()

	var allIPs []ReservedIP
	var start *string

	for {
		listOpts := &vpcv1.ListSubnetReservedIpsOptions{
			SubnetID: &subnetID,
		}
		if start != nil {
			listOpts.Start = start
		}

		result, _, err := c.service.ListSubnetReservedIpsWithContext(ctx, listOpts)
		if err != nil {
			return nil, fmt.Errorf("VPC API ListSubnetReservedIps(%s): %w", subnetID, err)
		}

		for i := range result.ReservedIps {
			rip := &result.ReservedIps[i]
			ip := ReservedIP{
				ID:         derefString(rip.ID),
				Address:    derefString(rip.Address),
				Name:       derefString(rip.Name),
				AutoDelete: rip.AutoDelete != nil && *rip.AutoDelete,
				Owner:      derefString(rip.Owner),
			}
			if rip.CreatedAt != nil {
				ip.CreatedAt = rip.CreatedAt.String()
			}
			if rip.Target != nil {
				switch t := rip.Target.(type) {
				case *vpcv1.ReservedIPTarget:
					ip.TargetID = derefString(t.ID)
					ip.Target = derefString(t.Name)
				}
			}
			allIPs = append(allIPs, ip)
		}

		if result.Next == nil || result.Next.Href == nil {
			break
		}
		start = result.Next.Href
	}

	return allIPs, nil
}

func subnetFromSDK(s *vpcv1.Subnet) *Subnet {
	sub := &Subnet{
		ID:                        derefString(s.ID),
		Name:                      derefString(s.Name),
		CIDR:                      derefString(s.Ipv4CIDRBlock),
		Status:                    derefString(s.Status),
		AvailableIPv4AddressCount: derefInt64(s.AvailableIpv4AddressCount),
		TotalIPv4AddressCount:     derefInt64(s.TotalIpv4AddressCount),
	}
	if s.VPC != nil {
		sub.VPCID = derefString(s.VPC.ID)
		sub.VPCName = derefString(s.VPC.Name)
	}
	if s.Zone != nil {
		sub.Zone = derefString(s.Zone.Name)
	}
	if s.NetworkACL != nil {
		sub.NetworkACLID = derefString(s.NetworkACL.ID)
		sub.NetworkACLName = derefString(s.NetworkACL.Name)
	}
	if s.CreatedAt != nil {
		sub.CreatedAt = s.CreatedAt.String()
	}
	return sub
}

// Suppress unused import warnings
var _ = core.StringPtr
