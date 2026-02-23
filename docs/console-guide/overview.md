# Console Plugin Overview

The VPC Network Operator includes an OpenShift Console dynamic plugin that adds VPC networking pages to the OpenShift web console. These pages provide visibility into VPC resources associated with your cluster and allow administrators to manage security groups and network ACLs.

---

## Accessing the Plugin

After [installation](../getting-started/installation.md), the VPC networking pages appear under the **Networking** section in the OpenShift Console sidebar.

If the pages do not appear, ensure the plugin is enabled:

```bash
oc patch consoles.operator.openshift.io cluster \
  --type=merge \
  --patch '{"spec":{"plugins":["vpc-network-console-plugin"]}}'
```

Verify the plugin pods are running:

```bash
oc get pods -n roks-vpc-network-operator -l app=vpc-network-console-plugin
```

---

## Navigation

The plugin adds the following pages under **Networking**:

| Page | Path | Description |
|------|------|-------------|
| [VPC Dashboard](dashboard.md) | `/vpc-networking` | At-a-glance overview of all VPC resources |
| [VPC Subnets](managing-resources.md#subnets) | `/vpc-networking/subnets` | List and detail view of VPC subnets |
| [Virtual Network Interfaces](managing-resources.md#virtual-network-interfaces) | `/vpc-networking/vnis` | List and detail view of VNIs |
| [VLAN Attachments](managing-resources.md#vlan-attachments) | `/vpc-networking/vlan-attachments` | VLAN attachments on bare metal nodes |
| [Floating IPs](managing-resources.md#floating-ips) | `/vpc-networking/floating-ips` | Public floating IP addresses |
| [Security Groups](security.md#security-groups) | `/vpc-networking/security-groups` | Security group management with rule editing |
| [Network ACLs](security.md#network-acls) | `/vpc-networking/network-acls` | Network ACL management with rule editing |
| [Network Topology](topology.md) | `/vpc-networking/topology` | Visual graph of VPC resource relationships |

---

## Cluster Mode Awareness

The console plugin adapts its UI based on the cluster mode:

- **Unmanaged mode** — All create/delete operations are available for VNIs and VLAN Attachments
- **ROKS mode** — VNI and VLAN Attachment pages are read-only (managed by the ROKS platform)

The plugin detects the mode automatically by calling the BFF's `/api/v1/cluster-info` endpoint.

---

## Authentication and Authorization

- **Authentication** is handled by the OpenShift OAuth proxy, which injects user identity headers
- **Read operations** (list, get) are available to all authenticated users
- **Write operations** (create, delete security groups/ACLs and their rules) require appropriate RBAC permissions

See [RBAC](../admin-guide/rbac.md) for configuring access control.

---

## Next Steps

- [Dashboard](dashboard.md) — VPC Dashboard overview
- [Managing Resources](managing-resources.md) — Subnets, VNIs, VLAN Attachments, Floating IPs
- [Security](security.md) — Security Groups and Network ACLs
- [Topology](topology.md) — Network topology viewer
