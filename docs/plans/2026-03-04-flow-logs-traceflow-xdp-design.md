# Design: VPC Flow Logs + Traceflow + XDP Wiring

**Date**: 2026-03-04
**Status**: Approved
**Branch**: feat/openvpn-protocol (continuing)

## Overview

Three features completing the observability stack and router performance tiers:

1. **VPC Flow Logs SDK Wiring** — Wire the 4 stubbed VPC client methods, add BFF endpoints, add console plugin UI
2. **VPCTraceflow** — New CRD for active network path tracing from router pods
3. **Router Tier 2: XDP/eBPF** — Complete the XDP integration loop with cilium/ebpf

## Feature 1: VPC Flow Logs

### VPC Client (`pkg/vpc/flow_logs.go`)

Wire the 4 stubbed methods to the IBM VPC Go SDK:

| Method | SDK Call | Notes |
|--------|----------|-------|
| `CreateFlowLogCollector` | `vpcService.CreateFlowLogCollector()` | Tag with cluster/owner after creation |
| `GetFlowLogCollector` | `vpcService.GetFlowLogCollector()` | By ID |
| `ListFlowLogCollectors` | `vpcService.ListFlowLogCollectors()` | Filter by target subnet ID |
| `DeleteFlowLogCollector` | `vpcService.DeleteFlowLogCollector()` | By ID |

The reconciler (`pkg/controller/vpcsubnet/reconciler.go`) already calls these methods in `reconcileFlowLogs()`. Errors are non-blocking (logged but don't fail reconciliation).

### BFF Endpoints

- `GET /api/v1/subnets/{id}/flow-logs` — List collectors for a subnet
- `POST /api/v1/subnets/{id}/flow-logs` — Enable flow logs (patches VPCSubnet CR `spec.flowLogs`)
- `DELETE /api/v1/subnets/{id}/flow-logs/{collectorId}` — Disable flow logs (patches VPCSubnet CR)
- Extend `SubnetResponse` with `flowLogCollectorID`, `flowLogActive`, `flowLogs` config

### Console Plugin

- Subnet detail page: "Flow Logs" tab — enable/disable toggle, COS bucket CRN input, interval selector, status badge
- Subnet list page: Flow log status column (green dot = active, gray = inactive)

### Tests

- Unit tests for all 4 VPC client methods (mock HTTP responses)
- Reconciler tests for flow log enable/disable/error scenarios
- BFF handler tests

## Feature 2: VPCTraceflow

### CRD (`vpc.roks.ibm.com/v1alpha1`, short name: `vtf`)

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
    # or direct IP:
    # ip: 10.100.0.15
  destination:
    ip: 8.8.8.8
    port: 443
    protocol: TCP  # TCP | UDP | ICMP
  routerRef:
    name: my-router
  timeout: 30s
  # Auto-delete after this duration (default 1h)
  ttl: 1h

status:
  phase: Completed  # Pending | Running | Completed | Failed
  startTime: "2026-03-04T10:00:00Z"
  completionTime: "2026-03-04T10:00:15Z"
  hops:
    - order: 1
      node: source
      component: "VM test-vm-1 (10.100.0.15)"
      action: Originated
      latency: 0ms
    - order: 2
      node: router-ingress
      component: "VPCRouter/my-router (net0)"
      action: "Received"
      nftablesHits:
        - rule: "snat to 169.48.x.x"
          chain: nat/postrouting
          packets: 1
      latency: 2ms
    - order: 3
      node: router-egress
      component: "VPCRouter/my-router (uplink)"
      action: "SNAT → 169.48.x.x, Forwarded"
      latency: 3ms
    - order: 4
      node: destination
      component: "8.8.8.8:443"
      action: "SYN-ACK received"
      latency: 15ms
  result: Reachable  # Reachable | Unreachable | Filtered | Timeout
  totalLatency: 15ms
```

### Probe Mechanism (Active Probing)

The reconciler execs into the router pod via the K8s API:

1. **Snapshot nftables counters** (before) — `nft list ruleset -j` parsed for counter values
2. **Run probe**:
   - TCP: `nping --tcp -p <port> <dest>` for latency + reachability
   - UDP: `nping --udp -p <port> <dest>`
   - ICMP: `ping -c 3 -W <timeout> <dest>`
   - Plus `traceroute -n <dest>` for intermediate hops
3. **Snapshot nftables counters** (after) — diff to find which rules were hit
4. **Build hop list** from traceroute output + nftables diffs
5. **Write status** to CRD

### Reconciler (`pkg/controller/traceflow/`)

- On create: set phase=Running, exec probes, write results, set phase=Completed/Failed
- TTL-based cleanup: delete CRs older than `spec.ttl` (default 1h)
- Requeue on Running phase (poll for completion)

### BFF Endpoints

- `GET /api/v1/traceflows` — List all
- `GET /api/v1/traceflows/{name}` — Get with hops
- `POST /api/v1/traceflows` — Create (creates CRD)
- `DELETE /api/v1/traceflows/{name}` — Delete

### Console Plugin

- **TraceflowsListPage**: Table of traces with phase, source, destination, result, latency
- **TraceflowCreatePage**: Source picker (VM dropdown or IP input), destination (IP + port + protocol), router selector
- **TraceflowDetailPage**: Hop-by-hop path visualization (vertical timeline), nftables rule hits, latency per hop

### Router Pod Requirements

Standard mode: `traceroute`, `nping`, `nft` already available (installed by init script).
Fast-path mode: Add `traceroute`, `nmap-ncat` (nping) to Alpine Dockerfile.

## Feature 3: Router XDP/eBPF Wiring

### Dependencies

Add to `cmd/vpc-router/go.mod` (or parent `go.mod`):
```
github.com/cilium/ebpf v0.17.x
```

### eBPF Compilation

Makefile target:
```makefile
bpf-compile:
    clang -target bpf -D__TARGET_ARCH_x86 -O2 \
        -c cmd/vpc-router/bpf/fwd.c \
        -o cmd/vpc-router/bpf/fwd_bpfel.o
```

Embed in Go binary:
```go
//go:embed bpf/fwd_bpfel.o
var xdpProgram []byte
```

### `xdp.go` Implementation

```go
func attachXDP(cfg *Config) (func(), error) {
    // 1. Check kernel version >= 5.8
    // 2. Load eBPF collection from embedded bytes
    // 3. Populate route_table map from cfg.Networks
    // 4. Populate nat_cidrs map from cfg.NftablesConfig (parse SNAT CIDRs)
    // 5. Set flags[0] = 1 if cfg.FirewallConfig != ""
    // 6. Attach XDP program to each workload interface
    // 7. Return cleanup function
}
```

### Graceful Degradation

If XDP attachment fails (unsupported kernel, missing capabilities, eBPF verifier rejection):
- Log warning with reason
- Continue with nftables-only forwarding
- `status.xdpEnabled` reflects actual state (false if attachment failed)
- Health probes still pass (XDP is optimization, not requirement)

### Dockerfile Update

Builder stage adds clang for eBPF compilation:
```dockerfile
RUN apk add --no-cache clang llvm
RUN clang -target bpf -O2 -c cmd/vpc-router/bpf/fwd.c -o cmd/vpc-router/bpf/fwd_bpfel.o
```

### Tests

- Unit tests for map population logic (route parsing, CIDR extraction)
- Mock-based tests for XDP attach/detach lifecycle
- Kernel version check tests

## File Inventory

### Feature 1 (Flow Logs)
| File | Action |
|------|--------|
| `pkg/vpc/flow_logs.go` | **Edit** — Wire 4 methods to VPC SDK |
| `pkg/controller/vpcsubnet/reconciler_test.go` | **Edit** — Add flow log test cases |
| `cmd/bff/internal/handler/flowlog_handler.go` | **New** — BFF handler |
| `cmd/bff/internal/handler/router.go` | **Edit** — Register flow log routes |
| `cmd/bff/internal/handler/subnet.go` | **Edit** — Extend SubnetResponse |
| `console-plugin/src/pages/SubnetDetailPage.tsx` | **Edit** — Add Flow Logs tab |
| `console-plugin/src/pages/SubnetsListPage.tsx` | **Edit** — Add flow log status column |

### Feature 2 (Traceflow)
| File | Action |
|------|--------|
| `api/v1alpha1/vpctraceflow_types.go` | **New** — CRD types |
| `api/v1alpha1/zz_generated.deepcopy.go` | **Edit** — DeepCopy for new types |
| `pkg/controller/traceflow/reconciler.go` | **New** — Reconciler |
| `pkg/controller/traceflow/reconciler_test.go` | **New** — Tests |
| `pkg/controller/traceflow/prober.go` | **New** — Probe execution logic |
| `deploy/helm/.../templates/crds/vpctraceflow-crd.yaml` | **New** — Helm CRD |
| `deploy/helm/.../templates/bff-clusterrole.yaml` | **Edit** — Add traceflow RBAC |
| `cmd/bff/internal/handler/traceflow_handler.go` | **New** — BFF handler |
| `cmd/bff/internal/handler/router.go` | **Edit** — Register routes |
| `console-plugin/src/pages/TraceflowsListPage.tsx` | **New** |
| `console-plugin/src/pages/TraceflowCreatePage.tsx` | **New** |
| `console-plugin/src/pages/TraceflowDetailPage.tsx` | **New** |
| `console-plugin/console-extensions.json` | **Edit** — Add routes |
| `console-plugin/package.json` | **Edit** — Add exposed modules |

### Feature 3 (XDP)
| File | Action |
|------|--------|
| `cmd/vpc-router/xdp.go` | **Edit** — Implement attachXDP() |
| `cmd/vpc-router/xdp_test.go` | **New** — Map population tests |
| `cmd/vpc-router/bpf/fwd.c` | **Edit** — Minor fixes if needed |
| `cmd/vpc-router/Dockerfile` | **Edit** — Add clang for eBPF compilation |
| `Makefile` | **Edit** — Add bpf-compile target |
| `cmd/vpc-router/go.mod` or parent | **Edit** — Add cilium/ebpf dep |
