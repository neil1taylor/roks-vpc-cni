# IDS/IPS with Suricata on VPCRouter

This tutorial covers adding Suricata-based intrusion detection (IDS) and inline intrusion prevention (IPS) to a VPCRouter. It walks through enabling passive monitoring, switching to inline blocking, writing custom rules, selecting interfaces, configuring syslog output, and troubleshooting.

---

## Overview

Every VPCRouter can run an optional **Suricata 7.0 sidecar container** that inspects traffic flowing through the router. Two modes are available:

| Mode | Engine | Behavior |
|------|--------|----------|
| **IDS** (default) | AF_PACKET | Passive monitoring — mirrors packets for analysis, generates alerts, never drops traffic |
| **IPS** | NFQUEUE | Inline inspection — forwarded packets pass through Suricata; `drop` rules actively block traffic |

IPS mode uses the `bypass` (fail-open) flag, so traffic continues to flow even if Suricata stalls or crashes.

### Architecture

```
                          Router Pod
  ┌────────────────────────────────────────────────┐
  │                                                │
  │   ┌──────────┐         ┌──────────────────┐    │
  │   │  router   │ ──────▶│    suricata       │    │
  │   │ container │         │    sidecar        │    │
  │   └──────────┘         └──────────────────┘    │
  │        │                     │                 │
  │   IDS: AF_PACKET (passive mirror)              │
  │   IPS: NFQUEUE (inline, fail-open)             │
  │                                                │
  └────────────────────────────────────────────────┘

  VM traffic ──▶ workload interface ──▶ router ──▶ Suricata ──▶ EVE JSON logs
                                         │
                                         ▼
                                   uplink / internet
```

- **EVE JSON** logs stream to stdout for collection by IBM Cloud Logging (or any log aggregator)
- **ET Open** rules are fetched automatically via `suricata-update` at container start
- **Custom rules** are injected via the `ids.customRules` field

---

## Prerequisites

Before starting this tutorial you need:

1. **A working VPCGateway and VPCRouter** — follow [Gateway & Router Tutorial](gateway-router.md) Parts 3–5 to create these.
2. **A VM on a connected Layer2 network** — for generating traffic to test alerts. See [Gateway & Router Tutorial](gateway-router.md) Part 6 for VM creation.
3. **`oc` CLI** authenticated to your cluster.

All examples assume the operator namespace is `roks-vpc-network-operator` and an existing gateway named `demo-gw-routes`. Adjust to match your environment.

---

## 1. Enable IDS (Passive Monitoring)

Create or patch your VPCRouter to enable Suricata in IDS mode:

```yaml
# demo-router-ids.yaml
apiVersion: vpc.roks.ibm.com/v1alpha1
kind: VPCRouter
metadata:
  name: demo-router-ids
  namespace: roks-vpc-network-operator
spec:
  gateway: demo-gw-routes
  networks:
  - name: demo-l2
    namespace: vm-demo
    address: "10.100.0.1/24"
  dhcp:
    enabled: true
  ids:
    enabled: true
    mode: ids
```

Apply and wait for the router to reach Ready:

```bash
oc apply -f demo-router-ids.yaml
oc wait --for=jsonpath='{.status.phase}'=Ready vpcrouter/demo-router-ids \
  -n roks-vpc-network-operator --timeout=120s
```

### Verify IDS is active

**Check the IDS printer column** (visible with `-o wide` since IDS has `priority=1`):

```bash
oc get vrt -n roks-vpc-network-operator -o wide
```

Expected output:

```
NAME              GATEWAY          PHASE   SYNC     AGE   IDS
demo-router-ids   demo-gw-routes   Ready   Synced   30s   ids
```

**Check the pod has 2 containers** (router + suricata):

```bash
oc get pod demo-router-ids-pod -n roks-vpc-network-operator
```

Expected output:

```
NAME                    READY   STATUS    RESTARTS   AGE
demo-router-ids-pod     2/2     Running   0          30s
```

**Check `status.idsMode`**:

```bash
oc get vpcrouter demo-router-ids -n roks-vpc-network-operator \
  -o jsonpath='{.status.idsMode}'
# Output: ids
```

**Check the `IDSReady` condition**:

```bash
oc get vpcrouter demo-router-ids -n roks-vpc-network-operator \
  -o jsonpath='{.status.conditions[?(@.type=="IDSReady")].status}'
# Output: True
```

### View EVE JSON logs

Suricata streams EVE JSON to stdout via `tail -F /var/log/suricata/eve.json`:

```bash
oc logs demo-router-ids-pod -n roks-vpc-network-operator -c suricata --tail=10
```

You should see JSON lines with `event_type` fields like `stats`, `flow`, etc.

### Trigger a test alert

From a VM behind this router, run:

```bash
# Via virtctl ssh into the VM
curl http://testmynids.org/uid/index.html
```

Then check for the alert in Suricata logs:

```bash
oc logs demo-router-ids-pod -n roks-vpc-network-operator -c suricata | grep '"alert"'
```

You should see an alert with `signature: "ET POLICY curl User-Agent Outbound"` or similar ET Open rule match.

---

## 2. Switch to IPS (Inline Blocking)

Patch the router to switch from passive IDS to inline IPS:

```bash
# Record current pod UID
OLD_UID=$(oc get pod demo-router-ids-pod -n roks-vpc-network-operator \
  -o jsonpath='{.metadata.uid}')

# Switch to IPS mode
oc patch vpcrouter demo-router-ids -n roks-vpc-network-operator --type=merge \
  -p '{"spec":{"ids":{"mode":"ips"}}}'
```

The router reconciler detects the mode change and **recreates the pod**:

```bash
sleep 10
oc wait --for=condition=Ready pod/demo-router-ids-pod \
  -n roks-vpc-network-operator --timeout=120s

# Verify pod was recreated (different UID)
NEW_UID=$(oc get pod demo-router-ids-pod -n roks-vpc-network-operator \
  -o jsonpath='{.metadata.uid}')
if [ "$OLD_UID" != "$NEW_UID" ]; then
  echo "PASS: Pod recreated for IPS"
else
  echo "FAIL: Pod not recreated"
fi
```

### Verify IPS is active

**Check `status.idsMode`**:

```bash
oc get vpcrouter demo-router-ids -n roks-vpc-network-operator \
  -o jsonpath='{.status.idsMode}'
# Output: ips
```

**Check NFQUEUE nftables rules** on the router container:

```bash
oc exec demo-router-ids-pod -n roks-vpc-network-operator -c router \
  -- nft list table ip suricata
```

Expected output:

```
table ip suricata {
  chain forward_ips {
    type filter hook forward priority -10; policy accept;
    ct state established,related accept
    queue num 0 bypass
  }
}
```

Key details:
- **`priority -10`** — runs before the default forward chain
- **`ct state established,related accept`** — only new connections go through Suricata
- **`queue num 0 bypass`** — NFQUEUE 0 with fail-open; if Suricata is unavailable, packets are accepted rather than dropped

---

## 3. Custom Rules

Add custom Suricata rules to detect or block specific traffic patterns. Rules use standard Suricata syntax — `alert` generates a log entry, `drop` (IPS mode only) blocks the packet.

### Example: block reverse shells and detect scanners

```bash
oc patch vpcrouter demo-router-ids -n roks-vpc-network-operator --type=merge -p '
{
  "spec": {
    "ids": {
      "customRules": "drop tcp any any -> any 4444 (msg:\"Block reverse shell port\"; sid:1000001; rev:1;)\nalert http any any -> any any (msg:\"Suspicious user agent\"; content:\"nikto\"; http_user_agent; sid:1000002; rev:1;)"
    }
  }
}'
```

The pod is recreated when custom rules change. Wait for it:

```bash
sleep 10
oc wait --for=condition=Ready pod/demo-router-ids-pod \
  -n roks-vpc-network-operator --timeout=120s
```

### Verify rules are loaded

```bash
oc exec demo-router-ids-pod -n roks-vpc-network-operator -c suricata \
  -- cat /var/lib/suricata/rules/custom.rules
```

Expected output:

```
drop tcp any any -> any 4444 (msg:"Block reverse shell port"; sid:1000001; rev:1;)
alert http any any -> any any (msg:"Suspicious user agent"; content:"nikto"; http_user_agent; sid:1000002; rev:1;)
```

### Test a custom rule

From a VM, try to connect to port 4444 on any host — in IPS mode the connection will be blocked. In IDS mode, an alert is generated but traffic is not dropped.

> **Tip**: Use SIDs starting at 1000001 to avoid conflicts with ET Open rules. Each rule needs a unique `sid`.

---

## 4. Interface Selection

By default, Suricata monitors **all** interfaces (uplink + workload networks). You can restrict monitoring to specific interface groups:

| Value | Interfaces monitored | Use case |
|-------|---------------------|----------|
| `all` (default) | `uplink` + `net0`, `net1`, ... | Full visibility — north-south and east-west |
| `uplink` | Only the transit/uplink interface | North-south traffic only (internet-facing) |
| `workload` | Only workload networks (`net0`, `net1`, ...) | East-west traffic only (VM-to-VM) |

### Example: monitor only north-south traffic

```bash
oc patch vpcrouter demo-router-ids -n roks-vpc-network-operator --type=merge \
  -p '{"spec":{"ids":{"interfaces":"uplink"}}}'
```

### Example: monitor only east-west traffic

```bash
oc patch vpcrouter demo-router-ids -n roks-vpc-network-operator --type=merge \
  -p '{"spec":{"ids":{"interfaces":"workload"}}}'
```

> **Note**: In IDS mode (AF_PACKET), interface selection determines which interfaces get packet capture. In IPS mode (NFQUEUE), all forwarded traffic goes through the NFQUEUE regardless of interface selection — the NFQUEUE hooks the kernel's forward chain.

---

## 5. Syslog Output

Forward Suricata alerts to an external syslog server for centralized security monitoring:

```bash
oc patch vpcrouter demo-router-ids -n roks-vpc-network-operator --type=merge \
  -p '{"spec":{"ids":{"syslogTarget":"syslog.example.com:514"}}}'
```

This adds a second EVE log output using the syslog filetype. It sends alert events via syslog **in addition to** the default file-based EVE JSON streamed to stdout.

Syslog output configuration:
- **Identity**: `suricata-<mode>` (e.g., `suricata-ips`)
- **Facility**: `local5`
- **Level**: `info`
- **Event types**: alerts only (not flow/stats/dns/etc.)

To remove syslog output, clear the field:

```bash
oc patch vpcrouter demo-router-ids -n roks-vpc-network-operator --type=merge \
  -p '{"spec":{"ids":{"syslogTarget":""}}}'
```

---

## 6. Full IPS Example

A complete VPCRouter with all IDS/IPS features enabled:

```yaml
# demo-router-ips-full.yaml
apiVersion: vpc.roks.ibm.com/v1alpha1
kind: VPCRouter
metadata:
  name: demo-router-ips
  namespace: roks-vpc-network-operator
spec:
  gateway: demo-gw-routes
  networks:
  - name: demo-l2
    namespace: vm-demo
    address: "10.100.0.1/24"
  dhcp:
    enabled: true
  ids:
    enabled: true
    mode: ips
    interfaces: all
    syslogTarget: "syslog.example.com:514"
    customRules: |
      drop tcp any any -> any 4444 (msg:"Block reverse shell port"; sid:1000001; rev:1;)
      alert http any any -> any any (msg:"Suspicious user agent"; content:"nikto"; http_user_agent; sid:1000002; rev:1;)
```

Apply:

```bash
oc apply -f demo-router-ips-full.yaml
oc wait --for=jsonpath='{.status.phase}'=Ready vpcrouter/demo-router-ips \
  -n roks-vpc-network-operator --timeout=120s
```

---

## 7. CRD Reference

### `RouterIDS` Spec Fields

The `ids` field on `VPCRouterSpec` accepts a `RouterIDS` object:

| Field | Type | Default | Required | Description |
|-------|------|---------|----------|-------------|
| `enabled` | `bool` | `false` | Yes | Controls whether the Suricata sidecar is deployed |
| `mode` | `string` | `ids` | Yes | Inspection mode: `ids` (AF_PACKET) or `ips` (NFQUEUE) |
| `interfaces` | `string` | `all` | No | Which interfaces to monitor: `all`, `uplink`, or `workload` |
| `customRules` | `string` | — | No | Additional Suricata rules (one rule per line) |
| `syslogTarget` | `string` | — | No | Syslog destination (`host:port`) for EVE JSON alerts |
| `image` | `string` | — | No | Override the Suricata container image (default: `docker.io/jasonish/suricata:7.0`) |
| `nfqueueNum` | `*int32` | `0` | No | NFQUEUE number used in IPS mode |

### Image Resolution Order

1. `ids.image` (per-router override)
2. `SURICATA_IMAGE` environment variable (operator-level override)
3. `docker.io/jasonish/suricata:7.0` (default)

### Status Fields

| Field | Description |
|-------|-------------|
| `status.idsMode` | Active IDS/IPS mode: `ids`, `ips`, or empty if disabled |

### Conditions

| Type | Meaning |
|------|---------|
| `IDSReady` | `True` when Suricata sidecar is running and healthy |

### Printer Columns

The `IDS` column is a `priority=1` printer column, visible with `-o wide`:

```bash
oc get vrt -o wide
```

```
NAME              GATEWAY          PHASE   SYNC     AGE   IDS
demo-router-ids   demo-gw-routes   Ready   Synced   5m    ips
```

---

## 8. Troubleshooting

### Suricata alerts

View all EVE JSON alerts:

```bash
oc logs <pod-name> -n roks-vpc-network-operator -c suricata | grep '"event_type":"alert"'
```

Filter for a specific signature:

```bash
oc logs <pod-name> -n roks-vpc-network-operator -c suricata \
  | grep '"alert"' | jq -r '.alert.signature'
```

### View Suricata configuration

```bash
oc exec <pod-name> -n roks-vpc-network-operator -c suricata \
  -- cat /etc/suricata/suricata.yaml
```

### Verify custom rules

```bash
oc exec <pod-name> -n roks-vpc-network-operator -c suricata \
  -- cat /var/lib/suricata/rules/custom.rules
```

### Check NFQUEUE rules (IPS mode)

```bash
oc exec <pod-name> -n roks-vpc-network-operator -c router \
  -- nft list table ip suricata
```

If this returns "No such file or directory", the router is not in IPS mode — NFQUEUE rules are only created when `mode: ips`.

### Pod not showing 2/2 containers

- Verify `ids.enabled: true` is set on the VPCRouter
- Check the Suricata image can be pulled: `oc describe pod <pod-name>` and look for image pull errors
- The default image `docker.io/jasonish/suricata:7.0` requires internet access — if your cluster has restricted egress, override with `ids.image` pointing to a mirrored image

### NFQUEUE rules missing in IPS mode

- Confirm the router is in IPS mode: `oc get vrt <name> -o wide` should show `ips` in the IDS column
- Check router container logs for nftables errors: `oc logs <pod-name> -c router`
- The router container must have `NET_ADMIN` capability (granted automatically by the privileged pod spec)

### No alerts appearing

- Ensure traffic is flowing through the router (not bypassing it)
- In IDS mode, verify AF_PACKET is binding to the correct interfaces — check Suricata startup logs
- Try the test alert: `curl http://testmynids.org/uid/index.html` from a VM behind the router
- Check that `suricata-update` ran successfully at startup: `oc logs <pod-name> -c suricata | head -20`

---

## 9. Disable IDS/IPS

Remove the `ids` field entirely or set `enabled: false`:

```bash
oc patch vpcrouter demo-router-ids -n roks-vpc-network-operator --type=merge \
  -p '{"spec":{"ids":{"enabled":false}}}'
```

The pod is recreated with a single container (router only):

```bash
sleep 10
oc wait --for=condition=Ready pod/demo-router-ids-pod \
  -n roks-vpc-network-operator --timeout=120s

# Verify single container
oc get pod demo-router-ids-pod -n roks-vpc-network-operator
# READY: 1/1
```

The `status.idsMode` field is cleared and the `IDS` printer column becomes empty.
