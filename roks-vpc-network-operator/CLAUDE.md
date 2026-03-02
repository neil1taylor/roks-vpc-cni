# CLAUDE.md — Implementation Guide for ROKS VPC Network Operator

## Project Overview

This is a Kubernetes operator that automates IBM Cloud VPC resource lifecycle for OpenShift Virtualization (KubeVirt) VMs running on bare metal workers in ROKS clusters with OVN LocalNet networking.

**Read `../DESIGN.md` first.** It contains the full architecture, annotation schemas, API references, and reconciler specifications.

## What This Operator Does

When an admin creates a `ClusterUserDefinedNetwork` (CUDN) with LocalNet topology and `vpc.roks.ibm.com/*` annotations, the operator:
1. Creates a VPC subnet
2. Creates VLAN attachments on every bare metal node (`floatable: true`)

When an admin creates a `VirtualMachine` referencing that CUDN, a mutating webhook:
1. Creates a floating VNI (`auto_delete: false`, `allow_ip_spoofing: true`, `enable_infrastructure_nat: false`)
2. Reads back the VPC-generated MAC address and reserved IP
3. Injects the MAC into the VM's interface spec and the IP into cloud-init
4. Adds annotations and a finalizer for cleanup

On deletion, finalizers clean up all VPC resources.

## Language and Framework

- **Go** (1.22+)
- **controller-runtime** (sigs.k8s.io/controller-runtime) for reconcilers and webhook server
- **IBM VPC Go SDK** (github.com/IBM/vpc-go-sdk) for VPC API calls
- **KubeVirt client-go** (kubevirt.io/client-go) for VirtualMachine types
- Standard `kubebuilder`-style project layout

## Architecture

Eleven reconciliation loops + one mutating webhook + orphan GC:

### Network Reconcilers
- **CUDN Reconciler** (`pkg/controller/cudn/reconciler.go`) — watches `ClusterUserDefinedNetwork` with LocalNet topology. Creates VPC subnet + VLAN attachments on all BM nodes.
- **UDN Reconciler** (`pkg/controller/udn/reconciler.go`) — watches `UserDefinedNetwork` (namespace-scoped). Same logic as CUDN.
- **Node Reconciler** (`pkg/controller/node/reconciler.go`) — watches bare metal `Node` objects. Ensures VLAN attachments on new nodes.

### VM Reconciler + Webhook
- **VM Reconciler** (`pkg/controller/vm/reconciler.go`) — drift detection, multi-VNI cleanup on VM deletion.
- **Mutating Webhook** (`pkg/webhook/vm_mutating.go`) — intercepts VM CREATE, creates VNI per LocalNet interface, injects MAC+IP.

### CRD Reconcilers
- **VPCSubnet** (`pkg/controller/vpcsubnet/`) — full VPC subnet lifecycle
- **VNI** (`pkg/controller/vni/`) — dual-mode: VPC API (unmanaged) or ROKS API (roks)
- **VLANAttachment** (`pkg/controller/vlanattachment/`) — dual-mode like VNI
- **FloatingIP** (`pkg/controller/floatingip/`) — full FIP lifecycle via VPC API
- **VPCL2Bridge** (`pkg/controller/l2bridge/reconciler.go`) — manages L2 bridge pods (GRETAP+WireGuard, NSX L2VPN, EVPN-VXLAN) for tunneling between NSX-T and OVN-K networks. References VPCGateway for tunnel endpoint FIP.

### Gateway + Router Reconcilers
- **VPCGateway** (`pkg/controller/gateway/reconciler.go`) — creates uplink VNI via VLAN attachment, manages FIP, PAR, VPC routes. Also watches VPCRouter status to auto-collect `advertisedRoutes` and create/delete VPC routes. See `api/v1alpha1/vpcgateway_types.go`.
- **VPCRouter** (`pkg/controller/router/reconciler.go`) — creates privileged router pod with Multus attachments, IP forwarding, nftables NAT/firewall, optional dnsmasq DHCP, and optional Suricata IDS/IPS sidecar (`pkg/controller/router/suricata.go`). Also watches referenced VPCGateway for NAT/firewall/image/MAC changes and auto-recreates the router pod when they change (including IDS mode changes). Exposes `status.podIP` and `status.idsMode`. See `api/v1alpha1/vpcrouter_types.go`.

**Bidirectional watching pattern**: Gateway watches Router status (for route advertisement), Router watches Gateway spec (for config propagation). This creates a reactive loop where gateway config changes flow down to router pods, and router route advertisements flow up to VPC routes.

### Orphan GC (`pkg/gc/orphan_collector.go`)
- Periodic (every 10 min), lists VPC resources by cluster tag, cross-references with K8s objects, deletes orphans older than 15 min.
- Covers all operator-managed VPC resource types: VNIs, floating IPs, public address ranges (PARs), and VPC routes.

## Key Implementation Details

### Annotation Keys (`pkg/annotations/keys.go`)
All annotation keys are constants prefixed with `vpc.roks.ibm.com/`. See DESIGN.md §4 and §5 for the full list.

### VPC Client (`pkg/vpc/`)
Wraps `github.com/IBM/vpc-go-sdk`. Each file handles one resource type:
- `client.go` — constructor, auth (reads API key from K8s Secret), base config, rate limiter
- `subnet.go` — `CreateSubnet`, `DeleteSubnet`, `GetSubnet`
- `vni.go` — `CreateVNI`, `DeleteVNI`, `GetVNI`, `ListVNIsByTag`
- `vlan_attachment.go` — `CreateVLANAttachment`, `DeleteVLANAttachment`, `ListAttachments`, `CreateVMAttachment`
- `floating_ip.go` — `CreateFloatingIP`, `GetFloatingIP`, `UpdateFloatingIP`, `DeleteFloatingIP`
- `routing.go` — `ListRoutingTables`, `CreateRoute`, `DeleteRoute`, `ListRoutes`
- `par.go` — `CreatePublicAddressRange`, `GetPublicAddressRange`, `DeletePublicAddressRange`, `ListPublicAddressRanges`
- `ratelimiter.go` — channel-based rate limiter (10 concurrent max)
- `instrumented.go` — `InstrumentedClient` wrapper for Prometheus metrics

**All VPC operations must be idempotent.** Use resource tags (cluster ID + namespace + name) to detect existing resources before creating duplicates.

### Finalizers (`pkg/finalizers/`)
Six finalizer names:
- `vpc.roks.ibm.com/cudn-cleanup` — on CUDNs
- `vpc.roks.ibm.com/vm-cleanup` — on VMs
- `vpc.roks.ibm.com/udn-cleanup` — on UDNs
- `vpc.roks.ibm.com/gateway-cleanup` — on VPCGateways (cleans up FIP, PAR, VPC routes, VLAN attachment)
- `vpc.roks.ibm.com/router-cleanup` — on VPCRouters (deletes router pod)
- `vpc.roks.ibm.com/l2bridge-cleanup` — on VPCL2Bridges (deletes L2 bridge pod)

### Orphan GC (`pkg/gc/orphan_collector.go`)
Periodic goroutine (every 10 min). Lists VPC resources by cluster tag, cross-references with K8s objects, deletes orphans older than 15 min. Covers VNIs, floating IPs, PARs, and VPC routes.

### VNI Creation Parameters
Critical — every VNI must be created with:
```go
AllowIPSpoofing: true,
EnableInfrastructureNat: false,
AutoDelete: false,
```
These are non-negotiable for the bare metal + OVN LocalNet architecture.

### VLAN Attachment Parameters
Every VLAN attachment must have:
```go
InterfaceType: "vlan",
AllowToFloat: true,
VLAN: <vlan-id from CUDN annotation>,
```

### MAC Injection
The webhook reads `mac_address` from the VNI creation response and sets it on the VM:
```go
vm.Spec.Template.Spec.Domain.Devices.Interfaces[i].MacAddress = vni.MacAddress
```

### Cloud-init IP Injection
If the VM has a `cloudInitNoCloud` volume, inject network-config with the reserved IP:
```yaml
network:
  version: 2
  ethernets:
    enp1s0:
      addresses:
        - 10.240.64.12/24
      gateway4: 10.240.64.1
```

## Build and Test

```bash
make build          # Build binary
make test           # Run unit tests
make docker-build   # Build container image
make docker-push    # Push to registry
make deploy         # Deploy via Helm
make generate       # Generate deepcopy if needed
```

## Testing Strategy

- Unit tests for each reconciler using `envtest` (fake K8s API server)
- Mock the VPC client interface for unit tests
- Integration tests against a real VPC API (optional, needs API key)
- The VPC client should be defined as an interface so it's mockable

## Configuration

The operator reads configuration from:
1. **K8s Secret** `roks-vpc-network-operator-credentials` in operator namespace — contains `IBMCLOUD_API_KEY`
2. **ConfigMap** `roks-vpc-network-operator-config` — contains `VPC_REGION`, `CLUSTER_ID`, `RESOURCE_GROUP_ID`

## Error Handling Patterns

- VPC API failures in reconcilers: requeue with exponential backoff
- VPC API failures in webhook: reject admission request with descriptive error
- Orphaned resources: GC job handles cleanup
- Out-of-band deletion: drift detection emits K8s warning events

## File-by-File Implementation Order

Recommended order for implementation:

1. `pkg/annotations/keys.go` — constants only, no dependencies
2. `pkg/finalizers/finalizers.go` — simple helpers
3. `pkg/vpc/client.go` — VPC client interface + constructor
4. `pkg/vpc/ratelimiter.go` — rate limiter
5. `pkg/vpc/subnet.go`, `vni.go`, `vlan_attachment.go`, `floating_ip.go` — VPC operations
6. `pkg/controller/cudn/reconciler.go` — CUDN reconciler
7. `pkg/controller/node/reconciler.go` — Node reconciler
8. `pkg/controller/vm/reconciler.go` — VM reconciler
9. `pkg/webhook/vm_mutating.go` — mutating webhook
10. `pkg/gc/orphan_collector.go` — GC job
11. `cmd/manager/main.go` — wire everything together
12. Tests for each package
