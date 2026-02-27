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
	allowIPSpoofing := false
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

// EnsurePCIAllowedVLAN ensures the given VLAN ID is in the allowed_vlans list of a
// PCI network attachment on the bare metal server. If no PCI attachment allows the
// VLAN, it picks the first PCI attachment found (preferring secondary) and adds it.
func (c *vpcClient) EnsurePCIAllowedVLAN(ctx context.Context, bmServerID string, vlanID int64) error {
	if err := c.limiter.Acquire(ctx); err != nil {
		return err
	}
	defer c.limiter.Release()

	result, _, err := c.service.ListBareMetalServerNetworkAttachmentsWithContext(ctx, &vpcv1.ListBareMetalServerNetworkAttachmentsOptions{
		BareMetalServerID: &bmServerID,
	})
	if err != nil {
		return fmt.Errorf("VPC API ListBareMetalServerNetworkAttachments: %w", err)
	}

	// Find PCI attachments and check if any already allows this VLAN.
	type pciInfo struct {
		id           string
		isPrimary    bool
		allowedVlans []int64
	}
	var pciAttachments []pciInfo

	for _, attIntf := range result.NetworkAttachments {
		att, ok := attIntf.(*vpcv1.BareMetalServerNetworkAttachment)
		if !ok || att.InterfaceType == nil || *att.InterfaceType != "pci" {
			continue
		}
		pi := pciInfo{
			id:           derefString(att.ID),
			isPrimary:    att.Type != nil && *att.Type == "primary",
			allowedVlans: att.AllowedVlans,
		}
		// Check if VLAN already allowed
		for _, v := range pi.allowedVlans {
			if v == vlanID {
				return nil // Already allowed, nothing to do
			}
		}
		pciAttachments = append(pciAttachments, pi)
	}

	if len(pciAttachments) == 0 {
		return fmt.Errorf("no PCI network attachment found on bare metal server %s", bmServerID)
	}

	// Prefer a secondary PCI attachment over primary
	target := pciAttachments[0]
	for _, pi := range pciAttachments {
		if !pi.isPrimary {
			target = pi
			break
		}
	}

	// Build new allowed VLANs list (existing + new)
	newAllowed := append(target.allowedVlans, vlanID)

	patch := &vpcv1.BareMetalServerNetworkAttachmentPatch{
		AllowedVlans: newAllowed,
	}
	patchMap, err := patch.AsPatch()
	if err != nil {
		return fmt.Errorf("failed to build PCI attachment patch: %w", err)
	}

	_, _, err = c.service.UpdateBareMetalServerNetworkAttachmentWithContext(ctx, &vpcv1.UpdateBareMetalServerNetworkAttachmentOptions{
		BareMetalServerID:                     &bmServerID,
		ID:                                    &target.id,
		BareMetalServerNetworkAttachmentPatch: patchMap,
	})
	if err != nil {
		return fmt.Errorf("VPC API UpdateBareMetalServerNetworkAttachment (add VLAN %d): %w", vlanID, err)
	}

	return nil
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
