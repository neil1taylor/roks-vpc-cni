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

Three reconciliation loops + one mutating webhook:

### 1. CUDN Reconciler (`pkg/controller/cudn/reconciler.go`)
- Watches `ClusterUserDefinedNetwork` with `spec.topology == LocalNet`
- On create: validate annotations → create VPC subnet → create VLAN attachments on all BM nodes → write status annotations
- On delete: finalizer checks no VMs reference CUDN → delete VLAN attachments → delete subnet → remove finalizer
- See DESIGN.md §6.1 for full flow

### 2. Node Reconciler (`pkg/controller/node/reconciler.go`)
- Watches `Node` objects (filter to bare metal workers)
- On join: list all LocalNet CUDNs → create VLAN attachment on new node for each
- On remove: delete VLAN attachments → update CUDN status
- See DESIGN.md §6.2

### 3. VM Reconciler (`pkg/controller/vm/reconciler.go`)
- Watches `VirtualMachine` CRs with operator annotations
- Drift detection: periodic verify VPC resources exist
- On delete: finalizer → delete FIP → delete VNI → remove finalizer
- See DESIGN.md §6.3

### 4. Mutating Webhook (`pkg/webhook/vm_mutating.go`)
- Intercepts `VirtualMachine` CREATE
- Creates VNI, reads MAC+IP, mutates VM spec
- See DESIGN.md §7 for full flow

## Key Implementation Details

### Annotation Keys (`pkg/annotations/keys.go`)
All annotation keys are constants prefixed with `vpc.roks.ibm.com/`. See DESIGN.md §4 and §5 for the full list.

### VPC Client (`pkg/vpc/`)
Wraps `github.com/IBM/vpc-go-sdk`. Each file handles one resource type:
- `client.go` — constructor, auth (reads API key from K8s Secret), base config, rate limiter
- `subnet.go` — `CreateSubnet`, `DeleteSubnet`, `GetSubnet`
- `vni.go` — `CreateVNI`, `DeleteVNI`, `GetVNI`, `ListVNIsByTag`
- `vlan_attachment.go` — `CreateVLANAttachment`, `DeleteVLANAttachment`, `ListAttachments`
- `floating_ip.go` — `CreateFIP`, `DeleteFIP`, `AttachFIP`
- `ratelimiter.go` — token bucket rate limiter (10 concurrent max for webhook)

**All VPC operations must be idempotent.** Use resource tags (cluster ID + namespace + name) to detect existing resources before creating duplicates.

### Finalizers (`pkg/finalizers/`)
Two finalizer names:
- `vpc.roks.ibm.com/cudn-cleanup` — on CUDNs
- `vpc.roks.ibm.com/vm-cleanup` — on VMs

### Orphan GC (`pkg/gc/orphan_collector.go`)
Periodic goroutine (every 10 min). Lists VPC resources by cluster tag, cross-references with K8s objects, deletes orphans older than 15 min.

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
