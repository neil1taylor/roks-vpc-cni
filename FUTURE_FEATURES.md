# Future Features

Ideas and enhancements under consideration for the ROKS VPC Network Operator.

## SDN / Overlay Mesh Networking

**Status**: Under consideration

Integrate WireGuard-based mesh networking for use cases beyond single-VPC connectivity.

### Candidates

- **NetBird** — Zero-trust, identity-first mesh with built-in NAT traversal (ICE/STUN/TURN). Best fit for developer/operator access to VMs without requiring floating IPs.
- **Netmaker** — Traditional network-admin model (CIDR ranges, ACLs). Best fit for multi-cluster/multi-VPC private connectivity as an alternative to IBM Transit Gateway.

### Use Cases

- **Developer access**: Reach VMs directly without floating IPs (NetBird)
- **Multi-cluster/multi-VPC**: Private mesh across regions without Transit Gateway (Netmaker)
- **Hybrid connectivity**: Connect on-prem workloads to VPC-hosted KubeVirt VMs
- **Tenant isolation**: Cryptographic separation for multi-tenant VMs sharing a VPC

### Notes

- Would sit alongside existing VPC networking, not replace it
- Not needed for single-VPC, single-cluster deployments where VPCGateway/VPCRouter already handles routing
- Adds a fourth networking layer (VPC L3 > OVN LocalNet L2 > VPCRouter > WireGuard) — debuggability impact should be considered

## Router Pod Performance Improvements

**Status**: Tier 1 implemented, Tier 2-3 under consideration

The VPCRouter pod uses a Fedora container running a bash init script with kernel nftables forwarding over Multus veth interfaces. Tier 1 (resource limits, CPU pinning, node scheduling) is implemented via `spec.pod`. Higher tiers target throughput beyond ~15 Gbps.

### Current Architecture

Traffic path: **VM → OVN overlay → Multus veth → router pod (nftables) → uplink veth → OVN overlay → VPC fabric**

| Factor | Current State | Impact |
|--------|--------------|--------|
| **Data path** | Veth pairs (Multus CNI) | ~5-10% overhead vs native bridge |
| **Forwarding engine** | Kernel nftables | Single-threaded per flow, no fast-path |
| **Resource limits** | Configurable via `spec.pod.resources` | ✅ Guaranteed CPU/memory |
| **CPU affinity** | Configurable via `spec.pod.runtimeClassName` | ✅ CPU Manager static policy |
| **Node scheduling** | Configurable via `spec.pod.nodeSelector` + `tolerations` | ✅ Target specific nodes |
| **NIC mode** | OVN LocalNet (software) | No SR-IOV passthrough |
| **Container image** | `fedora:40` + bash init | Not a purpose-built forwarding binary |
| **Acceleration** | None | No DPDK, XDP, or eBPF fast-path |

### Remaining Improvements

**Tier 2 — Medium effort, significant gains**

- **Purpose-built router image** — Replace `fedora:40` + bash script with a compiled Go or C binary that configures interfaces and applies nftables at startup. Eliminates `dnf install` overhead (~30s startup), reduces image size from ~1GB to ~50MB, and enables structured health reporting.
- **XDP/eBPF fast-path** — Attach XDP programs to Multus interfaces for forwarding decisions before packets enter the kernel network stack. 10-100x PPS improvement for simple forwarding rules. Complex NAT still falls back to nftables.
- **Host networking mode** — Run the router pod with `hostNetwork: true` and use VRF or network namespaces for isolation. Eliminates veth overhead entirely. Tradeoff: loses Multus interface abstraction.

**Tier 3 — High effort, maximum performance**

- **SR-IOV interfaces** — Use SR-IOV VFs for the uplink interface, bypassing OVN entirely on the VPC side. Requires SR-IOV device plugin and Network Operator configuration. Near line-rate performance.
- **DPDK userspace forwarding** — Run VPP (fd.io) or a custom DPDK application inside the router pod for full userspace packet processing. Requires hugepages, dedicated cores, and SR-IOV VFs. Achieves 40-100 Gbps on modern NICs.
- **FRRouting replacement** — Replace the bash init script with FRRouting (FRR) for dynamic routing protocol support (BGP, OSPF) alongside high-performance forwarding via its built-in dataplane.

### When to Invest

For most KubeVirt VM workloads (tens of VMs, moderate traffic), Tier 1 is sufficient. Higher tiers become important for:

- **NFV workloads** — Network functions requiring line-rate packet processing
- **Storage replication** — iSCSI/NFS between VMs across subnets
- **High VM density** — 100+ VMs routing through a single router pod
- **Latency-sensitive** — Financial, gaming, or real-time communication workloads

### Implementation Notes

- XDP programs can coexist with nftables — use XDP for fast-path forwarding, nftables for complex NAT/firewall
- SR-IOV requires the OpenShift SR-IOV Network Operator to be installed on the cluster
- DPDK requires hugepages configuration via MachineConfig and node tuning
- A purpose-built image could be maintained as `de.icr.io/roks/vpc-network-router:latest` alongside the operator/BFF/plugin images

---

## DHCP Enhancements

**Status**: Core features and advanced features implemented; IPv6 under consideration

Per-network DHCP configuration, custom ranges, static reservations, DNS settings, DHCP options (router, MTU, NTP, custom), and configurable lease times are all implemented. Lease persistence and auto-reservations are also implemented.

### Implemented Enhancements

**Lease persistence** (2026-03-04)
- PVC per router pod for DHCP lease file persistence across restarts
- `spec.dhcp.leasePersistence` with configurable size and StorageClass
- `status.leasePersistenceReady` condition tracks PVC binding
- Implementation: `pkg/controller/router/pvc.go`

**Auto-populated reservations** (2026-03-04)
- Opt-in `spec.dhcp.autoReservations: true` on VPCRouter
- Router reconciler discovers VMs via `vpc.roks.ibm.com/network-interfaces` annotation
- Auto-merges MAC→IP reservations into dnsmasq config (manual wins on collision)
- `status.networks[].dhcp.autoReservationCount` reports discovered count
- Implementation: `pkg/controller/router/auto_reservations.go`

### Remaining Enhancements

**IPv6 support**
- Router Advertisement (RA) for SLAAC
- DHCPv6 for stateful IPv6 assignment
- Dual-stack configuration per network

## DNS Filtering and Secure DNS

**Status**: Implemented (2026-03-04) — VPCDNSPolicy CRD + AdGuard Home sidecar

> **Implementation complete**: `VPCDNSPolicy` CRD (`vdp`), reconciler, AdGuard Home sidecar injection into router pods, BFF endpoints, and console plugin UI (list/detail/create pages + dashboard card). Option A (sidecar) was chosen. dnsmasq forwards all DNS to `127.0.0.1#5353` (AdGuard Home). Supports DoH/DoT upstream, blocklists, allow/deny lists, and local DNS resolution. Configuration is declarative via the CRD spec, with AdGuard Home YAML auto-generated and delivered via ConfigMap.

Implementation files:
- CRD types: `api/v1alpha1/vpcdnspolicy_types.go`
- Reconciler: `pkg/controller/dnspolicy/reconciler.go`
- Config generator: `pkg/controller/dnspolicy/adguard_config.go`
- Sidecar builder: `pkg/controller/router/adguard_sidecar.go`
- Helm CRD: `deploy/helm/.../templates/crds/vpcdnspolicy-crd.yaml`
- BFF handler: `cmd/bff/internal/handler/dnspolicy_handler.go`
- Console pages: `DNSPoliciesListPage`, `DNSPolicyDetailPage`, `DNSPolicyCreatePage`

### Remaining Enhancements

- **Option C migration**: Replace dnsmasq with AdGuard Home's built-in DHCP server for unified DNS+DHCP
- **Per-network policies**: Different VPCDNSPolicy per router network (currently one policy per router)
- **Query logging and analytics**: Expose AdGuard Home query metrics to Prometheus

## VPN Gateway

**Status**: Implemented (2026-03-04) — See `docs/tutorials/vpn-gateway.md`

> **Implementation complete**: VPCVPNGateway CRD (`vvg`), reconciler, BFF endpoints, and console plugin UI (list/detail/create pages + dashboard card). Supports WireGuard, IPsec/StrongSwan, and OpenVPN protocols. Both site-to-site and client-to-site (remote access). References VPCGateway for FIP tunnel endpoint. Advertised routes auto-collected by VPCGateway for VPC route creation. OpenVPN includes a status-exporter sidecar for runtime client stats, client certificate lifecycle (issue/revoke with CRL), and optional DH/TLS-Auth secret support. Console plugin includes remote access configuration card, DH and TLS-Auth fields on the create form, and connected client stats on the detail page.

Provide encrypted site-to-site and client-to-site VPN tunnels for VM networks, managed declaratively through CRDs. Distinct from the mesh networking section — this is traditional hub-and-spoke VPN, not peer-to-peer mesh.

### Use Cases

- **Site-to-site**: Connect VM workload networks to on-prem data centers or branch offices over IPsec/WireGuard tunnels
- **Multi-cloud**: Encrypted tunnels to AWS VPN Gateway, Azure VPN Gateway, GCP Cloud VPN
- **Cross-VPC**: Private connectivity between VPCs without IBM Transit Gateway
- **Client-to-site (remote access)**: Developers/admins connect laptops directly into VM networks
- **Regulated workloads**: Encrypted transit required by compliance (PCI-DSS, HIPAA) even within cloud

### Approaches

**Option A — WireGuard in the router pod**
- Add WireGuard as an additional function of VPCRouter (alongside SNAT, DHCP, firewall)
- Configure via `spec.vpn` on VPCRouter or VPCGateway
- Pros: reuses existing pod lifecycle, no new components, lightweight (~3-5% overhead)
- Cons: couples VPN to the router pod lifecycle, router restart = tunnel flap

**Option B — Dedicated VPN pod (new `VPCVPNGateway` CRD)**
- Separate pod with WireGuard or StrongSwan, managed by a new reconciler
- Gets its own VNI + floating IP via VPCGateway, independent of the router
- Pros: independent lifecycle, can have HA (active/standby), isolated failure domain
- Cons: additional CRD, more VPC resources consumed

**Option C — IBM Cloud VPN for VPC (managed service)**
- Orchestrate the IBM Cloud VPN Gateway API to create managed IPsec tunnels
- Operator creates/configures VPN gateway, connections, and IKE/IPsec policies via VPC API
- Pros: no pods to manage, IBM-managed HA, integrates with VPC routing natively
- Cons: operates at the VPC subnet level (not per-VM), less flexible, additional cost

### Protocol Support

| Protocol | Site-to-site | Client-to-site | Enterprise compat | Performance |
|----------|-------------|----------------|-------------------|-------------|
| **WireGuard** | Yes | Yes (wg-quick) | Limited (newer firewalls only) | Excellent | ✅ Implemented |
| **IPsec/IKEv2** (StrongSwan) | Yes | Yes | Excellent (Cisco, Fortinet, Palo Alto) | Good | ✅ Implemented |
| **OpenVPN** | Yes | Yes (broad client support) | Moderate | Moderate | ✅ Implemented |

### CRD Sketch

```yaml
apiVersion: vpc.roks.ibm.com/v1alpha1
kind: VPCVPNGateway
metadata:
  name: site-to-onprem
spec:
  gatewayRef:
    name: my-gateway
  protocol: wireguard  # wireguard | ipsec | openvpn

  # Site-to-site tunnels
  tunnels:
    - name: onprem-dc1
      remoteEndpoint: 203.0.113.10
      remoteNetworks:
        - 10.0.0.0/8
        - 192.168.0.0/16
      presharedKey:
        secretRef:
          name: vpn-psk-dc1
          key: psk
      # IPsec-specific
      ikePolicy:
        version: 2
        encryption: aes256
        hash: sha256
        dhGroup: 14
      ipsecPolicy:
        encryption: aes256
        hash: sha256
        pfs: group14

    - name: aws-vpc
      remoteEndpoint: 52.1.2.3
      remoteNetworks:
        - 172.31.0.0/16
      presharedKey:
        secretRef:
          name: vpn-psk-aws
          key: psk

  # Client-to-site (remote access)
  remoteAccess:
    enabled: false
    addressPool: 10.99.0.0/24
    allowedUsers:
      - dev-team
    dns:
      servers:
        - 10.100.0.1

  # Networks to advertise to remote peers
  localNetworks:
    - networkRef:
        name: workload-net-1
    - cidr: 10.100.0.0/24  # explicit CIDR override

status:
  tunnels:
    - name: onprem-dc1
      status: Established
      remoteEndpoint: 203.0.113.10
      lastHandshake: "2026-03-01T10:30:00Z"
      txBytes: 1073741824
      rxBytes: 536870912
    - name: aws-vpc
      status: Connecting
      remoteEndpoint: 52.1.2.3
```

### Integration with Existing Components

- **VPCGateway**: VPN pod gets a floating IP via the same PAR/FIP mechanism as the router
- **VPCRouter**: Route advertisement — VPN gateway advertises remote networks, router picks them up and routes VM traffic into the tunnel
- **Firewall**: nftables rules on the router can filter VPN-bound traffic
- **Console plugin**: Tunnel status dashboard, connection health, bandwidth metrics
- **Orphan GC**: Clean up VPN-related VPC resources (FIPs, routes) on CRD deletion

### Implementation Notes

- WireGuard is the best default — lowest overhead, simplest config, kernel-level performance
- IPsec (StrongSwan) needed for enterprise interop — many corporate firewalls don't support WireGuard yet
- Could support both protocols simultaneously (WireGuard for cloud-to-cloud, IPsec for on-prem)
- Key management via K8s Secrets (PSKs) or cert-manager integration (X.509 for IKEv2)
- Tunnel health monitoring should emit Prometheus metrics and K8s events
- HA: active/standby with VRRP or keepalived for site-to-site tunnels that can't tolerate flaps
- Route injection: VPN gateway should advertise remote networks into VPCRouter's route table automatically via the existing bidirectional watch pattern

## Traffic Analysis and Deep Packet Inspection

**Status**: Under consideration

Provide real-time network traffic visibility, flow analysis, and deep packet inspection for VM workload networks using ntopng.

### Why ntopng

ntopng is the leading open-source network traffic analysis tool. It combines:
- **Deep packet inspection (nDPI)** — application-layer protocol detection (HTTP, DNS, TLS, SSH, RDP, databases, etc.) without decryption
- **Real-time flow visualization** — top talkers, traffic matrices, geo-mapping
- **Historical analysis** — time-series traffic data with drill-down
- **Alerting** — anomaly detection, threshold-based alerts, flow-based triggers
- **REST API** — programmatic access to all data (fits CRD-driven model)

Alternatives like Zeek focus on security/IDS rather than operational visibility. Note: Suricata IDS/IPS is already implemented as a VPCRouter sidecar (`spec.ids`) — this section is about traffic *analytics* and *visibility*, which is complementary. ntopng covers both traffic engineering and security use cases.

### Use Cases

- **Capacity planning**: Identify bandwidth-heavy VMs, forecast subnet utilization
- **Troubleshooting**: Pinpoint which VM is saturating a network, trace flow paths through the router
- **Security monitoring**: Detect lateral movement, port scans, DNS exfiltration, C2 beaconing
- **Application discovery**: Identify what protocols/services VMs are running without agent installation
- **Compliance auditing**: Record network flows for regulatory requirements (who talked to whom, when)
- **Cost attribution**: Per-VM/per-network bandwidth accounting for chargeback

### Architecture

```
                          ┌─────────────────────────┐
                          │     Console Plugin       │
                          │  (traffic dashboards)    │
                          └────────┬────────────────┘
                                   │ REST API
                          ┌────────▼────────────────┐
                          │       ntopng pod         │
                          │  ┌───────┐ ┌──────────┐  │
                          │  │ntopng │ │ ClickHouse│  │
                          │  │(nDPI) │ │ (storage) │  │
                          │  └───┬───┘ └──────────┘  │
                          └──────┼───────────────────┘
                                 │ mirror/tap
        ┌────────────────────────┼────────────────────────┐
        │              VPCRouter pod                      │
        │  ┌────────┐  ┌────────┴───┐  ┌──────────────┐  │
        │  │ SNAT   │  │ tc mirror  │  │   dnsmasq    │  │
        │  │nftables│  │ or nflog   │  │   (DHCP)     │  │
        │  └────────┘  └────────────┘  └──────────────┘  │
        │      net0 ◄──── net1 ◄──── net2                 │
        └───────┼──────────┼──────────┼───────────────────┘
                │          │          │
           ┌────▼───┐ ┌───▼────┐ ┌───▼────┐
           │uplink  │ │ work-1 │ │ work-2 │
           │subnet  │ │ subnet │ │ subnet │
           └────────┘ └────────┘ └────────┘
```

### Traffic Capture Methods

**Option A — tc mirror on the router pod**
- Mirror all router interfaces to a GRE/VXLAN tunnel or Unix socket consumed by ntopng
- Captures all transit traffic (VM ↔ internet, VM ↔ VM across subnets)
- Pros: sees everything that passes through the router, no VM changes
- Cons: only sees routed traffic, not intra-subnet VM-to-VM

**Option B — nflog + nftables on the router pod**
- Add nftables `log group` rules to capture packet metadata (not payloads)
- ntopng reads from nflog
- Pros: lightweight, metadata-only, integrates with existing firewall rules
- Cons: no DPI on payload, limited application-layer visibility

**Option C — Dedicated tap/mirror pod per subnet**
- Promiscuous-mode pod on each workload network captures all L2 traffic
- Forwards to ntopng via sFlow/NetFlow/IPFIX
- Pros: sees all traffic including intra-subnet VM-to-VM
- Cons: additional pods per network, higher resource overhead

**Recommended**: Option A (tc mirror) for transit traffic + Option C selectively for subnets that need full visibility.

### Integration with Existing Components

- **VPCRouter**: tc mirror or nflog rules added to `buildInitScript()` when traffic analysis is enabled
- **VPCGateway**: New `spec.trafficAnalysis` section to enable/configure per gateway
- **Console plugin**: Embed ntopng dashboards via iframe or build native PatternFly views consuming ntopng's REST API
- **Prometheus**: ntopng exports metrics — connect to existing operator monitoring stack
- **DNS filtering**: Correlate ntopng flow data with AdGuard/Pi-hole DNS logs for full request tracing

### CRD Sketch

```yaml
apiVersion: vpc.roks.ibm.com/v1alpha1
kind: VPCTrafficAnalyzer
metadata:
  name: production-monitor
spec:
  gatewayRef:
    name: my-gateway

  capture:
    mode: mirror        # mirror | nflog | sflow
    interfaces: all     # all | [net0, net1]
    # Filter to reduce capture volume
    bpfFilter: "not port 22"

  storage:
    backend: clickhouse  # clickhouse | sqlite | timescaledb
    retention: 30d
    volumeSize: 50Gi

  analysis:
    dpi: true            # Enable nDPI deep packet inspection
    geoIP: true          # GeoIP lookups for external IPs
    alerting:
      enabled: true
      anomalyDetection: true
      thresholds:
        - metric: throughput
          interface: net0
          above: 1Gbps
          action: event    # event | webhook

  access:
    # Expose ntopng web UI via OpenShift Route
    exposeUI: true
    authProxy: openshift  # Use OpenShift OAuth proxy
```

### Implementation Notes

- ntopng Community Edition is GPLv3 — fine for internal use; Enterprise (commercial) adds encrypted traffic analysis, SNMP, extended retention
- ClickHouse is the recommended storage backend for production — handles high-volume flow data efficiently
- nDPI identifies 300+ protocols and applications without decryption — identifies app-layer traffic by pattern matching
- Router pod needs `NET_ADMIN` capability for tc mirror (already has it for nftables)
- ntopng pod resource requirements: ~1 CPU, ~2GB RAM for moderate traffic volumes; scales with flow rate
- sFlow/IPFIX export from ntopng can feed into external SIEM tools (Splunk, Elastic)
- Consider making traffic analysis opt-in per network to control overhead

## Network Observability Platform

**Status**: Phases 1-2 implemented, Phases 3-4 under consideration

Build a comprehensive network monitoring, analytics, and alerting stack comparable to VMware NSX's built-in observability — traceflow, flow monitoring, micro-segmentation analytics, health dashboards, and latency analysis.

### NSX Feature Mapping

| NSX Feature | Current State | Proposed Solution |
|-------------|--------------|-------------------|
| **Traceflow** (synthetic packet path tracing) | None | Custom CRD + eBPF or `traceroute`/`nping` in router pod |
| **Flow Monitoring** (top talkers, app stats) | None (see Traffic Analysis section) | VPC Flow Logs + ntopng + Prometheus |
| **Live Traffic Analysis** (per-interface stats) | **Implemented** (metrics exporter sidecar) | `/proc/net/dev` polling in router pod sidecar → Prometheus |
| **Micro-segmentation Analytics** (rule hits, unused rules) | **Implemented** (nftables `counter` keyword) | nftables counters → metrics exporter → Prometheus |
| **Network Topology** (interactive, status overlays) | **Implemented** (health overlays, auto-refresh, edge throughput) | TopologyViewer with NodeStatus mapping, Thanos health queries, 30s polling |
| **Latency Monitoring** (per-hop latency) | None | Synthetic probes between router pods (Goldpinger-style) |
| **Health Dashboard** (component health, alarms) | **Implemented** (Observability page + RouterHealthCard) | Unified health view with SLA tracking |
| **Port Mirroring** | None (see Traffic Analysis section) | tc mirror in router pod |
| **Correlation Engine** (cross-component event correlation) | None | Event aggregation service in BFF |
| **IPFIX/sFlow Export** | None | Router pod sFlow agent → collector |
| **Alerting** (threshold + anomaly) | **Implemented** (alert timeline on dashboard) | K8s Warning events + Prometheus alerts aggregated via BFF, AlertTimelineCard |

### Component 1: Enhanced Prometheus Metrics

Extend the existing 5 metrics in `pkg/metrics/metrics.go` with network-level telemetry:

**Router pod metrics** (exported via sidecar or node exporter textfile)
- Per-interface packet/byte counters (rx/tx)
- Per-interface error/drop counters
- nftables rule hit counters (firewall + NAT)
- Connection tracking table size
- DHCP lease count and pool utilization
- VPN tunnel status and throughput (if VPN feature enabled)

**Operator-level metrics**
- VPC resource sync latency (time from K8s change to VPC API completion)
- Resource drift detection count
- CRD status condition transitions
- Webhook admission latency (p50/p95/p99)

**VPC Flow Logs integration** (Phase 2 — CRD scaffolding implemented 2026-03-04)
- IBM Cloud VPC Flow Logs capture all traffic on VPC subnets natively
- `VPCSubnet` CRD now has `spec.flowLogs` (enabled, cosBucketCRN, interval) and `status.flowLogCollectorID`
- VPCSubnet reconciler calls `reconcileFlowLogs()` for lifecycle management
- VPC client methods are stubbed (`flow_logs.go`) — awaiting VPC SDK wiring
- Provides: source/dest IP, port, protocol, action (accept/reject), bytes, packets

### Component 2: Traceflow

NSX's signature feature — inject a synthetic packet and trace its path through the network. Build an equivalent:

```yaml
apiVersion: vpc.roks.ibm.com/v1alpha1
kind: VPCTraceflow
metadata:
  name: debug-vm1-to-internet
spec:
  source:
    vmRef:
      name: test-vm-1
      namespace: default
    # or direct IP
    # ip: 10.100.0.15
  destination:
    ip: 8.8.8.8
    port: 443
    protocol: TCP
  timeout: 30s

status:
  phase: Completed  # Pending | Running | Completed | Failed
  hops:
    - node: vm-interface
      component: VNI (roks-d62e-default-test-vm-1)
      action: Forward
      latency: 0ms
    - node: ovn-localnet
      component: localnet-1 (VLAN 100)
      action: Forward
      latency: 1ms
    - node: router-pod
      component: VPCRouter/my-router (net0)
      action: SNAT → 169.48.x.x
      nftablesRule: "snat to 169.48.x.x"
      latency: 2ms
    - node: vpc-uplink
      component: VPCGateway/my-gateway (FIP)
      action: Forward
      latency: 3ms
    - node: destination
      component: 8.8.8.8:443
      action: Received (SYN-ACK)
      latency: 15ms
  result: Reachable
  totalLatency: 15ms
```

**Implementation approaches:**
- **eBPF-based**: Attach BPF programs at each hop (VM VNI, OVN bridge, router pod interfaces) to trace packet flow — most accurate but requires kernel support and privileged access
- **Active probing**: Run `traceroute`/`nping`/`curl` from the router pod toward the destination, cross-reference with nftables counters — simpler but less granular
- **Hybrid**: Use nftables `log` with unique marks at each stage, parse logs to reconstruct the path

### Component 3: Micro-segmentation Analytics

Track nftables firewall rule effectiveness on router pods:

- **Rule hit counters**: Every nftables rule gets a counter; export to Prometheus
- **Unused rule detection**: Rules with zero hits over configurable window → alert
- **Blocked flow log**: Log denied packets with source/dest/port/protocol → surface in console plugin
- **Security group correlation**: Cross-reference VPC security group rules with nftables rules to show the effective policy

```
# Example: nftables with counters (generated in buildInitScript)
nft add rule inet filter forward \
  iifname "net0" ip daddr 10.200.0.0/16 counter accept \
  comment "allow-workload-to-workload"
```

### Component 4: Latency Monitoring

Continuous synthetic probing between network components:

- **VM-to-router latency**: Router pod pings each VM's VNI IP periodically
- **Router-to-gateway latency**: Uplink interface RTT measurement
- **Gateway-to-internet latency**: FIP egress RTT to well-known targets
- **Cross-subnet latency**: Between router pods on different workload networks
- **Latency heatmap**: Matrix view in console plugin (similar to NSX's latency dashboard)

**Tool options:**
- **Goldpinger-style mesh probing** — lightweight Go binary that probes peers and exports Prometheus metrics. Could run as a sidecar in each router pod.
- **Smokeping/mtr** — traditional latency monitoring with loss detection
- **eBPF TCP RTT** — kernel-level per-connection RTT tracking without synthetic probes

### Component 5: Unified Health Dashboard

Extend the existing VPC Dashboard page with NSX-style operational views:

**System health panel**
- Operator pod status (already exists) + webhook health + GC status
- VPC API health (success rate, latency trends from existing `vpc_api_latency_seconds`)
- Per-reconciler health (error rate from `reconcile_operations_total`)

**Network health panel**
- Per-subnet: VM count, IP utilization, bandwidth, error rate
- Per-router: interface status, NAT connection count, DHCP pool usage
- Per-gateway: FIP status, VPC route count, PAR allocation

**Traffic flow overlay on topology**
- Animate traffic flows on the existing TopologyViewer
- Line thickness = bandwidth, color = health (green/yellow/red)
- Click a flow to see protocol breakdown (from ntopng DPI data)

**Alert timeline**
- Chronological event stream combining K8s events, Prometheus alerts, VPC API errors
- Filter by network, VM, component
- Correlation hints (e.g., "VPC API latency spike → 3 reconcile failures → 2 VMs lost connectivity")

### Component 6: Alerting Framework

Declarative alerting via CRD, generating Prometheus AlertManager rules:

```yaml
apiVersion: vpc.roks.ibm.com/v1alpha1
kind: VPCNetworkAlert
metadata:
  name: high-latency-alert
spec:
  # Threshold alerts
  rules:
    - name: RouterHighLatency
      metric: vpc_router_latency_ms
      condition: "> 50"
      duration: 5m
      severity: warning
      labels:
        network: "{{ $labels.network }}"

    - name: SubnetIPExhaustion
      metric: vpc_subnet_ip_utilization_percent
      condition: "> 90"
      duration: 15m
      severity: critical

    - name: FirewallRuleUnused
      metric: nftables_rule_hits_total
      condition: "== 0"
      duration: 7d
      severity: info

    - name: VPCAPIDegraded
      metric: rate(vpc_api_calls_total{status="error"}[5m])
      condition: "> 0.1"
      severity: warning

  # Notification channels
  notifications:
    - type: event     # K8s Event on the affected resource
    - type: webhook
      url: https://hooks.slack.com/services/...
    - type: pagerduty
      serviceKey:
        secretRef:
          name: pagerduty-key
```

### Implementation Roadmap

**Phase 1 — Metrics foundation** (IMPLEMENTED)
- Metrics exporter sidecar binary (`cmd/metrics-exporter/`) with 5 collectors: interface throughput, nftables rule counters, conntrack usage, DHCP pool utilization, process health
- `spec.metrics.enabled: true` on VPCRouter injects the sidecar; `MetricsReady` status condition
- nftables `counter` keyword on all firewall, NAT, and IPS rules for rule hit tracking
- PodMonitor for OpenShift Prometheus scraping (15s interval)
- BFF Thanos Query integration with 5 metrics endpoints (summary, interfaces, conntrack, dhcp, nft)
- Console plugin Observability page with multi-router selector, time range controls, throughput charts, conntrack/DHCP gauges, NFT counters table
- RouterDetailPage enhanced with Monitoring tab and NFT Rules tab
- Dashboard Router Health section for metrics-enabled routers

**Phase 2 — Console plugin enhancements** (under consideration)
- Health status overlays on TopologyViewer
- Alert timeline panel on VPC Dashboard
- Per-subnet traffic breakdown views
- VPC Flow Logs integration (IBM Cloud native feature, no pod changes)

**Phase 3 — Traceflow** (under consideration)
- `VPCTraceflow` CRD + reconciler
- Active probing from router pod with nftables log correlation
- Results visualization in console plugin (path diagram)

**Phase 4 — Full observability stack** (under consideration)
- Latency mesh probing (Goldpinger sidecar)
- Anomaly detection and correlation engine
- ntopng integration for DPI (cross-ref with Traffic Analysis section)
- Grafana dashboard templates in Helm chart

### Integration Notes

- Router pod already has `NET_ADMIN` — nftables counters, tc stats, /proc/net are all accessible
- VPC Flow Logs are an IBM Cloud native feature — no pod changes needed, just a VPC API call per subnet
- Prometheus is standard on OpenShift (via Cluster Monitoring Operator) — custom metrics just need ServiceMonitor CRDs
- Console plugin's TopologyViewer is the natural home for traffic overlays and health visualization
- The BFF service can aggregate metrics + K8s events + VPC Flow Log data for the console plugin
- Traceflow and latency probing can reuse the router pod rather than deploying new infrastructure

## NSX-T to OVN-Kubernetes L2 Bridging

**Status**: Implemented (2026-03-02) — See `docs/plans/2026-03-01-vpcl2bridge-design.md`

> **Implementation complete**: VPCL2Bridge CRD, reconciler, BFF endpoints, and console plugin UI (list/detail/create pages + dashboard card). Supports GRETAP+WireGuard, NSX-T L2VPN, and EVPN-VXLAN tunnel types. Branch: `worktree-l2bridge`.

Enable Layer 2 connectivity between VMware NSX-T segments and OpenShift OVN-Kubernetes secondary networks over any IP-reachable fabric. Primary use case: live migration from VMware/NSX-T to OpenShift Virtualization where VMs on both platforms must share a broadcast domain during the transition.

### Problem Statement

NSX-T uses Geneve overlay encapsulation. OVN-Kubernetes also uses Geneve. These are independent overlay planes that don't naturally interoperate. The physical fabric between them may be anything — ACI, standard leaf-spine, public cloud VPC — so the solution must be **fabric-agnostic**, tunneling L2 frames over L3.

### Approach 1: NSX-T L2 VPN (Recommended for NSX shops)

The most robust option when NSX-T is available on the VMware side.

**NSX side (Server)**
- Configure L2 VPN Server on an NSX-T Tier-0 or Tier-1 Gateway
- Creates a secure tunnel carrying multiple tagged segments

**OpenShift side (Client)**
- Deploy an Autonomous NSX Edge as a VM or container within the OpenShift cluster
- Acts as L2 VPN Client, connecting back to NSX Manager over any IP path
- Decapsulates frames and presents them to a local interface

**Bridging to OVN**
- Use Multus CNI to attach pods/VMs to the Edge's local interface via bridge or macvlan `NetworkAttachmentDefinition`
- Frames arrive on the OVN LocalNet secondary network as native Ethernet

**Operator integration**
- New `VPCBridge` or `L2Bridge` CRD to declare the bridge relationship
- Operator manages the Autonomous Edge pod lifecycle (similar to VPCRouter)
- Auto-creates the Multus `NetworkAttachmentDefinition` and maps to the CUDN/UDN

### Approach 2: GRETAP over WireGuard (Fabric-agnostic, no NSX dependency)

Uses the existing VPCRouter pod architecture with FRRouting + WireGuard:

**Encapsulation stack**
```
Ethernet frame → GRETAP (L2 GRE) → WireGuard (encrypted) → Fabric (IP)
```

- GRETAP tunnel provides the L2 bridge — preserves broadcast domain, MAC learning, BUM traffic
- WireGuard provides encryption and NAT traversal
- Terminates inside the router pod, bridged to OVN LocalNet via Multus

**Traffic flow**
```
NSX-T Segment ──► NSX Edge TEP ──► [IP Fabric] ──► WireGuard ──► GRETAP ──► Router Pod ──► OVN LocalNet ──► KubeVirt VM
```

**Fit with VPCRouter**
- Router pod already has `NET_ADMIN`, Multus attachments, and nftables — adding GRETAP + WireGuard is incremental
- Could be a new `spec.l2Bridge` section on VPCRouter or a dedicated `VPCBridgePod`

### Approach 3: EVPN-VXLAN with FRRouting (Most scalable)

For environments where NSX-T supports hardware VTEP integration:

- FRRouting's BGP EVPN exchanges MAC/IP advertisements with NSX-T
- VXLAN tunnels carry L2 frames between FRR (in router pod) and NSX VTEPs
- MAC addresses learned dynamically — no static configuration per VM
- Scales to large environments with many segments and VMs

**Advantages over GRETAP**
- Dynamic MAC learning (no flooding for unknown unicasts)
- Multi-homing and fast failover via EVPN multi-homing (ESI)
- Standard protocol — interoperates with any EVPN-capable fabric

**Disadvantages**
- More complex to configure and debug
- Requires BGP peering between FRR and NSX-T (or a route reflector)
- FRRouting container image is heavier

### Key Requirements

| Component | Requirement |
|-----------|-------------|
| **MTU** | Fabric MTU must be 1700+ (or clamp MSS). Double encapsulation: NSX Geneve (50B) + WireGuard (60B) or GRETAP (24B) + WireGuard (60B) |
| **Multus CNI** | Essential — provides pods/VMs with secondary interfaces bypassing OVN-K north-south path |
| **Promiscuous mode** | Required on vSphere port groups (or BM NIC) to allow foreign MAC addresses from the bridged side |
| **IP reachability** | Tunnel endpoints must be IP-reachable (floating IP on VPC side, public/VPN IP on NSX side) |
| **BUM handling** | GRETAP: flood-and-learn. EVPN: head-end replication or ingress replication |

### CRD Sketch

```yaml
apiVersion: vpc.roks.ibm.com/v1alpha1
kind: VPCL2Bridge
metadata:
  name: nsx-migration-bridge
spec:
  # Which OVN network to bridge
  networkRef:
    name: workload-net-1
    kind: ClusterUserDefinedNetwork

  # Remote endpoint (NSX Edge or another cluster)
  remote:
    endpoint: 203.0.113.50
    type: gretap-wireguard  # gretap-wireguard | l2vpn | evpn-vxlan

    # GRETAP + WireGuard specific
    wireguard:
      privateKey:
        secretRef:
          name: wg-bridge-key
          key: privateKey
      peerPublicKey: "aB3d...="
      listenPort: 51820

    # EVPN-VXLAN specific (alternative)
    # evpn:
    #   asn: 65000
    #   vni: 10100
    #   routeReflector: 10.0.0.1

  # MTU handling
  mtu:
    tunnelMTU: 1400        # inner MTU after encapsulation overhead
    mssClamp: true         # auto-clamp TCP MSS

  # Gateway for bridged traffic
  gatewayRef:
    name: my-gateway       # gets FIP for tunnel endpoint

status:
  phase: Established       # Pending | Establishing | Established | Degraded
  tunnelEndpoint: 169.48.x.x:51820
  remoteMACsLearned: 12
  localMACsAdvertised: 8
  bytesIn: 1073741824
  bytesOut: 536870912
```

### Use Cases Beyond NSX Migration

- **Multi-cloud L2**: Bridge a VPC workload network to an AWS/Azure VNET at L2
- **Disaster recovery**: L2 stretch for VM live migration between sites
- **Lab-to-production**: Bridge a local lab network to cloud VMs during development
- **Bare metal integration**: Connect non-virtualized bare metal servers to OVN LocalNet networks

### Reference Implementation: GRETAP + WireGuard CLI

Detailed commands for Approach 2, which maps directly to additions in `buildInitScript()`.

**Step 1 — WireGuard underlay (secure L3 path)**
```bash
# Create WireGuard interface with point-to-point addressing
ip link add dev wg0 type wireguard
ip addr add 10.0.0.1/30 dev wg0

# Configure peer (NSX Edge or remote appliance)
wg set wg0 private-key /run/secrets/wg/privateKey \
    peer <REMOTE_PUBLIC_KEY> \
    endpoint <REMOTE_PUBLIC_IP>:51820 \
    allowed-ips 10.0.0.0/30

ip link set wg0 up
```

**Step 2 — GRETAP tunnel (L2 over the WG underlay)**
```bash
# GRETAP uses the WireGuard tunnel IPs (not real IPs) as endpoints
# This is what makes it fabric-agnostic — only needs IP reachability
ip link add dev gretap0 type gretap \
    local 10.0.0.1 \
    remote 10.0.0.2 \
    ttl 255

# MTU: 1500 - 60 (WG) - 38 (GRETAP+Eth header) = 1402 → use 1400
ip link set gretap0 mtu 1400
ip link set gretap0 up
```

**Step 3 — Bridge GRETAP to the OVN LocalNet interface**
```bash
# Create bridge and attach tunnel + local workload interface
ip link add name br-bridge type bridge
ip link set gretap0 master br-bridge
ip link set net0 master br-bridge    # net0 = Multus-attached OVN LocalNet interface

ip link set br-bridge up
```

After these 3 steps, Ethernet frames from the NSX-T segment arrive at `gretap0`, cross the bridge, and appear on `net0` (the OVN LocalNet interface) — KubeVirt VMs on that network see them as native L2 traffic.

### Node-Level Alternative: NMState Operator

For deployments where the bridge should be persistent on the node (not inside the router pod), use the NMState Operator:

**NodeNetworkConfigurationPolicy (NNCP)**
```yaml
apiVersion: nmstate.io/v1
kind: NodeNetworkConfigurationPolicy
metadata:
  name: br-nsx-gretap-policy
spec:
  nodeSelector:
    node-role.kubernetes.io/worker: ""
  desiredState:
    interfaces:
      - name: br-nsx
        type: bridge
        state: up
        bridge:
          port:
            - name: gretap-nsx
      - name: gretap-nsx
        type: gretap
        state: up
        local: 10.0.0.1     # WireGuard tunnel IP on this node
        remote: 10.0.0.2    # WireGuard tunnel IP on NSX side
        ttl: 64
        mtu: 1400
```

**NetworkAttachmentDefinition (NAD)**
```yaml
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: nsx-l2-ext
  namespace: my-app-namespace
spec:
  config: '{
    "cniVersion": "0.3.1",
    "name": "nsx-l2-ext",
    "type": "bridge",
    "bridge": "br-nsx",
    "ipam": { "type": "static" }
  }'
```

### Operator Integration: Two Deployment Models

| | Router Pod Model | Node-Level Model |
|---|---|---|
| **Where tunnel lives** | Inside VPCRouter pod init script | On the node via NMState NNCP |
| **Lifecycle** | Tied to router pod (tunnel flaps on restart) | Persistent across pod restarts |
| **Operator manages** | Init script commands in `buildInitScript()` | NNCP + NAD CRDs via K8s API |
| **WireGuard keys** | Mounted from Secret into pod | Mounted into node via MachineConfig |
| **Best for** | Single-gateway, simple setups | Production, multi-node, HA |
| **Existing pattern** | Extends VPCRouter (familiar) | New reconciler for NNCP/NAD |

**Recommended path**: Start with the router pod model (init script additions) for a quick proof-of-concept, then graduate to the node-level NMState model for production, where the operator manages NNCP and NAD CRDs declaratively.

### Operational Considerations

- **MAC learning**: OVN-K treats the bridge as a standard L2 segment. Enable MAC learning on the NSX-T side to handle return traffic to bridged pods/VMs
- **Promiscuous mode**: Required on vSphere port groups (or BM NIC config) to allow "forged" MAC addresses from the remote side of the bridge
- **Security context**: Pods using the bridged interface may need `NET_ADMIN` capability to manipulate secondary interface parameters
- **WireGuard routing**: Ensure the WireGuard routing table allows `10.0.0.2` to be reachable over `wg0` before the GRETAP tunnel will function
- **Jumbo frames**: If the physical fabric supports MTU 9000, use it — eliminates the inner MTU reduction entirely. If stuck at 1500, clamp inner MTU to 1350-1400 and enable TCP MSS clamping via nftables:
  ```bash
  nft add rule inet filter forward tcp flags syn / syn,rst \
      tcp option maxseg size set 1360
  ```

### Implementation Notes

- GRETAP + WireGuard is the simplest starting point — both are kernel modules, minimal config
- The router pod's init script already has a pattern for adding interfaces and nftables rules — GRETAP follows the same pattern
- MTU is the biggest operational concern — must be validated before bridge activation
- Consider a "bridge health" probe that verifies L2 reachability (ARP the remote gateway) and reports to status
- EVPN-VXLAN is the "Phase 2" approach for scale — start with GRETAP for single-segment bridges
- The console plugin topology view should show bridged segments with a visual link to the remote side
