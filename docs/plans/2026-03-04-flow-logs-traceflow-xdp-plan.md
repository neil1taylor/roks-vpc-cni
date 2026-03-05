# VPC Flow Logs + Traceflow + XDP Wiring — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Complete three features: wire VPC Flow Logs SDK calls with full-stack UI, implement VPCTraceflow CRD with active probing, and complete XDP/eBPF integration for the fast-path router.

**Architecture:** Feature 1 wires 4 stubbed VPC client methods and adds BFF/console UI for flow log management. Feature 2 introduces a new `VPCTraceflow` CRD whose reconciler execs into router pods to run active network probes. Feature 3 adds `cilium/ebpf` to load the existing `fwd.c` eBPF program into the kernel for XDP fast-path forwarding.

**Tech Stack:** Go (controller-runtime, IBM VPC SDK, cilium/ebpf), TypeScript/React (PatternFly 5, OpenShift Console SDK), eBPF/XDP (clang/LLVM)

---

## Feature 1: VPC Flow Logs SDK Wiring

### Task 1: Wire VPC SDK Flow Log Methods

**Files:**
- Modify: `roks-vpc-network-operator/pkg/vpc/flow_logs.go`

**Step 1: Write tests for all 4 VPC client methods**

Create `roks-vpc-network-operator/pkg/vpc/flow_logs_test.go`:

```go
package vpc

import (
	"context"
	"testing"
)

func TestCreateFlowLogCollector(t *testing.T) {
	// Uses the mock client pattern — the real test is that reconciler_test.go
	// exercises the flow through the mock. This validates the type conversions.
	mock := &MockClient{
		CreateFlowLogCollectorFn: func(ctx context.Context, opts CreateFlowLogCollectorOptions) (*FlowLogCollector, error) {
			if opts.Name == "" {
				t.Error("expected non-empty name")
			}
			if opts.TargetSubnetID == "" {
				t.Error("expected non-empty target subnet ID")
			}
			if opts.COSBucketCRN == "" {
				t.Error("expected non-empty COS bucket CRN")
			}
			return &FlowLogCollector{
				ID:             "fl-123",
				Name:           opts.Name,
				TargetSubnetID: opts.TargetSubnetID,
				COSBucketCRN:   opts.COSBucketCRN,
				IsActive:       opts.IsActive,
				LifecycleState: "stable",
			}, nil
		},
	}

	fl, err := mock.CreateFlowLogCollector(context.Background(), CreateFlowLogCollectorOptions{
		Name:           "test-flowlog",
		TargetSubnetID: "subnet-123",
		COSBucketCRN:   "crn:v1:bluemix:public:cloud-object-storage:global:a/abc:bucket-123::",
		IsActive:       true,
		ClusterID:      "cluster-1",
		OwnerKind:      "vpcsubnet",
		OwnerName:      "my-subnet",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fl.ID != "fl-123" {
		t.Errorf("expected ID fl-123, got %s", fl.ID)
	}
}

func TestGetFlowLogCollector(t *testing.T) {
	mock := &MockClient{
		GetFlowLogCollectorFn: func(ctx context.Context, id string) (*FlowLogCollector, error) {
			return &FlowLogCollector{
				ID:             id,
				Name:           "test-flowlog",
				IsActive:       true,
				LifecycleState: "stable",
			}, nil
		},
	}

	fl, err := mock.GetFlowLogCollector(context.Background(), "fl-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fl.ID != "fl-123" {
		t.Errorf("expected ID fl-123, got %s", fl.ID)
	}
}

func TestDeleteFlowLogCollector(t *testing.T) {
	deleted := false
	mock := &MockClient{
		DeleteFlowLogCollectorFn: func(ctx context.Context, id string) error {
			deleted = true
			return nil
		},
	}

	err := mock.DeleteFlowLogCollector(context.Background(), "fl-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !deleted {
		t.Error("expected delete to be called")
	}
}

func TestListFlowLogCollectors(t *testing.T) {
	mock := &MockClient{
		ListFlowLogCollectorsFn: func(ctx context.Context) ([]FlowLogCollector, error) {
			return []FlowLogCollector{
				{ID: "fl-1", Name: "flowlog-1", IsActive: true},
				{ID: "fl-2", Name: "flowlog-2", IsActive: false},
			}, nil
		},
	}

	fls, err := mock.ListFlowLogCollectors(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fls) != 2 {
		t.Errorf("expected 2 flow logs, got %d", len(fls))
	}
}
```

**Step 2: Run tests to verify they pass (mock-based)**

Run: `cd roks-vpc-network-operator && go test ./pkg/vpc/ -run TestCreateFlowLog -v`
Expected: PASS (mock client doesn't touch the stub)

**Step 3: Implement the 4 VPC SDK methods**

Replace the stub implementations in `roks-vpc-network-operator/pkg/vpc/flow_logs.go`:

```go
package vpc

import (
	"context"
	"fmt"

	"github.com/IBM/vpc-go-sdk/vpcv1"
)

// [Keep existing type definitions unchanged]

// CreateFlowLogCollector creates a VPC flow log collector targeting a subnet.
func (c *vpcClient) CreateFlowLogCollector(ctx context.Context, opts CreateFlowLogCollectorOptions) (*FlowLogCollector, error) {
	if err := c.limiter.Acquire(ctx); err != nil {
		return nil, err
	}
	defer c.limiter.Release()

	target := &vpcv1.FlowLogCollectorTargetPrototypeSubnetIdentitySubnetIdentityByID{
		ID: &opts.TargetSubnetID,
	}

	cosBucket := &vpcv1.LegacyCloudObjectStorageBucketIdentityCloudObjectStorageBucketIdentityByCRN{
		CRN: &opts.COSBucketCRN,
	}

	createOpts := &vpcv1.CreateFlowLogCollectorOptions{
		StorageBucket: cosBucket,
		Target:        target,
		Name:          &opts.Name,
		Active:        &opts.IsActive,
	}

	result, _, err := c.service.CreateFlowLogCollectorWithContext(ctx, createOpts)
	if err != nil {
		return nil, fmt.Errorf("VPC API CreateFlowLogCollector: %w", err)
	}

	// Tag for traceability and orphan GC
	if opts.ClusterID != "" || opts.OwnerKind != "" {
		c.tagResource(ctx, derefString(result.CRN), BuildTags(opts.ClusterID, "flowlog", opts.OwnerKind, opts.OwnerName))
	}

	return flowLogFromSDK(result), nil
}

// DeleteFlowLogCollector deletes a VPC flow log collector by ID.
func (c *vpcClient) DeleteFlowLogCollector(ctx context.Context, id string) error {
	if err := c.limiter.Acquire(ctx); err != nil {
		return err
	}
	defer c.limiter.Release()

	_, err := c.service.DeleteFlowLogCollectorWithContext(ctx, &vpcv1.DeleteFlowLogCollectorOptions{
		ID: &id,
	})
	if err != nil {
		return fmt.Errorf("VPC API DeleteFlowLogCollector(%s): %w", id, err)
	}

	return nil
}

// ListFlowLogCollectors lists all VPC flow log collectors.
func (c *vpcClient) ListFlowLogCollectors(ctx context.Context) ([]FlowLogCollector, error) {
	if err := c.limiter.Acquire(ctx); err != nil {
		return nil, err
	}
	defer c.limiter.Release()

	var all []FlowLogCollector
	var start *string

	for {
		listOpts := &vpcv1.ListFlowLogCollectorsOptions{}
		if start != nil {
			listOpts.Start = start
		}

		result, _, err := c.service.ListFlowLogCollectorsWithContext(ctx, listOpts)
		if err != nil {
			return nil, fmt.Errorf("VPC API ListFlowLogCollectors: %w", err)
		}

		for i := range result.FlowLogCollectors {
			all = append(all, *flowLogFromSDK(&result.FlowLogCollectors[i]))
		}

		if result.Next == nil || result.Next.Href == nil {
			break
		}
		start = result.Next.Href
	}

	return all, nil
}

// GetFlowLogCollector retrieves a VPC flow log collector by ID.
func (c *vpcClient) GetFlowLogCollector(ctx context.Context, id string) (*FlowLogCollector, error) {
	if err := c.limiter.Acquire(ctx); err != nil {
		return nil, err
	}
	defer c.limiter.Release()

	result, _, err := c.service.GetFlowLogCollectorWithContext(ctx, &vpcv1.GetFlowLogCollectorOptions{
		ID: &id,
	})
	if err != nil {
		return nil, fmt.Errorf("VPC API GetFlowLogCollector(%s): %w", id, err)
	}

	return flowLogFromSDK(result), nil
}

// flowLogFromSDK converts a VPC SDK FlowLogCollector to our internal type.
func flowLogFromSDK(fl *vpcv1.FlowLogCollector) *FlowLogCollector {
	collector := &FlowLogCollector{
		ID:             derefString(fl.ID),
		Name:           derefString(fl.Name),
		IsActive:       fl.Active != nil && *fl.Active,
		LifecycleState: derefString(fl.LifecycleState),
	}
	if fl.Target != nil {
		switch t := fl.Target.(type) {
		case *vpcv1.FlowLogCollectorTarget:
			collector.TargetSubnetID = derefString(t.ID)
		}
	}
	if fl.StorageBucket != nil {
		switch b := fl.StorageBucket.(type) {
		case *vpcv1.LegacyCloudObjectStorageBucketReference:
			collector.COSBucketCRN = derefString(b.CRN)
		}
	}
	return collector
}
```

**Step 4: Run all VPC package tests**

Run: `cd roks-vpc-network-operator && go test ./pkg/vpc/ -v`
Expected: PASS

**Step 5: Run reconciler tests to verify flow log integration**

Run: `cd roks-vpc-network-operator && go test ./pkg/controller/vpcsubnet/ -v`
Expected: PASS (reconciler tests use mock client)

**Step 6: Commit**

```bash
git add pkg/vpc/flow_logs.go pkg/vpc/flow_logs_test.go
git commit -m "feat(flowlogs): wire VPC SDK flow log collector methods

Implement CreateFlowLogCollector, GetFlowLogCollector,
ListFlowLogCollectors, and DeleteFlowLogCollector using the
IBM VPC Go SDK. Adds flowLogFromSDK helper and resource tagging."
```

### Task 2: Add Flow Log BFF Endpoints

**Files:**
- Create: `roks-vpc-network-operator/cmd/bff/internal/handler/flowlog_handler.go`
- Modify: `roks-vpc-network-operator/cmd/bff/internal/handler/router.go` (route registration)
- Modify: `roks-vpc-network-operator/cmd/bff/internal/model/subnet.go` (extend response)

**Step 1: Add flow log fields to SubnetResponse model**

In `cmd/bff/internal/model/subnet.go`, add to `SubnetResponse`:

```go
// Flow log fields
FlowLogCollectorID string `json:"flowLogCollectorID,omitempty"`
FlowLogActive      bool   `json:"flowLogActive,omitempty"`
```

**Step 2: Create the flow log handler**

Create `cmd/bff/internal/handler/flowlog_handler.go`:

```go
package handler

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/IBM/roks-vpc-network-operator/cmd/bff/internal/auth"
	"github.com/IBM/roks-vpc-network-operator/pkg/vpc"
)

// FlowLogHandler handles VPC flow log collector operations.
type FlowLogHandler struct {
	vpcClient vpc.ExtendedClient
	rbac      *auth.RBACChecker
}

// NewFlowLogHandler creates a new flow log handler.
func NewFlowLogHandler(vpcClient vpc.ExtendedClient, rbac *auth.RBACChecker) *FlowLogHandler {
	return &FlowLogHandler{
		vpcClient: vpcClient,
		rbac:      rbac,
	}
}

// FlowLogResponse is the API response for a flow log collector.
type FlowLogResponse struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	TargetSubnetID string `json:"targetSubnetID"`
	COSBucketCRN   string `json:"cosBucketCRN"`
	IsActive       bool   `json:"isActive"`
	LifecycleState string `json:"lifecycleState"`
}

// ListFlowLogCollectors handles GET /api/v1/flow-logs
func (h *FlowLogHandler) ListFlowLogCollectors(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		return
	}

	collectors, err := h.vpcClient.ListFlowLogCollectors(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to list flow log collectors", "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to list flow log collectors", "LIST_FLOW_LOGS_FAILED")
		return
	}

	responses := make([]FlowLogResponse, 0, len(collectors))
	for _, c := range collectors {
		responses = append(responses, flowLogToResponse(c))
	}

	WriteJSON(w, http.StatusOK, responses)
}

// GetFlowLogCollector handles GET /api/v1/flow-logs/{id}
func (h *FlowLogHandler) GetFlowLogCollector(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/v1/flow-logs/")
	id = strings.Split(id, "/")[0]

	collector, err := h.vpcClient.GetFlowLogCollector(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to get flow log collector", "id", id, "error", err)
		WriteError(w, http.StatusNotFound, "flow log collector not found", "FLOW_LOG_NOT_FOUND")
		return
	}

	WriteJSON(w, http.StatusOK, flowLogToResponse(*collector))
}

// DeleteFlowLogCollector handles DELETE /api/v1/flow-logs/{id}
func (h *FlowLogHandler) DeleteFlowLogCollector(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		return
	}

	userInfo := auth.GetUserFromContext(r.Context())
	if userInfo == nil || userInfo.Name == "" {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	allowed, err := h.rbac.CheckAccess(r.Context(), userInfo.Name, userInfo.Groups, "delete", "vpcsubnets", "")
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "authorization check failed", "AUTHZ_CHECK_FAILED")
		return
	}
	if !allowed {
		WriteError(w, http.StatusForbidden, "forbidden", "FORBIDDEN")
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/v1/flow-logs/")
	id = strings.Split(id, "/")[0]

	if err := h.vpcClient.DeleteFlowLogCollector(r.Context(), id); err != nil {
		slog.ErrorContext(r.Context(), "failed to delete flow log collector", "id", id, "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to delete flow log collector", "DELETE_FLOW_LOG_FAILED")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func flowLogToResponse(fl vpc.FlowLogCollector) FlowLogResponse {
	return FlowLogResponse{
		ID:             fl.ID,
		Name:           fl.Name,
		TargetSubnetID: fl.TargetSubnetID,
		COSBucketCRN:   fl.COSBucketCRN,
		IsActive:       fl.IsActive,
		LifecycleState: fl.LifecycleState,
	}
}
```

**Step 3: Register flow log routes in router.go**

In `SetupRoutesWithClusterInfo`, add after the subnet routes:

```go
// Flow log collectors
flowLogHandler := NewFlowLogHandler(vpcClient, rbacChecker)
mux.Handle("/api/v1/flow-logs", auth.AuthMiddleware(rbacChecker)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    flowLogHandler.ListFlowLogCollectors(w, r)
})))
mux.Handle("/api/v1/flow-logs/", auth.AuthMiddleware(rbacChecker)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case http.MethodGet:
        flowLogHandler.GetFlowLogCollector(w, r)
    case http.MethodDelete:
        flowLogHandler.DeleteFlowLogCollector(w, r)
    default:
        WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
    }
})))
```

**Step 4: Build BFF to verify compilation**

Run: `cd roks-vpc-network-operator/cmd/bff && go build ./...`
Expected: BUILD SUCCESS

**Step 5: Commit**

```bash
git add cmd/bff/internal/handler/flowlog_handler.go cmd/bff/internal/handler/router.go cmd/bff/internal/model/subnet.go
git commit -m "feat(flowlogs): add BFF endpoints for flow log collectors

GET/DELETE /api/v1/flow-logs, GET /api/v1/flow-logs/{id}.
Extend SubnetResponse with flowLogCollectorID and flowLogActive."
```

### Task 3: Add Flow Log Console Plugin UI

**Files:**
- Modify: `console-plugin/src/pages/SubnetDetailPage.tsx` (add Flow Logs tab)
- Modify: `console-plugin/src/pages/SubnetsListPage.tsx` (add status column)

**Step 1: Add Flow Logs tab to SubnetDetailPage**

Add a new tab in SubnetDetailPage.tsx after the existing tabs. The tab shows:
- Enable/disable toggle (displays current `flowLogActive` status)
- COS Bucket CRN (read-only, from VPCSubnet CR)
- Collector ID (link, from status)
- Lifecycle state badge

Use the existing tab pattern in the file. Add a `FlowLogsTab` component that fetches from `/api/v1/flow-logs` filtered by subnet ID client-side.

**Step 2: Add flow log status column to SubnetsListPage**

Add a column after the Status column showing a green/gray dot indicating flow log status. Use `Label` from PatternFly with `color="green"` for active and `color="grey"` for inactive/none.

**Step 3: Run TypeScript check**

Run: `cd console-plugin && npm run ts-check`
Expected: No errors

**Step 4: Build console plugin**

Run: `cd console-plugin && npm run build`
Expected: BUILD SUCCESS

**Step 5: Commit**

```bash
git add console-plugin/src/pages/SubnetDetailPage.tsx console-plugin/src/pages/SubnetsListPage.tsx
git commit -m "feat(flowlogs): add flow log status to console plugin

Flow Logs tab on SubnetDetailPage with collector status.
Flow log status column on SubnetsListPage."
```

---

## Feature 2: VPCTraceflow

### Task 4: Define VPCTraceflow CRD Types

**Files:**
- Create: `roks-vpc-network-operator/api/v1alpha1/vpctraceflow_types.go`
- Modify: `roks-vpc-network-operator/api/v1alpha1/zz_generated.deepcopy.go` (via `make generate`)

**Step 1: Create CRD type definitions**

Create `roks-vpc-network-operator/api/v1alpha1/vpctraceflow_types.go`:

```go
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TraceflowSource defines the source of a traceflow probe.
type TraceflowSource struct {
	// VMRef references a VirtualMachine as the source.
	// +optional
	VMRef *VMReference `json:"vmRef,omitempty"`

	// IP is a direct source IP address.
	// +optional
	IP string `json:"ip,omitempty"`
}

// VMReference identifies a VirtualMachine.
type VMReference struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
}

// TraceflowDestination defines the destination of a traceflow probe.
type TraceflowDestination struct {
	// IP is the destination IP address.
	IP string `json:"ip"`

	// Port is the destination port (for TCP/UDP).
	// +optional
	Port *int32 `json:"port,omitempty"`

	// Protocol is the IP protocol to probe.
	// +kubebuilder:validation:Enum=TCP;UDP;ICMP
	// +kubebuilder:default=ICMP
	Protocol string `json:"protocol,omitempty"`
}

// VPCTraceflowSpec defines the desired state of VPCTraceflow.
type VPCTraceflowSpec struct {
	// Source is the origin of the probe.
	Source TraceflowSource `json:"source"`

	// Destination is the target of the probe.
	Destination TraceflowDestination `json:"destination"`

	// RouterRef is the VPCRouter to probe from.
	RouterRef string `json:"routerRef"`

	// Timeout is the maximum probe duration (default 30s).
	// +optional
	Timeout string `json:"timeout,omitempty"`

	// TTL is the duration to keep this resource before auto-deletion (default 1h).
	// +optional
	TTL string `json:"ttl,omitempty"`
}

// TraceflowHop represents a single hop in the trace path.
type TraceflowHop struct {
	// Order is the sequence number of this hop.
	Order int `json:"order"`

	// Node identifies the network element (source, router-ingress, router-egress, destination).
	Node string `json:"node"`

	// Component describes the specific component (e.g. "VPCRouter/my-router (net0)").
	Component string `json:"component"`

	// Action describes what happened at this hop (e.g. "SNAT → 169.48.x.x").
	Action string `json:"action"`

	// Latency in milliseconds from the source.
	Latency string `json:"latency,omitempty"`

	// NFTablesHits lists nftables rules that were hit at this hop.
	// +optional
	NFTablesHits []NFTablesRuleHit `json:"nftablesHits,omitempty"`
}

// NFTablesRuleHit represents a matched nftables rule.
type NFTablesRuleHit struct {
	// Rule is the nftables rule text.
	Rule string `json:"rule"`

	// Chain is the chain that contains the rule.
	Chain string `json:"chain"`

	// Packets is the number of packets matched.
	Packets int64 `json:"packets"`
}

// VPCTraceflowStatus defines the observed state of VPCTraceflow.
type VPCTraceflowStatus struct {
	// Phase is the current phase of the traceflow.
	// +kubebuilder:validation:Enum=Pending;Running;Completed;Failed
	Phase string `json:"phase,omitempty"`

	// StartTime is when the probe started.
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// CompletionTime is when the probe finished.
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// Hops is the ordered list of trace hops.
	// +optional
	Hops []TraceflowHop `json:"hops,omitempty"`

	// Result is the overall probe result.
	// +kubebuilder:validation:Enum=Reachable;Unreachable;Filtered;Timeout
	// +optional
	Result string `json:"result,omitempty"`

	// TotalLatency is the end-to-end latency.
	// +optional
	TotalLatency string `json:"totalLatency,omitempty"`

	// Message contains additional details about the result.
	// +optional
	Message string `json:"message,omitempty"`

	// Conditions represent the latest available observations.
	// +optional
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
```

**Step 2: Generate DeepCopy methods**

Run: `cd roks-vpc-network-operator && make generate`
Expected: `zz_generated.deepcopy.go` updated with VPCTraceflow methods

**Step 3: Verify compilation**

Run: `cd roks-vpc-network-operator && go build ./...`
Expected: BUILD SUCCESS

**Step 4: Commit**

```bash
git add api/v1alpha1/vpctraceflow_types.go api/v1alpha1/zz_generated.deepcopy.go
git commit -m "feat(traceflow): add VPCTraceflow CRD type definitions

New CRD with spec (source, destination, routerRef, timeout, ttl)
and status (phase, hops, result, totalLatency). Short name: vtf."
```

### Task 5: Create Traceflow Helm CRD YAML

**Files:**
- Create: `roks-vpc-network-operator/deploy/helm/roks-vpc-network-operator/templates/crds/vpctraceflow-crd.yaml`

**Step 1: Write the CRD YAML**

Follow the pattern from `vpcdnspolicy-crd.yaml`. Include full OpenAPI schema matching the Go types above, print columns for Phase/Result/Latency/Router/Age, status subresource.

**Step 2: Lint the Helm chart**

Run: `cd roks-vpc-network-operator && helm lint deploy/helm/roks-vpc-network-operator/`
Expected: PASS

**Step 3: Commit**

```bash
git add deploy/helm/roks-vpc-network-operator/templates/crds/vpctraceflow-crd.yaml
git commit -m "feat(traceflow): add VPCTraceflow CRD Helm template"
```

### Task 6: Implement Traceflow Prober

**Files:**
- Create: `roks-vpc-network-operator/pkg/controller/traceflow/prober.go`

**Step 1: Write prober tests**

Create `roks-vpc-network-operator/pkg/controller/traceflow/prober_test.go`:

```go
package traceflow

import (
	"testing"
)

func TestParseTracerouteOutput(t *testing.T) {
	output := `traceroute to 8.8.8.8 (8.8.8.8), 30 hops max, 60 byte packets
 1  172.16.100.1 (172.16.100.1)  0.456 ms  0.312 ms  0.298 ms
 2  10.0.0.1 (10.0.0.1)  1.234 ms  1.123 ms  1.098 ms
 3  8.8.8.8 (8.8.8.8)  14.567 ms  14.432 ms  14.321 ms`

	hops := parseTracerouteOutput(output)
	if len(hops) != 3 {
		t.Fatalf("expected 3 hops, got %d", len(hops))
	}
	if hops[0].IP != "172.16.100.1" {
		t.Errorf("hop 1 IP: expected 172.16.100.1, got %s", hops[0].IP)
	}
	if hops[2].IP != "8.8.8.8" {
		t.Errorf("hop 3 IP: expected 8.8.8.8, got %s", hops[2].IP)
	}
}

func TestParseNftCountersDiff(t *testing.T) {
	before := map[string]int64{
		"nat/postrouting/snat-rule": 100,
		"filter/forward/allow-all":  200,
	}
	after := map[string]int64{
		"nat/postrouting/snat-rule": 101,
		"filter/forward/allow-all":  200,
	}

	hits := nftCountersDiff(before, after)
	if len(hits) != 1 {
		t.Fatalf("expected 1 hit, got %d", len(hits))
	}
	if hits[0].Chain != "nat/postrouting" {
		t.Errorf("expected chain nat/postrouting, got %s", hits[0].Chain)
	}
	if hits[0].Packets != 1 {
		t.Errorf("expected 1 packet, got %d", hits[0].Packets)
	}
}

func TestBuildProbeCommand(t *testing.T) {
	tests := []struct {
		name     string
		dest     string
		port     int32
		protocol string
		contains string
	}{
		{"tcp", "8.8.8.8", 443, "TCP", "nping --tcp"},
		{"udp", "8.8.8.8", 53, "UDP", "nping --udp"},
		{"icmp", "8.8.8.8", 0, "ICMP", "ping -c 3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := buildProbeCommand(tt.dest, tt.port, tt.protocol, 30)
			if !containsString(cmd, tt.contains) {
				t.Errorf("expected command to contain %q, got %v", tt.contains, cmd)
			}
		})
	}
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}
```

**Step 2: Run tests to verify they fail**

Run: `cd roks-vpc-network-operator && go test ./pkg/controller/traceflow/ -run TestParse -v`
Expected: FAIL (functions not defined)

**Step 3: Implement the prober**

Create `roks-vpc-network-operator/pkg/controller/traceflow/prober.go`:

```go
package traceflow

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	v1alpha1 "github.com/IBM/roks-vpc-network-operator/api/v1alpha1"
)

// tracerouteHop is a parsed hop from traceroute output.
type tracerouteHop struct {
	HopNum  int
	IP      string
	Latency string // e.g. "14.567 ms"
}

var tracerouteLineRe = regexp.MustCompile(`^\s*(\d+)\s+(\S+)\s+\(([^)]+)\)\s+([\d.]+)\s+ms`)

// parseTracerouteOutput extracts hops from traceroute output.
func parseTracerouteOutput(output string) []tracerouteHop {
	var hops []tracerouteHop
	for _, line := range strings.Split(output, "\n") {
		matches := tracerouteLineRe.FindStringSubmatch(line)
		if len(matches) >= 5 {
			hopNum, _ := strconv.Atoi(matches[1])
			hops = append(hops, tracerouteHop{
				HopNum:  hopNum,
				IP:      matches[3],
				Latency: matches[4] + "ms",
			})
		}
	}
	return hops
}

// nftCountersDiff computes the difference between before and after nftables counter snapshots.
func nftCountersDiff(before, after map[string]int64) []v1alpha1.NFTablesRuleHit {
	var hits []v1alpha1.NFTablesRuleHit
	for key, afterVal := range after {
		beforeVal := before[key]
		if afterVal > beforeVal {
			parts := strings.SplitN(key, "/", 3)
			chain := key
			rule := key
			if len(parts) >= 3 {
				chain = parts[0] + "/" + parts[1]
				rule = parts[2]
			}
			hits = append(hits, v1alpha1.NFTablesRuleHit{
				Rule:    rule,
				Chain:   chain,
				Packets: afterVal - beforeVal,
			})
		}
	}
	return hits
}

// parseNftCounters parses `nft list ruleset` output into a map of rule-key → packet-count.
func parseNftCounters(output string) map[string]int64 {
	counters := make(map[string]int64)
	var currentChain string
	counterRe := regexp.MustCompile(`counter packets (\d+)`)

	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "chain ") {
			currentChain = strings.TrimSuffix(strings.TrimPrefix(trimmed, "chain "), " {")
		}
		if matches := counterRe.FindStringSubmatch(trimmed); len(matches) >= 2 {
			packets, _ := strconv.ParseInt(matches[1], 10, 64)
			// Use chain + truncated rule as key
			ruleKey := currentChain + "/" + truncateRule(trimmed)
			counters[ruleKey] = packets
		}
	}
	return counters
}

func truncateRule(rule string) string {
	if len(rule) > 80 {
		return rule[:80]
	}
	return rule
}

// buildProbeCommand constructs the probe command for the given protocol.
func buildProbeCommand(dest string, port int32, protocol string, timeoutSec int) []string {
	switch strings.ToUpper(protocol) {
	case "TCP":
		return []string{"nping", "--tcp", "-p", fmt.Sprintf("%d", port), "-c", "3", "--delay", "1s", dest}
	case "UDP":
		return []string{"nping", "--udp", "-p", fmt.Sprintf("%d", port), "-c", "3", "--delay", "1s", dest}
	default: // ICMP
		return []string{"ping", "-c", "3", "-W", fmt.Sprintf("%d", timeoutSec), dest}
	}
}

// buildTracerouteCommand constructs the traceroute command.
func buildTracerouteCommand(dest string, timeoutSec int) []string {
	return []string{"traceroute", "-n", "-w", fmt.Sprintf("%d", timeoutSec), dest}
}
```

**Step 4: Run tests to verify they pass**

Run: `cd roks-vpc-network-operator && go test ./pkg/controller/traceflow/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/controller/traceflow/prober.go pkg/controller/traceflow/prober_test.go
git commit -m "feat(traceflow): add probe command builder and output parsers

Parses traceroute output into hops, diffs nftables counters,
builds probe commands for TCP/UDP/ICMP protocols."
```

### Task 7: Implement Traceflow Reconciler

**Files:**
- Create: `roks-vpc-network-operator/pkg/controller/traceflow/reconciler.go`
- Create: `roks-vpc-network-operator/pkg/controller/traceflow/reconciler_test.go`

**Step 1: Write reconciler tests**

Test cases:
1. New traceflow → sets phase to Running, execs probe (mocked), writes hops to status
2. Completed traceflow → no-op
3. Expired traceflow (past TTL) → deletes CR
4. Missing router ref → sets phase to Failed

**Step 2: Implement reconciler**

The reconciler:
1. Fetches VPCTraceflow CR
2. If phase is Completed/Failed, check TTL and delete if expired
3. If phase is empty/Pending, set to Running and start probe
4. Find the router pod via VPCRouter status
5. Exec into router pod:
   a. Snapshot nftables counters (before)
   b. Run traceroute
   c. Run protocol-specific probe (nping/ping)
   d. Snapshot nftables counters (after)
6. Parse outputs, compute diffs, build hops list
7. Set status phase=Completed, result, hops, totalLatency

Key: Use `remotecommand.NewSPDYExecutor` for pod exec (same pattern as debugging tools).

**Step 3: Register with manager in `cmd/manager/main.go`**

Add the traceflow reconciler alongside existing reconcilers.

**Step 4: Run tests**

Run: `cd roks-vpc-network-operator && go test ./pkg/controller/traceflow/ -v`
Expected: PASS

**Step 5: Verify full build**

Run: `cd roks-vpc-network-operator && go build ./...`
Expected: BUILD SUCCESS

**Step 6: Commit**

```bash
git add pkg/controller/traceflow/reconciler.go pkg/controller/traceflow/reconciler_test.go cmd/manager/main.go
git commit -m "feat(traceflow): add reconciler with pod exec probing

Reconciler execs into router pods to run traceroute and protocol
probes, diffs nftables counters, writes hop-by-hop results to
status. TTL-based auto-cleanup of expired traces."
```

### Task 8: Add Traceflow BFF Endpoints

**Files:**
- Create: `roks-vpc-network-operator/cmd/bff/internal/handler/traceflow_handler.go`
- Modify: `roks-vpc-network-operator/cmd/bff/internal/handler/router.go`

**Step 1: Create traceflow BFF handler**

Pattern: Use dynamic client (like `dnspolicy_handler.go`) to list/get/create/delete VPCTraceflow CRs.

Endpoints:
- `GET /api/v1/traceflows` — list all traceflows
- `GET /api/v1/traceflows/{name}` — get with hops
- `POST /api/v1/traceflows` — create (accepts JSON with source, dest, routerRef)
- `DELETE /api/v1/traceflows/{name}` — delete

**Step 2: Register routes in router.go**

**Step 3: Build BFF**

Run: `cd roks-vpc-network-operator/cmd/bff && go build ./...`
Expected: BUILD SUCCESS

**Step 4: Commit**

```bash
git add cmd/bff/internal/handler/traceflow_handler.go cmd/bff/internal/handler/router.go
git commit -m "feat(traceflow): add BFF endpoints for traceflow management

CRUD endpoints via dynamic client for VPCTraceflow CRs."
```

### Task 9: Add Traceflow Console Plugin Pages

**Files:**
- Create: `console-plugin/src/pages/TraceflowsListPage.tsx`
- Create: `console-plugin/src/pages/TraceflowCreatePage.tsx`
- Create: `console-plugin/src/pages/TraceflowDetailPage.tsx`
- Modify: `console-plugin/console-extensions.json`
- Modify: `console-plugin/package.json`

**Step 1: Create TraceflowsListPage**

Table with columns: Name, Phase (badge), Result (badge), Source, Destination, Router, Latency, Age.
Name links to detail page. "Create Traceflow" button.

**Step 2: Create TraceflowCreatePage**

Form with:
- Source: VM selector (dropdown) or manual IP input
- Destination: IP, Port, Protocol (TCP/UDP/ICMP select)
- Router: dropdown of available VPCRouters
- Submit → POST to BFF

**Step 3: Create TraceflowDetailPage**

- Breadcrumb back to list
- Summary card (phase, result, total latency)
- Hop timeline: vertical list of cards showing each hop with node, component, action, latency, nftables hits
- Use `DescriptionList` for hop details, `Label` badges for phase/result

**Step 4: Register routes in console-extensions.json**

Add navigation item and 3 page routes:
```json
{"type": "console.navigation/href", "properties": {"id": "traceflows", "section": "vpc-networking", "name": "Traceflows", "href": "/vpc-networking/traceflows"}},
{"type": "console.page/route", "properties": {"exact": true, "path": "/vpc-networking/traceflows", "component": {"$codeRef": "TraceflowsListPage"}}},
{"type": "console.page/route", "properties": {"exact": true, "path": "/vpc-networking/traceflows/create", "component": {"$codeRef": "TraceflowCreatePage"}}},
{"type": "console.page/route", "properties": {"exact": true, "path": "/vpc-networking/traceflows/:name", "component": {"$codeRef": "TraceflowDetailPage"}}}
```

**Step 5: Add exposed modules to package.json**

```json
"TraceflowsListPage": "./src/pages/TraceflowsListPage",
"TraceflowCreatePage": "./src/pages/TraceflowCreatePage",
"TraceflowDetailPage": "./src/pages/TraceflowDetailPage"
```

**Step 6: TypeScript check and build**

Run: `cd console-plugin && npm run ts-check && npm run build`
Expected: PASS

**Step 7: Commit**

```bash
git add console-plugin/src/pages/Traceflow*.tsx console-plugin/console-extensions.json console-plugin/package.json
git commit -m "feat(traceflow): add console plugin list, create, and detail pages

TraceflowsListPage with phase/result badges, TraceflowCreatePage
with VM/IP source picker, TraceflowDetailPage with hop timeline."
```

### Task 10: Update RBAC for Traceflow

**Files:**
- Modify: `roks-vpc-network-operator/deploy/helm/roks-vpc-network-operator/templates/bff-clusterrole.yaml`

**Step 1: Verify bff-clusterrole.yaml already covers `*` on vpc.roks.ibm.com**

The existing rule `resources: ["*"]` on `apiGroups: ["vpc.roks.ibm.com"]` should already cover `vpctraceflows`. Also verify `pods/exec` create permission exists (needed for reconciler). Both should already be present — verify and commit if no changes needed.

**Step 2: Commit (if changes needed)**

```bash
git add deploy/helm/roks-vpc-network-operator/templates/bff-clusterrole.yaml
git commit -m "feat(traceflow): update RBAC for VPCTraceflow resources"
```

---

## Feature 3: Router XDP/eBPF Wiring

### Task 11: Add cilium/ebpf Dependency and Compile eBPF

**Files:**
- Modify: `roks-vpc-network-operator/go.mod` (add cilium/ebpf)
- Modify: `roks-vpc-network-operator/Makefile` (add bpf-compile target)
- Modify: `roks-vpc-network-operator/cmd/vpc-router/Dockerfile` (add clang)

**Step 1: Add cilium/ebpf dependency**

Run: `cd roks-vpc-network-operator && go get github.com/cilium/ebpf@latest`

**Step 2: Add Makefile target for eBPF compilation**

Add to Makefile:

```makefile
.PHONY: bpf-compile
bpf-compile: ## Compile eBPF program for XDP fast-path router
	clang -target bpf -D__TARGET_ARCH_x86 -O2 \
		-g -c cmd/vpc-router/bpf/fwd.c \
		-o cmd/vpc-router/bpf/fwd_bpfel.o
```

**Step 3: Update Dockerfile to compile eBPF in build stage**

Add clang to the builder stage and compile the eBPF program:

```dockerfile
# In builder stage:
RUN apk add --no-cache clang llvm

# After go build:
RUN clang -target bpf -D__TARGET_ARCH_x86 -O2 \
    -g -c cmd/vpc-router/bpf/fwd.c \
    -o cmd/vpc-router/bpf/fwd_bpfel.o

# In runtime stage:
COPY --from=builder /build/cmd/vpc-router/bpf/fwd_bpfel.o /bpf/fwd_bpfel.o
```

**Step 4: Verify build**

Run: `cd roks-vpc-network-operator && go build ./cmd/vpc-router/...`
Expected: BUILD SUCCESS

**Step 5: Commit**

```bash
git add go.mod go.sum Makefile cmd/vpc-router/Dockerfile
git commit -m "feat(xdp): add cilium/ebpf dependency and eBPF build tooling

Add cilium/ebpf Go module, Makefile bpf-compile target,
and Dockerfile clang stage for eBPF program compilation."
```

### Task 12: Implement XDP Loading in xdp.go

**Files:**
- Modify: `roks-vpc-network-operator/cmd/vpc-router/xdp.go`
- Create: `roks-vpc-network-operator/cmd/vpc-router/xdp_test.go`

**Step 1: Write tests for map population**

Create `cmd/vpc-router/xdp_test.go`:

```go
package main

import (
	"testing"
)

func TestBuildRouteEntries(t *testing.T) {
	cfg := &Config{
		Networks: NetworkConfig{
			Interfaces: []InterfaceConfig{
				{Name: "net0", Address: "172.16.100.10/24"},
				{Name: "net1", Address: "10.0.0.5/16"},
			},
		},
	}

	entries := buildRouteEntries(cfg)
	if len(entries) != 2 {
		t.Fatalf("expected 2 route entries, got %d", len(entries))
	}

	// net0: 172.16.100.0/24
	if entries[0].PrefixLen != 24 {
		t.Errorf("entry 0 prefix: expected 24, got %d", entries[0].PrefixLen)
	}

	// net1: 10.0.0.0/16
	if entries[1].PrefixLen != 16 {
		t.Errorf("entry 1 prefix: expected 16, got %d", entries[1].PrefixLen)
	}
}

func TestExtractNATCIDRs(t *testing.T) {
	nftConfig := `table inet nat {
  chain postrouting {
    type nat hook postrouting priority srcnat; policy accept;
    oifname "uplink" ip saddr 172.16.100.0/24 counter snat to 169.48.1.1
    oifname "uplink" ip saddr 10.0.0.0/16 counter snat to 169.48.1.1
  }
}`

	cidrs := extractNATCIDRs(nftConfig)
	if len(cidrs) != 2 {
		t.Fatalf("expected 2 NAT CIDRs, got %d", len(cidrs))
	}
}

func TestCheckKernelVersion(t *testing.T) {
	// Test with a known-good version string
	ok := isKernelVersionSufficient("5.15.0-100-generic")
	if !ok {
		t.Error("expected 5.15.0 to be sufficient (>= 5.8)")
	}

	ok = isKernelVersionSufficient("4.19.0-generic")
	if ok {
		t.Error("expected 4.19.0 to be insufficient (< 5.8)")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd roks-vpc-network-operator && go test ./cmd/vpc-router/ -run TestBuild -v`
Expected: FAIL (functions not defined)

**Step 3: Implement xdp.go**

Replace the stub in `cmd/vpc-router/xdp.go`:

```go
package main

import (
	"encoding/binary"
	"fmt"
	"log/slog"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
)

// lpmKey matches the struct lpm_key in fwd.c.
type lpmKey struct {
	PrefixLen uint32
	Addr      uint32
}

// routeEntry is a parsed route for BPF map population.
type routeEntry struct {
	Network   uint32
	PrefixLen uint32
	IfIndex   uint32
}

// attachXDP loads and attaches XDP/eBPF programs for L3 fast-path forwarding.
func attachXDP(cfg *Config) (func(), error) {
	// Check kernel version
	uname, err := os.ReadFile("/proc/sys/kernel/osrelease")
	if err != nil {
		slog.Warn("Cannot read kernel version, skipping XDP", "error", err)
		return func() {}, nil
	}
	if !isKernelVersionSufficient(strings.TrimSpace(string(uname))) {
		slog.Warn("Kernel version too old for XDP (need >= 5.8), using kernel forwarding")
		return func() {}, nil
	}

	// Load eBPF program from embedded or file path
	bpfPath := "/bpf/fwd_bpfel.o"
	if _, err := os.Stat(bpfPath); os.IsNotExist(err) {
		// Try relative path (for development)
		bpfPath = "bpf/fwd_bpfel.o"
		if _, err := os.Stat(bpfPath); os.IsNotExist(err) {
			slog.Warn("eBPF object not found, using kernel forwarding", "path", bpfPath)
			return func() {}, nil
		}
	}

	spec, err := ebpf.LoadCollectionSpec(bpfPath)
	if err != nil {
		slog.Warn("Failed to load eBPF collection spec, using kernel forwarding", "error", err)
		return func() {}, nil
	}

	coll, err := ebpf.NewCollection(spec)
	if err != nil {
		slog.Warn("Failed to create eBPF collection, using kernel forwarding", "error", err)
		return func() {}, nil
	}

	// Populate route_table map
	routeMap := coll.Maps["route_table"]
	if routeMap == nil {
		coll.Close()
		slog.Warn("route_table map not found in eBPF object")
		return func() {}, nil
	}

	entries := buildRouteEntries(cfg)
	for _, entry := range entries {
		key := lpmKey{PrefixLen: entry.PrefixLen, Addr: entry.Network}
		if err := routeMap.Put(key, entry.IfIndex); err != nil {
			slog.Warn("Failed to populate route_table entry", "error", err, "prefix", entry.PrefixLen)
		}
	}

	// Populate nat_cidrs map
	natMap := coll.Maps["nat_cidrs"]
	if natMap != nil {
		natCIDRs := extractNATCIDRs(cfg.NftablesConfig)
		for _, cidr := range natCIDRs {
			_, ipNet, err := net.ParseCIDR(cidr)
			if err != nil {
				continue
			}
			prefixLen, _ := ipNet.Mask.Size()
			ip := ipToUint32(ipNet.IP)
			key := lpmKey{PrefixLen: uint32(prefixLen), Addr: ip}
			val := uint8(1)
			if err := natMap.Put(key, val); err != nil {
				slog.Warn("Failed to populate nat_cidrs entry", "error", err, "cidr", cidr)
			}
		}
	}

	// Set firewall flag
	flagsMap := coll.Maps["flags"]
	if flagsMap != nil && cfg.FirewallConfig != "" {
		if err := flagsMap.Put(uint32(0), uint32(1)); err != nil {
			slog.Warn("Failed to set firewall flag", "error", err)
		}
	}

	// Attach XDP to each workload interface
	prog := coll.Programs["xdp_fwd"]
	if prog == nil {
		coll.Close()
		slog.Warn("xdp_fwd program not found in eBPF object")
		return func() {}, nil
	}

	var links []link.Link
	for _, iface := range cfg.Networks.Interfaces {
		netIf, err := net.InterfaceByName(iface.Name)
		if err != nil {
			slog.Warn("Interface not found for XDP attach", "name", iface.Name, "error", err)
			continue
		}
		l, err := link.AttachXDP(link.XDPOptions{
			Program:   prog,
			Interface: netIf.Index,
		})
		if err != nil {
			slog.Warn("Failed to attach XDP to interface", "name", iface.Name, "error", err)
			continue
		}
		links = append(links, l)
		slog.Info("Attached XDP to interface", "name", iface.Name, "ifindex", netIf.Index)
	}

	if len(links) == 0 {
		coll.Close()
		slog.Warn("No XDP programs attached, using kernel forwarding")
		return func() {}, nil
	}

	slog.Info("XDP fast-path forwarding enabled", "interfaces", len(links))

	cleanup := func() {
		for _, l := range links {
			l.Close()
		}
		coll.Close()
		slog.Info("XDP programs detached")
	}

	return cleanup, nil
}

// buildRouteEntries parses NETWORK_CONFIG into route table entries.
func buildRouteEntries(cfg *Config) []routeEntry {
	var entries []routeEntry
	for i, iface := range cfg.Networks.Interfaces {
		_, ipNet, err := net.ParseCIDR(iface.Address)
		if err != nil {
			continue
		}
		prefixLen, _ := ipNet.Mask.Size()
		network := ipToUint32(ipNet.IP)

		// Interface index: net0=3, net1=4, etc. (after lo=1, uplink=2)
		ifIndex := uint32(i + 3)

		entries = append(entries, routeEntry{
			Network:   network,
			PrefixLen: uint32(prefixLen),
			IfIndex:   ifIndex,
		})
	}
	return entries
}

var snatCIDRRe = regexp.MustCompile(`ip saddr (\d+\.\d+\.\d+\.\d+/\d+)`)

// extractNATCIDRs parses SNAT source CIDRs from nftables config.
func extractNATCIDRs(nftConfig string) []string {
	var cidrs []string
	matches := snatCIDRRe.FindAllStringSubmatch(nftConfig, -1)
	for _, m := range matches {
		if len(m) >= 2 {
			cidrs = append(cidrs, m[1])
		}
	}
	return cidrs
}

// ipToUint32 converts a net.IP to a uint32 in network byte order.
func ipToUint32(ip net.IP) uint32 {
	ip = ip.To4()
	if ip == nil {
		return 0
	}
	return binary.BigEndian.Uint32(ip)
}

// isKernelVersionSufficient checks if the kernel version is >= 5.8.
func isKernelVersionSufficient(version string) bool {
	parts := strings.SplitN(version, ".", 3)
	if len(parts) < 2 {
		return false
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return false
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return false
	}
	return major > 5 || (major == 5 && minor >= 8)
}
```

**Step 4: Run tests**

Run: `cd roks-vpc-network-operator && go test ./cmd/vpc-router/ -v`
Expected: PASS

**Step 5: Build the binary**

Run: `cd roks-vpc-network-operator && go build ./cmd/vpc-router/...`
Expected: BUILD SUCCESS

**Step 6: Commit**

```bash
git add cmd/vpc-router/xdp.go cmd/vpc-router/xdp_test.go
git commit -m "feat(xdp): implement XDP/eBPF loading via cilium/ebpf

Load compiled eBPF object, populate route_table and nat_cidrs
BPF maps from config, attach XDP to workload interfaces.
Graceful fallback to kernel forwarding on any failure.
Kernel version check (>= 5.8 required)."
```

---

## Final Tasks

### Task 13: Update FUTURE_FEATURES.md

**Files:**
- Modify: `FUTURE_FEATURES.md`

Mark the following as implemented:
- VPC Flow Logs: change from "CRD scaffolding" to "Implemented (2026-03-04)"
- Observability Phase 3 Traceflow: mark as "Implemented (2026-03-04)"
- Router Tier 2 purpose-built image + XDP: mark as "Implemented (2026-03-04)"

**Commit:**

```bash
git add FUTURE_FEATURES.md
git commit -m "docs: mark flow logs, traceflow, and XDP as implemented"
```

### Task 14: Full Verification

**Step 1: Run all Go tests**

Run: `cd roks-vpc-network-operator && go test ./... -count=1`
Expected: ALL PASS

**Step 2: Build all Go binaries**

Run: `cd roks-vpc-network-operator && go build ./... && cd cmd/bff && go build ./...`
Expected: BUILD SUCCESS

**Step 3: Console plugin**

Run: `cd console-plugin && npm run ts-check && npm run build`
Expected: PASS

**Step 4: Helm lint**

Run: `cd roks-vpc-network-operator && helm lint deploy/helm/roks-vpc-network-operator/`
Expected: PASS
