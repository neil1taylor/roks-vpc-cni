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
	VPCID       string         `json:"vpc_id"`
	Description string         `json:"description"`
	CreatedAt   string         `json:"created_at"`
	Rules       []RuleResponse `json:"rules,omitempty"`
	ResourceURL string         `json:"resource_url,omitempty"`
}

// RuleRequest represents a request to add/update a security group rule
type RuleRequest struct {
	Direction       string `json:"direction" binding:"required,oneof=inbound outbound"`
	Protocol        string `json:"protocol" binding:"required"`
	PortMin         *int64 `json:"port_min"`
	PortMax         *int64 `json:"port_max"`
	RemoteCIDR      string `json:"cidr"`
	RemoteSGID      string `json:"security_group_id"`
}

// RuleResponse represents a security group rule
type RuleResponse struct {
	ID              string `json:"id"`
	Direction       string `json:"direction"`
	Protocol        string `json:"protocol"`
	PortMin         *int64 `json:"port_min,omitempty"`
	PortMax         *int64 `json:"port_max,omitempty"`
	RemoteCIDR      string `json:"cidr,omitempty"`
	RemoteSGID      string `json:"security_group_id,omitempty"`
}

// NetworkACLRequest represents a request to create/update a network ACL
type NetworkACLRequest struct {
	Name  string `json:"name" binding:"required"`
	VPCID string `json:"vpc_id" binding:"required"`
}

// NetworkACLResponse represents a network ACL
type NetworkACLResponse struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	VPCID       string            `json:"vpc_id"`
	CreatedAt   string            `json:"created_at"`
	Rules       []ACLRuleResponse `json:"rules,omitempty"`
	ResourceURL string            `json:"resource_url,omitempty"`
}

// ACLRuleRequest represents a request to add/update a network ACL rule
type ACLRuleRequest struct {
	Name        string `json:"name"`
	Action      string `json:"action" binding:"required,oneof=allow deny"`
	Direction   string `json:"direction" binding:"required,oneof=inbound outbound"`
	Protocol    string `json:"protocol" binding:"required"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
	PortMin     *int64 `json:"port_min"`
	PortMax     *int64 `json:"port_max"`
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
	PortMin     *int64 `json:"port_min,omitempty"`
	PortMax     *int64 `json:"port_max,omitempty"`
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

// TopologyNode represents a node in the topology graph
type TopologyNode struct {
	ID   string      `json:"id"`
	Type string      `json:"type"` // vpc, subnet, vni, vm, sg, acl
	Data interface{} `json:"data"`
}

// TopologyEdge represents an edge in the topology graph
type TopologyEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type"` // contains, binds, associates
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
