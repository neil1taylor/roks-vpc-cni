# What Is the VPC Network Operator?

## The Problem It Solves

Imagine you work at a company that runs applications inside **virtual machines** (VMs) — software-based computers that behave like physical servers. Your company uses IBM Cloud, and these VMs run inside a **Kubernetes cluster** — a system that orchestrates many containers and VMs across a pool of physical servers.

For each VM to communicate over the network, it needs several things set up in IBM Cloud's **Virtual Private Cloud** (VPC) — a private, isolated network environment:

- A **subnet** (a range of IP addresses) for the VM to live on
- A **virtual network interface** (VNI) that gives the VM its own MAC address and IP address
- **VLAN attachments** on each physical server so traffic can flow between the cluster and the VPC
- Optionally, a **floating IP** — a public address so the VM can be reached from the internet
- **Security groups** and **access control lists** to protect the VM

Without the VPC Network Operator, an administrator must manually create each of these resources across two different systems: the Kubernetes API and the IBM Cloud VPC API. For a single VM, this means multiple API calls, careful coordination of identifiers, and manual cleanup when the VM is deleted. For dozens or hundreds of VMs, this is slow, error-prone, and does not scale.

## What It Does

The **VPC Network Operator** automates all of this. It is a Kubernetes operator — a piece of software that watches for changes in your cluster and automatically manages the corresponding IBM Cloud VPC resources.

Here is what happens when you use it:

1. **An administrator creates a network** by applying a `ClusterUserDefinedNetwork` (CUDN) resource with VPC annotations. The operator automatically creates the VPC subnet and sets up VLAN attachments on every bare metal server in the cluster.

2. **A user creates a virtual machine** by applying a standard KubeVirt `VirtualMachine` resource. A webhook intercepts the request and transparently:
   - Creates a virtual network interface (VNI) in the VPC
   - Reads the VPC-generated MAC address and IP
   - Injects them into the VM configuration
   - Optionally creates a floating IP for public access

3. **When a VM is deleted**, the operator's finalizers automatically clean up the VNI, reserved IP, and floating IP in VPC.

4. **When a bare metal node joins or leaves** the cluster, the operator automatically creates or removes VLAN attachments.

5. **An orphan garbage collector** periodically scans for VPC resources that lost their corresponding Kubernetes object and cleans them up.

The result: administrators and users work only with Kubernetes resources. The VPC Network Operator handles all IBM Cloud VPC operations behind the scenes.

## Key Benefits

- **Zero manual VPC management** — No need to switch between the Kubernetes and IBM Cloud consoles
- **Transparent to VM users** — Users create standard KubeVirt VMs; the webhook handles everything
- **Full lifecycle management** — Resources are created, monitored, and deleted automatically
- **Live migration support** — VMs can move between physical servers without network disruption
- **Drift detection** — The operator warns you if someone modifies VPC resources outside the cluster
- **Orphan cleanup** — No leaked VPC resources, even in edge cases
- **OpenShift Console integration** — A built-in UI for viewing and managing VPC networking resources

## Who Is It For?

| Audience | How They Use It |
|----------|----------------|
| **Platform administrators** | Install the operator, configure VPC credentials, create networks (CUDNs) |
| **Application developers** | Deploy VMs onto existing networks, request floating IPs |
| **Security teams** | Pre-create security groups and ACLs referenced by the operator |
| **Operations teams** | Monitor VPC resource status through the OpenShift Console plugin |

## How It Fits Together

```
                    ┌─────────────────────────────────┐
                    │        Kubernetes Cluster        │
                    │                                  │
  kubectl apply ──► │  CUDN ──► CUDN Reconciler ─────────► VPC Subnet
                    │                                  │    + VLAN Attachments
                    │                                  │
  kubectl apply ──► │  VM ────► Mutating Webhook ─────────► VNI + Reserved IP
                    │           (injects MAC + IP)     │    + Floating IP
                    │                                  │
                    │  Node ──► Node Reconciler ───────────► VLAN Attachments
                    │                                  │
                    │  Orphan GC ──────────────────────────► Cleanup leaked
                    │                                  │    VPC resources
                    └─────────────────────────────────┘
                                    │
                                    ▼
                          IBM Cloud VPC API
```

## Next Steps

- **New to the concepts?** Read [Key Concepts](key-concepts.md) to understand VPC, Kubernetes, and KubeVirt fundamentals
- **Want to see real scenarios?** Read [Use Cases](use-cases.md)
- **Ready to install?** Jump to [Prerequisites](../getting-started/prerequisites.md) and [Installation](../getting-started/installation.md)
- **Unfamiliar with a term?** Check the [Glossary](../glossary.md)
