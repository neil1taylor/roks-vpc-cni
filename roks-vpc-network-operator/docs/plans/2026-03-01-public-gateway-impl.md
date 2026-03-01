# Public Gateway Support — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Allow admins to attach a pre-existing Public Gateway to a network's VPC subnet so VMs get outbound internet without per-VM floating IPs.

**Architecture:** A new optional annotation `vpc.roks.ibm.com/public-gateway-id` on CUDNs/UDNs is read during subnet creation and passed to the VPC `CreateSubnet` API, which atomically attaches the PGW. The BFF lists available PGWs, and the console plugin adds a dropdown to the network creation wizard.

**Tech Stack:** Go (VPC SDK, controller-runtime), TypeScript/React (PatternFly 5, OpenShift Console SDK)

---

### Task 1: Add annotation constant and CreateSubnetOptions field

**Files:**
- Modify: `pkg/annotations/keys.go`
- Modify: `pkg/vpc/client.go`

**Step 1: Add PublicGatewayID annotation constant**

In `pkg/annotations/keys.go`, add to the CUDN Annotations (admin-provided) section, after `ACLID`:

```go
// PublicGatewayID is the pre-existing public gateway ID for outbound internet (optional)
PublicGatewayID = Prefix + "public-gateway-id"
```

**Step 2: Add PublicGatewayID field to CreateSubnetOptions**

In `pkg/vpc/client.go`, add to the `CreateSubnetOptions` struct after the `ACLID` field:

```go
type CreateSubnetOptions struct {
	Name             string
	VPCID            string
	Zone             string
	CIDR             string
	ACLID            string
	PublicGatewayID  string // optional: attach pre-existing PGW for outbound internet
	ResourceGroupID  string
	ClusterID        string // for tagging
	CUDNName         string // for tagging
}
```

**Step 3: Verify it compiles**

Run: `cd roks-vpc-network-operator && go build ./...`
Expected: SUCCESS (new field is unused, that's fine)

**Step 4: Commit**

```bash
git add pkg/annotations/keys.go pkg/vpc/client.go
git commit -m "feat: add public-gateway-id annotation and CreateSubnetOptions field"
```

---

### Task 2: Pass PGW ID through to VPC API in CreateSubnet

**Files:**
- Modify: `pkg/vpc/subnet.go`

**Step 1: Write the failing test**

In `pkg/vpc/subnet_test.go` (create if needed — but since VPC client tests are integration-only, skip unit test here).

**Step 2: Add PGW to subnet prototype**

In `pkg/vpc/subnet.go`, in the `CreateSubnet` method, after the `ResourceGroup` conditional (line ~30), add:

```go
if opts.PublicGatewayID != "" {
	prototype.PublicGateway = &vpcv1.PublicGatewayIdentityPublicGatewayIdentityByID{
		ID: &opts.PublicGatewayID,
	}
}
```

**Step 3: Verify it compiles**

Run: `cd roks-vpc-network-operator && go build ./...`
Expected: SUCCESS

**Step 4: Commit**

```bash
git add pkg/vpc/subnet.go
git commit -m "feat: pass public gateway ID to VPC CreateSubnet API"
```

---

### Task 3: Read PGW annotation in EnsureVPCSubnet

**Files:**
- Modify: `pkg/controller/network/helpers.go`

**Step 1: Pass PublicGatewayID through to CreateSubnetOptions**

In `pkg/controller/network/helpers.go`, in the `EnsureVPCSubnet` function, modify the `CreateSubnet` call (line ~104) to include the PGW annotation:

```go
subnet, err := vpcClient.CreateSubnet(ctx, vpc.CreateSubnetOptions{
	Name:            subnetName,
	VPCID:           annots[annotations.VPCID],
	Zone:            annots[annotations.Zone],
	CIDR:            cidr,
	ACLID:           annots[annotations.ACLID],
	PublicGatewayID: annots[annotations.PublicGatewayID],
	ClusterID:       clusterID,
	CUDNName:        obj.GetName(),
})
```

The `annotations.PublicGatewayID` will be empty string if not set, and `CreateSubnet` already checks for empty before setting the field.

**Step 2: Verify it compiles**

Run: `cd roks-vpc-network-operator && go build ./...`
Expected: SUCCESS

**Step 3: Commit**

```bash
git add pkg/controller/network/helpers.go
git commit -m "feat: pass public gateway annotation through EnsureVPCSubnet"
```

---

### Task 4: Add test for PGW passthrough in EnsureVPCSubnet

**Files:**
- Modify: `pkg/controller/network/helpers_test.go`

**Step 1: Write the test**

Add this test to `pkg/controller/network/helpers_test.go`:

```go
func TestEnsureVPCSubnet_PassesThroughPublicGatewayID(t *testing.T) {
	scheme := newTestScheme()
	annots := map[string]string{
		annotations.VPCID:           "vpc-123",
		annotations.Zone:            "us-south-1",
		annotations.CIDR:            "10.240.64.0/24",
		annotations.ACLID:           "acl-1",
		annotations.PublicGatewayID: "pgw-abc123",
	}
	obj := makeTestObj("test-cudn", annots)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(obj).
		Build()

	mockVPC := vpc.NewMockClient()
	var capturedOpts vpc.CreateSubnetOptions
	mockVPC.CreateSubnetFn = func(ctx context.Context, opts vpc.CreateSubnetOptions) (*vpc.Subnet, error) {
		capturedOpts = opts
		return &vpc.Subnet{
			ID:     "subnet-new-1",
			Name:   opts.Name,
			CIDR:   opts.CIDR,
			Status: "available",
		}, nil
	}

	created, err := EnsureVPCSubnet(context.Background(), fakeClient, mockVPC, obj, "cluster-abc", "test")
	if err != nil {
		t.Fatalf("EnsureVPCSubnet() error = %v", err)
	}
	if !created {
		t.Error("expected created=true for new subnet")
	}

	if capturedOpts.PublicGatewayID != "pgw-abc123" {
		t.Errorf("expected PublicGatewayID='pgw-abc123', got %q", capturedOpts.PublicGatewayID)
	}
}

func TestEnsureVPCSubnet_OmitsPublicGatewayWhenNotSet(t *testing.T) {
	scheme := newTestScheme()
	annots := map[string]string{
		annotations.VPCID: "vpc-123",
		annotations.Zone:  "us-south-1",
		annotations.CIDR:  "10.240.64.0/24",
		annotations.ACLID: "acl-1",
	}
	obj := makeTestObj("test-cudn-no-pgw", annots)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(obj).
		Build()

	mockVPC := vpc.NewMockClient()
	var capturedOpts vpc.CreateSubnetOptions
	mockVPC.CreateSubnetFn = func(ctx context.Context, opts vpc.CreateSubnetOptions) (*vpc.Subnet, error) {
		capturedOpts = opts
		return &vpc.Subnet{
			ID:     "subnet-new-2",
			Name:   opts.Name,
			CIDR:   opts.CIDR,
			Status: "available",
		}, nil
	}

	_, err := EnsureVPCSubnet(context.Background(), fakeClient, mockVPC, obj, "cluster-abc", "test")
	if err != nil {
		t.Fatalf("EnsureVPCSubnet() error = %v", err)
	}

	if capturedOpts.PublicGatewayID != "" {
		t.Errorf("expected empty PublicGatewayID, got %q", capturedOpts.PublicGatewayID)
	}
}
```

**Step 2: Run tests to verify they pass**

Run: `cd roks-vpc-network-operator && go test ./pkg/controller/network/ -run TestEnsureVPCSubnet_PassesThroughPublicGateway -v`
Expected: PASS

Run: `cd roks-vpc-network-operator && go test ./pkg/controller/network/ -run TestEnsureVPCSubnet_OmitsPublicGateway -v`
Expected: PASS

**Step 3: Commit**

```bash
git add pkg/controller/network/helpers_test.go
git commit -m "test: add PGW passthrough tests for EnsureVPCSubnet"
```

---

### Task 5: Add PublicGateway type and ListPublicGateways to VPC client

**Files:**
- Modify: `pkg/vpc/client.go` (add interface + type)
- Create: `pkg/vpc/public_gateway.go` (implement ListPublicGateways)
- Modify: `pkg/vpc/mock_client.go` (add mock method)
- Modify: `pkg/vpc/instrumented_client.go` (add instrumented wrapper)

**Step 1: Add PublicGateway type and PublicGatewayService interface**

In `pkg/vpc/client.go`, add the type after the existing `FloatingIP` type:

```go
// PublicGateway represents a VPC public gateway.
type PublicGateway struct {
	ID                string
	Name              string
	Status            string
	Zone              string
	VPCID             string
	VPCName           string
	FloatingIPID      string
	FloatingIPAddress string
	ResourceGroupID   string
	ResourceGroupName string
	CreatedAt         string
}
```

Add the interface (after `FloatingIPLister`):

```go
// PublicGatewayService handles listing public gateways (read-only, BFF use).
type PublicGatewayService interface {
	ListPublicGateways(ctx context.Context, vpcID string) ([]PublicGateway, error)
}
```

Add `PublicGatewayService` to `ExtendedClient`:

```go
type ExtendedClient interface {
	Client
	SecurityGroupService
	NetworkACLService
	VPCService
	ZoneService
	VNILister
	FloatingIPLister
	RoutingTableService
	RouteService
	PublicGatewayService
}
```

**Step 2: Create `pkg/vpc/public_gateway.go`**

```go
package vpc

import (
	"context"
	"fmt"

	"github.com/IBM/vpc-go-sdk/vpcv1"
)

// ListPublicGateways lists all public gateways, optionally filtered by VPC ID.
func (c *vpcClient) ListPublicGateways(ctx context.Context, vpcID string) ([]PublicGateway, error) {
	if err := c.limiter.Acquire(ctx); err != nil {
		return nil, err
	}
	defer c.limiter.Release()

	var allPGWs []PublicGateway
	var start *string

	for {
		listOpts := &vpcv1.ListPublicGatewaysOptions{}
		if start != nil {
			listOpts.Start = start
		}

		result, _, err := c.service.ListPublicGatewaysWithContext(ctx, listOpts)
		if err != nil {
			return nil, fmt.Errorf("VPC API ListPublicGateways: %w", err)
		}

		for i := range result.PublicGateways {
			pgw := &result.PublicGateways[i]
			// Filter by VPC if provided
			if vpcID != "" && pgw.VPC != nil && derefString(pgw.VPC.ID) != vpcID {
				continue
			}
			allPGWs = append(allPGWs, pgwFromSDK(pgw))
		}

		if result.Next == nil || result.Next.Href == nil {
			break
		}
		start = result.Next.Href
	}

	return allPGWs, nil
}

func pgwFromSDK(pgw *vpcv1.PublicGateway) PublicGateway {
	p := PublicGateway{
		ID:     derefString(pgw.ID),
		Name:   derefString(pgw.Name),
		Status: derefString(pgw.Status),
	}
	if pgw.Zone != nil {
		p.Zone = derefString(pgw.Zone.Name)
	}
	if pgw.VPC != nil {
		p.VPCID = derefString(pgw.VPC.ID)
		p.VPCName = derefString(pgw.VPC.Name)
	}
	if pgw.FloatingIP != nil {
		p.FloatingIPID = derefString(pgw.FloatingIP.ID)
		p.FloatingIPAddress = derefString(pgw.FloatingIP.Address)
	}
	if pgw.ResourceGroup != nil {
		p.ResourceGroupID = derefString(pgw.ResourceGroup.ID)
		p.ResourceGroupName = derefString(pgw.ResourceGroup.Name)
	}
	if pgw.CreatedAt != nil {
		p.CreatedAt = pgw.CreatedAt.String()
	}
	return p
}
```

**Step 3: Add mock method**

In `pkg/vpc/mock_client.go`, add the function field:

```go
ListPublicGatewaysFn func(ctx context.Context, vpcID string) ([]PublicGateway, error)
```

Add the implementation:

```go
func (m *MockClient) ListPublicGateways(ctx context.Context, vpcID string) ([]PublicGateway, error) {
	m.trackCall("ListPublicGateways")
	if m.ListPublicGatewaysFn != nil {
		return m.ListPublicGatewaysFn(ctx, vpcID)
	}
	return nil, nil
}
```

**Step 4: Add instrumented wrapper**

In `pkg/vpc/instrumented_client.go`, add:

```go
func (c *InstrumentedClient) ListPublicGateways(ctx context.Context, vpcID string) ([]PublicGateway, error) {
	start := time.Now()
	result, err := c.inner.ListPublicGateways(ctx, vpcID)
	recordCall("ListPublicGateways", start, err)
	return result, err
}
```

**Step 5: Verify it compiles**

Run: `cd roks-vpc-network-operator && go build ./...`
Expected: SUCCESS

**Step 6: Commit**

```bash
git add pkg/vpc/client.go pkg/vpc/public_gateway.go pkg/vpc/mock_client.go pkg/vpc/instrumented_client.go
git commit -m "feat: add PublicGateway type and ListPublicGateways VPC client method"
```

---

### Task 6: Add BFF public gateway handler and route

**Files:**
- Create: `cmd/bff/internal/handler/public_gateway.go`
- Modify: `cmd/bff/internal/handler/router.go`
- Modify: `cmd/bff/internal/model/types.go`

**Step 1: Add BFF response type**

In `cmd/bff/internal/model/types.go`, add after `AddressPrefixResponse`:

```go
// PublicGatewayResponse represents a VPC public gateway.
type PublicGatewayResponse struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Status     string      `json:"status"`
	Zone       RefResponse `json:"zone"`
	FloatingIP *IPResponse `json:"floatingIp,omitempty"`
	CreatedAt  string      `json:"createdAt,omitempty"`
}
```

**Step 2: Create BFF handler**

Create `cmd/bff/internal/handler/public_gateway.go`:

```go
package handler

import (
	"log/slog"
	"net/http"

	"github.com/IBM/roks-vpc-network-operator/cmd/bff/internal/model"
	"github.com/IBM/roks-vpc-network-operator/pkg/vpc"
)

// PublicGatewayHandler handles public gateway operations.
type PublicGatewayHandler struct {
	vpcClient    vpc.ExtendedClient
	defaultVPCID string
}

// NewPublicGatewayHandler creates a new public gateway handler.
func NewPublicGatewayHandler(vpcClient vpc.ExtendedClient, defaultVPCID string) *PublicGatewayHandler {
	return &PublicGatewayHandler{
		vpcClient:    vpcClient,
		defaultVPCID: defaultVPCID,
	}
}

// ListPublicGateways handles GET /api/v1/public-gateways
func (h *PublicGatewayHandler) ListPublicGateways(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		return
	}

	vpcID := r.URL.Query().Get("vpcId")
	if vpcID == "" {
		vpcID = h.defaultVPCID
	}

	slog.DebugContext(r.Context(), "listing public gateways", "vpcId", vpcID)

	pgws, err := h.vpcClient.ListPublicGateways(r.Context(), vpcID)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to list public gateways", "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to list public gateways", "LIST_PUBLIC_GATEWAYS_FAILED")
		return
	}

	responses := make([]model.PublicGatewayResponse, 0, len(pgws))
	for _, p := range pgws {
		resp := model.PublicGatewayResponse{
			ID:        p.ID,
			Name:      p.Name,
			Status:    p.Status,
			Zone:      model.RefResponse{ID: p.Zone, Name: p.Zone},
			CreatedAt: p.CreatedAt,
		}
		if p.FloatingIPAddress != "" {
			resp.FloatingIP = &model.IPResponse{Address: p.FloatingIPAddress}
		}
		responses = append(responses, resp)
	}

	WriteJSON(w, http.StatusOK, responses)
}
```

**Step 3: Register route in router.go**

In `cmd/bff/internal/handler/router.go`, in `SetupRoutesWithClusterInfo`, after the address prefix routes (line ~154), add:

```go
// Public Gateway routes (read-only, for network creation dropdown)
pgwHandler := NewPublicGatewayHandler(vpcClient, clusterInfo.VPCID)
mux.HandleFunc("/api/v1/public-gateways", wrapHandler(authMiddleware(pgwHandler.ListPublicGateways)))
```

**Step 4: Verify it compiles**

Run: `cd roks-vpc-network-operator && go build ./cmd/bff/...`
Expected: SUCCESS

**Step 5: Commit**

```bash
git add cmd/bff/internal/handler/public_gateway.go cmd/bff/internal/handler/router.go cmd/bff/internal/model/types.go
git commit -m "feat: add BFF public gateway list endpoint"
```

---

### Task 7: Add public_gateway_id to CreateNetworkRequest and BFF handler

**Files:**
- Modify: `cmd/bff/internal/model/types.go`
- Modify: `cmd/bff/internal/handler/network.go`

**Step 1: Add field to CreateNetworkRequest**

In `cmd/bff/internal/model/types.go`, add to `CreateNetworkRequest` after `ACLID`:

```go
type CreateNetworkRequest struct {
	Name             string   `json:"name"`
	Namespace        string   `json:"namespace,omitempty"`
	Topology         string   `json:"topology"`
	Role             string   `json:"role,omitempty"`
	VPCID            string   `json:"vpc_id,omitempty"`
	Zone             string   `json:"zone,omitempty"`
	CIDR             string   `json:"cidr,omitempty"`
	VLANID           string   `json:"vlan_id,omitempty"`
	SecurityGroupIDs string   `json:"security_group_ids,omitempty"`
	ACLID            string   `json:"acl_id,omitempty"`
	PublicGatewayID  string   `json:"public_gateway_id,omitempty"`
	TargetNamespaces []string `json:"target_namespaces,omitempty"`
}
```

**Step 2: Set annotation in network handler**

In `cmd/bff/internal/handler/network.go`, find the section where `acl-id` annotation is set (around line 582-584). Add after it:

```go
if req.PublicGatewayID != "" {
	annots["vpc.roks.ibm.com/public-gateway-id"] = req.PublicGatewayID
}
```

**Step 3: Verify it compiles**

Run: `cd roks-vpc-network-operator && go build ./cmd/bff/...`
Expected: SUCCESS

**Step 4: Commit**

```bash
git add cmd/bff/internal/model/types.go cmd/bff/internal/handler/network.go
git commit -m "feat: pass public-gateway-id annotation through network creation"
```

---

### Task 8: Add console plugin API client and hooks for public gateways

**Files:**
- Modify: `console-plugin/src/api/types.ts`
- Modify: `console-plugin/src/api/client.ts`
- Modify: `console-plugin/src/api/hooks.ts`

**Step 1: Add TypeScript type**

In `console-plugin/src/api/types.ts`, add after `FloatingIPResponse` (or similar):

```typescript
export interface PublicGateway {
  id: string;
  name: string;
  status: string;
  zone: Zone;
  floatingIp?: { address: string };
  createdAt?: string;
}
```

**Step 2: Add API client method**

In `console-plugin/src/api/client.ts`, add to the `VPCNetworkClient` class:

```typescript
async listPublicGateways(vpcId?: string): Promise<ApiResponse<PublicGateway[]>> {
  const endpoint = vpcId ? `/public-gateways?vpcId=${vpcId}` : '/public-gateways';
  return this.request<PublicGateway[]>('GET', endpoint);
}
```

**Step 3: Add React hook**

In `console-plugin/src/api/hooks.ts`, add:

```typescript
export function usePublicGateways(vpcId?: string): {
  publicGateways: PublicGateway[] | null;
  loading: boolean;
  error: ApiError | null;
} {
  const { data: publicGateways, loading, error } = useBFFData(
    () => apiClient.listPublicGateways(vpcId),
    [vpcId],
  );
  return { publicGateways, loading, error };
}
```

Import `PublicGateway` in the hooks file if not auto-imported.

**Step 4: Verify it compiles**

Run: `cd console-plugin && npm run ts-check`
Expected: SUCCESS

**Step 5: Commit**

```bash
git add console-plugin/src/api/types.ts console-plugin/src/api/client.ts console-plugin/src/api/hooks.ts
git commit -m "feat: add public gateway API client and React hook"
```

---

### Task 9: Add PGW dropdown to NetworkCreationWizard

**Files:**
- Modify: `console-plugin/src/components/NetworkCreationWizard.tsx`

**Step 1: Import the hook**

Add `usePublicGateways` to the import from `../api/hooks`.

**Step 2: Add state and hook call**

After the `aclId` state declaration, add:

```typescript
const [publicGatewayId, setPublicGatewayId] = useState('');
```

After the `useNetworkACLs()` hook call, add:

```typescript
const { publicGateways } = usePublicGateways(vpcId);
```

**Step 3: Add FormSelect dropdown**

After the Network ACL `FormGroup` in the LocalNet VPC settings section, add:

```typescript
<FormGroup label="Public Gateway" fieldId="pgw-select" helperText="Optional: provides outbound internet for VMs without per-VM floating IPs">
  <FormSelect
    id="pgw-select"
    value={publicGatewayId}
    onChange={(_e, val) => setPublicGatewayId(val)}
  >
    <FormSelectOption value="" label="None (private only)" isPlaceholder />
    {(publicGateways || []).map((pgw) => (
      <FormSelectOption key={pgw.id} value={pgw.id} label={`${pgw.name} (${pgw.zone?.name || pgw.zone})`} />
    ))}
  </FormSelect>
</FormGroup>
```

**Step 4: Include PGW in request payload**

Find where `req.acl_id = aclId` is set (in the submit handler). After it, add:

```typescript
if (publicGatewayId) {
  req.public_gateway_id = publicGatewayId;
}
```

**Step 5: Add to review step**

After the Network ACL review `DescriptionListGroup`, add:

```typescript
{publicGatewayId && (
  <DescriptionListGroup>
    <DescriptionListTerm>Public Gateway</DescriptionListTerm>
    <DescriptionListDescription>
      {publicGateways?.find((p) => p.id === publicGatewayId)?.name || publicGatewayId}
    </DescriptionListDescription>
  </DescriptionListGroup>
)}
```

**Step 6: Verify it compiles**

Run: `cd console-plugin && npm run ts-check`
Expected: SUCCESS

**Step 7: Commit**

```bash
git add console-plugin/src/components/NetworkCreationWizard.tsx
git commit -m "feat: add public gateway dropdown to network creation wizard"
```

---

### Task 10: Run all tests and verify

**Files:** None (verification only)

**Step 1: Run Go tests**

Run: `cd roks-vpc-network-operator && go test ./... -count=1`
Expected: ALL PASS

**Step 2: Run Go vet**

Run: `cd roks-vpc-network-operator && go vet ./...`
Expected: SUCCESS

**Step 3: Build BFF**

Run: `cd roks-vpc-network-operator && go build ./cmd/bff/...`
Expected: SUCCESS

**Step 4: Build console plugin**

Run: `cd console-plugin && npm run ts-check && npm run build`
Expected: SUCCESS

**Step 5: Commit any fixups if needed**

If any tests fail, fix and commit.
