package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RouterTransit defines the transit network configuration for a router.
type RouterTransit struct {
	// Network is the name of the transit L2 network.
	// +optional
	Network string `json:"network,omitempty"`

	// Address is the IP address on the transit network.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Address string `json:"address"`
}

// RouterNetwork defines a workload network attached to the router.
type RouterNetwork struct {
	// Name is the name of the network (CUDN or UDN).
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Namespace is the namespace where the network-attachment-definition exists.
	// Required when the router pod runs in a different namespace than the NAD.
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Address is the IP address on this network.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Address string `json:"address"`
}

// RouteAdvertisement controls which routes the router advertises.
type RouteAdvertisement struct {
	// ConnectedSegments advertises routes for directly connected network segments.
	// +kubebuilder:default=true
	ConnectedSegments bool `json:"connectedSegments"`

	// StaticRoutes advertises configured static routes.
	// +kubebuilder:default=false
	StaticRoutes bool `json:"staticRoutes"`

	// NATIPs advertises NAT-translated IP addresses.
	// +kubebuilder:default=false
	NATIPs bool `json:"natIPs"`
}

// RouterDHCP controls DHCP server functionality on the router.
type RouterDHCP struct {
	// Enabled controls whether the router acts as a DHCP server.
	// +kubebuilder:default=false
	Enabled bool `json:"enabled"`
}

// RouterNetworkStatus reports the status of a network attached to the router.
type RouterNetworkStatus struct {
	// Name is the network name.
	Name string `json:"name"`

	// Address is the router's IP address on this network.
	Address string `json:"address"`

	// Connected indicates whether the router has connectivity to this network.
	Connected bool `json:"connected"`
}

// RouterIDS defines IDS/IPS configuration using a Suricata sidecar container.
type RouterIDS struct {
	// Enabled controls whether the Suricata IDS/IPS sidecar is deployed.
	// +kubebuilder:default=false
	Enabled bool `json:"enabled"`

	// Mode selects the inspection mode: "ids" for passive monitoring (AF_PACKET),
	// "ips" for inline blocking (NFQUEUE).
	// +kubebuilder:validation:Enum=ids;ips
	// +kubebuilder:default=ids
	Mode string `json:"mode"`

	// Interfaces selects which interfaces Suricata monitors.
	// +kubebuilder:validation:Enum=all;uplink;workload
	// +kubebuilder:default=all
	// +optional
	Interfaces string `json:"interfaces,omitempty"`

	// CustomRules contains additional Suricata rules (one rule per line).
	// +optional
	CustomRules string `json:"customRules,omitempty"`

	// SyslogTarget is an optional syslog destination (host:port) for EVE JSON alerts.
	// +optional
	SyslogTarget string `json:"syslogTarget,omitempty"`

	// Image overrides the default Suricata container image.
	// +optional
	Image string `json:"image,omitempty"`

	// NFQueueNum is the NFQUEUE number used in IPS mode.
	// +kubebuilder:default=0
	// +optional
	NFQueueNum *int32 `json:"nfqueueNum,omitempty"`
}

// VPCRouterSpec defines the desired state of a VPCRouter.
type VPCRouterSpec struct {
	// Gateway is the name of the VPCGateway this router is associated with.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Gateway string `json:"gateway"`

	// Transit defines the transit network configuration for the router.
	// +optional
	Transit *RouterTransit `json:"transit,omitempty"`

	// Networks is the list of workload networks attached to the router.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Networks []RouterNetwork `json:"networks"`

	// RouteAdvertisement controls which routes the router advertises.
	// +optional
	RouteAdvertisement *RouteAdvertisement `json:"routeAdvertisement,omitempty"`

	// DHCP controls DHCP server functionality on the router.
	// +optional
	DHCP *RouterDHCP `json:"dhcp,omitempty"`

	// Firewall defines firewall rules for the router.
	// +optional
	Firewall *GatewayFirewall `json:"firewall,omitempty"`

	// IDS configures the Suricata IDS/IPS sidecar container.
	// +optional
	IDS *RouterIDS `json:"ids,omitempty"`

	// Pod defines pod-level overrides for the router pod.
	// +optional
	Pod *RouterPodSpec `json:"pod,omitempty"`
}

// VPCRouterStatus defines the observed state of a VPCRouter.
type VPCRouterStatus struct {
	// Phase is the current lifecycle phase of the router.
	// +kubebuilder:validation:Enum=Pending;Provisioning;Ready;Error
	Phase string `json:"phase,omitempty"`

	// PodIP is the cluster IP address of the router pod.
	PodIP string `json:"podIP,omitempty"`

	// TransitIP is the router's IP address on the transit network.
	TransitIP string `json:"transitIP,omitempty"`

	// IDSMode reports the active IDS/IPS mode ("ids", "ips", or "" if disabled).
	IDSMode string `json:"idsMode,omitempty"`

	// Networks reports the status of each attached network.
	// +optional
	Networks []RouterNetworkStatus `json:"networks,omitempty"`

	// AdvertisedRoutes is the list of routes the router is currently advertising.
	// +optional
	AdvertisedRoutes []string `json:"advertisedRoutes,omitempty"`

	// SyncStatus indicates whether the CR is in sync with the VPC API.
	// +kubebuilder:validation:Enum=Synced;Pending;Failed
	SyncStatus string `json:"syncStatus,omitempty"`

	// LastSyncTime is the timestamp of the last successful sync.
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

	// Message provides human-readable detail about the current status.
	Message string `json:"message,omitempty"`

	// Conditions represent the latest available observations of the router's state.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=vrt
// +kubebuilder:printcolumn:name="Gateway",type=string,JSONPath=`.spec.gateway`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Sync",type=string,JSONPath=`.status.syncStatus`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:printcolumn:name="IDS",type=string,JSONPath=`.status.idsMode`,priority=1

// VPCRouter is the Schema for the vpcrouters API.
// It represents a workload router that connects multiple L2 network segments via a transit network.
type VPCRouter struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VPCRouterSpec   `json:"spec,omitempty"`
	Status VPCRouterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// VPCRouterList contains a list of VPCRouter.
type VPCRouterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VPCRouter `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VPCRouter{}, &VPCRouterList{})
}
