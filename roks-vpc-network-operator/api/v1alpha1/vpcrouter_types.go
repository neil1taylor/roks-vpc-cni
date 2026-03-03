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

	// DHCP provides per-network DHCP overrides.
	// +optional
	DHCP *NetworkDHCP `json:"dhcp,omitempty"`
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

	// LeaseTime is the default DHCP lease duration (e.g. "12h", "1h", "30m").
	// Defaults to "12h" if not specified.
	// +optional
	LeaseTime string `json:"leaseTime,omitempty"`

	// DNS configures DNS settings for DHCP responses.
	// +optional
	DNS *DHCPDNSConfig `json:"dns,omitempty"`

	// Options configures additional DHCP options.
	// +optional
	Options *DHCPOptions `json:"options,omitempty"`
}

// NetworkDHCP provides per-network DHCP overrides.
type NetworkDHCP struct {
	// Enabled overrides the global DHCP enabled setting for this network.
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// Range defines a custom DHCP address range for this network.
	// +optional
	Range *NetworkDHCPRange `json:"range,omitempty"`

	// LeaseTime overrides the global lease duration for this network.
	// +optional
	LeaseTime string `json:"leaseTime,omitempty"`

	// Reservations defines static MAC→IP reservations for this network.
	// +optional
	Reservations []DHCPStaticReservation `json:"reservations,omitempty"`

	// DNS overrides the global DNS settings for this network.
	// +optional
	DNS *DHCPDNSConfig `json:"dns,omitempty"`

	// Options overrides the global DHCP options for this network.
	// +optional
	Options *DHCPOptions `json:"options,omitempty"`
}

// NetworkDHCPRange defines custom start/end addresses for a DHCP pool.
type NetworkDHCPRange struct {
	// Start is the first IP address in the DHCP pool.
	// +kubebuilder:validation:Required
	Start string `json:"start"`

	// End is the last IP address in the DHCP pool.
	// +kubebuilder:validation:Required
	End string `json:"end"`
}

// DHCPStaticReservation maps a MAC address to a fixed IP.
type DHCPStaticReservation struct {
	// MAC is the hardware address (e.g. "fa:16:3e:aa:bb:cc").
	// +kubebuilder:validation:Required
	MAC string `json:"mac"`

	// IP is the reserved IP address (e.g. "10.100.0.50").
	// +kubebuilder:validation:Required
	IP string `json:"ip"`

	// Hostname is an optional hostname for the reservation.
	// +optional
	Hostname string `json:"hostname,omitempty"`
}

// DHCPDNSConfig configures DNS settings for DHCP responses.
type DHCPDNSConfig struct {
	// Nameservers is a list of DNS server IP addresses (DHCP option 6).
	// +optional
	Nameservers []string `json:"nameservers,omitempty"`

	// SearchDomains is a list of DNS search domains (DHCP option 119).
	// +optional
	SearchDomains []string `json:"searchDomains,omitempty"`

	// LocalDomain sets the local domain name for DHCP clients.
	// +optional
	LocalDomain string `json:"localDomain,omitempty"`
}

// DHCPOptions configures additional DHCP options.
type DHCPOptions struct {
	// Router overrides the default gateway (DHCP option 3).
	// +optional
	Router string `json:"router,omitempty"`

	// MTU sets the interface MTU for DHCP clients (DHCP option 26).
	// +optional
	MTU *int32 `json:"mtu,omitempty"`

	// NTPServers is a list of NTP server addresses (DHCP option 42).
	// +optional
	NTPServers []string `json:"ntpServers,omitempty"`

	// Custom is a list of raw dnsmasq --dhcp-option values for passthrough.
	// +optional
	Custom []string `json:"custom,omitempty"`
}

// DHCPNetworkStatus reports the DHCP status of a single network.
type DHCPNetworkStatus struct {
	// Enabled indicates whether DHCP is active on this network.
	Enabled bool `json:"enabled"`

	// PoolStart is the first IP in the DHCP range.
	// +optional
	PoolStart string `json:"poolStart,omitempty"`

	// PoolEnd is the last IP in the DHCP range.
	// +optional
	PoolEnd string `json:"poolEnd,omitempty"`

	// ReservationCount is the number of static reservations configured.
	// +optional
	ReservationCount int32 `json:"reservationCount,omitempty"`
}

// RouterNetworkStatus reports the status of a network attached to the router.
type RouterNetworkStatus struct {
	// Name is the network name.
	Name string `json:"name"`

	// Address is the router's IP address on this network.
	Address string `json:"address"`

	// Connected indicates whether the router has connectivity to this network.
	Connected bool `json:"connected"`

	// DHCP reports the DHCP status of this network.
	// +optional
	DHCP *DHCPNetworkStatus `json:"dhcp,omitempty"`
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

// RouterMetrics defines the metrics exporter sidecar configuration.
type RouterMetrics struct {
	// Enabled controls whether the metrics exporter sidecar is deployed.
	// +kubebuilder:default=false
	Enabled bool `json:"enabled"`

	// Port is the port the metrics exporter listens on.
	// +kubebuilder:default=9100
	// +optional
	Port *int32 `json:"port,omitempty"`

	// Image overrides the default metrics exporter container image.
	// +optional
	Image string `json:"image,omitempty"`
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

	// Metrics configures the metrics exporter sidecar container.
	// +optional
	Metrics *RouterMetrics `json:"metrics,omitempty"`

	// Pod defines pod-level overrides for the router pod.
	// +optional
	Pod *RouterPodSpec `json:"pod,omitempty"`

	// Mode selects the router runtime mode.
	// "standard" uses a Fedora container with bash init script.
	// "fast-path" uses a purpose-built Go binary with optional XDP/eBPF acceleration.
	// +kubebuilder:validation:Enum=standard;fast-path
	// +kubebuilder:default=standard
	// +optional
	Mode string `json:"mode,omitempty"`
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

	// MetricsEnabled reports whether the metrics exporter sidecar is active.
	MetricsEnabled bool `json:"metricsEnabled,omitempty"`

	// Mode reports the active router mode ("standard" or "fast-path").
	Mode string `json:"mode,omitempty"`

	// XDPEnabled reports whether XDP/eBPF fast-path forwarding is active.
	XDPEnabled bool `json:"xdpEnabled,omitempty"`

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
// +kubebuilder:printcolumn:name="Mode",type=string,JSONPath=`.spec.mode`,priority=1
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
