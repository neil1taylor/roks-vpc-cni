package vpc

import (
	"context"
	"fmt"

	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/vpc-go-sdk/vpcv1"
)

// ListSecurityGroups lists all security groups, optionally filtered by VPC.
func (c *vpcClient) ListSecurityGroups(ctx context.Context, vpcID string) ([]SecurityGroup, error) {
	if err := c.limiter.Acquire(ctx); err != nil {
		return nil, err
	}
	defer c.limiter.Release()

	var allSGs []SecurityGroup
	var start *string

	for {
		listOpts := &vpcv1.ListSecurityGroupsOptions{}
		if vpcID != "" {
			listOpts.VPCID = &vpcID
		}
		if start != nil {
			listOpts.Start = start
		}

		result, _, err := c.service.ListSecurityGroupsWithContext(ctx, listOpts)
		if err != nil {
			return nil, fmt.Errorf("VPC API ListSecurityGroups: %w", err)
		}

		for i := range result.SecurityGroups {
			allSGs = append(allSGs, *sgFromSDK(&result.SecurityGroups[i]))
		}

		if result.Next == nil || result.Next.Href == nil {
			break
		}
		start = result.Next.Href
	}

	return allSGs, nil
}

// GetSecurityGroup retrieves a security group by ID, including all rules.
func (c *vpcClient) GetSecurityGroup(ctx context.Context, sgID string) (*SecurityGroup, error) {
	if err := c.limiter.Acquire(ctx); err != nil {
		return nil, err
	}
	defer c.limiter.Release()

	result, _, err := c.service.GetSecurityGroupWithContext(ctx, &vpcv1.GetSecurityGroupOptions{
		ID: &sgID,
	})
	if err != nil {
		return nil, fmt.Errorf("VPC API GetSecurityGroup(%s): %w", sgID, err)
	}

	return sgFromSDK(result), nil
}

// CreateSecurityGroup creates a new security group in a VPC.
func (c *vpcClient) CreateSecurityGroup(ctx context.Context, opts CreateSecurityGroupOptions) (*SecurityGroup, error) {
	if err := c.limiter.Acquire(ctx); err != nil {
		return nil, err
	}
	defer c.limiter.Release()

	createOpts := &vpcv1.CreateSecurityGroupOptions{
		VPC:  &vpcv1.VPCIdentityByID{ID: &opts.VPCID},
		Name: &opts.Name,
	}

	if c.resourceGroupID != "" {
		createOpts.ResourceGroup = &vpcv1.ResourceGroupIdentityByID{ID: &c.resourceGroupID}
	}

	result, _, err := c.service.CreateSecurityGroupWithContext(ctx, createOpts)
	if err != nil {
		return nil, fmt.Errorf("VPC API CreateSecurityGroup: %w", err)
	}

	return sgFromSDK(result), nil
}

// DeleteSecurityGroup deletes a security group.
func (c *vpcClient) DeleteSecurityGroup(ctx context.Context, sgID string) error {
	if err := c.limiter.Acquire(ctx); err != nil {
		return err
	}
	defer c.limiter.Release()

	_, err := c.service.DeleteSecurityGroupWithContext(ctx, &vpcv1.DeleteSecurityGroupOptions{
		ID: &sgID,
	})
	if err != nil {
		return fmt.Errorf("VPC API DeleteSecurityGroup(%s): %w", sgID, err)
	}

	return nil
}

// UpdateSecurityGroup updates a security group's name or description.
func (c *vpcClient) UpdateSecurityGroup(ctx context.Context, sgID string, opts UpdateSecurityGroupOptions) (*SecurityGroup, error) {
	if err := c.limiter.Acquire(ctx); err != nil {
		return nil, err
	}
	defer c.limiter.Release()

	patchMap := map[string]interface{}{}
	if opts.Name != nil {
		patchMap["name"] = *opts.Name
	}
	if opts.Description != nil {
		patchMap["description"] = *opts.Description
	}

	result, _, err := c.service.UpdateSecurityGroupWithContext(ctx, &vpcv1.UpdateSecurityGroupOptions{
		ID:                 &sgID,
		SecurityGroupPatch: patchMap,
	})
	if err != nil {
		return nil, fmt.Errorf("VPC API UpdateSecurityGroup(%s): %w", sgID, err)
	}

	return sgFromSDK(result), nil
}

// AddSecurityGroupRule adds a rule to a security group.
func (c *vpcClient) AddSecurityGroupRule(ctx context.Context, sgID string, opts CreateSGRuleOptions) (*SecurityGroupRule, error) {
	if err := c.limiter.Acquire(ctx); err != nil {
		return nil, err
	}
	defer c.limiter.Release()

	// Build remote
	var remote vpcv1.SecurityGroupRuleRemotePrototypeIntf
	if opts.RemoteCIDR != "" {
		remote = &vpcv1.SecurityGroupRuleRemotePrototypeCIDR{CIDRBlock: &opts.RemoteCIDR}
	} else if opts.RemoteSGID != "" {
		remote = &vpcv1.SecurityGroupRuleRemotePrototypeSecurityGroupIdentitySecurityGroupIdentityByID{ID: &opts.RemoteSGID}
	}

	// Build protocol-specific rule prototype
	var prototype vpcv1.SecurityGroupRulePrototypeIntf

	switch opts.Protocol {
	case "tcp", "udp":
		rule := &vpcv1.SecurityGroupRulePrototypeSecurityGroupRuleProtocolTcpudp{
			Direction: &opts.Direction,
			Protocol:  &opts.Protocol,
		}
		if opts.PortMin != nil {
			rule.PortMin = opts.PortMin
		}
		if opts.PortMax != nil {
			rule.PortMax = opts.PortMax
		}
		if remote != nil {
			rule.Remote = remote
		}
		prototype = rule

	case "icmp":
		rule := &vpcv1.SecurityGroupRulePrototypeSecurityGroupRuleProtocolIcmp{
			Direction: &opts.Direction,
			Protocol:  core.StringPtr("icmp"),
		}
		if opts.ICMPType != nil {
			rule.Type = opts.ICMPType
		}
		if opts.ICMPCode != nil {
			rule.Code = opts.ICMPCode
		}
		if remote != nil {
			rule.Remote = remote
		}
		prototype = rule

	default: // "all"
		rule := &vpcv1.SecurityGroupRulePrototypeSecurityGroupRuleProtocolAnyPrototype{
			Direction: &opts.Direction,
			Protocol:  core.StringPtr("all"),
		}
		if remote != nil {
			rule.Remote = remote
		}
		prototype = rule
	}

	result, _, err := c.service.CreateSecurityGroupRuleWithContext(ctx, &vpcv1.CreateSecurityGroupRuleOptions{
		SecurityGroupID:            &sgID,
		SecurityGroupRulePrototype: prototype,
	})
	if err != nil {
		return nil, fmt.Errorf("VPC API AddSecurityGroupRule(%s): %w", sgID, err)
	}

	return sgRuleFromSDKIntf(result), nil
}

// UpdateSecurityGroupRule updates an existing security group rule.
func (c *vpcClient) UpdateSecurityGroupRule(ctx context.Context, sgID, ruleID string, opts UpdateSGRuleOptions) (*SecurityGroupRule, error) {
	if err := c.limiter.Acquire(ctx); err != nil {
		return nil, err
	}
	defer c.limiter.Release()

	patchMap := map[string]interface{}{}
	if opts.Direction != nil {
		patchMap["direction"] = *opts.Direction
	}
	if opts.PortMin != nil {
		patchMap["port_min"] = *opts.PortMin
	}
	if opts.PortMax != nil {
		patchMap["port_max"] = *opts.PortMax
	}
	if opts.ICMPType != nil {
		patchMap["type"] = *opts.ICMPType
	}
	if opts.ICMPCode != nil {
		patchMap["code"] = *opts.ICMPCode
	}
	if opts.RemoteCIDR != nil {
		patchMap["remote"] = map[string]interface{}{"cidr_block": *opts.RemoteCIDR}
	} else if opts.RemoteSGID != nil {
		patchMap["remote"] = map[string]interface{}{"id": *opts.RemoteSGID}
	}

	result, _, err := c.service.UpdateSecurityGroupRuleWithContext(ctx, &vpcv1.UpdateSecurityGroupRuleOptions{
		SecurityGroupID:        &sgID,
		ID:                     &ruleID,
		SecurityGroupRulePatch: patchMap,
	})
	if err != nil {
		return nil, fmt.Errorf("VPC API UpdateSecurityGroupRule(%s/%s): %w", sgID, ruleID, err)
	}

	return sgRuleFromSDKIntf(result), nil
}

// DeleteSecurityGroupRule removes a rule from a security group.
func (c *vpcClient) DeleteSecurityGroupRule(ctx context.Context, sgID, ruleID string) error {
	if err := c.limiter.Acquire(ctx); err != nil {
		return err
	}
	defer c.limiter.Release()

	_, err := c.service.DeleteSecurityGroupRuleWithContext(ctx, &vpcv1.DeleteSecurityGroupRuleOptions{
		SecurityGroupID: &sgID,
		ID:              &ruleID,
	})
	if err != nil {
		return fmt.Errorf("VPC API DeleteSecurityGroupRule(%s/%s): %w", sgID, ruleID, err)
	}

	return nil
}

func sgFromSDK(sg *vpcv1.SecurityGroup) *SecurityGroup {
	s := &SecurityGroup{
		ID:   derefString(sg.ID),
		Name: derefString(sg.Name),
	}
	if sg.VPC != nil {
		s.VPCID = derefString(sg.VPC.ID)
	}
	if sg.CreatedAt != nil {
		s.CreatedAt = sg.CreatedAt.String()
	}

	// Convert rules
	for i := range sg.Rules {
		rule := sgRuleFromSDKIntf(sg.Rules[i])
		if rule != nil {
			s.Rules = append(s.Rules, *rule)
		}
	}

	return s
}

func sgRuleFromSDKIntf(ruleIntf vpcv1.SecurityGroupRuleIntf) *SecurityGroupRule {
	if ruleIntf == nil {
		return nil
	}

	rule := &SecurityGroupRule{}

	switch r := ruleIntf.(type) {
	case *vpcv1.SecurityGroupRule:
		rule.ID = derefString(r.ID)
		rule.Direction = derefString(r.Direction)
		rule.Protocol = derefString(r.Protocol)
		rule.PortMin = r.PortMin
		rule.PortMax = r.PortMax
		rule.ICMPType = r.Type
		rule.ICMPCode = r.Code
		if r.Remote != nil {
			switch remote := r.Remote.(type) {
			case *vpcv1.SecurityGroupRuleRemote:
				rule.Remote.CIDRBlock = derefString(remote.CIDRBlock)
				rule.Remote.SecurityGroupID = derefString(remote.ID)
			}
		}
	case *vpcv1.SecurityGroupRuleSecurityGroupRuleProtocolTcpudp:
		rule.ID = derefString(r.ID)
		rule.Direction = derefString(r.Direction)
		rule.Protocol = derefString(r.Protocol)
		rule.PortMin = r.PortMin
		rule.PortMax = r.PortMax
		if r.Remote != nil {
			switch remote := r.Remote.(type) {
			case *vpcv1.SecurityGroupRuleRemote:
				rule.Remote.CIDRBlock = derefString(remote.CIDRBlock)
				rule.Remote.SecurityGroupID = derefString(remote.ID)
			}
		}
	case *vpcv1.SecurityGroupRuleSecurityGroupRuleProtocolIcmp:
		rule.ID = derefString(r.ID)
		rule.Direction = derefString(r.Direction)
		rule.Protocol = derefString(r.Protocol)
		rule.ICMPType = r.Type
		rule.ICMPCode = r.Code
		if r.Remote != nil {
			switch remote := r.Remote.(type) {
			case *vpcv1.SecurityGroupRuleRemote:
				rule.Remote.CIDRBlock = derefString(remote.CIDRBlock)
				rule.Remote.SecurityGroupID = derefString(remote.ID)
			}
		}
	case *vpcv1.SecurityGroupRuleProtocolAny:
		rule.ID = derefString(r.ID)
		rule.Direction = derefString(r.Direction)
		rule.Protocol = derefString(r.Protocol)
		if r.Remote != nil {
			switch remote := r.Remote.(type) {
			case *vpcv1.SecurityGroupRuleRemote:
				rule.Remote.CIDRBlock = derefString(remote.CIDRBlock)
				rule.Remote.SecurityGroupID = derefString(remote.ID)
			}
		}
	case *vpcv1.SecurityGroupRuleProtocolIcmptcpudp:
		rule.ID = derefString(r.ID)
		rule.Direction = derefString(r.Direction)
		rule.Protocol = derefString(r.Protocol)
		if r.Remote != nil {
			switch remote := r.Remote.(type) {
			case *vpcv1.SecurityGroupRuleRemote:
				rule.Remote.CIDRBlock = derefString(remote.CIDRBlock)
				rule.Remote.SecurityGroupID = derefString(remote.ID)
			}
		}
	case *vpcv1.SecurityGroupRuleProtocolIndividual:
		rule.ID = derefString(r.ID)
		rule.Direction = derefString(r.Direction)
		rule.Protocol = derefString(r.Protocol)
		if r.Remote != nil {
			switch remote := r.Remote.(type) {
			case *vpcv1.SecurityGroupRuleRemote:
				rule.Remote.CIDRBlock = derefString(remote.CIDRBlock)
				rule.Remote.SecurityGroupID = derefString(remote.ID)
			}
		}
	}

	return rule
}
