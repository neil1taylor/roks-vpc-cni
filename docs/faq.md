# Frequently Asked Questions

Questions grouped by audience.

---

## General

### What is the VPC Network Operator?

A Kubernetes operator that automates IBM Cloud VPC resource management for KubeVirt virtual machines running on bare metal OpenShift (ROKS) clusters. It creates VPC subnets, virtual network interfaces, VLAN attachments, and floating IPs automatically when you create Kubernetes resources.

See [What Is the VPC Network Operator?](overview/what-is-vpc-network-operator.md) for a full introduction.

### Do I need to learn the IBM Cloud VPC API to use this operator?

No. The operator abstracts away all VPC API interactions. Administrators configure networks using Kubernetes annotations on CUDNs, and users deploy VMs using standard KubeVirt manifests. The operator and webhook handle all VPC operations transparently.

### What happens if the operator goes down?

- **Existing VMs** continue running normally — VPC resources are persistent and independent of the operator
- **New VM creation** will fail (the webhook is unavailable)
- **VM deletion** will wait for the finalizer to run once the operator restarts
- **Orphaned resources** from the downtime period will be cleaned up by the orphan GC after restart

### Does this work with non-bare-metal workers?

No. VLAN attachments require bare metal servers with PCI network interfaces. Virtual server instances (VSIs) do not support VLAN attachments. The operator filters for bare metal nodes using instance type labels.

---

## Administrator Questions

### What IAM permissions does the operator need?

The operator requires a Service ID with:
- **VPC Infrastructure Services: Editor** — for subnet, VNI, VLAN attachment, floating IP, reserved IP CRUD
- **VPC Infrastructure Services: IP Spoofing Operator** — for enabling `allow_ip_spoofing` on VNIs

Both should be scoped to the cluster's resource group. See [Prerequisites](getting-started/prerequisites.md) for setup instructions.

### Can I use an existing VPC subnet instead of letting the operator create one?

Yes. If you pre-create the VPC subnet, you can set the `vpc.roks.ibm.com/subnet-id` annotation on the CUDN manually. The operator will skip subnet creation and use the existing subnet for VLAN attachments.

### How do I rotate the VPC API key?

1. Create a new API key for the Service ID
2. Update the Kubernetes Secret:
   ```bash
   oc delete secret roks-vpc-network-operator-credentials -n roks-vpc-network-operator
   oc create secret generic roks-vpc-network-operator-credentials \
     --namespace roks-vpc-network-operator \
     --from-literal=IBMCLOUD_API_KEY=<new-api-key>
   ```
3. Restart the operator: `oc rollout restart deployment/vpc-network-operator-manager -n roks-vpc-network-operator`

### Can I run the operator in multiple clusters sharing the same VPC?

Yes. Each operator instance uses a unique `cluster.id` for tagging VPC resources. Resources created by different clusters are distinguished by their tags and do not interfere with each other.

### How many VMs and networks can the operator handle?

The operator is designed for production scale. The formula for VLAN attachments is: N nodes x M CUDNs = N x M VLAN attachments. For example, 20 nodes with 5 CUDNs = 100 VLAN attachments. The rate limiter (10 concurrent VPC API calls) prevents overwhelming the VPC API during bulk operations.

Each VM requires one VNI and optionally one floating IP. VPC quotas apply — check `ibmcloud is quotas` for limits.

### What is the difference between ROKS mode and unmanaged mode?

- **ROKS mode** (`CLUSTER_MODE=roks`): The ROKS platform manages VNIs and VLAN attachments. The operator syncs their status into CRDs but does not create or delete them.
- **Unmanaged mode** (`CLUSTER_MODE=unmanaged`): The operator manages all VPC resources directly via the VPC API.

See [Dual Cluster Mode](architecture/dual-cluster-mode.md) for details.

### How does the orphan GC work?

Every 10 minutes, the GC lists VPC resources tagged with the cluster ID and checks if the corresponding Kubernetes object still exists. Resources older than 15 minutes without a K8s object are deleted. Both the interval and grace period are configurable via Helm values.

---

## Developer / User Questions

### How do I deploy a VM with VPC networking?

1. Ensure a CUDN with LocalNet topology exists and is active
2. Create a `VirtualMachine` with a Multus network referencing the CUDN
3. Optionally add `vpc.roks.ibm.com/fip: "true"` for a floating IP
4. `oc apply -f vm.yaml` — the webhook handles VPC provisioning automatically

See [Creating VMs](user-guide/creating-vms.md) and [Quick Start](getting-started/quick-start.md).

### How do I request a floating IP for my VM?

Add the annotation `vpc.roks.ibm.com/fip: "true"` to the VM metadata. The webhook will create a floating IP and write its address to `vpc.roks.ibm.com/fip-address`.

See [Floating IPs](user-guide/floating-ips.md).

### Can I choose the private IP address for my VM?

Not currently. The VPC API assigns a reserved IP from the subnet automatically when the VNI is created. The operator reads and injects this assigned IP.

### Does live migration work with VPC networking?

Yes. The operator creates `floatable: true` VLAN attachments on every bare metal node and `auto_delete: false` VNIs. During KubeVirt live migration, the VNI floats to the destination node's VLAN attachment, preserving the MAC address, IP, and floating IP.

See [Live Migration](user-guide/live-migration.md).

### What happens to VPC resources when I delete a VM?

The `vpc.roks.ibm.com/vm-cleanup` finalizer runs and:
1. Deletes the floating IP (if present)
2. Deletes the VNI (which releases the reserved IP)
3. Removes the finalizer, allowing Kubernetes to complete the deletion

### Can my VM have multiple VPC network interfaces?

Currently, the webhook processes the first LocalNet interface it finds. Multi-interface support may be added in a future release.

---

## Console Plugin Questions

### How do I access the VPC networking pages?

After enabling the console plugin, navigate to **Networking** in the OpenShift Console sidebar. You will see entries for VPC Dashboard, VPC Subnets, Virtual Network Interfaces, VLAN Attachments, Floating IPs, Security Groups, Network ACLs, and Network Topology.

### Why are some buttons grayed out?

Write operations (create, delete) require RBAC permissions. Ask your cluster administrator to grant you the appropriate roles. See [RBAC](admin-guide/rbac.md).

### Why do VNI and VLAN Attachment pages show "managed by platform"?

You are running in ROKS mode. In this mode, the ROKS platform manages VNIs and VLAN attachments, and the console shows them as read-only. If you need to manage them directly, switch to unmanaged mode.

---

## Security Questions

### Is the VPC API key stored securely?

Yes. The API key is stored as a Kubernetes Secret (base64-encoded, with optional encryption at rest via etcd encryption). It is mounted into the operator pod as an environment variable and never exposed via the API or console.

### How is console plugin access controlled?

The console plugin uses the OpenShift OAuth proxy for authentication. Write operations go through the BFF service, which checks authorization via Kubernetes SubjectAccessReview before executing VPC API calls.

### Can I restrict which namespaces can deploy VMs with VPC networking?

Yes. The CUDN is cluster-scoped, but RBAC policies can restrict which namespaces are allowed to create VirtualMachines. Additionally, the `pluginRbac.developerNamespaces` Helm value controls which namespaces get developer access to the console plugin.
