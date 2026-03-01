# Managing Resources

This page describes the console plugin pages for viewing and managing VPC networking resources: Networks, Subnets, Virtual Network Interfaces, VLAN Attachments, Floating IPs, Public Address Ranges, and Routes.

For Gateways and Routers, see [Gateways & Routers](gateways-routers.md).

---

## Networks

**Path:** `/vpc-networking/networks`

The Networks page displays all overlay network definitions — both ClusterUserDefinedNetworks (CUDNs) and UserDefinedNetworks (UDNs).

### List View

| Column | Description |
|--------|-------------|
| Name | Network definition name |
| Kind | CUDN or UDN |
| Topology | LocalNet or Layer2 |
| Zone | VPC availability zone (LocalNet only) |
| CIDR | Assigned CIDR block |
| VLAN ID | VLAN tag (LocalNet only) |
| Subnet Status | VPC subnet sync state |

### Detail View

**Path:** `/vpc-networking/networks/:name`

Shows the network definition details including topology, role, CIDR, VLAN ID, subnet mapping, and associated VLAN attachments.

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
- **Reserved IPs** — Table of reserved IPs in the subnet (`/vpc-networking/subnets/:id/reserved-ips`)
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

### Detail View

**Path:** `/vpc-networking/floating-ips/:id`

Shows floating IP details including the public address, target VNI, zone, and sync conditions.

### Actions

- **Create** — Create a new floating IP (specify zone and target VNI)
- **Delete** — Release the floating IP

---

## Public Address Ranges

**Path:** `/vpc-networking/pars`

Public Address Ranges (PARs) provide blocks of contiguous public IPs that can be routed through a VPCGateway.

### List View

| Column | Description |
|--------|-------------|
| Name | PAR name |
| CIDR | Public IP block |
| Zone | Availability zone |
| Gateway | Attached VPCGateway (if any) |
| Status | Provisioned, Pending |

### Detail View

**Path:** `/vpc-networking/pars/:id`

Shows the PAR details including CIDR, zone, prefix length, and attached gateway.

### Actions

- **Create** — Provision a new PAR (specify zone and prefix length)
- **Adopt** — Import an existing VPC PAR into operator management
- **Delete** — Release the PAR

---

## Routes

**Path:** `/vpc-networking/routes`

Displays VPC routing tables and their routes, grouped by routing table.

### List View

| Column | Description |
|--------|-------------|
| Routing Table | Name of the VPC routing table |
| Destination | Route destination CIDR |
| Next Hop | Next hop IP or action |
| Zone | Availability zone |
| Priority | Route priority |

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

- [Gateways & Routers](gateways-routers.md) — VPCGateway and VPCRouter management
- [Security](security.md) — Managing security groups and ACLs
- [Topology](topology.md) — Visual resource map
- [Dashboard](dashboard.md) — Overview dashboard
