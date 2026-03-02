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

The router detail page uses a tabbed layout with up to 4 tabs:

**Overview tab** — Router health card (when metrics enabled), followed by a details card showing Name, Namespace, Gateway (with inline zone badge and phase status), Phase, Transit IP, Advertised Routes, IDS/IPS (color-coded label: blue for IDS, orange for IPS), Metrics (Enabled/Disabled label), DHCP, Sync Status, Created. Below: DHCP Configuration card with global defaults and per-network overrides.

**Monitoring tab** (only visible when `spec.metrics.enabled: true`) — Time range selector (5m/15m/1h/6h/24h), health summary card with uptime and process status, conntrack utilization gauge, DHCP pool utilization gauges (one per network), and per-interface throughput area charts (RX/TX bytes/sec). All data auto-refreshes every 15 seconds.

**Networks tab** — Table showing each network's name, address, connection status, DHCP state, pool range, and reservation count.

**NFT Rules tab** (only visible when `spec.metrics.enabled: true`) — Sortable table of nftables rule hit counters showing table, chain, rule comment, packet count, and byte count.

The **Delete Router** button opens a confirmation modal requiring the router name.

---

## Observability

**Path:** `/vpc-networking/observability`

The Observability page provides a multi-router monitoring view for all metrics-enabled routers.

**Features:**
- **Router selector** dropdown — Switch between metrics-enabled routers
- **Time range selector** — 5m, 15m, 1h, 6h, 24h
- **Health summary** — Router status, uptime, per-interface throughput rates, process status (dnsmasq, suricata, etc.)
- **Conntrack gauge** — Connection tracking table utilization with color thresholds (green < 60%, yellow < 80%, red > 80%)
- **DHCP pool gauges** — Per-network lease utilization donut charts
- **Interface throughput charts** — Per-interface RX/TX area charts with time series data
- **NFT rule counters** — Sortable table of firewall and NAT rule hit counts

When no routers have metrics enabled, the page shows an empty state with instructions to set `spec.metrics.enabled: true` on a VPCRouter.

All metrics data is polled every 15 seconds from the BFF, which queries OpenShift Prometheus via Thanos.

---

## Next Steps

- [Managing Resources](managing-resources.md) — Subnets, VNIs, VLAN Attachments, Floating IPs
- [Dashboard](dashboard.md) — Overview dashboard
- [Tutorial: Gateway & Router](../tutorials/gateway-router.md) — End-to-end walkthrough
