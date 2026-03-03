# Fast-Path Router Mode

The VPCRouter supports two runtime modes:

| Aspect | Standard | Fast-path |
|--------|----------|-----------|
| Image | Fedora 40 (~1GB) | Alpine + Go binary (~50MB) |
| Startup | ~30s (dnf install at boot) | ~2s |
| Forwarding | Kernel nftables | XDP/eBPF + kernel fallback |
| Command | `/bin/bash -c {init-script}` | `/vpc-router` |
| Health probes | Exec (sysctl, ip) | HTTP (`/healthz`, `/readyz`) |
| PPS (simple L3) | Baseline | 10-100x improvement |

Both modes support all features: DHCP, NAT, firewall, IDS/IPS sidecar, metrics sidecar, and VPN route advertisement.

## When to Use Fast-Path

- **High-throughput workloads** — VMs generating >100k PPS of simple L3 traffic
- **Low-latency requirements** — Bypass kernel network stack for forwarded packets
- **Resource-constrained clusters** — 50MB image vs ~1GB, 2s startup vs 30s
- **Production deployments** — Deterministic Go binary vs shell script

Standard mode is recommended for development, debugging, and when you need to customize the init script.

## Creating a Fast-Path Router

```yaml
apiVersion: vpc.roks.ibm.com/v1alpha1
kind: VPCRouter
metadata:
  name: router-fastpath
  namespace: default
spec:
  gateway: my-gateway
  mode: fast-path
  networks:
  - name: localnet-app
    address: 10.100.0.1/24
  - name: localnet-db
    address: 10.200.0.1/24
  routeAdvertisement:
    connectedSegments: true
  dhcp:
    enabled: true
    leaseTime: "12h"
```

The only difference from a standard router is `spec.mode: fast-path`.

## Verifying Fast-Path Mode

```bash
# Check router status
oc get vrt router-fastpath -o wide
# MODE column shows "fast-path" (use -o wide or --show-labels for priority=1 columns)

# Check pod is using the Go binary
oc get pod router-fastpath-pod -o jsonpath='{.spec.containers[0].command}'
# Should show: ["/vpc-router"]

# Check XDP status
oc get vrt router-fastpath -o jsonpath='{.status.xdpEnabled}'
# Should show: true

# Check health endpoints
oc exec router-fastpath-pod -- wget -qO- http://localhost:8080/healthz
# Should show: ok

oc exec router-fastpath-pod -- wget -qO- http://localhost:8080/readyz
# Should show: ready
```

## Fast-Path with Sidecars

The IDS/IPS (Suricata) and metrics exporter sidecars work identically in both modes:

```yaml
apiVersion: vpc.roks.ibm.com/v1alpha1
kind: VPCRouter
metadata:
  name: router-fp-full
spec:
  gateway: my-gateway
  mode: fast-path
  networks:
  - name: localnet-app
    address: 10.100.0.1/24
  dhcp:
    enabled: true
  ids:
    enabled: true
    mode: ips
  metrics:
    enabled: true
```

This creates a 3-container pod: `router` (Go binary), `suricata` (IPS sidecar), `metrics-exporter`.

## Switching Between Modes

Changing `spec.mode` triggers automatic pod recreation via drift detection:

```bash
# Switch from standard to fast-path
oc patch vrt my-router --type=merge -p '{"spec":{"mode":"fast-path"}}'

# Switch back to standard
oc patch vrt my-router --type=merge -p '{"spec":{"mode":"standard"}}'
```

The old pod is deleted and a new one is created with the appropriate image and command. There is a brief connectivity interruption during the switch.

## XDP Fallback

If XDP/eBPF attachment fails (e.g., older kernel, missing BPF capabilities), the Go binary logs a warning and continues with kernel-only forwarding. All functionality remains available — XDP is purely a performance optimization.

## Image Configuration

The fast-path image defaults to `de.icr.io/roks/vpc-router-fastpath:latest`. Override via Helm:

```yaml
routerPod:
  fastpathImage: "my-registry.example.com/vpc-router-fastpath:v1.0"
```

Or via environment variable on the operator deployment:

```
ROUTER_POD_FASTPATH_IMAGE=my-registry.example.com/vpc-router-fastpath:v1.0
```
