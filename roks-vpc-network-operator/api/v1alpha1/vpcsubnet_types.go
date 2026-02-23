package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// VPCSubnetSpec defines the desired state of a VPC subnet.
type VPCSubnetSpec struct {
	// VPCID is the IBM Cloud VPC ID in which to create the subnet.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	VPCID string `json:"vpcID"`

	// Zone is the VPC availability zone (e.g., "us-south-1").
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Zone string `json:"zone"`

	// IPv4CIDRBlock is the CIDR block for the subnet (e.g., "10.240.0.0/24").
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^\d+\.\d+\.\d+\.\d+/\d+$`
	IPv4CIDRBlock string `json:"ipv4CIDRBlock"`

	// ACLID is the ID of the network ACL to associate with this subnet.
	// +optional
	ACLID string `json:"aclID,omitempty"`

	// ResourceGroupID is the IBM Cloud resource group for the subnet.
	// +optional
	ResourceGroupID string `json:"resourceGroupID,omitempty"`

	// SecurityGroupIDs is a list of security group IDs to attach to VNIs on this subnet.
	// +optional
	SecurityGroupIDs []string `json:"securityGroupIDs,omitempty"`

	// VLANID is the VLAN ID for OVN LocalNet and bare metal VLAN attachments.
	// +optional
	VLANID *int64 `json:"vlanID,omitempty"`

	// ClusterID is the ROKS cluster ID, used for tagging VPC resources.
	// +optional
	ClusterID string `json:"clusterID,omitempty"`

	// CUDNName is the name of the associated ClusterUserDefinedNetwork, if any.
	// +optional
	CUDNName string `json:"cudnName,omitempty"`
}

// VPCSubnetStatus defines the observed state of a VPC subnet.
type VPCSubnetStatus struct {
	// SyncStatus indicates whether the CR is in sync with the VPC API.
	// +kubebuilder:validation:Enum=Synced;Pending;Failed
	SyncStatus string `json:"syncStatus,omitempty"`

	// SubnetID is the VPC-assigned subnet ID.
	SubnetID string `json:"subnetID,omitempty"`

	// VPCSubnetStatus is the status of the subnet as reported by the VPC API.
	VPCSubnetStatus string `json:"vpcSubnetStatus,omitempty"`

	// AvailableIPv4 is the count of available IPv4 addresses in the subnet.
	AvailableIPv4 int64 `json:"availableIPv4,omitempty"`

	// LastSyncTime is the timestamp of the last successful sync with VPC API.
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

	// Message provides human-readable detail about the current status.
	Message string `json:"message,omitempty"`

	// Conditions represent the latest available observations of the subnet's state.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=vsn
// +kubebuilder:printcolumn:name="VPC",type=string,JSONPath=`.spec.vpcID`
// +kubebuilder:printcolumn:name="Zone",type=string,JSONPath=`.spec.zone`
// +kubebuilder:printcolumn:name="CIDR",type=string,JSONPath=`.spec.ipv4CIDRBlock`
// +kubebuilder:printcolumn:name="Sync",type=string,JSONPath=`.status.syncStatus`
// +kubebuilder:printcolumn:name="Subnet ID",type=string,JSONPath=`.status.subnetID`,priority=1
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// VPCSubnet is the Schema for the vpcsubnets API.
// It represents a VPC subnet managed by the ROKS VPC Network Operator.
type VPCSubnet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VPCSubnetSpec   `json:"spec,omitempty"`
	Status VPCSubnetStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// VPCSubnetList contains a list of VPCSubnet.
type VPCSubnetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VPCSubnet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VPCSubnet{}, &VPCSubnetList{})
}
