package vpc

import (
	"context"
	"fmt"

	"github.com/IBM/vpc-go-sdk/vpcv1"
)

// ListZones lists all availability zones in a region.
func (c *vpcClient) ListZones(ctx context.Context, region string) ([]Zone, error) {
	if err := c.limiter.Acquire(ctx); err != nil {
		return nil, err
	}
	defer c.limiter.Release()

	result, _, err := c.service.ListRegionZonesWithContext(ctx, &vpcv1.ListRegionZonesOptions{
		RegionName: &region,
	})
	if err != nil {
		return nil, fmt.Errorf("VPC API ListZones(%s): %w", region, err)
	}

	zones := make([]Zone, len(result.Zones))
	for i, z := range result.Zones {
		zones[i] = Zone{
			Name:   derefString(z.Name),
			Region: region,
			Status: derefString(z.Status),
		}
	}

	return zones, nil
}
