# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Monorepo for the ROKS VPC Network Operator — a Kubernetes operator that automates IBM Cloud VPC resource lifecycle for OpenShift Virtualization (KubeVirt) VMs on bare metal ROKS clusters with OVN LocalNet networking. **Read `DESIGN.md` for the full architecture, annotation schemas, and reconciler specifications.**

Three components:
- **`roks-vpc-network-operator/`** — Go operator + BFF service (see its own `CLAUDE.md` for detailed implementation guide)
- **`console-plugin/`** — OpenShift Console dynamic plugin (TypeScript/React, PatternFly 5)

## Build and Development Commands

### Go Operator (`roks-vpc-network-operator/`)

```bash
make build          # go build -o bin/manager cmd/manager/main.go
make test           # go test ./... -coverprofile cover.out
make fmt            # go fmt ./...
make vet            # go vet ./...
make lint           # golangci-lint run
make generate       # go generate ./...
make docker-build   # Build operator image (icr.io/roks/vpc-network-operator:latest)
make deploy         # helm upgrade --install from deploy/helm/
make clean          # Remove bin/ and cover.out
```

Run a single test: `cd roks-vpc-network-operator && go test ./pkg/controller/cudn/ -run TestReconcileName -v`

Integration tests (real VPC API): `go test ./... -tags integration`

### BFF Service (`roks-vpc-network-operator/cmd/bff/`)

Separate Go module with `replace` directive to parent. Build via container:
```bash
podman build -t vpc-bff:latest -f cmd/bff/Dockerfile .   # run from roks-vpc-network-operator/
```

### Console Plugin (`console-plugin/`)

```bash
npm install         # Install dependencies
npm run build       # Webpack production build
npm run build:dev   # Webpack development build
npm start           # Dev server on port 9001
npm run ts-check    # TypeScript type checking (tsc --noEmit)
```

## Architecture

### Operator (Go, controller-runtime)

Twelve reconciliation loops + one mutating webhook + orphan GC:

| Component | Path | Watches | Purpose |
|-----------|------|---------|---------|
| CUDN Reconciler | `pkg/controller/cudn/` | `ClusterUserDefinedNetwork` | Creates VPC subnet + VLAN attachments per LocalNet CUDN |
| UDN Reconciler | `pkg/controller/udn/` | `UserDefinedNetwork` | Namespace-scoped, same logic as CUDN |
| Node Reconciler | `pkg/controller/node/` | `Node` (bare metal) | Ensures VLAN attachments exist on new BM nodes for all CUDNs |
| VM Reconciler | `pkg/controller/vm/` | `VirtualMachine` | Drift detection, cleanup of VNI/FIP on VM deletion |
| VPCSubnet Reconciler | `pkg/controller/vpcsubnet/` | `VPCSubnet` CRD | Full VPC subnet lifecycle |
| VNI Reconciler | `pkg/controller/vni/` | `VirtualNetworkInterface` CRD | Dual-mode: VPC API (unmanaged) or ROKS API (roks) |
| VLANAttachment Reconciler | `pkg/controller/vlanattachment/` | `VLANAttachment` CRD | Dual-mode like VNI |
| FloatingIP Reconciler | `pkg/controller/floatingip/` | `FloatingIP` CRD | Full FIP lifecycle via VPC API |
| VPCGateway Reconciler | `pkg/controller/gateway/` | `VPCGateway` CRD | VPC uplink VNI, FIP, PAR, VPC routes |
| VPCRouter Reconciler | `pkg/controller/router/` | `VPCRouter` CRD | Router pod with IP forwarding, NAT, DHCP, Suricata IDS/IPS sidecar |
| VPCL2Bridge Reconciler | `pkg/controller/l2bridge/` | `VPCL2Bridge` CRD | L2 bridge pod lifecycle for NSX-T/OVN tunneling |
| VPCVPNGateway Reconciler | `pkg/controller/vpngateway/` | `VPCVPNGateway` CRD | VPN pod lifecycle (WireGuard/IPsec), tunnel management |
| VM Webhook | `pkg/webhook/` | `VirtualMachine` CREATE | Creates VNI, injects MAC+IP into VM spec |
| Orphan GC | `pkg/gc/` | Periodic (10 min) | Deletes orphaned VPC resources: VNIs, FIPs, PARs, VPC routes (15 min grace) |

**CRDs** (API group `vpc.roks.ibm.com/v1alpha1`): `VPCSubnet` (vsn), `VirtualNetworkInterface` (vni), `VLANAttachment` (vla), `FloatingIP` (fip), `VPCGateway` (vgw), `VPCRouter` (vrt), `VPCL2Bridge` (vlb), `VPCVPNGateway` (vvg)

**Dual cluster mode** (`CLUSTER_MODE` env var): `"roks"` uses ROKS platform API for VNI/VLAN (stub until API exists); `"unmanaged"` (default) uses VPC API directly.

### Key Packages

- **`pkg/vpc/`** — VPC API client. `Client` interface (composition of per-resource sub-interfaces) is the primary mock boundary for tests. Includes channel-based rate limiter (10 concurrent).
- **`pkg/roks/`** — ROKS platform API client (stub, awaiting API availability).
- **`pkg/annotations/`** — All `vpc.roks.ibm.com/*` annotation key constants.
- **`pkg/finalizers/`** — Finalizer CRUD helpers. Finalizer names: `vpc.roks.ibm.com/cudn-cleanup`, `vpc.roks.ibm.com/vm-cleanup`, `vpc.roks.ibm.com/udn-cleanup`, `vpc.roks.ibm.com/gateway-cleanup`, `vpc.roks.ibm.com/router-cleanup`, `vpc.roks.ibm.com/l2bridge-cleanup`, `vpc.roks.ibm.com/vpngateway-cleanup`.
- **`api/v1alpha1/`** — CRD type definitions with DeepCopy.

### BFF Service

Go HTTP server (`cmd/bff/`) that aggregates VPC API + K8s API data for the console plugin. Auth via request headers, authz via Kubernetes SubjectAccessReview. Exposes REST endpoints for subnets, VNIs, VLAN attachments, floating IPs, security groups, network ACLs, and topology.

### Console Plugin (TypeScript/React)

OpenShift dynamic plugin using Module Federation (`@openshift-console/dynamic-plugin-sdk-webpack`). PatternFly 5 components. 26 routes under `/vpc-networking/*` covering Dashboard, Subnets, VNIs, VLAN Attachments, Floating IPs, PARs, Security Groups, Network ACLs, Routes, Topology, Networks, Gateways, Routers, L2 Bridges, and Observability — each resource type has list, detail, and create pages. Plugin metadata in `console-extensions.json`, exposed modules in `package.json`.

## Implementation Conventions

- **All VPC operations must be idempotent** — use resource tags (cluster ID + namespace + name) to detect existing resources before creating.
- **VNIs require**: `AllowIPSpoofing: true`, `EnableInfrastructureNat: false`, `AutoDelete: false`.
- **VLAN Attachments require**: `InterfaceType: "vlan"`, `AllowToFloat: true`, plus VLAN ID from CUDN annotation.
- **Annotation-driven config** — CUDNs and VMs are configured via `vpc.roks.ibm.com/*` annotations (keys centralized in `pkg/annotations/keys.go`).
- **Error handling** — reconcilers requeue with exponential backoff on VPC failures; webhook rejects admission on failure; orphan GC handles leaked resources.
- **Helm chart** (`deploy/helm/roks-vpc-network-operator/`) is the deployment mechanism for all three components.

## Implementation Status

All phases are implemented. The VPC client has 30+ methods, all reconcilers are functional with tests, and the console plugin compiles. Mock the `vpc.Client` interface for unit tests; use `envtest` for reconciler tests.
