# VPCRouter

The `VPCRouter` custom resource represents a network router that connects workload networks to a VPCGateway for external connectivity. The operator creates a privileged pod with IP forwarding, nftables NAT/firewall, optional DHCP (dnsmasq), optional Suricata IDS/IPS, and optional metrics exporter sidecar.

## API Information

| Field | Value |
|-------|-------|
| Group | `vpc.roks.ibm.com` |
| Version | `v1alpha1` |
| Kind | `VPCRouter` |
| Short Name | `vrt` |
| Scope | Namespaced |

## Spec

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `gateway` | `string` | Yes | - | Name of the VPCGateway this router is associated with. Must exist in the same namespace. |
| `transit` | [`RouterTransit`](#routertransit) | No | - | Transit network configuration for the router. |
| `networks` | [`[]RouterNetwork`](#routernetwork) | Yes (min 1) | - | List of workload networks attached to the router. |
| `routeAdvertisement` | [`RouteAdvertisement`](#routeadvertisement) | No | - | Controls which routes the router advertises to the gateway. |
| `dhcp` | [`RouterDHCP`](#routerdhcp) | No | - | Global DHCP server configuration. |
| `firewall` | [`GatewayFirewall`](#gatewayfirewall) | No | - | Firewall rules for traffic filtering. |
| `ids` | [`RouterIDS`](#routerids) | No | - | Suricata IDS/IPS sidecar configuration. |
| `metrics` | [`RouterMetrics`](#routermetrics) | No | - | Metrics exporter sidecar configuration. |
| `pod` | [`RouterPodSpec`](#routerpodspec) | No | - | Pod-level overrides (image, resources, scheduling). |

### RouterTransit

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `network` | `string` | No | - | Name of the transit L2 network. |
| `address` | `string` | Yes | - | IP address on the transit network. |

### RouterNetwork

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `name` | `string` | Yes | - | Name of the network (CUDN or UDN). |
| `namespace` | `string` | No | - | Namespace of the network-attachment-definition (if different from router namespace). |
| `address` | `string` | Yes | - | IP address on this network. |
| `dhcp` | [`NetworkDHCP`](#networkdhcp) | No | - | Per-network DHCP overrides. |

### RouteAdvertisement

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `connectedSegments` | `bool` | No | `true` | Advertise routes for directly connected network segments. |
| `staticRoutes` | `bool` | No | `false` | Advertise configured static routes. |
| `natIPs` | `bool` | No | `false` | Advertise NAT-translated IP addresses. |

### RouterDHCP

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `enabled` | `bool` | No | `false` | Enable the router as a DHCP server. |
| `leaseTime` | `string` | No | `12h` | Default DHCP lease duration (e.g., `12h`, `1h`, `30m`). |
| `dns` | [`DHCPDNSConfig`](#dhcpdnsconfig) | No | - | DNS settings for DHCP responses. |
| `options` | [`DHCPOptions`](#dhcpoptions) | No | - | Additional DHCP options. |

### NetworkDHCP

Per-network overrides for DHCP configuration.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `enabled` | `*bool` | No | - | Override global DHCP enabled setting for this network. |
| `range` | [`NetworkDHCPRange`](#networkdhcprange) | No | - | Custom DHCP address range. |
| `leaseTime` | `string` | No | - | Override global lease duration. |
| `reservations` | [`[]DHCPStaticReservation`](#dhcpstaticreservation) | No | - | Static MAC-to-IP reservations. |
| `dns` | [`DHCPDNSConfig`](#dhcpdnsconfig) | No | - | Override global DNS settings. |
| `options` | [`DHCPOptions`](#dhcpoptions) | No | - | Override global DHCP options. |

### NetworkDHCPRange

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `start` | `string` | Yes | - | First IP address in the DHCP pool. |
| `end` | `string` | Yes | - | Last IP address in the DHCP pool. |

### DHCPStaticReservation

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `mac` | `string` | Yes | - | Hardware address (e.g., `fa:16:3e:aa:bb:cc`). |
| `ip` | `string` | Yes | - | Reserved IP address (e.g., `10.100.0.50`). |
| `hostname` | `string` | No | - | Optional hostname for the reservation. |

### DHCPDNSConfig

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `nameservers` | `[]string` | No | - | DNS server IP addresses (DHCP option 6). |
| `searchDomains` | `[]string` | No | - | DNS search domains (DHCP option 119). |
| `localDomain` | `string` | No | - | Local domain name for DHCP clients. |

### DHCPOptions

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `router` | `string` | No | - | Override default gateway (DHCP option 3). |
| `mtu` | `*int32` | No | - | Interface MTU (DHCP option 26). |
| `ntpServers` | `[]string` | No | - | NTP server addresses (DHCP option 42). |
| `custom` | `[]string` | No | - | Raw dnsmasq `--dhcp-option` values for passthrough. |

### GatewayFirewall

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `enabled` | `bool` | No | `false` | Enable the firewall. |
| `rules` | `[]FirewallRule` | No | - | Ordered list of firewall rules. |

Each `FirewallRule` has: `name` (string), `action` (`allow`/`deny`), `direction` (`in`/`out`/`forward`), `protocol` (`tcp`/`udp`/`icmp`/`any`), `sourceIP` (CIDR), `destIP` (CIDR), `port` (int), `portRange` (string).

### RouterIDS

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `enabled` | `bool` | No | `false` | Deploy the Suricata IDS/IPS sidecar. |
| `mode` | `string` | No | `ids` | Inspection mode: `ids` (passive AF_PACKET) or `ips` (inline NFQUEUE). |
| `interfaces` | `string` | No | `all` | Which interfaces to monitor: `all`, `uplink`, or `workload`. |
| `customRules` | `string` | No | - | Additional Suricata rules (one per line). |
| `syslogTarget` | `string` | No | - | Syslog destination (`host:port`) for EVE JSON alerts. |
| `image` | `string` | No | - | Override the Suricata container image. |
| `nfqueueNum` | `*int32` | No | `0` | NFQUEUE number for IPS mode. |

### RouterMetrics

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `enabled` | `bool` | No | `false` | Deploy the metrics exporter sidecar. |
| `port` | `*int32` | No | `9100` | Port the metrics exporter listens on. |
| `image` | `string` | No | - | Override the metrics exporter container image. |

### RouterPodSpec

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `image` | `string` | No | - | Override the router container image. |
| `resources` | `ResourceRequirements` | No | - | CPU and memory requests/limits. |
| `nodeSelector` | `map[string]string` | No | - | Constrain router pod to matching nodes. |
| `tolerations` | `[]Toleration` | No | - | Allow scheduling on tainted nodes. |
| `runtimeClassName` | `*string` | No | - | RuntimeClass for CPU pinning or other runtime features. |
| `priorityClassName` | `string` | No | - | PriorityClass for pod scheduling priority. |

## Status

| Field | Type | Description |
|-------|------|-------------|
| `phase` | `string` | Current lifecycle phase: `Pending`, `Provisioning`, `Ready`, or `Error`. |
| `podIP` | `string` | Cluster IP address of the router pod. |
| `transitIP` | `string` | Router's IP address on the transit network. |
| `idsMode` | `string` | Active IDS/IPS mode (`ids`, `ips`, or empty if disabled). |
| `metricsEnabled` | `bool` | Whether the metrics exporter sidecar is active. |
| `networks` | `[]RouterNetworkStatus` | Status of each attached network. |
| `advertisedRoutes` | `[]string` | Routes currently advertised to the gateway. |
| `syncStatus` | `string` | Sync state: `Synced`, `Pending`, or `Failed`. |
| `lastSyncTime` | `*metav1.Time` | Timestamp of the last successful sync. |
| `message` | `string` | Human-readable detail about the current status. |
| `conditions` | `[]metav1.Condition` | Standard Kubernetes conditions. |

### RouterNetworkStatus

| Field | Type | Description |
|-------|------|-------------|
| `name` | `string` | Network name. |
| `address` | `string` | Router's IP address on this network. |
| `connected` | `bool` | Whether the router has connectivity to this network. |
| `dhcp` | `DHCPNetworkStatus` | DHCP status (enabled, poolStart, poolEnd, reservationCount). |

## kubectl Output Columns

When you run `kubectl get vpcrouters` (or `kubectl get vrt`):

| Column | Source | Priority | Description |
|--------|--------|----------|-------------|
| Gateway | `spec.gateway` | 0 | Associated VPCGateway name. |
| Phase | `status.phase` | 0 | Current lifecycle phase. |
| Sync | `status.syncStatus` | 0 | Sync status. |
| Age | `metadata.creationTimestamp` | 0 | Time since creation. |
| IDS | `status.idsMode` | 1 | Active IDS/IPS mode (wide output only). |

## Example

### Minimal Router

```yaml
apiVersion: vpc.roks.ibm.com/v1alpha1
kind: VPCRouter
metadata:
  name: my-router
  namespace: default
spec:
  gateway: my-gateway
  networks:
    - name: localnet-1
      address: "10.100.0.1"
```

### Full-Featured Router

```yaml
apiVersion: vpc.roks.ibm.com/v1alpha1
kind: VPCRouter
metadata:
  name: production-router
  namespace: default
spec:
  gateway: prod-gateway
  networks:
    - name: app-network
      address: "10.100.0.1"
      dhcp:
        enabled: true
        range:
          start: "10.100.0.100"
          end: "10.100.0.200"
        reservations:
          - mac: "fa:16:3e:aa:bb:cc"
            ip: "10.100.0.50"
            hostname: "db-server"
    - name: mgmt-network
      address: "10.200.0.1"
      dhcp:
        enabled: false
  dhcp:
    enabled: true
    leaseTime: "6h"
    dns:
      nameservers:
        - "161.26.0.7"
        - "161.26.0.8"
      searchDomains:
        - "cluster.local"
  firewall:
    enabled: true
    rules:
      - name: allow-ssh
        action: allow
        direction: in
        protocol: tcp
        port: 22
      - name: deny-all-in
        action: deny
        direction: in
        protocol: any
  ids:
    enabled: true
    mode: ips
  metrics:
    enabled: true
  pod:
    resources:
      requests:
        cpu: "500m"
        memory: "512Mi"
      limits:
        cpu: "2"
        memory: "1Gi"
    nodeSelector:
      node.kubernetes.io/instance-type: bx2d-metal-96x384
    priorityClassName: system-node-critical
```

## Lifecycle

1. The VPCRouter reconciler detects a new CR.
2. It resolves the referenced VPCGateway to obtain the transit network and NAT configuration.
3. A privileged pod is created with Multus network attachments for the transit and workload networks.
4. The pod runs an init script that configures IP forwarding, nftables NAT/firewall rules, and optional dnsmasq.
5. If `ids.enabled: true`, a Suricata sidecar is added (IDS mode uses AF_PACKET; IPS uses NFQUEUE).
6. If `metrics.enabled: true`, a metrics exporter sidecar is added on port 9100.
7. The reconciler watches the gateway for config changes and recreates the pod on drift.
8. Route advertisements flow from the router to the gateway, which creates corresponding VPC routes.
9. On deletion, the finalizer (`vpc.roks.ibm.com/router-cleanup`) deletes the pod and ConfigMaps.

## Related Resources

- [VPCGateway](../crds/) — The gateway that provides the VPC uplink for this router.
- [Observability Tutorial](../../tutorials/observability.md) — Enabling and using router metrics.
- [IDS/IPS Tutorial](../../tutorials/ids-ips-suricata.md) — Configuring Suricata on a router.
- [Metrics Reference](../metrics.md) — All router pod metrics.
