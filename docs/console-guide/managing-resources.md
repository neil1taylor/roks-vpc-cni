# Managing Resources

This page describes the console plugin pages for viewing and managing VPC networking resources: Subnets, Virtual Network Interfaces, VLAN Attachments, and Floating IPs.

---

## Subnets

**Path:** `/vpc-networking/subnets`

### List View

Displays all VPC subnets managed by the operator as a table with columns:

| Column | Description |
|--------|-------------|
| Name | Subnet CRD name |
| VPC | VPC ID |
| Zone | Availability zone |
| CIDR | IPv4 CIDR block |
| Sync Status | Synced, Pending, or Failed |
| Subnet ID | VPC-assigned subnet ID (expandable) |
| Age | Time since creation |

### Detail View

**Path:** `/vpc-networking/subnets/:name`

Shows the full subnet details:
- **Spec** — VPC ID, zone, CIDR, ACL ID, security group IDs, VLAN ID
- **Status** — Subnet ID, VPC status, available IPv4 count, last sync time, conditions
- **Associated Resources** — VNIs on this subnet, VLAN attachments using this subnet
- **Events** — Kubernetes events related to this subnet

---

## Virtual Network Interfaces

**Path:** `/vpc-networking/vnis`

### List View

| Column | Description |
|--------|-------------|
| Name | VNI CRD name |
| VNI ID | VPC-assigned VNI ID |
| MAC Address | VPC-generated MAC |
| IP Address | Primary reserved IPv4 |
| Subnet | Referenced VPCSubnet |
| Sync Status | Synced, Pending, or Failed |
| Age | Time since creation |

### Detail View

**Path:** `/vpc-networking/vnis/:name`

Shows:
- **Spec** — Subnet reference, security group IDs, IP spoofing, infrastructure NAT, auto-delete settings, VM reference
- **Status** — VNI ID, MAC address, primary IP, reserved IP ID, sync status, conditions
- **Associated VM** — Link to the KubeVirt VirtualMachine this VNI is bound to

### Cluster Mode Behavior

- **Unmanaged mode** — Create and delete actions are available
- **ROKS mode** — VNIs are read-only; managed by the ROKS platform

---

## VLAN Attachments

**Path:** `/vpc-networking/vlan-attachments`

### List View

| Column | Description |
|--------|-------------|
| Name | VLANAttachment CRD name |
| BM Server | Bare metal server ID |
| VLAN ID | VLAN tag number |
| Node | Kubernetes node name |
| Attachment Status | attached, pending, detached, failed |
| Sync Status | Synced, Pending, or Failed |
| Age | Time since creation |

### Cluster Mode Behavior

- **Unmanaged mode** — Create and delete actions are available
- **ROKS mode** — VLAN Attachments are read-only

---

## Floating IPs

**Path:** `/vpc-networking/floating-ips`

### List View

| Column | Description |
|--------|-------------|
| Name | FloatingIP CRD name |
| Address | Public IPv4 address |
| Zone | Availability zone |
| VNI | Target VNI reference |
| Sync Status | Synced, Pending, or Failed |
| Age | Time since creation |

### Actions

- **Create** — Create a new floating IP (specify zone and target VNI)
- **Delete** — Release the floating IP

---

## Common Patterns

### Filtering

All list views support filtering by:
- Sync status (Synced, Pending, Failed)
- Namespace
- Text search on name

### Sorting

Click column headers to sort by any column. Default sort is by creation time (newest first).

### Refresh

Resource lists auto-refresh periodically. Click the refresh button for immediate updates.

---

## Next Steps

- [Security](security.md) — Managing security groups and ACLs
- [Topology](topology.md) — Visual resource map
- [Dashboard](dashboard.md) — Overview dashboard
