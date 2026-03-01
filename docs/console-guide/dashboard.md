# VPC Dashboard

The VPC Dashboard (`/vpc-networking`) is the landing page for VPC networking in the OpenShift Console. It provides an at-a-glance view of all VPC resources associated with your cluster.

---

## Dashboard Contents

The dashboard displays summary cards for each resource type:

### Resource Counts

| Card | Shows |
|------|-------|
| **VPC Subnets** | Total subnets managed by the operator, with breakdown by status (Synced, Pending, Failed) |
| **Virtual Network Interfaces** | Total VNIs, mapped to VMs |
| **VLAN Attachments** | Total attachments across all bare metal nodes |
| **Floating IPs** | Total floating IPs with allocated addresses |
| **Security Groups** | Total security groups in the VPC |
| **Network ACLs** | Total network ACLs in the VPC |
| **CUDNs** | Total ClusterUserDefinedNetworks (LocalNet and Layer2) |
| **UDNs** | Total UserDefinedNetworks |
| **Gateways** | Total VPCGateway resources with sync status |
| **Routers** | Total VPCRouter resources with sync status |

### Status Indicators

Each card shows a status badge:
- **Green (Synced)** — All resources of this type are in sync with the VPC API
- **Yellow (Pending)** — Some resources are still being provisioned
- **Red (Failed)** — One or more resources have sync errors

### Quick Links

Each card links to the corresponding resource list page for detailed viewing.

---

## Cluster Mode Banner

At the top of the dashboard, a banner indicates the cluster mode:

- **ROKS Mode** — "This cluster is managed by ROKS. VNI and VLAN attachment lifecycle is handled by the platform."
- **Unmanaged Mode** — "This cluster operates in unmanaged mode. The operator manages all VPC resources via the VPC API."

---

## Usage

The dashboard is primarily a monitoring tool. Use it to:

1. **Verify installation** — After installing the operator, check that resource counts are populated
2. **Monitor health** — Identify resources with Failed sync status
3. **Navigate** — Click through to detailed resource pages

---

## Next Steps

- [Managing Resources](managing-resources.md) — Detailed resource pages
- [Gateways & Routers](gateways-routers.md) — Gateway and router management
- [Security](security.md) — Security group and ACL management
- [Topology](topology.md) — Visual network map
