package network

// NetworkInfo describes a Kubernetes network definition (CUDN or UDN).
type NetworkInfo struct {
	// Name of the network resource
	Name string
	// Namespace of the network resource (empty for cluster-scoped CUDNs)
	Namespace string
	// Topology is "LocalNet" or "Layer2"
	Topology string
	// Role is "Primary" or "Secondary"
	Role string
	// Kind is "ClusterUserDefinedNetwork" or "UserDefinedNetwork"
	Kind string
}

// IsLocalNet returns true if the network uses LocalNet topology (requires VPC resources).
func (n *NetworkInfo) IsLocalNet() bool {
	return n.Topology == "LocalNet"
}

// IsLayer2 returns true if the network uses Layer2 topology (overlay-only, no VPC resources).
func (n *NetworkInfo) IsLayer2() bool {
	return n.Topology == "Layer2"
}

// VMNetworkInterface describes one network attachment on a VM.
// Serialized as JSON in the vpc.roks.ibm.com/network-interfaces annotation.
type VMNetworkInterface struct {
	// NetworkName is the name of the CUDN or UDN
	NetworkName string `json:"networkName"`
	// NetworkKind is "ClusterUserDefinedNetwork" or "UserDefinedNetwork"
	NetworkKind string `json:"networkKind"`
	// Topology is "LocalNet" or "Layer2"
	Topology string `json:"topology"`
	// Role is "Primary" or "Secondary"
	Role string `json:"role"`
	// InterfaceName is the name used in the VM spec (e.g., "net1")
	InterfaceName string `json:"interfaceName"`

	// VPC resource fields (only populated for LocalNet)
	VNIID        string `json:"vniId,omitempty"`
	MACAddress   string `json:"macAddress,omitempty"`
	ReservedIP   string `json:"reservedIp,omitempty"`
	ReservedIPID string `json:"reservedIpId,omitempty"`
	FIPID        string `json:"fipId,omitempty"`
	FIPAddress   string `json:"fipAddress,omitempty"`
}
