package vpc

import (
	"context"
	"fmt"

	"github.com/IBM/vpc-go-sdk/vpcv1"
)

// ListRoutingTables lists all routing tables for a VPC.
func (c *vpcClient) ListRoutingTables(ctx context.Context, vpcID string) ([]RoutingTable, error) {
	if err := c.limiter.Acquire(ctx); err != nil {
		return nil, err
	}
	defer c.limiter.Release()

	var allTables []RoutingTable
	var start *string

	for {
		listOpts := &vpcv1.ListVPCRoutingTablesOptions{
			VPCID: &vpcID,
		}
		if start != nil {
			listOpts.Start = start
		}

		result, _, err := c.service.ListVPCRoutingTablesWithContext(ctx, listOpts)
		if err != nil {
			return nil, fmt.Errorf("VPC API ListRoutingTables(%s): %w", vpcID, err)
		}

		for i := range result.RoutingTables {
			allTables = append(allTables, routingTableFromSDK(&result.RoutingTables[i]))
		}

		if result.Next == nil || result.Next.Href == nil {
			break
		}
		start = result.Next.Href
	}

	return allTables, nil
}

// GetRoutingTable retrieves a single routing table.
func (c *vpcClient) GetRoutingTable(ctx context.Context, vpcID, routingTableID string) (*RoutingTable, error) {
	if err := c.limiter.Acquire(ctx); err != nil {
		return nil, err
	}
	defer c.limiter.Release()

	result, _, err := c.service.GetVPCRoutingTableWithContext(ctx, &vpcv1.GetVPCRoutingTableOptions{
		VPCID: &vpcID,
		ID:    &routingTableID,
	})
	if err != nil {
		return nil, fmt.Errorf("VPC API GetRoutingTable(%s/%s): %w", vpcID, routingTableID, err)
	}

	rt := routingTableFromSDK(result)
	return &rt, nil
}

// CreateRoutingTable creates a new routing table for a VPC.
func (c *vpcClient) CreateRoutingTable(ctx context.Context, vpcID string, opts CreateRoutingTableOptions) (*RoutingTable, error) {
	if err := c.limiter.Acquire(ctx); err != nil {
		return nil, err
	}
	defer c.limiter.Release()

	createOpts := &vpcv1.CreateVPCRoutingTableOptions{
		VPCID: &vpcID,
	}
	if opts.Name != "" {
		createOpts.Name = &opts.Name
	}
	if opts.RouteInternetIngress {
		createOpts.RouteInternetIngress = &opts.RouteInternetIngress
	}

	result, _, err := c.service.CreateVPCRoutingTableWithContext(ctx, createOpts)
	if err != nil {
		return nil, fmt.Errorf("VPC API CreateRoutingTable(%s): %w", vpcID, err)
	}

	rt := routingTableFromSDK(result)

	// Tag for traceability (guard: routing tables may not have CRN)
	if result.CRN != nil && (opts.ClusterID != "" || opts.OwnerKind != "") {
		c.tagResource(ctx, *result.CRN, BuildTags(opts.ClusterID, "routing-table", opts.OwnerKind, opts.OwnerName))
	}

	return &rt, nil
}

// DeleteRoutingTable deletes a routing table from a VPC.
func (c *vpcClient) DeleteRoutingTable(ctx context.Context, vpcID, routingTableID string) error {
	if err := c.limiter.Acquire(ctx); err != nil {
		return err
	}
	defer c.limiter.Release()

	_, err := c.service.DeleteVPCRoutingTableWithContext(ctx, &vpcv1.DeleteVPCRoutingTableOptions{
		VPCID: &vpcID,
		ID:    &routingTableID,
	})
	if err != nil {
		return fmt.Errorf("VPC API DeleteRoutingTable(%s/%s): %w", vpcID, routingTableID, err)
	}

	return nil
}

// ListRoutes lists all routes in a routing table.
func (c *vpcClient) ListRoutes(ctx context.Context, vpcID, routingTableID string) ([]Route, error) {
	if err := c.limiter.Acquire(ctx); err != nil {
		return nil, err
	}
	defer c.limiter.Release()

	var allRoutes []Route
	var start *string

	for {
		listOpts := &vpcv1.ListVPCRoutingTableRoutesOptions{
			VPCID:          &vpcID,
			RoutingTableID: &routingTableID,
		}
		if start != nil {
			listOpts.Start = start
		}

		result, _, err := c.service.ListVPCRoutingTableRoutesWithContext(ctx, listOpts)
		if err != nil {
			return nil, fmt.Errorf("VPC API ListRoutes(%s/%s): %w", vpcID, routingTableID, err)
		}

		for i := range result.Routes {
			allRoutes = append(allRoutes, routeFromSDK(&result.Routes[i]))
		}

		if result.Next == nil || result.Next.Href == nil {
			break
		}
		start = result.Next.Href
	}

	return allRoutes, nil
}

// CreateRoute creates a route in a routing table.
func (c *vpcClient) CreateRoute(ctx context.Context, vpcID, routingTableID string, opts CreateRouteOptions) (*Route, error) {
	if err := c.limiter.Acquire(ctx); err != nil {
		return nil, err
	}
	defer c.limiter.Release()

	createOpts := &vpcv1.CreateVPCRoutingTableRouteOptions{
		VPCID:          &vpcID,
		RoutingTableID: &routingTableID,
		Destination:    &opts.Destination,
		Zone:           &vpcv1.ZoneIdentityByName{Name: &opts.Zone},
		Action:         &opts.Action,
	}

	if opts.Name != "" {
		createOpts.Name = &opts.Name
	}

	if opts.Priority != nil {
		createOpts.Priority = opts.Priority
	}

	// Set next hop for "deliver" action
	if opts.Action == "deliver" && opts.NextHopIP != "" {
		createOpts.NextHop = &vpcv1.RouteNextHopPrototypeRouteNextHopIP{
			Address: &opts.NextHopIP,
		}
	}

	result, _, err := c.service.CreateVPCRoutingTableRouteWithContext(ctx, createOpts)
	if err != nil {
		return nil, fmt.Errorf("VPC API CreateRoute(%s/%s): %w", vpcID, routingTableID, err)
	}

	route := routeFromSDK(result)
	// Note: VPC routes don't expose CRN, so tagging is not possible.
	return &route, nil
}

// DeleteRoute deletes a route from a routing table.
func (c *vpcClient) DeleteRoute(ctx context.Context, vpcID, routingTableID, routeID string) error {
	if err := c.limiter.Acquire(ctx); err != nil {
		return err
	}
	defer c.limiter.Release()

	_, err := c.service.DeleteVPCRoutingTableRouteWithContext(ctx, &vpcv1.DeleteVPCRoutingTableRouteOptions{
		VPCID:          &vpcID,
		RoutingTableID: &routingTableID,
		ID:             &routeID,
	})
	if err != nil {
		return fmt.Errorf("VPC API DeleteRoute(%s/%s/%s): %w", vpcID, routingTableID, routeID, err)
	}

	return nil
}

func routingTableFromSDK(rt *vpcv1.RoutingTable) RoutingTable {
	t := RoutingTable{
		ID:   derefString(rt.ID),
		Name: derefString(rt.Name),
	}
	if rt.IsDefault != nil {
		t.IsDefault = *rt.IsDefault
	}
	if rt.RouteInternetIngress != nil {
		t.RouteInternetIngress = *rt.RouteInternetIngress
	}
	if rt.LifecycleState != nil {
		t.LifecycleState = *rt.LifecycleState
	}
	t.RouteCount = len(rt.Routes)
	if rt.CreatedAt != nil {
		t.CreatedAt = rt.CreatedAt.String()
	}
	return t
}

func routeFromSDK(r *vpcv1.Route) Route {
	route := Route{
		ID:          derefString(r.ID),
		Name:        derefString(r.Name),
		Destination: derefString(r.Destination),
		Action:      derefString(r.Action),
	}
	if r.Zone != nil {
		route.Zone = derefString(r.Zone.Name)
	}
	if r.Priority != nil {
		route.Priority = *r.Priority
	}
	if r.Origin != nil {
		route.Origin = *r.Origin
	}
	if r.LifecycleState != nil {
		route.LifecycleState = *r.LifecycleState
	}
	if r.CreatedAt != nil {
		route.CreatedAt = r.CreatedAt.String()
	}

	// NextHop is polymorphic — extract IP address
	if r.NextHop != nil {
		switch nh := r.NextHop.(type) {
		case *vpcv1.RouteNextHopIP:
			route.NextHop = derefString(nh.Address)
		case *vpcv1.RouteNextHop:
			route.NextHop = derefString(nh.Address)
		}
	}

	return route
}
