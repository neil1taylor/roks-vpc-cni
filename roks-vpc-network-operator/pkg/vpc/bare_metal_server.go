package vpc

import (
	"context"
	"fmt"

	"github.com/IBM/vpc-go-sdk/vpcv1"
)

// ListBareMetalServers lists all bare metal servers in a VPC.
func (c *vpcClient) ListBareMetalServers(ctx context.Context, vpcID string) ([]BareMetalServerInfo, error) {
	if err := c.limiter.Acquire(ctx); err != nil {
		return nil, err
	}
	defer c.limiter.Release()

	var all []BareMetalServerInfo
	var start *string

	for {
		opts := &vpcv1.ListBareMetalServersOptions{}
		if vpcID != "" {
			opts.VPCID = &vpcID
		}
		if start != nil {
			opts.Start = start
		}

		result, _, err := c.service.ListBareMetalServersWithContext(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("VPC API ListBareMetalServers: %w", err)
		}

		for i := range result.BareMetalServers {
			bm := &result.BareMetalServers[i]
			info := BareMetalServerInfo{
				ID:   derefString(bm.ID),
				Name: derefString(bm.Name),
			}
			if bm.Zone != nil {
				info.Zone = derefString(bm.Zone.Name)
			}
			all = append(all, info)
		}

		if result.Next == nil || result.Next.Href == nil {
			break
		}
		start = result.Next.Href
	}

	return all, nil
}
