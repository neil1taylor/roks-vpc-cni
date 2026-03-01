package vpc

import (
	"context"
	"fmt"
	"sync"
)

// MockClient implements both Client and ExtendedClient for testing.
// Each method can be overridden via function fields.
type MockClient struct {
	mu sync.Mutex

	// Call tracking
	Calls map[string]int

	// Subnet operations
	CreateSubnetFn func(ctx context.Context, opts CreateSubnetOptions) (*Subnet, error)
	GetSubnetFn    func(ctx context.Context, subnetID string) (*Subnet, error)
	DeleteSubnetFn func(ctx context.Context, subnetID string) error

	// VNI operations
	CreateVNIFn     func(ctx context.Context, opts CreateVNIOptions) (*VNI, error)
	GetVNIFn        func(ctx context.Context, vniID string) (*VNI, error)
	UpdateVNIFn     func(ctx context.Context, vniID, name string) (*VNI, error)
	DeleteVNIFn     func(ctx context.Context, vniID string) error
	ListVNIsByTagFn func(ctx context.Context, clusterID, namespace, vmName string) ([]VNI, error)

	// VLAN Attachment operations
	CreateVLANAttachmentFn func(ctx context.Context, opts CreateVLANAttachmentOptions) (*VLANAttachment, error)
	CreateVMAttachmentFn   func(ctx context.Context, opts CreateVMAttachmentOptions) (*VMAttachmentResult, error)
	DeleteVLANAttachmentFn func(ctx context.Context, bmServerID, attachmentID string) error
	ListVLANAttachmentsFn  func(ctx context.Context, bmServerID string) ([]VLANAttachment, error)

	// Floating IP operations
	CreateFloatingIPFn func(ctx context.Context, opts CreateFloatingIPOptions) (*FloatingIP, error)
	GetFloatingIPFn    func(ctx context.Context, fipID string) (*FloatingIP, error)
	UpdateFloatingIPFn func(ctx context.Context, fipID string, opts UpdateFloatingIPOptions) (*FloatingIP, error)
	DeleteFloatingIPFn func(ctx context.Context, fipID string) error

	// Security Group operations
	ListSecurityGroupsFn      func(ctx context.Context, vpcID string) ([]SecurityGroup, error)
	GetSecurityGroupFn        func(ctx context.Context, sgID string) (*SecurityGroup, error)
	CreateSecurityGroupFn     func(ctx context.Context, opts CreateSecurityGroupOptions) (*SecurityGroup, error)
	DeleteSecurityGroupFn     func(ctx context.Context, sgID string) error
	UpdateSecurityGroupFn     func(ctx context.Context, sgID string, opts UpdateSecurityGroupOptions) (*SecurityGroup, error)
	AddSecurityGroupRuleFn    func(ctx context.Context, sgID string, opts CreateSGRuleOptions) (*SecurityGroupRule, error)
	UpdateSecurityGroupRuleFn func(ctx context.Context, sgID, ruleID string, opts UpdateSGRuleOptions) (*SecurityGroupRule, error)
	DeleteSecurityGroupRuleFn func(ctx context.Context, sgID, ruleID string) error

	// Network ACL operations
	ListNetworkACLsFn      func(ctx context.Context, vpcID string) ([]NetworkACL, error)
	GetNetworkACLFn        func(ctx context.Context, aclID string) (*NetworkACL, error)
	CreateNetworkACLFn     func(ctx context.Context, opts CreateNetworkACLOptions) (*NetworkACL, error)
	DeleteNetworkACLFn     func(ctx context.Context, aclID string) error
	UpdateNetworkACLFn     func(ctx context.Context, aclID string, opts UpdateNetworkACLOptions) (*NetworkACL, error)
	AddNetworkACLRuleFn    func(ctx context.Context, aclID string, opts CreateACLRuleOptions) (*NetworkACLRule, error)
	UpdateNetworkACLRuleFn func(ctx context.Context, aclID, ruleID string, opts UpdateACLRuleOptions) (*NetworkACLRule, error)
	DeleteNetworkACLRuleFn func(ctx context.Context, aclID, ruleID string) error

	// VPC operations
	ListVPCsFn func(ctx context.Context) ([]VPC, error)
	GetVPCFn   func(ctx context.Context, vpcID string) (*VPC, error)

	// Subnet listing
	ListSubnetsFn func(ctx context.Context, vpcID string) ([]Subnet, error)

	// VNI listing
	ListVNIsFn func(ctx context.Context) ([]VNI, error)

	// Floating IP listing
	ListFloatingIPsFn func(ctx context.Context) ([]FloatingIP, error)

	// Zone operations
	ListZonesFn func(ctx context.Context, region string) ([]Zone, error)

	// Routing Table operations
	ListRoutingTablesFn func(ctx context.Context, vpcID string) ([]RoutingTable, error)
	GetRoutingTableFn   func(ctx context.Context, vpcID, routingTableID string) (*RoutingTable, error)

	// Route operations
	ListRoutesFn  func(ctx context.Context, vpcID, routingTableID string) ([]Route, error)
	CreateRouteFn func(ctx context.Context, vpcID, routingTableID string, opts CreateRouteOptions) (*Route, error)
	DeleteRouteFn func(ctx context.Context, vpcID, routingTableID, routeID string) error

	// Subnet Reserved IP operations
	ListSubnetReservedIPsFn func(ctx context.Context, subnetID string) ([]ReservedIP, error)

	// Address Prefix operations
	ListVPCAddressPrefixesFn  func(ctx context.Context, vpcID string) ([]AddressPrefix, error)
	CreateVPCAddressPrefixFn  func(ctx context.Context, opts CreateAddressPrefixOptions) (*AddressPrefix, error)

	// PCI allowed VLAN operations
	EnsurePCIAllowedVLANFn func(ctx context.Context, bmServerID string, vlanID int64) error

	// Bare Metal Server operations
	ListBareMetalServersFn func(ctx context.Context, vpcID string) ([]BareMetalServerInfo, error)

	// Public Gateway operations
	ListPublicGatewaysFn func(ctx context.Context, vpcID string) ([]PublicGateway, error)
}

// NewMockClient creates a new MockClient with default error implementations.
func NewMockClient() *MockClient {
	return &MockClient{
		Calls: make(map[string]int),
	}
}

func (m *MockClient) trackCall(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Calls[name]++
}

// CallCount returns the number of times a method was called.
func (m *MockClient) CallCount(name string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.Calls[name]
}

// Subnet operations
func (m *MockClient) CreateSubnet(ctx context.Context, opts CreateSubnetOptions) (*Subnet, error) {
	m.trackCall("CreateSubnet")
	if m.CreateSubnetFn != nil {
		return m.CreateSubnetFn(ctx, opts)
	}
	return nil, fmt.Errorf("CreateSubnet not configured in mock")
}

func (m *MockClient) GetSubnet(ctx context.Context, subnetID string) (*Subnet, error) {
	m.trackCall("GetSubnet")
	if m.GetSubnetFn != nil {
		return m.GetSubnetFn(ctx, subnetID)
	}
	return nil, fmt.Errorf("GetSubnet not configured in mock")
}

func (m *MockClient) DeleteSubnet(ctx context.Context, subnetID string) error {
	m.trackCall("DeleteSubnet")
	if m.DeleteSubnetFn != nil {
		return m.DeleteSubnetFn(ctx, subnetID)
	}
	return fmt.Errorf("DeleteSubnet not configured in mock")
}

// VNI operations
func (m *MockClient) CreateVNI(ctx context.Context, opts CreateVNIOptions) (*VNI, error) {
	m.trackCall("CreateVNI")
	if m.CreateVNIFn != nil {
		return m.CreateVNIFn(ctx, opts)
	}
	return nil, fmt.Errorf("CreateVNI not configured in mock")
}

func (m *MockClient) GetVNI(ctx context.Context, vniID string) (*VNI, error) {
	m.trackCall("GetVNI")
	if m.GetVNIFn != nil {
		return m.GetVNIFn(ctx, vniID)
	}
	return nil, fmt.Errorf("GetVNI not configured in mock")
}

func (m *MockClient) UpdateVNI(ctx context.Context, vniID, name string) (*VNI, error) {
	m.trackCall("UpdateVNI")
	if m.UpdateVNIFn != nil {
		return m.UpdateVNIFn(ctx, vniID, name)
	}
	return nil, fmt.Errorf("UpdateVNI not configured in mock")
}

func (m *MockClient) DeleteVNI(ctx context.Context, vniID string) error {
	m.trackCall("DeleteVNI")
	if m.DeleteVNIFn != nil {
		return m.DeleteVNIFn(ctx, vniID)
	}
	return fmt.Errorf("DeleteVNI not configured in mock")
}

func (m *MockClient) ListVNIsByTag(ctx context.Context, clusterID, namespace, vmName string) ([]VNI, error) {
	m.trackCall("ListVNIsByTag")
	if m.ListVNIsByTagFn != nil {
		return m.ListVNIsByTagFn(ctx, clusterID, namespace, vmName)
	}
	return nil, fmt.Errorf("ListVNIsByTag not configured in mock")
}

// VLAN Attachment operations
func (m *MockClient) CreateVLANAttachment(ctx context.Context, opts CreateVLANAttachmentOptions) (*VLANAttachment, error) {
	m.trackCall("CreateVLANAttachment")
	if m.CreateVLANAttachmentFn != nil {
		return m.CreateVLANAttachmentFn(ctx, opts)
	}
	return nil, fmt.Errorf("CreateVLANAttachment not configured in mock")
}

func (m *MockClient) CreateVMAttachment(ctx context.Context, opts CreateVMAttachmentOptions) (*VMAttachmentResult, error) {
	m.trackCall("CreateVMAttachment")
	if m.CreateVMAttachmentFn != nil {
		return m.CreateVMAttachmentFn(ctx, opts)
	}
	return nil, fmt.Errorf("CreateVMAttachment not configured in mock")
}

func (m *MockClient) DeleteVLANAttachment(ctx context.Context, bmServerID, attachmentID string) error {
	m.trackCall("DeleteVLANAttachment")
	if m.DeleteVLANAttachmentFn != nil {
		return m.DeleteVLANAttachmentFn(ctx, bmServerID, attachmentID)
	}
	return fmt.Errorf("DeleteVLANAttachment not configured in mock")
}

func (m *MockClient) ListVLANAttachments(ctx context.Context, bmServerID string) ([]VLANAttachment, error) {
	m.trackCall("ListVLANAttachments")
	if m.ListVLANAttachmentsFn != nil {
		return m.ListVLANAttachmentsFn(ctx, bmServerID)
	}
	return nil, fmt.Errorf("ListVLANAttachments not configured in mock")
}

// Floating IP operations
func (m *MockClient) CreateFloatingIP(ctx context.Context, opts CreateFloatingIPOptions) (*FloatingIP, error) {
	m.trackCall("CreateFloatingIP")
	if m.CreateFloatingIPFn != nil {
		return m.CreateFloatingIPFn(ctx, opts)
	}
	return nil, fmt.Errorf("CreateFloatingIP not configured in mock")
}

func (m *MockClient) GetFloatingIP(ctx context.Context, fipID string) (*FloatingIP, error) {
	m.trackCall("GetFloatingIP")
	if m.GetFloatingIPFn != nil {
		return m.GetFloatingIPFn(ctx, fipID)
	}
	return nil, fmt.Errorf("GetFloatingIP not configured in mock")
}

func (m *MockClient) UpdateFloatingIP(ctx context.Context, fipID string, opts UpdateFloatingIPOptions) (*FloatingIP, error) {
	m.trackCall("UpdateFloatingIP")
	if m.UpdateFloatingIPFn != nil {
		return m.UpdateFloatingIPFn(ctx, fipID, opts)
	}
	return nil, fmt.Errorf("UpdateFloatingIP not configured in mock")
}

func (m *MockClient) DeleteFloatingIP(ctx context.Context, fipID string) error {
	m.trackCall("DeleteFloatingIP")
	if m.DeleteFloatingIPFn != nil {
		return m.DeleteFloatingIPFn(ctx, fipID)
	}
	return fmt.Errorf("DeleteFloatingIP not configured in mock")
}

// Security Group operations
func (m *MockClient) ListSecurityGroups(ctx context.Context, vpcID string) ([]SecurityGroup, error) {
	m.trackCall("ListSecurityGroups")
	if m.ListSecurityGroupsFn != nil {
		return m.ListSecurityGroupsFn(ctx, vpcID)
	}
	return nil, fmt.Errorf("ListSecurityGroups not configured in mock")
}

func (m *MockClient) GetSecurityGroup(ctx context.Context, sgID string) (*SecurityGroup, error) {
	m.trackCall("GetSecurityGroup")
	if m.GetSecurityGroupFn != nil {
		return m.GetSecurityGroupFn(ctx, sgID)
	}
	return nil, fmt.Errorf("GetSecurityGroup not configured in mock")
}

func (m *MockClient) CreateSecurityGroup(ctx context.Context, opts CreateSecurityGroupOptions) (*SecurityGroup, error) {
	m.trackCall("CreateSecurityGroup")
	if m.CreateSecurityGroupFn != nil {
		return m.CreateSecurityGroupFn(ctx, opts)
	}
	return nil, fmt.Errorf("CreateSecurityGroup not configured in mock")
}

func (m *MockClient) DeleteSecurityGroup(ctx context.Context, sgID string) error {
	m.trackCall("DeleteSecurityGroup")
	if m.DeleteSecurityGroupFn != nil {
		return m.DeleteSecurityGroupFn(ctx, sgID)
	}
	return fmt.Errorf("DeleteSecurityGroup not configured in mock")
}

func (m *MockClient) UpdateSecurityGroup(ctx context.Context, sgID string, opts UpdateSecurityGroupOptions) (*SecurityGroup, error) {
	m.trackCall("UpdateSecurityGroup")
	if m.UpdateSecurityGroupFn != nil {
		return m.UpdateSecurityGroupFn(ctx, sgID, opts)
	}
	return nil, fmt.Errorf("UpdateSecurityGroup not configured in mock")
}

func (m *MockClient) AddSecurityGroupRule(ctx context.Context, sgID string, opts CreateSGRuleOptions) (*SecurityGroupRule, error) {
	m.trackCall("AddSecurityGroupRule")
	if m.AddSecurityGroupRuleFn != nil {
		return m.AddSecurityGroupRuleFn(ctx, sgID, opts)
	}
	return nil, fmt.Errorf("AddSecurityGroupRule not configured in mock")
}

func (m *MockClient) UpdateSecurityGroupRule(ctx context.Context, sgID, ruleID string, opts UpdateSGRuleOptions) (*SecurityGroupRule, error) {
	m.trackCall("UpdateSecurityGroupRule")
	if m.UpdateSecurityGroupRuleFn != nil {
		return m.UpdateSecurityGroupRuleFn(ctx, sgID, ruleID, opts)
	}
	return nil, fmt.Errorf("UpdateSecurityGroupRule not configured in mock")
}

func (m *MockClient) DeleteSecurityGroupRule(ctx context.Context, sgID, ruleID string) error {
	m.trackCall("DeleteSecurityGroupRule")
	if m.DeleteSecurityGroupRuleFn != nil {
		return m.DeleteSecurityGroupRuleFn(ctx, sgID, ruleID)
	}
	return fmt.Errorf("DeleteSecurityGroupRule not configured in mock")
}

// Network ACL operations
func (m *MockClient) ListNetworkACLs(ctx context.Context, vpcID string) ([]NetworkACL, error) {
	m.trackCall("ListNetworkACLs")
	if m.ListNetworkACLsFn != nil {
		return m.ListNetworkACLsFn(ctx, vpcID)
	}
	return nil, fmt.Errorf("ListNetworkACLs not configured in mock")
}

func (m *MockClient) GetNetworkACL(ctx context.Context, aclID string) (*NetworkACL, error) {
	m.trackCall("GetNetworkACL")
	if m.GetNetworkACLFn != nil {
		return m.GetNetworkACLFn(ctx, aclID)
	}
	return nil, fmt.Errorf("GetNetworkACL not configured in mock")
}

func (m *MockClient) CreateNetworkACL(ctx context.Context, opts CreateNetworkACLOptions) (*NetworkACL, error) {
	m.trackCall("CreateNetworkACL")
	if m.CreateNetworkACLFn != nil {
		return m.CreateNetworkACLFn(ctx, opts)
	}
	return nil, fmt.Errorf("CreateNetworkACL not configured in mock")
}

func (m *MockClient) DeleteNetworkACL(ctx context.Context, aclID string) error {
	m.trackCall("DeleteNetworkACL")
	if m.DeleteNetworkACLFn != nil {
		return m.DeleteNetworkACLFn(ctx, aclID)
	}
	return fmt.Errorf("DeleteNetworkACL not configured in mock")
}

func (m *MockClient) UpdateNetworkACL(ctx context.Context, aclID string, opts UpdateNetworkACLOptions) (*NetworkACL, error) {
	m.trackCall("UpdateNetworkACL")
	if m.UpdateNetworkACLFn != nil {
		return m.UpdateNetworkACLFn(ctx, aclID, opts)
	}
	return nil, fmt.Errorf("UpdateNetworkACL not configured in mock")
}

func (m *MockClient) AddNetworkACLRule(ctx context.Context, aclID string, opts CreateACLRuleOptions) (*NetworkACLRule, error) {
	m.trackCall("AddNetworkACLRule")
	if m.AddNetworkACLRuleFn != nil {
		return m.AddNetworkACLRuleFn(ctx, aclID, opts)
	}
	return nil, fmt.Errorf("AddNetworkACLRule not configured in mock")
}

func (m *MockClient) UpdateNetworkACLRule(ctx context.Context, aclID, ruleID string, opts UpdateACLRuleOptions) (*NetworkACLRule, error) {
	m.trackCall("UpdateNetworkACLRule")
	if m.UpdateNetworkACLRuleFn != nil {
		return m.UpdateNetworkACLRuleFn(ctx, aclID, ruleID, opts)
	}
	return nil, fmt.Errorf("UpdateNetworkACLRule not configured in mock")
}

func (m *MockClient) DeleteNetworkACLRule(ctx context.Context, aclID, ruleID string) error {
	m.trackCall("DeleteNetworkACLRule")
	if m.DeleteNetworkACLRuleFn != nil {
		return m.DeleteNetworkACLRuleFn(ctx, aclID, ruleID)
	}
	return fmt.Errorf("DeleteNetworkACLRule not configured in mock")
}

// VPC operations
func (m *MockClient) ListVPCs(ctx context.Context) ([]VPC, error) {
	m.trackCall("ListVPCs")
	if m.ListVPCsFn != nil {
		return m.ListVPCsFn(ctx)
	}
	return nil, fmt.Errorf("ListVPCs not configured in mock")
}

func (m *MockClient) GetVPC(ctx context.Context, vpcID string) (*VPC, error) {
	m.trackCall("GetVPC")
	if m.GetVPCFn != nil {
		return m.GetVPCFn(ctx, vpcID)
	}
	return nil, fmt.Errorf("GetVPC not configured in mock")
}

// Subnet listing
func (m *MockClient) ListSubnets(ctx context.Context, vpcID string) ([]Subnet, error) {
	m.trackCall("ListSubnets")
	if m.ListSubnetsFn != nil {
		return m.ListSubnetsFn(ctx, vpcID)
	}
	return nil, fmt.Errorf("ListSubnets not configured in mock")
}

// VNI listing
func (m *MockClient) ListVNIs(ctx context.Context) ([]VNI, error) {
	m.trackCall("ListVNIs")
	if m.ListVNIsFn != nil {
		return m.ListVNIsFn(ctx)
	}
	return nil, fmt.Errorf("ListVNIs not configured in mock")
}

// Floating IP listing
func (m *MockClient) ListFloatingIPs(ctx context.Context) ([]FloatingIP, error) {
	m.trackCall("ListFloatingIPs")
	if m.ListFloatingIPsFn != nil {
		return m.ListFloatingIPsFn(ctx)
	}
	return nil, fmt.Errorf("ListFloatingIPs not configured in mock")
}

// Zone operations
func (m *MockClient) ListZones(ctx context.Context, region string) ([]Zone, error) {
	m.trackCall("ListZones")
	if m.ListZonesFn != nil {
		return m.ListZonesFn(ctx, region)
	}
	return nil, fmt.Errorf("ListZones not configured in mock")
}

// Routing Table operations
func (m *MockClient) ListRoutingTables(ctx context.Context, vpcID string) ([]RoutingTable, error) {
	m.trackCall("ListRoutingTables")
	if m.ListRoutingTablesFn != nil {
		return m.ListRoutingTablesFn(ctx, vpcID)
	}
	return nil, fmt.Errorf("ListRoutingTables not configured in mock")
}

func (m *MockClient) GetRoutingTable(ctx context.Context, vpcID, routingTableID string) (*RoutingTable, error) {
	m.trackCall("GetRoutingTable")
	if m.GetRoutingTableFn != nil {
		return m.GetRoutingTableFn(ctx, vpcID, routingTableID)
	}
	return nil, fmt.Errorf("GetRoutingTable not configured in mock")
}

// Route operations
func (m *MockClient) ListRoutes(ctx context.Context, vpcID, routingTableID string) ([]Route, error) {
	m.trackCall("ListRoutes")
	if m.ListRoutesFn != nil {
		return m.ListRoutesFn(ctx, vpcID, routingTableID)
	}
	return nil, fmt.Errorf("ListRoutes not configured in mock")
}

func (m *MockClient) CreateRoute(ctx context.Context, vpcID, routingTableID string, opts CreateRouteOptions) (*Route, error) {
	m.trackCall("CreateRoute")
	if m.CreateRouteFn != nil {
		return m.CreateRouteFn(ctx, vpcID, routingTableID, opts)
	}
	return nil, fmt.Errorf("CreateRoute not configured in mock")
}

func (m *MockClient) DeleteRoute(ctx context.Context, vpcID, routingTableID, routeID string) error {
	m.trackCall("DeleteRoute")
	if m.DeleteRouteFn != nil {
		return m.DeleteRouteFn(ctx, vpcID, routingTableID, routeID)
	}
	return fmt.Errorf("DeleteRoute not configured in mock")
}

// Subnet Reserved IP operations
func (m *MockClient) ListSubnetReservedIPs(ctx context.Context, subnetID string) ([]ReservedIP, error) {
	m.trackCall("ListSubnetReservedIPs")
	if m.ListSubnetReservedIPsFn != nil {
		return m.ListSubnetReservedIPsFn(ctx, subnetID)
	}
	return nil, fmt.Errorf("ListSubnetReservedIPs not configured in mock")
}

// Address Prefix operations
func (m *MockClient) ListVPCAddressPrefixes(ctx context.Context, vpcID string) ([]AddressPrefix, error) {
	m.trackCall("ListVPCAddressPrefixes")
	if m.ListVPCAddressPrefixesFn != nil {
		return m.ListVPCAddressPrefixesFn(ctx, vpcID)
	}
	return nil, fmt.Errorf("ListVPCAddressPrefixes not configured in mock")
}

func (m *MockClient) CreateVPCAddressPrefix(ctx context.Context, opts CreateAddressPrefixOptions) (*AddressPrefix, error) {
	m.trackCall("CreateVPCAddressPrefix")
	if m.CreateVPCAddressPrefixFn != nil {
		return m.CreateVPCAddressPrefixFn(ctx, opts)
	}
	return nil, fmt.Errorf("CreateVPCAddressPrefix not configured in mock")
}

// PCI allowed VLAN operations
func (m *MockClient) EnsurePCIAllowedVLAN(ctx context.Context, bmServerID string, vlanID int64) error {
	m.trackCall("EnsurePCIAllowedVLAN")
	if m.EnsurePCIAllowedVLANFn != nil {
		return m.EnsurePCIAllowedVLANFn(ctx, bmServerID, vlanID)
	}
	return nil // Default: no-op (VLAN already allowed)
}

// Bare Metal Server operations
func (m *MockClient) ListBareMetalServers(ctx context.Context, vpcID string) ([]BareMetalServerInfo, error) {
	m.trackCall("ListBareMetalServers")
	if m.ListBareMetalServersFn != nil {
		return m.ListBareMetalServersFn(ctx, vpcID)
	}
	return nil, fmt.Errorf("ListBareMetalServers not configured in mock")
}

// Public Gateway operations
func (m *MockClient) ListPublicGateways(ctx context.Context, vpcID string) ([]PublicGateway, error) {
	m.trackCall("ListPublicGateways")
	if m.ListPublicGatewaysFn != nil {
		return m.ListPublicGatewaysFn(ctx, vpcID)
	}
	return nil, fmt.Errorf("ListPublicGateways not configured in mock")
}

// Compile-time interface checks
var _ Client = (*MockClient)(nil)
var _ ExtendedClient = (*MockClient)(nil)
