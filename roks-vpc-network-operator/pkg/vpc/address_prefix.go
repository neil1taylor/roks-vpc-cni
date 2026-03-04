package vpc

import (
	"context"
	"fmt"

	"github.com/IBM/vpc-go-sdk/vpcv1"
)

// ListVPCAddressPrefixes lists all address prefixes for a VPC.
func (c *vpcClient) ListVPCAddressPrefixes(ctx context.Context, vpcID string) ([]AddressPrefix, error) {
	if err := c.limiter.Acquire(ctx); err != nil {
		return nil, err
	}
	defer c.limiter.Release()

	var allPrefixes []AddressPrefix
	var start *string

	for {
		listOpts := &vpcv1.ListVPCAddressPrefixesOptions{
			VPCID: &vpcID,
		}
		if start != nil {
			listOpts.Start = start
		}

		result, _, err := c.service.ListVPCAddressPrefixesWithContext(ctx, listOpts)
		if err != nil {
			return nil, fmt.Errorf("VPC API ListVPCAddressPrefixes(%s): %w", vpcID, err)
		}

		for i := range result.AddressPrefixes {
			ap := &result.AddressPrefixes[i]
			allPrefixes = append(allPrefixes, AddressPrefix{
				ID:        derefString(ap.ID),
				Name:      derefString(ap.Name),
				CIDR:      derefString(ap.CIDR),
				Zone:      zoneNameFromRef(ap.Zone),
				IsDefault: ap.IsDefault != nil && *ap.IsDefault,
			})
		}

		if result.Next == nil || result.Next.Href == nil {
			break
		}
		start = result.Next.Href
	}

	return allPrefixes, nil
}

// CreateVPCAddressPrefix creates a new address prefix in a VPC.
func (c *vpcClient) CreateVPCAddressPrefix(ctx context.Context, opts CreateAddressPrefixOptions) (*AddressPrefix, error) {
	if err := c.limiter.Acquire(ctx); err != nil {
		return nil, err
	}
	defer c.limiter.Release()

	createOpts := &vpcv1.CreateVPCAddressPrefixOptions{
		VPCID: &opts.VPCID,
		CIDR:  &opts.CIDR,
		Zone:  &vpcv1.ZoneIdentityByName{Name: &opts.Zone},
	}
	if opts.Name != "" {
		createOpts.Name = &opts.Name
	}

	result, _, err := c.service.CreateVPCAddressPrefixWithContext(ctx, createOpts)
	if err != nil {
		return nil, fmt.Errorf("VPC API CreateVPCAddressPrefix(%s, %s): %w", opts.VPCID, opts.CIDR, err)
	}

	// Note: VPC address prefixes don't expose CRN, so tagging is not possible.
	return &AddressPrefix{
		ID:        derefString(result.ID),
		Name:      derefString(result.Name),
		CIDR:      derefString(result.CIDR),
		Zone:      zoneNameFromRef(result.Zone),
		IsDefault: result.IsDefault != nil && *result.IsDefault,
	}, nil
}

// zoneNameFromRef extracts the zone name from a VPC SDK ZoneReference.
func zoneNameFromRef(z *vpcv1.ZoneReference) string {
	if z == nil {
		return ""
	}
	return derefString(z.Name)
}
