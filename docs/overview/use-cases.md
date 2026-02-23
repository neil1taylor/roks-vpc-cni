# Use Cases

This page describes real-world scenarios where the VPC Network Operator provides value. Each scenario outlines the challenge, how the operator solves it, and which features are involved.

---

## 1. Migrating Legacy VMs to Kubernetes

### Scenario

Your organization runs critical workloads on traditional virtual machines (databases, legacy applications, license-locked software) and wants to modernize its infrastructure by moving to Kubernetes. However, these VMs cannot be containerized — they need a full operating system, specific kernel versions, or hardware-level isolation.

### How the Operator Helps

- Deploy KubeVirt VMs on bare metal ROKS workers alongside containerized workloads
- Each VM gets a first-class VPC network identity (private IP, MAC address, security groups)
- VMs are reachable from other VPC resources (VSIs, load balancers, VPN gateways) using standard VPC networking
- No special networking knowledge required — apply a VM manifest and the webhook handles everything

### Features Used

- Mutating webhook for transparent VPC provisioning
- VNI creation with reserved IP
- Security group attachment
- Cloud-init IP injection

---

## 2. Multi-Tenant VM Hosting with Network Isolation

### Scenario

You are a platform team providing VM-as-a-Service to multiple internal teams. Each team needs its own isolated network segment with separate IP ranges, security policies, and access controls.

### How the Operator Helps

- Create one CUDN per tenant, each mapped to a different VPC subnet with its own CIDR, security groups, and network ACL
- Kubernetes namespaces provide additional isolation — each team deploys VMs in their own namespace
- RBAC controls which teams can create VMs on which networks
- The operator ensures VLAN attachments exist on all bare metal nodes for every tenant network

### Features Used

- Multiple CUDNs with distinct subnets, security groups, and ACLs
- Per-subnet network ACLs for tenant isolation
- RBAC enforcement via the BFF service
- Node Reconciler for automatic VLAN attachment on new nodes

---

## 3. Internet-Facing VMs with Floating IPs

### Scenario

You need to deploy web servers, bastion hosts, or API gateways as VMs that are accessible from the public internet.

### How the Operator Helps

- Add a single annotation (`vpc.roks.ibm.com/fip: "true"`) to the VM and the operator automatically provisions a floating IP and binds it to the VM's VNI
- The floating IP persists independently — if the VM is deleted and recreated, a new floating IP is assigned automatically
- Inbound traffic flows through VPC security groups, giving you fine-grained control

### Example

```yaml
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: bastion-host
  annotations:
    vpc.roks.ibm.com/fip: "true"
spec:
  running: true
  template:
    spec:
      networks:
      - name: vpc-net
        multus:
          networkName: management-network
      domain:
        devices:
          interfaces:
          - name: vpc-net
            bridge: {}
```

### Features Used

- Floating IP provisioning via annotation
- Webhook-based FIP creation
- Security group enforcement on inbound traffic

---

## 4. High-Availability VMs with Live Migration

### Scenario

You run stateful workloads (databases, message queues) as VMs that cannot tolerate downtime during node maintenance, upgrades, or hardware failures.

### How the Operator Helps

- The operator creates `floatable: true` VLAN attachments on every bare metal node for each CUDN
- VNIs are created with `auto_delete: false`, so the network identity persists through migration
- When KubeVirt live-migrates a VM to another node, the VNI (with its MAC, IP, and floating IP) follows seamlessly
- No operator intervention is needed — the destination node already has the required VLAN attachment

### What Happens During Migration

1. KubeVirt initiates live migration to a target node
2. The VM's memory is copied to the target
3. The VNI "floats" from the source VLAN attachment to the target's VLAN attachment
4. Traffic continues flowing to the same MAC/IP address with minimal interruption

### Features Used

- `floatable: true` VLAN attachments
- `auto_delete: false` VNIs
- Node Reconciler ensuring all nodes have VLAN attachments
- Drift detection for post-migration verification

---

## 5. Automated VPC Resource Cleanup

### Scenario

In a dynamic environment with frequent VM creation and deletion, VPC resources can leak — VNIs, reserved IPs, and floating IPs left behind when VMs are deleted improperly (force-deleted, deleted while the operator is down, or rejected by a later admission webhook).

### How the Operator Helps

- **Finalizers** on CUDNs and VMs ensure cleanup runs before deletion completes
- **Orphan Garbage Collection** runs every 10 minutes, scanning for VPC resources tagged with the cluster ID that no longer have a corresponding Kubernetes object
- A 15-minute grace period prevents deleting resources that are still being set up
- All VPC resources are tagged with cluster ID, namespace, and name for reliable identification

### Features Used

- Finalizers (`vpc.roks.ibm.com/cudn-cleanup`, `vpc.roks.ibm.com/vm-cleanup`)
- Orphan GC with configurable interval and grace period
- Resource tagging for orphan detection

---

## 6. Centralized VPC Networking Visibility

### Scenario

Your platform team needs a single pane of glass to view all VPC networking resources associated with the cluster — subnets, VNIs, VLAN attachments, floating IPs, security groups, and network ACLs — without switching between the IBM Cloud console and the OpenShift console.

### How the Operator Helps

- The OpenShift Console plugin adds 8 pages under **Networking > VPC Dashboard**
- The VPC Dashboard provides at-a-glance status of all VPC resources
- Dedicated pages for subnets, VNIs, VLAN attachments, floating IPs, security groups, and ACLs
- A **Network Topology** viewer shows the relationships between VPCs, subnets, VMs, and security groups
- The BFF service aggregates data from the VPC API and Kubernetes API

### Features Used

- Console plugin with PatternFly 5 UI
- BFF service for data aggregation
- Network topology visualization
- RBAC-enforced read and write operations

---

## 7. Scaling to Many Nodes and Networks

### Scenario

Your cluster grows from 5 bare metal nodes to 50, with 10 different VM networks (CUDNs). You need the networking infrastructure to scale automatically.

### How the Operator Helps

- When a new bare metal node joins the cluster, the **Node Reconciler** automatically creates VLAN attachments for all existing CUDNs on the new node
- When a new CUDN is created, the **CUDN Reconciler** creates VLAN attachments on all existing nodes
- VPC API rate limiting (10 concurrent calls max) prevents overwhelming the API during bulk operations
- The formula is N nodes x M CUDNs = N x M VLAN attachments. For 50 nodes and 10 CUDNs: 500 VLAN attachments, all managed automatically

### Features Used

- Node Reconciler for new-node provisioning
- CUDN Reconciler for new-network provisioning
- Rate limiter for VPC API protection
- Batch operations with exponential backoff

---

## Next Steps

- [Prerequisites](../getting-started/prerequisites.md) — What you need before installing
- [Quick Start](../getting-started/quick-start.md) — Deploy your first VM in 10 minutes
- [Architecture Overview](../architecture/overview.md) — How it all works under the hood
