package model

// ErrorResponse represents a standardized error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

// SecurityGroupRequest represents a request to create/update a security group
type SecurityGroupRequest struct {
	Name        string `json:"name" binding:"required"`
	VPCID       string `json:"vpc_id" binding:"required"`
	Description string `json:"description"`
}

// SecurityGroupResponse represents a security group
type SecurityGroupResponse struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	VPC         RefResponse    `json:"vpc"`
	Description string         `json:"description"`
	Status      string         `json:"status"`
	CreatedAt   string         `json:"createdAt"`
	Rules       []RuleResponse `json:"rules,omitempty"`
}

// RuleRequest represents a request to add/update a security group rule
type RuleRequest struct {
	Direction  string `json:"direction" binding:"required,oneof=inbound outbound"`
	Protocol   string `json:"protocol" binding:"required"`
	PortMin    *int64 `json:"portMin"`
	PortMax    *int64 `json:"portMax"`
	RemoteCIDR string `json:"remote"`
	RemoteSGID string `json:"remoteSgId"`
}

// RuleResponse represents a security group rule
type RuleResponse struct {
	ID         string `json:"id"`
	Direction  string `json:"direction"`
	Protocol   string `json:"protocol"`
	PortMin    *int64 `json:"portMin,omitempty"`
	PortMax    *int64 `json:"portMax,omitempty"`
	Remote     string `json:"remote,omitempty"`
	RemoteType string `json:"remoteType,omitempty"`
}

// NetworkACLRequest represents a request to create/update a network ACL
type NetworkACLRequest struct {
	Name  string `json:"name" binding:"required"`
	VPCID string `json:"vpc_id" binding:"required"`
}

// NetworkACLResponse represents a network ACL
type NetworkACLResponse struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	VPC       RefResponse       `json:"vpc"`
	Status    string            `json:"status"`
	Subnets   []RefResponse     `json:"subnets,omitempty"`
	CreatedAt string            `json:"createdAt"`
	Rules     []ACLRuleResponse `json:"rules,omitempty"`
}

// ACLRuleRequest represents a request to add/update a network ACL rule
type ACLRuleRequest struct {
	Name        string `json:"name"`
	Action      string `json:"action" binding:"required,oneof=allow deny"`
	Direction   string `json:"direction" binding:"required,oneof=inbound outbound"`
	Protocol    string `json:"protocol" binding:"required"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
	PortMin     *int64 `json:"portMin"`
	PortMax     *int64 `json:"portMax"`
}

// ACLRuleResponse represents a network ACL rule
type ACLRuleResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Action      string `json:"action"`
	Direction   string `json:"direction"`
	Protocol    string `json:"protocol"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
	PortMin     *int64 `json:"portMin,omitempty"`
	PortMax     *int64 `json:"portMax,omitempty"`
}

// VPCResponse represents a VPC
type VPCResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Region      string `json:"region"`
	CreatedAt   string `json:"created_at"`
	Status      string `json:"status"`
	ResourceURL string `json:"resource_url,omitempty"`
}

// ZoneResponse represents an availability zone
type ZoneResponse struct {
	Name   string `json:"name"`
	Region string `json:"region"`
	Status string `json:"status"`
}

// RefResponse is a shared sub-type for referencing related resources.
type RefResponse struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

// IPResponse represents an IP address.
type IPResponse struct {
	Address string `json:"address"`
}

// SubnetRequest represents a request to create a subnet.
type SubnetRequest struct {
	Name          string      `json:"name"`
	VPC           RefResponse `json:"vpc"`
	Zone          RefResponse `json:"zone"`
	IPV4CIDRBlock string      `json:"ipv4CidrBlock"`
}

// SubnetResponse represents a VPC subnet.
type SubnetResponse struct {
	ID                        string       `json:"id"`
	Name                      string       `json:"name"`
	IPV4CIDRBlock             string       `json:"ipv4CidrBlock"`
	Status                    string       `json:"status"`
	AvailableIPv4AddressCount int64        `json:"availableIpv4AddressCount"`
	TotalIPv4AddressCount     int64        `json:"totalIpv4AddressCount"`
	VPC                       RefResponse  `json:"vpc"`
	Zone                      RefResponse  `json:"zone"`
	NetworkACL                *RefResponse `json:"networkAcl,omitempty"`
	CreatedAt                 string       `json:"createdAt,omitempty"`
}

// VNIResponse represents a Virtual Network Interface.
type VNIResponse struct {
	ID                      string       `json:"id"`
	Name                    string       `json:"name"`
	AllowIPSpoofing         bool         `json:"allowIpSpoofing"`
	EnableInfrastructureNat bool         `json:"enableInfrastructureNat"`
	Subnet                  *RefResponse `json:"subnet,omitempty"`
	PrimaryIP               *IPResponse  `json:"primaryIp,omitempty"`
	Status                  string       `json:"status"`
	CreatedAt               string       `json:"createdAt,omitempty"`
}

// FloatingIPRequest represents a request to reserve a floating IP.
type FloatingIPRequest struct {
	Name string `json:"name"`
	Zone string `json:"zone"`
}

// FloatingIPUpdateRequest represents a request to bind/unbind a floating IP.
type FloatingIPUpdateRequest struct {
	TargetID string `json:"target_id"` // VNI ID to bind; empty string to unbind
}

// FloatingIPResponse represents a floating IP.
type FloatingIPResponse struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	Address   string       `json:"address"`
	Status    string       `json:"status"`
	Zone      RefResponse  `json:"zone"`
	Target    *RefResponse `json:"target,omitempty"`
	CreatedAt string       `json:"createdAt,omitempty"`
}

// ReservedIPResponse represents a reserved IP in a subnet.
type ReservedIPResponse struct {
	ID         string       `json:"id"`
	Name       string       `json:"name"`
	Address    string       `json:"address"`
	AutoDelete bool         `json:"autoDelete"`
	Owner      string       `json:"owner"`
	Target     *RefResponse `json:"target,omitempty"`
	CreatedAt  string       `json:"createdAt,omitempty"`
}

// TopologyNode represents a node in the topology graph.
// Fields match the console plugin's TopologyNode interface.
type TopologyNode struct {
	ID       string                 `json:"id"`
	Label    string                 `json:"label"`
	Type     string                 `json:"type"` // vpc, subnet, vni, security-group, network-acl, floating-ip, network
	Status   string                 `json:"status,omitempty"` // available, pending, error
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// TopologyEdge represents an edge in the topology graph
type TopologyEdge struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type,omitempty"` // contains, connected, protected-by, associates, targets
}

// TopologyResponse represents the aggregated topology graph
type TopologyResponse struct {
	Nodes []TopologyNode `json:"nodes"`
	Edges []TopologyEdge `json:"edges"`
}

// ListOptions represents common list query parameters
type ListOptions struct {
	VPCID  string
	Region string
	Limit  int
	Offset int
}

// NodeData represents data for different node types
type VPCNodeData struct {
	Name   string `json:"name"`
	Region string `json:"region"`
	Status string `json:"status"`
}

type SubnetNodeData struct {
	Name         string `json:"name"`
	VPCID        string `json:"vpc_id"`
	CIDR         string `json:"cidr"`
	Zone         string `json:"zone"`
	AvailableIPs int    `json:"available_ips"`
}

type VNINodeData struct {
	ID        string `json:"id"`
	SubnetID  string `json:"subnet_id"`
	Primary   bool   `json:"primary"`
	IPAddress string `json:"ip_address"`
}

type VMNodeData struct {
	Name  string `json:"name"`
	Zone  string `json:"zone"`
	State string `json:"state"`
}

type SecurityGroupNodeData struct {
	Name        string `json:"name"`
	VPCID       string `json:"vpc_id"`
	RuleCount   int    `json:"rule_count"`
	Description string `json:"description,omitempty"`
}

type ACLNodeData struct {
	Name      string `json:"name"`
	VPCID     string `json:"vpc_id"`
	RuleCount int    `json:"rule_count"`
}

// IPAssignmentMode describes how VMs on a network receive IP addresses
type IPAssignmentMode string

const (
	IPModeStaticReserved IPAssignmentMode = "static_reserved"
	IPModeDHCP           IPAssignmentMode = "dhcp"
	IPModeNone           IPAssignmentMode = "none"
)

// NetworkTier indicates the complexity/risk level of a network combination
type NetworkTier string

const (
	TierRecommended NetworkTier = "recommended"
	TierAdvanced    NetworkTier = "advanced"
	TierExpert      NetworkTier = "expert"
)

// NetworkCombination describes one of the 4 valid topology+scope+role combos
type NetworkCombination struct {
	ID          string           `json:"id"`
	Topology    string           `json:"topology"`
	Scope       string           `json:"scope"`
	Role        string           `json:"role"`
	Tier        NetworkTier      `json:"tier"`
	IPMode      IPAssignmentMode `json:"ip_mode"`
	Label       string           `json:"label"`
	Description string           `json:"description"`
	IPModeDesc  string           `json:"ip_mode_description"`
	RequiresVPC bool             `json:"requires_vpc"`
}

// NetworkDefinition represents a CUDN or UDN network resource
type NetworkDefinition struct {
	Name            string           `json:"name"`
	Namespace       string           `json:"namespace,omitempty"`
	Kind            string           `json:"kind"`
	Topology        string           `json:"topology"`
	Role            string           `json:"role,omitempty"`
	Tier            NetworkTier      `json:"tier,omitempty"`
	IPMode          IPAssignmentMode `json:"ip_mode,omitempty"`
	SubnetID        string           `json:"subnet_id,omitempty"`
	SubnetName      string           `json:"subnet_name,omitempty"`
	SubnetStatus    string           `json:"subnet_status,omitempty"`
	VPCID           string           `json:"vpc_id,omitempty"`
	Zone            string           `json:"zone,omitempty"`
	CIDR            string           `json:"cidr,omitempty"`
	VLANID          string           `json:"vlan_id,omitempty"`
	VLANAttachments string           `json:"vlan_attachments,omitempty"`
}

// CreateNetworkRequest represents a request to create a CUDN or UDN
type CreateNetworkRequest struct {
	Name             string   `json:"name"`
	Namespace        string   `json:"namespace,omitempty"`
	Topology         string   `json:"topology"`
	Role             string   `json:"role,omitempty"`
	VPCID            string   `json:"vpc_id,omitempty"`
	Zone             string   `json:"zone,omitempty"`
	CIDR             string   `json:"cidr,omitempty"`
	VLANID           string   `json:"vlan_id,omitempty"`
	SecurityGroupIDs string   `json:"security_group_ids,omitempty"`
	ACLID            string   `json:"acl_id,omitempty"`
	PublicGatewayID  string   `json:"public_gateway_id,omitempty"`
	TargetNamespaces []string `json:"target_namespaces,omitempty"`
}

// NetworkTypesResponse returns available network configurations
type NetworkTypesResponse struct {
	Topologies   []string             `json:"topologies"`
	Scopes       []string             `json:"scopes"`
	Roles        []string             `json:"roles"`
	Combinations []NetworkCombination `json:"combinations"`
}

// NetworkNodeData represents a network definition in topology
type NetworkNodeData struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Topology  string `json:"topology"`
	SubnetID  string `json:"subnet_id,omitempty"`
	Namespace string `json:"namespace,omitempty"`
}

// NamespaceInfo represents a namespace with label information.
type NamespaceInfo struct {
	Name            string `json:"name"`
	HasPrimaryLabel bool   `json:"hasPrimaryLabel"`
}

// CreateNamespaceRequest represents a request to create a namespace.
type CreateNamespaceRequest struct {
	Name   string            `json:"name"`
	Labels map[string]string `json:"labels,omitempty"`
}

// AddressPrefixResponse represents a VPC address prefix.
type AddressPrefixResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CIDR      string `json:"cidr"`
	Zone      string `json:"zone"`
	IsDefault bool   `json:"isDefault"`
}

// PublicGatewayResponse represents a VPC public gateway.
type PublicGatewayResponse struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Status     string      `json:"status"`
	Zone       RefResponse `json:"zone"`
	FloatingIP *IPResponse `json:"floatingIp,omitempty"`
	CreatedAt  string      `json:"createdAt,omitempty"`
}

// RoutingTableResponse represents a VPC routing table.
type RoutingTableResponse struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	IsDefault      bool   `json:"isDefault"`
	LifecycleState string `json:"lifecycleState"`
	RouteCount     int    `json:"routeCount"`
	CreatedAt      string `json:"createdAt,omitempty"`
}

// RouteResponse represents a VPC route.
type RouteResponse struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Destination    string `json:"destination"`
	Action         string `json:"action"`
	NextHop        string `json:"nextHop,omitempty"`
	Zone           string `json:"zone"`
	Priority       int64  `json:"priority"`
	Origin         string `json:"origin"`
	LifecycleState string `json:"lifecycleState"`
	CreatedAt      string `json:"createdAt,omitempty"`
}

// CreateRouteRequest represents a request to create a VPC route.
type CreateRouteRequest struct {
	Name        string `json:"name"`
	Destination string `json:"destination"`
	Action      string `json:"action"`
	NextHopIP   string `json:"nextHopIp,omitempty"`
	Zone        string `json:"zone"`
	Priority    *int64 `json:"priority,omitempty"`
}

// ── Gateway ──

// GatewayResponse represents a VPCGateway resource.
type GatewayResponse struct {
	Name           string `json:"name"`
	Namespace      string `json:"namespace"`
	Zone           string `json:"zone"`
	Phase          string `json:"phase"`
	UplinkNetwork  string `json:"uplinkNetwork"`
	TransitNetwork string `json:"transitNetwork"`
	VNIID          string `json:"vniID,omitempty"`
	ReservedIP     string `json:"reservedIP,omitempty"`
	FloatingIP     string `json:"floatingIP,omitempty"`
	VPCRouteCount  int    `json:"vpcRouteCount"`
	NATRuleCount   int    `json:"natRuleCount"`
	SyncStatus     string `json:"syncStatus"`
	CreatedAt      string `json:"createdAt,omitempty"`
}

// GatewayRequest represents a request to create a VPCGateway.
type GatewayRequest struct {
	Name    string `json:"name"`
	Zone    string `json:"zone"`
	Uplink  string `json:"uplink"`
	Transit string `json:"transit,omitempty"`
}

// ── Router ──

// RouterResponse represents a VPCRouter resource.
type RouterResponse struct {
	Name             string              `json:"name"`
	Namespace        string              `json:"namespace"`
	Gateway          string              `json:"gateway"`
	Phase            string              `json:"phase"`
	TransitIP        string              `json:"transitIP,omitempty"`
	Networks         []RouterNetworkResp `json:"networks"`
	AdvertisedRoutes []string            `json:"advertisedRoutes,omitempty"`
	Functions        []string            `json:"functions,omitempty"`
	SyncStatus       string              `json:"syncStatus"`
	CreatedAt        string              `json:"createdAt,omitempty"`
}

// RouterNetworkResp represents a network attached to a router.
type RouterNetworkResp struct {
	Name      string `json:"name"`
	Address   string `json:"address"`
	Connected bool   `json:"connected"`
}

// RouterRequest represents a request to create a VPCRouter.
type RouterRequest struct {
	Name    string `json:"name"`
	Gateway string `json:"gateway"`
}
