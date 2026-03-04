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
	FlowLogCollectorID        string       `json:"flowLogCollectorId,omitempty"`
	FlowLogActive             bool         `json:"flowLogActive,omitempty"`
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

// NodeHealth represents health data for a topology node.
type NodeHealth struct {
	Status  string             `json:"status"`            // healthy, warning, critical
	Metrics map[string]float64 `json:"metrics,omitempty"` // throughputBps, conntrackPct, errorRate, ipUtilization
}

// TopologyNode represents a node in the topology graph.
// Fields match the console plugin's TopologyNode interface.
type TopologyNode struct {
	ID       string                 `json:"id"`
	Label    string                 `json:"label"`
	Type     string                 `json:"type"` // vpc, subnet, vni, security-group, network-acl, floating-ip, network, router, gateway
	Status   string                 `json:"status,omitempty"` // available, pending, error
	Health   *NodeHealth            `json:"health,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// TopologyEdge represents an edge in the topology graph
type TopologyEdge struct {
	ID       string                 `json:"id"`
	Source   string                 `json:"source"`
	Target   string                 `json:"target"`
	Type     string                 `json:"type,omitempty"` // contains, connected, protected-by, associates, targets
	Metadata map[string]interface{} `json:"metadata,omitempty"`
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
	// PAR fields
	PAREnabled             bool   `json:"parEnabled"`
	PARPrefixLength        int    `json:"parPrefixLength,omitempty"`
	PublicAddressRangeID   string `json:"publicAddressRangeID,omitempty"`
	PublicAddressRangeCIDR string `json:"publicAddressRangeCIDR,omitempty"`
	IngressRoutingTableID  string `json:"ingressRoutingTableID,omitempty"`
}

// GatewayRequest represents a request to create a VPCGateway.
type GatewayRequest struct {
	Name           string `json:"name"`
	Namespace      string `json:"namespace,omitempty"`
	Zone           string `json:"zone"`
	UplinkNetwork  string `json:"uplinkNetwork"`
	TransitAddress string `json:"transitAddress"`
	TransitCIDR    string `json:"transitCIDR,omitempty"`
	TransitNetwork string `json:"transitNetwork,omitempty"`
	// PAR fields
	PAREnabled      bool   `json:"parEnabled,omitempty"`
	PARPrefixLength int    `json:"parPrefixLength,omitempty"`
	PARID           string `json:"parID,omitempty"`
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
	PodIP            string              `json:"podIP,omitempty"`
	IDSMode          string              `json:"idsMode,omitempty"`
	IDS              *RouterIDSResp      `json:"ids,omitempty"`
	MetricsEnabled   bool                `json:"metricsEnabled"`
	Mode             string              `json:"mode,omitempty"`
	XDPEnabled       bool                `json:"xdpEnabled"`
	DHCP             *RouterDHCPResp     `json:"dhcp,omitempty"`
	SyncStatus       string              `json:"syncStatus"`
	CreatedAt        string              `json:"createdAt,omitempty"`
}

// RouterNetworkResp represents a network attached to a router.
type RouterNetworkResp struct {
	Name      string                  `json:"name"`
	Address   string                  `json:"address"`
	Connected bool                    `json:"connected"`
	DHCP      *RouterNetworkDHCPResp  `json:"dhcp,omitempty"`
}

// RouterNetworkReq represents a network entry in a router create request.
type RouterNetworkReq struct {
	Name    string               `json:"name"`
	Address string               `json:"address"`
	DHCP    *RouterNetworkDHCPReq `json:"dhcp,omitempty"`
}

// RouterRequest represents a request to create a VPCRouter.
type RouterRequest struct {
	Name      string             `json:"name"`
	Namespace string             `json:"namespace,omitempty"`
	Gateway   string             `json:"gateway"`
	Networks  []RouterNetworkReq `json:"networks,omitempty"`
	DHCP      *RouterDHCPReq     `json:"dhcp,omitempty"`
	IDS       *RouterIDSReq      `json:"ids,omitempty"`
	Mode      string             `json:"mode,omitempty"`
}

// ── Router DHCP Types ──

// RouterDHCPResp represents global DHCP configuration in a router response.
type RouterDHCPResp struct {
	Enabled   bool             `json:"enabled"`
	LeaseTime string           `json:"leaseTime,omitempty"`
	DNS       *DHCPDNSResp     `json:"dns,omitempty"`
	Options   *DHCPOptionsResp `json:"options,omitempty"`
}

// RouterNetworkDHCPResp represents per-network DHCP state (merged spec+status).
type RouterNetworkDHCPResp struct {
	Enabled          bool                  `json:"enabled"`
	HasOverride      bool                  `json:"hasOverride"`
	PoolStart        string                `json:"poolStart,omitempty"`
	PoolEnd          string                `json:"poolEnd,omitempty"`
	RangeOverride    *DHCPRangeResp        `json:"rangeOverride,omitempty"`
	LeaseTime        string                `json:"leaseTime,omitempty"`
	Reservations     []DHCPReservationResp `json:"reservations,omitempty"`
	ReservationCount int                   `json:"reservationCount"`
	DNS              *DHCPDNSResp          `json:"dns,omitempty"`
	Options          *DHCPOptionsResp      `json:"options,omitempty"`
}

// DHCPReservationResp represents a static DHCP reservation.
type DHCPReservationResp struct {
	MAC      string `json:"mac"`
	IP       string `json:"ip"`
	Hostname string `json:"hostname,omitempty"`
}

// DHCPDNSResp represents DNS settings in a DHCP response.
type DHCPDNSResp struct {
	Nameservers   []string `json:"nameservers,omitempty"`
	SearchDomains []string `json:"searchDomains,omitempty"`
	LocalDomain   string   `json:"localDomain,omitempty"`
}

// DHCPOptionsResp represents additional DHCP options in a response.
type DHCPOptionsResp struct {
	Router     string   `json:"router,omitempty"`
	MTU        *int32   `json:"mtu,omitempty"`
	NTPServers []string `json:"ntpServers,omitempty"`
	Custom     []string `json:"custom,omitempty"`
}

// DHCPRangeResp represents a DHCP address range.
type DHCPRangeResp struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// RouterDHCPReq represents global DHCP configuration in a router create request.
type RouterDHCPReq struct {
	Enabled   bool             `json:"enabled"`
	LeaseTime string           `json:"leaseTime,omitempty"`
	DNS       *DHCPDNSResp     `json:"dns,omitempty"`
	Options   *DHCPOptionsResp `json:"options,omitempty"`
}

// RouterNetworkDHCPReq represents per-network DHCP configuration in a create request.
type RouterNetworkDHCPReq struct {
	Override     string                `json:"override"`
	RangeStart   string                `json:"rangeStart,omitempty"`
	RangeEnd     string                `json:"rangeEnd,omitempty"`
	LeaseTime    string                `json:"leaseTime,omitempty"`
	Reservations []DHCPReservationResp `json:"reservations,omitempty"`
	DNS          *DHCPDNSResp          `json:"dns,omitempty"`
	Options      *DHCPOptionsResp      `json:"options,omitempty"`
}

// ── Public Address Range (PAR) ──

// PARResponse represents a VPC public address range.
type PARResponse struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	CIDR           string `json:"cidr"`
	Zone           string `json:"zone"`
	LifecycleState string `json:"lifecycleState"`
	CreatedAt      string `json:"createdAt,omitempty"`
	// Cross-reference: which gateway owns this PAR (from K8s CRD status)
	GatewayName string `json:"gatewayName,omitempty"`
	GatewayNS   string `json:"gatewayNamespace,omitempty"`
}

// CreatePARRequest represents a request to create a public address range.
type CreatePARRequest struct {
	Name         string `json:"name"`
	Zone         string `json:"zone"`
	PrefixLength int    `json:"prefixLength"`
}

// ── DHCP Lease & Reservation Management ──

// DHCPLeaseResp represents an active DHCP lease from dnsmasq.
type DHCPLeaseResp struct {
	ExpiresAt int64  `json:"expiresAt"`
	MAC       string `json:"mac"`
	IP        string `json:"ip"`
	Hostname  string `json:"hostname"`
	ClientID  string `json:"clientId,omitempty"`
}

// UpdateReservationsReq represents a request to update DHCP reservations for a network.
type UpdateReservationsReq struct {
	Network      string                `json:"network"`
	Reservations []DHCPReservationResp `json:"reservations"`
}

// ── Router IDS/IPS Types ──

// RouterIDSResp represents IDS/IPS config in a router response.
type RouterIDSResp struct {
	Enabled      bool   `json:"enabled"`
	Mode         string `json:"mode"`
	Interfaces   string `json:"interfaces,omitempty"`
	CustomRules  string `json:"customRules,omitempty"`
	SyslogTarget string `json:"syslogTarget,omitempty"`
	Image        string `json:"image,omitempty"`
	NFQueueNum   *int32 `json:"nfqueueNum,omitempty"`
}

// RouterIDSReq represents IDS/IPS config in a create request.
type RouterIDSReq struct {
	Enabled      bool   `json:"enabled"`
	Mode         string `json:"mode"`
	Interfaces   string `json:"interfaces,omitempty"`
	CustomRules  string `json:"customRules,omitempty"`
	SyslogTarget string `json:"syslogTarget,omitempty"`
}

// UpdateIDSReq represents a request to update IDS/IPS config.
type UpdateIDSReq struct {
	Enabled      bool   `json:"enabled"`
	Mode         string `json:"mode"`
	Interfaces   string `json:"interfaces,omitempty"`
	CustomRules  string `json:"customRules,omitempty"`
	SyslogTarget string `json:"syslogTarget,omitempty"`
}

// ── L2 Bridge ──

// L2BridgeNetworkRefResp represents the networkRef in L2Bridge responses.
type L2BridgeNetworkRefResp struct {
	Name      string `json:"name"`
	Kind      string `json:"kind,omitempty"`
	Namespace string `json:"namespace,omitempty"`
}

// L2BridgeResponse represents a VPCL2Bridge resource.
type L2BridgeResponse struct {
	Name                string                 `json:"name"`
	Namespace           string                 `json:"namespace"`
	Type                string                 `json:"type"`
	GatewayRef          string                 `json:"gatewayRef"`
	NetworkRef          L2BridgeNetworkRefResp `json:"networkRef"`
	RemoteEndpoint      string                 `json:"remoteEndpoint"`
	Phase               string                 `json:"phase"`
	TunnelEndpoint      string                 `json:"tunnelEndpoint,omitempty"`
	RemoteMACsLearned   int32                  `json:"remoteMACsLearned"`
	LocalMACsAdvertised int32                  `json:"localMACsAdvertised"`
	BytesIn             int64                  `json:"bytesIn"`
	BytesOut            int64                  `json:"bytesOut"`
	LastHandshake       string                 `json:"lastHandshake,omitempty"`
	TunnelMTU           int32                  `json:"tunnelMTU,omitempty"`
	MSSClamp            *bool                  `json:"mssClamp,omitempty"`
	PodName             string                 `json:"podName,omitempty"`
	SyncStatus          string                 `json:"syncStatus"`
	CreatedAt           string                 `json:"createdAt,omitempty"`
}

// L2BridgeNetworkRefReq represents the networkRef in L2Bridge create requests.
type L2BridgeNetworkRefReq struct {
	Name      string `json:"name"`
	Kind      string `json:"kind,omitempty"`
	Namespace string `json:"namespace,omitempty"`
}

// L2BridgeWireGuardReq represents WireGuard config in L2Bridge create requests.
type L2BridgeWireGuardReq struct {
	PrivateKeySecret     string `json:"privateKeySecret"`
	PrivateKeySecretKey  string `json:"privateKeySecretKey"`
	PeerPublicKey        string `json:"peerPublicKey"`
	ListenPort           *int32 `json:"listenPort,omitempty"`
	TunnelAddressLocal   string `json:"tunnelAddressLocal"`
	TunnelAddressRemote  string `json:"tunnelAddressRemote"`
}

// L2BridgeRemoteReq represents the remote endpoint in L2Bridge create requests.
type L2BridgeRemoteReq struct {
	Endpoint  string                `json:"endpoint"`
	WireGuard *L2BridgeWireGuardReq `json:"wireGuard,omitempty"`
}

// L2BridgeMTUReq represents MTU settings in L2Bridge create requests.
type L2BridgeMTUReq struct {
	TunnelMTU *int32 `json:"tunnelMTU,omitempty"`
	MSSClamp  *bool  `json:"mssClamp,omitempty"`
}

// L2BridgeRequest represents a request to create a VPCL2Bridge.
type L2BridgeRequest struct {
	Name       string                `json:"name"`
	Namespace  string                `json:"namespace,omitempty"`
	Type       string                `json:"type"`
	GatewayRef string                `json:"gatewayRef"`
	NetworkRef L2BridgeNetworkRefReq `json:"networkRef"`
	Remote     L2BridgeRemoteReq     `json:"remote"`
	MTU        *L2BridgeMTUReq       `json:"mtu,omitempty"`
}

// ── VPN Gateway ──

// VPNRemoteAccessResp represents remote access configuration in a VPN gateway response.
type VPNRemoteAccessResp struct {
	Enabled     bool     `json:"enabled"`
	AddressPool string   `json:"addressPool,omitempty"`
	DNSServers  []string `json:"dnsServers,omitempty"`
	MaxClients  *int32   `json:"maxClients,omitempty"`
}

// VPNGatewayResponse represents a VPCVPNGateway resource.
type VPNGatewayResponse struct {
	Name             string                `json:"name"`
	Namespace        string                `json:"namespace"`
	Protocol         string                `json:"protocol"`
	GatewayRef       string                `json:"gatewayRef"`
	Phase            string                `json:"phase"`
	TunnelEndpoint   string                `json:"tunnelEndpoint,omitempty"`
	ActiveTunnels    int32                 `json:"activeTunnels"`
	TotalTunnels     int32                 `json:"totalTunnels"`
	ConnectedClients int32                 `json:"connectedClients"`
	IssuedClients    int32                 `json:"issuedClients"`
	Tunnels          []VPNTunnelStatusResp `json:"tunnels,omitempty"`
	AdvertisedRoutes []string              `json:"advertisedRoutes,omitempty"`
	TunnelMTU        int32                 `json:"tunnelMTU,omitempty"`
	MSSClamp         *bool                 `json:"mssClamp,omitempty"`
	RemoteAccess     *VPNRemoteAccessResp  `json:"remoteAccess,omitempty"`
	PodName          string                `json:"podName,omitempty"`
	SyncStatus       string                `json:"syncStatus"`
	Message          string                `json:"message,omitempty"`
	CreatedAt        string                `json:"createdAt,omitempty"`
}

// VPNTunnelStatusResp represents per-tunnel status in a VPN gateway.
type VPNTunnelStatusResp struct {
	Name          string `json:"name"`
	Status        string `json:"status"`
	LastHandshake string `json:"lastHandshake,omitempty"`
	BytesIn       int64  `json:"bytesIn"`
	BytesOut      int64  `json:"bytesOut"`
}

// VPNTunnelReq represents a tunnel entry in a VPN gateway create request.
type VPNTunnelReq struct {
	Name                  string   `json:"name"`
	RemoteEndpoint        string   `json:"remoteEndpoint"`
	RemoteNetworks        []string `json:"remoteNetworks"`
	PeerPublicKey         string   `json:"peerPublicKey,omitempty"`
	TunnelAddressLocal    string   `json:"tunnelAddressLocal,omitempty"`
	TunnelAddressRemote   string   `json:"tunnelAddressRemote,omitempty"`
	PresharedKeySecret    string   `json:"presharedKeySecret,omitempty"`
	PresharedKeySecretKey string   `json:"presharedKeySecretKey,omitempty"`
}

// VPNWireGuardReq represents WireGuard config in a VPN gateway create request.
type VPNWireGuardReq struct {
	PrivateKeySecret    string `json:"privateKeySecret"`
	PrivateKeySecretKey string `json:"privateKeySecretKey"`
	ListenPort          *int32 `json:"listenPort,omitempty"`
}

// VPNIPsecReq represents IPsec config in a VPN gateway create request.
type VPNIPsecReq struct {
	Image string `json:"image,omitempty"`
}

// VPNMTUReq represents MTU settings in a VPN gateway create request.
type VPNMTUReq struct {
	TunnelMTU *int32 `json:"tunnelMTU,omitempty"`
	MSSClamp  *bool  `json:"mssClamp,omitempty"`
}

// VPNOpenVPNReq represents OpenVPN config in a VPN gateway create request.
type VPNOpenVPNReq struct {
	CASecret         string `json:"caSecret"`
	CASecretKey      string `json:"caSecretKey"`
	CertSecret       string `json:"certSecret"`
	CertSecretKey    string `json:"certSecretKey"`
	KeySecret        string `json:"keySecret"`
	KeySecretKey     string `json:"keySecretKey"`
	DHSecret         string `json:"dhSecret,omitempty"`
	DHSecretKey      string `json:"dhSecretKey,omitempty"`
	TLSAuthSecret    string `json:"tlsAuthSecret,omitempty"`
	TLSAuthSecretKey string `json:"tlsAuthSecretKey,omitempty"`
	ListenPort       *int32 `json:"listenPort,omitempty"`
	Proto            string `json:"proto,omitempty"`
	Cipher           string `json:"cipher,omitempty"`
	ClientSubnet     string `json:"clientSubnet,omitempty"`
	Image            string `json:"image,omitempty"`
}

// VPNRemoteAccessReq represents remote access config in a VPN gateway create request.
type VPNRemoteAccessReq struct {
	Enabled     bool     `json:"enabled"`
	AddressPool string   `json:"addressPool,omitempty"`
	DNSServers  []string `json:"dnsServers,omitempty"`
	MaxClients  *int32   `json:"maxClients,omitempty"`
}

// VPNLocalNetworkReq represents a local network in a VPN gateway create request.
type VPNLocalNetworkReq struct {
	CIDR string `json:"cidr,omitempty"`
}

// VPNGatewayRequest represents a request to create a VPCVPNGateway.
type VPNGatewayRequest struct {
	Name          string              `json:"name"`
	Namespace     string              `json:"namespace,omitempty"`
	Protocol      string              `json:"protocol"`
	GatewayRef    string              `json:"gatewayRef"`
	WireGuard     *VPNWireGuardReq    `json:"wireGuard,omitempty"`
	IPsec         *VPNIPsecReq        `json:"ipsec,omitempty"`
	OpenVPN       *VPNOpenVPNReq      `json:"openVPN,omitempty"`
	Tunnels       []VPNTunnelReq      `json:"tunnels"`
	MTU           *VPNMTUReq          `json:"mtu,omitempty"`
	RemoteAccess  *VPNRemoteAccessReq  `json:"remoteAccess,omitempty"`
	LocalNetworks []VPNLocalNetworkReq `json:"localNetworks,omitempty"`
}

// ClientConfigRequest represents a request to generate a client .ovpn config.
type ClientConfigRequest struct {
	ClientName string `json:"clientName"`
}

// ClientConfigResponse represents a generated client config.
type ClientConfigResponse struct {
	ClientName string `json:"clientName"`
	SecretName string `json:"secretName"`
	OVPNConfig string `json:"ovpnConfig"`
}

// IssuedClientResponse represents an issued client certificate.
type IssuedClientResponse struct {
	ClientName string `json:"clientName"`
	SecretName string `json:"secretName"`
	SerialHex  string `json:"serialHex"`
	IssuedAt   string `json:"issuedAt"`
	ExpiresAt  string `json:"expiresAt"`
	Revoked    bool   `json:"revoked"`
}

// ── DNS Policy ──

// DNSPolicyResponse represents a VPCDNSPolicy resource.
type DNSPolicyResponse struct {
	Name             string   `json:"name"`
	Namespace        string   `json:"namespace"`
	RouterRef        string   `json:"routerRef"`
	Phase            string   `json:"phase"`
	SyncStatus       string   `json:"syncStatus"`
	FilterRulesLoaded int64   `json:"filterRulesLoaded"`
	UpstreamServers  []string `json:"upstreamServers"`
	FilteringEnabled bool     `json:"filteringEnabled"`
	LocalDNSEnabled  bool     `json:"localDNSEnabled"`
	LocalDNSDomain   string   `json:"localDNSDomain,omitempty"`
	ConfigMapName    string   `json:"configMapName,omitempty"`
	Message          string   `json:"message,omitempty"`
	CreatedAt        string   `json:"createdAt,omitempty"`
}

// DNSPolicyRequest represents a request to create a VPCDNSPolicy.
type DNSPolicyRequest struct {
	Name        string                    `json:"name"`
	Namespace   string                    `json:"namespace,omitempty"`
	RouterRef   string                    `json:"routerRef"`
	Upstream    *DNSPolicyUpstreamReq     `json:"upstream,omitempty"`
	Filtering   *DNSPolicyFilteringReq    `json:"filtering,omitempty"`
	LocalDNS    *DNSPolicyLocalDNSReq     `json:"localDNS,omitempty"`
	Image       string                    `json:"image,omitempty"`
}

// DNSPolicyUpstreamReq represents upstream DNS config in a create request.
type DNSPolicyUpstreamReq struct {
	Servers []string `json:"servers"`
}

// DNSPolicyFilteringReq represents filtering config in a create request.
type DNSPolicyFilteringReq struct {
	Enabled    bool     `json:"enabled"`
	Blocklists []string `json:"blocklists,omitempty"`
	Allowlist  []string `json:"allowlist,omitempty"`
	Denylist   []string `json:"denylist,omitempty"`
}

// DNSPolicyLocalDNSReq represents local DNS config in a create request.
type DNSPolicyLocalDNSReq struct {
	Enabled bool   `json:"enabled"`
	Domain  string `json:"domain,omitempty"`
}
