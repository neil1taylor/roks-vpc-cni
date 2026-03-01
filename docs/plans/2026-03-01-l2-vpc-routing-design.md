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

## Design: NSX T0/T1 Tiered Routing

Two new CRDs model a tiered routing system inspired by NSX-T:

- **VPCGateway (T0)**: Per-zone gateway that bridges a LocalNet (VPC fabric) and a transit L2 network. Has a VPC identity (VNI, reserved IP), manages VPC custom routes for return traffic, and optionally binds a floating IP for internet ingress.
- **VPCRouter (T1)**: Workload router that connects multiple L2 networks and uplinks to a T0 via the transit L2. Handles east-west routing between L2 segments and forwards north-south traffic to T0.

A dedicated "transit" L2 network connects T0 and T1, following the NSX transit segment pattern.

## Architecture

```
                    Internet / VPC Services
                           |
                    VPC Routing Table
                    (custom routes: L2 CIDRs -> T0 VNI IP)
                           |
              +------------+------------+
              |                         |
     [VPCGateway: gw-zone-1]  [VPCGateway: gw-zone-2]
     Pod w/ 2 interfaces:      Pod w/ 2 interfaces:
       net1: LocalNet (VNI)      net1: LocalNet (VNI)
       net2: transit-L2          net2: transit-L2
              |                         |
    ----------+-------------------------+----------
              |     transit-L2 (stretched OVN)
              |
     [VPCRouter: workload-router]
     Pod w/ N+1 interfaces:
       net1: transit-L2 (uplink to T0)
       net2: l2-app-tier
       net3: l2-db-tier
              |         |
           L2-App    L2-DB
           (VMs)     (VMs)
```

### Traffic Flows

| Flow | Path |
|------|------|
| L2 VM -> Internet | VM -> T1 (default gw) -> transit L2 -> T0 -> LocalNet VNI -> VPC fabric -> Internet |
| Internet -> L2 VM | Internet -> FIP on T0 VNI -> T0 -> DNAT -> transit L2 -> T1 -> L2 -> VM |
| L2-A VM -> L2-B VM | VM-A -> T1 (routes between L2 legs) -> VM-B (stays in OVN) |
| L2 VM -> VPC subnet VM | VM -> T1 -> transit -> T0 -> LocalNet -> VPC route -> VPC subnet VM |
| VPC return to L2 | VPC route (L2 CIDR -> T0 VNI next-hop) -> T0 -> transit -> T1 -> L2 -> VM |

## CRD Specifications

### VPCGateway (T0)

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

  # Downlink: transit L2 network connecting to T1 routers
  transit:
    network: transit-internal       # L2 CUDN name
    address: 172.16.0.1/24          # gateway IP on transit segment

  # VPC routes the operator should create (L2 CIDRs -> this gateway's VNI)
  vpcRoutes:
    - destination: 10.100.0.0/24    # L2-A CIDR
    - destination: 10.200.0.0/24    # L2-B CIDR

  # Optional: bind a floating IP for internet ingress
  floatingIP:
    enabled: true

  # Router pod configuration
  pod:
    image: de.icr.io/roks/vpc-router:latest
    resources:
      requests: { cpu: 100m, memory: 128Mi }
      limits:   { cpu: 500m, memory: 256Mi }
    replicas: 1

status:
  phase: Ready                # Pending | Provisioning | Ready | Error
  vniID: "02r7-..."
  reservedIP: 10.240.1.5
  floatingIP: 169.48.x.x
  vpcRouteIDs:
    - r010-route-1
    - r010-route-2
  conditions:
    - type: VNIReady
    - type: RoutesConfigured
    - type: PodReady
```

### VPCRouter (T1)

```yaml
apiVersion: vpc.roks.ibm.com/v1alpha1
kind: VPCRouter
metadata:
  name: workload-router
spec:
  # Reference to the T0 gateway for north-south traffic
  gateway: gw-eu-de-1

  # Transit link (must match the gateway's transit network)
  transit:
    network: transit-internal
    address: 172.16.0.2/24

  # L2 networks this router connects
  networks:
    - name: l2-app-tier
      address: 10.100.0.1/24       # router's IP on this L2 (default gw for VMs)
    - name: l2-db-tier
      address: 10.200.0.1/24

  # Pluggable functions
  functions:
    - type: routing                 # always required
    # Future:
    # - type: firewall
    #   config:
    #     rules: firewall-rules-cm
    # - type: wireguard
    #   config:
    #     peers: wg-peers-cm

  pod:
    image: de.icr.io/roks/vpc-router:latest
    resources:
      requests: { cpu: 100m, memory: 128Mi }
      limits:   { cpu: 500m, memory: 256Mi }
    replicas: 1

status:
  phase: Ready
  transitIP: 172.16.0.2
  networks:
    - name: l2-app-tier
      address: 10.100.0.1
      connected: true
    - name: l2-db-tier
      address: 10.200.0.1
      connected: true
  conditions:
    - type: TransitConnected
    - type: RoutesConfigured
    - type: PodReady
```

## Reconciler Design

### VPCGateway Reconciler (`pkg/controller/gateway/`)

**Watches**: VPCGateway CRD

**Create/Update**:
1. Validate spec: verify LocalNet CUDN exists with subnet-id, verify transit L2 CUDN exists, verify zone match
2. Ensure VNI on LocalNet subnet (reuse `ensureVNI` pattern from webhook): `AllowIPSpoofing: true`, `EnableInfrastructureNat: false`, tag `roks-gateway:<cluster-id>-<gateway-name>`
3. Create VPC custom routes: for each `vpcRoutes[].destination`, call `CreateRoute` with `action: deliver`, `nextHop: <VNI reserved IP>`, `zone: spec.zone`. Idempotent by name/tag.
4. Create/bind Floating IP if enabled
5. Deploy router pod: K8s Deployment with Multus annotations (LocalNet + transit L2), init container for IP forwarding + static routes, node selector for zone
6. Update status conditions

**Delete**:
1. Delete VPC routes by stored IDs
2. Delete Floating IP if exists
3. Delete VNI
4. Delete router Deployment
5. Remove finalizer

### VPCRouter Reconciler (`pkg/controller/router/`)

**Watches**: VPCRouter CRD

**Create/Update**:
1. Validate spec: verify referenced gateway exists and is Ready, verify all L2 networks exist, verify transit network matches gateway
2. Build route ConfigMap: default route via T0 transit IP, connected routes for each L2 CIDR
3. Deploy router pod: Deployment with Multus annotations (transit + N x L2), init container applies routes from ConfigMap and enables IP forwarding
4. Update status

**Delete**:
1. Delete router Deployment
2. Delete ConfigMap
3. Remove finalizer

### Client Interface Changes

Promote `RoutingTableService` and `RouteService` from `ExtendedClient` to the base `Client` interface. `MockClient` and `InstrumentedClient` already implement these methods.

## Router Pod Design

### Container Image

Minimal Alpine Linux with iproute2. Kernel IP forwarding handles all routing — no userspace daemon needed for MVP.

### Startup Sequence (Init Container)

```bash
# Enable IP forwarding
sysctl -w net.ipv4.ip_forward=1

# Interface IPs configured via Multus (LocalNet gets VNI MAC/IP, L2 gets static IP)
# Add static routes from ConfigMap

# T0 example:
ip route add 10.100.0.0/24 via 172.16.0.2 dev net2   # L2-A via T1
ip route add 10.200.0.0/24 via 172.16.0.2 dev net2   # L2-B via T1

# T1 example:
ip route add default via 172.16.0.1 dev net1           # default to T0
# L2 connected routes are implicit from interface addresses
```

### Pluggable Functions (Future)

Each function is a sidecar container injected by the reconciler based on `spec.functions`:
- **firewall**: nftables rules from ConfigMap, watches for updates
- **wireguard**: WireGuard interface + peer management for cross-cluster tunnelling

## Multi-AZ

- L2 networks (OVN overlay) stretch across zones
- LocalNet (VPC subnet + VLAN attachments) is zone-specific
- One VPCGateway per zone, each with its own VNI and VPC routes
- VPC routes are zone-scoped: each zone's routing table points L2 CIDRs to that zone's T0 VNI IP
- T1 routers are zone-agnostic (scheduled by K8s, transit L2 is stretched)
- HA: K8s Deployment with `topologySpreadConstraints` for multi-zone spread

```
Zone eu-de-1                        Zone eu-de-2
─────────────                       ─────────────
VPC Subnet A (10.240.1.0/24)        VPC Subnet B (10.240.2.0/24)
     |                                    |
[VPCGateway: gw-eu-de-1]           [VPCGateway: gw-eu-de-2]
  VNI IP: 10.240.1.5                  VNI IP: 10.240.2.5
  transit IP: 172.16.0.1              transit IP: 172.16.0.3
     |                                    |
─────+────────── transit-L2 (stretched) ──+──────
                      |
              [VPCRouter: workload-router]
              transit IP: 172.16.0.2
              L2-A: 10.100.0.1, L2-B: 10.200.0.1
```

## VM Integration

### Webhook Enhancement

Current L2 flow: no VPC resources, empty entry in `network-interfaces` annotation.

New flow: webhook checks if the L2 network has an associated VPCRouter. If found, injects the T1's address for that network as the default gateway in cloud-init.

`VMNetworkInterface` struct gains a `Gateway` field:
```go
type VMNetworkInterface struct {
    // ... existing fields ...
    Gateway string `json:"gateway,omitempty"`
}
```

## Implementation Phases

1. **Foundation**: CRD types, promote route APIs to Client, scaffold reconcilers, build router image
2. **VPCGateway (T0)**: VNI creation, VPC route lifecycle, FIP binding, router pod deployment
3. **VPCRouter (T1)**: ConfigMap routes, multi-leg Multus pod, default route to T0
4. **VM Integration**: Webhook gateway injection for L2 interfaces, cloud-init enhancement
5. **Console Plugin + BFF**: Gateway/Router list and detail pages, topology integration
6. **Hardening**: Orphan GC for gateway VNIs/routes, drift detection, multi-AZ scheduling, events

## Out of Scope (Future)

- Firewall sidecar (nftables rules via CRD/ConfigMap)
- WireGuard tunnelling sidecar (cross-cluster connectivity)
- VRRP/keepalived for T1 HA with virtual IP
- OVN DHCP gateway advertisement
- Active-active ECMP routing
- T1 auto-scaling based on throughput
