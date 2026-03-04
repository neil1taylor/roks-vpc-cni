package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DNSUpstreamServer defines an upstream DNS server.
type DNSUpstreamServer struct {
	// URL is the upstream DNS server address.
	// Use https:// for DoH, tls:// for DoT, or plain IP for standard DNS.
	// +kubebuilder:validation:Required
	URL string `json:"url"`
}

// DNSUpstreamConfig defines upstream DNS servers.
type DNSUpstreamConfig struct {
	// Servers is the list of upstream DNS servers.
	// +kubebuilder:validation:MinItems=1
	Servers []DNSUpstreamServer `json:"servers"`
}

// DNSFilteringConfig defines DNS filtering rules.
type DNSFilteringConfig struct {
	// Enabled controls whether DNS filtering is active.
	// +kubebuilder:default=false
	Enabled bool `json:"enabled"`

	// Blocklists is a list of URLs to blocklist files (hosts format).
	// +optional
	Blocklists []string `json:"blocklists,omitempty"`

	// Allowlist is a list of domain patterns to always allow.
	// +optional
	Allowlist []string `json:"allowlist,omitempty"`

	// Denylist is a list of domain patterns to always block.
	// +optional
	Denylist []string `json:"denylist,omitempty"`
}

// DNSLocalConfig defines local DNS resolution settings.
type DNSLocalConfig struct {
	// Enabled controls whether local DNS resolution is active.
	// +kubebuilder:default=false
	Enabled bool `json:"enabled"`

	// Domain is the local domain suffix (e.g. "vm.local").
	// +optional
	Domain string `json:"domain,omitempty"`
}

// VPCDNSPolicySpec defines the desired state of a VPCDNSPolicy.
type VPCDNSPolicySpec struct {
	// RouterRef is the name of the VPCRouter this policy applies to.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	RouterRef string `json:"routerRef"`

	// Upstream defines upstream DNS server configuration.
	// +optional
	Upstream *DNSUpstreamConfig `json:"upstream,omitempty"`

	// Filtering defines DNS filtering rules.
	// +optional
	Filtering *DNSFilteringConfig `json:"filtering,omitempty"`

	// LocalDNS defines local DNS resolution settings.
	// +optional
	LocalDNS *DNSLocalConfig `json:"localDNS,omitempty"`

	// Image overrides the default AdGuard Home container image.
	// +optional
	Image string `json:"image,omitempty"`
}

// VPCDNSPolicyStatus defines the observed state of a VPCDNSPolicy.
type VPCDNSPolicyStatus struct {
	// Phase is the current lifecycle phase.
	// +kubebuilder:validation:Enum=Pending;Active;Degraded;Error
	Phase string `json:"phase,omitempty"`

	// FilterRulesLoaded is the number of DNS filter rules loaded.
	FilterRulesLoaded int64 `json:"filterRulesLoaded,omitempty"`

	// ConfigMapName is the name of the generated ConfigMap.
	ConfigMapName string `json:"configMapName,omitempty"`

	// SyncStatus indicates sync state.
	// +kubebuilder:validation:Enum=Synced;Pending;Failed
	SyncStatus string `json:"syncStatus,omitempty"`

	// Message provides human-readable detail.
	Message string `json:"message,omitempty"`

	// Conditions represent the latest available observations.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=vdp
// +kubebuilder:printcolumn:name="Router",type=string,JSONPath=`.spec.routerRef`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Rules",type=integer,JSONPath=`.status.filterRulesLoaded`
// +kubebuilder:printcolumn:name="Sync",type=string,JSONPath=`.status.syncStatus`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// VPCDNSPolicy is the Schema for the vpcdnspolicies API.
type VPCDNSPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VPCDNSPolicySpec   `json:"spec,omitempty"`
	Status VPCDNSPolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// VPCDNSPolicyList contains a list of VPCDNSPolicy.
type VPCDNSPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VPCDNSPolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VPCDNSPolicy{}, &VPCDNSPolicyList{})
}
