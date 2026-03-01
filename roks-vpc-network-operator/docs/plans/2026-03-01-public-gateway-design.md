# Public Gateway Support for VPC Network Operator

## Problem

VMs on LocalNet networks need outbound internet access. Without a Public Gateway (PGW) on the VPC subnet, the only option is per-VM floating IPs — expensive, quota-limited, and unnecessary when VMs only need outbound NAT.

## Decision

Reference a pre-existing PGW by ID via an optional CUDN/UDN annotation. The operator attaches it to the subnet at creation time. This matches the existing pattern for security groups (`security-group-ids`) and network ACLs (`acl-id`).

Alternatives considered:
- **Separate CRD with reconciler** — rejected; PGWs are shared, pre-existing resources. A CRD adds lifecycle complexity for no gain.
- **Auto-create PGW if annotation flag set** — rejected; PGWs are zone-scoped with tight quotas (often 1 per zone per VPC). Operator-managed creation risks duplicates and quota exhaustion.

## Design

### Annotation

```
vpc.roks.ibm.com/public-gateway-id: pgw-abc123
```

Optional. When present on a CUDN/UDN with LocalNet topology, the operator passes the PGW ID to the VPC `CreateSubnet` API call. The VPC API atomically attaches the PGW to the subnet during creation.

Not included in `RequiredLocalNetAnnotations` — networks without this annotation simply have no outbound internet (private-only).

### Operator Changes

`EnsureVPCSubnet` reads the annotation and passes it through `CreateSubnetOptions.PublicGatewayID`. One new field in the options struct, one conditional in `subnet.go`. No delete-side changes — the PGW persists independently of subnet lifecycle.

### BFF Changes

New `GET /api/v1/public-gateways` endpoint returns PGWs in the VPC for the console dropdown. Calls `ListPublicGateways` on the VPC API filtered by VPC ID.

### Console Plugin Changes

Optional "Public Gateway" dropdown in the NetworkCreationWizard, after the ACL field. Populated from the BFF endpoint. If selected, the PGW ID is included in the annotation set written to the CUDN/UDN.

## Files to Change

| File | Change |
|------|--------|
| `pkg/annotations/keys.go` | Add `PublicGatewayID` constant |
| `pkg/vpc/client.go` | Add `PublicGatewayID` to `CreateSubnetOptions`; add `PublicGatewayService` interface |
| `pkg/vpc/subnet.go` | Pass PGW ID to VPC API in `CreateSubnet` |
| `pkg/vpc/public_gateway.go` | Implement `ListPublicGateways` |
| `pkg/controller/network/helpers.go` | Read annotation, pass to `CreateSubnetOptions` |
| `pkg/controller/network/helpers_test.go` | Test PGW passthrough |
| `cmd/bff/internal/handler/public_gateway.go` | BFF handler |
| `cmd/bff/internal/handler/router.go` | Register route |
| `cmd/bff/internal/model/types.go` | PGW response type |
| `console-plugin/src/api/types.ts` | PGW TypeScript type |
| `console-plugin/src/api/hooks.ts` | `usePublicGateways` hook |
| `console-plugin/src/components/NetworkCreationWizard.tsx` | PGW dropdown |
