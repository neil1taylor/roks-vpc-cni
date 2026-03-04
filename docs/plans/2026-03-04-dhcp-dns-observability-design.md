# Design: DHCP Persistence, DNS Filtering, Observability Phase 2, Auto-Reservations

**Date**: 2026-03-04
**Status**: Approved
**Branch**: `feat/dhcp-dns-obs-phase2`

---

## Feature 1: DHCP Lease Persistence

### Problem

DHCP lease files live in emptyDir volumes. When the router pod restarts, all leases are lost — VMs get new IPs on next DHCP renewal, breaking static expectations.

### Solution

Add a small PVC per router pod for lease file storage.

### CRD Changes

Add `spec.dhcp.leasePersistence` to VPCRouter:

```yaml
dhcp:
  leasePersistence:
    enabled: true
    storageSize: 100Mi          # default
    storageClassName: ""        # empty = cluster default SC
```

**Types** (`api/v1alpha1/vpcrouter_types.go`):

```go
type DHCPLeasePersistence struct {
    Enabled          bool   `json:"enabled"`
    StorageSize      string `json:"storageSize,omitempty"`      // default "100Mi"
    StorageClassName string `json:"storageClassName,omitempty"` // empty = default SC
}
```

Add `LeasePersistence *DHCPLeasePersistence` to `RouterDHCP`.

### Reconciler Changes

In `pkg/controller/router/reconciler.go`:

1. When `leasePersistence.enabled`:
   - Create PVC `<router>-dhcp-leases` (100Mi RWO) with ownerReference to VPCRouter CR
   - If PVC already exists and is Bound, proceed; if Pending, requeue
2. Set `LeasePersistenceReady` status condition

### Pod Construction Changes

In `buildRouterPod()` and `buildFastpathRouterPod()`:

- If persistence enabled: use PVC volume instead of emptyDir for `dnsmasq-leases`
- If persistence disabled: use emptyDir (current behavior)
- Metrics exporter sidecar mount unchanged (same path, read-only)

### Status

Add `LeasePersistenceReady` condition:
- `True` when PVC is Bound
- `False` with reason `PVCPending` or `PVCCreateFailed`

### Helm

- Add PVC RBAC (create/get/delete persistentvolumeclaims) to operator ClusterRole

---

## Feature 2: DNS Filtering (AdGuard Home Sidecar)

### Problem

VMs get raw upstream DNS — no ad/malware filtering, no encrypted DNS (DoH/DoT), no per-network policies.

### Solution

New `VPCDNSPolicy` CRD. When referencing a VPCRouter, the router reconciler injects an AdGuard Home sidecar and configures dnsmasq to forward DNS to it.

### CRD

```yaml
apiVersion: vpc.roks.ibm.com/v1alpha1
kind: VPCDNSPolicy
metadata:
  name: production-dns
spec:
  routerRef:
    name: my-router
  upstream:
    servers:
      - url: https://cloudflare-dns.com/dns-query   # DoH
      - url: tls://dns.quad9.net                      # DoT
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
  image: adguard/adguardhome:latest
status:
  phase: Active         # Pending | Active | Degraded
  filterRulesLoaded: 54321
  conditions: [...]
```

**Short name**: `vdp`

**Types** (`api/v1alpha1/vpcdnspolicy_types.go`):

```go
type VPCDNSPolicy struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`
    Spec              VPCDNSPolicySpec   `json:"spec,omitempty"`
    Status            VPCDNSPolicyStatus `json:"status,omitempty"`
}

type VPCDNSPolicySpec struct {
    RouterRef  RouterReference       `json:"routerRef"`
    Upstream   DNSUpstreamConfig     `json:"upstream,omitempty"`
    Filtering  DNSFilteringConfig    `json:"filtering,omitempty"`
    LocalDNS   DNSLocalConfig        `json:"localDNS,omitempty"`
    Image      string                `json:"image,omitempty"`
}

type DNSUpstreamConfig struct {
    Servers []DNSUpstreamServer `json:"servers"`
}

type DNSUpstreamServer struct {
    URL string `json:"url"` // https:// for DoH, tls:// for DoT, plain IP for standard
}

type DNSFilteringConfig struct {
    Enabled    bool     `json:"enabled"`
    Blocklists []string `json:"blocklists,omitempty"`
    Allowlist  []string `json:"allowlist,omitempty"`
    Denylist   []string `json:"denylist,omitempty"`
}

type DNSLocalConfig struct {
    Enabled bool   `json:"enabled"`
    Domain  string `json:"domain,omitempty"`
}

type VPCDNSPolicyStatus struct {
    Phase             string             `json:"phase,omitempty"`
    FilterRulesLoaded int64              `json:"filterRulesLoaded,omitempty"`
    Conditions        []metav1.Condition `json:"conditions,omitempty"`
}
```

### Reconciler

New `pkg/controller/dnspolicy/reconciler.go`:

1. Watch `VPCDNSPolicy` CRDs
2. On create/update:
   - Validate `routerRef` points to existing VPCRouter
   - Generate AdGuard Home YAML config from spec
   - Create/update ConfigMap `<dnspolicy-name>-adguard-config` with the YAML
   - Set status phase and conditions
3. Finalizer: `vpc.roks.ibm.com/dnspolicy-cleanup` — deletes ConfigMap on CR deletion

### Router Pod Integration

Router reconciler checks for VPCDNSPolicy referencing it:

1. **List** VPCDNSPolicies where `spec.routerRef.name == router.Name`
2. If found, inject AdGuard Home sidecar:
   - Container: `adguard-home` image, ports 5353 (DNS) and 3000 (web UI)
   - Mount ConfigMap at `/opt/adguardhome/conf/AdGuardHome.yaml`
   - Working dir volume (emptyDir) at `/opt/adguardhome/work/`
   - Health probe: HTTP GET `http://127.0.0.1:3000/control/status`
   - Resources: 50m CPU request, 128Mi memory request
3. Modify dnsmasq args: append `--server=127.0.0.1#5353` (forward all DNS to sidecar)
4. Track DNS policy config hash for pod drift detection

### AdGuard Home Config Generation

`pkg/controller/dnspolicy/adguard_config.go`:

```go
func generateAdGuardConfig(spec VPCDNSPolicySpec) string {
    // Generates AdGuardHome.yaml with:
    // - bind_host: 127.0.0.1, bind_port: 5353
    // - upstream_dns: spec.upstream.servers[].url
    // - filtering: spec.filtering (blocklists, allow/deny)
    // - local_domain: spec.localDNS.domain
    // - web UI on port 3000 (no auth, cluster-internal only)
}
```

### BFF

- `GET /api/v1/dns-policies` — list all
- `GET /api/v1/dns-policies/:name` — get one
- `POST /api/v1/dns-policies` — create
- `DELETE /api/v1/dns-policies/:name` — delete
- Handler in `cmd/bff/internal/handler/dnspolicy.go`, uses dynamic client

### Console Plugin

- **List page**: Table with name, router ref, phase, filter rules count, upstream servers
- **Detail page**: Overview card, upstream servers card, filtering config card (blocklists, allow/deny lists), status conditions
- **Create page**: Form with router selector, upstream server repeater, blocklist URLs, allow/deny patterns
- **Dashboard card**: Active DNS policies count, total rules loaded

### Helm

- CRD YAML with printer columns (Router, Phase, Rules, Age)
- RBAC for `vpcdnspolicies` (get/list/watch/create/update/delete)
- RBAC for ConfigMaps in operator namespace
- Default image value: `adguardhome.image: adguard/adguardhome:latest`

---

## Feature 3: Observability Phase 2

Four enhancements building on Phase 1's metrics exporter, BFF Thanos integration, and console Observability page.

### 3a. Health Status Overlays on TopologyViewer

#### BFF Changes

Extend `GET /api/v1/topology` with `?includeHealth=true`:

- For router nodes: query Thanos for interface error rates, conntrack %, process status
- For subnet nodes: compute IP utilization from DHCP pool metrics
- For VNI nodes: use CRD sync status
- Add to node metadata:

```go
type NodeHealth struct {
    Status       string            `json:"status"`       // healthy | warning | critical
    Metrics      map[string]float64 `json:"metrics"`     // throughputBps, conntrackPct, errorRate
}
```

For edges between routers and subnets, include throughput data for animated edge widths.

#### Console Changes

In `TopologyViewer.tsx`:

- Map `metadata.health.status` to PatternFly `NodeStatus`:
  - `healthy` → `NodeStatus.success` (green)
  - `warning` → `NodeStatus.warning` (yellow) — conntrack >80% or error rate >0
  - `critical` → `NodeStatus.danger` (red) — process down or conntrack >95%
- Edge stroke width proportional to throughput (min 1px, max 6px)
- Add auto-refresh toggle in toolbar (poll every 30s when enabled)
- Add health legend in toolbar

### 3b. Alert Timeline on VPC Dashboard

#### BFF Endpoint

`GET /api/v1/alerts/timeline?range=24h`:

```go
type AlertTimelineEntry struct {
    Timestamp  time.Time `json:"timestamp"`
    Severity   string    `json:"severity"`    // info | warning | critical
    Source     string    `json:"source"`       // "k8s-event" | "prometheus-alert"
    Message    string    `json:"message"`
    ResourceRef *ResourceRef `json:"resourceRef,omitempty"`
}

type ResourceRef struct {
    Kind      string `json:"kind"`       // VPCRouter, VPCSubnet, etc.
    Name      string `json:"name"`
    Namespace string `json:"namespace"`
}
```

Implementation:
- Fetch K8s Warning events for `vpc.roks.ibm.com` CRDs via K8s API
- Fetch firing Prometheus alerts from Thanos `/api/v1/alerts`
- Merge, deduplicate, sort by timestamp descending
- Cap at 100 entries

#### Console Component

`AlertTimelineCard` on VPC Dashboard page:

- Vertical timeline with colored severity dots
- Filter chips by resource type (router, gateway, subnet, VNI)
- Click entry → navigate to resource detail page
- Auto-refresh every 30s via `useBFFDataPolling`
- Empty state: "No alerts in the last 24 hours"

### 3c. Per-Subnet Traffic Breakdown

#### BFF Endpoint

`GET /api/v1/subnets/{name}/metrics?namespace=...&range=1h&step=1m`:

```go
type SubnetMetrics struct {
    ThroughputRx  []TimeSeriesPoint `json:"throughputRx"`
    ThroughputTx  []TimeSeriesPoint `json:"throughputTx"`
    DHCPPoolSize  int               `json:"dhcpPoolSize"`
    DHCPActive    int               `json:"dhcpActiveLeases"`
    DHCPUtilPct   float64           `json:"dhcpUtilizationPct"`
}
```

Implementation:
- Cross-reference subnet name → VPCRouter `spec.networks` → interface name (net0, net1, etc.)
- Query Thanos for `router_interface_rx_bytes_total{interface="<iface>"}` and tx counterpart
- Query DHCP pool metrics for that interface
- Return combined metrics

#### Console Component

`SubnetMetricsTab` on VPCSubnet detail page:

- Throughput line chart (rx/tx bps over time, same style as Observability page)
- DHCP pool utilization gauge (reuse existing `DHCPPoolGauge` component)
- Time range selector (5m/15m/1h/6h/24h)

### 3d. VPC Flow Logs Integration

#### VPC Client

Add to `pkg/vpc/client.go`:

```go
type FlowLogClient interface {
    CreateFlowLogCollector(ctx context.Context, opts CreateFlowLogCollectorOptions) (*FlowLogCollector, error)
    DeleteFlowLogCollector(ctx context.Context, id string) error
    ListFlowLogCollectors(ctx context.Context) ([]FlowLogCollector, error)
    GetFlowLogCollector(ctx context.Context, id string) (*FlowLogCollector, error)
}

type CreateFlowLogCollectorOptions struct {
    Name           string
    TargetSubnetID string
    COSBucketCRN   string
    IsActive       bool
}

type FlowLogCollector struct {
    ID             string
    Name           string
    TargetSubnetID string
    COSBucketCRN   string
    IsActive       bool
    LifecycleState string
}
```

#### CRD Changes

Add to VPCSubnet spec:

```go
type FlowLogConfig struct {
    Enabled      bool   `json:"enabled"`
    COSBucketCRN string `json:"cosBucketCRN"`
    Interval     *int32 `json:"interval,omitempty"` // aggregation seconds, default 300
}
```

Add `FlowLogs *FlowLogConfig` to `VPCSubnetSpec`.

#### Reconciler Changes

In VPCSubnet reconciler:
- When `spec.flowLogs.enabled` and `cosBucketCRN` set:
  - Check if flow log collector already exists (tag-based lookup)
  - Create if missing, tag with cluster ID + subnet name
  - Store collector ID in `status.flowLogCollectorID`
- On delete (finalizer): delete the flow log collector

#### Status

```go
FlowLogCollectorID string `json:"flowLogCollectorID,omitempty"`
FlowLogActive      bool   `json:"flowLogActive,omitempty"`
```

#### BFF

- `GET /api/v1/subnets/{name}/flow-logs` — returns flow log collector status
- Handler reads from VPCSubnet CRD status + VPC API for collector details

#### Console

- Toggle switch on VPCSubnet detail page to enable/disable flow logs
- COS bucket CRN input field (required when enabling)
- Status badge showing collection state (Active/Inactive/Creating)

---

## Feature 4: DHCP Auto-populated Reservations

### Problem

Admins must manually extract MAC+IP from VM annotations and add them to VPCRouter DHCP reservations.

### Solution

Opt-in `spec.dhcp.autoReservations` flag. Router reconciler watches VMs and auto-populates reservations from `vpc.roks.ibm.com/network-interfaces` annotations.

### CRD Changes

Add to `RouterDHCP`:

```go
AutoReservations bool `json:"autoReservations,omitempty"` // default false
```

Extend `DHCPNetworkStatus`:

```go
AutoReservationCount int32 `json:"autoReservationCount,omitempty"`
```

### Reconciler Changes

In `pkg/controller/router/reconciler.go`:

1. When `spec.dhcp.autoReservations == true`:
   - List all VMs cluster-wide (unstructured, GVR `kubevirt.io/v1/virtualmachines`)
   - Parse `vpc.roks.ibm.com/network-interfaces` JSON annotation
   - For each VM interface, match `NetworkName` against `spec.networks[].name`
   - Build auto-reservations: `{MAC: iface.MACAddress, IP: iface.ReservedIP, Hostname: vm.Name}`
   - Skip if MAC or IP is empty
2. Merge with manual reservations (manual wins on MAC collision)
3. Pass merged list to `resolvedDHCPConfig()` and `generateDnsmasqCommand()`
4. Track auto-reservation hash for pod drift detection (pod recreated when VMs change)

### Watch Setup

Add VM watch to router's `SetupWithManager()`:

```go
Watches(&source.Kind{Type: &unstructured.Unstructured{}}, // VirtualMachine
    handler.EnqueueRequestsFromMapFunc(r.mapVMToRouters),
)
```

`mapVMToRouters()`:
- Parse VM's `network-interfaces` annotation
- Find all VPCRouters with `autoReservations: true` whose networks match
- Enqueue those routers

### Status

`status.networks[].dhcp.autoReservationCount` shows how many auto-discovered reservations exist per network.

### Console

Router detail page DHCP section shows:
- "Manual reservations: 3"
- "Auto-discovered reservations: 12"
- Table listing all reservations with source column (Manual / Auto)

---

## Implementation Order

1. **Feature 1: DHCP Lease Persistence** — Smallest scope, no new CRDs
2. **Feature 4: DHCP Auto-Reservations** — Builds on DHCP work, extends existing reconciler
3. **Feature 2: DNS Filtering** — New CRD + reconciler + sidecar, independent of 1/4
4. **Feature 3: Observability Phase 2** — Console-heavy, can be parallelized across sub-features

## New Files Summary

| Feature | New Files |
|---------|-----------|
| 1. Lease Persistence | `pkg/controller/router/pvc.go`, CRD type additions |
| 2. DNS Filtering | `api/v1alpha1/vpcdnspolicy_types.go`, `pkg/controller/dnspolicy/reconciler.go`, `pkg/controller/dnspolicy/adguard_config.go`, `cmd/bff/internal/handler/dnspolicy.go`, console pages (3) |
| 3a. Topology Health | BFF topology handler changes, `TopologyViewer.tsx` changes |
| 3b. Alert Timeline | `cmd/bff/internal/handler/alerts.go`, `console-plugin/src/components/AlertTimelineCard.tsx` |
| 3c. Subnet Metrics | `cmd/bff/internal/handler/subnet_metrics.go`, `console-plugin/src/components/SubnetMetricsTab.tsx` |
| 3d. Flow Logs | VPC client methods, VPCSubnet reconciler changes, console toggle |
| 4. Auto-Reservations | Router reconciler changes, VM watch additions |
