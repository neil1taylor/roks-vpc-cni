package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// --- Shared types (used by both VPCGateway and VPCRouter) ---

// RouterPodSpec defines pod-level overrides for gateway/router pods.
type RouterPodSpec struct {
	// Image is the container image to use for the gateway/router pod.
	// +optional
	Image string `json:"image,omitempty"`

	// Replicas is the number of gateway/router pod replicas.
	// +optional
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=3
	Replicas *int32 `json:"replicas,omitempty"`
}

// GatewayFirewall defines firewall configuration for a gateway or router.
type GatewayFirewall struct {
	// Enabled controls whether the firewall is active.
	// +kubebuilder:default=false
	Enabled bool `json:"enabled"`

	// Rules is the ordered list of firewall rules.
	// +optional
	Rules []FirewallRule `json:"rules,omitempty"`
}

// FirewallRule defines a single firewall rule.
type FirewallRule struct {
	// Name is the human-readable name of this rule.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Action is the action to take when the rule matches.
	// +kubebuilder:validation:Enum=allow;deny
	Action string `json:"action"`

	// Direction is the traffic direction this rule applies to.
	// +kubebuilder:validation:Enum=ingress;egress
	Direction string `json:"direction"`

	// Source is the source CIDR or address to match.
	// +optional
	Source string `json:"source,omitempty"`

	// Destination is the destination CIDR or address to match.
	// +optional
	Destination string `json:"destination,omitempty"`

	// Protocol is the IP protocol to match.
	// +kubebuilder:validation:Enum=tcp;udp;icmp;any
	// +kubebuilder:default=any
	Protocol string `json:"protocol"`

	// Port is the port number to match (for TCP/UDP).
	// +optional
	Port *int32 `json:"port,omitempty"`

	// Priority controls the evaluation order (lower = higher priority).
	// +kubebuilder:default=100
	Priority int32 `json:"priority"`
}

// --- VPCGateway-specific types ---

// GatewayUplink defines the uplink (external-facing) network configuration.
type GatewayUplink struct {
	// Network is the name of the uplink network (LocalNet CUDN/UDN).
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Network string `json:"network"`

	// SecurityGroupIDs is a list of VPC security group IDs for the uplink VNI.
	// +optional
	SecurityGroupIDs []string `json:"securityGroupIDs,omitempty"`
}

// GatewayTransit defines the transit (inter-router) network configuration.
type GatewayTransit struct {
	// Network is the name of the transit L2 network.
	// +optional
	Network string `json:"network,omitempty"`

	// Address is the IP address on the transit network.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Address string `json:"address"`

	// CIDR is the transit network CIDR block.
	// +optional
	CIDR string `json:"cidr,omitempty"`
}

// VPCRouteSpec defines a VPC route to be created for the gateway.
type VPCRouteSpec struct {
	// Destination is the destination CIDR for the route.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Destination string `json:"destination"`
}

// GatewayNAT defines NAT rules for the gateway.
type GatewayNAT struct {
	// SNAT defines source NAT rules.
	// +optional
	SNAT []SNATRule `json:"snat,omitempty"`

	// DNAT defines destination NAT rules.
	// +optional
	DNAT []DNATRule `json:"dnat,omitempty"`

	// NoNAT defines exceptions to NAT processing.
	// +optional
	NoNAT []NoNATRule `json:"noNAT,omitempty"`
}

// SNATRule defines a source NAT rule.
type SNATRule struct {
	// Source is the source CIDR to match for SNAT.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Source string `json:"source"`

	// TranslatedAddress is the address to translate the source to.
	// If empty, the gateway's floating IP is used.
	// +optional
	TranslatedAddress string `json:"translatedAddress,omitempty"`

	// Priority controls the evaluation order (lower = higher priority).
	// +kubebuilder:default=100
	Priority int32 `json:"priority"`
}

// DNATRule defines a destination NAT rule.
type DNATRule struct {
	// ExternalAddress is the external address to match. If empty, matches the gateway's floating IP.
	// +optional
	ExternalAddress string `json:"externalAddress,omitempty"`

	// ExternalPort is the external port to match.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	ExternalPort int32 `json:"externalPort"`

	// InternalAddress is the internal address to forward to.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	InternalAddress string `json:"internalAddress"`

	// InternalPort is the internal port to forward to.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	InternalPort int32 `json:"internalPort"`

	// Protocol is the IP protocol for the DNAT rule.
	// +kubebuilder:validation:Enum=tcp;udp
	// +kubebuilder:default=tcp
	Protocol string `json:"protocol"`

	// Priority controls the evaluation order (lower = higher priority).
	// +kubebuilder:default=50
	Priority int32 `json:"priority"`
}

// NoNATRule defines an exception to NAT processing.
type NoNATRule struct {
	// Source is the source CIDR to exempt from NAT.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Source string `json:"source"`

	// Destination is the destination CIDR to exempt from NAT.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Destination string `json:"destination"`

	// Priority controls the evaluation order (lower = higher priority).
	// +kubebuilder:default=10
	Priority int32 `json:"priority"`
}

// GatewayFloatingIP controls floating IP allocation for the gateway.
type GatewayFloatingIP struct {
	// Enabled controls whether a floating IP is allocated for the gateway.
	// +kubebuilder:default=false
	Enabled bool `json:"enabled"`

	// ID is an existing floating IP ID to use instead of creating a new one.
	// +optional
	ID string `json:"id,omitempty"`
}

// GatewayInterface describes a network interface on the gateway.
type GatewayInterface struct {
	// Role is the role of this interface.
	// +kubebuilder:validation:Enum=uplink;downlink
	Role string `json:"role"`

	// Network is the network name this interface is attached to.
	Network string `json:"network"`

	// Address is the IP address assigned to this interface.
	Address string `json:"address"`
}

// VPCGatewaySpec defines the desired state of a VPCGateway.
type VPCGatewaySpec struct {
	// Zone is the VPC availability zone for the gateway (e.g., "eu-de-1").
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Zone string `json:"zone"`

	// Uplink defines the uplink (external-facing) network configuration.
	// +kubebuilder:validation:Required
	Uplink GatewayUplink `json:"uplink"`

	// Transit defines the transit (inter-router) network configuration.
	// +kubebuilder:validation:Required
	Transit GatewayTransit `json:"transit"`

	// VPCRoutes defines VPC routes to be created for the gateway.
	// +optional
	VPCRoutes []VPCRouteSpec `json:"vpcRoutes,omitempty"`

	// NAT defines NAT rules for the gateway.
	// +optional
	NAT *GatewayNAT `json:"nat,omitempty"`

	// FloatingIP controls floating IP allocation for the gateway.
	// +optional
	FloatingIP *GatewayFloatingIP `json:"floatingIP,omitempty"`

	// Firewall defines firewall rules for the gateway.
	// +optional
	Firewall *GatewayFirewall `json:"firewall,omitempty"`

	// Pod defines pod-level overrides for the gateway pod.
	// +optional
	Pod *RouterPodSpec `json:"pod,omitempty"`
}

// VPCGatewayStatus defines the observed state of a VPCGateway.
type VPCGatewayStatus struct {
	// Phase is the current lifecycle phase of the gateway.
	// +kubebuilder:validation:Enum=Pending;Provisioning;Ready;Error
	Phase string `json:"phase,omitempty"`

	// VNIID is the VPC VNI ID allocated for the gateway's uplink interface.
	VNIID string `json:"vniID,omitempty"`

	// ReservedIP is the reserved IP address on the uplink VNI.
	ReservedIP string `json:"reservedIP,omitempty"`

	// FloatingIP is the public floating IP address assigned to the gateway.
	FloatingIP string `json:"floatingIP,omitempty"`

	// TransitNetwork is the name of the transit network the gateway is connected to.
	TransitNetwork string `json:"transitNetwork,omitempty"`

	// VPCRouteIDs is the list of VPC route IDs created for the gateway.
	// +optional
	VPCRouteIDs []string `json:"vpcRouteIDs,omitempty"`

	// Interfaces describes the network interfaces provisioned on the gateway.
	// +optional
	Interfaces []GatewayInterface `json:"interfaces,omitempty"`

	// SyncStatus indicates whether the CR is in sync with the VPC API.
	// +kubebuilder:validation:Enum=Synced;Pending;Failed
	SyncStatus string `json:"syncStatus,omitempty"`

	// LastSyncTime is the timestamp of the last successful sync.
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

	// Message provides human-readable detail about the current status.
	Message string `json:"message,omitempty"`

	// Conditions represent the latest available observations of the gateway's state.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=vgw
// +kubebuilder:printcolumn:name="Zone",type=string,JSONPath=`.spec.zone`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="VNI IP",type=string,JSONPath=`.status.reservedIP`
// +kubebuilder:printcolumn:name="FIP",type=string,JSONPath=`.status.floatingIP`
// +kubebuilder:printcolumn:name="Sync",type=string,JSONPath=`.status.syncStatus`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// VPCGateway is the Schema for the vpcgateways API.
// It represents a T0 per-zone gateway that bridges a LocalNet CUDN to a transit L2 network.
type VPCGateway struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VPCGatewaySpec   `json:"spec,omitempty"`
	Status VPCGatewayStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// VPCGatewayList contains a list of VPCGateway.
type VPCGatewayList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VPCGateway `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VPCGateway{}, &VPCGatewayList{})
}
