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

func fipFromSDK(f *vpcv1.FloatingIP) *FloatingIP {
	fip := &FloatingIP{
		ID:      derefString(f.ID),
		Name:    derefString(f.Name),
		Address: derefString(f.Address),
	}
	if f.Zone != nil {
		fip.Zone = derefString(f.Zone.Name)
	}
	if f.Target != nil {
		switch t := f.Target.(type) {
		case *vpcv1.FloatingIPTarget:
			fip.Target = derefString(t.ID)
		}
	}
	return fip
}
