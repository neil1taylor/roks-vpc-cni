# Key Concepts

This page explains the foundational technologies that the VPC Network Operator builds upon. If you are already familiar with IBM Cloud VPC, Kubernetes, and KubeVirt, you can skip ahead to the [Architecture Overview](../architecture/overview.md).

---

## IBM Cloud VPC Networking

### Virtual Private Cloud (VPC)

A VPC is an isolated network environment within IBM Cloud. Think of it as your own private section of the cloud where you control:
- IP address ranges
- Subnets (subdivisions of the network)
- Who can send and receive traffic (security groups, ACLs)
- Connections to the public internet (floating IPs)

### Subnets

A subnet is a range of IP addresses within a VPC, scoped to a single **availability zone** (data center). For example, `10.240.64.0/24` provides 256 addresses in the `us-south-1` zone. Every resource that needs an IP address in the VPC — including VMs managed by this operator — gets one from a subnet.

### Virtual Network Interface (VNI)

A VNI is a VPC resource that represents a network identity: a MAC address, an IP address, and a set of security groups. VNIs are "floating" — they are not tied to a specific server, which is what enables VM live migration. When the operator creates a VNI for a VM, the VPC auto-generates a unique MAC address and reserves an IP from the subnet.

### VLAN Attachment

Bare metal servers connect to VPC subnets through VLAN attachments. A VLAN attachment is a software-defined network interface associated with a specific VLAN ID and subnet. The operator creates one VLAN attachment per network per bare metal node, with `allow_to_float: true` so that VNIs can move between nodes during live migration.

### Floating IP

A floating IP is a public IPv4 address that can be associated with a VNI to allow inbound traffic from the internet. Floating IPs are independent resources that persist when detached and can be moved between VNIs.

### Security Groups and Network ACLs

Both control network traffic, but at different levels:

| Feature | Security Group | Network ACL |
|---------|---------------|-------------|
| **Scope** | Network interface (VNI) | Subnet |
| **Statefulness** | Stateful (return traffic auto-allowed) | Stateless (explicit rules for both directions) |
| **Default** | Deny all inbound, allow all outbound | Allow all traffic |
| **Rule evaluation** | All rules evaluated | Rules evaluated in order |

The VPC Network Operator attaches admin-configured security groups to each VM's VNI and applies an ACL to each VPC subnet it creates.

---

## Kubernetes Fundamentals

### Custom Resources and CRDs

Kubernetes is extensible. A **Custom Resource Definition** (CRD) lets you define new resource types beyond the built-in ones (Pods, Services, etc.). The VPC Network Operator defines four CRDs:

| CRD | Short Name | Represents |
|-----|-----------|------------|
| `VPCSubnet` | `vsn` | A VPC subnet |
| `VirtualNetworkInterface` | `vni` | A VPC virtual network interface |
| `VLANAttachment` | `vla` | A VLAN attachment on a bare metal server |
| `FloatingIP` | `fip` | A VPC floating IP |

### Operators and Reconcilers

A Kubernetes **operator** is a controller that manages custom resources. It runs a **reconciliation loop**: when a resource changes, the operator compares the desired state (what the resource says) with the actual state (what exists in the VPC) and takes corrective action. The VPC Network Operator has seven reconcilers, each responsible for a different resource type.

### Finalizers

A **finalizer** is a marker on a Kubernetes resource that prevents deletion until cleanup logic has run. When you delete a VM, the operator's finalizer fires first, deletes the VPC resources (VNI, floating IP), and then allows the VM to be removed from Kubernetes. This prevents orphaned VPC resources.

### Mutating Admission Webhooks

A **mutating webhook** intercepts Kubernetes API requests and modifies the resource before it is saved. The VPC Network Operator's webhook intercepts `VirtualMachine` CREATE requests, creates the necessary VPC resources, and injects the MAC address and IP into the VM spec — all transparently to the user.

### Namespaces

Kubernetes **namespaces** provide isolation within a cluster. The VPC Network Operator's CRDs are namespace-scoped, while CUDNs are cluster-scoped (available across all namespaces).

---

## OpenShift and ROKS

### Red Hat OpenShift

An enterprise Kubernetes platform by Red Hat that adds developer and operational tools, a web console, and enhanced security. The VPC Network Operator is designed for OpenShift clusters.

### ROKS (Red Hat OpenShift Kubernetes Service)

IBM Cloud's managed OpenShift offering. ROKS handles the control plane, upgrades, and integration with IBM Cloud services. The VPC Network Operator targets ROKS clusters with bare metal workers.

### OVN-Kubernetes and LocalNet

**OVN-Kubernetes** is OpenShift's default network plugin. It creates an overlay network for pods. A **LocalNet** topology is a special configuration that bridges this overlay to an external network (such as a VPC subnet) using VLAN tagging. This is how KubeVirt VMs get direct access to VPC subnets.

### ClusterUserDefinedNetwork (CUDN)

A CUDN is an OVN-Kubernetes custom resource that defines a cluster-wide user-defined network. When a CUDN has `topology: LocalNet`, it bridges the cluster overlay to a VPC subnet. The VPC Network Operator watches these CUDNs and automatically creates the corresponding VPC resources.

---

## KubeVirt and Virtual Machines

### KubeVirt

**KubeVirt** is an open-source project that adds virtual machine management to Kubernetes. It lets you run traditional VMs (Linux, Windows) alongside containers, using the same Kubernetes tooling (`kubectl`, YAML manifests, RBAC).

### VirtualMachine (VM) Resource

A KubeVirt `VirtualMachine` is a Kubernetes custom resource that defines a VM: its CPU, memory, disks, and network interfaces. When you create a VM that references a LocalNet-backed CUDN, the VPC Network Operator's webhook automatically provisions its VPC networking.

### Live Migration

KubeVirt supports **live migration** — moving a running VM from one physical host to another without downtime. The VPC Network Operator enables this by:
1. Creating VLAN attachments on **every** bare metal node (so the destination already has connectivity)
2. Using `floatable: true` VLAN attachments
3. Using `auto_delete: false` VNIs (so the VNI persists through migration)

The VM keeps its MAC address, IP address, and floating IP throughout the migration.

---

## Putting It All Together

When you deploy a VM with the VPC Network Operator:

1. A **CUDN** defines which VPC subnet the VM connects to (via annotations)
2. The operator has already created the **VPC subnet** and **VLAN attachments** when the CUDN was applied
3. The **mutating webhook** intercepts the VM creation, creates a **VNI** (getting a MAC and IP from the VPC), and injects them into the VM spec
4. The VM boots with its VPC-assigned MAC on the **LocalNet** interface
5. OVS/OVN bridges the VM traffic through the **VLAN attachment** to the VPC fabric
6. The VPC fabric matches the MAC to the VNI, applies **security groups**, and routes traffic

## Next Steps

- [Use Cases](use-cases.md) — See real-world scenarios
- [Architecture Overview](../architecture/overview.md) — Detailed system architecture
- [Quick Start](../getting-started/quick-start.md) — Deploy your first VM in 10 minutes
