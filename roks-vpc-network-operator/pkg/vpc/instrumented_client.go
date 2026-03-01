package vpc

import (
	"context"
	"time"

	operatormetrics "github.com/IBM/roks-vpc-network-operator/pkg/metrics"
)

// InstrumentedClient wraps a Client and records Prometheus metrics for every VPC API call.
type InstrumentedClient struct {
	inner Client
}

// NewInstrumentedClient wraps a Client with Prometheus metrics instrumentation.
func NewInstrumentedClient(inner Client) Client {
	return &InstrumentedClient{inner: inner}
}

func recordCall(method string, start time.Time, err error) {
	status := "success"
	if err != nil {
		status = "error"
	}
	operatormetrics.VPCAPICallsTotal.WithLabelValues(method, status).Inc()
	operatormetrics.VPCAPILatency.WithLabelValues(method).Observe(time.Since(start).Seconds())
}

func (c *InstrumentedClient) CreateSubnet(ctx context.Context, opts CreateSubnetOptions) (*Subnet, error) {
	start := time.Now()
	result, err := c.inner.CreateSubnet(ctx, opts)
	recordCall("CreateSubnet", start, err)
	return result, err
}

func (c *InstrumentedClient) GetSubnet(ctx context.Context, subnetID string) (*Subnet, error) {
	start := time.Now()
	result, err := c.inner.GetSubnet(ctx, subnetID)
	recordCall("GetSubnet", start, err)
	return result, err
}

func (c *InstrumentedClient) DeleteSubnet(ctx context.Context, subnetID string) error {
	start := time.Now()
	err := c.inner.DeleteSubnet(ctx, subnetID)
	recordCall("DeleteSubnet", start, err)
	return err
}

func (c *InstrumentedClient) ListSubnets(ctx context.Context, vpcID string) ([]Subnet, error) {
	start := time.Now()
	result, err := c.inner.ListSubnets(ctx, vpcID)
	recordCall("ListSubnets", start, err)
	return result, err
}

func (c *InstrumentedClient) CreateVNI(ctx context.Context, opts CreateVNIOptions) (*VNI, error) {
	start := time.Now()
	result, err := c.inner.CreateVNI(ctx, opts)
	recordCall("CreateVNI", start, err)
	return result, err
}

func (c *InstrumentedClient) GetVNI(ctx context.Context, vniID string) (*VNI, error) {
	start := time.Now()
	result, err := c.inner.GetVNI(ctx, vniID)
	recordCall("GetVNI", start, err)
	return result, err
}

func (c *InstrumentedClient) UpdateVNI(ctx context.Context, vniID, name string) (*VNI, error) {
	start := time.Now()
	result, err := c.inner.UpdateVNI(ctx, vniID, name)
	recordCall("UpdateVNI", start, err)
	return result, err
}

func (c *InstrumentedClient) DeleteVNI(ctx context.Context, vniID string) error {
	start := time.Now()
	err := c.inner.DeleteVNI(ctx, vniID)
	recordCall("DeleteVNI", start, err)
	return err
}

func (c *InstrumentedClient) ListVNIsByTag(ctx context.Context, clusterID, namespace, vmName string) ([]VNI, error) {
	start := time.Now()
	result, err := c.inner.ListVNIsByTag(ctx, clusterID, namespace, vmName)
	recordCall("ListVNIsByTag", start, err)
	return result, err
}

func (c *InstrumentedClient) CreateVLANAttachment(ctx context.Context, opts CreateVLANAttachmentOptions) (*VLANAttachment, error) {
	start := time.Now()
	result, err := c.inner.CreateVLANAttachment(ctx, opts)
	recordCall("CreateVLANAttachment", start, err)
	return result, err
}

func (c *InstrumentedClient) CreateVMAttachment(ctx context.Context, opts CreateVMAttachmentOptions) (*VMAttachmentResult, error) {
	start := time.Now()
	result, err := c.inner.CreateVMAttachment(ctx, opts)
	recordCall("CreateVMAttachment", start, err)
	return result, err
}

func (c *InstrumentedClient) DeleteVLANAttachment(ctx context.Context, bmServerID, attachmentID string) error {
	start := time.Now()
	err := c.inner.DeleteVLANAttachment(ctx, bmServerID, attachmentID)
	recordCall("DeleteVLANAttachment", start, err)
	return err
}

func (c *InstrumentedClient) ListVLANAttachments(ctx context.Context, bmServerID string) ([]VLANAttachment, error) {
	start := time.Now()
	result, err := c.inner.ListVLANAttachments(ctx, bmServerID)
	recordCall("ListVLANAttachments", start, err)
	return result, err
}

func (c *InstrumentedClient) EnsurePCIAllowedVLAN(ctx context.Context, bmServerID string, vlanID int64) error {
	start := time.Now()
	err := c.inner.EnsurePCIAllowedVLAN(ctx, bmServerID, vlanID)
	recordCall("EnsurePCIAllowedVLAN", start, err)
	return err
}

func (c *InstrumentedClient) CreateFloatingIP(ctx context.Context, opts CreateFloatingIPOptions) (*FloatingIP, error) {
	start := time.Now()
	result, err := c.inner.CreateFloatingIP(ctx, opts)
	recordCall("CreateFloatingIP", start, err)
	return result, err
}

func (c *InstrumentedClient) GetFloatingIP(ctx context.Context, fipID string) (*FloatingIP, error) {
	start := time.Now()
	result, err := c.inner.GetFloatingIP(ctx, fipID)
	recordCall("GetFloatingIP", start, err)
	return result, err
}

func (c *InstrumentedClient) UpdateFloatingIP(ctx context.Context, fipID string, opts UpdateFloatingIPOptions) (*FloatingIP, error) {
	start := time.Now()
	result, err := c.inner.UpdateFloatingIP(ctx, fipID, opts)
	recordCall("UpdateFloatingIP", start, err)
	return result, err
}

func (c *InstrumentedClient) DeleteFloatingIP(ctx context.Context, fipID string) error {
	start := time.Now()
	err := c.inner.DeleteFloatingIP(ctx, fipID)
	recordCall("DeleteFloatingIP", start, err)
	return err
}

func (c *InstrumentedClient) ListFloatingIPs(ctx context.Context) ([]FloatingIP, error) {
	start := time.Now()
	result, err := c.inner.ListFloatingIPs(ctx)
	recordCall("ListFloatingIPs", start, err)
	return result, err
}

func (c *InstrumentedClient) ListVPCAddressPrefixes(ctx context.Context, vpcID string) ([]AddressPrefix, error) {
	start := time.Now()
	result, err := c.inner.ListVPCAddressPrefixes(ctx, vpcID)
	recordCall("ListVPCAddressPrefixes", start, err)
	return result, err
}

func (c *InstrumentedClient) CreateVPCAddressPrefix(ctx context.Context, opts CreateAddressPrefixOptions) (*AddressPrefix, error) {
	start := time.Now()
	result, err := c.inner.CreateVPCAddressPrefix(ctx, opts)
	recordCall("CreateVPCAddressPrefix", start, err)
	return result, err
}

func (c *InstrumentedClient) ListSubnetReservedIPs(ctx context.Context, subnetID string) ([]ReservedIP, error) {
	start := time.Now()
	result, err := c.inner.ListSubnetReservedIPs(ctx, subnetID)
	recordCall("ListSubnetReservedIPs", start, err)
	return result, err
}

func (c *InstrumentedClient) ListBareMetalServers(ctx context.Context, vpcID string) ([]BareMetalServerInfo, error) {
	start := time.Now()
	result, err := c.inner.ListBareMetalServers(ctx, vpcID)
	recordCall("ListBareMetalServers", start, err)
	return result, err
}

func (c *InstrumentedClient) ListRoutingTables(ctx context.Context, vpcID string) ([]RoutingTable, error) {
	start := time.Now()
	result, err := c.inner.ListRoutingTables(ctx, vpcID)
	recordCall("ListRoutingTables", start, err)
	return result, err
}

func (c *InstrumentedClient) GetRoutingTable(ctx context.Context, vpcID, routingTableID string) (*RoutingTable, error) {
	start := time.Now()
	result, err := c.inner.GetRoutingTable(ctx, vpcID, routingTableID)
	recordCall("GetRoutingTable", start, err)
	return result, err
}

func (c *InstrumentedClient) ListRoutes(ctx context.Context, vpcID, routingTableID string) ([]Route, error) {
	start := time.Now()
	result, err := c.inner.ListRoutes(ctx, vpcID, routingTableID)
	recordCall("ListRoutes", start, err)
	return result, err
}

func (c *InstrumentedClient) CreateRoute(ctx context.Context, vpcID, routingTableID string, opts CreateRouteOptions) (*Route, error) {
	start := time.Now()
	result, err := c.inner.CreateRoute(ctx, vpcID, routingTableID, opts)
	recordCall("CreateRoute", start, err)
	return result, err
}

func (c *InstrumentedClient) DeleteRoute(ctx context.Context, vpcID, routingTableID, routeID string) error {
	start := time.Now()
	err := c.inner.DeleteRoute(ctx, vpcID, routingTableID, routeID)
	recordCall("DeleteRoute", start, err)
	return err
}

func (c *InstrumentedClient) CreateRoutingTable(ctx context.Context, vpcID string, opts CreateRoutingTableOptions) (*RoutingTable, error) {
	start := time.Now()
	result, err := c.inner.CreateRoutingTable(ctx, vpcID, opts)
	recordCall("CreateRoutingTable", start, err)
	return result, err
}

func (c *InstrumentedClient) DeleteRoutingTable(ctx context.Context, vpcID, routingTableID string) error {
	start := time.Now()
	err := c.inner.DeleteRoutingTable(ctx, vpcID, routingTableID)
	recordCall("DeleteRoutingTable", start, err)
	return err
}

func (c *InstrumentedClient) CreatePublicAddressRange(ctx context.Context, opts CreatePublicAddressRangeOptions) (*PublicAddressRange, error) {
	start := time.Now()
	result, err := c.inner.CreatePublicAddressRange(ctx, opts)
	recordCall("CreatePublicAddressRange", start, err)
	return result, err
}

func (c *InstrumentedClient) GetPublicAddressRange(ctx context.Context, parID string) (*PublicAddressRange, error) {
	start := time.Now()
	result, err := c.inner.GetPublicAddressRange(ctx, parID)
	recordCall("GetPublicAddressRange", start, err)
	return result, err
}

func (c *InstrumentedClient) ListPublicAddressRanges(ctx context.Context, vpcID string) ([]PublicAddressRange, error) {
	start := time.Now()
	result, err := c.inner.ListPublicAddressRanges(ctx, vpcID)
	recordCall("ListPublicAddressRanges", start, err)
	return result, err
}

func (c *InstrumentedClient) DeletePublicAddressRange(ctx context.Context, parID string) error {
	start := time.Now()
	err := c.inner.DeletePublicAddressRange(ctx, parID)
	recordCall("DeletePublicAddressRange", start, err)
	return err
}

var _ Client = (*InstrumentedClient)(nil)
