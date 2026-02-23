package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// VNISpec defines the desired state of a Virtual Network Interface.
type VNISpec struct {
	// SubnetRef is the name of the VPCSubnet CR this VNI belongs to.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	SubnetRef string `json:"subnetRef"`

	// SubnetID is the VPC subnet ID (alternative to SubnetRef for direct ID reference).
	// +optional
	SubnetID string `json:"subnetID,omitempty"`

	// SecurityGroupIDs is a list of VPC security group IDs to associate with this VNI.
	// +optional
	SecurityGroupIDs []string `json:"securityGroupIDs,omitempty"`

	// AllowIPSpoofing enables IP spoofing on the VNI (required for KubeVirt VMs).
	// +kubebuilder:default=true
	AllowIPSpoofing bool `json:"allowIPSpoofing"`

	// EnableInfrastructureNat controls whether infrastructure NAT is enabled.
	// Set to false for VMs that need their own routable IP.
	// +kubebuilder:default=false
	EnableInfrastructureNat bool `json:"enableInfrastructureNat"`

	// AutoDelete controls whether the VNI is deleted when its target is removed.
	// Set to false for KubeVirt VMs to survive live migration.
	// +kubebuilder:default=false
	AutoDelete bool `json:"autoDelete"`

	// VMRef is a reference to the KubeVirt VirtualMachine this VNI is bound to.
	// +optional
	VMRef *VMReference `json:"vmRef,omitempty"`

	// ClusterID is the ROKS cluster ID, used for tagging.
	// +optional
	ClusterID string `json:"clusterID,omitempty"`
}

// VMReference identifies a KubeVirt VirtualMachine.
type VMReference struct {
	// Namespace of the VirtualMachine.
	Namespace string `json:"namespace"`
	// Name of the VirtualMachine.
	Name string `json:"name"`
}

// VNIStatus defines the observed state of a Virtual Network Interface.
type VNIStatus struct {
	// SyncStatus indicates whether the CR is in sync with the VPC API.
	// +kubebuilder:validation:Enum=Synced;Pending;Failed
	SyncStatus string `json:"syncStatus,omitempty"`

	// VNIID is the VPC-assigned Virtual Network Interface ID.
	VNIID string `json:"vniID,omitempty"`

	// MACAddress is the VPC-generated MAC address for this VNI.
	MACAddress string `json:"macAddress,omitempty"`

	// PrimaryIPv4 is the primary reserved IP address on the VNI.
	PrimaryIPv4 string `json:"primaryIPv4,omitempty"`

	// ReservedIPID is the VPC reserved IP resource ID.
	ReservedIPID string `json:"reservedIPID,omitempty"`

	// LastSyncTime is the timestamp of the last successful sync.
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

	// Message provides human-readable detail about the current status.
	Message string `json:"message,omitempty"`

	// Conditions represent the latest available observations of the VNI's state.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=vni
// +kubebuilder:printcolumn:name="VNI ID",type=string,JSONPath=`.status.vniID`
// +kubebuilder:printcolumn:name="MAC",type=string,JSONPath=`.status.macAddress`
// +kubebuilder:printcolumn:name="IP",type=string,JSONPath=`.status.primaryIPv4`
// +kubebuilder:printcolumn:name="Subnet",type=string,JSONPath=`.spec.subnetRef`
// +kubebuilder:printcolumn:name="Sync",type=string,JSONPath=`.status.syncStatus`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// VirtualNetworkInterface is the Schema for the virtualnetworkinterfaces API.
// It represents a VPC Virtual Network Interface managed by the operator.
type VirtualNetworkInterface struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VNISpec   `json:"spec,omitempty"`
	Status VNIStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// VirtualNetworkInterfaceList contains a list of VirtualNetworkInterface.
type VirtualNetworkInterfaceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VirtualNetworkInterface `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VirtualNetworkInterface{}, &VirtualNetworkInterfaceList{})
}
