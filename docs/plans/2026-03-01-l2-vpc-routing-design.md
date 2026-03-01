# L2-VPC Tiered Routing Design

**Date**: 2026-03-01
**Status**: Approved

## Problem

Layer2 (OVN overlay) networks are isolated islands. VMs on L2 have no path to the VPC fabric, the internet, or VMs on other L2 networks. LocalNet networks have full VPC identity, but there is no bridge between the two topologies.

## Goals

1. **Egress**: L2 VMs reach internet and VPC services
2. **Ingress**: External clients reach services on L2 VMs (via Floating IP)
3. **East-West**: VMs on different L2 networks communicate with each other
4. **Multi-AZ**: Support stretched L2 networks across availability zones with zone-specific LocalNet uplinks
5. **Extensible**: Pluggable architecture for firewall, tunnelling, and other network functions

## Design: Tiered Routing (inspired by NSX)

Two new CRDs model a tiered routing system inspired by NSX-T's gateway architecture:

- **VPCGateway**: Per-zone gateway that bridges a LocalNet (VPC fabric) and a transit L2 network. Has a VPC identity (VNI, reserved IP), manages VPC custom routes for return traffic, and optionally binds a floating IP for internet ingress. Handles NAT (SNAT/DNAT) for north-south traffic. Watches associated VPCRouters to auto-collect advertised routes.
- **VPCRouter**: Workload router that connects multiple L2 networks and uplinks to a gateway via the transit L2. Handles east-west routing between L2 segments and forwards north-south traffic to the gateway. Can optionally serve DHCP for connected L2 segments. Watches the referenced gateway and auto-recreates its pod on config drift.

A dedicated "transit" L2 network connects the gateway and router. The operator auto-creates this transit network when a VPCGateway is created.

### NSX Feature Mapping

Inspired by VMware NSX 4.x gateway architecture:

| NSX Feature | Our Equivalent | Phase |
|---|---|---|
| Routing (DR — distributed) | Kernel IP forwarding in router pod | Phase 1 |
| NAT (SNAT/DNAT/No-NAT) | nftables rules in gateway pod, CRD-driven | Phase 1 |
| Route advertisement | Router `routeAdvertisement` flags, gateway creates VPC routes | Phase 1 |
| Auto-provisioned transit | Operator creates transit L2 CUDN automatically | Phase 1 |
| Gateway firewall | nftables stateful firewall, rules via CRD | Phase 2 |
| DHCP server on router | dnsmasq in router pod, per-L2 pool config | Phase 3 |
| DNS forwarder | CoreDNS sidecar with conditional forwarding | Future |
| IPsec/WireGuard VPN | WireGuard sidecar for cross-cluster tunnelling | Future |
| QoS profiles | tc/nftables rate limiting per interface | Future |
| Active-active ECMP | Multiple gateway instances with VPC ECMP routes | Future |
| VRF-lite (multi-tenancy) | Separate VPCGateway per tenant with isolated routes | Future |

### DR/SR Architecture (from NSX)

NSX gateways have two logical components — we adopt this pattern:

- **Distributed Router (DR)**: Always present. In our design, this is the kernel `ip_forward` in every router pod. Handles pure IP forwarding for east-west and transit traffic.
- **Service Router (SR)**: Only instantiated when stateful services are needed. In our design, these are network functions (NAT, firewall, DHCP) configured in the router pod when `spec.nat`, `spec.firewall`, or `spec.dhcp` are set on the CRDs.

## Architecture

```
                    Internet / VPC Services
                           |
                    VPC Routing Table
                    (custom routes: L2 CIDRs -> gateway VNI IP)
                           |
              +------------+------------+
              |                         |
     [VPCGateway: gw-zone-1]  [VPCGateway: gw-zone-2]
     Pod w/ 2 interfaces:      Pod w/ 2 interfaces:
       net1: LocalNet (VNI)      net1: LocalNet (VNI)
       net2: transit-L2          net2: transit-L2
     + NAT (SNAT/DNAT)        + NAT (SNAT/DNAT)
              |                         |
    ----------+-------------------------+----------
              |     transit-L2 (stretched OVN, auto-created)
              |
     [VPCRouter: workload-router]
     Pod w/ N+1 interfaces:
       net1: transit-L2 (uplink to gateway)
       net2: l2-app-tier
       net3: l2-db-tier
     + optional DHCP, firewall
              |         |
           L2-App    L2-DB
           (VMs)     (VMs)
```

### Traffic Flows

| Flow | Path |
|------|------|
| L2 VM -> Internet | VM -> router (default gw) -> transit L2 -> gateway SNAT -> LocalNet VNI -> VPC fabric -> Internet |
| Internet -> L2 VM | Internet -> FIP on gateway VNI -> gateway DNAT -> transit L2 -> router -> L2 -> VM |
| L2-A VM -> L2-B VM | VM-A -> router (routes between L2 legs) -> VM-B (stays in OVN, no NAT) |
| L2 VM -> VPC subnet VM | VM -> router -> transit -> gateway (no-NAT rule) -> LocalNet -> VPC route -> VPC subnet VM |
| VPC return to L2 | VPC route (L2 CIDR -> gateway VNI next-hop) -> gateway -> transit -> router -> L2 -> VM |

## CRD Specifications

### VPCGateway

```yaml
apiVersion: vpc.roks.ibm.com/v1alpha1
kind: VPCGateway
metadata:
  name: gw-eu-de-1
spec:
  # Zone this gateway serves (required — LocalNet is zone-scoped)
  zone: eu-de-1

  # Uplink: existing LocalNet CUDN for VPC connectivity
  uplink:
    network: localnet-vpc           # CUDN name
    securityGroupIDs:               # optional SG override for the gateway VNI
      - r010-abc123

  # Downlink: transit L2 network connecting to routers
  # If omitted, operator auto-creates a transit L2 CUDN named "<gateway-name>-transit"
  transit:
    network: ""                     # auto-created if empty
    address: 172.16.0.1/24          # gateway IP on transit segment
    cidr: 172.16.0.0/24             # transit segment CIDR (for auto-creation)

  # VPC routes: operator creates these automatically based on router route advertisements
  # Can also be declared explicitly for static routes
  vpcRoutes:
    - destination: 10.100.0.0/24    # L2-A CIDR
    - destination: 10.200.0.0/24    # L2-B CIDR

  # NAT rules (Phase 1)
  nat:
    # SNAT: masquerade L2 egress traffic behind the gateway's VNI IP
    snat:
      - source: 10.100.0.0/24        # L2-A CIDR
        translatedAddress: ""         # empty = use VNI reserved IP (auto)
        priority: 100
      - source: 10.200.0.0/24        # L2-B CIDR
        translatedAddress: ""
        priority: 100
    # DNAT: expose L2 services externally
    dnat:
      - externalAddress: ""           # empty = use floating IP
        externalPort: 443
        internalAddress: 10.100.0.10  # L2 VM IP
        internalPort: 8443
        protocol: tcp
        priority: 50
    # No-NAT: exempt specific traffic from SNAT (e.g., L2-to-VPC subnet)
    noNat:
      - source: 10.100.0.0/24
        destination: 10.240.0.0/16    # VPC subnet range — don't SNAT
        priority: 10                   # lower number = higher priority

  # Optional: bind a floating IP for internet ingress
  floatingIP:
    enabled: true

  # Gateway firewall rules (Phase 2 — spec'd now, implemented later)
  firewall:
    enabled: false
    rules: []                         # future: stateful nftables rules

  # Router pod configuration
  pod:
    image: de.icr.io/roks/vpc-router:latest
    resources:
      requests: { cpu: 100m, memory: 128Mi }
      limits:   { cpu: 500m, memory: 256Mi }

status:
  phase: Ready                # Pending | Provisioning | Ready | Error
  vniID: "02r7-..."
  reservedIP: 10.240.1.5
  floatingIP: 169.48.x.x
  transitNetwork: gw-eu-de-1-transit    # auto-created transit CUDN name
  vpcRouteIDs:
    - r010-route-1
    - r010-route-2
  interfaces:
    - role: uplink
      network: localnet-vpc
      address: 10.240.1.5
    - role: downlink
      network: gw-eu-de-1-transit
      address: 172.16.0.1
  conditions:
    - type: VNIReady
    - type: RoutesConfigured
    - type: NATConfigured
    - type: PodReady
```

### VPCRouter

```yaml
apiVersion: vpc.roks.ibm.com/v1alpha1
kind: VPCRouter
metadata:
  name: workload-router
spec:
  # Reference to the gateway for north-south traffic
  gateway: gw-eu-de-1

  # Transit link (auto-populated from gateway if omitted)
  transit:
    network: ""                     # auto-resolved from gateway's transit
    address: 172.16.0.2/24

  # L2 networks this router connects
  networks:
    - name: l2-app-tier
      address: 10.100.0.1/24       # router's IP on this L2 (default gw for VMs)
    - name: l2-db-tier
      address: 10.200.0.1/24

  # Route advertisement: which CIDRs to push to the gateway
  # Gateway auto-creates VPC routes for advertised CIDRs
  routeAdvertisement:
    connectedSegments: true         # advertise all connected L2 CIDRs
    staticRoutes: false             # advertise manually configured static routes
    natIPs: false                   # future: advertise NAT external IPs

  # DHCP server for connected L2 segments (Phase 3 — spec'd now)
  dhcp:
    enabled: false
    # Per-network DHCP pools (future)
    # pools:
    #   - network: l2-app-tier
    #     range: 10.100.0.100-10.100.0.200
    #     gateway: 10.100.0.1         # router's address (auto if omitted)
    #     dns: [10.0.0.10]
    #     leaseTime: 3600

  # Gateway firewall (Phase 2 — spec'd now)
  firewall:
    enabled: false
    rules: []

  pod:
    image: de.icr.io/roks/vpc-router:latest
    resources:
      requests: { cpu: 100m, memory: 128Mi }
      limits:   { cpu: 500m, memory: 256Mi }

status:
  phase: Ready
  transitIP: 172.16.0.2
  podIP: 10.131.0.15              # router pod IP (used as VPC route next-hop)
  networks:
    - name: l2-app-tier
      address: 10.100.0.1
      connected: true
    - name: l2-db-tier
      address: 10.200.0.1
      connected: true
  advertisedRoutes:
    - 10.100.0.0/24
    - 10.200.0.0/24
  conditions:
    - type: TransitConnected
    - type: RoutesConfigured
    - type: PodReady
```

## Reconciler Design

### VPCGateway Reconciler (`pkg/controller/gateway/`)

**Watches**: VPCGateway CRD, VPCRouter status (for route advertisement auto-collection)

**Create/Update**:
1. Validate spec: verify LocalNet CUDN exists with subnet-id, verify zone match
2. Auto-create transit L2 CUDN if `spec.transit.network` is empty: create a `ClusterUserDefinedNetwork` with `topology: Layer2`, CIDR from `spec.transit.cidr`, name `<gateway-name>-transit`
3. Ensure VNI on LocalNet subnet (reuse `ensureVNI` pattern from webhook): `AllowIPSpoofing: true`, `EnableInfrastructureNat: false`, tag `roks-gateway:<cluster-id>-<gateway-name>`
4. Create VPC custom routes: for each `vpcRoutes[].destination`, call `CreateRoute` with `action: deliver`, `nextHop: <VNI reserved IP>`, `zone: spec.zone`. Idempotent by name/tag.
5. Configure NAT rules: generate nftables rules from `spec.nat`, write to ConfigMap, mount in router pod. Priority ordering: no-NAT (lowest number) > DNAT > SNAT.
6. Create/bind Floating IP if enabled
7. Deploy router pod: K8s Deployment with Multus annotations (LocalNet + transit L2), init container for IP forwarding + static routes + nftables, node selector for zone
8. Update status conditions (VNIReady, RoutesConfigured, NATConfigured, PodReady)

**Delete**:
1. Delete VPC routes by stored IDs
2. Delete Floating IP if exists
3. Delete VNI
4. Delete router Deployment + ConfigMaps
5. Delete auto-created transit CUDN (if operator-created)
6. Remove finalizer

### VPCRouter Reconciler (`pkg/controller/router/`)

**Watches**: VPCRouter CRD, VPCGateway changes (NAT, firewall, image, MAC trigger pod recreation)

**Create/Update**:
1. Validate spec: verify referenced gateway exists and is Ready, verify all L2 networks exist
2. Auto-resolve transit: if `spec.transit.network` is empty, read gateway's `status.transitNetwork`
3. Build route ConfigMap: default route via gateway transit IP, connected routes for each L2 CIDR
4. Route advertisement: if `routeAdvertisement.connectedSegments` is true, ensure the gateway's `vpcRoutes` include all connected L2 CIDRs. Update the VPCGateway CR if needed (or use a shared annotation).
5. Deploy router pod: Deployment with Multus annotations (transit + N x L2), init container applies routes from ConfigMap and enables IP forwarding
6. Update status with connected networks and advertised routes

**Delete**:
1. Delete router Deployment
2. Delete ConfigMap
3. Remove advertised routes from gateway (if dynamic)
4. Remove finalizer

### Client Interface Changes

Promote `RoutingTableService` and `RouteService` from `ExtendedClient` to the base `Client` interface. `MockClient` and `InstrumentedClient` already implement these methods.

## Router Pod Design

### Container Image

Minimal Alpine Linux with:
- `iproute2` — IP forwarding, route management
- `nftables` — NAT and firewall rules (Phase 1 NAT, Phase 2 firewall)
- `dnsmasq` — DHCP server (Phase 3, optional sidecar)

Kernel IP forwarding handles all routing. nftables handles NAT in the same pod (not a sidecar, since NAT is core to gateway function).

### Startup Sequence (Init Container)

```bash
# Enable IP forwarding
sysctl -w net.ipv4.ip_forward=1

# Interface IPs configured via Multus (LocalNet gets VNI MAC/IP, L2 gets static IP)
# Add static routes from ConfigMap

# Gateway example:
ip route add 10.100.0.0/24 via 172.16.0.2 dev net2   # L2-A via router
ip route add 10.200.0.0/24 via 172.16.0.2 dev net2   # L2-B via router

# Apply NAT rules from ConfigMap
nft -f /etc/nftables/gateway.conf

# Router example:
ip route add default via 172.16.0.1 dev net1           # default to gateway
# L2 connected routes are implicit from interface addresses
```

### NAT Rule Generation (Gateway)

The reconciler generates nftables config from `spec.nat`:

```nft
table ip nat {
  # No-NAT rules (highest priority — skip NAT for these flows)
  chain prerouting {
    type nat hook prerouting priority -100; policy accept;
    # No-DNAT exemptions evaluated here
  }

  chain postrouting {
    type nat hook postrouting priority 100; policy accept;
    # No-SNAT: skip SNAT for L2 -> VPC subnet traffic
    ip saddr 10.100.0.0/24 ip daddr 10.240.0.0/16 accept
    # SNAT: masquerade L2 egress
    ip saddr 10.100.0.0/24 snat to 10.240.1.5
    ip saddr 10.200.0.0/24 snat to 10.240.1.5
  }

  chain prerouting_dnat {
    type nat hook prerouting priority -50; policy accept;
    # DNAT: external port 443 -> internal 10.100.0.10:8443
    tcp dport 443 dnat to 10.100.0.10:8443
  }
}
```

### Network Functions

All network functions run in the router pod's main container (no sidecars). The init script conditionally enables each function based on CRD spec fields.

| Function | Implementation | Trigger |
|---|---|---|
| Routing (IP forwarding) | `sysctl net.ipv4.ip_forward=1` | Always |
| NAT (SNAT/DNAT) | nftables rules from `NFTABLES_CONFIG` env | Gateway `spec.nat` present |
| Firewall | nftables rules from `FIREWALL_CONFIG` env | Router `spec.firewall.enabled` |
| DHCP | dnsmasq per workload interface | Router `spec.dhcp.enabled` |
| WireGuard | Future | Dedicated CRD field (future) |

## Multi-AZ

- L2 networks (OVN overlay) stretch across zones
- LocalNet (VPC subnet + VLAN attachments) is zone-specific
- One VPCGateway per zone, each with its own VNI, VPC routes, and NAT rules
- VPC routes are zone-scoped: each zone's routing table points L2 CIDRs to that zone's gateway VNI IP
- Transit L2 is auto-created per gateway and stretched across zones
- Routers are zone-agnostic (scheduled by K8s, transit L2 is stretched)
- HA: K8s Deployment with `topologySpreadConstraints` for multi-zone spread

```
Zone eu-de-1                        Zone eu-de-2
─────────────                       ─────────────
VPC Subnet A (10.240.1.0/24)        VPC Subnet B (10.240.2.0/24)
     |                                    |
[VPCGateway: gw-eu-de-1]           [VPCGateway: gw-eu-de-2]
  VNI IP: 10.240.1.5                  VNI IP: 10.240.2.5
  transit IP: 172.16.0.1              transit IP: 172.16.0.3
  SNAT: L2 CIDRs -> 10.240.1.5       SNAT: L2 CIDRs -> 10.240.2.5
     |                                    |
─────+────────── transit-L2 (stretched) ──+──────
                      |
              [VPCRouter: workload-router]
              transit IP: 172.16.0.2
              L2-A: 10.100.0.1, L2-B: 10.200.0.1
              advertises: 10.100.0.0/24, 10.200.0.0/24
```

## VM Integration

### Webhook Enhancement

Current L2 flow: no VPC resources, empty entry in `network-interfaces` annotation.

New flow: webhook checks if the L2 network has an associated VPCRouter. If found, injects the router's address for that network as the default gateway in cloud-init.

`VMNetworkInterface` struct gains a `Gateway` field:
```go
type VMNetworkInterface struct {
    // ... existing fields ...
    Gateway string `json:"gateway,omitempty"`
}
```

When DHCP is enabled on the router, the webhook can skip cloud-init gateway injection for L2 networks served by a router with DHCP — the VM will get its gateway via DHCP instead.

## Console Plugin UI Design

### Navigation

Two new tabs in the VPCNetworkingShell tab bar:

```
Dashboard | Networks | Subnets | VNIs | VLAN Attachments | Floating IPs |
  Gateways | Routers | Security Groups | ACLs | Routes | Topology
```

### Dashboard Integration

New count cards in the dashboard for Gateways and Routers, plus a "Routing" summary section showing the gateway/router topology at a glance.

### Gateways List Page (`/vpc-networking/gateways`)

Subtitle: "Gateways bridge Layer2 overlay networks to the VPC fabric, providing north-south connectivity."

Toolbar: `[+ Create Gateway]` button.

Table columns: Name (link) | Zone | Uplink Network (link) | Floating IP | VNI IP | Routes | NAT Rules | Status (badge) | Age

Empty state: `<EmptyState>` with gateway icon and "No gateways configured" message + Create button.

### Gateway Creation Wizard

`<Modal variant="large">` with `<Wizard>` steps:

1. **Configuration**: Name (TextInput), Zone (FormSelect), Uplink Network (FormSelect, filtered to LocalNet CUDNs)
2. **Transit**: Auto-create toggle (default on), transit CIDR (TextInput, default 172.16.0.0/24), gateway address (auto-computed as .1)
3. **VPC Routes**: Table of L2 CIDRs with "Add Route" button. Each row: Destination CIDR + Remove.
4. **NAT Rules**: Toggle "Enable SNAT for L2 egress" (auto-generates SNAT rules from routes). Optional DNAT rules table. No-NAT exemptions for VPC subnets.
5. **Options**: Floating IP checkbox + existing FIP selector. Pod resources (expandable advanced section).
6. **Review & Create**: DescriptionList summary of all config.

### Gateway Detail Page (`/vpc-networking/gateways/:name`)

Breadcrumb: Gateways > gw-eu-de-1. Action button: `[Delete Gateway]`.

Cards:
1. **Overview** — `DescriptionList isHorizontal`: Name, Zone, Phase (badge), Uplink (link), Transit, VNI ID, VNI IP, Floating IP (link), Created.
2. **VPC Routes** — Table: Destination | Next Hop | Zone | Status. `[+ Add Route]` button.
3. **NAT Rules** — Table: Type (SNAT/DNAT/No-NAT) | Source | Destination | Translated | Priority. `[+ Add Rule]` button.
4. **Conditions** — Grid of condition badges: VNIReady, RoutesConfigured, NATConfigured, PodReady.

### Routers List Page (`/vpc-networking/routers`)

Subtitle: "Routers connect Layer2 networks and uplink to gateways for north-south traffic."

Toolbar: `[+ Create Router]` button.

Table columns: Name (link) | Gateway (link) | Networks | Transit IP | Status (badge) | Age

### Router Creation Wizard

`<Modal variant="large">` with `<Wizard>` steps:

1. **Gateway**: Select the gateway to uplink to (FormSelect, from useGateways()). Transit auto-resolved.
2. **Networks**: Multi-select L2 networks with per-network address assignment. Table: Network Name (FormSelect) | Router IP/CIDR (TextInput) | Remove. `[+ Add Network]`.
3. **Route Advertisement**: Checkboxes: "Advertise connected segments" (default on), "Advertise static routes", "Advertise NAT IPs" (future, disabled).
4. **Review & Create**.

### Router Detail Page (`/vpc-networking/routers/:name`)

Breadcrumb: Routers > workload-router. Action button: `[Delete Router]`.

Cards:
1. **Overview** — `DescriptionList isHorizontal`: Name, Gateway (link), Phase (badge), Transit IP, Pod IP, Created.
2. **Connected Networks** — Table: Network (link) | Router IP | CIDR | Connected (check). `[+ Add Network]` button.
3. **Advertised Routes** — List of CIDRs being advertised to the gateway.
4. **DHCP Configuration** — "Coming Soon" empty state (Phase 3).
5. **Firewall Rules** — "Coming Soon" empty state (Phase 2).
6. **Conditions** — Grid: TransitConnected, RoutesConfigured, PodReady.

### Topology Integration

New node types in the topology viewer:

- **VPCGateway**: Shield/gateway icon, blue color. Shows zone, VNI IP, FIP. Edges: uplink to VPC Subnet node, downlink to transit segment, edges to connected VPCRouters.
- **VPCRouter**: Router icon, purple color. Shows connected network count, pod IP. Edges: uplink to transit segment (and transitively to VPCGateway), downlink edges to each connected L2 network.

Filter checkboxes added for both node types in the topology toolbar.

Node click opens the drawer panel with full detail (type, name, status, interfaces, routes, NAT rules).

### BFF Endpoints

New endpoints following existing patterns:

```
GET    /api/v1/gateways              — list all VPCGateway CRs
GET    /api/v1/gateways/:name        — get single VPCGateway
POST   /api/v1/gateways              — create VPCGateway
DELETE /api/v1/gateways/:name        — delete VPCGateway
GET    /api/v1/routers               — list all VPCRouter CRs
GET    /api/v1/routers/:name         — get single VPCRouter
POST   /api/v1/routers               — create VPCRouter
DELETE /api/v1/routers/:name         — delete VPCRouter
```

## Implementation Phases

### Phase 1: Foundation + Core Routing + NAT
- Define VPCGateway and VPCRouter CRD types in `api/v1alpha1/` (full spec including future fields)
- Promote `RoutingTableService`/`RouteService` to base `Client` interface
- Build router container image (Alpine + iproute2 + nftables)
- VPCGateway reconciler: VNI creation, auto-create transit L2, VPC route lifecycle, NAT config generation, FIP binding, router pod deployment
- VPCRouter reconciler: ConfigMap routes, multi-leg Multus pod, default route to gateway, route advertisement

### Phase 2: Gateway Firewall
- Firewall sidecar container (nftables-controller)
- Firewall rules in CRD spec -> ConfigMap -> sidecar watches and applies
- Stateful rules with connection tracking
- Per-gateway rule sets (gateway and router independent)

### Phase 3: VM Integration + DHCP
- Webhook gateway injection for L2 interfaces via cloud-init
- DHCP (dnsmasq) on router for connected L2 segments
- Per-network DHCP pool configuration
- When DHCP is enabled, webhook skips cloud-init gateway injection

### Phase 4: Console Plugin + BFF
- BFF endpoints for VPCGateway/VPCRouter CRUD
- Gateways list + detail pages
- Routers list + detail pages
- Creation wizards for both
- Dashboard cards
- Topology viewer integration

### Phase 5: Hardening
- Orphan GC for VNIs, floating IPs, PARs, and VPC routes
- Drift detection on VPC routes (route exists? next-hop correct?)
- Multi-AZ: zone-aware scheduling, topology spread constraints
- Status conditions and events on both CRDs
- Prometheus metrics for gateway/router health

## Out of Scope (Future Phases)

- WireGuard tunnelling sidecar (cross-cluster connectivity)
- DNS forwarder (CoreDNS sidecar with conditional forwarding)
- QoS profiles (tc/nftables rate limiting per interface)
- VRRP/keepalived for router HA with virtual IP
- Active-active ECMP routing (multiple gateway instances)
- VRF-lite multi-tenancy (isolated routing tables per tenant)
- OVN DHCP gateway advertisement (alternative to dnsmasq)
- Router auto-scaling based on throughput
- IDS/IPS on gateways
- L7 load balancing on routers
