package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// FloatingIPSpec defines the desired state of a Floating IP.
type FloatingIPSpec struct {
	// Zone is the VPC zone for the floating IP (e.g., "us-south-1").
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Zone string `json:"zone"`

	// VNIRef is the name of the VirtualNetworkInterface CR to bind this FIP to.
	// +optional
	VNIRef string `json:"vniRef,omitempty"`

	// VNIID is the VPC VNI ID to bind to (alternative to VNIRef).
	// +optional
	VNIID string `json:"vniID,omitempty"`

	// Name is the desired name for the floating IP in VPC.
	// +optional
	Name string `json:"name,omitempty"`
}

// FloatingIPStatus defines the observed state of a Floating IP.
type FloatingIPStatus struct {
	// SyncStatus indicates whether the CR is in sync with the VPC API.
	// +kubebuilder:validation:Enum=Synced;Pending;Failed
	SyncStatus string `json:"syncStatus,omitempty"`

	// FIPID is the VPC-assigned floating IP resource ID.
	FIPID string `json:"fipID,omitempty"`

	// Address is the allocated public IP address.
	Address string `json:"address,omitempty"`

	// TargetVNIID is the VNI this FIP is currently bound to.
	TargetVNIID string `json:"targetVNIID,omitempty"`

	// LastSyncTime is the timestamp of the last successful sync.
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

	// Message provides human-readable detail about the current status.
	Message string `json:"message,omitempty"`

	// Conditions represent the latest available observations.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=fip
// +kubebuilder:printcolumn:name="Address",type=string,JSONPath=`.status.address`
// +kubebuilder:printcolumn:name="Zone",type=string,JSONPath=`.spec.zone`
// +kubebuilder:printcolumn:name="VNI",type=string,JSONPath=`.spec.vniRef`
// +kubebuilder:printcolumn:name="Sync",type=string,JSONPath=`.status.syncStatus`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// FloatingIP is the Schema for the floatingips API.
// It represents a VPC floating IP managed by the operator.
type FloatingIP struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FloatingIPSpec   `json:"spec,omitempty"`
	Status FloatingIPStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// FloatingIPList contains a list of FloatingIP.
type FloatingIPList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FloatingIP `json:"items"`
}

func init() {
	SchemeBuilder.Register(&FloatingIP{}, &FloatingIPList{})
}
