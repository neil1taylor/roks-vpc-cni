package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TraceflowSource defines the source of a traceflow probe.
type TraceflowSource struct {
	// VMRef is a reference to a source VirtualMachine.
	// +optional
	VMRef *VMReference `json:"vmRef,omitempty"`

	// IP is the source IP address. Used if VMRef is not set.
	// +optional
	IP string `json:"ip,omitempty"`
}

// TraceflowDestination defines the destination of a traceflow probe.
type TraceflowDestination struct {
	// IP is the destination IP address.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	IP string `json:"ip"`

	// Port is the destination port number.
	// +optional
	Port *int32 `json:"port,omitempty"`

	// Protocol is the IP protocol to use for the probe.
	// +kubebuilder:validation:Enum=TCP;UDP;ICMP
	// +kubebuilder:default=ICMP
	Protocol string `json:"protocol,omitempty"`
}

// NFTablesRuleHit represents a matched nftables rule during traceflow.
type NFTablesRuleHit struct {
	// Rule is the nftables rule expression.
	Rule string `json:"rule"`

	// Chain is the nftables chain containing the rule.
	Chain string `json:"chain"`

	// Packets is the number of packets matched by this rule.
	Packets int64 `json:"packets"`
}

// TraceflowHop represents a single hop in the traceflow path.
type TraceflowHop struct {
	// Order is the sequence number of this hop in the path.
	Order int `json:"order"`

	// Node is the Kubernetes node where this hop occurred.
	Node string `json:"node"`

	// Component is the networking component at this hop (e.g. "nftables", "ovn", "bridge").
	Component string `json:"component"`

	// Action is the forwarding decision at this hop (e.g. "forward", "drop", "snat", "dnat").
	Action string `json:"action"`

	// Latency is the measured latency at this hop (e.g. "1.2ms").
	Latency string `json:"latency"`

	// NftablesHits is the list of nftables rules matched at this hop.
	// +optional
	NftablesHits []NFTablesRuleHit `json:"nftablesHits,omitempty"`
}

// VPCTraceflowSpec defines the desired state of a VPCTraceflow.
type VPCTraceflowSpec struct {
	// Source defines the traceflow source.
	// +kubebuilder:validation:Required
	Source TraceflowSource `json:"source"`

	// Destination defines the traceflow destination.
	// +kubebuilder:validation:Required
	Destination TraceflowDestination `json:"destination"`

	// RouterRef is the name of the VPCRouter to probe from.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	RouterRef string `json:"routerRef"`

	// Timeout is the maximum duration to wait for the traceflow to complete.
	// +kubebuilder:default="30s"
	Timeout string `json:"timeout,omitempty"`

	// TTL is the auto-cleanup duration after which the traceflow resource is deleted.
	// +kubebuilder:default="1h"
	TTL string `json:"ttl,omitempty"`
}

// VPCTraceflowStatus defines the observed state of a VPCTraceflow.
type VPCTraceflowStatus struct {
	// Phase is the current lifecycle phase of the traceflow.
	// +kubebuilder:validation:Enum=Pending;Running;Completed;Failed
	Phase string `json:"phase,omitempty"`

	// StartTime is the time the traceflow began executing.
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// CompletionTime is the time the traceflow finished.
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// Hops is the ordered list of hops in the traceflow path.
	// +optional
	Hops []TraceflowHop `json:"hops,omitempty"`

	// Result is the overall traceflow verdict.
	// +kubebuilder:validation:Enum=Reachable;Unreachable;Filtered;Timeout
	Result string `json:"result,omitempty"`

	// TotalLatency is the end-to-end latency of the traceflow (e.g. "4.5ms").
	TotalLatency string `json:"totalLatency,omitempty"`

	// Message provides human-readable detail about the traceflow result.
	Message string `json:"message,omitempty"`

	// Conditions represent the latest available observations of the traceflow state.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=vtf
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Result",type=string,JSONPath=`.status.result`
// +kubebuilder:printcolumn:name="Latency",type=string,JSONPath=`.status.totalLatency`
// +kubebuilder:printcolumn:name="Router",type=string,JSONPath=`.spec.routerRef`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// VPCTraceflow is the Schema for the vpctraceflows API.
type VPCTraceflow struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VPCTraceflowSpec   `json:"spec,omitempty"`
	Status VPCTraceflowStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// VPCTraceflowList contains a list of VPCTraceflow.
type VPCTraceflowList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VPCTraceflow `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VPCTraceflow{}, &VPCTraceflowList{})
}
