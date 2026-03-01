package vpc

import (
	"context"
	"fmt"

	"github.com/IBM/vpc-go-sdk/vpcv1"
)

// ListPublicGateways lists all public gateways, optionally filtered by VPC ID.
func (c *vpcClient) ListPublicGateways(ctx context.Context, vpcID string) ([]PublicGateway, error) {
	if err := c.limiter.Acquire(ctx); err != nil {
		return nil, err
	}
	defer c.limiter.Release()

	var allPGWs []PublicGateway
	var start *string

	for {
		listOpts := &vpcv1.ListPublicGatewaysOptions{}
		if start != nil {
			listOpts.Start = start
		}

		result, _, err := c.service.ListPublicGatewaysWithContext(ctx, listOpts)
		if err != nil {
			return nil, fmt.Errorf("VPC API ListPublicGateways: %w", err)
		}

		for i := range result.PublicGateways {
			pgw := &result.PublicGateways[i]
			if vpcID != "" && pgw.VPC != nil && derefString(pgw.VPC.ID) != vpcID {
				continue
			}
			allPGWs = append(allPGWs, pgwFromSDK(pgw))
		}

		if result.Next == nil || result.Next.Href == nil {
			break
		}
		start = result.Next.Href
	}

	return allPGWs, nil
}

func pgwFromSDK(pgw *vpcv1.PublicGateway) PublicGateway {
	p := PublicGateway{
		ID:     derefString(pgw.ID),
		Name:   derefString(pgw.Name),
		Status: derefString(pgw.Status),
	}
	if pgw.Zone != nil {
		p.Zone = derefString(pgw.Zone.Name)
	}
	if pgw.VPC != nil {
		p.VPCID = derefString(pgw.VPC.ID)
		p.VPCName = derefString(pgw.VPC.Name)
	}
	if pgw.FloatingIP != nil {
		p.FloatingIPID = derefString(pgw.FloatingIP.ID)
		p.FloatingIPAddress = derefString(pgw.FloatingIP.Address)
	}
	if pgw.ResourceGroup != nil {
		p.ResourceGroupID = derefString(pgw.ResourceGroup.ID)
		p.ResourceGroupName = derefString(pgw.ResourceGroup.Name)
	}
	if pgw.CreatedAt != nil {
		p.CreatedAt = pgw.CreatedAt.String()
	}
	return p
}
