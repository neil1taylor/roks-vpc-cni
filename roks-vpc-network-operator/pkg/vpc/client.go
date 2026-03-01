package vpc

import (
	"context"
	"fmt"

	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/vpc-go-sdk/vpcv1"
)

// Client defines the interface for all VPC operations used by the operator.
// This interface is the primary mock boundary for unit testing.
type Client interface {
	SubnetService
	VNIService
	VLANAttachmentService
	FloatingIPService
	AddressPrefixService
	BareMetalServerService
	SubnetReservedIPService
	RoutingTableService
	RouteService
	PublicAddressRangeService
}

// BareMetalServerService handles listing VPC bare metal servers.
type BareMetalServerService interface {
	ListBareMetalServers(ctx context.Context, vpcID string) ([]BareMetalServerInfo, error)
}

// BareMetalServerInfo holds the ID and name of a VPC bare metal server.
type BareMetalServerInfo struct {
	ID   string
	Name string
	Zone string
}

// ExtendedClient adds security group, ACL, VPC, and zone operations
// used by the BFF service for console plugin data.
type ExtendedClient interface {
	Client
	SecurityGroupService
	NetworkACLService
	VPCService
	ZoneService
	VNILister
	FloatingIPLister
	PublicGatewayService
}

// VNILister lists all VNIs in the account.
type VNILister interface {
	ListVNIs(ctx context.Context) ([]VNI, error)
}

// FloatingIPLister lists all floating IPs in the account.
type FloatingIPLister interface {
	ListFloatingIPs(ctx context.Context) ([]FloatingIP, error)
}

// PublicGatewayService handles listing public gateways (read-only, BFF use).
type PublicGatewayService interface {
	ListPublicGateways(ctx context.Context, vpcID string) ([]PublicGateway, error)
}

// SubnetService handles VPC subnet CRUD.
type SubnetService interface {
	CreateSubnet(ctx context.Context, opts CreateSubnetOptions) (*Subnet, error)
	GetSubnet(ctx context.Context, subnetID string) (*Subnet, error)
	DeleteSubnet(ctx context.Context, subnetID string) error
	ListSubnets(ctx context.Context, vpcID string) ([]Subnet, error)
}

// VNIService handles Virtual Network Interface CRUD.
type VNIService interface {
	CreateVNI(ctx context.Context, opts CreateVNIOptions) (*VNI, error)
	GetVNI(ctx context.Context, vniID string) (*VNI, error)
	UpdateVNI(ctx context.Context, vniID, name string) (*VNI, error)
	DeleteVNI(ctx context.Context, vniID string) error
	ListVNIsByTag(ctx context.Context, clusterID, namespace, vmName string) ([]VNI, error)
}

// VLANAttachmentService handles bare metal VLAN attachment CRUD.
type VLANAttachmentService interface {
	CreateVLANAttachment(ctx context.Context, opts CreateVLANAttachmentOptions) (*VLANAttachment, error)
	CreateVMAttachment(ctx context.Context, opts CreateVMAttachmentOptions) (*VMAttachmentResult, error)
	DeleteVLANAttachment(ctx context.Context, bmServerID, attachmentID string) error
	ListVLANAttachments(ctx context.Context, bmServerID string) ([]VLANAttachment, error)
	EnsurePCIAllowedVLAN(ctx context.Context, bmServerID string, vlanID int64) error
}

// FloatingIPService handles floating IP CRUD.
type FloatingIPService interface {
	CreateFloatingIP(ctx context.Context, opts CreateFloatingIPOptions) (*FloatingIP, error)
	GetFloatingIP(ctx context.Context, fipID string) (*FloatingIP, error)
	UpdateFloatingIP(ctx context.Context, fipID string, opts UpdateFloatingIPOptions) (*FloatingIP, error)
	DeleteFloatingIP(ctx context.Context, fipID string) error
}

// SecurityGroupService handles VPC security group and rule CRUD.
// Used by the BFF service for console plugin SG management.
type SecurityGroupService interface {
	ListSecurityGroups(ctx context.Context, vpcID string) ([]SecurityGroup, error)
	GetSecurityGroup(ctx context.Context, sgID string) (*SecurityGroup, error)
	CreateSecurityGroup(ctx context.Context, opts CreateSecurityGroupOptions) (*SecurityGroup, error)
	DeleteSecurityGroup(ctx context.Context, sgID string) error
	UpdateSecurityGroup(ctx context.Context, sgID string, opts UpdateSecurityGroupOptions) (*SecurityGroup, error)
	// Rule operations
	AddSecurityGroupRule(ctx context.Context, sgID string, opts CreateSGRuleOptions) (*SecurityGroupRule, error)
	UpdateSecurityGroupRule(ctx context.Context, sgID, ruleID string, opts UpdateSGRuleOptions) (*SecurityGroupRule, error)
	DeleteSecurityGroupRule(ctx context.Context, sgID, ruleID string) error
}

// NetworkACLService handles VPC network ACL and rule CRUD.
// Used by the BFF service for console plugin ACL management.
type NetworkACLService interface {
	ListNetworkACLs(ctx context.Context, vpcID string) ([]NetworkACL, error)
	GetNetworkACL(ctx context.Context, aclID string) (*NetworkACL, error)
	CreateNetworkACL(ctx context.Context, opts CreateNetworkACLOptions) (*NetworkACL, error)
	DeleteNetworkACL(ctx context.Context, aclID string) error
	UpdateNetworkACL(ctx context.Context, aclID string, opts UpdateNetworkACLOptions) (*NetworkACL, error)
	// Rule operations
	AddNetworkACLRule(ctx context.Context, aclID string, opts CreateACLRuleOptions) (*NetworkACLRule, error)
	UpdateNetworkACLRule(ctx context.Context, aclID, ruleID string, opts UpdateACLRuleOptions) (*NetworkACLRule, error)
	DeleteNetworkACLRule(ctx context.Context, aclID, ruleID string) error
}

// VPCService handles listing VPCs in the account.
type VPCService interface {
	ListVPCs(ctx context.Context) ([]VPC, error)
	GetVPC(ctx context.Context, vpcID string) (*VPC, error)
}

// ZoneService handles listing available zones.
type ZoneService interface {
	ListZones(ctx context.Context, region string) ([]Zone, error)
}

// RoutingTableService handles VPC routing table operations.
type RoutingTableService interface {
	ListRoutingTables(ctx context.Context, vpcID string) ([]RoutingTable, error)
	GetRoutingTable(ctx context.Context, vpcID, routingTableID string) (*RoutingTable, error)
	CreateRoutingTable(ctx context.Context, vpcID string, opts CreateRoutingTableOptions) (*RoutingTable, error)
	DeleteRoutingTable(ctx context.Context, vpcID, routingTableID string) error
}

// PublicAddressRangeService handles VPC public address range CRUD.
type PublicAddressRangeService interface {
	CreatePublicAddressRange(ctx context.Context, opts CreatePublicAddressRangeOptions) (*PublicAddressRange, error)
	GetPublicAddressRange(ctx context.Context, parID string) (*PublicAddressRange, error)
	ListPublicAddressRanges(ctx context.Context, vpcID string) ([]PublicAddressRange, error)
	DeletePublicAddressRange(ctx context.Context, parID string) error
}

// RouteService handles VPC route CRUD.
type RouteService interface {
	ListRoutes(ctx context.Context, vpcID, routingTableID string) ([]Route, error)
	CreateRoute(ctx context.Context, vpcID, routingTableID string, opts CreateRouteOptions) (*Route, error)
	DeleteRoute(ctx context.Context, vpcID, routingTableID, routeID string) error
}

// AddressPrefixService handles VPC address prefix operations.
type AddressPrefixService interface {
	ListVPCAddressPrefixes(ctx context.Context, vpcID string) ([]AddressPrefix, error)
	CreateVPCAddressPrefix(ctx context.Context, opts CreateAddressPrefixOptions) (*AddressPrefix, error)
}

// CreateAddressPrefixOptions holds parameters for creating a VPC address prefix.
type CreateAddressPrefixOptions struct {
	VPCID string
	CIDR  string
	Zone  string
	Name  string
}

// SubnetReservedIPService lists reserved IPs for a subnet.
type SubnetReservedIPService interface {
	ListSubnetReservedIPs(ctx context.Context, subnetID string) ([]ReservedIP, error)
}

// ── Option types ──

type CreateSubnetOptions struct {
	Name            string
	VPCID           string
	Zone            string
	CIDR            string
	ACLID           string
	PublicGatewayID string // optional: attach pre-existing PGW for outbound internet
	ResourceGroupID string
	ClusterID       string // for tagging
	CUDNName        string // for tagging
}

type CreateVNIOptions struct {
	Name                    string
	SubnetID                string
	SecurityGroupIDs        []string
	EnableInfrastructureNat *bool  // nil defaults to true for backward compat
	ClusterID               string // for tagging
	Namespace               string // for tagging
	VMName                  string // for tagging
}

type CreateVLANAttachmentOptions struct {
	BMServerID string
	Name       string
	VLANID     int64
	SubnetID   string
}

// CreateVMAttachmentOptions holds parameters for creating a per-VM VLAN
// attachment with an inline VNI on a bare metal server.
type CreateVMAttachmentOptions struct {
	BMServerID       string
	Name             string
	VLANID           int64
	SubnetID         string
	VNIName          string
	SecurityGroupIDs []string
}

// VMAttachmentResult holds the result of creating a per-VM VLAN attachment
// including the inline VNI details (populated via GetVNI follow-up).
type VMAttachmentResult struct {
	AttachmentID string
	BMServerID   string
	VNI          VNI
}

type CreateFloatingIPOptions struct {
	Name   string
	Zone   string
	VNIID  string
}

type UpdateFloatingIPOptions struct {
	TargetID string // VNI ID to bind; empty string to unbind
}

// ── Response types ──

type Subnet struct {
	ID                        string
	Name                      string
	CIDR                      string
	Status                    string
	VPCID                     string
	VPCName                   string
	Zone                      string
	NetworkACLID              string
	NetworkACLName            string
	AvailableIPv4AddressCount int64
	TotalIPv4AddressCount     int64
	CreatedAt                 string
}

type VNI struct {
	ID                      string
	Name                    string
	MACAddress              string
	PrimaryIP               ReservedIP
	SubnetID                string
	SubnetName              string
	AllowIPSpoofing         bool
	EnableInfrastructureNat bool
	Status                  string
	CreatedAt               string
}

type ReservedIP struct {
	ID         string
	Address    string
	Name       string
	AutoDelete bool
	CreatedAt  string
	Owner      string // "user", "provider", etc.
	Target     string // VNI name or resource name if bound
	TargetID   string
}

type VLANAttachment struct {
	ID         string
	Name       string
	VLANID     int64
	BMServerID string
}

type FloatingIP struct {
	ID         string
	Name       string
	Address    string
	Zone       string
	Target     string // VNI ID if bound
	TargetName string
	Status     string
	CreatedAt  string
}

// PublicGateway represents a VPC public gateway.
type PublicGateway struct {
	ID                string
	Name              string
	Status            string
	Zone              string
	VPCID             string
	VPCName           string
	FloatingIPID      string
	FloatingIPAddress string
	ResourceGroupID   string
	ResourceGroupName string
	CreatedAt         string
}

// ── Security Group types ──

type SecurityGroup struct {
	ID          string
	Name        string
	Description string
	VPCID       string
	Rules       []SecurityGroupRule
	Tags        []string
	CreatedAt   string
}

type SecurityGroupRule struct {
	ID        string
	Direction string // "inbound" or "outbound"
	Protocol  string // "tcp", "udp", "icmp", "all"
	PortMin   *int64
	PortMax   *int64
	ICMPType  *int64
	ICMPCode  *int64
	Remote    SecurityGroupRuleRemote
}

type SecurityGroupRuleRemote struct {
	CIDRBlock       string
	SecurityGroupID string
}

type CreateSecurityGroupOptions struct {
	Name        string
	VPCID       string
	Description string
}

type UpdateSecurityGroupOptions struct {
	Name        *string
	Description *string
}

type CreateSGRuleOptions struct {
	Direction string
	Protocol  string
	PortMin   *int64
	PortMax   *int64
	ICMPType  *int64
	ICMPCode  *int64
	RemoteCIDR      string
	RemoteSGID      string
}

type UpdateSGRuleOptions struct {
	Direction  *string
	PortMin    *int64
	PortMax    *int64
	ICMPType   *int64
	ICMPCode   *int64
	RemoteCIDR *string
	RemoteSGID *string
}

// ── Network ACL types ──

type NetworkACL struct {
	ID        string
	Name      string
	VPCID     string
	Subnets   []string // Associated subnet IDs
	Rules     []NetworkACLRule
	CreatedAt string
}

type NetworkACLRule struct {
	ID          string
	Name        string
	Direction   string // "inbound" or "outbound"
	Action      string // "allow" or "deny"
	Protocol    string // "tcp", "udp", "icmp", "all"
	Source      string // CIDR
	Destination string // CIDR
	PortMin     *int64
	PortMax     *int64
	ICMPType    *int64
	ICMPCode    *int64
	Priority    int64
}

type CreateNetworkACLOptions struct {
	Name  string
	VPCID string
}

type UpdateNetworkACLOptions struct {
	Name *string
}

type CreateACLRuleOptions struct {
	Name        string
	Direction   string
	Action      string
	Protocol    string
	Source      string
	Destination string
	PortMin     *int64
	PortMax     *int64
	ICMPType    *int64
	ICMPCode    *int64
}

type UpdateACLRuleOptions struct {
	Name        *string
	Direction   *string
	Action      *string
	PortMin     *int64
	PortMax     *int64
	Source      *string
	Destination *string
}

// ── VPC and Zone types ──

type VPC struct {
	ID              string
	Name            string
	Status          string
	Region          string
	DefaultSGID     string
	DefaultACLID    string
	ResourceGroupID string
	CreatedAt       string
}

type Zone struct {
	Name   string
	Region string
	Status string
}

// ── Address Prefix types ──

type AddressPrefix struct {
	ID        string
	Name      string
	CIDR      string
	Zone      string
	IsDefault bool
}

// ── Public Address Range types ──

// PublicAddressRange represents a VPC public address range.
type PublicAddressRange struct {
	ID             string
	Name           string
	CIDR           string
	Zone           string
	VPCID          string
	LifecycleState string
	CreatedAt      string
}

// CreatePublicAddressRangeOptions holds parameters for creating a PAR.
type CreatePublicAddressRangeOptions struct {
	Name         string
	VPCID        string
	Zone         string
	PrefixLength int // CIDR prefix: 28, 29, 30, 31, or 32
}

// ── Routing Table and Route types ──

// CreateRoutingTableOptions holds parameters for creating a routing table.
type CreateRoutingTableOptions struct {
	Name                 string
	RouteInternetIngress bool // true = ingress routing table for PAR traffic
}

type RoutingTable struct {
	ID                   string
	Name                 string
	IsDefault            bool
	RouteInternetIngress bool
	LifecycleState       string
	RouteCount           int
	CreatedAt            string
}

type Route struct {
	ID             string
	Name           string
	Destination    string
	Action         string // "delegate", "delegate_vpc", "deliver", "drop"
	NextHop        string // IP address for "deliver" action
	Zone           string
	Priority       int64
	Origin         string // "service" or "user"
	LifecycleState string
	CreatedAt      string
}

type CreateRouteOptions struct {
	Name        string
	Destination string
	Action      string
	NextHopIP   string
	Zone        string
	Priority    *int64
}

// ── Implementation ──

// vpcClient implements Client using the IBM VPC Go SDK.
type vpcClient struct {
	service         *vpcv1.VpcV1
	resourceGroupID string
	clusterID       string
	limiter         *RateLimiter
}

// ClientConfig holds configuration for creating a VPC client.
type ClientConfig struct {
	APIKey          string
	Region          string
	ResourceGroupID string
	ClusterID       string
	MaxConcurrent   int // max concurrent VPC API calls (default 10)
}

// NewExtendedClient creates a VPC API client with all services including SG, ACL, VPC, Zone.
// Used by the BFF service.
func NewExtendedClient(cfg ClientConfig) (ExtendedClient, error) {
	c, err := NewClient(cfg)
	if err != nil {
		return nil, err
	}
	return c.(*vpcClient), nil
}

// NewClient creates a new VPC API client.
func NewClient(cfg ClientConfig) (Client, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("VPC API key is required")
	}
	if cfg.Region == "" {
		return nil, fmt.Errorf("VPC region is required")
	}

	maxConcurrent := cfg.MaxConcurrent
	if maxConcurrent <= 0 {
		maxConcurrent = 10
	}

	authenticator := &core.IamAuthenticator{ApiKey: cfg.APIKey}
	service, err := vpcv1.NewVpcV1(&vpcv1.VpcV1Options{
		Authenticator: authenticator,
		URL:           fmt.Sprintf("https://%s.iaas.cloud.ibm.com/v1", cfg.Region),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create VPC SDK service: %w", err)
	}

	return &vpcClient{
		service:         service,
		resourceGroupID: cfg.ResourceGroupID,
		clusterID:       cfg.ClusterID,
		limiter:         NewRateLimiter(maxConcurrent),
	}, nil
}

// tagResource attaches user tags to a VPC resource via the Global Tagging API.
// This is best-effort — tagging failures are logged but not returned as errors.
func (c *vpcClient) tagResource(ctx context.Context, crn string, tagNames []string) {
	// The VPC Go SDK doesn't have a built-in tagging method for user tags.
	// In production, use the Global Tagging API (github.com/IBM/platform-services-go-sdk/globaltaggingv1).
	// For now, this is a placeholder — tags can be set via the IBM Cloud CLI or API separately.
	_ = ctx
	_ = crn
	_ = tagNames
}

// derefString safely dereferences a *string, returning "" if nil.
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// derefInt64 safely dereferences a *int64, returning 0 if nil.
func derefInt64(i *int64) int64 {
	if i == nil {
		return 0
	}
	return *i
}

// Compile-time interface checks
var _ Client = (*vpcClient)(nil)
var _ ExtendedClient = (*vpcClient)(nil)

// Suppress unused import warnings during scaffolding
var _ = context.Background
