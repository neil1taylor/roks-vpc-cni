# Gateways & Routers

This page describes the console plugin pages for managing VPCGateway and VPCRouter resources. Gateways connect overlay networks to the VPC fabric; routers connect workload networks to a gateway for external access.

---

## Gateways

### List Page

**Path:** `/vpc-networking/gateways`

Displays all VPCGateway resources as a table.

| Column | Description |
|--------|-------------|
| Name | Gateway resource name (links to detail page) |
| Zone | VPC availability zone |
| Uplink Network | LocalNet CUDN/UDN used for the uplink |
| Floating IP | Public floating IP address |
| PAR CIDR | Public Address Range CIDR (if enabled) |
| VNI IP | Reserved IP of the uplink VNI |
| Routes | Number of VPC routes managed by this gateway |
| Status | Sync status badge |
| Age | Time since creation |

**Toolbar features:**
- **Search filter** — Filter gateways by name (case-insensitive)
- **Create Gateway** button — Navigates to the create page

**Delete** — Clicking Delete on a row opens a confirmation modal that requires typing the gateway name. The modal explains the impact: removal of the uplink VNI, floating IP, all VPC routes, NAT rules, and any associated PAR.

### Create Page

**Path:** `/vpc-networking/gateways/create`

A form for creating a new VPCGateway with the following fields:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| Name | Text input | Yes | Kubernetes resource name (lowercase, hyphens, numbers) |
| Namespace | Dropdown | No | Populated from cluster namespaces (default: `default`) |
| Zone | Dropdown | Yes | Populated from VPC availability zones via `useZones()` |
| Uplink Network | Dropdown | Yes | Filtered to LocalNet networks only. Shows `"name (CUDN)"` or `"name (UDN)"`. Warning if none exist. |
| Transit Address | Text input | Yes | IPv4 address with inline validation |
| Transit CIDR | Text input | No | IPv4 CIDR with inline validation. Defaults to /24 if omitted. |
| PAR | Toggle + Radio | No | Enable PAR with choice of "Create new" (prefix length dropdown) or "Use existing" (unattached PAR dropdown) |

**Validation:**
- Transit Address and Transit CIDR show error states for invalid input
- The Create button is disabled until all required fields are valid

### Detail Page

**Path:** `/vpc-networking/gateways/:name`

Displays gateway details in four grouped card sections:

**Overview** — Name, Namespace, Zone, Phase, Sync Status, Created

**Networking** — Uplink Network, Transit Network, VNI ID, VNI IP, Floating IP

**Routing** — VPC Routes count, NAT Rules count

**Public Address Range** — Status (Enabled/Disabled), PAR ID, PAR CIDR, Prefix Length, Ingress Routing Table

The **Delete Gateway** button in the Overview section opens a confirmation modal requiring the gateway name.

---

## Routers

### List Page

**Path:** `/vpc-networking/routers`

Displays all VPCRouter resources as a table.

| Column | Description |
|--------|-------------|
| Name | Router resource name (links to detail page) |
| Gateway | Associated VPCGateway (links to gateway detail) |
| Networks | Number of connected networks |
| Transit IP | Router's IP on the transit network |
| Functions | Active functions (NAT, DHCP, Firewall, IDS/IPS) |
| Status | Sync status badge |
| Age | Time since creation |

**Toolbar features:**
- **Search filter** — Filter routers by name (case-insensitive)
- **Create Router** button — Navigates to the create page

**Delete** — Confirmation modal requires typing the router name. Explains impact: router pod removal, network disconnection, and loss of external connectivity for VMs.

### Create Page

**Path:** `/vpc-networking/routers/create`

A form for creating a new VPCRouter:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| Name | Text input | Yes | Kubernetes resource name |
| Gateway | Dropdown | Yes | Populated from existing gateways. Shows `"name (zone, namespace)"`. Warning if none exist. Auto-populates namespace. |
| Namespace | Text input | Auto | Auto-set from the selected gateway's namespace. Read-only when a gateway is selected. |
| Networks | Repeatable row | Yes (at least one) | Each row has a network dropdown (LocalNet only, no duplicates) and an address text input with IPv4 validation |

**Behavior:**
- Selecting a gateway auto-sets the namespace field and disables it
- Network dropdowns filter out already-selected networks to prevent duplicates
- The "Add network" button appends another row; each row has a remove button
- The Create button requires at least one network with a valid IPv4 address

### Detail Page

**Path:** `/vpc-networking/routers/:name`

Displays router details in two cards:

**Overview** — Name, Namespace, Gateway (with inline zone badge and phase status from the gateway), Phase, Transit IP, Advertised Routes, IDS/IPS (color-coded label: blue for IDS, orange for IPS), Functions, Sync Status, Created

**Connected Networks** — Table showing each network's name, address, and connection status (Connected/Disconnected labels).

The **Delete Router** button opens a confirmation modal requiring the router name.

---

## Next Steps

- [Managing Resources](managing-resources.md) — Subnets, VNIs, VLAN Attachments, Floating IPs
- [Dashboard](dashboard.md) — Overview dashboard
- [Tutorial: Gateway & Router](../tutorials/gateway-router.md) — End-to-end walkthrough
