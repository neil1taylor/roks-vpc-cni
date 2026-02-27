# ROKS API Contract for VNI and VLAN Attachment Management

This document describes the provisional API contract for ROKS-managed Virtual Network Interfaces (VNIs) and VLAN Attachments. The ROKS API is not yet available; the operator uses a stub client that returns `ErrROKSAPINotAvailable` for all operations. This document captures the expected interface so that when the ROKS API is implemented, integration can proceed with minimal code changes.

## Cluster Modes

The operator supports two modes via the `CLUSTER_MODE` environment variable:

| Mode | Value | VNI/VLAN Management | Client Used |
|------|-------|---------------------|-------------|
| ROKS | `"roks"` | Managed by ROKS platform | `roks.ROKSClient` |
| Unmanaged | `"unmanaged"` (default) | Direct VPC API calls | `vpc.Client` |

## ROKSClient Interface

Defined in `pkg/roks/client.go`. Seven methods:

### VNI Operations

#### `ListVNIs(ctx context.Context) ([]ROKSVNI, error)`

List all VNIs managed by ROKS for this cluster.

- **Provisional endpoint**: `GET /v1/clusters/{cluster_id}/vnis`
- **Expected response**: Array of ROKSVNI objects

```json
[
  {
    "id": "roks-vni-abc123",
    "vpc_vni_id": "0717-abcd-1234",
    "name": "roks-cluster1-default-myvm",
    "mac_address": "fa:16:3e:aa:bb:cc",
    "primary_ipv4": "10.240.0.5",
    "subnet_id": "0717-subnet-1234",
    "security_group_ids": ["0717-sg-1"],
    "vm_name": "myvm",
    "vm_namespace": "default",
    "status": "available",
    "created_at": "2026-01-15T10:30:00Z"
  }
]
```

#### `GetVNI(ctx context.Context, vniID string) (*ROKSVNI, error)`

Get a specific VNI by its ROKS-assigned ID.

- **Provisional endpoint**: `GET /v1/clusters/{cluster_id}/vnis/{vni_id}`
- **Expected response**: Single ROKSVNI object (same schema as above)

#### `GetVNIByVM(ctx context.Context, namespace, vmName string) (*ROKSVNI, error)`

Look up the VNI associated with a specific VirtualMachine.

- **Provisional endpoint**: `GET /v1/clusters/{cluster_id}/vnis?vm_namespace={namespace}&vm_name={vmName}`
- **Expected response**: Single ROKSVNI object (first match)
- **Note**: Returns `nil, nil` if no VNI is associated with the VM

### VLAN Attachment Operations

#### `ListVLANAttachments(ctx context.Context) ([]ROKSVLANAttachment, error)`

List all VLAN attachments managed by ROKS for this cluster.

- **Provisional endpoint**: `GET /v1/clusters/{cluster_id}/vlan-attachments`
- **Expected response**: Array of ROKSVLANAttachment objects

```json
[
  {
    "id": "roks-vla-def456",
    "vpc_attachment_id": "0717-att-5678",
    "bm_server_id": "0717-bms-1234",
    "node_name": "worker-bm-0",
    "vlan_id": 100,
    "subnet_id": "0717-subnet-1234",
    "status": "attached",
    "created_at": "2026-01-15T10:25:00Z"
  }
]
```

#### `GetVLANAttachment(ctx context.Context, attachmentID string) (*ROKSVLANAttachment, error)`

Get a specific VLAN attachment by its ROKS-assigned ID.

- **Provisional endpoint**: `GET /v1/clusters/{cluster_id}/vlan-attachments/{attachment_id}`
- **Expected response**: Single ROKSVLANAttachment object

#### `ListVLANAttachmentsByNode(ctx context.Context, nodeName string) ([]ROKSVLANAttachment, error)`

List VLAN attachments for a specific bare metal node.

- **Provisional endpoint**: `GET /v1/clusters/{cluster_id}/vlan-attachments?node_name={nodeName}`
- **Expected response**: Array of ROKSVLANAttachment objects for that node

### Health / Connectivity

#### `IsAvailable(ctx context.Context) bool`

Check whether the ROKS API is reachable and ready.

- **Provisional endpoint**: `GET /v1/clusters/{cluster_id}/health` or `GET /healthz`
- **Expected behavior**: Returns `true` if the API responds with a 2xx status, `false` otherwise
- **Note**: This is a lightweight probe; implementations should cache the result and avoid calling on every reconcile

## Authentication Expectations

Authentication details are TBD pending ROKS API finalization:

| Mechanism | Status | Notes |
|-----------|--------|-------|
| IAM Token | Likely | Operator already has IAM token from VPC API auth; may be reusable |
| Service Account Token | Possible | Kubernetes-native; projected volume or TokenRequest API |
| mTLS | Possible | Certificate-based auth for service-to-service |
| API Key in Secret | Possible | Separate secret or shared with VPC API key |

The `ROKSClientConfig` struct has an `AuthToken` field and an `APIEndpoint` field. These will be populated from environment variables or a Kubernetes Secret when the auth mechanism is finalized.

## Integration Points in the Codebase

Files that reference `roks.ROKSClient`, `roks.ClusterMode`, or ROKS mode constants:

### Core Package

- `pkg/roks/client.go` — Interface definition, types, stub client, `NewClient()` constructor
- `pkg/roks/mock_client.go` — Mock client for unit testing

### Reconcilers

- `pkg/controller/vni/reconciler.go` — VNI reconciler with `reconcileROKS()` method; branches on `r.Mode == roks.ModeROKS`
- `pkg/controller/vni/reconciler_test.go` — Tests for both modes including dual-mode switching
- `pkg/controller/vlanattachment/reconciler.go` — VLAN attachment reconciler with `reconcileROKS()` method; same branching pattern
- `pkg/controller/vlanattachment/reconciler_test.go` — Tests for both modes including dual-mode switching

### Entry Point

- `cmd/manager/main.go` — Reads `CLUSTER_MODE` env var, creates ROKS client if mode is `"roks"`, passes to VNI and VLAN attachment reconcilers

### BFF Service

- `cmd/bff/internal/handler/router.go` — Reports `roksAPIAvailable: false` in cluster-info endpoint
- `cmd/bff/internal/credentials/loader.go` — Branches on `clusterMode == "roks"` for credential loading

## Activation Checklist

When the ROKS API becomes available, the following steps are required:

### 1. Implement the Real Client

- [ ] Replace the `NewClient()` stub in `pkg/roks/client.go` with actual HTTP client initialization
- [ ] Implement all 7 interface methods against the real ROKS API
- [ ] Add proper error types for ROKS API errors (not found, auth failure, rate limit, etc.)
- [ ] Implement authentication (update `ROKSClientConfig` fields as needed)
- [ ] Add rate limiting if needed (similar to `pkg/vpc/ratelimiter.go`)

### 2. Update Reconcilers

- [ ] Implement `reconcileROKS()` in `pkg/controller/vni/reconciler.go` to sync VNI status from ROKS API
- [ ] Implement `reconcileROKS()` in `pkg/controller/vlanattachment/reconciler.go` to sync VLAN attachment status from ROKS API
- [ ] Handle ROKS deletion flow (VNIs/VLANs managed by platform may have different cleanup semantics)
- [ ] Update status conditions to reflect ROKS-specific states

### 3. Update BFF Service

- [ ] Set `roksAPIAvailable: true` in `cmd/bff/internal/handler/router.go`
- [ ] Add ROKS-specific data aggregation endpoints if needed

### 4. Configuration

- [ ] Document the ROKS API endpoint discovery mechanism (env var, ConfigMap, or auto-discovery)
- [ ] Add ROKS API credentials to the operator's Kubernetes Secret
- [ ] Update Helm chart `values.yaml` with ROKS-specific configuration fields

### 5. Testing

- [ ] Update `pkg/roks/client_test.go` with tests for the real client (behind build tag)
- [ ] Run integration tests: `go test ./test/integration/ -tags roks_integration`
- [ ] Verify dual-mode switching works end-to-end on both ROKS and unmanaged clusters
- [ ] Test graceful degradation when ROKS API is temporarily unavailable

### 6. Monitoring

- [ ] Add Prometheus metrics for ROKS API calls (similar to `InstrumentedClient` for VPC)
- [ ] Add alerts for ROKS API unavailability
- [ ] Ensure orphan GC handles ROKS-managed resources correctly
