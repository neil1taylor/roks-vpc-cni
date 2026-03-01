package vpc

import (
	"context"
	"fmt"

	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/vpc-go-sdk/vpcv1"
)

// CreateVNI creates a floating Virtual Network Interface.
// CRITICAL settings:
//   - auto_delete: false
//   - allow_ip_spoofing: true
//   - enable_infrastructure_nat: configurable (default true; gateway VNIs use false)
func (c *vpcClient) CreateVNI(ctx context.Context, opts CreateVNIOptions) (*VNI, error) {
	if err := c.limiter.Acquire(ctx); err != nil {
		return nil, err
	}
	defer c.limiter.Release()

	enableInfraNat := true
	if opts.EnableInfrastructureNat != nil {
		enableInfraNat = *opts.EnableInfrastructureNat
	}

	createOpts := &vpcv1.CreateVirtualNetworkInterfaceOptions{
		Name:                    &opts.Name,
		Subnet:                  &vpcv1.SubnetIdentityByID{ID: &opts.SubnetID},
		AllowIPSpoofing:         core.BoolPtr(true),
		EnableInfrastructureNat: core.BoolPtr(enableInfraNat),
		AutoDelete:              core.BoolPtr(false),
		PrimaryIP: &vpcv1.VirtualNetworkInterfacePrimaryIPPrototype{
			AutoDelete: core.BoolPtr(true),
		},
	}

	// Build security group list
	if len(opts.SecurityGroupIDs) > 0 {
		sgIdentities := make([]vpcv1.SecurityGroupIdentityIntf, len(opts.SecurityGroupIDs))
		for i, sgID := range opts.SecurityGroupIDs {
			id := sgID
			sgIdentities[i] = &vpcv1.SecurityGroupIdentityByID{ID: &id}
		}
		createOpts.SecurityGroups = sgIdentities
	}

	if c.resourceGroupID != "" {
		createOpts.ResourceGroup = &vpcv1.ResourceGroupIdentityByID{ID: &c.resourceGroupID}
	}

	result, _, err := c.service.CreateVirtualNetworkInterfaceWithContext(ctx, createOpts)
	if err != nil {
		return nil, fmt.Errorf("VPC API CreateVNI: %w", err)
	}

	// Tag for idempotency and orphan GC
	if opts.ClusterID != "" || opts.Namespace != "" || opts.VMName != "" {
		var tagNames []string
		if opts.ClusterID != "" {
			tagNames = append(tagNames, fmt.Sprintf("roks-cluster:%s", opts.ClusterID))
		}
		if opts.Namespace != "" {
			tagNames = append(tagNames, fmt.Sprintf("roks-ns:%s", opts.Namespace))
		}
		if opts.VMName != "" {
			tagNames = append(tagNames, fmt.Sprintf("roks-vm:%s", opts.VMName))
		}
		c.tagResource(ctx, derefString(result.CRN), tagNames)
	}

	vni := vniFromSDK(result)

	// The VPC API may silently ignore the name field for VNIs, returning a
	// random-word name. If that happens, PATCH the VNI to set our expected name.
	if vni.Name != opts.Name {
		renamed, err := c.UpdateVNI(ctx, vni.ID, opts.Name)
		if err != nil {
			// Non-fatal: the VNI was created, just with the wrong name.
			// The VM reconciler drift check will retry the rename.
			fmt.Printf("WARN: VNI %s created with name %q instead of %q, rename failed: %v\n", vni.ID, vni.Name, opts.Name, err)
		} else {
			vni = renamed
		}
	}

	return vni, nil
}

// UpdateVNI renames a VNI by PATCHing its name field.
// Used to fix VNIs that the VPC API created with random-word names.
func (c *vpcClient) UpdateVNI(ctx context.Context, vniID, name string) (*VNI, error) {
	if err := c.limiter.Acquire(ctx); err != nil {
		return nil, err
	}
	defer c.limiter.Release()

	patch := map[string]interface{}{
		"name": name,
	}

	result, _, err := c.service.UpdateVirtualNetworkInterfaceWithContext(ctx,
		c.service.NewUpdateVirtualNetworkInterfaceOptions(vniID, patch))
	if err != nil {
		return nil, fmt.Errorf("VPC API UpdateVNI(%s): %w", vniID, err)
	}

	return vniFromSDK(result), nil
}

// GetVNI retrieves a VNI by ID. Used for drift detection.
func (c *vpcClient) GetVNI(ctx context.Context, vniID string) (*VNI, error) {
	if err := c.limiter.Acquire(ctx); err != nil {
		return nil, err
	}
	defer c.limiter.Release()

	result, _, err := c.service.GetVirtualNetworkInterfaceWithContext(ctx, &vpcv1.GetVirtualNetworkInterfaceOptions{
		ID: &vniID,
	})
	if err != nil {
		return nil, fmt.Errorf("VPC API GetVNI(%s): %w", vniID, err)
	}

	return vniFromSDK(result), nil
}

// DeleteVNI deletes a VNI.
func (c *vpcClient) DeleteVNI(ctx context.Context, vniID string) error {
	if err := c.limiter.Acquire(ctx); err != nil {
		return err
	}
	defer c.limiter.Release()

	_, _, err := c.service.DeleteVirtualNetworkInterfacesWithContext(ctx, &vpcv1.DeleteVirtualNetworkInterfacesOptions{
		ID: &vniID,
	})
	if err != nil {
		return fmt.Errorf("VPC API DeleteVNI(%s): %w", vniID, err)
	}

	return nil
}

// ListVNIsByTag finds VNIs tagged with the given cluster/namespace/vm identifiers.
// Used by the webhook for idempotency and by orphan GC for cleanup.
func (c *vpcClient) ListVNIsByTag(ctx context.Context, clusterID, namespace, vmName string) ([]VNI, error) {
	if err := c.limiter.Acquire(ctx); err != nil {
		return nil, err
	}
	defer c.limiter.Release()

	// List all VNIs and filter by name convention: "roks-{clusterID}-{namespace}-{vmName}"
	var allVNIs []VNI
	var start *string

	for {
		listOpts := &vpcv1.ListVirtualNetworkInterfacesOptions{}
		if start != nil {
			listOpts.Start = start
		}

		result, _, err := c.service.ListVirtualNetworkInterfacesWithContext(ctx, listOpts)
		if err != nil {
			return nil, fmt.Errorf("VPC API ListVNIs: %w", err)
		}

		for i := range result.VirtualNetworkInterfaces {
			vni := &result.VirtualNetworkInterfaces[i]
			name := derefString(vni.Name)

			// Match by name convention
			expectedPrefix := fmt.Sprintf("roks-%s", clusterID)
			if clusterID != "" && len(name) < len(expectedPrefix) {
				continue
			}
			if clusterID != "" && name[:len(expectedPrefix)] != expectedPrefix {
				continue
			}
			if namespace != "" && vmName != "" {
				expected := fmt.Sprintf("roks-%s-%s-%s", clusterID, namespace, vmName)
				if name != expected {
					continue
				}
			}

			allVNIs = append(allVNIs, *vniFromSDK(vni))
		}

		if result.Next == nil || result.Next.Href == nil {
			break
		}
		start = result.Next.Href
	}

	return allVNIs, nil
}

// ListVNIs lists all VNIs in the account. Used by the BFF for the VNIs tab.
func (c *vpcClient) ListVNIs(ctx context.Context) ([]VNI, error) {
	if err := c.limiter.Acquire(ctx); err != nil {
		return nil, err
	}
	defer c.limiter.Release()

	var allVNIs []VNI
	var start *string

	for {
		listOpts := &vpcv1.ListVirtualNetworkInterfacesOptions{}
		if start != nil {
			listOpts.Start = start
		}

		result, _, err := c.service.ListVirtualNetworkInterfacesWithContext(ctx, listOpts)
		if err != nil {
			return nil, fmt.Errorf("VPC API ListVNIs: %w", err)
		}

		for i := range result.VirtualNetworkInterfaces {
			allVNIs = append(allVNIs, *vniFromSDK(&result.VirtualNetworkInterfaces[i]))
		}

		if result.Next == nil || result.Next.Href == nil {
			break
		}
		start = result.Next.Href
	}

	return allVNIs, nil
}

func vniFromSDK(v *vpcv1.VirtualNetworkInterface) *VNI {
	vni := &VNI{
		ID:                      derefString(v.ID),
		Name:                    derefString(v.Name),
		MACAddress:              derefString(v.MacAddress),
		AllowIPSpoofing:         v.AllowIPSpoofing != nil && *v.AllowIPSpoofing,
		EnableInfrastructureNat: v.EnableInfrastructureNat != nil && *v.EnableInfrastructureNat,
		Status:                  derefString(v.LifecycleState),
	}
	if v.PrimaryIP != nil {
		vni.PrimaryIP = ReservedIP{
			ID:      derefString(v.PrimaryIP.ID),
			Address: derefString(v.PrimaryIP.Address),
		}
	}
	if v.Subnet != nil {
		vni.SubnetID = derefString(v.Subnet.ID)
		vni.SubnetName = derefString(v.Subnet.Name)
	}
	if v.CreatedAt != nil {
		vni.CreatedAt = v.CreatedAt.String()
	}
	return vni
}
