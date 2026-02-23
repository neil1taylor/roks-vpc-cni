# Glossary

A comprehensive A-to-Z reference of terms used throughout the VPC Network Operator documentation. If you are new to IBM Cloud, Kubernetes, or virtualization, start here.

---

### ACL (Network Access Control List)

A set of ordered rules that control inbound and outbound traffic at the subnet level. Each rule specifies an action (allow or deny), a direction, a protocol, and source/destination CIDR ranges. In IBM Cloud VPC, every subnet is associated with exactly one network ACL.

### Admission Webhook

A Kubernetes extension point that intercepts API requests (such as creating a resource) before the object is persisted. The VPC Network Operator uses a **mutating admission webhook** to automatically inject VPC networking configuration into VirtualMachine resources at creation time.

### Annotation

A key-value pair attached to a Kubernetes resource's metadata. The VPC Network Operator uses annotations prefixed with `vpc.roks.ibm.com/` to configure and track VPC resource state on CUDNs and VirtualMachines.

### Bare Metal Server

A physical server (not a virtual machine) provisioned in IBM Cloud VPC. In ROKS clusters, bare metal workers provide the hardware isolation and performance required for running KubeVirt virtual machines. Each bare metal server has one or more PCI network interfaces.

### BFF (Backend for Frontend)

An API server that sits between the OpenShift Console plugin (frontend) and the IBM Cloud VPC API (backend). It aggregates data from multiple sources, handles authentication, and enforces RBAC authorization. The VPC Network Operator includes a BFF service at `cmd/bff/`.

### CIDR (Classless Inter-Domain Routing)

A notation for specifying IP address ranges, written as an IP address followed by a slash and a prefix length (e.g., `10.240.64.0/24` represents 256 addresses from 10.240.64.0 to 10.240.64.255).

### Cloud-init

An industry-standard tool for automating the initialization of cloud instances on first boot. The VPC Network Operator injects network configuration (IP address, gateway) into a VM's cloud-init data so the VM automatically configures its network interface at startup.

### Cluster Mode

The VPC Network Operator supports two modes: **ROKS** (managed by the IBM Cloud ROKS platform) and **Unmanaged** (the operator manages VPC resources directly via the VPC API). Set via the `CLUSTER_MODE` environment variable.

### Controller (Kubernetes Controller)

A control loop that watches the state of Kubernetes resources and takes action to move the current state toward the desired state. The VPC Network Operator contains seven controllers (reconcilers), each responsible for a different resource type.

### Controller-Runtime

A Go framework (`sigs.k8s.io/controller-runtime`) for building Kubernetes controllers and webhooks. It provides the reconciler pattern, client abstractions, and manager lifecycle used by the VPC Network Operator.

### CRD (Custom Resource Definition)

A Kubernetes extension that defines a new resource type. The VPC Network Operator defines four CRDs: `VPCSubnet`, `VirtualNetworkInterface`, `VLANAttachment`, and `FloatingIP`.

### CUDN (ClusterUserDefinedNetwork)

A Kubernetes custom resource from the OVN-Kubernetes project (API group `k8s.ovn.org/v1`) that defines a cluster-wide user-defined network. The VPC Network Operator watches CUDNs with `topology: LocalNet` and provisions corresponding VPC subnets and VLAN attachments.

### Drift Detection

The process of periodically verifying that VPC resources referenced by Kubernetes objects still exist and are correctly configured. If a resource has been modified or deleted outside the operator (e.g., via the IBM Cloud console), the operator emits a warning event.

### Finalizer

A Kubernetes mechanism that prevents a resource from being deleted until cleanup logic has run. The VPC Network Operator adds finalizers to CUDNs and VMs to ensure VPC resources (subnets, VNIs, floating IPs) are deleted before the Kubernetes object is removed.

### Floating IP (FIP)

A public IPv4 address that can be associated with a VPC resource (such as a VNI) to enable inbound internet access. Floating IPs persist independently of the resource they are attached to and can be moved between resources.

### Helm

A package manager for Kubernetes that uses "charts" (templated YAML bundles) to deploy applications. The VPC Network Operator is deployed via a Helm chart located at `deploy/helm/roks-vpc-network-operator/`.

### IAM (Identity and Access Management)

IBM Cloud's system for controlling who can access which resources. The VPC Network Operator requires a Service ID with specific IAM roles to create and manage VPC resources.

### Idempotent

An operation that produces the same result whether it is executed once or multiple times. All VPC operations in the operator are idempotent, using resource tags to detect existing resources before attempting creation.

### IP Spoofing

Sending network packets with a source IP address different from the sender's assigned address. The VPC Network Operator enables IP spoofing on VNIs (`allow_ip_spoofing: true`) because KubeVirt VMs use a MAC address that differs from the underlying VLAN interface's MAC.

### KubeVirt

An open-source project that enables running traditional virtual machines inside Kubernetes pods. KubeVirt extends Kubernetes with the `VirtualMachine` custom resource. The VPC Network Operator integrates KubeVirt VMs with IBM Cloud VPC networking.

### Leader Election

A mechanism ensuring only one instance of a controller is actively reconciling at a time, even when multiple replicas are running for high availability. The VPC Network Operator uses controller-runtime's built-in leader election.

### Live Migration

The process of moving a running virtual machine from one physical host to another without downtime. The VPC Network Operator supports live migration through `floatable: true` VLAN attachments and `auto_delete: false` VNIs, so the VM's network identity follows it seamlessly.

### LocalNet

A network topology in OVN-Kubernetes that bridges the cluster's internal overlay network to an external network (such as a VPC subnet) via VLAN tagging. The VPC Network Operator specifically manages CUDNs with LocalNet topology.

### MAC Address

A hardware-level network identifier assigned to a network interface. The VPC API auto-generates a MAC address when a VNI is created. This MAC is the critical link between the VM inside the cluster and its VPC network identity.

### Module Federation

A Webpack feature that allows separately built JavaScript applications to share code at runtime. The OpenShift Console plugin uses Module Federation to integrate the VPC Network Operator's UI pages into the existing console.

### Multus

A Kubernetes CNI (Container Network Interface) meta-plugin that enables attaching multiple network interfaces to pods. KubeVirt VMs reference CUDN networks via Multus network names.

### Namespace

A Kubernetes mechanism for isolating groups of resources within a cluster. VPC Network Operator CRDs (VPCSubnet, VNI, VLANAttachment, FloatingIP) are namespace-scoped, while CUDNs are cluster-scoped.

### Node Reconciler

The VPC Network Operator controller that watches Kubernetes Node objects. When a new bare metal node joins the cluster, it creates VLAN attachments for all existing CUDNs. When a node is removed, it cleans up its VLAN attachments.

### OpenShift Console

The web-based management interface for Red Hat OpenShift clusters. The VPC Network Operator includes a console plugin that adds VPC networking pages (Dashboard, Subnets, VNIs, VLAN Attachments, Floating IPs, Security Groups, ACLs, Topology) to the console.

### Operator (Kubernetes Operator)

A pattern for extending Kubernetes that uses custom resources and controllers to automate the management of complex applications. The VPC Network Operator automates IBM Cloud VPC resource lifecycle for KubeVirt VMs.

### Orphan GC (Garbage Collection)

A periodic background job in the operator that scans for VPC resources tagged with the cluster ID but lacking a corresponding Kubernetes object. After a 15-minute grace period, orphaned resources are deleted. This handles edge cases such as webhook-created VNIs for VMs that were subsequently rejected.

### OVN-Kubernetes

The default network plugin for OpenShift, based on Open Virtual Network (OVN). It provides overlay networking for pods and supports user-defined networks through CUDNs.

### PatternFly

Red Hat's open-source design system used for building enterprise web applications. The VPC Network Operator's console plugin uses PatternFly 5 components for its UI.

### PCI Interface

A physical network interface on a bare metal server connected via the PCI bus. PCI interfaces are created at server provisioning time and cannot be modified while the server is running. VLAN interfaces attach dynamically to PCI interfaces.

### Rate Limiter

A mechanism that controls the frequency of API calls to prevent overwhelming a service. The VPC Network Operator uses a channel-based rate limiter allowing a maximum of 10 concurrent VPC API calls.

### RBAC (Role-Based Access Control)

A Kubernetes authorization mechanism that grants permissions based on roles assigned to users or service accounts. The VPC Network Operator requires specific RBAC permissions and the BFF service enforces RBAC via SubjectAccessReview.

### Reconciler

A controller-runtime concept: a function that is called whenever a watched resource changes. It compares the desired state (the resource spec) with the actual state (VPC resources) and takes corrective action. Also called a "reconciliation loop."

### Reserved IP

A private IPv4 address allocated from a VPC subnet and assigned to a VNI. The reserved IP is the VM's private address within the VPC. It is automatically created when a VNI is created and can be injected into the VM via cloud-init.

### Resource Group

An IBM Cloud organizational concept for grouping related resources (VPCs, subnets, servers) for access control and billing purposes.

### ROKS (Red Hat OpenShift Kubernetes Service)

IBM Cloud's managed OpenShift service. ROKS clusters with bare metal workers are the primary deployment target for the VPC Network Operator.

### Security Group

A set of rules that control inbound and outbound traffic at the network interface level. Unlike ACLs, security groups are stateful (return traffic is automatically allowed). Each VNI is associated with one or more security groups.

### Service ID

An IBM Cloud identity used by applications (rather than human users) to authenticate with IBM Cloud services. The VPC Network Operator uses a Service ID API key to authenticate with the VPC API.

### SubjectAccessReview (SAR)

A Kubernetes API for checking whether a user has permission to perform an action. The BFF service uses SARs to enforce authorization before executing write operations.

### Subnet (VPC Subnet)

A range of IP addresses within a VPC, scoped to a single availability zone. The VPC Network Operator creates one VPC subnet per CUDN. VMs on that CUDN receive IP addresses from this subnet.

### Tag (VPC Resource Tag)

A label attached to IBM Cloud resources for identification and organization. The VPC Network Operator tags all created VPC resources with the cluster ID, namespace, and resource name to enable idempotent creation and orphan detection.

### VLAN (Virtual Local Area Network)

A network segmentation technology that uses numeric IDs (1-4094) to isolate traffic on a shared physical network. The VPC Network Operator uses VLAN IDs to connect OVN LocalNet networks to VPC subnets.

### VLAN Attachment

A software-defined network interface on a bare metal server associated with a specific VLAN ID and VPC subnet. The VPC Network Operator creates one VLAN attachment per CUDN per bare metal node, with `allow_to_float: true` to support VM live migration.

### VNI (Virtual Network Interface)

A VPC resource that represents a network identity (MAC address, IP address, security groups) independent of any specific physical or virtual server. The VPC Network Operator creates a floating VNI for each KubeVirt VM, providing the VM's identity in the VPC.

### VPC (Virtual Private Cloud)

An isolated virtual network environment in IBM Cloud where you can deploy compute resources, define subnets, and control network access. The VPC Network Operator bridges Kubernetes networking with VPC networking.

### Zone (Availability Zone)

A physically isolated data center within an IBM Cloud region. Each VPC subnet exists in exactly one zone. Multi-zone architectures use separate CUDNs (and subnets) per zone.
