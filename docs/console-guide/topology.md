# Network Topology

The Network Topology page (`/vpc-networking/topology`) provides a visual graph of VPC networking resources and their relationships.

---

## What It Shows

The topology view renders an interactive graph with:

### Node Types

| Node Type | Icon | Represents |
|-----------|------|------------|
| VPC | Cloud | IBM Cloud VPCs in the account |
| Security Group | Shield | Security groups within each VPC |
| Network ACL | Shield | Network ACLs within each VPC |
| Subnet | Network | VPC subnets (planned) |
| VNI | Network | Virtual network interfaces (planned) |
| VM | Server | KubeVirt VMs (planned) |

### Edge Types

| Edge Type | Meaning |
|-----------|---------|
| `contains` | Parent-child relationship (e.g., VPC contains SG) |
| `attached-to` | Resource attachment (e.g., VNI attached to subnet, planned) |
| `bound-to` | Binding (e.g., floating IP bound to VNI, planned) |

---

## Interacting with the Graph

### Navigation

- **Zoom** — Scroll to zoom in/out
- **Pan** — Click and drag the background to pan
- **Select** — Click a node to highlight it and its connections
- **Details** — Click a node to view summary information in a side panel

### Filtering

- Filter by node type (show only VPCs, or only security groups)
- Filter by VPC (show resources within a specific VPC)

### Layout

The graph uses an automatic hierarchical layout:
- VPCs at the top level
- Security groups and ACLs grouped under their VPC
- Subnets, VNIs, and VMs at lower levels

---

## Data Sources

The topology data comes from the BFF service's `/api/v1/topology` endpoint, which aggregates:

1. **VPC API** — VPCs, security groups, network ACLs
2. **Kubernetes API** — CRD instances (VPCSubnets, VNIs, VLANAttachments, FloatingIPs) — planned

The graph refreshes automatically and can be manually refreshed using the refresh button.

---

## Use Cases

### Visualize VPC Structure

See all security groups and ACLs organized by VPC, understanding which resources protect which parts of your network.

### Identify Orphaned Resources

Spot security groups or ACLs that are not connected to any subnet or VNI.

### Audit Security Configuration

Quickly see the security posture — how many security groups exist, how they relate to VPCs and subnets.

---

## Next Steps

- [Dashboard](dashboard.md) — Overview with resource counts
- [Security](security.md) — Manage security groups and ACLs
- [Architecture: Console Plugin](../architecture/console-plugin.md) — How the topology is built
