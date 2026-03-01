# VPCL2Bridge Design Document

**Date**: 2026-03-01
**Status**: Approved
**Scope**: CRD, Reconciler, Console Plugin UI, Unit Tests, Tutorial

## Overview

The VPCL2Bridge feature enables Layer 2 connectivity between remote network segments (VMware NSX-T, on-prem, multi-cloud) and OpenShift OVN-Kubernetes secondary networks via encrypted tunnels. The primary use case is NSX-T migration — bridging NSX-T segments to OVN-K during VMware-to-OpenShift Virtualization transitions where VMs on both platforms must share a broadcast domain.

## Architecture

```
VPCGateway (provides FIP, uplink, VPC routes)
  ├── VPCRouter (references gateway, handles SNAT/DHCP/firewall)
  └── VPCL2Bridge (references gateway, handles L2 tunnels)
       └── Bridge Pod (WireGuard + GRETAP or NSX Edge or FRR+EVPN)
```

VPCL2Bridge is a standalone CRD with its own reconciler, referencing a VPCGateway for the tunnel endpoint floating IP. Multiple bridges can reference the same gateway. The bridge pod is independent from the router pod — tunnel stays up when the router restarts.

### Why Standalone CRD

- **Independent lifecycle** — bridge stays up when gateway config changes (NAT, firewall, routes)
- **Multiple bridges per gateway** — one gateway can serve bridges to NSX, to on-prem, to another cloud
- **All three approaches** (GRETAP+WG, L2VPN, EVPN) fit under `spec.type`
- **Follows existing pattern** — VPCRouter is a standalone CRD referencing VPCGateway; VPCL2Bridge follows the same pattern

## Three Tunnel Approaches

### Approach 1: GRETAP over WireGuard

Fabric-agnostic, no NSX dependency. GRETAP provides L2 bridging, WireGuard provides encryption and NAT traversal.

**Encapsulation stack:**
```
Ethernet frame → GRETAP (L2 GRE) → WireGuard (encrypted) → Fabric (IP)
```

**Traffic flow:**
```
NSX-T Segment → NSX Edge TEP → [IP Fabric] → WireGuard → GRETAP → Bridge Pod → OVN LocalNet → KubeVirt VM
```

**Reference CLI commands (maps to bridge pod init script):**

```bash
# Step 1: WireGuard underlay
ip link add dev wg0 type wireguard
ip addr add 10.0.0.1/30 dev wg0
wg set wg0 private-key /run/secrets/wireguard/privateKey \
    peer <REMOTE_PUBLIC_KEY> \
    endpoint <REMOTE_PUBLIC_IP>:51820 \
    allowed-ips 0.0.0.0/0
ip link set wg0 up

# Step 2: GRETAP (endpoints are WireGuard tunnel IPs, not real IPs)
ip link add dev gretap0 type gretap \
    local 10.0.0.1 remote 10.0.0.2 ttl 255
ip link set gretap0 mtu 1400
ip link set gretap0 up

# Step 3: Bridge to OVN LocalNet
ip link add name br-l2 type bridge
ip link set gretap0 master br-l2
ip link set net0 master br-l2
ip link set br-l2 up

# Step 4: MSS clamping
nft add table inet mangle
nft add chain inet mangle forward '{ type filter hook forward priority -150; }'
nft add rule inet mangle forward tcp flags syn / syn,rst \
    tcp option maxseg size set 1360
```

### Approach 2: NSX-T L2 VPN

For environments with NSX-T. Deploys an Autonomous NSX Edge as a container.

- NSX side: L2 VPN Server on Tier-0/Tier-1 Gateway
- OpenShift side: Autonomous Edge pod acts as L2 VPN Client
- Decapsulates frames and bridges to OVN LocalNet via Multus

### Approach 3: EVPN-VXLAN with FRRouting

Most scalable. FRRouting's BGP EVPN exchanges MAC/IP advertisements with NSX-T VTEPs.

- Dynamic MAC learning (no flooding for unknown unicasts)
- Multi-homing and fast failover via ESI
- Requires BGP peering between FRR and NSX-T

## CRD: VPCL2Bridge

**API Group**: `vpc.roks.ibm.com/v1alpha1`
**Short Name**: `vlb`
**Finalizer**: `vpc.roks.ibm.com/l2bridge-cleanup`

### Type Definition

```go
// VPCL2Bridge bridges a remote L2 segment (NSX-T, on-prem, multi-cloud)
// to an OVN-Kubernetes secondary network via an encrypted tunnel.
//
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=vlb
// +kubebuilder:printcolumn:name="Type",type=string,JSONPath=`.spec.type`
// +kubebuilder:printcolumn:name="Network",type=string,JSONPath=`.spec.networkRef.name`
// +kubebuilder:printcolumn:name="Remote",type=string,JSONPath=`.spec.remote.endpoint`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Tunnel IP",type=string,JSONPath=`.status.tunnelEndpoint`,priority=1
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type VPCL2Bridge struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`
    Spec              VPCL2BridgeSpec   `json:"spec,omitempty"`
    Status            VPCL2BridgeStatus `json:"status,omitempty"`
}
```

### Spec

```go
type VPCL2BridgeSpec struct {
    // Type selects the tunneling approach.
    // +kubebuilder:validation:Required
    // +kubebuilder:validation:Enum=gretap-wireguard;l2vpn;evpn-vxlan
    Type string `json:"type"`

    // GatewayRef references the VPCGateway providing the FIP and uplink.
    // +kubebuilder:validation:Required
    // +kubebuilder:validation:MinLength=1
    GatewayRef string `json:"gatewayRef"`

    // NetworkRef identifies the OVN network to bridge.
    NetworkRef BridgeNetworkRef `json:"networkRef"`

    // Remote defines the remote tunnel endpoint.
    Remote BridgeRemote `json:"remote"`

    // MTU controls tunnel MTU and MSS clamping.
    // +optional
    MTU *BridgeMTU `json:"mtu,omitempty"`

    // Pod allows overriding the bridge pod image.
    // +optional
    Pod *RouterPodSpec `json:"pod,omitempty"`
}

type BridgeNetworkRef struct {
    // Name of the CUDN or UDN to bridge.
    // +kubebuilder:validation:Required
    // +kubebuilder:validation:MinLength=1
    Name string `json:"name"`

    // Kind: ClusterUserDefinedNetwork or UserDefinedNetwork.
    // +kubebuilder:validation:Enum=ClusterUserDefinedNetwork;UserDefinedNetwork
    // +kubebuilder:default=ClusterUserDefinedNetwork
    Kind string `json:"kind,omitempty"`

    // Namespace (required for UDN).
    // +optional
    Namespace string `json:"namespace,omitempty"`
}

type BridgeRemote struct {
    // Endpoint is the remote tunnel peer IP or hostname.
    // +kubebuilder:validation:Required
    // +kubebuilder:validation:MinLength=1
    Endpoint string `json:"endpoint"`

    // WireGuard config (required for gretap-wireguard type).
    // +optional
    WireGuard *BridgeWireGuard `json:"wireguard,omitempty"`

    // L2VPN config (required for l2vpn type).
    // +optional
    L2VPN *BridgeL2VPN `json:"l2vpn,omitempty"`

    // EVPN config (required for evpn-vxlan type).
    // +optional
    EVPN *BridgeEVPN `json:"evpn,omitempty"`
}

type BridgeWireGuard struct {
    // PrivateKey reference to a K8s Secret containing the WireGuard private key.
    PrivateKey SecretKeyRef `json:"privateKey"`

    // PeerPublicKey is the remote peer's WireGuard public key.
    // +kubebuilder:validation:Required
    // +kubebuilder:validation:MinLength=1
    PeerPublicKey string `json:"peerPublicKey"`

    // ListenPort for WireGuard.
    // +kubebuilder:default=51820
    // +kubebuilder:validation:Minimum=1
    // +kubebuilder:validation:Maximum=65535
    // +optional
    ListenPort int32 `json:"listenPort,omitempty"`

    // TunnelAddressLocal is the WireGuard tunnel IP (e.g., 10.0.0.1/30).
    // +kubebuilder:validation:Required
    TunnelAddressLocal string `json:"tunnelAddressLocal"`

    // TunnelAddressRemote is the remote WireGuard tunnel IP (e.g., 10.0.0.2).
    // +kubebuilder:validation:Required
    TunnelAddressRemote string `json:"tunnelAddressRemote"`
}

type BridgeL2VPN struct {
    // NSXManagerHost is the NSX-T Manager hostname or IP.
    // +kubebuilder:validation:Required
    // +kubebuilder:validation:MinLength=1
    NSXManagerHost string `json:"nsxManagerHost"`

    // L2VPNServiceID is the NSX-T L2 VPN service ID on the Tier-0/Tier-1.
    // +kubebuilder:validation:Required
    // +kubebuilder:validation:MinLength=1
    L2VPNServiceID string `json:"l2vpnServiceID"`

    // Credentials for the Autonomous Edge to authenticate to NSX Manager.
    Credentials SecretKeyRef `json:"credentials"`

    // EdgeImage is the container image for the Autonomous NSX Edge.
    // +optional
    EdgeImage string `json:"edgeImage,omitempty"`
}

type BridgeEVPN struct {
    // ASN is the local BGP autonomous system number.
    // +kubebuilder:validation:Required
    // +kubebuilder:validation:Minimum=1
    // +kubebuilder:validation:Maximum=4294967295
    ASN int32 `json:"asn"`

    // VNI is the VXLAN Network Identifier for this segment.
    // +kubebuilder:validation:Required
    // +kubebuilder:validation:Minimum=1
    // +kubebuilder:validation:Maximum=16777215
    VNI int32 `json:"vni"`

    // PeerASN is the remote BGP peer's ASN.
    // +kubebuilder:validation:Required
    // +kubebuilder:validation:Minimum=1
    // +kubebuilder:validation:Maximum=4294967295
    PeerASN int32 `json:"peerASN"`

    // RouteReflector IP (optional, for scaled deployments).
    // +optional
    RouteReflector string `json:"routeReflector,omitempty"`

    // FRRImage overrides the FRRouting container image.
    // +optional
    FRRImage string `json:"frrImage,omitempty"`
}

type BridgeMTU struct {
    // TunnelMTU sets the inner MTU after encapsulation overhead.
    // +kubebuilder:default=1400
    // +kubebuilder:validation:Minimum=1200
    // +kubebuilder:validation:Maximum=9000
    TunnelMTU int32 `json:"tunnelMTU,omitempty"`

    // MSSClamp enables TCP MSS clamping via nftables.
    // +kubebuilder:default=true
    MSSClamp bool `json:"mssClamp,omitempty"`
}

type SecretKeyRef struct {
    // Name of the K8s Secret.
    // +kubebuilder:validation:Required
    Name string `json:"name"`

    // Key within the Secret.
    // +kubebuilder:validation:Required
    Key string `json:"key"`
}
```

### Status

```go
type VPCL2BridgeStatus struct {
    // Phase of the bridge lifecycle.
    // +kubebuilder:validation:Enum=Pending;Provisioning;Established;Degraded;Error
    Phase string `json:"phase,omitempty"`

    // TunnelEndpoint is the local FIP:port used as the tunnel endpoint.
    TunnelEndpoint string `json:"tunnelEndpoint,omitempty"`

    // RemoteMACsLearned is the count of MAC addresses learned from the remote side.
    RemoteMACsLearned int32 `json:"remoteMACsLearned,omitempty"`

    // LocalMACsAdvertised is the count of local MACs visible to the remote side.
    LocalMACsAdvertised int32 `json:"localMACsAdvertised,omitempty"`

    // BytesIn tracks total bytes received through the tunnel.
    BytesIn int64 `json:"bytesIn,omitempty"`

    // BytesOut tracks total bytes sent through the tunnel.
    BytesOut int64 `json:"bytesOut,omitempty"`

    // LastHandshake is the last successful WireGuard handshake (gretap-wireguard only).
    // +optional
    LastHandshake *metav1.Time `json:"lastHandshake,omitempty"`

    // PodName is the name of the bridge pod.
    PodName string `json:"podName,omitempty"`

    // SyncStatus indicates reconciliation state.
    // +kubebuilder:validation:Enum=Synced;Pending;Failed
    SyncStatus string `json:"syncStatus,omitempty"`

    // LastSyncTime is the last successful reconciliation.
    LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

    // Message is a human-readable status message.
    Message string `json:"message,omitempty"`

    // Conditions are standard K8s conditions (Ready, TunnelEstablished, PodReady).
    Conditions []metav1.Condition `json:"conditions,omitempty"`
}
```

### CRD YAML (Helm template)

Location: `deploy/helm/roks-vpc-network-operator/templates/crds/vpcl2bridge-crd.yaml`

Print columns for `kubectl get vlb`:
```
NAME              TYPE                NETWORK        REMOTE          PHASE         AGE
nsx-migration     gretap-wireguard    workload-1     203.0.113.50    Established   5m
```

## Reconciler

### File: `pkg/controller/l2bridge/reconciler.go`

**Watches:**
- `VPCL2Bridge` (primary)
- `VPCGateway` (for FIP changes — triggers bridge pod recreation)
- `Pod` (owned bridge pods — for readiness tracking)

**Reconcile Flow:**

```
1. Fetch VPCL2Bridge CR
   └─ Not found → return (deleted)

2. Handle deletion (DeletionTimestamp set)
   ├─ Delete bridge pod
   ├─ Delete any VPC routes added for remote networks
   └─ Remove finalizer: vpc.roks.ibm.com/l2bridge-cleanup

3. Ensure finalizer
   └─ Add vpc.roks.ibm.com/l2bridge-cleanup if missing

4. Fetch referenced VPCGateway
   ├─ Not found → set Phase=Pending, Message="Gateway not found", requeue 30s
   └─ Not Ready → set Phase=Pending, Message="Waiting for gateway", requeue 30s

5. Get gateway's floating IP (tunnel endpoint)
   └─ No FIP → set Phase=Pending, Message="Gateway has no floating IP", requeue 30s

6. Build bridge pod based on spec.type
   ├─ gretap-wireguard → buildGRETAPPod(bridge, gateway)
   ├─ l2vpn → buildL2VPNPod(bridge, gateway)
   └─ evpn-vxlan → buildEVPNPod(bridge, gateway)

7. Create or update bridge pod
   ├─ Pod doesn't exist → Create, set Phase=Provisioning
   ├─ Pod spec changed (gateway FIP, WG keys, etc.) → Delete + recreate
   └─ Pod exists, unchanged → no-op

8. Check pod readiness
   ├─ Pod not ready → Phase=Provisioning, requeue 10s
   └─ Pod ready → Phase=Established

9. Collect tunnel stats
   └─ Read /tmp/bridge-stats.json from pod (via exec or shared annotation)
   └─ Update status: BytesIn, BytesOut, LastHandshake, RemoteMACsLearned

10. Set status conditions
    ├─ Ready (true if Phase=Established)
    ├─ TunnelEstablished (true if WG handshake recent)
    └─ PodReady (true if pod is Running+Ready)

11. Update status, requeue after 5min (drift detection)
```

### File: `pkg/controller/l2bridge/pod.go`

Bridge pod construction functions:

**`buildGRETAPPod(bridge, gateway)`**
- Privileged pod with `NET_ADMIN` capability
- Multus annotation for workload network (`net0`)
- Init script: WireGuard → GRETAP → Linux bridge → MSS clamp → stats loop
- WireGuard private key mounted from Secret as volume
- Environment variables: `WG_LOCAL_ADDR`, `WG_REMOTE_ENDPOINT`, `WG_PEER_PUBLIC_KEY`, `WG_LISTEN_PORT`, `GRETAP_LOCAL`, `GRETAP_REMOTE`, `TUNNEL_MTU`, `MSS_CLAMP`

**`buildL2VPNPod(bridge, gateway)`**
- Runs Autonomous NSX Edge container image
- NSX Manager credentials mounted from Secret
- Environment variables: `NSX_MANAGER_HOST`, `L2VPN_SERVICE_ID`

**`buildEVPNPod(bridge, gateway)`**
- Runs FRRouting container with BGP EVPN config
- VXLAN interface bridged to OVN LocalNet
- FRR config generated as ConfigMap and mounted
- Environment variables: `LOCAL_ASN`, `PEER_ASN`, `VNI`, `ROUTE_REFLECTOR`

**Shared helpers:**
- `bridgePodName(bridge) string` — `l2bridge-<bridge-name>`
- `buildMultusAnnotation(bridge) string` — Multus network attachment JSON
- `computeMSS(mtu int32) int32` — `mtu - 40` (TCP+IP headers)

## Console Plugin

### New Pages

#### L2 Bridges List Page

**File**: `console-plugin/src/pages/L2BridgesListPage.tsx`
**Route**: `/vpc-networking/l2-bridges`

```
VPCNetworkingShell
  └─ PageSection (description text)
  └─ PageSection
      ├─ Toolbar (SearchInput + "Create L2 Bridge" button)
      ├─ EmptyState (no bridges)
      └─ Table (compact)
          ├─ Columns: Name, Type, Network, Remote, Phase, Age, Actions
          ├─ Type column: Label badges (blue=GRETAP+WG, purple=L2VPN, cyan=EVPN)
          ├─ Phase column: StatusBadge
          └─ Actions: Delete button → DeleteConfirmModal
```

#### L2 Bridge Detail Page

**File**: `console-plugin/src/pages/L2BridgeDetailPage.tsx`
**Route**: `/vpc-networking/l2-bridges/:name`

```
VPCNetworkingShell
  └─ PageSection (breadcrumb: L2 Bridges > {name})
  └─ PageSection
      ├─ Card: Overview
      │   └─ DescriptionList: Name, Namespace, Type, Phase, Sync Status, Created
      ├─ Card: Tunnel
      │   └─ DescriptionList: Remote Endpoint, Tunnel Endpoint, Last Handshake,
      │      Tunnel MTU, MSS Clamping, type-specific fields
      ├─ Card: Network
      │   └─ DescriptionList: Bridged Network (link), Gateway (link),
      │      Remote MACs Learned, Local MACs Advertised
      └─ Card: Throughput
          └─ DescriptionList: Bytes In, Bytes Out (formatted with units)
```

#### L2 Bridge Create Page

**File**: `console-plugin/src/pages/L2BridgeCreatePage.tsx`
**Route**: `/vpc-networking/l2-bridges/create`

```
VPCNetworkingShell
  └─ PageSection (breadcrumb + title)
  └─ PageSection
      └─ Card
          └─ Form
              ├─ Name (TextInput, required)
              ├─ Type (FormSelect: gretap-wireguard / l2vpn / evpn-vxlan)
              ├─ Gateway (FormSelect, populated from gateway list API)
              ├─ Network (FormSelect, populated from CUDN/UDN list API)
              ├─ Remote Endpoint (TextInput, IP validation)
              │
              ├─ [If gretap-wireguard]:
              │   ├─ WG Private Key Secret Name (TextInput)
              │   ├─ WG Private Key Secret Key (TextInput, default: "privateKey")
              │   ├─ Peer Public Key (TextInput)
              │   ├─ Listen Port (NumberInput, default: 51820)
              │   ├─ Tunnel Address Local (TextInput, e.g., "10.0.0.1/30")
              │   └─ Tunnel Address Remote (TextInput, e.g., "10.0.0.2")
              │
              ├─ [If l2vpn]:
              │   ├─ NSX Manager Host (TextInput)
              │   ├─ L2VPN Service ID (TextInput)
              │   ├─ Credentials Secret Name (TextInput)
              │   └─ Edge Image (TextInput, optional)
              │
              ├─ [If evpn-vxlan]:
              │   ├─ Local ASN (NumberInput)
              │   ├─ Peer ASN (NumberInput)
              │   ├─ VNI (NumberInput)
              │   ├─ Route Reflector (TextInput, optional)
              │   └─ FRR Image (TextInput, optional)
              │
              ├─ Tunnel MTU (NumberInput, default: 1400)
              ├─ MSS Clamping (Switch, default: on)
              └─ ActionGroup: Create + Cancel
```

### TypeScript Types

**File**: `console-plugin/src/api/types.ts` (additions)

```typescript
export interface L2Bridge {
  name: string;
  namespace: string;
  type: 'gretap-wireguard' | 'l2vpn' | 'evpn-vxlan';
  gatewayRef: string;
  networkRef: {
    name: string;
    kind: string;
    namespace?: string;
  };
  remoteEndpoint: string;
  phase: string;
  tunnelEndpoint?: string;
  remoteMACsLearned: number;
  localMACsAdvertised: number;
  bytesIn: number;
  bytesOut: number;
  lastHandshake?: string;
  tunnelMTU: number;
  mssClamp: boolean;
  podName?: string;
  syncStatus: string;
  createdAt?: string;
}

export interface CreateL2BridgeRequest {
  name: string;
  namespace?: string;
  type: string;
  gatewayRef: string;
  networkRef: {
    name: string;
    kind: string;
    namespace?: string;
  };
  remoteEndpoint: string;
  wireguard?: {
    privateKeySecretName: string;
    privateKeySecretKey: string;
    peerPublicKey: string;
    listenPort?: number;
    tunnelAddressLocal: string;
    tunnelAddressRemote: string;
  };
  l2vpn?: {
    nsxManagerHost: string;
    l2vpnServiceID: string;
    credentialsSecretName: string;
    credentialsSecretKey: string;
    edgeImage?: string;
  };
  evpn?: {
    asn: number;
    peerASN: number;
    vni: number;
    routeReflector?: string;
    frrImage?: string;
  };
  tunnelMTU?: number;
  mssClamp?: boolean;
}
```

### API Client Methods

**File**: `console-plugin/src/api/client.ts` (additions)

```typescript
async listL2Bridges(): Promise<ApiResponse<L2Bridge[]>>
async getL2Bridge(name: string, namespace?: string): Promise<ApiResponse<L2Bridge>>
async createL2Bridge(req: CreateL2BridgeRequest): Promise<ApiResponse<L2Bridge>>
async deleteL2Bridge(name: string, namespace?: string): Promise<ApiResponse<void>>
```

### Console Extensions

**File**: `console-plugin/console-extensions.json` (additions)

```json
{
  "type": "console.page/route",
  "properties": {
    "exact": true,
    "path": "/vpc-networking/l2-bridges",
    "component": { "$codeRef": "L2BridgesListPage" }
  }
},
{
  "type": "console.page/route",
  "properties": {
    "exact": true,
    "path": "/vpc-networking/l2-bridges/create",
    "component": { "$codeRef": "L2BridgeCreatePage" }
  }
},
{
  "type": "console.page/route",
  "properties": {
    "exact": true,
    "path": "/vpc-networking/l2-bridges/:name",
    "component": { "$codeRef": "L2BridgeDetailPage" }
  }
}
```

### Navigation

Add "L2 Bridges" to the VPCNetworkingShell sidebar navigation, after "Routers".

### BFF Endpoints

**File**: `cmd/bff/internal/handler/l2bridge.go`

```
GET    /api/v1/l2bridges              — list all VPCL2Bridge CRs
GET    /api/v1/l2bridges/:name        — get single bridge (query: ?ns=)
POST   /api/v1/l2bridges              — create bridge CR
DELETE /api/v1/l2bridges/:name        — delete bridge CR (query: ?ns=)
```

BFF reads VPCL2Bridge CRs via dynamic client, maps to TypeScript-friendly JSON, and handles SubjectAccessReview authorization.

## Unit Tests

### File: `pkg/controller/l2bridge/reconciler_test.go`

Uses fake K8s client + MockClient, following existing patterns.

| Test Name | Description |
|-----------|-------------|
| `TestReconcileNormal_CreateGRETAPBridge` | Creates bridge pod with WG+GRETAP init script. Validates: pod created, Phase=Provisioning, Multus annotation, env vars, WG secret volume mount. |
| `TestReconcileNormal_BridgeEstablished` | Bridge pod becomes Ready → Phase=Established, stats collected, conditions set. |
| `TestReconcileNormal_CreateL2VPNBridge` | L2VPN type creates Autonomous Edge pod with NSX Manager config. |
| `TestReconcileNormal_CreateEVPNBridge` | EVPN type creates FRR pod with BGP config. Validates FRR ConfigMap. |
| `TestReconcileNormal_GatewayNotReady` | Gateway not Ready → requeue, Phase=Pending, Message set. |
| `TestReconcileNormal_GatewayNotFound` | Gateway doesn't exist → requeue, Phase=Pending. |
| `TestReconcileNormal_GatewayFIPChange` | Gateway FIP changes → bridge pod deleted and recreated. |
| `TestReconcileDelete_CleanupPod` | Deletion timestamp set → bridge pod deleted, finalizer removed. |
| `TestReconcileDelete_CleanupVPCRoutes` | Deletion cleans up VPC routes for remote networks. |
| `TestReconcileNormal_MissingWireGuardSecret` | WG secret doesn't exist → Phase=Error, clear message. |

### File: `pkg/controller/l2bridge/pod_test.go`

Table-driven tests for pod construction utilities:

| Test Name | Cases |
|-----------|-------|
| `TestBuildGRETAPInitScript` | Validates init script contains: WG setup, GRETAP creation, bridge setup, MSS clamp. |
| `TestBuildGRETAPInitScript_NoMSSClamp` | MSSClamp=false → no nftables mangle rules in script. |
| `TestComputeMSS` | MTU 1400→1360, 1300→1260, 9000→8960. |
| `TestBridgePodName` | `l2bridge-my-bridge` for bridge named `my-bridge`. |
| `TestBuildMultusAnnotation` | Correct JSON for workload network attachment. |

## Tutorial / E2E Skill

### Skill: `test-l2bridge`

Following the `test-gateway` skill pattern.

#### Quick Smoke Test
1. Verify a VPCGateway exists and is Ready with a FIP
2. Generate WireGuard key pair (`wg genkey`, `wg pubkey`)
3. Create K8s Secret with WireGuard private key
4. Apply VPCL2Bridge CR (GRETAP+WG type with loopback test config)
5. Wait for bridge pod to reach Running state
6. Verify VPCL2Bridge status shows Phase=Provisioning→Established
7. Exec into pod: `wg show wg0` — verify interface exists
8. Exec into pod: `ip link show gretap0` — verify GRETAP interface
9. Exec into pod: `ip link show br-l2` — verify bridge
10. Clean up: delete VPCL2Bridge, verify pod deleted, finalizer removed

#### Full Tutorial Walkthrough

**Prerequisites:**
- VPCGateway with FIP (Ready)
- Workload network (CUDN with LocalNet topology)
- Remote endpoint configured (NSX Edge or test peer)

**Steps:**
1. Generate WireGuard keys on both sides
2. Create K8s Secret: `kubectl create secret generic wg-bridge-key --from-file=privateKey=./private.key`
3. Apply VPCL2Bridge CR:
   ```yaml
   apiVersion: vpc.roks.ibm.com/v1alpha1
   kind: VPCL2Bridge
   metadata:
     name: nsx-migration
   spec:
     type: gretap-wireguard
     gatewayRef: my-gateway
     networkRef:
       name: workload-net-1
       kind: ClusterUserDefinedNetwork
     remote:
       endpoint: 203.0.113.50
       wireguard:
         privateKey:
           name: wg-bridge-key
           key: privateKey
         peerPublicKey: "aB3d...="
         listenPort: 51820
         tunnelAddressLocal: "10.0.0.1/30"
         tunnelAddressRemote: "10.0.0.2"
     mtu:
       tunnelMTU: 1400
       mssClamp: true
   ```
4. Watch bridge pod startup: `kubectl get pods -l app=l2bridge`
5. Monitor status: `kubectl get vlb nsx-migration -w`
6. Verify tunnel inside pod:
   - `kubectl exec l2bridge-nsx-migration -- wg show`
   - `kubectl exec l2bridge-nsx-migration -- ip link show gretap0`
   - `kubectl exec l2bridge-nsx-migration -- bridge fdb show dev gretap0`
7. Test L2 connectivity (if remote side configured):
   - `kubectl exec l2bridge-nsx-migration -- arping -I br-l2 <remote-vm-ip>`
8. Check console UI: Navigate to VPC Networking → L2 Bridges
9. Cleanup: `kubectl delete vlb nsx-migration`

## Key Requirements

| Component | Requirement |
|-----------|-------------|
| **MTU** | Fabric MTU 1700+ or clamp MSS. Overhead: WG (60B) + GRETAP (38B) = 98B |
| **Multus CNI** | Required for workload network attachment on bridge pod |
| **Promiscuous mode** | Required on port groups/BM NICs for foreign MAC addresses |
| **IP reachability** | Remote endpoint must be reachable from gateway's FIP |
| **NET_ADMIN** | Bridge pod needs `NET_ADMIN` capability for WG, GRETAP, nftables |
| **WireGuard kernel module** | Must be available on worker nodes (standard on RHEL 9+/CoreOS) |

## File Inventory

### New Files
| Path | Description |
|------|-------------|
| `api/v1alpha1/vpcl2bridge_types.go` | CRD type definitions |
| `pkg/controller/l2bridge/reconciler.go` | Reconciler logic |
| `pkg/controller/l2bridge/pod.go` | Bridge pod construction |
| `pkg/controller/l2bridge/reconciler_test.go` | Reconciler unit tests |
| `pkg/controller/l2bridge/pod_test.go` | Pod construction unit tests |
| `cmd/bff/internal/handler/l2bridge.go` | BFF REST handlers |
| `console-plugin/src/pages/L2BridgesListPage.tsx` | List page |
| `console-plugin/src/pages/L2BridgeDetailPage.tsx` | Detail page |
| `console-plugin/src/pages/L2BridgeCreatePage.tsx` | Create page |
| `deploy/helm/.../templates/crds/vpcl2bridge-crd.yaml` | CRD YAML |

### Modified Files
| Path | Change |
|------|--------|
| `api/v1alpha1/zz_generated.deepcopy.go` | Regenerated (make generate) |
| `cmd/manager/main.go` | Register L2Bridge reconciler |
| `cmd/bff/internal/router.go` | Register L2Bridge routes |
| `cmd/bff/internal/model/types.go` | Add L2Bridge model types |
| `console-plugin/src/api/types.ts` | Add L2Bridge TypeScript types |
| `console-plugin/src/api/client.ts` | Add L2Bridge API methods |
| `console-plugin/src/api/hooks.ts` | Add useL2Bridge, useL2Bridges hooks |
| `console-plugin/console-extensions.json` | Register 3 new routes |
| `console-plugin/package.json` | Expose new page modules |
| `console-plugin/src/components/VPCNetworkingShell.tsx` | Add nav item |
| `console-plugin/src/pages/VPCDashboardPage.tsx` | Add L2 Bridges card |
| `pkg/gc/orphan_collector.go` | (Optional) Clean up orphaned bridge pods |
