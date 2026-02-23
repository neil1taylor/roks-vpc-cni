# Metrics Reference

The VPC Network Operator exposes Prometheus metrics for monitoring VPC API operations, resource counts, and garbage collection activity.

---

## VPC API Metrics

### vpc_api_calls_total

**Type:** Counter
**Description:** Total number of VPC API calls made by the operator.
**Labels:**

| Label | Values | Description |
|-------|--------|-------------|
| `operation` | `create_subnet`, `delete_subnet`, `get_subnet`, `create_vni`, `delete_vni`, `get_vni`, `list_vnis_by_tag`, `create_vlan_attachment`, `delete_vlan_attachment`, `list_vlan_attachments`, `create_floating_ip`, `delete_floating_ip`, `get_floating_ip` | VPC API operation name |
| `status` | `success`, `error` | Result of the API call |

**Example queries:**
```promql
# Total successful VNI creations
vpc_api_calls_total{operation="create_vni", status="success"}

# Error rate over 5 minutes
rate(vpc_api_calls_total{status="error"}[5m])
```

### vpc_api_errors_total

**Type:** Counter
**Description:** Total number of VPC API errors, broken down by error type.
**Labels:**

| Label | Values | Description |
|-------|--------|-------------|
| `operation` | Same as `vpc_api_calls_total` | VPC API operation |
| `error_type` | `rate_limited`, `not_found`, `conflict`, `unauthorized`, `server_error`, `timeout` | Category of error |

**Example queries:**
```promql
# Rate limiting incidents over time
vpc_api_errors_total{error_type="rate_limited"}

# All errors for subnet operations
vpc_api_errors_total{operation=~".*subnet.*"}
```

### vpc_api_latency_seconds

**Type:** Histogram
**Description:** Latency of VPC API calls in seconds.
**Labels:**

| Label | Values | Description |
|-------|--------|-------------|
| `operation` | Same as `vpc_api_calls_total` | VPC API operation |

**Buckets:** 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0, 30.0

**Example queries:**
```promql
# P99 latency for VNI creation
histogram_quantile(0.99, rate(vpc_api_latency_seconds_bucket{operation="create_vni"}[5m]))

# Average latency across all operations
rate(vpc_api_latency_seconds_sum[5m]) / rate(vpc_api_latency_seconds_count[5m])
```

---

## Resource Count Metrics

### vni_count

**Type:** Gauge
**Description:** Current number of VNIs managed by the operator (tracked via CRDs or annotations).

**Example query:**
```promql
vni_count
```

### subnet_count

**Type:** Gauge
**Description:** Current number of VPC subnets managed by the operator.

**Example query:**
```promql
subnet_count
```

---

## Garbage Collection Metrics

### orphan_gc_deleted_total

**Type:** Counter
**Description:** Total number of orphaned VPC resources deleted by the garbage collector.
**Labels:**

| Label | Values | Description |
|-------|--------|-------------|
| `resource_type` | `vni`, `floating_ip`, `subnet`, `vlan_attachment` | Type of orphaned resource |

**Example queries:**
```promql
# Total orphaned VNIs cleaned up
orphan_gc_deleted_total{resource_type="vni"}

# Orphan cleanup rate
rate(orphan_gc_deleted_total[1h])
```

---

## Reconciler Metrics

Controller-runtime provides built-in reconciler metrics:

### controller_runtime_reconcile_total

**Type:** Counter
**Description:** Total number of reconciliations per controller.
**Labels:** `controller`, `result` (`success`, `error`, `requeue`, `requeue_after`)

```promql
# Error rate for the CUDN reconciler
rate(controller_runtime_reconcile_total{controller="cudn", result="error"}[5m])
```

### controller_runtime_reconcile_time_seconds

**Type:** Histogram
**Description:** Time taken per reconciliation.
**Labels:** `controller`

```promql
# P95 reconciliation time for VNI controller
histogram_quantile(0.95, rate(controller_runtime_reconcile_time_seconds_bucket{controller="vni"}[5m]))
```

### workqueue_depth

**Type:** Gauge
**Description:** Current depth of the work queue per controller.
**Labels:** `name`

```promql
# All controller queue depths
workqueue_depth
```

---

## Webhook Metrics

### controller_runtime_webhook_requests_total

**Type:** Counter
**Description:** Total number of admission webhook requests.
**Labels:** `webhook`, `code` (HTTP status code)

```promql
# Webhook rejection rate
rate(controller_runtime_webhook_requests_total{code!="200"}[5m])
```

### controller_runtime_webhook_latency_seconds

**Type:** Histogram
**Description:** Latency of webhook request processing.

```promql
# P99 webhook latency
histogram_quantile(0.99, rate(controller_runtime_webhook_latency_seconds_bucket[5m]))
```

---

## Scrape Configuration

The operator exposes metrics on port `8080` at `/metrics`. To scrape with Prometheus on OpenShift, create a `ServiceMonitor`:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: vpc-network-operator
  namespace: roks-vpc-network-operator
spec:
  selector:
    matchLabels:
      app: vpc-network-operator
  endpoints:
  - port: metrics
    interval: 30s
    path: /metrics
```

---

## Alerting Examples

### High VPC API Error Rate

```yaml
alert: VPCAPIHighErrorRate
expr: rate(vpc_api_errors_total[5m]) > 0.1
for: 10m
labels:
  severity: warning
annotations:
  summary: "VPC API error rate exceeds threshold"
```

### Orphan Resources Accumulating

```yaml
alert: OrphanResourcesAccumulating
expr: rate(orphan_gc_deleted_total[1h]) > 5
for: 30m
labels:
  severity: warning
annotations:
  summary: "High rate of orphaned VPC resources being cleaned up"
```

### Webhook Latency

```yaml
alert: WebhookHighLatency
expr: histogram_quantile(0.99, rate(controller_runtime_webhook_latency_seconds_bucket[5m])) > 10
for: 5m
labels:
  severity: critical
annotations:
  summary: "VM admission webhook P99 latency exceeds 10 seconds"
```
