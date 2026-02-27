package vpc

import (
	"context"
	"fmt"

	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/vpc-go-sdk/vpcv1"
)

// ListNetworkACLs lists all network ACLs, optionally filtered by VPC.
func (c *vpcClient) ListNetworkACLs(ctx context.Context, vpcID string) ([]NetworkACL, error) {
	if err := c.limiter.Acquire(ctx); err != nil {
		return nil, err
	}
	defer c.limiter.Release()

	var allACLs []NetworkACL
	var start *string

	for {
		listOpts := &vpcv1.ListNetworkAclsOptions{}
		if start != nil {
			listOpts.Start = start
		}

		result, _, err := c.service.ListNetworkAclsWithContext(ctx, listOpts)
		if err != nil {
			return nil, fmt.Errorf("VPC API ListNetworkACLs: %w", err)
		}

		for i := range result.NetworkAcls {
			acl := &result.NetworkAcls[i]
			// Filter by VPC if provided
			if vpcID != "" && acl.VPC != nil && derefString(acl.VPC.ID) != vpcID {
				continue
			}
			allACLs = append(allACLs, *aclFromSDK(acl))
		}

		if result.Next == nil || result.Next.Href == nil {
			break
		}
		start = result.Next.Href
	}

	return allACLs, nil
}

// GetNetworkACL retrieves a network ACL by ID, including all rules.
func (c *vpcClient) GetNetworkACL(ctx context.Context, aclID string) (*NetworkACL, error) {
	if err := c.limiter.Acquire(ctx); err != nil {
		return nil, err
	}
	defer c.limiter.Release()

	result, _, err := c.service.GetNetworkACLWithContext(ctx, &vpcv1.GetNetworkACLOptions{
		ID: &aclID,
	})
	if err != nil {
		return nil, fmt.Errorf("VPC API GetNetworkACL(%s): %w", aclID, err)
	}

	acl := aclFromSDK(result)

	// Also list rules
	rules, err := c.listACLRules(ctx, aclID)
	if err != nil {
		return nil, err
	}
	acl.Rules = rules

	return acl, nil
}

// CreateNetworkACL creates a new network ACL in a VPC.
func (c *vpcClient) CreateNetworkACL(ctx context.Context, opts CreateNetworkACLOptions) (*NetworkACL, error) {
	if err := c.limiter.Acquire(ctx); err != nil {
		return nil, err
	}
	defer c.limiter.Release()

	createOpts := &vpcv1.CreateNetworkACLOptions{
		NetworkACLPrototype: &vpcv1.NetworkACLPrototypeNetworkACLByRules{
			Name: &opts.Name,
			VPC:  &vpcv1.VPCIdentityByID{ID: &opts.VPCID},
		},
	}

	result, _, err := c.service.CreateNetworkACLWithContext(ctx, createOpts)
	if err != nil {
		return nil, fmt.Errorf("VPC API CreateNetworkACL: %w", err)
	}

	return aclFromSDK(result), nil
}

// DeleteNetworkACL deletes a network ACL.
func (c *vpcClient) DeleteNetworkACL(ctx context.Context, aclID string) error {
	if err := c.limiter.Acquire(ctx); err != nil {
		return err
	}
	defer c.limiter.Release()

	_, err := c.service.DeleteNetworkACLWithContext(ctx, &vpcv1.DeleteNetworkACLOptions{
		ID: &aclID,
	})
	if err != nil {
		return fmt.Errorf("VPC API DeleteNetworkACL(%s): %w", aclID, err)
	}

	return nil
}

// UpdateNetworkACL updates a network ACL's name.
func (c *vpcClient) UpdateNetworkACL(ctx context.Context, aclID string, opts UpdateNetworkACLOptions) (*NetworkACL, error) {
	if err := c.limiter.Acquire(ctx); err != nil {
		return nil, err
	}
	defer c.limiter.Release()

	patchMap := map[string]interface{}{}
	if opts.Name != nil {
		patchMap["name"] = *opts.Name
	}

	result, _, err := c.service.UpdateNetworkACLWithContext(ctx, &vpcv1.UpdateNetworkACLOptions{
		ID:              &aclID,
		NetworkACLPatch: patchMap,
	})
	if err != nil {
		return nil, fmt.Errorf("VPC API UpdateNetworkACL(%s): %w", aclID, err)
	}

	return aclFromSDK(result), nil
}

// AddNetworkACLRule adds a rule to a network ACL.
func (c *vpcClient) AddNetworkACLRule(ctx context.Context, aclID string, opts CreateACLRuleOptions) (*NetworkACLRule, error) {
	if err := c.limiter.Acquire(ctx); err != nil {
		return nil, err
	}
	defer c.limiter.Release()

	var prototype vpcv1.NetworkACLRulePrototypeIntf

	switch opts.Protocol {
	case "tcp", "udp":
		rule := &vpcv1.NetworkACLRulePrototypeNetworkACLRuleProtocolTcpudpPrototype{
			Name:      &opts.Name,
			Direction: &opts.Direction,
			Action:    &opts.Action,
			Protocol:  &opts.Protocol,
			Source:     &opts.Source,
			Destination: &opts.Destination,
		}
		if opts.PortMin != nil {
			rule.DestinationPortMin = opts.PortMin
		}
		if opts.PortMax != nil {
			rule.DestinationPortMax = opts.PortMax
		}
		prototype = rule

	case "icmp":
		rule := &vpcv1.NetworkACLRulePrototypeNetworkACLRuleProtocolIcmpPrototype{
			Name:      &opts.Name,
			Direction: &opts.Direction,
			Action:    &opts.Action,
			Protocol:  core.StringPtr("icmp"),
			Source:     &opts.Source,
			Destination: &opts.Destination,
		}
		if opts.ICMPType != nil {
			rule.Type = opts.ICMPType
		}
		if opts.ICMPCode != nil {
			rule.Code = opts.ICMPCode
		}
		prototype = rule

	default: // "all"
		prototype = &vpcv1.NetworkACLRulePrototypeNetworkACLRuleProtocolAnyPrototype{
			Name:        &opts.Name,
			Direction:   &opts.Direction,
			Action:      &opts.Action,
			Protocol:    core.StringPtr("all"),
			Source:      &opts.Source,
			Destination: &opts.Destination,
		}
	}

	result, _, err := c.service.CreateNetworkACLRuleWithContext(ctx, &vpcv1.CreateNetworkACLRuleOptions{
		NetworkACLID:             &aclID,
		NetworkACLRulePrototype: prototype,
	})
	if err != nil {
		return nil, fmt.Errorf("VPC API AddNetworkACLRule(%s): %w", aclID, err)
	}

	return aclRuleFromRuleIntf(result), nil
}

// UpdateNetworkACLRule updates an existing ACL rule.
func (c *vpcClient) UpdateNetworkACLRule(ctx context.Context, aclID, ruleID string, opts UpdateACLRuleOptions) (*NetworkACLRule, error) {
	if err := c.limiter.Acquire(ctx); err != nil {
		return nil, err
	}
	defer c.limiter.Release()

	patchMap := map[string]interface{}{}
	if opts.Name != nil {
		patchMap["name"] = *opts.Name
	}
	if opts.Direction != nil {
		patchMap["direction"] = *opts.Direction
	}
	if opts.Action != nil {
		patchMap["action"] = *opts.Action
	}
	if opts.PortMin != nil {
		patchMap["destination_port_min"] = *opts.PortMin
	}
	if opts.PortMax != nil {
		patchMap["destination_port_max"] = *opts.PortMax
	}
	if opts.Source != nil {
		patchMap["source"] = *opts.Source
	}
	if opts.Destination != nil {
		patchMap["destination"] = *opts.Destination
	}

	result, _, err := c.service.UpdateNetworkACLRuleWithContext(ctx, &vpcv1.UpdateNetworkACLRuleOptions{
		NetworkACLID:         &aclID,
		ID:                   &ruleID,
		NetworkACLRulePatch: patchMap,
	})
	if err != nil {
		return nil, fmt.Errorf("VPC API UpdateNetworkACLRule(%s/%s): %w", aclID, ruleID, err)
	}

	return aclRuleFromRuleIntf(result), nil
}

// DeleteNetworkACLRule removes a rule from a network ACL.
func (c *vpcClient) DeleteNetworkACLRule(ctx context.Context, aclID, ruleID string) error {
	if err := c.limiter.Acquire(ctx); err != nil {
		return err
	}
	defer c.limiter.Release()

	_, err := c.service.DeleteNetworkACLRuleWithContext(ctx, &vpcv1.DeleteNetworkACLRuleOptions{
		NetworkACLID: &aclID,
		ID:           &ruleID,
	})
	if err != nil {
		return fmt.Errorf("VPC API DeleteNetworkACLRule(%s/%s): %w", aclID, ruleID, err)
	}

	return nil
}

// listACLRules lists all rules for a network ACL (internal helper, no rate limiter).
func (c *vpcClient) listACLRules(ctx context.Context, aclID string) ([]NetworkACLRule, error) {
	var allRules []NetworkACLRule
	var start *string

	for {
		listOpts := &vpcv1.ListNetworkACLRulesOptions{
			NetworkACLID: &aclID,
		}
		if start != nil {
			listOpts.Start = start
		}

		result, _, err := c.service.ListNetworkACLRulesWithContext(ctx, listOpts)
		if err != nil {
			return nil, fmt.Errorf("VPC API ListNetworkACLRules(%s): %w", aclID, err)
		}

		for i := range result.Rules {
			rule := aclRuleFromSDKIntf(result.Rules[i])
			if rule != nil {
				allRules = append(allRules, *rule)
			}
		}

		if result.Next == nil || result.Next.Href == nil {
			break
		}
		start = result.Next.Href
	}

	return allRules, nil
}

func aclFromSDK(acl *vpcv1.NetworkACL) *NetworkACL {
	a := &NetworkACL{
		ID:   derefString(acl.ID),
		Name: derefString(acl.Name),
	}
	if acl.VPC != nil {
		a.VPCID = derefString(acl.VPC.ID)
	}
	if acl.CreatedAt != nil {
		a.CreatedAt = acl.CreatedAt.String()
	}
	for _, subnet := range acl.Subnets {
		if subnet.ID != nil {
			a.Subnets = append(a.Subnets, *subnet.ID)
		}
	}
	return a
}

// aclRuleFromRuleIntf converts NetworkACLRuleIntf (returned by create/update) to our type.
func aclRuleFromRuleIntf(ruleIntf vpcv1.NetworkACLRuleIntf) *NetworkACLRule {
	if ruleIntf == nil {
		return nil
	}
	switch r := ruleIntf.(type) {
	case *vpcv1.NetworkACLRule:
		return &NetworkACLRule{
			ID:          derefString(r.ID),
			Name:        derefString(r.Name),
			Direction:   derefString(r.Direction),
			Action:      derefString(r.Action),
			Protocol:    derefString(r.Protocol),
			Source:      derefString(r.Source),
			Destination: derefString(r.Destination),
			PortMin:     r.DestinationPortMin,
			PortMax:     r.DestinationPortMax,
			ICMPType:    r.Type,
			ICMPCode:    r.Code,
		}
	}
	return &NetworkACLRule{}
}

func aclRuleFromSDKIntf(ruleIntf vpcv1.NetworkACLRuleItemIntf) *NetworkACLRule {
	if ruleIntf == nil {
		return nil
	}

	rule := &NetworkACLRule{}

	switch r := ruleIntf.(type) {
	case *vpcv1.NetworkACLRuleItem:
		rule.ID = derefString(r.ID)
		rule.Name = derefString(r.Name)
		rule.Direction = derefString(r.Direction)
		rule.Action = derefString(r.Action)
		rule.Protocol = derefString(r.Protocol)
		rule.Source = derefString(r.Source)
		rule.Destination = derefString(r.Destination)
		rule.PortMin = r.DestinationPortMin
		rule.PortMax = r.DestinationPortMax
		rule.ICMPType = r.Type
		rule.ICMPCode = r.Code
	case *vpcv1.NetworkACLRuleItemNetworkACLRuleProtocolAny:
		rule.ID = derefString(r.ID)
		rule.Name = derefString(r.Name)
		rule.Direction = derefString(r.Direction)
		rule.Action = derefString(r.Action)
		rule.Protocol = derefString(r.Protocol)
		rule.Source = derefString(r.Source)
		rule.Destination = derefString(r.Destination)
	case *vpcv1.NetworkACLRuleItemNetworkACLRuleProtocolTcpudp:
		rule.ID = derefString(r.ID)
		rule.Name = derefString(r.Name)
		rule.Direction = derefString(r.Direction)
		rule.Action = derefString(r.Action)
		rule.Protocol = derefString(r.Protocol)
		rule.Source = derefString(r.Source)
		rule.Destination = derefString(r.Destination)
		rule.PortMin = r.DestinationPortMin
		rule.PortMax = r.DestinationPortMax
	case *vpcv1.NetworkACLRuleItemNetworkACLRuleProtocolIcmp:
		rule.ID = derefString(r.ID)
		rule.Name = derefString(r.Name)
		rule.Direction = derefString(r.Direction)
		rule.Action = derefString(r.Action)
		rule.Protocol = derefString(r.Protocol)
		rule.Source = derefString(r.Source)
		rule.Destination = derefString(r.Destination)
		rule.ICMPType = r.Type
		rule.ICMPCode = r.Code
	}

	return rule
}
