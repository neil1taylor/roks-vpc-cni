# Operator Internals

This page provides a deep dive into the ten reconcilers, the mutating webhook, and the orphan garbage collector that make up the VPC Network Operator.

---

## Manager Lifecycle

The operator runs as a single Go binary (`cmd/manager/main.go`) using controller-runtime's `Manager`. On startup:

1. Read configuration from environment variables (`IBMCLOUD_API_KEY`, `VPC_REGION`, `CLUSTER_ID`, `RESOURCE_GROUP_ID`, `CLUSTER_MODE`)
2. Create the VPC API client with rate limiter (10 concurrent max)
3. Optionally create the ROKS API client (if `CLUSTER_MODE=roks`)
4. Create the controller manager with leader election
5. Register all ten reconcilers
6. Register the mutating webhook at `/mutate-virtualmachine`
7. Start the orphan GC as a runnable
8. Start the manager (blocks until signal)

---

## CUDN Reconciler

**Package:** `pkg/controller/cudn/`
**Watches:** `ClusterUserDefinedNetwork` (OVN-Kubernetes, `k8s.ovn.org/v1`)
**Filters:** Only processes CUDNs with `spec.topology == LocalNet`

### Creation Flow

```
CUDN applied ──► Validate annotations ──► Add finalizer
                                              │
                                              ▼
                                   Create VPC subnet
                                   (POST /v1/subnets)
                                              │
                                              ▼
                                   For each BM node:
                                     Create VLAN attachment
                                     (POST /v1/bare_metal_servers/{id}/network_attachments)
                                              │
                                              ▼
                                   Write status annotations
                                   (subnet-id, subnet-status, vlan-attachments)
```

The CUDN Reconciler validates that all six required annotations are present (`zone`, `cidr`, `vpc-id`, `vlan-id`, `security-group-ids`, `acl-id`), adds the `vpc.roks.ibm.com/cudn-cleanup` finalizer, creates the VPC subnet, then iterates all bare metal nodes to create VLAN attachments.

### Deletion Flow

1. Check that no VMs reference this CUDN (block deletion with warning event if any exist)
2. Delete all VLAN attachments (parsed from `vlan-attachments` annotation)
3. Delete the VPC subnet
4. Remove the `vpc.roks.ibm.com/cudn-cleanup` finalizer

---

## Node Reconciler

**Package:** `pkg/controller/node/`
**Watches:** `Node` objects
**Filters:** Only processes bare metal workers (instance type contains "metal")

### Node Join

When a new bare metal node becomes Ready:
1. List all LocalNet CUDNs
2. For each CUDN, get the VPC subnet ID and VLAN ID from annotations
3. Create a VLAN attachment on the new node's PCI interface (`floatable: true`)
4. Update each CUDN's `vlan-attachments` annotation to include the new node

### Node Removal

When a node is deleted:
1. List all CUDNs
2. Find this node's entry in each CUDN's `vlan-attachments` annotation
3. Delete the VLAN attachment via VPC API
4. Remove the node entry from the annotation

---

## VM Reconciler

**Package:** `pkg/controller/vm/`
**Watches:** `VirtualMachine` (KubeVirt, `kubevirt.io/v1`)
**Filters:** Only processes VMs with `vpc.roks.ibm.com/vni-id` annotation (operator-managed)

### Drift Detection

Periodically (every 5 minutes), the reconciler verifies:
- The VNI referenced by `vpc.roks.ibm.com/vni-id` still exists in VPC
- If the VNI has been deleted out-of-band, emits a Kubernetes warning event

Drift detection is read-only — it does not auto-correct to avoid conflicts with Terraform or console changes.

### Deletion Flow

When a VM with the `vpc.roks.ibm.com/vm-cleanup` finalizer is deleted:
1. Delete the floating IP (if `vpc.roks.ibm.com/fip-id` is set)
2. Delete the VNI (if `vpc.roks.ibm.com/vni-id` is set) — this auto-deletes the reserved IP
3. Remove the finalizer

---

## VPCSubnet Reconciler

**Package:** `pkg/controller/vpcsubnet/`
**Watches:** `VPCSubnet` CRD (`vpc.roks.ibm.com/v1alpha1`)

### Creation

1. Add finalizer `vpc.roks.ibm.com/subnet-protection`
2. If `status.subnetID` is empty, create VPC subnet via `CreateSubnet()`
3. Update status: `subnetID`, `syncStatus: Synced`, `lastSyncTime`

### Deletion

1. Delete VPC subnet via `DeleteSubnet(status.subnetID)`
2. Remove finalizer

### Error Handling

On VPC API failure, sets `syncStatus: Failed` with error message and requeues after 30 seconds.

---

## VNI Reconciler

**Package:** `pkg/controller/vni/`
**Watches:** `VirtualNetworkInterface` CRD (`vpc.roks.ibm.com/v1alpha1`)
**Dual-mode:** Behavior depends on `CLUSTER_MODE`

### Unmanaged Mode

Full lifecycle via VPC API:
1. Add finalizer `vpc.roks.ibm.com/vni-protection`
2. Create VNI via `CreateVNI()` with `allowIPSpoofing: true`, `enableInfrastructureNat: false`, `autoDelete: false`
3. Update status: `vniID`, `macAddress`, `primaryIPv4`, `reservedIPID`, `syncStatus: Synced`
4. On deletion: delete VNI via `DeleteVNI()`, remove finalizer

### ROKS Mode

Read-only sync from ROKS platform:
1. If ROKS API is not available, set `syncStatus: Pending` with informative message
2. When ROKS API becomes available, sync VNI details from platform
3. Requeue every 5 minutes to check for updates

---

## VLANAttachment Reconciler

**Package:** `pkg/controller/vlanattachment/`
**Watches:** `VLANAttachment` CRD (`vpc.roks.ibm.com/v1alpha1`)
**Dual-mode:** Like VNI Reconciler

### Unmanaged Mode

1. Add finalizer `vpc.roks.ibm.com/vlan-protection`
2. Create VLAN attachment via `CreateVLANAttachment()` with the specified `bmServerID`, `vlanID`, and `subnetID`
3. Update status: `attachmentID`, `attachmentStatus: attached`, `syncStatus: Synced`
4. On deletion: delete attachment via `DeleteVLANAttachment()`, remove finalizer

### ROKS Mode

Read-only sync, same pattern as VNI Reconciler.

---

## FloatingIP Reconciler

**Package:** `pkg/controller/floatingip/`
**Watches:** `FloatingIP` CRD (`vpc.roks.ibm.com/v1alpha1`)

### Creation

1. Add finalizer `vpc.roks.ibm.com/fip-protection`
2. Create floating IP via `CreateFloatingIP()` with zone and target VNI
3. Update status: `fipID`, `address`, `targetVNIID`, `syncStatus: Synced`

### Deletion

1. Delete floating IP via `DeleteFloatingIP(status.fipID)`
2. Remove finalizer

---

## UDN Reconciler

**Package:** `pkg/controller/udn/`
**Watches:** `UserDefinedNetwork` (OVN-Kubernetes, `k8s.ovn.org/v1`)
**Filters:** Only processes UDNs with `spec.topology == LocalNet`

The UDN Reconciler is the namespace-scoped counterpart to the CUDN Reconciler. It uses the same shared helpers in `pkg/controller/network/` (`EnsureVPCSubnet`, `EnsureVLANAttachments`, `DeleteVLANAttachments`, `DeleteVPCSubnet`) and follows an identical creation and deletion flow.

The only difference is scope: UDN resources are namespaced while CUDNs are cluster-scoped. The finalizer is `vpc.roks.ibm.com/udn-cleanup`.

---

## VPCGateway Reconciler

**Package:** `pkg/controller/gateway/`
**Watches:** `VPCGateway` CRD (`vpc.roks.ibm.com/v1alpha1`), also watches `VPCRouter` for route advertisement

The VPCGateway reconciler manages **only VPC API resources** — it does not create any pods. A gateway provides a shared uplink to the VPC fabric that one or more VPCRouters connect through.

### Creation Flow

```
VPCGateway applied ──► Add finalizer (gateway-cleanup)
                              │
                              ▼
                   Pick a bare metal server
                   Create VLAN attachment with inline VNI
                   (AllowIPSpoofing: true, EnableInfrastructureNat: false)
                              │
                              ▼
                   Create floating IP (if spec.floatingIP.enabled)
                   Bind to uplink VNI
                              │
                              ▼
                   Create PAR + ingress routing table (if spec.publicAddressRange.enabled)
                   Create ingress route: PAR CIDR → gateway reserved IP
                              │
                              ▼
                   Collect routes from spec.vpcRoutes + all associated router advertisedRoutes
                   Create VPC routes in default routing table
                              │
                              ▼
                   Update status (VNIID, FloatingIP, ReservedIP, VPCRouteIDs, phase: Ready)
```

### VPC Resources Created

| Resource | VPC API Call | Tracked In |
|----------|-------------|------------|
| VLAN attachment + inline VNI | `CreateVMAttachment()` | `status.VNIID`, `status.AttachmentID` |
| Floating IP | `CreateFloatingIP()` | `status.FloatingIPID`, `status.FloatingIP` |
| PAR | `CreatePublicAddressRange()` | `status.PublicAddressRangeID`, `status.PublicAddressRangeCIDR` |
| Ingress routing table | `CreateRoutingTable()` | `status.IngressRoutingTableID` |
| VPC routes (default + ingress) | `CreateRoute()` | `status.VPCRouteIDs`, `status.IngressRouteIDs` |

### Route Advertisement (Gateway ↔ Router)

The gateway reconciler watches all VPCRouter objects. When a router's `status.advertisedRoutes` changes, the gateway:
1. Collects `advertisedRoutes` from **all** routers that reference this gateway
2. Merges them with the gateway's own `spec.vpcRoutes`
3. Diffs against existing VPC routes and creates/deletes as needed

This means routers do not need to know about VPC routing — they simply advertise their connected network CIDRs, and the gateway handles the VPC plumbing.

### Deletion Flow

1. Delete ingress routes and ingress routing table
2. Delete PAR
3. Delete VPC routes from default routing table
4. Release floating IP
5. Delete VLAN attachment (which deletes the inline VNI)
6. Remove `vpc.roks.ibm.com/gateway-cleanup` finalizer

---

## VPCRouter Reconciler

**Package:** `pkg/controller/router/`
**Watches:** `VPCRouter` CRD (`vpc.roks.ibm.com/v1alpha1`), also watches `VPCGateway` for config changes

The VPCRouter reconciler creates a **privileged Kubernetes pod** that acts as the data-plane router between workload networks and the gateway's VPC uplink. Each router references exactly one VPCGateway; a gateway can serve multiple routers.

### Creation Flow

```
VPCRouter applied ──► Add finalizer (router-cleanup)
                             │
                             ▼
                  Resolve referenced VPCGateway
                  Read uplink MAC, reserved IP, network name
                             │
                             ▼
                  Build pod spec:
                  - Privileged container (Fedora)
                  - Multus annotations: uplink network + each workload network
                  - Init script: IP forwarding, nftables NAT/firewall, dnsmasq DHCP
                  - Liveness probe: sysctl net.ipv4.ip_forward
                  - Readiness probe: uplink interface UP
                             │
                             ▼
                  Create pod in router's namespace
                             │
                             ▼
                  Update status (phase: Ready, advertisedRoutes, transitIP)
```

### What the Router Pod Does

The router pod is a Fedora container that runs a bash init script configuring:

| Function | Tool | Purpose |
|----------|------|---------|
| IP forwarding | `sysctl net.ipv4.ip_forward=1` | Enable packet forwarding between interfaces |
| NAT (SNAT/DNAT) | `nftables` | Translate between workload IPs and the gateway's VPC IP |
| Firewall | `nftables` | Optional packet filtering rules |
| DHCP | `dnsmasq` | Optional DHCP server for workload networks |
| IDS/IPS | `suricata` (sidecar) | Optional intrusion detection/prevention |

The pod connects to the VPC fabric via Multus veth interfaces — one for the gateway uplink and one per connected workload network.

### Router Watches Gateway

The router reconciler watches the referenced VPCGateway. If any of these change, the router pod is deleted and recreated:
- Gateway NAT rules or firewall rules
- Container image
- Uplink MAC address or reserved IP

### Deletion Flow

1. Delete the router pod
2. Remove `vpc.roks.ibm.com/router-cleanup` finalizer

Note: the router does not create any VPC API resources — only a Kubernetes pod. All VPC resources are managed by the gateway.

### Cardinality

```
VPCGateway (1) ◄──── references ──── (N) VPCRouter
    │                                        │
    │ manages VPC resources:                 │ manages Kubernetes resources:
    │ - Uplink VNI + VLAN attachment         │ - Privileged router pod
    │ - Floating IP                          │   (NAT, DHCP, firewall)
    │ - PAR + ingress routing table          │
    │ - VPC routes                           │
    │                                        │
    └── watches routers for routes ──────────┘
        (bidirectional)
```

---

## Mutating Admission Webhook

**Package:** `pkg/webhook/`
**Registered at:** `/mutate-virtualmachine`
**Intercepts:** `VirtualMachine` CREATE operations

### 10-Step Flow

| Step | Action | VPC API Call |
|------|--------|-------------|
| 1 | Intercept VM CREATE admission request | — |
| 2 | Find LocalNet CUDN references in VM spec (Multus networks) | — |
| 3 | Pass-through if no LocalNet interfaces | — |
| 4 | Look up CUDN for VPC subnet ID and security group IDs | — |
| 5 | Create floating VNI (idempotent via tag check) | `POST /v1/virtual_network_interfaces` |
| 6 | Read VPC-generated MAC + reserved IP from response | — |
| 7 | Create floating IP if `vpc.roks.ibm.com/fip: "true"` | `POST /v1/floating_ips` |
| 8 | Inject `macAddress` into VM interface spec; inject IP into cloud-init | — |
| 9 | Set operator annotations + add `vpc.roks.ibm.com/vm-cleanup` finalizer | — |
| 10 | Return mutated admission response | — |

### Idempotency

Before creating a VNI, the webhook calls `ListVNIsByTag()` with the cluster ID, namespace, and VM name. If a matching VNI already exists (e.g., from a previous rejected attempt), it reuses it instead of creating a duplicate.

### Error Handling

- **VPC API failure:** Admission request is rejected with a descriptive error. The user retries `kubectl apply`.
- **Orphaned VNI:** If the webhook creates a VNI but a later validation webhook rejects the VM, the Orphan GC detects and deletes the VNI after the grace period.

### Timeout

VNI creation typically takes 1-3 seconds. Webhook timeout is configured at 15 seconds. The Kubernetes API server admission timeout is 30 seconds.

---

## Orphan Garbage Collector

**Package:** `pkg/gc/`
**Schedule:** Every 10 minutes (configurable via `gc.interval`)
**Grace period:** 15 minutes (configurable via `gc.gracePeriod`)

### Collection Algorithm

1. List all VNIs tagged with the cluster ID
2. For each VNI, extract namespace and VM name from tags
3. Check if the corresponding VirtualMachine exists in Kubernetes
4. If the VM does not exist and the VNI is older than the grace period:
   - Delete the VNI's floating IP (if any)
   - Delete the VNI
   - Log and emit `orphan_gc_deleted_total` metric
5. Similarly check subnets tagged with the cluster ID against CUDNs

The grace period prevents deleting resources that are still being set up (e.g., VNI created by webhook but VM not yet persisted).

---

## VPC API Client

**Package:** `pkg/vpc/`
**Interface:** `Client` (composition of `SubnetService`, `VNIService`, `VLANAttachmentService`, `FloatingIPService`, `RoutingService`, `PublicAddressRangeService`)
**Extended:** `ExtendedClient` adds `SecurityGroupService`, `NetworkACLService`, `VPCService`, `ZoneService`

### Rate Limiting

A channel-based rate limiter allows a maximum of 10 concurrent VPC API calls. This prevents overwhelming the VPC API during bulk operations (e.g., Node Reconciler creating VLAN attachments on all nodes).

### Resource Tagging via Global Tagging API

All operator-created VPC resources are tagged via the IBM Cloud Global Tagging API (`github.com/IBM/platform-services-go-sdk/globaltaggingv1`). The `vpcClient` initializes a `GlobalTaggingV1` client alongside the VPC SDK client, using the same IAM API key.

After each `Create*` call, `tagResource()` attaches standardized user tags:

| Tag | Purpose |
|-----|---------|
| `roks-operator:true` | Marks resource as operator-managed |
| `roks-cluster:<clusterID>` | Identifies the owning cluster (used by orphan GC) |
| `roks-resource-type:<type>` | Resource type: `subnet`, `vni`, `fip`, `par`, `routing-table` |
| `roks-owner:<kind>/<name>` | K8s object owner (e.g., `gateway/my-gw`, `cudn/localnet-1`) |

**Design decisions:**
- **Best-effort** — tagging failures are logged but never block resource creation
- **Separate API** — uses Global Tagging API (not VPC API), so does not consume VPC rate limit tokens
- **CRN guarding** — resources without CRN (routes, address prefixes) are skipped
- **VLAN attachments** — identified by naming convention (`roks-{clusterID}-{networkName}-vlan{vlanID}`) instead of tags, as they are BM sub-resources without standalone CRNs

### Retry Strategy

- Reconcilers: controller-runtime's built-in work queue with exponential backoff
- Webhook: single attempt, rejection on failure (user retries)

---

## Next Steps

- [Data Path](data-path.md) — How traffic flows between VM and VPC
- [BFF Service](bff-service.md) — REST API architecture
- [Dual Cluster Mode](dual-cluster-mode.md) — ROKS vs. unmanaged
- [CRD References](../reference/crds/vpcsubnet.md) — Detailed CRD field documentation
