package vpc

import (
	"context"
	"fmt"

	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/vpc-go-sdk/vpcv1"
)

// CreateVLANAttachment creates a VLAN interface on a bare metal server's PCI interface.
func (c *vpcClient) CreateVLANAttachment(ctx context.Context, opts CreateVLANAttachmentOptions) (*VLANAttachment, error) {
	if err := c.limiter.Acquire(ctx); err != nil {
		return nil, err
	}
	defer c.limiter.Release()

	vlanID := opts.VLANID
	allowToFloat := true
	allowIPSpoofing := true
	enableInfraNat := false

	createOpts := &vpcv1.CreateBareMetalServerNetworkAttachmentOptions{
		BareMetalServerID: &opts.BMServerID,
		BareMetalServerNetworkAttachmentPrototype: &vpcv1.BareMetalServerNetworkAttachmentPrototypeBareMetalServerNetworkAttachmentByVlanPrototype{
			Name:          &opts.Name,
			InterfaceType: core.StringPtr("vlan"),
			Vlan:          &vlanID,
			AllowToFloat:  &allowToFloat,
			VirtualNetworkInterface: &vpcv1.BareMetalServerNetworkAttachmentPrototypeVirtualNetworkInterface{
				Subnet:                  &vpcv1.SubnetIdentityByID{ID: &opts.SubnetID},
				AllowIPSpoofing:         &allowIPSpoofing,
				EnableInfrastructureNat: &enableInfraNat,
			},
		},
	}

	result, _, err := c.service.CreateBareMetalServerNetworkAttachmentWithContext(ctx, createOpts)
	if err != nil {
		return nil, fmt.Errorf("VPC API CreateVLANAttachment: %w", err)
	}

	return vlanAttachmentFromSDKIntf(result, opts.BMServerID), nil
}

// DeleteVLANAttachment removes a VLAN interface from a bare metal server.
func (c *vpcClient) DeleteVLANAttachment(ctx context.Context, bmServerID, attachmentID string) error {
	if err := c.limiter.Acquire(ctx); err != nil {
		return err
	}
	defer c.limiter.Release()

	_, err := c.service.DeleteBareMetalServerNetworkAttachmentWithContext(ctx, &vpcv1.DeleteBareMetalServerNetworkAttachmentOptions{
		BareMetalServerID: &bmServerID,
		ID:                &attachmentID,
	})
	if err != nil {
		return fmt.Errorf("VPC API DeleteVLANAttachment(%s/%s): %w", bmServerID, attachmentID, err)
	}

	return nil
}

// ListVLANAttachments lists all network attachments on a bare metal server.
func (c *vpcClient) ListVLANAttachments(ctx context.Context, bmServerID string) ([]VLANAttachment, error) {
	if err := c.limiter.Acquire(ctx); err != nil {
		return nil, err
	}
	defer c.limiter.Release()

	result, _, err := c.service.ListBareMetalServerNetworkAttachmentsWithContext(ctx, &vpcv1.ListBareMetalServerNetworkAttachmentsOptions{
		BareMetalServerID: &bmServerID,
	})
	if err != nil {
		return nil, fmt.Errorf("VPC API ListVLANAttachments(%s): %w", bmServerID, err)
	}

	var attachments []VLANAttachment
	for _, attIntf := range result.NetworkAttachments {
		att := vlanAttachmentFromSDKIntf(attIntf, bmServerID)
		if att != nil {
			attachments = append(attachments, *att)
		}
	}

	return attachments, nil
}

func vlanAttachmentFromSDKIntf(attIntf vpcv1.BareMetalServerNetworkAttachmentIntf, bmServerID string) *VLANAttachment {
	if attIntf == nil {
		return nil
	}

	switch att := attIntf.(type) {
	case *vpcv1.BareMetalServerNetworkAttachment:
		// Only include VLAN types
		if att.InterfaceType != nil && *att.InterfaceType != "vlan" {
			return nil
		}
		va := &VLANAttachment{
			ID:         derefString(att.ID),
			Name:       derefString(att.Name),
			BMServerID: bmServerID,
		}
		if att.Vlan != nil {
			va.VLANID = *att.Vlan
		}
		return va
	}

	return nil
}
