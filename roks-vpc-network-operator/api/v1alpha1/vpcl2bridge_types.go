package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SecretKeyRef is a reference to a key in a Kubernetes Secret.
type SecretKeyRef struct {
	// Name is the name of the Secret.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Key is the key within the Secret data.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Key string `json:"key"`
}

// BridgeNetworkRef identifies the network this bridge extends.
type BridgeNetworkRef struct {
	// Name is the name of the network resource.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Kind is the kind of the network resource.
	// +kubebuilder:validation:Enum=ClusterUserDefinedNetwork;UserDefinedNetwork
	// +kubebuilder:default=ClusterUserDefinedNetwork
	// +optional
	Kind string `json:"kind,omitempty"`

	// Namespace is the namespace of the network resource (required when Kind is UserDefinedNetwork).
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// BridgeWireGuard defines WireGuard tunnel configuration.
type BridgeWireGuard struct {
	// PrivateKey is a reference to a Secret containing the WireGuard private key.
	// +kubebuilder:validation:Required
	PrivateKey SecretKeyRef `json:"privateKey"`

	// PeerPublicKey is the WireGuard public key of the remote peer.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	PeerPublicKey string `json:"peerPublicKey"`

	// ListenPort is the UDP port for WireGuard.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:default=51820
	// +optional
	ListenPort *int32 `json:"listenPort,omitempty"`

	// TunnelAddressLocal is the local tunnel IP address (e.g., "10.99.0.1/30").
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	TunnelAddressLocal string `json:"tunnelAddressLocal"`

	// TunnelAddressRemote is the remote tunnel IP address (e.g., "10.99.0.2/30").
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	TunnelAddressRemote string `json:"tunnelAddressRemote"`
}

// BridgeL2VPN defines NSX-T L2VPN configuration.
type BridgeL2VPN struct {
	// NSXManagerHost is the hostname or IP of the NSX-T Manager.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	NSXManagerHost string `json:"nsxManagerHost"`

	// L2VPNServiceID is the NSX-T L2VPN service ID.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	L2VPNServiceID string `json:"l2vpnServiceID"`

	// Credentials is a reference to a Secret containing NSX-T credentials.
	// +kubebuilder:validation:Required
	Credentials SecretKeyRef `json:"credentials"`

	// EdgeImage overrides the default NSX-T edge container image.
	// +optional
	EdgeImage string `json:"edgeImage,omitempty"`
}

// BridgeEVPN defines EVPN-VXLAN configuration using FRR.
type BridgeEVPN struct {
	// ASN is the local BGP Autonomous System Number.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=4294967295
	ASN int64 `json:"asn"`

	// VNI is the VXLAN Network Identifier.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=16777215
	VNI int32 `json:"vni"`

	// PeerASN is the remote BGP Autonomous System Number.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=4294967295
	PeerASN int64 `json:"peerASN"`

	// RouteReflector is the IP address of the BGP route reflector.
	// +optional
	RouteReflector string `json:"routeReflector,omitempty"`

	// FRRImage overrides the default FRR container image.
	// +optional
	FRRImage string `json:"frrImage,omitempty"`
}

// BridgeRemote defines the remote endpoint and tunnel-type-specific configuration.
type BridgeRemote struct {
	// Endpoint is the remote endpoint IP address or hostname.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Endpoint string `json:"endpoint"`

	// WireGuard contains WireGuard-specific tunnel configuration.
	// Required when spec.type is "gretap-wireguard".
	// +optional
	WireGuard *BridgeWireGuard `json:"wireGuard,omitempty"`

	// L2VPN contains NSX-T L2VPN-specific configuration.
	// Required when spec.type is "l2vpn".
	// +optional
	L2VPN *BridgeL2VPN `json:"l2vpn,omitempty"`

	// EVPN contains EVPN-VXLAN-specific configuration.
	// Required when spec.type is "evpn-vxlan".
	// +optional
	EVPN *BridgeEVPN `json:"evpn,omitempty"`
}

// BridgeMTU defines MTU and TCP MSS clamping for the tunnel.
type BridgeMTU struct {
	// TunnelMTU is the MTU for the tunnel interface.
	// +kubebuilder:validation:Minimum=1200
	// +kubebuilder:validation:Maximum=9000
	// +kubebuilder:default=1400
	// +optional
	TunnelMTU *int32 `json:"tunnelMTU,omitempty"`

	// MSSClamp enables TCP MSS clamping on tunnel traffic to prevent fragmentation.
	// +kubebuilder:default=true
	// +optional
	MSSClamp *bool `json:"mssClamp,omitempty"`
}

// VPCL2BridgeSpec defines the desired state of a VPCL2Bridge.
type VPCL2BridgeSpec struct {
	// Type is the tunnel technology used for L2 bridging.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=gretap-wireguard;l2vpn;evpn-vxlan
	Type string `json:"type"`

	// GatewayRef is the name of the VPCGateway this bridge is associated with.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	GatewayRef string `json:"gatewayRef"`

	// NetworkRef identifies the network this bridge extends to the remote site.
	// +kubebuilder:validation:Required
	NetworkRef BridgeNetworkRef `json:"networkRef"`

	// Remote defines the remote endpoint and tunnel-type-specific configuration.
	// +kubebuilder:validation:Required
	Remote BridgeRemote `json:"remote"`

	// MTU defines MTU and TCP MSS clamping settings for the tunnel.
	// +optional
	MTU *BridgeMTU `json:"mtu,omitempty"`

	// Pod defines pod-level overrides for the bridge pod.
	// +optional
	Pod *RouterPodSpec `json:"pod,omitempty"`
}

// VPCL2BridgeStatus defines the observed state of a VPCL2Bridge.
type VPCL2BridgeStatus struct {
	// Phase is the current lifecycle phase of the bridge.
	// +kubebuilder:validation:Enum=Pending;Provisioning;Established;Degraded;Error
	Phase string `json:"phase,omitempty"`

	// TunnelEndpoint is the local tunnel endpoint IP address.
	TunnelEndpoint string `json:"tunnelEndpoint,omitempty"`

	// RemoteMACsLearned is the number of remote MAC addresses learned via the tunnel.
	RemoteMACsLearned int32 `json:"remoteMACsLearned,omitempty"`

	// LocalMACsAdvertised is the number of local MAC addresses advertised to the remote peer.
	LocalMACsAdvertised int32 `json:"localMACsAdvertised,omitempty"`

	// BytesIn is the total bytes received through the tunnel.
	BytesIn int64 `json:"bytesIn,omitempty"`

	// BytesOut is the total bytes sent through the tunnel.
	BytesOut int64 `json:"bytesOut,omitempty"`

	// LastHandshake is the timestamp of the last successful tunnel handshake.
	LastHandshake *metav1.Time `json:"lastHandshake,omitempty"`

	// PodName is the name of the bridge pod.
	PodName string `json:"podName,omitempty"`

	// SyncStatus indicates whether the CR is in sync with the VPC API.
	// +kubebuilder:validation:Enum=Synced;Pending;Failed
	SyncStatus string `json:"syncStatus,omitempty"`

	// LastSyncTime is the timestamp of the last successful sync.
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

	// Message provides human-readable detail about the current status.
	Message string `json:"message,omitempty"`

	// Conditions represent the latest available observations of the bridge's state.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=vlb
// +kubebuilder:printcolumn:name="Type",type=string,JSONPath=`.spec.type`
// +kubebuilder:printcolumn:name="Network",type=string,JSONPath=`.spec.networkRef.name`
// +kubebuilder:printcolumn:name="Remote",type=string,JSONPath=`.spec.remote.endpoint`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Sync",type=string,JSONPath=`.status.syncStatus`
// +kubebuilder:printcolumn:name="Tunnel IP",type=string,JSONPath=`.status.tunnelEndpoint`,priority=1
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// VPCL2Bridge is the Schema for the vpcl2bridges API.
// It represents an L2 bridge that extends a VPC network segment to a remote site via a tunnel.
type VPCL2Bridge struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VPCL2BridgeSpec   `json:"spec,omitempty"`
	Status VPCL2BridgeStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// VPCL2BridgeList contains a list of VPCL2Bridge.
type VPCL2BridgeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VPCL2Bridge `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VPCL2Bridge{}, &VPCL2BridgeList{})
}
