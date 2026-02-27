package vpc

import (
	"context"
	"fmt"

	"github.com/IBM/vpc-go-sdk/vpcv1"
)

// CreateFloatingIP creates a floating IP and optionally attaches it to a VNI.
func (c *vpcClient) CreateFloatingIP(ctx context.Context, opts CreateFloatingIPOptions) (*FloatingIP, error) {
	if err := c.limiter.Acquire(ctx); err != nil {
		return nil, err
	}
	defer c.limiter.Release()

	var prototype vpcv1.FloatingIPPrototypeIntf

	if opts.VNIID != "" {
		// Create FIP bound to a VNI target
		prototype = &vpcv1.FloatingIPPrototypeFloatingIPByTarget{
			Name: &opts.Name,
			Target: &vpcv1.FloatingIPTargetPrototypeVirtualNetworkInterfaceIdentity{
				ID: &opts.VNIID,
			},
		}
	} else {
		// Create unbound FIP in a zone
		prototype = &vpcv1.FloatingIPPrototypeFloatingIPByZone{
			Name: &opts.Name,
			Zone: &vpcv1.ZoneIdentityByName{Name: &opts.Zone},
		}
	}

	result, _, err := c.service.CreateFloatingIPWithContext(ctx, &vpcv1.CreateFloatingIPOptions{
		FloatingIPPrototype: prototype,
	})
	if err != nil {
		return nil, fmt.Errorf("VPC API CreateFloatingIP: %w", err)
	}

	return fipFromSDK(result), nil
}

// GetFloatingIP retrieves a floating IP by ID.
func (c *vpcClient) GetFloatingIP(ctx context.Context, fipID string) (*FloatingIP, error) {
	if err := c.limiter.Acquire(ctx); err != nil {
		return nil, err
	}
	defer c.limiter.Release()

	result, _, err := c.service.GetFloatingIPWithContext(ctx, &vpcv1.GetFloatingIPOptions{
		ID: &fipID,
	})
	if err != nil {
		return nil, fmt.Errorf("VPC API GetFloatingIP(%s): %w", fipID, err)
	}

	return fipFromSDK(result), nil
}

// DeleteFloatingIP releases and deletes a floating IP.
func (c *vpcClient) DeleteFloatingIP(ctx context.Context, fipID string) error {
	if err := c.limiter.Acquire(ctx); err != nil {
		return err
	}
	defer c.limiter.Release()

	_, err := c.service.DeleteFloatingIPWithContext(ctx, &vpcv1.DeleteFloatingIPOptions{
		ID: &fipID,
	})
	if err != nil {
		return fmt.Errorf("VPC API DeleteFloatingIP(%s): %w", fipID, err)
	}

	return nil
}

// ListFloatingIPs lists all floating IPs in the account. Used by the BFF for the Floating IPs tab.
func (c *vpcClient) ListFloatingIPs(ctx context.Context) ([]FloatingIP, error) {
	if err := c.limiter.Acquire(ctx); err != nil {
		return nil, err
	}
	defer c.limiter.Release()

	var allFIPs []FloatingIP
	var start *string

	for {
		listOpts := &vpcv1.ListFloatingIpsOptions{}
		if start != nil {
			listOpts.Start = start
		}

		result, _, err := c.service.ListFloatingIpsWithContext(ctx, listOpts)
		if err != nil {
			return nil, fmt.Errorf("VPC API ListFloatingIPs: %w", err)
		}

		for i := range result.FloatingIps {
			allFIPs = append(allFIPs, *fipFromSDK(&result.FloatingIps[i]))
		}

		if result.Next == nil || result.Next.Href == nil {
			break
		}
		start = result.Next.Href
	}

	return allFIPs, nil
}

func fipFromSDK(f *vpcv1.FloatingIP) *FloatingIP {
	fip := &FloatingIP{
		ID:      derefString(f.ID),
		Name:    derefString(f.Name),
		Address: derefString(f.Address),
		Status:  derefString(f.Status),
	}
	if f.Zone != nil {
		fip.Zone = derefString(f.Zone.Name)
	}
	if f.Target != nil {
		switch t := f.Target.(type) {
		case *vpcv1.FloatingIPTarget:
			fip.Target = derefString(t.ID)
			fip.TargetName = derefString(t.Name)
		}
	}
	if f.CreatedAt != nil {
		fip.CreatedAt = f.CreatedAt.String()
	}
	return fip
}
