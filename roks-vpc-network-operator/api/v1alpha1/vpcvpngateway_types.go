package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// VPNWireGuardConfig defines global WireGuard configuration for the VPN gateway.
type VPNWireGuardConfig struct {
	// PrivateKey is a reference to a Secret containing the WireGuard private key.
	// +kubebuilder:validation:Required
	PrivateKey SecretKeyRef `json:"privateKey"`

	// ListenPort is the UDP port for WireGuard.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:default=51820
	// +optional
	ListenPort *int32 `json:"listenPort,omitempty"`
}

// VPNIPsecPolicy defines IKE and IPsec policy parameters.
type VPNIPsecPolicy struct {
	// IKEVersion is the IKE protocol version (1 or 2).
	// +kubebuilder:validation:Enum=1;2
	// +kubebuilder:default=2
	// +optional
	IKEVersion *int32 `json:"ikeVersion,omitempty"`

	// Encryption is the encryption algorithm.
	// +kubebuilder:validation:Enum=aes128;aes256;aes128gcm16;aes256gcm16
	// +kubebuilder:default=aes256
	// +optional
	Encryption string `json:"encryption,omitempty"`

	// Integrity is the integrity/authentication algorithm.
	// +kubebuilder:validation:Enum=sha256;sha384;sha512
	// +kubebuilder:default=sha256
	// +optional
	Integrity string `json:"integrity,omitempty"`

	// DHGroup is the Diffie-Hellman group.
	// +kubebuilder:validation:Enum=14;15;16;19;20;21
	// +kubebuilder:default=14
	// +optional
	DHGroup *int32 `json:"dhGroup,omitempty"`
}

// VPNIPsecConfig defines global IPsec/StrongSwan configuration for the VPN gateway.
type VPNIPsecConfig struct {
	// IKEPolicy defines the IKE policy parameters.
	// +optional
	IKEPolicy *VPNIPsecPolicy `json:"ikePolicy,omitempty"`

	// IPsecPolicy defines the IPsec (child SA) policy parameters.
	// +optional
	IPsecPolicy *VPNIPsecPolicy `json:"ipsecPolicy,omitempty"`

	// Image overrides the default StrongSwan container image.
	// +optional
	Image string `json:"image,omitempty"`
}

// VPNTunnel defines a single VPN tunnel to a remote peer.
type VPNTunnel struct {
	// Name is a unique identifier for this tunnel.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// RemoteEndpoint is the IP address or hostname of the remote VPN peer.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	RemoteEndpoint string `json:"remoteEndpoint"`

	// RemoteNetworks is a list of CIDRs reachable via this tunnel.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	RemoteNetworks []string `json:"remoteNetworks"`

	// PeerPublicKey is the WireGuard public key of the remote peer.
	// Required when protocol is "wireguard".
	// +optional
	PeerPublicKey string `json:"peerPublicKey,omitempty"`

	// TunnelAddressLocal is the local tunnel IP address (e.g., "10.99.0.1/30").
	// Required when protocol is "wireguard".
	// +optional
	TunnelAddressLocal string `json:"tunnelAddressLocal,omitempty"`

	// TunnelAddressRemote is the remote tunnel IP address (e.g., "10.99.0.2/30").
	// Required when protocol is "wireguard".
	// +optional
	TunnelAddressRemote string `json:"tunnelAddressRemote,omitempty"`

	// PresharedKey is a reference to a Secret containing the IPsec pre-shared key.
	// Required when protocol is "ipsec".
	// +optional
	PresharedKey *SecretKeyRef `json:"presharedKey,omitempty"`
}

// VPNRemoteAccess defines client-to-site VPN configuration.
type VPNRemoteAccess struct {
	// Enabled controls whether remote access (client-to-site) is active.
	// +kubebuilder:default=false
	Enabled bool `json:"enabled"`

	// AddressPool is the IP address pool for remote clients (e.g., "10.200.0.0/24").
	// +kubebuilder:validation:MinLength=1
	// +optional
	AddressPool string `json:"addressPool,omitempty"`

	// DNSServers is a list of DNS server addresses pushed to remote clients.
	// +optional
	DNSServers []string `json:"dnsServers,omitempty"`

	// MaxClients is the maximum number of concurrent remote access clients.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=10
	// +optional
	MaxClients *int32 `json:"maxClients,omitempty"`
}

// VPNLocalNetwork identifies a local network to advertise to VPN peers.
type VPNLocalNetwork struct {
	// NetworkRef is a reference to a CUDN or UDN resource. The network's CIDR
	// is resolved automatically from the VPC subnet.
	// +optional
	NetworkRef *BridgeNetworkRef `json:"networkRef,omitempty"`

	// CIDR is an explicit CIDR to advertise (used when not referencing a network resource).
	// +optional
	CIDR string `json:"cidr,omitempty"`
}

// VPNGatewayMTU defines MTU and TCP MSS clamping for VPN tunnels.
type VPNGatewayMTU struct {
	// TunnelMTU is the MTU for the tunnel interface.
	// +kubebuilder:validation:Minimum=1200
	// +kubebuilder:validation:Maximum=9000
	// +kubebuilder:default=1420
	// +optional
	TunnelMTU *int32 `json:"tunnelMTU,omitempty"`

	// MSSClamp enables TCP MSS clamping on tunnel traffic to prevent fragmentation.
	// +kubebuilder:default=true
	// +optional
	MSSClamp *bool `json:"mssClamp,omitempty"`
}

// VPNOpenVPNConfig defines global OpenVPN configuration for the VPN gateway.
type VPNOpenVPNConfig struct {
	// CA is a reference to a Secret containing the CA certificate.
	// +kubebuilder:validation:Required
	CA SecretKeyRef `json:"ca"`

	// Cert is a reference to a Secret containing the server certificate.
	// +kubebuilder:validation:Required
	Cert SecretKeyRef `json:"cert"`

	// Key is a reference to a Secret containing the server private key.
	// +kubebuilder:validation:Required
	Key SecretKeyRef `json:"key"`

	// DH is a reference to a Secret containing Diffie-Hellman parameters.
	// Optional — omit to use ECDH.
	// +optional
	DH *SecretKeyRef `json:"dh,omitempty"`

	// TLSAuth is a reference to a Secret containing the TLS-Auth HMAC key.
	// +optional
	TLSAuth *SecretKeyRef `json:"tlsAuth,omitempty"`

	// ListenPort is the OpenVPN listen port.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:default=1194
	// +optional
	ListenPort *int32 `json:"listenPort,omitempty"`

	// Proto is the transport protocol: "udp" (default) or "tcp".
	// +kubebuilder:validation:Enum=udp;tcp
	// +kubebuilder:default=udp
	// +optional
	Proto string `json:"proto,omitempty"`

	// Cipher is the data channel cipher.
	// +kubebuilder:default="AES-256-GCM"
	// +optional
	Cipher string `json:"cipher,omitempty"`

	// ClientSubnet is the CIDR for the remote-access client IP pool (e.g., "10.8.0.0/24").
	// Required when remoteAccess is enabled.
	// +optional
	ClientSubnet string `json:"clientSubnet,omitempty"`

	// Image overrides the default OpenVPN container image.
	// +optional
	Image string `json:"image,omitempty"`
}

// VPCVPNGatewaySpec defines the desired state of a VPCVPNGateway.
type VPCVPNGatewaySpec struct {
	// Protocol is the VPN protocol to use.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=wireguard;ipsec;openvpn
	Protocol string `json:"protocol"`

	// GatewayRef is the name of the VPCGateway this VPN gateway is associated with.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	GatewayRef string `json:"gatewayRef"`

	// WireGuard contains global WireGuard configuration.
	// Required when protocol is "wireguard".
	// +optional
	WireGuard *VPNWireGuardConfig `json:"wireGuard,omitempty"`

	// IPsec contains global IPsec/StrongSwan configuration.
	// Required when protocol is "ipsec".
	// +optional
	IPsec *VPNIPsecConfig `json:"ipsec,omitempty"`

	// OpenVPN contains global OpenVPN configuration.
	// Required when protocol is "openvpn".
	// +optional
	OpenVPN *VPNOpenVPNConfig `json:"openVPN,omitempty"`

	// Tunnels is the list of VPN tunnels to remote peers.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Tunnels []VPNTunnel `json:"tunnels"`

	// RemoteAccess defines optional client-to-site VPN configuration.
	// +optional
	RemoteAccess *VPNRemoteAccess `json:"remoteAccess,omitempty"`

	// LocalNetworks lists the local networks to advertise to VPN peers.
	// +optional
	LocalNetworks []VPNLocalNetwork `json:"localNetworks,omitempty"`

	// MTU defines MTU and TCP MSS clamping settings for VPN tunnels.
	// +optional
	MTU *VPNGatewayMTU `json:"mtu,omitempty"`

	// Pod defines pod-level overrides for the VPN gateway pod.
	// +optional
	Pod *RouterPodSpec `json:"pod,omitempty"`
}

// VPNTunnelStatus reports the observed state of a single VPN tunnel.
type VPNTunnelStatus struct {
	// Name is the tunnel identifier.
	Name string `json:"name"`

	// Status is the tunnel connection status.
	// +kubebuilder:validation:Enum=Up;Down;Connecting
	Status string `json:"status"`

	// LastHandshake is the timestamp of the last successful handshake.
	// +optional
	LastHandshake *metav1.Time `json:"lastHandshake,omitempty"`

	// BytesIn is the total bytes received through this tunnel.
	BytesIn int64 `json:"bytesIn,omitempty"`

	// BytesOut is the total bytes sent through this tunnel.
	BytesOut int64 `json:"bytesOut,omitempty"`
}

// VPCVPNGatewayStatus defines the observed state of a VPCVPNGateway.
type VPCVPNGatewayStatus struct {
	// Phase is the current lifecycle phase of the VPN gateway.
	// +kubebuilder:validation:Enum=Pending;Provisioning;Ready;Degraded;Error
	Phase string `json:"phase,omitempty"`

	// TunnelEndpoint is the public IP used as the VPN endpoint.
	TunnelEndpoint string `json:"tunnelEndpoint,omitempty"`

	// PodName is the name of the VPN gateway pod.
	PodName string `json:"podName,omitempty"`

	// Tunnels reports per-tunnel status.
	// +optional
	Tunnels []VPNTunnelStatus `json:"tunnels,omitempty"`

	// AdvertisedRoutes is the list of remote CIDRs collected from all tunnels
	// for the VPCGateway to create VPC routes.
	// +optional
	AdvertisedRoutes []string `json:"advertisedRoutes,omitempty"`

	// ActiveTunnels is the number of tunnels in the Up state.
	ActiveTunnels int32 `json:"activeTunnels,omitempty"`

	// TotalTunnels is the total number of configured tunnels.
	TotalTunnels int32 `json:"totalTunnels,omitempty"`

	// ConnectedClients is the number of currently connected remote access clients.
	ConnectedClients int32 `json:"connectedClients,omitempty"`

	// SyncStatus indicates whether the CR is in sync.
	// +kubebuilder:validation:Enum=Synced;Pending;Failed
	SyncStatus string `json:"syncStatus,omitempty"`

	// LastSyncTime is the timestamp of the last successful sync.
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

	// Message provides human-readable detail about the current status.
	Message string `json:"message,omitempty"`

	// Conditions represent the latest available observations of the VPN gateway's state.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=vvg
// +kubebuilder:printcolumn:name="Protocol",type=string,JSONPath=`.spec.protocol`
// +kubebuilder:printcolumn:name="Gateway",type=string,JSONPath=`.spec.gatewayRef`
// +kubebuilder:printcolumn:name="Tunnels",type=string,JSONPath=`.status.activeTunnels`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Sync",type=string,JSONPath=`.status.syncStatus`
// +kubebuilder:printcolumn:name="Endpoint",type=string,JSONPath=`.status.tunnelEndpoint`,priority=1
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// VPCVPNGateway is the Schema for the vpcvpngateways API.
// It represents a VPN gateway that provides site-to-site and client-to-site VPN
// connectivity for VM workload networks.
type VPCVPNGateway struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VPCVPNGatewaySpec   `json:"spec,omitempty"`
	Status VPCVPNGatewayStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// VPCVPNGatewayList contains a list of VPCVPNGateway.
type VPCVPNGatewayList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VPCVPNGateway `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VPCVPNGateway{}, &VPCVPNGatewayList{})
}
