# OVN Network Types Reference

This page documents the valid network topology, scope, and role combinations for OVN-Kubernetes user-defined networks, as used by the VPC Network Operator.

## Valid Combinations

Only 4 topology+scope+role combinations are supported by the OVN CRD schema:

| ID | Topology | Scope | Role | IP Mode | VPC? | Tier |
|----|----------|-------|------|---------|------|------|
| `localnet-cudn-secondary` | LocalNet | CUDN | Secondary | Static Reserved (VPC) | Yes | Recommended |
| `layer2-cudn-secondary` | Layer2 | CUDN | Secondary | DHCP or Disabled | No | Recommended |
| `layer2-udn-secondary` | Layer2 | UDN | Secondary | DHCP or Disabled | No | Advanced |
| `layer2-cudn-primary` | Layer2 | CUDN | Primary | Persistent IPAM | No | Expert |

### Why only 4?

The OVN CRD schema is more restrictive than it might appear:

- **UDN has no `localnet` field** — the `localnet` topology block only exists on CUDN (`spec.network.localnet`). UDNs only support `layer2` and `layer3`.
- **LocalNet is Secondary-only** — the `Localnet` topology only supports `role: Secondary` in the OVN CRD.
- **UDN Layer2 is Secondary-only** — UDN `layer2` only supports `role: Secondary`. Use CUDN for Primary Layer2 networks.
- **`ipam.mode: Disabled` is Secondary-only** — setting IPAM to Disabled on a Primary network is rejected by the CRD validation.

## CRD Schema Differences

### ClusterUserDefinedNetwork (CUDN)

CUDNs are cluster-scoped. The topology config is nested under `spec.network`:

```yaml
apiVersion: k8s.ovn.org/v1
kind: ClusterUserDefinedNetwork
metadata:
  name: my-network
spec:
  namespaceSelector: {}    # required field
  network:
    topology: Localnet     # or Layer2
    localnet:              # topology-specific block
      role: Secondary
      ...
```

Key fields:
- `spec.namespaceSelector` — required; `{}` means all namespaces
- `spec.network.topology` — `Localnet` or `Layer2` (note: `Localnet` with lowercase 'n')
- `spec.network.localnet` or `spec.network.layer2` — topology-specific configuration

### UserDefinedNetwork (UDN)

UDNs are namespace-scoped. The topology config is directly under `spec`:

```yaml
apiVersion: k8s.ovn.org/v1
kind: UserDefinedNetwork
metadata:
  name: my-network
  namespace: my-namespace
spec:
  topology: Layer2
  layer2:
    role: Secondary
    ...
```

Key fields:
- `spec.topology` — `Layer2` only (no `Localnet` support)
- `spec.layer2` — topology-specific configuration

## IPAM Modes

| Mode | When to Use | Subnets Field |
|------|-------------|---------------|
| `mode: Disabled` | LocalNet (VPC manages IPs) or Secondary without CIDR | Must be **omitted** |
| `lifecycle: Persistent` | Layer2 with CIDR (Primary or Secondary) | Required: `subnets: ["10.0.0.0/24"]` |

### Rules

- `ipam.mode: Disabled` is **only valid for Secondary** role networks
- When `ipam.mode: Disabled` is set, the `subnets` field **must be omitted**
- For LocalNet with VPC IP management, always use `ipam.mode: Disabled` (the CIDR goes in VPC annotations, not in the CRD)
- For Primary Layer2, subnets are **required** — use `ipam.lifecycle: Persistent` so VM addresses survive restarts
- Subnets are a flat string array: `subnets: ["10.0.0.0/24"]` (NOT `subnets.cidrs`)

## YAML Examples

### LocalNet CUDN Secondary (Recommended)

VPC-backed network with static reserved IPs. Best for VMs that need VPC-routable addresses.

```yaml
apiVersion: k8s.ovn.org/v1
kind: ClusterUserDefinedNetwork
metadata:
  name: production-net
  annotations:
    vpc.roks.ibm.com/zone: "eu-de-1"
    vpc.roks.ibm.com/cidr: "10.240.10.0/24"
    vpc.roks.ibm.com/vpc-id: "r010-xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
    vpc.roks.ibm.com/vlan-id: "100"
    vpc.roks.ibm.com/security-group-ids: "r010-aaaaaaaa-..."
spec:
  namespaceSelector: {}
  network:
    topology: Localnet
    localnet:
      role: Secondary
      physicalNetworkName: production-net
      ipam:
        mode: Disabled
```

> The CIDR is in the annotation only. OVN IPAM is disabled because the VPC API reserves IPs when VNIs are created.

### Layer2 CUDN Secondary (Recommended)

Cluster-internal L2 network with OVN DHCP. No VPC resources needed.

```yaml
apiVersion: k8s.ovn.org/v1
kind: ClusterUserDefinedNetwork
metadata:
  name: internal-net
spec:
  namespaceSelector: {}
  network:
    topology: Layer2
    layer2:
      role: Secondary
      subnets:
        - "10.100.0.0/24"
      ipam:
        lifecycle: Persistent
```

Without a CIDR (IPAM disabled — VMs must configure IPs manually or via external DHCP):

```yaml
spec:
  namespaceSelector: {}
  network:
    topology: Layer2
    layer2:
      role: Secondary
      ipam:
        mode: Disabled
```

### Layer2 UDN Secondary (Advanced)

Namespace-scoped L2 network, isolated to a single namespace.

```yaml
apiVersion: k8s.ovn.org/v1
kind: UserDefinedNetwork
metadata:
  name: app-net
  namespace: my-app
spec:
  topology: Layer2
  layer2:
    role: Secondary
    subnets:
      - "10.200.0.0/24"
    ipam:
      lifecycle: Persistent
```

### Layer2 CUDN Primary (Expert)

Replaces the default pod network cluster-wide. Requires a subnet CIDR.

```yaml
apiVersion: k8s.ovn.org/v1
kind: ClusterUserDefinedNetwork
metadata:
  name: primary-l2
spec:
  namespaceSelector:
    matchExpressions:
      - key: k8s.ovn.org/primary-user-defined-network
        operator: Exists
  network:
    topology: Layer2
    layer2:
      role: Primary
      subnets:
        - "10.222.0.0/16"
      ipam:
        lifecycle: Persistent
```

> **Warning:** Primary networks replace the default pod network. All pods in namespaces matching the `namespaceSelector` will use this network. Target namespaces must have the `k8s.ovn.org/primary-user-defined-network` label set at creation time (the label is immutable).

---

**See also:**

- [Network Setup Guide](../admin-guide/network-setup.md) — Creating and managing VPC-backed networks
- [BFF API Reference](api/bff-api.md) — REST API for network management
