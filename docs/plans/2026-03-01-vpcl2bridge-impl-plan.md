# VPCL2Bridge Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement the VPCL2Bridge CRD, reconciler, BFF endpoints, console plugin UI, and tests to enable fabric-agnostic L2 bridging between NSX-T segments and OVN-Kubernetes networks.

**Architecture:** Standalone `VPCL2Bridge` CRD (short name: `vlb`) with its own reconciler, referencing a `VPCGateway` for tunnel endpoint FIP. Bridge pod runs WireGuard+GRETAP, NSX L2VPN, or EVPN-VXLAN depending on `spec.type`. Console plugin adds list/detail/create pages. BFF exposes CRUD REST endpoints.

**Tech Stack:** Go (controller-runtime), TypeScript/React (PatternFly 5), Helm CRD template

**Design Doc:** `docs/plans/2026-03-01-vpcl2bridge-design.md`

---

## Task 1: CRD Type Definitions

**Files:**
- Create: `roks-vpc-network-operator/api/v1alpha1/vpcl2bridge_types.go`

**Step 1: Create the type definitions file**

Create `roks-vpc-network-operator/api/v1alpha1/vpcl2bridge_types.go` with all types from the design doc:
- `VPCL2Bridge` (root type with kubebuilder markers, short name `vlb`)
- `VPCL2BridgeList`
- `VPCL2BridgeSpec` — Type, GatewayRef, NetworkRef, Remote, MTU, Pod
- `VPCL2BridgeStatus` — Phase, TunnelEndpoint, MACs, Bytes, LastHandshake, PodName, conditions
- `BridgeNetworkRef` — Name, Kind (CUDN/UDN), Namespace
- `BridgeRemote` — Endpoint, WireGuard, L2VPN, EVPN sub-structs
- `BridgeWireGuard` — PrivateKey (SecretKeyRef), PeerPublicKey, ListenPort, TunnelAddresses
- `BridgeL2VPN` — NSXManagerHost, L2VPNServiceID, Credentials, EdgeImage
- `BridgeEVPN` — ASN, VNI, PeerASN, RouteReflector, FRRImage
- `BridgeMTU` — TunnelMTU (default 1400), MSSClamp (default true)
- `SecretKeyRef` — Name, Key (reuse if already exists in the package)
- Register with `SchemeBuilder.Register(&VPCL2Bridge{}, &VPCL2BridgeList{})`

Follow the exact patterns from `vpcrouter_types.go`: embed `metav1.TypeMeta` and `metav1.ObjectMeta`, use `+kubebuilder:validation:Enum`, `+kubebuilder:default`, `+optional` markers.

Print columns: Type, Network, Remote, Phase, Tunnel IP (priority 1), Age.

**Step 2: Generate DeepCopy**

Run: `cd roks-vpc-network-operator && make generate`

This regenerates `api/v1alpha1/zz_generated.deepcopy.go` to include the new types.

**Step 3: Verify build**

Run: `cd roks-vpc-network-operator && go build ./...`
Expected: BUILD SUCCESS

**Step 4: Commit**

```bash
git add roks-vpc-network-operator/api/v1alpha1/vpcl2bridge_types.go \
       roks-vpc-network-operator/api/v1alpha1/zz_generated.deepcopy.go
git commit -m "feat(l2bridge): add VPCL2Bridge CRD type definitions"
```

---

## Task 2: CRD Helm Template

**Files:**
- Create: `roks-vpc-network-operator/deploy/helm/roks-vpc-network-operator/templates/crds/vpcl2bridge-crd.yaml`

**Step 1: Create the CRD YAML**

Follow the exact pattern from `vpcrouter-crd.yaml`. Key fields:
- `metadata.name`: `vpcl2bridges.vpc.roks.ibm.com`
- `spec.names`: kind=VPCL2Bridge, plural=vpcl2bridges, singular=vpcl2bridge, shortNames=[vlb]
- `spec.scope`: Namespaced
- `additionalPrinterColumns`: Type, Network, Remote, Phase, Tunnel IP (priority 1), Age
- `schema.openAPIV3Schema`: Full validation schema matching the Go types — enums, defaults, min/max, patterns

This is a large YAML file. Mirror the structure of the router CRD but with L2Bridge-specific fields. Include all nested objects (remote.wireguard, remote.l2vpn, remote.evpn, mtu, networkRef, etc.).

**Step 2: Lint the Helm chart**

Run: `helm lint roks-vpc-network-operator/deploy/helm/roks-vpc-network-operator/`
Expected: PASS (no errors)

**Step 3: Commit**

```bash
git add roks-vpc-network-operator/deploy/helm/roks-vpc-network-operator/templates/crds/vpcl2bridge-crd.yaml
git commit -m "feat(l2bridge): add VPCL2Bridge CRD Helm template"
```

---

## Task 3: Finalizer Constant

**Files:**
- Modify: `roks-vpc-network-operator/pkg/finalizers/finalizers.go`

**Step 1: Add the L2Bridge finalizer constant**

Add to the existing constants block:
```go
L2BridgeCleanup = "vpc.roks.ibm.com/l2bridge-cleanup"
```

Existing constants: `CUDNCleanup`, `VMCleanup`, `UDNCleanup`, `GatewayCleanup`, `RouterCleanup`.

**Step 2: Verify build**

Run: `cd roks-vpc-network-operator && go build ./...`
Expected: BUILD SUCCESS

**Step 3: Commit**

```bash
git add roks-vpc-network-operator/pkg/finalizers/finalizers.go
git commit -m "feat(l2bridge): add L2Bridge finalizer constant"
```

---

## Task 4: Bridge Pod Construction

**Files:**
- Create: `roks-vpc-network-operator/pkg/controller/l2bridge/pod.go`

**Step 1: Write the pod construction test file**

Create `roks-vpc-network-operator/pkg/controller/l2bridge/pod_test.go`:

```go
package l2bridge

import (
    "testing"

    v1alpha1 "github.com/IBM/roks-vpc-network-operator/api/v1alpha1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBridgePodName(t *testing.T) {
    tests := []struct {
        name string
        want string
    }{
        {"my-bridge", "l2bridge-my-bridge"},
        {"nsx-migration", "l2bridge-nsx-migration"},
    }
    for _, tt := range tests {
        if got := bridgePodName(tt.name); got != tt.want {
            t.Errorf("bridgePodName(%q) = %q, want %q", tt.name, got, tt.want)
        }
    }
}

func TestComputeMSS(t *testing.T) {
    tests := []struct {
        mtu  int32
        want int32
    }{
        {1400, 1360},
        {1300, 1260},
        {9000, 8960},
        {1500, 1460},
    }
    for _, tt := range tests {
        if got := computeMSS(tt.mtu); got != tt.want {
            t.Errorf("computeMSS(%d) = %d, want %d", tt.mtu, got, tt.want)
        }
    }
}

func TestBuildGRETAPInitScript(t *testing.T) {
    bridge := &v1alpha1.VPCL2Bridge{
        ObjectMeta: metav1.ObjectMeta{Name: "test-bridge", Namespace: "default"},
        Spec: v1alpha1.VPCL2BridgeSpec{
            Type: "gretap-wireguard",
            NetworkRef: v1alpha1.BridgeNetworkRef{Name: "workload-1"},
            Remote: v1alpha1.BridgeRemote{
                Endpoint: "203.0.113.50",
                WireGuard: &v1alpha1.BridgeWireGuard{
                    PeerPublicKey:       "aB3dEf...",
                    ListenPort:          51820,
                    TunnelAddressLocal:  "10.0.0.1/30",
                    TunnelAddressRemote: "10.0.0.2",
                },
            },
            MTU: &v1alpha1.BridgeMTU{TunnelMTU: 1400, MSSClamp: true},
        },
    }

    script := buildGRETAPInitScript(bridge)

    // Must contain WireGuard setup
    if !containsString(script, "ip link add dev wg0 type wireguard") {
        t.Error("script missing WireGuard interface creation")
    }
    if !containsString(script, "10.0.0.1/30") {
        t.Error("script missing WireGuard local address")
    }
    if !containsString(script, "aB3dEf...") {
        t.Error("script missing WireGuard peer public key")
    }

    // Must contain GRETAP setup
    if !containsString(script, "ip link add dev gretap0 type gretap") {
        t.Error("script missing GRETAP interface creation")
    }
    if !containsString(script, "local 10.0.0.1") {
        t.Error("script missing GRETAP local address")
    }
    if !containsString(script, "remote 10.0.0.2") {
        t.Error("script missing GRETAP remote address")
    }

    // Must contain bridge setup
    if !containsString(script, "ip link add name br-l2 type bridge") {
        t.Error("script missing bridge creation")
    }

    // Must contain MSS clamping
    if !containsString(script, "tcp option maxseg size set") {
        t.Error("script missing MSS clamping")
    }
}

func TestBuildGRETAPInitScript_NoMSSClamp(t *testing.T) {
    bridge := &v1alpha1.VPCL2Bridge{
        ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
        Spec: v1alpha1.VPCL2BridgeSpec{
            Type: "gretap-wireguard",
            Remote: v1alpha1.BridgeRemote{
                Endpoint: "203.0.113.50",
                WireGuard: &v1alpha1.BridgeWireGuard{
                    PeerPublicKey:       "key123",
                    TunnelAddressLocal:  "10.0.0.1/30",
                    TunnelAddressRemote: "10.0.0.2",
                },
            },
            MTU: &v1alpha1.BridgeMTU{TunnelMTU: 1400, MSSClamp: false},
        },
    }

    script := buildGRETAPInitScript(bridge)

    if containsString(script, "tcp option maxseg size set") {
        t.Error("script should NOT contain MSS clamping when disabled")
    }
}

func containsString(s, substr string) bool {
    return len(s) > 0 && len(substr) > 0 && len(s) >= len(substr) &&
        stringContains(s, substr)
}

func stringContains(s, substr string) bool {
    for i := 0; i <= len(s)-len(substr); i++ {
        if s[i:i+len(substr)] == substr {
            return true
        }
    }
    return false
}
```

**Step 2: Run the tests to verify they fail**

Run: `cd roks-vpc-network-operator && go test ./pkg/controller/l2bridge/ -v`
Expected: FAIL — functions not defined yet

**Step 3: Write the pod construction implementation**

Create `roks-vpc-network-operator/pkg/controller/l2bridge/pod.go`:

Implement:
- `bridgePodName(name string) string` — returns `"l2bridge-" + name`
- `computeMSS(mtu int32) int32` — returns `mtu - 40`
- `buildGRETAPPod(bridge *v1alpha1.VPCL2Bridge, gw *v1alpha1.VPCGateway) *corev1.Pod` — constructs privileged pod with:
  - Multus annotation for workload network (net0)
  - WireGuard secret volume mount from `bridge.Spec.Remote.WireGuard.PrivateKey`
  - Environment variables: `WG_LOCAL_ADDR`, `WG_REMOTE_ENDPOINT`, `WG_PEER_PUBLIC_KEY`, `WG_LISTEN_PORT`, `GRETAP_LOCAL`, `GRETAP_REMOTE`, `TUNNEL_MTU`, `MSS_CLAMP`
  - Init script via `buildGRETAPInitScript()`
  - Owner reference to the VPCL2Bridge CR
  - Labels: `app=l2bridge`, `vpc.roks.ibm.com/l2bridge=<name>`
- `buildGRETAPInitScript(bridge *v1alpha1.VPCL2Bridge) string` — generates the bash init script:
  1. Install tools (iproute, nftables, wireguard-tools)
  2. WireGuard setup (create wg0, set key, configure peer, bring up)
  3. GRETAP setup (create gretap0 with WG tunnel IPs, set MTU, bring up)
  4. Bridge setup (create br-l2, add gretap0 + net0, bring up)
  5. MSS clamping via nftables (if enabled)
  6. Stats reporter loop (writes WG stats to /tmp/bridge-stats.json)
  7. `exec sleep infinity`
- `buildL2VPNPod(bridge *v1alpha1.VPCL2Bridge, gw *v1alpha1.VPCGateway) *corev1.Pod` — Autonomous NSX Edge pod (stub — image + env vars)
- `buildEVPNPod(bridge *v1alpha1.VPCL2Bridge, gw *v1alpha1.VPCGateway) *corev1.Pod` — FRR pod with BGP EVPN config (stub — image + env vars)
- `buildMultusAnnotation(bridge *v1alpha1.VPCL2Bridge) string` — JSON for workload network attachment

Follow the patterns from `pkg/controller/router/pod.go`: same string builder approach for init script, same pod security context (privileged, NET_ADMIN), same Multus annotation format.

**Step 4: Run tests to verify they pass**

Run: `cd roks-vpc-network-operator && go test ./pkg/controller/l2bridge/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add roks-vpc-network-operator/pkg/controller/l2bridge/pod.go \
       roks-vpc-network-operator/pkg/controller/l2bridge/pod_test.go
git commit -m "feat(l2bridge): add bridge pod construction and init scripts"
```

---

## Task 5: Reconciler Implementation

**Files:**
- Create: `roks-vpc-network-operator/pkg/controller/l2bridge/reconciler.go`

**Step 1: Write the reconciler test file**

Create `roks-vpc-network-operator/pkg/controller/l2bridge/reconciler_test.go`:

Tests to implement (follow `pkg/controller/router/reconciler_test.go` patterns):

- `TestReconcileNormal_CreateGRETAPBridge` — Creates VPCL2Bridge + VPCGateway (Ready, with FIP). Expects: bridge pod created, Phase=Provisioning, finalizer added, pod has correct Multus annotation + env vars + WG secret volume.
- `TestReconcileNormal_GatewayNotReady` — Gateway exists but Phase=Pending. Expects: Phase=Pending, Message="Waiting for gateway", requeue.
- `TestReconcileNormal_GatewayNotFound` — Gateway doesn't exist. Expects: Phase=Pending, Message="Gateway not found", requeue.
- `TestReconcileDelete_CleanupPod` — VPCL2Bridge has DeletionTimestamp. Expects: bridge pod deleted, finalizer removed.
- `TestReconcileNormal_MissingWireGuardConfig` — Type=gretap-wireguard but no WireGuard section. Expects: Phase=Error, descriptive message.

Use `newTestScheme()` helper (registers clientgo + v1alpha1), `fake.NewClientBuilder()` with `.WithObjects()` and `.WithStatusSubresource()`.

**Step 2: Run tests to verify they fail**

Run: `cd roks-vpc-network-operator && go test ./pkg/controller/l2bridge/ -run TestReconcile -v`
Expected: FAIL — Reconciler not defined

**Step 3: Write the reconciler**

Create `roks-vpc-network-operator/pkg/controller/l2bridge/reconciler.go`:

```go
type Reconciler struct {
    client.Client
    Scheme   *runtime.Scheme
    Recorder record.EventRecorder
}
```

Implement `Reconcile(ctx, req)`:
1. Fetch `VPCL2Bridge` by name — not found → return
2. Check `DeletionTimestamp` → call `reconcileDelete()`
3. Call `reconcileNormal()`

Implement `reconcileNormal(ctx, bridge)`:
1. Ensure finalizer (`finalizers.L2BridgeCleanup`)
2. Fetch referenced VPCGateway — not found or not Ready → Phase=Pending, requeue 30s
3. Validate type-specific config (WG fields for gretap-wireguard, etc.) → Phase=Error if invalid
4. Build bridge pod based on `spec.type`
5. Get existing pod — if not found, create it, Phase=Provisioning
6. If pod exists but spec changed (compare hash), delete and recreate
7. Check pod readiness — Ready → Phase=Established
8. Update status (conditions, sync status, last sync time)
9. Requeue 5min

Implement `reconcileDelete(ctx, bridge)`:
1. Delete owned bridge pod (by label selector)
2. Remove finalizer
3. Return

Implement `SetupWithManager(mgr)`:
```go
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).
        For(&v1alpha1.VPCL2Bridge{}).
        Owns(&corev1.Pod{}).
        Complete(r)
}
```

**Step 4: Run tests to verify they pass**

Run: `cd roks-vpc-network-operator && go test ./pkg/controller/l2bridge/ -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add roks-vpc-network-operator/pkg/controller/l2bridge/reconciler.go \
       roks-vpc-network-operator/pkg/controller/l2bridge/reconciler_test.go
git commit -m "feat(l2bridge): add VPCL2Bridge reconciler with tests"
```

---

## Task 6: Register Reconciler in Manager

**Files:**
- Modify: `roks-vpc-network-operator/cmd/manager/main.go`

**Step 1: Add the import and registration**

Add import:
```go
l2bridgectr "github.com/IBM/roks-vpc-network-operator/pkg/controller/l2bridge"
```

Add registration block (after the router controller registration):
```go
if err := (&l2bridgectr.Reconciler{
    Client:   mgr.GetClient(),
    Scheme:   mgr.GetScheme(),
    Recorder: mgr.GetEventRecorderFor("l2bridge-controller"),
}).SetupWithManager(mgr); err != nil {
    logger.Error(err, "Unable to create VPCL2Bridge controller")
    os.Exit(1)
}
```

**Step 2: Verify build**

Run: `cd roks-vpc-network-operator && go build ./...`
Expected: BUILD SUCCESS

**Step 3: Run all tests**

Run: `cd roks-vpc-network-operator && go test ./... -count=1`
Expected: ALL PASS

**Step 4: Commit**

```bash
git add roks-vpc-network-operator/cmd/manager/main.go
git commit -m "feat(l2bridge): register L2Bridge controller in manager"
```

---

## Task 7: BFF Handler

**Files:**
- Create: `roks-vpc-network-operator/cmd/bff/internal/handler/l2bridge_handler.go`
- Modify: `roks-vpc-network-operator/cmd/bff/internal/handler/router.go` (route registration)

**Step 1: Create the L2Bridge handler**

Create `roks-vpc-network-operator/cmd/bff/internal/handler/l2bridge_handler.go`:

Follow the exact pattern from `router_handler.go`:

```go
type L2BridgeHandler struct {
    dynClient dynamic.Interface
    rbac      *auth.RBACChecker
}

func NewL2BridgeHandler(dynClient dynamic.Interface, rbac *auth.RBACChecker) *L2BridgeHandler {
    return &L2BridgeHandler{dynClient: dynClient, rbac: rbac}
}
```

Implement 4 methods:
- `ListL2Bridges(w, r)` — GET `/api/v1/l2bridges` — list all VPCL2Bridge CRs via dynamic client, map to JSON response
- `GetL2Bridge(w, r)` — GET `/api/v1/l2bridges/:name` — get single bridge, query param `?ns=`
- `CreateL2Bridge(w, r)` — POST `/api/v1/l2bridges` — decode JSON body, create unstructured CR
- `DeleteL2Bridge(w, r)` — DELETE `/api/v1/l2bridges/:name` — delete CR

The dynamic client GVR: `schema.GroupVersionResource{Group: "vpc.roks.ibm.com", Version: "v1alpha1", Resource: "vpcl2bridges"}`

Map unstructured objects to a flat JSON structure matching the TypeScript `L2Bridge` interface.

**Step 2: Register routes**

In `router.go`, inside `SetupRoutesWithClusterInfo()`, add:

```go
l2bHandler := NewL2BridgeHandler(dynClient, rbacChecker)
mux.HandleFunc("/api/v1/l2bridges", func(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case http.MethodGet:
        authMiddleware(l2bHandler.ListL2Bridges).ServeHTTP(w, r)
    case http.MethodPost:
        authMiddleware(l2bHandler.CreateL2Bridge).ServeHTTP(w, r)
    default:
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
    }
})
mux.HandleFunc("/api/v1/l2bridges/", func(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case http.MethodGet:
        authMiddleware(l2bHandler.GetL2Bridge).ServeHTTP(w, r)
    case http.MethodDelete:
        authMiddleware(l2bHandler.DeleteL2Bridge).ServeHTTP(w, r)
    default:
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
    }
})
```

**Step 3: Verify BFF build**

Run: `cd roks-vpc-network-operator/cmd/bff && go build ./...`
Expected: BUILD SUCCESS

**Step 4: Commit**

```bash
git add roks-vpc-network-operator/cmd/bff/internal/handler/l2bridge_handler.go \
       roks-vpc-network-operator/cmd/bff/internal/handler/router.go
git commit -m "feat(l2bridge): add BFF CRUD endpoints for L2 bridges"
```

---

## Task 8: Console Plugin — TypeScript Types and API Client

**Files:**
- Modify: `console-plugin/src/api/types.ts`
- Modify: `console-plugin/src/api/client.ts`
- Modify: `console-plugin/src/api/hooks.ts`

**Step 1: Add TypeScript types**

In `types.ts`, add:

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

**Step 2: Add API client methods**

In `client.ts`, add to the `VPCNetworkClient` class:

```typescript
async listL2Bridges(): Promise<ApiResponse<L2Bridge[]>> {
  return this.get<L2Bridge[]>('/api/v1/l2bridges');
}

async getL2Bridge(name: string, namespace?: string): Promise<ApiResponse<L2Bridge>> {
  const params = namespace ? `?ns=${encodeURIComponent(namespace)}` : '';
  return this.get<L2Bridge>(`/api/v1/l2bridges/${encodeURIComponent(name)}${params}`);
}

async createL2Bridge(req: CreateL2BridgeRequest): Promise<ApiResponse<L2Bridge>> {
  return this.post<L2Bridge>('/api/v1/l2bridges', req);
}

async deleteL2Bridge(name: string, namespace?: string): Promise<ApiResponse<void>> {
  const params = namespace ? `?ns=${encodeURIComponent(namespace)}` : '';
  return this.delete<void>(`/api/v1/l2bridges/${encodeURIComponent(name)}${params}`);
}
```

**Step 3: Add hooks**

In `hooks.ts`, add:

```typescript
export function useL2Bridges() {
  const { data: l2bridges, loading, error } = useBFFData(
    () => apiClient.listL2Bridges(),
    [],
  );
  return { l2bridges, loading, error };
}

export function useL2Bridge(name: string, namespace?: string) {
  const { data: l2bridge, loading, error } = useBFFData(
    () => apiClient.getL2Bridge(name, namespace),
    [name, namespace],
  );
  return { l2bridge, loading, error };
}
```

**Step 4: Verify TypeScript**

Run: `cd console-plugin && npm run ts-check`
Expected: PASS (no type errors)

**Step 5: Commit**

```bash
git add console-plugin/src/api/types.ts \
       console-plugin/src/api/client.ts \
       console-plugin/src/api/hooks.ts
git commit -m "feat(l2bridge): add TypeScript types, API client, and hooks"
```

---

## Task 9: Console Plugin — List Page

**Files:**
- Create: `console-plugin/src/pages/L2BridgesListPage.tsx`

**Step 1: Create the list page**

Follow the exact pattern from `GatewaysListPage.tsx` / `RoutersListPage.tsx`:

- `VPCNetworkingShell` wrapper
- `PageSection` with description text: "L2 Bridges extend Layer 2 networks across sites — connecting NSX-T segments, on-prem networks, or multi-cloud VNETs to OVN-Kubernetes workload networks via encrypted tunnels."
- Toolbar: `SearchInput` + "Create L2 Bridge" button (link to `/vpc-networking/l2-bridges/create`)
- `EmptyState` when no bridges exist
- Compact `Table` with columns: Name (link to detail), Type (Label badge), Network, Remote, Phase (StatusBadge), Age, Actions (delete)
- Type column badges: blue for `gretap-wireguard`, purple for `l2vpn`, cyan for `evpn-vxlan`
- `DeleteConfirmModal` for deletion with resource name confirmation
- Use `useL2Bridges()` hook for data
- Search filters on name, type, remote endpoint

**Step 2: Verify build**

Run: `cd console-plugin && npm run ts-check`
Expected: PASS

**Step 3: Commit**

```bash
git add console-plugin/src/pages/L2BridgesListPage.tsx
git commit -m "feat(l2bridge): add L2 Bridges list page"
```

---

## Task 10: Console Plugin — Detail Page

**Files:**
- Create: `console-plugin/src/pages/L2BridgeDetailPage.tsx`

**Step 1: Create the detail page**

Follow the exact pattern from `GatewayDetailPage.tsx` / `RouterDetailPage.tsx`:

- Breadcrumb: L2 Bridges > {name}
- 4 Cards:
  - **Overview**: Name, Namespace, Type, Phase (StatusBadge), Sync Status, Created
  - **Tunnel**: Remote Endpoint, Tunnel Endpoint, Last Handshake, Tunnel MTU, MSS Clamping (yes/no), type-specific fields (WG listen port, EVPN ASN/VNI, L2VPN service ID)
  - **Network**: Bridged Network (link to `/vpc-networking/networks/{name}`), Gateway (link to `/vpc-networking/gateways/{name}`), Remote MACs Learned, Local MACs Advertised
  - **Throughput**: Bytes In, Bytes Out (format with KB/MB/GB units)
- Delete button with `DeleteConfirmModal`
- Use `useL2Bridge(name, ns)` hook
- Loading spinner, not-found state

**Step 2: Verify build**

Run: `cd console-plugin && npm run ts-check`
Expected: PASS

**Step 3: Commit**

```bash
git add console-plugin/src/pages/L2BridgeDetailPage.tsx
git commit -m "feat(l2bridge): add L2 Bridge detail page"
```

---

## Task 11: Console Plugin — Create Page

**Files:**
- Create: `console-plugin/src/pages/L2BridgeCreatePage.tsx`

**Step 1: Create the create page**

Follow the exact pattern from `GatewayCreatePage.tsx` / `RouterCreatePage.tsx`:

- Breadcrumb: L2 Bridges > Create
- Single Card with Form
- Fields:
  - Name (`TextInput`, required)
  - Type (`FormSelect`: GRETAP+WireGuard, NSX L2VPN, EVPN-VXLAN)
  - Gateway (`FormSelect`, populated from `useGateways()` hook — show only Ready gateways)
  - Network (`FormSelect`, populated from `useNetworkTypes()` or CUDN/UDN list)
  - Remote Endpoint (`TextInput`, IP validation via `isValidIPv4()`)
  - Conditional type-specific sections (show/hide based on selected type):
    - **GRETAP+WG**: WG Secret Name, Secret Key, Peer Public Key, Listen Port, Tunnel Address Local (CIDR validation), Tunnel Address Remote (IP validation)
    - **L2VPN**: NSX Manager Host, L2VPN Service ID, Credentials Secret Name
    - **EVPN**: Local ASN, Peer ASN, VNI, Route Reflector (optional), FRR Image (optional)
  - Tunnel MTU (`TextInput` type number, default 1400)
  - MSS Clamping (`Switch`, default on)
- Validation: name required, type required, gateway required, network required, remote endpoint valid IP, type-specific required fields
- Submit: `apiClient.createL2Bridge(req)`, navigate to list on success
- Cancel: navigate to list

**Step 2: Verify build**

Run: `cd console-plugin && npm run ts-check`
Expected: PASS

**Step 3: Commit**

```bash
git add console-plugin/src/pages/L2BridgeCreatePage.tsx
git commit -m "feat(l2bridge): add L2 Bridge create page"
```

---

## Task 12: Console Plugin — Registration and Navigation

**Files:**
- Modify: `console-plugin/console-extensions.json`
- Modify: `console-plugin/package.json` (exposedModules)
- Modify: `console-plugin/src/components/VPCNetworkingShell.tsx` (navigation tab)

**Step 1: Register routes in console-extensions.json**

Add 3 route entries (after the router routes):

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

**Step 2: Register exposed modules in package.json**

Add to `consolePlugin.exposedModules`:
```json
"L2BridgesListPage": "./src/pages/L2BridgesListPage",
"L2BridgeCreatePage": "./src/pages/L2BridgeCreatePage",
"L2BridgeDetailPage": "./src/pages/L2BridgeDetailPage"
```

**Step 3: Add navigation tab**

In `VPCNetworkingShell.tsx`, add to the `tabs` array (after `routers`, before `topology`):
```typescript
{ key: 'l2-bridges', label: 'L2 Bridges', path: '/vpc-networking/l2-bridges' },
```

**Step 4: Verify full build**

Run: `cd console-plugin && npm run ts-check && npm run build`
Expected: PASS

**Step 5: Commit**

```bash
git add console-plugin/console-extensions.json \
       console-plugin/package.json \
       console-plugin/src/components/VPCNetworkingShell.tsx
git commit -m "feat(l2bridge): register L2 Bridge pages and navigation"
```

---

## Task 13: Dashboard Integration

**Files:**
- Modify: `console-plugin/src/pages/VPCDashboardPage.tsx`

**Step 1: Add L2 Bridges card to dashboard**

Add a card to the dashboard grid showing:
- Total L2 Bridge count
- Count by phase (Established / Provisioning / Pending / Error)
- Count by type (GRETAP+WG / L2VPN / EVPN)
- Link to L2 Bridges list page

Use `useL2Bridges()` hook. Follow the pattern of existing dashboard cards (Gateways, Routers cards).

**Step 2: Verify build**

Run: `cd console-plugin && npm run ts-check && npm run build`
Expected: PASS

**Step 3: Commit**

```bash
git add console-plugin/src/pages/VPCDashboardPage.tsx
git commit -m "feat(l2bridge): add L2 Bridges card to dashboard"
```

---

## Task 14: Final Verification

**Step 1: Run all Go tests**

Run: `cd roks-vpc-network-operator && go test ./... -count=1 -v`
Expected: ALL PASS

**Step 2: Run Go vet**

Run: `cd roks-vpc-network-operator && go vet ./...`
Expected: PASS

**Step 3: Verify BFF build**

Run: `cd roks-vpc-network-operator/cmd/bff && go build ./...`
Expected: BUILD SUCCESS

**Step 4: Verify console plugin**

Run: `cd console-plugin && npm run ts-check && npm run build`
Expected: PASS

**Step 5: Lint Helm chart**

Run: `helm lint roks-vpc-network-operator/deploy/helm/roks-vpc-network-operator/`
Expected: PASS

**Step 6: Final commit (if any outstanding changes)**

```bash
git status  # should be clean
```
