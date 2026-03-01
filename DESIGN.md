# ROKS VPC Network Operator — Design Document

**Version:** 1.0 — Draft
**Date:** February 2026
**Author:** Neil Taylor
**Status:** For Review

---

## 1. Executive Summary

### 1.1 Problem Statement

IBM Cloud ROKS supports OVNKubernetes and OpenShift 4.20, with upcoming support for OVN LocalNets. This enables KubeVirt virtual machines running on bare metal workers to be placed directly on VPC subnets, giving each VM a first-class VPC network identity (MAC address, reserved IP, security groups, optional floating IP).

Today this requires manual coordination across two interfaces: the Kubernetes/OpenShift API (CUDNs, VirtualMachine CRs) and the IBM Cloud VPC API (subnets, VNIs, VLAN attachments, reserved IPs, floating IPs, security groups, ACLs). This is error-prone, slow, and does not scale.

### 1.2 Proposed Solution

A standalone Kubernetes operator (the **ROKS VPC Network Operator**) that watches `ClusterUserDefinedNetwork` and `VirtualMachine` custom resources and automatically provisions, reconciles, and garbage-collects the required IBM Cloud VPC resources. A mutating admission webhook transparently injects VPC-allocated MAC addresses into VM specs at creation time.

### 1.3 Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| Kubernetes Operator (not CNI plugin) | VPC API calls are slow/async. CNI is synchronous and would block pod startup. Operator uses reconciliation with retry and backoff. |
| Mutating Webhook for MAC injection | Transparent to admin. No race conditions. VM born complete. No wrapper CRDs or two-step workflows. |
| Three reconciliation loops | CUDN Reconciler (subnet + VLAN attachments), Node Reconciler (VLAN on join/remove), VM Reconciler (VNI + reserved IP + FIP lifecycle). |
| Annotations on CUDN (not new CRD) | Keeps upstream CUDN CRD untouched. No extra objects. Can graduate to a dedicated CRD later. |
| Full lifecycle with finalizers | Creates and GCs all VPC resources. Finalizers prevent orphans. Retry-with-backoff handles VPC API failures. |
| 1:1 CUDN-to-VPC-Subnet mapping | One CUDN = one VPC subnet in one zone. Multi-zone uses multiple CUDNs. |
| Admin-managed SGs and ACLs | Admin pre-creates and references via CUDN annotations. Operator attaches but does not manage rules. |
| Auth via Service ID API key (same as CSI) | Consistent with ROKS patterns. Stored as K8s Secret with minimum IAM roles. |

---

## 2. Architecture Overview

### 2.1 Infrastructure Context

The operator runs in a ROKS cluster where workers are bare metal servers. Each has a primary PCI network interface (uplink). VLAN interfaces attach dynamically to PCI interfaces without server restart. OVN-Kubernetes with LocalNet topology bridges the cluster overlay to VPC subnets via VLAN-tagged traffic.

### 2.2 Data Path

VM-to-VPC data path:

1. KubeVirt VM sends traffic with VPC-assigned MAC on its localnet interface.
2. OVN `br-int` routes via localnet port to physical bridge through patch ports.
3. Physical OVS bridge VLAN-tags the frame with the CUDN VLAN ID.
4. Bare metal PCI uplink forwards VLAN-tagged frame to VPC fabric.
5. VPC fabric matches MAC to floating VNI, applies SG rules, routes packet.

Return path: VPC routes to VNI via reserved IP → VLAN attachment delivers to bare metal → OVN delivers to VM.

### 2.3 Operator Components

| Component | Type | Responsibility |
|-----------|------|----------------|
| CUDN Reconciler | Controller | Watches LocalNet CUDNs. Creates VPC subnets + VLAN attachments on all BM nodes. |
| Node Reconciler | Controller | Watches Nodes. Ensures new BM nodes get VLAN attachments for all CUDNs. Cleans up on removal. |
| VM Reconciler | Controller | Watches VMs. Manages VNI lifecycle, reserved IPs, FIPs. Drift detection. |
| VM Admission Webhook | Mutating Webhook | Intercepts VM CREATE. Creates VNI, injects MAC+IP into spec before persistence. |
| VPC API Client | Library | Wraps IBM Cloud VPC API with retry, rate limiting, idempotency. |
| Orphan GC | Periodic Job | Finds VPC resources tagged by operator with no K8s object. Deletes orphans. |

---

## 3. Bare Metal Networking Model

### 3.1 PCI and VLAN Interface Architecture

PCI interfaces are created at provisioning and cannot change while the server is running. VLAN interfaces are software-configurable and dynamic. The operator creates one VLAN attachment per CUDN per bare metal node:

- **`floatable: true`** — Enables VM live migration between bare metal hosts without re-provisioning.
- **VLAN ID from CUDN annotation** — Matches OVN LocalNet tagging. VPC routes to the correct interface.
- **Associated with CUDN VPC subnet** — Traffic enters the correct subnet.

### 3.2 Floating Virtual Network Interfaces (VNIs)

Each VM gets a dedicated floating VNI — the VM's identity in the VPC (MAC, IP, SGs, FIP):

| VNI Setting | Purpose |
|-------------|---------|
| `auto_delete: false` | VNI persists across host reprovisioning and live migration. |
| `allow_ip_spoofing: true` | VM MAC differs from VLAN interface MAC. Spoofing must be allowed. |
| `enable_infrastructure_nat: false` | VM needs its own routable IP, not hidden behind host NAT. |

### 3.3 MAC Address as Identity Anchor

The VPC auto-generates a MAC when a VNI is created. This MAC is the critical link between the VM (inside the cluster) and the VNI (in the VPC). The operator reads the MAC from the VPC API response and injects it into the VM spec via the mutating webhook.

When the VM boots, it uses this MAC on its localnet interface. The VPC fabric sees traffic with this MAC on the VLAN attachment and associates it with the correct VNI, applying security groups and routing to/from the reserved IP.

The reserved IP is injected via cloud-init network-config, or served via DHCP from the MAC-to-reserved-IP binding.

---

## 4. CUDN Annotation Schema

### 4.1 Required Annotations

```yaml
apiVersion: k8s.ovn.org/v1
kind: ClusterUserDefinedNetwork
metadata:
  name: vm-network-1
  annotations:
    vpc.roks.ibm.com/zone: "us-south-1"
    vpc.roks.ibm.com/cidr: "10.240.64.0/24"
    vpc.roks.ibm.com/vpc-id: "r006-xxxx-xxxx-xxxx"
    vpc.roks.ibm.com/vlan-id: "100"
    vpc.roks.ibm.com/security-group-ids: "r006-sg1,r006-sg2"
    vpc.roks.ibm.com/acl-id: "r006-acl1"
spec:
  topology: LocalNet
```

| Annotation | Required | Description |
|------------|----------|-------------|
| `vpc.roks.ibm.com/zone` | Yes | VPC zone for the subnet. Validated against cluster zones. |
| `vpc.roks.ibm.com/cidr` | Yes | CIDR block for the VPC subnet. Validated for non-overlap. |
| `vpc.roks.ibm.com/vpc-id` | Yes | VPC ID in which to create the subnet. |
| `vpc.roks.ibm.com/vlan-id` | Yes | VLAN ID for OVN LocalNet and BM VLAN attachments. Must be unique per CUDN. |
| `vpc.roks.ibm.com/security-group-ids` | Yes | Comma-separated pre-existing security group IDs. Attached to each VM VNI. |
| `vpc.roks.ibm.com/acl-id` | Yes | Pre-existing network ACL ID. Applied to VPC subnet at creation. |

### 4.2 Status Annotations (Operator-written)

```yaml
# Written by operator after successful provisioning:
vpc.roks.ibm.com/subnet-id: "0717-xxxx-xxxx-xxxx"
vpc.roks.ibm.com/subnet-name: "roks-<cluster>-<cudn>"
vpc.roks.ibm.com/subnet-status: "active"
vpc.roks.ibm.com/subnet-error: ""              # Set on failure
vpc.roks.ibm.com/vlan-attachments: "node1:att-id-1,node2:att-id-2"
```

---

## 5. VirtualMachine Annotations

### 5.1 Admin Annotations (Optional)

```yaml
vpc.roks.ibm.com/fip: "true"   # Request a floating IP
```

### 5.2 Operator-managed Annotations (Single-Network VMs)

| Annotation | Description |
|------------|-------------|
| `vpc.roks.ibm.com/vni-id` | VPC Virtual Network Interface ID bound to this VM. |
| `vpc.roks.ibm.com/mac-address` | VPC-generated MAC (also injected into VM interface spec). |
| `vpc.roks.ibm.com/reserved-ip` | Private IP reserved on the VPC subnet. |
| `vpc.roks.ibm.com/reserved-ip-id` | VPC reserved IP resource ID (for cleanup). |
| `vpc.roks.ibm.com/fip-id` | Floating IP ID (if requested). |
| `vpc.roks.ibm.com/fip-address` | Public floating IP address (if requested). |
| `vpc.roks.ibm.com/attachment-id` | Per-VM VLAN attachment ID. |
| `vpc.roks.ibm.com/bm-server-id` | Bare metal server ID hosting the VLAN attachment. |

### 5.3 Multi-Network VM Annotations

For VMs with multiple LocalNet interfaces, the webhook uses JSON annotations instead of the single-value annotations above:

| Annotation | Description |
|------------|-------------|
| `vpc.roks.ibm.com/network-interfaces` | JSON array of `{interface, network, vniID, mac, reservedIP, reservedIPID, fipID, fipAddress}` objects. |
| `vpc.roks.ibm.com/fip-networks` | Comma-separated interface names requesting FIPs (e.g., `"net1,net2"`). |
| `vpc.roks.ibm.com/nncp-name` | Name of the NNCP created for OVN bridge-mappings. |

---

## 6. Reconciler Specifications

### 6.1 CUDN Reconciler

**Watches:** `ClusterUserDefinedNetwork` with `spec.topology == LocalNet`
**Trigger:** Create, Update, Delete events

#### 6.1.1 Creation Flow

1. Validate all required annotations are present and correct.
2. Validate zone matches a cluster zone.
3. Validate CIDR does not overlap existing VPC subnets.
4. Validate VLAN ID is not in use by another CUDN.
5. Validate referenced security groups and ACL exist.
6. Add finalizer `vpc.roks.ibm.com/cudn-cleanup`.
7. Create VPC subnet via `POST /subnets` with CIDR, zone, VPC ID, ACL.
8. Tag VPC subnet with cluster ID and CUDN name.
9. For each bare metal node: create VLAN attachment on PCI interface (`floatable: true`, VLAN ID, subnet).
10. Write status annotations (subnet-id, subnet-status, vlan-attachments).

#### 6.1.2 Deletion Flow

1. Block if any VMs reference this CUDN (emit warning event).
2. Delete VLAN attachments on all bare metal nodes for this VLAN ID.
3. Delete VPC subnet.
4. Remove finalizer.

### 6.2 Node Reconciler

**Watches:** `Node` objects (filtered to bare metal workers)
**Trigger:** Node Ready, Node delete/drain

#### 6.2.1 Node Join

1. List all LocalNet CUDNs.
2. For each, look up VPC subnet ID and VLAN ID from status annotations.
3. Create VLAN attachment on the new node's PCI interface (`floatable: true`, VLAN ID, subnet).
4. Update CUDN status annotation to include the new node.

#### 6.2.2 Node Removal

Delete all VLAN attachments for the removed node across all CUDNs. Update CUDN status annotations.

### 6.3 VM Reconciler

**Watches:** `VirtualMachine` CRs with operator-managed annotations
**Trigger:** Update, Delete events

#### 6.3.1 Drift Detection

Periodically verifies VPC resources (VNI, reserved IP) still exist. Emits warning events if deleted out-of-band.

#### 6.3.2 Deletion Flow

1. Finalizer `vpc.roks.ibm.com/vm-cleanup` fires.
2. Delete floating IP (if present) via `DELETE /floating_ips/{fip-id}`.
3. Delete VNI via `DELETE /virtual_network_interfaces/{vni-id}` (auto-deletes reserved IP).
4. Remove finalizer.

### 6.4 UDN Reconciler

**Watches:** `UserDefinedNetwork` (namespace-scoped) with LocalNet or Layer2 topology
**Trigger:** Create, Update, Delete events

Same logic as the CUDN reconciler but namespace-scoped. Uses finalizer `vpc.roks.ibm.com/udn-cleanup`.

### 6.5 VPCGateway Reconciler

**Watches:** `VPCGateway` CRD, `VPCRouter` status changes (for route advertisement collection)
**Trigger:** Create, Update, Delete events on VPCGateway; status updates on associated VPCRouters

The gateway reconciler also watches VPCRouter objects. When any router's `status.advertisedRoutes` change, the gateway automatically collects `advertisedRoutes` from all routers that reference it and creates or deletes VPC routes accordingly. This provides dynamic route advertisement without manual `vpcRoutes` entries.

#### 6.5.1 Creation Flow

1. Add finalizer `vpc.roks.ibm.com/gateway-cleanup`.
2. Look up uplink CUDN for subnet ID and VLAN ID.
3. Pick a bare metal server, create VLAN attachment with inline VNI (`allow_ip_spoofing: true`, `enable_infrastructure_nat: false`).
4. Create VPC routes for each `spec.vpcRoutes[].destination` and each advertised route from associated VPCRouters (action: deliver, next_hop: VNI reserved IP).
5. If `spec.floatingIP.enabled`: create or adopt floating IP bound to VNI.
6. If `spec.publicAddressRange.enabled`: create PAR, ingress routing table, and ingress routes.

#### 6.5.2 Deletion Flow

1. Delete PAR resources (ingress routes, routing table, PAR).
2. Delete or unbind floating IP.
3. Delete VPC routes.
4. Delete VLAN attachment (auto-deletes inline VNI).
5. Remove finalizer.

### 6.6 VPCRouter Reconciler

**Watches:** `VPCRouter` CRD, `VPCGateway` changes (NAT, firewall, image, MAC)
**Trigger:** Create, Update, Delete events on VPCRouter; spec changes on referenced VPCGateway

The router reconciler watches the referenced VPCGateway. If the gateway's NAT rules, firewall rules, container image, or uplink MAC address change, the router pod is automatically recreated to pick up the new configuration.

#### 6.6.1 Creation Flow

1. Add finalizer `vpc.roks.ibm.com/router-cleanup`.
2. Validate referenced VPCGateway exists and is Ready.
3. Create a privileged router pod with Multus attachments to uplink and workload networks. Pod includes a liveness probe (`sysctl -n net.ipv4.ip_forward`) and readiness probe (uplink interface UP check).
4. Pod configures IP forwarding, nftables NAT/firewall, and optional dnsmasq DHCP.
5. Update `status.podIP` with the router pod's IP address.

#### 6.6.2 Deletion Flow

1. Delete router pod.
2. Remove finalizer.

### 6.7 Finalizer Summary

| Finalizer | Applied to | Cleans up |
|-----------|-----------|-----------|
| `vpc.roks.ibm.com/cudn-cleanup` | CUDN | VPC subnet + VLAN attachments |
| `vpc.roks.ibm.com/udn-cleanup` | UDN | VPC subnet + VLAN attachments |
| `vpc.roks.ibm.com/vm-cleanup` | VirtualMachine | VNI + reserved IP + floating IP |
| `vpc.roks.ibm.com/gateway-cleanup` | VPCGateway | FIP + PAR + VPC routes + VLAN attachment + VNI |
| `vpc.roks.ibm.com/router-cleanup` | VPCRouter | Router pod |

---

## 7. Mutating Admission Webhook

### 7.1 Overview

Intercepts `VirtualMachine` CREATE requests. The admin creates a standard KubeVirt VM and the webhook handles all VPC provisioning before the object is persisted.

### 7.2 Webhook Flow

1. Intercept VirtualMachine CREATE admission request.
2. Inspect VM spec for network interfaces referencing a LocalNet-backed CUDN (via Multus `networkName`).
3. If no localnet interfaces found, allow unmodified (pass-through).
4. Look up CUDN to get VPC subnet ID from status annotations.
5. Create floating VNI on VPC subnet: `auto_delete: false`, `allow_ip_spoofing: true`, `enable_infrastructure_nat: false`, SGs from CUDN annotations. Tag with cluster ID, namespace, VM name.
6. Read VPC-generated MAC address and reserved IP from API response.
7. If VM has annotation `vpc.roks.ibm.com/fip: "true"`, create and attach floating IP.
8. Mutate VM spec: set `macAddress` on matching interface. Inject reserved IP into cloud-init `network-config`.
9. Set operator-managed annotations and add finalizer `vpc.roks.ibm.com/vm-cleanup`.
10. Return mutated admission response.

### 7.3 Error Handling

- **VPC API failure:** Reject admission with clear error. Admin retries `kubectl apply`.
- **Idempotency:** Use VM namespace/name as deterministic VNI tag. Reuse if VNI already exists.
- **Orphaned VNIs:** If webhook creates VNI but a later validation webhook rejects the VM, the Orphan GC job detects and deletes the VNI.

### 7.4 Timeout Configuration

VNI creation: 1–3 seconds. Webhook timeout: 15s. K8s API server admission timeout: 30s.

---

## 8. VPC API Call Reference

### 8.1 Create Subnet (per CUDN)

```json
POST /v1/subnets
{
  "name": "roks-{cluster-id}-{cudn-name}",
  "vpc": { "id": "{vpc-id}" },
  "zone": { "name": "{zone}" },
  "ipv4_cidr_block": "{cidr}",
  "network_acl": { "id": "{acl-id}" },
  "resource_group": { "id": "{rg-id}" }
}
```

### 8.2 Create VLAN Attachment (per node, per CUDN)

```json
POST /v1/bare_metal_servers/{bm-id}/network_attachments
{
  "name": "roks-{cudn-name}-vlan{vlan-id}",
  "interface_type": "vlan",
  "vlan": {vlan-id},
  "virtual_network_interface": {
    "subnet": { "id": "{subnet-id}" },
    "allow_ip_spoofing": true,
    "enable_infrastructure_nat": false
  },
  "allow_to_float": true
}
```

### 8.3 Create Floating VNI (per VM)

```json
POST /v1/virtual_network_interfaces
{
  "name": "roks-{cluster-id}-{ns}-{vm-name}",
  "subnet": { "id": "{subnet-id}" },
  "primary_ip": { "auto_delete": true },
  "allow_ip_spoofing": true,
  "enable_infrastructure_nat": false,
  "auto_delete": false,
  "security_groups": [
    { "id": "{sg-id-1}" },
    { "id": "{sg-id-2}" }
  ]
}
```

**Response includes:** `id`, `mac_address` (auto-generated), `primary_ip.address`, `primary_ip.id`

### 8.4 Create Floating IP (optional, per VM)

```json
POST /v1/floating_ips
{
  "name": "roks-{cluster-id}-{ns}-{vm-name}-fip",
  "zone": { "name": "{zone}" },
  "target": { "id": "{vni-id}" }
}
```

### 8.5 Resource Deletion

```
DELETE /v1/floating_ips/{fip-id}
DELETE /v1/virtual_network_interfaces/{vni-id}
DELETE /v1/bare_metal_servers/{bm-id}/network_attachments/{att-id}
DELETE /v1/subnets/{subnet-id}
```

---

## 9. Administrator Workflow

### 9.1 One-time Setup

1. Install the ROKS VPC Network Operator (Helm chart or OLM).
2. Configure the operator with the VPC API key secret (same pattern as CSI driver).
3. Pre-create security groups and ACLs in the VPC console or via Terraform.

### 9.2 Creating a VM Network

Apply a CUDN with LocalNet topology and `vpc.roks.ibm.com/*` annotations. The operator automatically provisions the VPC subnet and VLAN attachments on all bare metal nodes. Monitor via `kubectl describe cudn <name>`.

### 9.3 Deploying a VM

Standard KubeVirt VM yaml — no VPC-specific fields needed beyond the optional FIP annotation:

```yaml
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: my-vm
  namespace: default
  annotations:
    vpc.roks.ibm.com/fip: "true"  # optional
spec:
  running: true
  template:
    spec:
      networks:
      - name: vpc-net
        multus:
          networkName: vm-network-1
      domain:
        devices:
          interfaces:
          - name: vpc-net
            bridge: {}
      volumes:
      - name: cloudinit
        cloudInitNoCloud:
          userData: |
            #cloud-config
            password: changeme
```

After `kubectl apply`, the webhook injects the MAC address and cloud-init network config transparently. Verify:

```bash
kubectl get vm my-vm -o jsonpath='{.metadata.annotations}'
```

### 9.4 Deleting a VM

`kubectl delete vm my-vm` — finalizer cleans up VNI, reserved IP, and FIP.

### 9.5 Deleting a Network

`kubectl delete cudn vm-network-1` — blocked if VMs exist. Once VMs removed, deletes VLAN attachments and subnet.

---

## 10. Code Structure and Implementation Guide

### 10.1 Repository Layout

```
roks-vpc-network-operator/
├── cmd/
│   └── manager/
│       └── main.go                      # Entrypoint: sets up manager, registers controllers + webhook
├── pkg/
│   ├── controller/
│   │   ├── cudn/
│   │   │   ├── reconciler.go            # CUDN reconciliation logic
│   │   │   └── reconciler_test.go
│   │   ├── node/
│   │   │   ├── reconciler.go            # Node reconciliation logic
│   │   │   └── reconciler_test.go
│   │   └── vm/
│   │       ├── reconciler.go            # VM reconciliation logic
│   │       └── reconciler_test.go
│   ├── webhook/
│   │   ├── vm_mutating.go               # Mutating admission webhook handler
│   │   └── vm_mutating_test.go
│   ├── vpc/
│   │   ├── client.go                    # VPC API client wrapper (auth, base URL, versioning)
│   │   ├── client_test.go
│   │   ├── subnet.go                    # CreateSubnet, DeleteSubnet, GetSubnet
│   │   ├── vni.go                       # CreateVNI, DeleteVNI, GetVNI, ListVNIsByTag
│   │   ├── vlan_attachment.go           # CreateVLANAttachment, DeleteVLANAttachment, ListAttachments
│   │   ├── floating_ip.go              # CreateFIP, DeleteFIP, AttachFIP
│   │   └── ratelimiter.go              # Token bucket rate limiter for VPC API
│   ├── annotations/
│   │   └── keys.go                      # All annotation key constants (vpc.roks.ibm.com/*)
│   ├── finalizers/
│   │   └── finalizers.go                # AddFinalizer, RemoveFinalizer, HasFinalizer helpers
│   └── gc/
│       └── orphan_collector.go          # Periodic scan for orphaned VPC resources
├── config/
│   ├── manager/                         # Kustomize base for manager deployment
│   ├── webhook/
│   │   ├── mutatingwebhookconfiguration.yaml
│   │   └── service.yaml
│   ├── rbac/
│   │   ├── role.yaml
│   │   └── rolebinding.yaml
│   └── samples/
│       ├── cudn-localnet.yaml           # Example CUDN with annotations
│       └── vm-with-vpc.yaml             # Example VM with FIP annotation
├── deploy/
│   └── helm/
│       └── roks-vpc-network-operator/
│           ├── Chart.yaml
│           ├── values.yaml
│           └── templates/
├── Dockerfile
├── Makefile
├── go.mod
└── go.sum
```

### 10.2 Key Dependencies

| Dependency | Version | Purpose |
|------------|---------|---------|
| `sigs.k8s.io/controller-runtime` | >= 0.18 | Controller framework, webhook server, reconciler base |
| `github.com/IBM/vpc-go-sdk` | latest | IBM Cloud VPC API client |
| `kubevirt.io/client-go` | >= 1.3 | VirtualMachine types and client |
| `k8s.ovn.org` types | 4.20+ | ClusterUserDefinedNetwork types |

### 10.3 RBAC Requirements

| Resource | API Group | Verbs |
|----------|-----------|-------|
| `clusteruserdefinednetworks` | `k8s.ovn.org` | get, list, watch, update, patch |
| `virtualmachines` | `kubevirt.io` | get, list, watch, update, patch |
| `nodes` | core | get, list, watch |
| `secrets` | core | get (for VPC API key) |
| `events` | core | create, patch |

### 10.4 IAM Permissions

The VPC service ID needs (scoped to cluster resource group):

- **VPC Infrastructure Services: Editor** — subnet, VNI, VLAN attachment, FIP, reserved IP CRUD
- **VPC Infrastructure Services: IP Spoofing Operator** — for `allow_ip_spoofing` on VNIs

---

## 11. Operational Concerns

### 11.1 Rate Limiting

VPC API client uses a token bucket. Webhook caps concurrent VPC API calls at 10 via shared semaphore. Controller-runtime provides built-in rate limiting for reconciler work queues.

### 11.2 Drift Detection

Every 5 minutes, verify VPC resources referenced in CUDN/VM annotations still exist and are correctly configured. Report drift as Kubernetes events and status conditions — do not auto-correct (avoids conflicts with Terraform or console changes).

### 11.3 Orphan Garbage Collection

Runs every 10 minutes. Scans for VPC resources tagged with the cluster ID that have no corresponding K8s object. Grace period: 15 minutes before deletion. Covers all operator-managed VPC resource types: VNIs, floating IPs, public address ranges (PARs), and VPC routes.

### 11.4 Scaling Considerations

N nodes × M CUDNs = N×M VLAN attachments + 1 VNI per VM. For 20 nodes and 5 CUDNs: 100 VLAN attachments. Node reconciler batches creation with rate limiting.

### 11.5 Live Migration

`floatable: true` VLAN attachments + `auto_delete: false` VNIs = transparent KubeVirt live migration. VNI with its MAC/IP follows the VM. No operator intervention needed — destination node already has the VLAN attachment (guaranteed by Node Reconciler).

### 11.6 Observability

- **Kubernetes events** on CUDN and VM objects for all VPC operations (success and failure).
- **Prometheus metrics:** `vpc_api_calls_total`, `vpc_api_errors_total`, `vpc_api_latency_seconds`, `vni_count`, `subnet_count`, `orphan_gc_deleted_total`.
- **Structured logging** with VPC resource IDs for correlation with IBM Cloud Activity Tracker.

---

## 12. Sample Manifests

### 12.1 MutatingWebhookConfiguration

```yaml
apiVersion: admissionregistration.k8s.io/v1
kind: MutatingWebhookConfiguration
metadata:
  name: roks-vpc-network-operator-webhook
webhooks:
- name: vm-vpc-inject.roks.ibm.com
  admissionReviewVersions: ["v1"]
  sideEffects: None
  timeoutSeconds: 30
  clientConfig:
    service:
      name: roks-vpc-network-operator-webhook
      namespace: roks-vpc-network-operator
      path: /mutate-virtualmachine
  rules:
  - apiGroups: ["kubevirt.io"]
    apiVersions: ["v1"]
    resources: ["virtualmachines"]
    operations: ["CREATE"]
  failurePolicy: Fail
  reinvocationPolicy: IfNeeded
```

### 12.2 Example CUDN

```yaml
apiVersion: k8s.ovn.org/v1
kind: ClusterUserDefinedNetwork
metadata:
  name: vm-production-network
  annotations:
    vpc.roks.ibm.com/zone: "us-south-1"
    vpc.roks.ibm.com/cidr: "10.240.64.0/24"
    vpc.roks.ibm.com/vpc-id: "r006-abc123"
    vpc.roks.ibm.com/vlan-id: "100"
    vpc.roks.ibm.com/security-group-ids: "r006-sg-web,r006-sg-mgmt"
    vpc.roks.ibm.com/acl-id: "r006-acl-prod"
spec:
  topology: LocalNet
```

### 12.3 Example VM with Floating IP

```yaml
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: web-server-1
  namespace: production
  annotations:
    vpc.roks.ibm.com/fip: "true"
spec:
  running: true
  template:
    spec:
      networks:
      - name: vpc-net
        multus:
          networkName: vm-production-network
      domain:
        resources:
          requests:
            memory: 4Gi
            cpu: "2"
        devices:
          interfaces:
          - name: vpc-net
            bridge: {}
          disks:
          - name: rootdisk
            disk:
              bus: virtio
      volumes:
      - name: rootdisk
        containerDisk:
          image: quay.io/containerdisks/ubuntu:22.04
      - name: cloudinit
        cloudInitNoCloud:
          userData: |
            #cloud-config
            password: changeme
            chpasswd: { expire: false }
```

---

## 13. Future Considerations

- **CRD graduation:** Introduce a `VPCNetworkBinding` CRD if the annotation schema grows too complex.
- **CIDR auto-allocation:** Integrate with IPAM to auto-allocate non-overlapping CIDRs from VPC address prefixes.
- **Security group lifecycle:** Manage SG rules as Kubernetes-native objects translated to VPC SG rules.
- **Multi-zone CUDN:** Single CUDN spanning zones, auto-creating a subnet per zone.
- **DNS integration:** Auto-create Custom Resolver DNS records for VM reserved IPs.
- **Quota pre-checks:** Query VPC quotas before resource creation and warn if limits would be exceeded.
