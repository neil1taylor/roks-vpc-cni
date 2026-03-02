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

**Status**: Under consideration

The current VPCRouter pod uses a Fedora container running a bash init script with kernel nftables forwarding over Multus veth interfaces. This is functional but has throughput limitations for high-bandwidth workloads.

### Current Architecture

Traffic path: **VM → OVN overlay → Multus veth → router pod (nftables) → uplink veth → OVN overlay → VPC fabric**

| Factor | Current State | Impact |
|--------|--------------|--------|
| **Data path** | Veth pairs (Multus CNI) | ~5-10% overhead vs native bridge |
| **Forwarding engine** | Kernel nftables | Single-threaded per flow, no fast-path |
| **Resource limits** | None set | Competes with all pods on the node |
| **CPU affinity** | None | Kernel scheduler decides, no pinning |
| **NIC mode** | OVN LocalNet (software) | No SR-IOV passthrough |
| **Container image** | `fedora:40` + bash init | Not a purpose-built forwarding binary |
| **Acceleration** | None | No DPDK, XDP, or eBPF fast-path |

### Estimated Throughput

- **Small packets (64B):** ~200-500 Kpps
- **Large packets / bulk transfer:** ~5-15 Gbps (depends on BM NIC and available CPU)
- **With NAT rules:** ~10-30% additional overhead per rule chain

### Proposed Improvements

**Tier 1 — Low effort, immediate gains**

- **Resource requests and limits** — Guarantee CPU/memory for the forwarding pod. Prevents starvation under node pressure.
- **CPU pinning** — Use `runtimeClassName` or Kubernetes CPU Manager (`static` policy) to pin the router pod to dedicated cores. Eliminates context-switch overhead.
- **Node affinity** — Schedule router pods on nodes with the most available bandwidth (bare metal workers with 25/100G NICs).

**Tier 2 — Medium effort, significant gains**

- **Purpose-built router image** — Replace `fedora:40` + bash script with a compiled Go or C binary that configures interfaces and applies nftables at startup. Eliminates `dnf install` overhead (~30s startup), reduces image size from ~1GB to ~50MB, and enables structured health reporting.
- **XDP/eBPF fast-path** — Attach XDP programs to Multus interfaces for forwarding decisions before packets enter the kernel network stack. 10-100x PPS improvement for simple forwarding rules. Complex NAT still falls back to nftables.
- **Host networking mode** — Run the router pod with `hostNetwork: true` and use VRF or network namespaces for isolation. Eliminates veth overhead entirely. Tradeoff: loses Multus interface abstraction.

**Tier 3 — High effort, maximum performance**

- **SR-IOV interfaces** — Use SR-IOV VFs for the uplink interface, bypassing OVN entirely on the VPC side. Requires SR-IOV device plugin and Network Operator configuration. Near line-rate performance.
- **DPDK userspace forwarding** — Run VPP (fd.io) or a custom DPDK application inside the router pod for full userspace packet processing. Requires hugepages, dedicated cores, and SR-IOV VFs. Achieves 40-100 Gbps on modern NICs.
- **FRRouting replacement** — Replace the bash init script with FRRouting (FRR) for dynamic routing protocol support (BGP, OSPF) alongside high-performance forwarding via its built-in dataplane.

### Resource Configuration Sketch

```yaml
# VPCRouter CRD extension
spec:
  pod:
    resources:
      requests:
        cpu: "2"
        memory: "1Gi"
      limits:
        cpu: "4"
        memory: "2Gi"
    runtimeClassName: performance  # CPU Manager static policy
    nodeSelector:
      node-role.kubernetes.io/worker: ""
      feature.node.kubernetes.io/network-sriov.capable: "true"
```

### When to Invest

For most KubeVirt VM workloads (tens of VMs, moderate traffic), the current approach is sufficient. Performance optimization becomes important for:

- **NFV workloads** — Network functions requiring line-rate packet processing
- **Storage replication** — iSCSI/NFS between VMs across subnets
- **Bulk data transfer** — Analytics, ML training data movement between many VMs
- **High VM density** — 100+ VMs routing through a single router pod
- **Latency-sensitive** — Financial, gaming, or real-time communication workloads

### Implementation Notes

- Tier 1 changes are additive — extend `buildRouterPod()` in `pkg/controller/router/pod.go` to set resource fields from `spec.pod.resources`
- XDP programs can coexist with nftables — use XDP for fast-path forwarding, nftables for complex NAT/firewall
- SR-IOV requires the OpenShift SR-IOV Network Operator to be installed on the cluster
- DPDK requires hugepages configuration via MachineConfig and node tuning
- A purpose-built image could be maintained as `de.icr.io/roks/vpc-network-router:latest` alongside the operator/BFF/plugin images

---

## Enhanced DHCP Management

**Status**: Under consideration

The current VPCRouter DHCP implementation is minimal — a single `Enabled` bool that starts dnsmasq with auto-computed ranges and hardcoded 12h leases. This section captures enhancements to make DHCP production-grade.

### Current State

- `RouterDHCP` struct: `{ Enabled: bool }` — global on/off only
- Range: auto-computed from network CIDR (network+10 to broadcast-1)
- Lease time: hardcoded 12h
- No DNS, no static reservations, no per-network control

### Proposed Enhancements

**Per-network DHCP configuration**
- Move DHCP config from `spec.dhcp` to `spec.networks[].dhcp` so each workload network can have independent settings
- Allow enabling DHCP on some networks but not others

**Custom address ranges**
- `rangeStart` / `rangeEnd` overrides (currently auto-computed)
- Exclude ranges for reserved IPs (e.g., gateways, load balancers)

**Static reservations**
- MAC-to-IP mappings for VMs that need stable addresses
- Integration with VPC reserved IPs so DHCP assignments match VPC state
- Could auto-populate from VNI MAC/IP data already tracked by the operator

**DNS configuration**
- Custom nameservers (currently none — VMs fall back to defaults)
- Search domains
- Local DNS for VM hostnames within the network (dnsmasq supports this natively)

**DHCP options**
- Gateway / router option (option 3)
- MTU (option 26) — important for VLAN-encapsulated networks
- NTP servers (option 42)
- Custom options passthrough

**Lease management**
- Configurable lease time (currently hardcoded 12h)
- Lease status reporting in `VPCRouterStatus`
- Lease file persistence across pod restarts (currently lost)

**IPv6 support**
- Router Advertisement (RA) for SLAAC
- DHCPv6 for stateful IPv6 assignment
- Dual-stack configuration per network

### Implementation Notes

- All enhancements use dnsmasq flags — no additional software needed
- Per-network config requires extending `RouterNetwork` struct in `vpcrouter_types.go`
- Static reservations could be auto-synced from VNI CRD status (MAC + reserved IP)
- Lease persistence needs a volume mount or ConfigMap-based storage

## DNS Filtering and Secure DNS

**Status**: Under consideration

Provide network-wide ad blocking, malware domain filtering, and encrypted DNS (DoH/DoT) for all VMs on operator-managed networks.

### Candidates

- **Pi-hole** — Mature, widely deployed DNS sinkhole. Web UI for management, extensive blocklists, per-client statistics. Heavier footprint (FTL engine + lighttpd + SQLite).
- **AdGuard Home** — Single Go binary, built-in DoH/DoT/DoQ server, native DHCP server, parental controls, per-client settings. Lighter than Pi-hole, more modern feature set.

### Integration with VPCRouter

The router pod already runs dnsmasq for DHCP. DNS filtering would extend this:

**Option A — Sidecar container**
- Run Pi-hole or AdGuard Home as a sidecar in the router pod
- dnsmasq forwards DNS to the sidecar (`--server=127.0.0.1#5353`)
- Sidecar handles filtering and upstream DoH/DoT
- Pros: single pod, shared network namespace, simple DHCP integration
- Cons: increased router pod resource requirements

**Option B — Dedicated DNS pod per gateway**
- Separate deployment managed by VPCGateway or a new `VPCDNSPolicy` CRD
- Router's dnsmasq points VMs to the DNS pod IP
- Pros: independent scaling, can serve multiple routers, isolated failure domain
- Cons: additional pod, needs service discovery

**Option C — AdGuard Home replaces dnsmasq**
- AdGuard Home has a built-in DHCP server — could replace dnsmasq entirely
- Single process handles DHCP + DNS + filtering + DoH/DoT
- Pros: simplest architecture, fewer moving parts
- Cons: tighter coupling, less flexibility if DHCP and DNS need independent lifecycle

### Features

- **Blocklists**: Configurable via CRD — ad domains, malware, tracking, custom lists
- **Encrypted upstream DNS**: DoH (DNS-over-HTTPS) and DoT (DNS-over-TLS) to upstream resolvers (Cloudflare, Quad9, Google, custom)
- **Per-network policies**: Different filtering rules per workload network (e.g., stricter for production, relaxed for dev)
- **Local DNS**: Resolve VM hostnames within the network (synced from VNI CRD metadata)
- **Allowlists/Denylists**: Per-tenant overrides via annotations or CRD spec
- **Query logging and analytics**: Expose metrics to Prometheus, surface in console plugin dashboard

### CRD Sketch

```yaml
apiVersion: vpc.roks.ibm.com/v1alpha1
kind: VPCDNSPolicy
metadata:
  name: production-dns
spec:
  gatewayRef:
    name: my-gateway
  upstream:
    servers:
      - url: https://cloudflare-dns.com/dns-query  # DoH
      - url: tls://dns.quad9.net                    # DoT
  filtering:
    enabled: true
    blocklists:
      - https://raw.githubusercontent.com/StevenBlack/hosts/master/hosts
      - https://adaway.org/hosts.txt
    allowlist:
      - "*.internal.example.com"
    denylist:
      - "tracking.example.com"
  localDNS:
    enabled: true
    domain: vm.local
```

### Implementation Notes

- AdGuard Home is the better fit — single binary, Go-native (matches operator stack), built-in DoH/DoT, lighter than Pi-hole
- Option C (AdGuard Home replacing dnsmasq) is the cleanest long-term but requires migrating DHCP config
- Option A (sidecar) is the lowest-risk incremental step
- DNS pod image could be configurable in VPCGateway spec (similar to `routerImage`)
- Filtering config should be declarative via CRD, not through the AdGuard/Pi-hole web UI (though the UI could be optionally exposed)

## VPN Gateway

**Status**: Under consideration

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
| **WireGuard** | Yes | Yes (wg-quick) | Limited (newer firewalls only) | Excellent |
| **IPsec/IKEv2** (StrongSwan) | Yes | Yes | Excellent (Cisco, Fortinet, Palo Alto) | Good |
| **OpenVPN** | Yes | Yes (broad client support) | Moderate | Moderate |

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

**Status**: Under consideration

Build a comprehensive network monitoring, analytics, and alerting stack comparable to VMware NSX's built-in observability — traceflow, flow monitoring, micro-segmentation analytics, health dashboards, and latency analysis.

### NSX Feature Mapping

| NSX Feature | Current State | Proposed Solution |
|-------------|--------------|-------------------|
| **Traceflow** (synthetic packet path tracing) | None | Custom CRD + eBPF or `traceroute`/`nping` in router pod |
| **Flow Monitoring** (top talkers, app stats) | None (see Traffic Analysis section) | VPC Flow Logs + ntopng + Prometheus |
| **Live Traffic Analysis** (per-interface stats) | None | eBPF (Pixie) or `/proc/net` polling in router pod |
| **Micro-segmentation Analytics** (rule hits, unused rules) | None | nftables counters + Prometheus export |
| **Network Topology** (interactive, status overlays) | Basic (TopologyViewer, resource relationships only) | Enhanced topology with health status, traffic flow overlays |
| **Latency Monitoring** (per-hop latency) | None | Synthetic probes between router pods (Goldpinger-style) |
| **Health Dashboard** (component health, alarms) | Basic (VPC Dashboard, reconciler events) | Unified health view with SLA tracking |
| **Port Mirroring** | None (see Traffic Analysis section) | tc mirror in router pod |
| **Correlation Engine** (cross-component event correlation) | None | Event aggregation service in BFF |
| **IPFIX/sFlow Export** | None | Router pod sFlow agent → collector |
| **Alerting** (threshold + anomaly) | None | Prometheus AlertManager + custom PrometheusRules |

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

**VPC Flow Logs integration**
- IBM Cloud VPC Flow Logs capture all traffic on VPC subnets natively
- Enable via VPC API (`CreateFlowLogCollector`) per subnet — operator can automate this
- Logs go to a COS bucket, then process with a flow log aggregator
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

**Phase 1 — Metrics foundation** (low effort, high value)
- Extend `pkg/metrics/` with router pod interface counters and nftables hit counts
- Export from router pod via Prometheus annotations
- Enable VPC Flow Logs per operator-managed subnet via VPC API
- Add Grafana dashboard templates to Helm chart

**Phase 2 — Console plugin enhancements** (medium effort)
- Health status overlays on TopologyViewer
- Alert timeline panel on VPC Dashboard
- Per-subnet / per-router detail views with live metrics

**Phase 3 — Traceflow** (higher effort, high value)
- `VPCTraceflow` CRD + reconciler
- Active probing from router pod with nftables log correlation
- Results visualization in console plugin (path diagram)

**Phase 4 — Full observability stack** (higher effort)
- Latency mesh probing (Goldpinger sidecar)
- Micro-segmentation analytics
- Anomaly detection and correlation engine
- ntopng integration for DPI (cross-ref with Traffic Analysis section)

### Integration Notes

- Router pod already has `NET_ADMIN` — nftables counters, tc stats, /proc/net are all accessible
- VPC Flow Logs are an IBM Cloud native feature — no pod changes needed, just a VPC API call per subnet
- Prometheus is standard on OpenShift (via Cluster Monitoring Operator) — custom metrics just need ServiceMonitor CRDs
- Console plugin's TopologyViewer is the natural home for traffic overlays and health visualization
- The BFF service can aggregate metrics + K8s events + VPC Flow Log data for the console plugin
- Traceflow and latency probing can reuse the router pod rather than deploying new infrastructure
