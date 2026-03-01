# Architecture Overview

This page describes the system architecture of the VPC Network Operator, including its components, CRDs, and how they interact with the IBM Cloud VPC API and the Kubernetes cluster.

---

## System Diagram

```
┌─────────────────────────────────────────────────────────────────────┐
│                     OpenShift / ROKS Cluster                        │
│                                                                     │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │              VPC Network Operator (manager pod)               │  │
│  │                                                               │  │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐       │  │
│  │  │    CUDN       │  │    Node       │  │    VM         │       │  │
│  │  │  Reconciler   │  │  Reconciler   │  │  Reconciler   │       │  │
│  │  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘       │  │
│  │  ┌──────┴───────┐  ┌──────┴───────┐  ┌──────┴───────┐       │  │
│  │  │  VPCSubnet    │  │VLANAttachment│  │    VNI        │       │  │
│  │  │  Reconciler   │  │  Reconciler  │  │  Reconciler   │       │  │
│  │  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘       │  │
│  │         │  ┌──────────────┐  ┌──────────────┐│               │  │
│  │         │  │  FloatingIP   │  │  VPCGateway   ││               │  │
│  │         │  │  Reconciler   │  │  Reconciler   ││               │  │
│  │         │  └──────┬───────┘  └──────┬───────┘│               │  │
│  │         │  ┌──────┴───────┐  ┌──────┴───────┐│               │  │
│  │         │  │  VPCRouter    │  │   Orphan GC   ││               │  │
│  │         │  │  Reconciler   │  │  (periodic)   ││               │  │
│  │         │  └──────┬───────┘  └──────┬───────┘│               │  │
│  │  ┌──────┴─────────┴─────────────────┴────────┴──────┐       │  │
│  │  │              VPC API Client (pkg/vpc/)             │       │  │
│  │  │         Rate limiter (10 concurrent max)           │       │  │
│  │  └──────────────────────┬────────────────────────────┘       │  │
│  │                         │                                     │  │
│  │  ┌─────────────────┐   │   ┌─────────────────────────┐      │  │
│  │  │ Mutating Webhook │   │   │  ROKS Client (pkg/roks/) │      │  │
│  │  │ (VM CREATE)      │───┘   │  (ROKS mode only)        │      │  │
│  │  └─────────────────┘       └─────────────────────────┘      │  │
│  └──────────────────────────────────────────────────────────────┘  │
│                                                                     │
│  ┌──────────────────────┐   ┌──────────────────────────────────┐  │
│  │  BFF Service          │   │  Console Plugin                   │  │
│  │  (cmd/bff/)           │◄──│  (console-plugin/)                │  │
│  │  REST API + AuthZ     │   │  PatternFly 5 / Module Federation │  │
│  └──────────┬───────────┘   └──────────────────────────────────┘  │
│             │                                                       │
│  ┌──────────┴──────────────────────────────────────────────────┐  │
│  │  VPCRouter Pods (data plane)                                 │  │
│  │  Privileged pods with Multus — NAT, DHCP, firewall           │  │
│  └─────────────────────────────────────────────────────────────┘  │
│                                                                     │
└─────────────┬───────────────────────────────────────────────────────┘
              │
              ▼
     ┌─────────────────┐
     │  IBM Cloud VPC   │
     │  API             │
     │  (iaas.cloud.ibm │
     │   .com/v1)       │
     └─────────────────┘
```

---

## Components

### Operator Manager

The core component, deployed as a single pod with leader election. It runs ten reconciliation loops and a mutating admission webhook within one process, using the controller-runtime framework.

| Component | Type | Watches | Purpose |
|-----------|------|---------|---------|
| CUDN Reconciler | Controller | `ClusterUserDefinedNetwork` | Creates VPC subnet + VLAN attachments per LocalNet CUDN |
| UDN Reconciler | Controller | `UserDefinedNetwork` | Namespace-scoped, same logic as CUDN |
| Node Reconciler | Controller | `Node` (bare metal) | Ensures VLAN attachments exist on new BM nodes for all CUDNs |
| VM Reconciler | Controller | `VirtualMachine` | Drift detection, cleanup of VNI/FIP on VM deletion |
| VPCSubnet Reconciler | Controller | `VPCSubnet` CRD | Full VPC subnet lifecycle via CRD |
| VNI Reconciler | Controller | `VirtualNetworkInterface` CRD | Dual-mode: VPC API (unmanaged) or ROKS API (roks) |
| VLANAttachment Reconciler | Controller | `VLANAttachment` CRD | Dual-mode like VNI |
| FloatingIP Reconciler | Controller | `FloatingIP` CRD | Full FIP lifecycle via VPC API |
| VPCGateway Reconciler | Controller | `VPCGateway` CRD | VPC uplink VNI, floating IP, PAR, VPC routes (no pod) |
| VPCRouter Reconciler | Controller | `VPCRouter` CRD | Creates privileged router pod with NAT, DHCP, firewall |
| VM Webhook | Mutating Webhook | `VirtualMachine` CREATE | Creates VNI, injects MAC+IP into VM spec |
| Orphan GC | Periodic Job | Timer (10 min) | Deletes orphaned VPC resources (15 min grace) |

### BFF Service

A Go HTTP server that aggregates VPC API and Kubernetes API data for the console plugin. It:
- Authenticates users via `X-Remote-User` / `X-Remote-Group` headers (passed by the OpenShift OAuth proxy)
- Authorizes write operations via Kubernetes SubjectAccessReview
- Exposes REST endpoints for security groups, network ACLs, VPCs, zones, topology, and cluster info
- Runs as a separate Deployment with 2 replicas

See [BFF Service Architecture](bff-service.md) for details.

### Console Plugin

An OpenShift Console dynamic plugin built with TypeScript, React, and PatternFly 5. It uses Webpack Module Federation to integrate into the existing OpenShift Console. It adds a single sidebar entry with 13 tabbed pages and 26 routes under `/vpc-networking/`.

See [Console Plugin Architecture](console-plugin.md) for details.

---

## Custom Resource Definitions (CRDs)

The operator defines six CRDs in the `vpc.roks.ibm.com/v1alpha1` API group:

| CRD | Short Name | Scope | Represents |
|-----|-----------|-------|------------|
| `VPCSubnet` | `vsn` | Namespaced | A VPC subnet managed by the operator |
| `VirtualNetworkInterface` | `vni` | Namespaced | A VPC virtual network interface |
| `VLANAttachment` | `vla` | Namespaced | A VLAN attachment on a bare metal server |
| `FloatingIP` | `fip` | Namespaced | A VPC floating IP |
| `VPCGateway` | `vgw` | Namespaced | A shared VPC uplink (VNI, FIP, routes, PAR) — manages VPC API resources only, no pod |
| `VPCRouter` | `vrt` | Namespaced | A router connecting workload networks to a gateway — creates a privileged pod |

Each CRD follows the same pattern:
- **Spec** defines the desired state (VPC resource parameters)
- **Status** reflects the observed state (VPC resource IDs, sync status, last sync time)
- **Finalizer** ensures cleanup before deletion
- **SyncStatus** is one of `Synced`, `Pending`, or `Failed`

### Gateway vs. Router

The **VPCGateway** and **VPCRouter** work together but have fundamentally different implementations:

- **VPCGateway** is purely a VPC API resource manager. It creates an uplink VNI + VLAN attachment on a bare metal server, provisions a floating IP and optional Public Address Range (PAR), and manages VPC routes. It does **not** create any pods. One gateway can serve multiple routers.

- **VPCRouter** creates a privileged Kubernetes pod with Multus network attachments to the gateway's uplink and to one or more workload networks. The pod runs IP forwarding, nftables NAT/firewall rules, and optional dnsmasq DHCP. Each router references exactly one gateway.

The two reconcilers watch each other: the gateway collects advertised routes from all its routers and creates corresponding VPC routes; the router watches the gateway for configuration changes and recreates its pod when needed.

See the [CRD Reference](../reference/crds/vpcsubnet.md) for complete field documentation.

---

## Repository Layout

```
roks_vpc_cni/
├── roks-vpc-network-operator/          # Go operator + BFF
│   ├── api/v1alpha1/                   # CRD type definitions
│   │   ├── vpcsubnet_types.go
│   │   ├── vni_types.go
│   │   ├── vlanattachment_types.go
│   │   ├── floatingip_types.go
│   │   ├── vpcgateway_types.go
│   │   └── vpcrouter_types.go
│   ├── cmd/
│   │   ├── manager/main.go            # Operator entrypoint
│   │   └── bff/                       # BFF service
│   │       ├── main.go
│   │       └── internal/
│   │           ├── handler/           # HTTP handlers
│   │           ├── auth/              # Auth middleware + RBAC
│   │           └── model/             # Request/response types
│   ├── pkg/
│   │   ├── controller/               # Reconcilers
│   │   │   ├── cudn/
│   │   │   ├── udn/
│   │   │   ├── network/              # Shared helpers for CUDN/UDN
│   │   │   ├── node/
│   │   │   ├── vm/
│   │   │   ├── vpcsubnet/
│   │   │   ├── vni/
│   │   │   ├── vlanattachment/
│   │   │   ├── floatingip/
│   │   │   ├── gateway/
│   │   │   └── router/
│   │   ├── webhook/                   # Mutating webhook
│   │   ├── vpc/                       # VPC API client
│   │   ├── roks/                      # ROKS platform API client
│   │   ├── annotations/               # Annotation key constants
│   │   ├── finalizers/                # Finalizer helpers
│   │   └── gc/                        # Orphan garbage collector
│   ├── config/
│   │   ├── samples/                   # Example CUDN and VM YAMLs
│   │   ├── rbac/                      # RBAC manifests
│   │   └── webhook/                   # Webhook configuration
│   └── deploy/helm/                   # Helm chart
│       └── roks-vpc-network-operator/
├── console-plugin/                    # OpenShift Console plugin
│   ├── src/
│   ├── console-extensions.json        # Plugin metadata
│   └── package.json
└── docs/                              # Documentation (this site)
```

---

## Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| **Kubernetes Operator (not CNI plugin)** | VPC API calls are slow/async (1-3s). CNI is synchronous and would block pod startup. The operator uses reconciliation with retry and backoff. |
| **Mutating Webhook for MAC injection** | Transparent to the user. No race conditions. The VM is born complete with its VPC identity. No wrapper CRDs or two-step workflows. |
| **Annotations on CUDN** | Keeps the upstream OVN-Kubernetes CUDN CRD untouched. No extra objects to manage. Can graduate to a dedicated CRD later if the annotation schema grows. |
| **Separate CRDs for VPC resources** | CRDs (VPCSubnet, VNI, VLANAttachment, FloatingIP, VPCGateway, VPCRouter) provide standard Kubernetes status tracking, conditions, and kubectl visibility. |
| **Gateway = VPC resources, Router = pod** | The gateway manages only VPC API resources (VNI, FIP, routes, PAR). The router creates an actual pod for data-plane functions (NAT, DHCP, firewall). This separation keeps VPC resource lifecycle clean and makes the data plane independently scalable and replaceable. |
| **1:1 CUDN-to-VPC-Subnet mapping** | One CUDN = one VPC subnet in one zone. Multi-zone setups use multiple CUDNs. Keeps the model simple and predictable. |
| **Admin-managed SGs and ACLs** | Administrators pre-create security groups and ACLs and reference them in CUDN annotations. The operator attaches but does not manage rules. |
| **Dual cluster mode** | ROKS mode defers VNI/VLAN management to the platform; unmanaged mode uses VPC API directly. Feature parity with different backends. |
| **Idempotent VPC operations** | All VPC creates use resource tags to detect existing resources, preventing duplicates on retries or webhook re-invocations. |

---

## Authentication and Authorization

### Operator to VPC API

The operator authenticates using a Service ID API key stored as a Kubernetes Secret. The VPC Go SDK's IAM authenticator exchanges the API key for a bearer token.

### Console Plugin to BFF

The OpenShift OAuth proxy injects `X-Remote-User` and `X-Remote-Group` headers. The BFF middleware extracts these and stores them in the request context.

### BFF Authorization

Write operations (create, delete SGs/ACLs) are authorized via Kubernetes SubjectAccessReview. The BFF creates a SAR checking if the user can perform the requested action on the `vpc.ibm.com/v1alpha1` API group.

---

## Next Steps

- [Data Path](data-path.md) — VM-to-VPC packet flow
- [Operator Internals](operator-internals.md) — Deep dive into reconcilers and webhook
- [BFF Service](bff-service.md) — REST API architecture
- [Console Plugin](console-plugin.md) — UI architecture
- [Dual Cluster Mode](dual-cluster-mode.md) — ROKS vs. unmanaged differences
