package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// VLANAttachmentSpec defines the desired state of a VLAN attachment.
type VLANAttachmentSpec struct {
	// BMServerID is the bare metal server ID to attach the VLAN to.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	BMServerID string `json:"bmServerID"`

	// VLANID is the VLAN tag for the attachment.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=4094
	VLANID int64 `json:"vlanID"`

	// SubnetRef is the name of the VPCSubnet CR this attachment is associated with.
	// +kubebuilder:validation:Required
	SubnetRef string `json:"subnetRef"`

	// SubnetID is the VPC subnet ID (alternative to SubnetRef).
	// +optional
	SubnetID string `json:"subnetID,omitempty"`

	// AllowToFloat enables the VLAN attachment to float during live migration.
	// +kubebuilder:default=true
	AllowToFloat bool `json:"allowToFloat"`

	// NodeName is the Kubernetes node name corresponding to the bare metal server.
	// +optional
	NodeName string `json:"nodeName,omitempty"`
}

// VLANAttachmentStatus defines the observed state of a VLAN attachment.
type VLANAttachmentStatus struct {
	// SyncStatus indicates whether the CR is in sync with the VPC API.
	// +kubebuilder:validation:Enum=Synced;Pending;Failed
	SyncStatus string `json:"syncStatus,omitempty"`

	// AttachmentID is the VPC-assigned VLAN attachment ID.
	AttachmentID string `json:"attachmentID,omitempty"`

	// AttachmentStatus is the current status of the attachment.
	// +kubebuilder:validation:Enum=attached;pending;detached;failed
	AttachmentStatus string `json:"attachmentStatus,omitempty"`

	// LastSyncTime is the timestamp of the last successful sync.
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

	// Message provides human-readable detail about the current status.
	Message string `json:"message,omitempty"`

	// Conditions represent the latest available observations.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=vla
// +kubebuilder:printcolumn:name="BM Server",type=string,JSONPath=`.spec.bmServerID`
// +kubebuilder:printcolumn:name="VLAN",type=integer,JSONPath=`.spec.vlanID`
// +kubebuilder:printcolumn:name="Node",type=string,JSONPath=`.spec.nodeName`
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.attachmentStatus`
// +kubebuilder:printcolumn:name="Sync",type=string,JSONPath=`.status.syncStatus`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// VLANAttachment is the Schema for the vlanattachments API.
// It represents a VLAN interface attachment on a bare metal server.
type VLANAttachment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VLANAttachmentSpec   `json:"spec,omitempty"`
	Status VLANAttachmentStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// VLANAttachmentList contains a list of VLANAttachment.
type VLANAttachmentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VLANAttachment `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VLANAttachment{}, &VLANAttachmentList{})
}
