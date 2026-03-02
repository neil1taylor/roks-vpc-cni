# Router Observability & Monitoring

This tutorial covers enabling live monitoring on VPCRouter pods, viewing metrics in the console plugin, and setting up Prometheus alerts. It walks through enabling the metrics exporter, exploring the monitoring UI, understanding each metric, and configuring alerts.

---

## Overview

Every VPCRouter can run an optional **metrics exporter sidecar** that collects runtime data from inside the router pod and exposes it as Prometheus metrics. When enabled, the operator:

1. Injects a metrics exporter sidecar container into the router pod
2. Creates a `PodMonitor` for automatic Prometheus scraping
3. Makes metrics available through the BFF to the console plugin

### Architecture

```
Router Pod                    OpenShift Monitoring           BFF                    Console Plugin
┌──────────────────┐         ┌─────────────────┐          ┌──────────────┐        ┌─────────────────┐
│ router container │         │ Prometheus       │          │ /metrics/*   │        │ Charts + gauges │
│ metrics-exporter │──:9100──│ (PodMonitor)     │──Thanos──│ endpoints    │──API──▶│ Monitoring tab  │
│ sidecar          │         └─────────────────┘          └──────────────┘        │ Observability   │
└──────────────────┘                                                               └─────────────────┘
```

### What's Collected

| Category | Metrics | Source |
|----------|---------|--------|
| Interface counters | RX/TX bytes, packets, errors, drops per interface | `/proc/net/dev` |
| NFT rule counters | Packets and bytes per nftables rule | `nft -j list ruleset` |
| Connection tracking | Conntrack table entries and max capacity | `/proc/sys/net/netfilter/` |
| DHCP pools | Active leases and pool size per network | dnsmasq lease files |
| Process health | Running status of dnsmasq, suricata, dhclient + uptime | `/proc` scan |

---

## Prerequisites

Before starting this tutorial you need:

1. **A working VPCGateway and VPCRouter** — follow [Gateway & Router Tutorial](gateway-router.md) Parts 3–5 to create these.
2. **`oc` CLI** authenticated to your cluster.
3. **OpenShift Monitoring** enabled on the cluster (default on ROKS).

All examples assume the operator namespace is `roks-vpc-network-operator` and an existing router named `demo-router`. Adjust to match your environment.

---

## 1. Enable Metrics on a Router

Patch your VPCRouter to enable the metrics exporter:

```bash
oc patch vpcrouter demo-router -n default --type=merge -p '
spec:
  metrics:
    enabled: true
'
```

Or create a new router with metrics enabled from the start:

```yaml
apiVersion: vpc.roks.ibm.com/v1alpha1
kind: VPCRouter
metadata:
  name: demo-router
  namespace: default
spec:
  gateway: demo-gw
  networks:
    - name: localnet-1
      address: "10.100.0.1"
  metrics:
    enabled: true
```

The operator recreates the router pod with an additional metrics exporter container.

### Verify the Sidecar

Check that the router pod has 2+ containers (router + metrics-exporter, possibly +suricata):

```bash
oc get pods -n default -l vpc.roks.ibm.com/router=demo-router -o wide
```

Verify the metrics endpoint is responding:

```bash
ROUTER_POD=$(oc get pods -n default -l vpc.roks.ibm.com/router=demo-router -o jsonpath='{.items[0].metadata.name}')
oc exec -n default $ROUTER_POD -c metrics-exporter -- wget -qO- http://localhost:9100/metrics | head -20
```

You should see Prometheus-format metrics like:

```
# HELP router_interface_rx_bytes_total Total bytes received per interface
# TYPE router_interface_rx_bytes_total counter
router_interface_rx_bytes_total{interface="eth0"} 1234567
router_interface_tx_bytes_total{interface="eth0"} 7654321
```

---

## 2. View Metrics in the Console

### Router Detail — Monitoring Tab

Navigate to **IBM VPC Networking > Routers > demo-router**. When metrics are enabled, a **Monitoring** tab appears with:

- **Health summary card** — Router status, uptime, aggregate throughput, process status
- **Conntrack gauge** — Connection tracking table utilization (green < 60%, yellow < 80%, red > 80%)
- **DHCP pool gauges** — One donut chart per network showing lease utilization
- **Interface throughput charts** — RX/TX bytes/sec area charts per interface with time range selector

Use the time range selector (5m / 15m / 1h / 6h / 24h) to zoom in or out. Data auto-refreshes every 15 seconds.

### Router Detail — NFT Rules Tab

The **NFT Rules** tab shows a sortable table of nftables rule hit counters:

| Column | Description |
|--------|-------------|
| Table | nftables table name (e.g., `nat`, `filter`) |
| Chain | Chain name (e.g., `postrouting`, `forward`) |
| Comment | Rule description |
| Packets | Total packets matching this rule |
| Bytes | Total bytes matching this rule |

Click column headers to sort. This is useful for verifying that NAT and firewall rules are active.

### Observability Page

Navigate to **IBM VPC Networking > Observability** for a multi-router overview. This page shows:

- **Router selector** — Switch between all metrics-enabled routers
- **Time range selector** — Shared across all charts
- **Health summary** — Status, uptime, interface throughput rates, process status
- **Conntrack gauge** — Connection tracking utilization
- **DHCP pool gauges** — Per-network lease utilization
- **Interface throughput charts** — Area charts per interface
- **NFT rule counters** — Sortable table of firewall/NAT rule hits

If no routers have metrics enabled, the page shows an empty state with instructions.

### Dashboard — Router Health

The **VPC Dashboard** (`/vpc-networking`) includes a **Router Health** section when any router has metrics enabled. Each metrics-enabled router gets a compact card showing:

- Router name (links to detail page)
- Phase status (green/red icon)
- Namespace
- IDS/IPS mode label (if enabled)

---

## 3. Query Metrics in Prometheus

You can also query router metrics directly in the OpenShift Prometheus UI or via the API.

### Interface Throughput

```promql
# RX bytes/sec for all interfaces on a specific router
rate(router_interface_rx_bytes_total{pod=~"demo-router.*"}[5m])

# Total throughput (RX + TX) across all routers
sum(rate(router_interface_rx_bytes_total[5m]) + rate(router_interface_tx_bytes_total[5m]))
```

### Conntrack Utilization

```promql
# Conntrack table utilization as a percentage
router_conntrack_entries / router_conntrack_max * 100
```

### DHCP Pool Utilization

```promql
# DHCP pools above 80% utilization
router_dhcp_active_leases / router_dhcp_pool_size > 0.8
```

### NFT Rule Activity

```promql
# SNAT rule hit rate
rate(router_nft_rule_packets_total{comment=~".*snat.*"}[5m])
```

---

## 4. Set Up Alerts

Create a `PrometheusRule` to alert on router issues:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: vpc-router-alerts
  namespace: roks-vpc-network-operator
spec:
  groups:
    - name: vpc-router
      rules:
        - alert: RouterConntrackNearFull
          expr: router_conntrack_entries / router_conntrack_max > 0.8
          for: 5m
          labels:
            severity: warning
          annotations:
            summary: "Router conntrack table above 80% on {{ $labels.pod }}"

        - alert: RouterDHCPPoolExhausted
          expr: router_dhcp_active_leases / router_dhcp_pool_size > 0.95
          for: 10m
          labels:
            severity: critical
          annotations:
            summary: "DHCP pool nearly exhausted on {{ $labels.pod }} interface {{ $labels.interface }}"

        - alert: RouterProcessDown
          expr: router_process_running == 0
          for: 2m
          labels:
            severity: critical
          annotations:
            summary: "Process {{ $labels.process }} not running on {{ $labels.pod }}"

        - alert: RouterHighDropRate
          expr: rate(router_interface_rx_drops_total[5m]) > 10
          for: 5m
          labels:
            severity: warning
          annotations:
            summary: "High RX drop rate on {{ $labels.pod }} {{ $labels.interface }}"
```

Apply it:

```bash
oc apply -f router-alerts.yaml
```

---

## 5. Advanced Configuration

### Custom Metrics Port

Override the default metrics exporter port (9100):

```yaml
spec:
  metrics:
    enabled: true
    port: 9200
```

### Custom Exporter Image

Use a custom metrics exporter image:

```yaml
spec:
  metrics:
    enabled: true
    image: "my-registry.io/custom-metrics-exporter:v1"
```

The image resolution order is:
1. `spec.metrics.image` on the VPCRouter CR
2. `METRICS_EXPORTER_IMAGE` environment variable on the operator
3. `metricsExporter.image` Helm value
4. Default image

### Disable PodMonitor

If you have a custom scrape config, disable the Helm-managed PodMonitor:

```yaml
# values.yaml
metricsExporter:
  podMonitor:
    enabled: false
```

---

## 6. Disable Metrics

To disable metrics and remove the sidecar:

```bash
oc patch vpcrouter demo-router -n default --type=merge -p '
spec:
  metrics:
    enabled: false
'
```

The operator recreates the router pod without the metrics exporter container. Historical metrics remain available in Prometheus for the configured retention period.

---

## Troubleshooting

### Metrics Not Appearing

1. **Check pod status**: Ensure the router pod has the metrics-exporter container running:
   ```bash
   oc get pods -l vpc.roks.ibm.com/router=demo-router -o jsonpath='{.items[0].spec.containers[*].name}'
   ```

2. **Check PodMonitor**: Verify the PodMonitor exists:
   ```bash
   oc get podmonitor -n roks-vpc-network-operator
   ```

3. **Check Prometheus targets**: In the OpenShift console, go to Observe > Targets and search for `router-metrics`.

4. **Check BFF connectivity**: The console plugin fetches metrics via the BFF. Verify the BFF has Thanos access:
   ```bash
   oc get clusterrolebinding | grep monitoring-view
   ```

### Console Charts Show "No Data"

- Ensure at least 2 minutes have passed since enabling metrics (Prometheus needs time to collect data points)
- The time range selector must cover the period when metrics were being collected
- Verify the BFF Thanos URL is correct in the Helm values

---

## Next Steps

- [IDS/IPS with Suricata](ids-ips-suricata.md) — Add intrusion detection alongside monitoring
- [Gateway & Router Tutorial](gateway-router.md) — Full gateway and router setup
- [Metrics Reference](../reference/metrics.md) — Complete metrics documentation
