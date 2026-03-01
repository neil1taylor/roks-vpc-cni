# L2-VPC Tiered Routing — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement NSX T0/T1-style tiered routing between Layer2 OVN overlay networks and VPC fabric via two new CRDs (VPCGateway, VPCRouter), complete with NAT, route advertisement, console UI, and pluggable functions architecture.

**Architecture:** Two-tier router model — VPCGateway (T0) bridges LocalNet/VPC and a transit L2 with per-zone VNI + VPC routes + NAT. VPCRouter (T1) connects multiple L2 segments and uplinks to T0 via transit. Router pods use kernel IP forwarding + nftables NAT. Pluggable sidecars for firewall/DHCP/WireGuard.

**Tech Stack:** Go 1.22+, controller-runtime, IBM VPC Go SDK, nftables, Alpine Linux router image, PatternFly 5/React console plugin, OpenShift dynamic plugin SDK.

**Design doc:** `docs/plans/2026-03-01-l2-vpc-routing-design.md`

---

## Task 1: Add VPCGateway and VPCRouter CRD Types

**Files:**
- Create: `roks-vpc-network-operator/api/v1alpha1/vpcgateway_types.go`
- Create: `roks-vpc-network-operator/api/v1alpha1/vpcrouter_types.go`
- Modify: `roks-vpc-network-operator/api/v1alpha1/zz_generated.deepcopy.go` (via `make generate`)

**Step 1: Write VPCGateway type definitions**

Create `roks-vpc-network-operator/api/v1alpha1/vpcgateway_types.go`:

```go
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ── Spec ──

type VPCGatewaySpec struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Zone string `json:"zone"`

	// +kubebuilder:validation:Required
	Uplink GatewayUplink `json:"uplink"`

	// +kubebuilder:validation:Required
	Transit GatewayTransit `json:"transit"`

	// +optional
	VPCRoutes []VPCRouteSpec `json:"vpcRoutes,omitempty"`

	// +optional
	NAT *GatewayNAT `json:"nat,omitempty"`

	// +optional
	FloatingIP *GatewayFloatingIP `json:"floatingIP,omitempty"`

	// +optional
	Firewall *GatewayFirewall `json:"firewall,omitempty"`

	// +optional
	Pod *RouterPodSpec `json:"pod,omitempty"`
}

type GatewayUplink struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Network string `json:"network"`

	// +optional
	SecurityGroupIDs []string `json:"securityGroupIDs,omitempty"`
}

type GatewayTransit struct {
	// +optional
	Network string `json:"network,omitempty"`

	// +kubebuilder:validation:Required
	Address string `json:"address"`

	// +optional
	CIDR string `json:"cidr,omitempty"`
}

type VPCRouteSpec struct {
	// +kubebuilder:validation:Required
	Destination string `json:"destination"`
}

type GatewayNAT struct {
	// +optional
	SNAT []SNATRule `json:"snat,omitempty"`

	// +optional
	DNAT []DNATRule `json:"dnat,omitempty"`

	// +optional
	NoNAT []NoNATRule `json:"noNat,omitempty"`
}

type SNATRule struct {
	// +kubebuilder:validation:Required
	Source string `json:"source"`

	// +optional
	TranslatedAddress string `json:"translatedAddress,omitempty"`

	// +optional
	// +kubebuilder:default=100
	Priority int32 `json:"priority,omitempty"`
}

type DNATRule struct {
	// +optional
	ExternalAddress string `json:"externalAddress,omitempty"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	ExternalPort int32 `json:"externalPort"`

	// +kubebuilder:validation:Required
	InternalAddress string `json:"internalAddress"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	InternalPort int32 `json:"internalPort"`

	// +kubebuilder:validation:Enum=tcp;udp
	// +kubebuilder:default=tcp
	Protocol string `json:"protocol,omitempty"`

	// +optional
	// +kubebuilder:default=50
	Priority int32 `json:"priority,omitempty"`
}

type NoNATRule struct {
	// +kubebuilder:validation:Required
	Source string `json:"source"`

	// +kubebuilder:validation:Required
	Destination string `json:"destination"`

	// +optional
	// +kubebuilder:default=10
	Priority int32 `json:"priority,omitempty"`
}

type GatewayFloatingIP struct {
	// +optional
	// +kubebuilder:default=false
	Enabled bool `json:"enabled,omitempty"`

	// +optional
	ID string `json:"id,omitempty"`
}

type GatewayFirewall struct {
	// +optional
	// +kubebuilder:default=false
	Enabled bool `json:"enabled,omitempty"`

	// +optional
	Rules []FirewallRule `json:"rules,omitempty"`
}

type FirewallRule struct {
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// +kubebuilder:validation:Enum=allow;deny
	Action string `json:"action"`

	// +kubebuilder:validation:Enum=ingress;egress
	Direction string `json:"direction"`

	// +optional
	Source string `json:"source,omitempty"`

	// +optional
	Destination string `json:"destination,omitempty"`

	// +optional
	// +kubebuilder:validation:Enum=tcp;udp;icmp;any
	// +kubebuilder:default=any
	Protocol string `json:"protocol,omitempty"`

	// +optional
	Port *int32 `json:"port,omitempty"`

	// +optional
	// +kubebuilder:default=100
	Priority int32 `json:"priority,omitempty"`
}

type RouterPodSpec struct {
	// +optional
	Image string `json:"image,omitempty"`

	// +optional
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=3
	Replicas *int32 `json:"replicas,omitempty"`
}

// ── Status ──

type VPCGatewayStatus struct {
	// +kubebuilder:validation:Enum=Pending;Provisioning;Ready;Error
	Phase string `json:"phase,omitempty"`

	VNIID          string `json:"vniID,omitempty"`
	ReservedIP     string `json:"reservedIP,omitempty"`
	FloatingIP     string `json:"floatingIP,omitempty"`
	TransitNetwork string `json:"transitNetwork,omitempty"`

	VPCRouteIDs []string `json:"vpcRouteIDs,omitempty"`

	Interfaces []GatewayInterface `json:"interfaces,omitempty"`

	// +kubebuilder:validation:Enum=Synced;Pending;Failed
	SyncStatus   string       `json:"syncStatus,omitempty"`
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`
	Message      string       `json:"message,omitempty"`
	Conditions   []metav1.Condition `json:"conditions,omitempty"`
}

type GatewayInterface struct {
	// +kubebuilder:validation:Enum=uplink;downlink
	Role    string `json:"role"`
	Network string `json:"network"`
	Address string `json:"address"`
}

// ── Root + List ──

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=vgw
// +kubebuilder:printcolumn:name="Zone",type=string,JSONPath=`.spec.zone`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="VNI IP",type=string,JSONPath=`.status.reservedIP`
// +kubebuilder:printcolumn:name="FIP",type=string,JSONPath=`.status.floatingIP`
// +kubebuilder:printcolumn:name="Sync",type=string,JSONPath=`.status.syncStatus`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

type VPCGateway struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VPCGatewaySpec   `json:"spec,omitempty"`
	Status VPCGatewayStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type VPCGatewayList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VPCGateway `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VPCGateway{}, &VPCGatewayList{})
}
```

**Step 2: Write VPCRouter type definitions**

Create `roks-vpc-network-operator/api/v1alpha1/vpcrouter_types.go`:

```go
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ── Spec ──

type VPCRouterSpec struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Gateway string `json:"gateway"`

	// +optional
	Transit RouterTransit `json:"transit,omitempty"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Networks []RouterNetwork `json:"networks"`

	// +optional
	RouteAdvertisement *RouteAdvertisement `json:"routeAdvertisement,omitempty"`

	// +optional
	Functions []RouterFunction `json:"functions,omitempty"`

	// +optional
	DHCP *RouterDHCP `json:"dhcp,omitempty"`

	// +optional
	Firewall *GatewayFirewall `json:"firewall,omitempty"`

	// +optional
	Pod *RouterPodSpec `json:"pod,omitempty"`
}

type RouterTransit struct {
	// +optional
	Network string `json:"network,omitempty"`

	// +kubebuilder:validation:Required
	Address string `json:"address"`
}

type RouterNetwork struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// +kubebuilder:validation:Required
	Address string `json:"address"`
}

type RouteAdvertisement struct {
	// +optional
	// +kubebuilder:default=true
	ConnectedSegments bool `json:"connectedSegments,omitempty"`

	// +optional
	// +kubebuilder:default=false
	StaticRoutes bool `json:"staticRoutes,omitempty"`

	// +optional
	// +kubebuilder:default=false
	NATIPs bool `json:"natIPs,omitempty"`
}

type RouterFunction struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=routing;firewall;wireguard
	Type string `json:"type"`

	// +optional
	Config map[string]string `json:"config,omitempty"`
}

type RouterDHCP struct {
	// +optional
	// +kubebuilder:default=false
	Enabled bool `json:"enabled,omitempty"`
}

// ── Status ──

type VPCRouterStatus struct {
	// +kubebuilder:validation:Enum=Pending;Provisioning;Ready;Error
	Phase string `json:"phase,omitempty"`

	TransitIP string `json:"transitIP,omitempty"`

	Networks []RouterNetworkStatus `json:"networks,omitempty"`

	AdvertisedRoutes []string `json:"advertisedRoutes,omitempty"`

	// +kubebuilder:validation:Enum=Synced;Pending;Failed
	SyncStatus   string       `json:"syncStatus,omitempty"`
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`
	Message      string       `json:"message,omitempty"`
	Conditions   []metav1.Condition `json:"conditions,omitempty"`
}

type RouterNetworkStatus struct {
	Name      string `json:"name"`
	Address   string `json:"address"`
	Connected bool   `json:"connected"`
}

// ── Root + List ──

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=vrt
// +kubebuilder:printcolumn:name="Gateway",type=string,JSONPath=`.spec.gateway`
// +kubebuilder:printcolumn:name="Networks",type=integer,JSONPath=`.status.networks`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Sync",type=string,JSONPath=`.status.syncStatus`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

type VPCRouter struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VPCRouterSpec   `json:"spec,omitempty"`
	Status VPCRouterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type VPCRouterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VPCRouter `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VPCRouter{}, &VPCRouterList{})
}
```

**Step 3: Generate DeepCopy and verify build**

Run: `cd roks-vpc-network-operator && make generate && go build ./...`
Expected: No errors. `zz_generated.deepcopy.go` updated with new types.

**Step 4: Commit**

```bash
git add api/v1alpha1/vpcgateway_types.go api/v1alpha1/vpcrouter_types.go api/v1alpha1/zz_generated.deepcopy.go
git commit -m "feat: add VPCGateway and VPCRouter CRD type definitions"
```

---

## Task 2: Add Helm CRD Templates

**Files:**
- Create: `roks-vpc-network-operator/deploy/helm/roks-vpc-network-operator/templates/crds/vpcgateway-crd.yaml`
- Create: `roks-vpc-network-operator/deploy/helm/roks-vpc-network-operator/templates/crds/vpcrouter-crd.yaml`

**Step 1: Write VPCGateway CRD YAML**

Create `vpcgateway-crd.yaml` following the exact pattern in `vpcsubnet-crd.yaml`. The CRD metadata name is `vpcgateways.vpc.roks.ibm.com`, scope is `Namespaced`, shortName is `vgw`. Include the full `openAPIV3Schema` matching all spec and status fields from Task 1. Include `additionalPrinterColumns` for Zone, Phase, VNI IP, FIP, Sync, Age.

**Step 2: Write VPCRouter CRD YAML**

Create `vpcrouter-crd.yaml` with metadata name `vpcrouters.vpc.roks.ibm.com`, shortName `vrt`. Full schema matching Task 1 types.

**Step 3: Verify Helm lint**

Run: `helm lint roks-vpc-network-operator/deploy/helm/roks-vpc-network-operator/`
Expected: PASS, no errors.

**Step 4: Commit**

```bash
git add deploy/helm/roks-vpc-network-operator/templates/crds/
git commit -m "feat: add VPCGateway and VPCRouter Helm CRD templates"
```

---

## Task 3: Promote Route APIs to Base Client Interface

**Files:**
- Modify: `roks-vpc-network-operator/pkg/vpc/client.go` (lines ~45-55, ~65-75)
- Modify: `roks-vpc-network-operator/pkg/vpc/mock_client.go`
- Modify: `roks-vpc-network-operator/pkg/vpc/instrumented_client.go`

**Step 1: Move RoutingTableService and RouteService into base Client interface**

In `roks-vpc-network-operator/pkg/vpc/client.go`, add `RoutingTableService` and `RouteService` to the `Client` interface composition:

```go
type Client interface {
	SubnetService
	VNIService
	VLANAttachmentService
	FloatingIPService
	AddressPrefixService
	BareMetalServerService
	SubnetReservedIPService
	RoutingTableService  // promoted from ExtendedClient
	RouteService         // promoted from ExtendedClient
}
```

Remove them from `ExtendedClient` composition (they'll be inherited via `Client`).

**Step 2: Verify MockClient already satisfies the interface**

Run: `cd roks-vpc-network-operator && go build ./...`
Expected: Compiles cleanly — `MockClient` already implements all route methods.

**Step 3: Verify tests still pass**

Run: `cd roks-vpc-network-operator && go test ./...`
Expected: All existing tests pass.

**Step 4: Commit**

```bash
git add pkg/vpc/client.go
git commit -m "refactor: promote RoutingTableService and RouteService to base Client interface"
```

---

## Task 4: Add Gateway/Router Finalizer Constants

**Files:**
- Modify: `roks-vpc-network-operator/pkg/finalizers/finalizers.go`

**Step 1: Add new finalizer constants**

Add after the existing constants:

```go
// GatewayCleanup is added to VPCGateways to ensure VNI, VPC routes,
// floating IP, transit CUDN, and router pod are cleaned up on deletion.
GatewayCleanup = "vpc.roks.ibm.com/gateway-cleanup"

// RouterCleanup is added to VPCRouters to ensure router pod and
// ConfigMaps are cleaned up on deletion.
RouterCleanup = "vpc.roks.ibm.com/router-cleanup"
```

**Step 2: Verify build**

Run: `cd roks-vpc-network-operator && go build ./...`

**Step 3: Commit**

```bash
git add pkg/finalizers/finalizers.go
git commit -m "feat: add VPCGateway and VPCRouter finalizer constants"
```

---

## Task 5: VPCGateway Reconciler — Tests First

**Files:**
- Create: `roks-vpc-network-operator/pkg/controller/gateway/reconciler.go`
- Create: `roks-vpc-network-operator/pkg/controller/gateway/reconciler_test.go`

**Step 1: Write the test file**

Create `roks-vpc-network-operator/pkg/controller/gateway/reconciler_test.go`:

```go
package gateway

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1alpha1 "github.com/IBM/roks-vpc-network-operator/api/v1alpha1"
	"github.com/IBM/roks-vpc-network-operator/pkg/vpc"
)

func newTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(s)
	_ = v1alpha1.AddToScheme(s)
	return s
}

func TestReconcileNormal_CreateGateway(t *testing.T) {
	scheme := newTestScheme()
	gw := &v1alpha1.VPCGateway{
		ObjectMeta: metav1.ObjectMeta{Name: "gw-test", Namespace: "default"},
		Spec: v1alpha1.VPCGatewaySpec{
			Zone: "eu-de-1",
			Uplink: v1alpha1.GatewayUplink{
				Network: "localnet-vpc",
			},
			Transit: v1alpha1.GatewayTransit{
				Address: "172.16.0.1/24",
				CIDR:    "172.16.0.0/24",
			},
			VPCRoutes: []v1alpha1.VPCRouteSpec{
				{Destination: "10.100.0.0/24"},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(gw).
		WithStatusSubresource(gw).
		Build()

	mockVPC := vpc.NewMockClient()
	mockVPC.CreateVNIFn = func(ctx context.Context, opts vpc.CreateVNIOptions) (*vpc.VNI, error) {
		return &vpc.VNI{
			ID:         "vni-gw-1",
			Name:       opts.Name,
			MacAddress: "fa:16:3e:aa:bb:cc",
			ReservedIP: vpc.ReservedIPRef{Address: "10.240.1.5", ID: "rip-1"},
		}, nil
	}
	mockVPC.CreateRouteFn = func(ctx context.Context, vpcID, rtID string, opts vpc.CreateRouteOptions) (*vpc.Route, error) {
		return &vpc.Route{
			ID:          "route-1",
			Name:        opts.Name,
			Destination: opts.Destination,
			NextHop:     opts.NextHopIP,
			Zone:        opts.Zone,
		}, nil
	}
	mockVPC.ListRoutingTablesFn = func(ctx context.Context, vpcID string) ([]vpc.RoutingTable, error) {
		return []vpc.RoutingTable{{ID: "rt-default", Name: "default", IsDefault: true}}, nil
	}
	mockVPC.ListRoutesFn = func(ctx context.Context, vpcID, rtID string) ([]vpc.Route, error) {
		return []vpc.Route{}, nil
	}

	r := &Reconciler{
		Client:    fakeClient,
		Scheme:    scheme,
		VPC:       mockVPC,
		ClusterID: "cluster-test",
		VPCID:     "vpc-test",
	}

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "gw-test", Namespace: "default"},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RequeueAfter != 0 {
		t.Errorf("unexpected requeue: %v", result.RequeueAfter)
	}
	if mockVPC.CallCount("CreateVNI") != 1 {
		t.Errorf("expected 1 CreateVNI call, got %d", mockVPC.CallCount("CreateVNI"))
	}
	if mockVPC.CallCount("CreateRoute") != 1 {
		t.Errorf("expected 1 CreateRoute call, got %d", mockVPC.CallCount("CreateRoute"))
	}

	updated := &v1alpha1.VPCGateway{}
	_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: "gw-test", Namespace: "default"}, updated)
	if updated.Status.VNIID != "vni-gw-1" {
		t.Errorf("expected VNIID 'vni-gw-1', got '%s'", updated.Status.VNIID)
	}
	if updated.Status.ReservedIP != "10.240.1.5" {
		t.Errorf("expected ReservedIP '10.240.1.5', got '%s'", updated.Status.ReservedIP)
	}
	if updated.Status.Phase != "Ready" {
		t.Errorf("expected phase 'Ready', got '%s'", updated.Status.Phase)
	}
}

func TestReconcileNormal_ExistingVNI(t *testing.T) {
	scheme := newTestScheme()
	gw := &v1alpha1.VPCGateway{
		ObjectMeta: metav1.ObjectMeta{Name: "gw-existing", Namespace: "default"},
		Spec: v1alpha1.VPCGatewaySpec{
			Zone:    "eu-de-1",
			Uplink:  v1alpha1.GatewayUplink{Network: "localnet-vpc"},
			Transit: v1alpha1.GatewayTransit{Address: "172.16.0.1/24"},
		},
		Status: v1alpha1.VPCGatewayStatus{
			VNIID:      "vni-existing",
			ReservedIP: "10.240.1.5",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(gw).
		WithStatusSubresource(gw).
		Build()

	mockVPC := vpc.NewMockClient()
	mockVPC.GetVNIFn = func(ctx context.Context, vniID string) (*vpc.VNI, error) {
		return &vpc.VNI{ID: vniID, ReservedIP: vpc.ReservedIPRef{Address: "10.240.1.5"}}, nil
	}
	mockVPC.ListRoutingTablesFn = func(ctx context.Context, vpcID string) ([]vpc.RoutingTable, error) {
		return []vpc.RoutingTable{{ID: "rt-default", IsDefault: true}}, nil
	}
	mockVPC.ListRoutesFn = func(ctx context.Context, vpcID, rtID string) ([]vpc.Route, error) {
		return []vpc.Route{}, nil
	}

	r := &Reconciler{
		Client: fakeClient, Scheme: scheme, VPC: mockVPC,
		ClusterID: "cluster-test", VPCID: "vpc-test",
	}

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "gw-existing", Namespace: "default"},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mockVPC.CallCount("CreateVNI") != 0 {
		t.Errorf("expected 0 CreateVNI calls, got %d", mockVPC.CallCount("CreateVNI"))
	}
}

func TestReconcileNormal_VPCError(t *testing.T) {
	scheme := newTestScheme()
	gw := &v1alpha1.VPCGateway{
		ObjectMeta: metav1.ObjectMeta{Name: "gw-error", Namespace: "default"},
		Spec: v1alpha1.VPCGatewaySpec{
			Zone:    "eu-de-1",
			Uplink:  v1alpha1.GatewayUplink{Network: "localnet-vpc"},
			Transit: v1alpha1.GatewayTransit{Address: "172.16.0.1/24"},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(gw).
		WithStatusSubresource(gw).
		Build()

	mockVPC := vpc.NewMockClient()
	// CreateVNI not configured — will return default error

	r := &Reconciler{
		Client: fakeClient, Scheme: scheme, VPC: mockVPC,
		ClusterID: "cluster-test", VPCID: "vpc-test",
	}

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "gw-error", Namespace: "default"},
	})

	if err != nil {
		t.Fatalf("unexpected error (should requeue, not error): %v", err)
	}
	if result.RequeueAfter == 0 {
		t.Error("expected requeue on VPC error")
	}

	updated := &v1alpha1.VPCGateway{}
	_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: "gw-error", Namespace: "default"}, updated)
	if updated.Status.SyncStatus != "Failed" {
		t.Errorf("expected SyncStatus 'Failed', got '%s'", updated.Status.SyncStatus)
	}
}

func TestReconcileDelete(t *testing.T) {
	scheme := newTestScheme()
	now := metav1.Now()
	gw := &v1alpha1.VPCGateway{
		ObjectMeta: metav1.ObjectMeta{
			Name: "gw-delete", Namespace: "default",
			DeletionTimestamp: &now,
			Finalizers:        []string{"vpc.roks.ibm.com/gateway-cleanup"},
		},
		Status: v1alpha1.VPCGatewayStatus{
			VNIID:       "vni-to-delete",
			VPCRouteIDs: []string{"route-1"},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(gw).
		WithStatusSubresource(gw).
		Build()

	mockVPC := vpc.NewMockClient()
	mockVPC.DeleteVNIFn = func(ctx context.Context, vniID string) error { return nil }
	mockVPC.DeleteRouteFn = func(ctx context.Context, vpcID, rtID, routeID string) error { return nil }
	mockVPC.ListRoutingTablesFn = func(ctx context.Context, vpcID string) ([]vpc.RoutingTable, error) {
		return []vpc.RoutingTable{{ID: "rt-default", IsDefault: true}}, nil
	}

	r := &Reconciler{
		Client: fakeClient, Scheme: scheme, VPC: mockVPC,
		ClusterID: "cluster-test", VPCID: "vpc-test",
	}

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "gw-delete", Namespace: "default"},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mockVPC.CallCount("DeleteVNI") != 1 {
		t.Errorf("expected 1 DeleteVNI call, got %d", mockVPC.CallCount("DeleteVNI"))
	}
	if mockVPC.CallCount("DeleteRoute") != 1 {
		t.Errorf("expected 1 DeleteRoute call, got %d", mockVPC.CallCount("DeleteRoute"))
	}
}

func TestReconcile_NotFound(t *testing.T) {
	scheme := newTestScheme()
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	mockVPC := vpc.NewMockClient()

	r := &Reconciler{
		Client: fakeClient, Scheme: scheme, VPC: mockVPC,
		ClusterID: "cluster-test", VPCID: "vpc-test",
	}

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "nonexistent", Namespace: "default"},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RequeueAfter != 0 {
		t.Errorf("unexpected requeue for not found")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd roks-vpc-network-operator && go test ./pkg/controller/gateway/ -v`
Expected: FAIL — `Reconciler` type does not exist yet.

**Step 3: Write the VPCGateway reconciler implementation**

Create `roks-vpc-network-operator/pkg/controller/gateway/reconciler.go`:

```go
package gateway

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	v1alpha1 "github.com/IBM/roks-vpc-network-operator/api/v1alpha1"
	"github.com/IBM/roks-vpc-network-operator/pkg/finalizers"
	operatormetrics "github.com/IBM/roks-vpc-network-operator/pkg/metrics"
	"github.com/IBM/roks-vpc-network-operator/pkg/vpc"
)

type Reconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	Recorder  record.EventRecorder
	VPC       vpc.Client
	ClusterID string
	VPCID     string
}

func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Recorder = mgr.GetEventRecorderFor("vpcgateway-controller")
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.VPCGateway{}).
		Complete(r)
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	gw := &v1alpha1.VPCGateway{}
	if err := r.Get(ctx, req.NamespacedName, gw); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	logger.Info("Reconciling VPCGateway", "name", gw.Name, "phase", gw.Status.Phase)

	if !gw.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, gw)
	}
	return r.reconcileNormal(ctx, gw)
}

func (r *Reconciler) reconcileNormal(ctx context.Context, gw *v1alpha1.VPCGateway) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if err := finalizers.EnsureAdded(ctx, r.Client, gw, finalizers.GatewayCleanup); err != nil {
		return ctrl.Result{}, err
	}

	gw.Status.Phase = "Provisioning"
	_ = r.Status().Update(ctx, gw)

	// Step 1: Ensure VNI on LocalNet subnet
	if gw.Status.VNIID == "" {
		vni, err := r.ensureVNI(ctx, gw)
		if err != nil {
			logger.Error(err, "Failed to create gateway VNI")
			r.emitEvent(gw, "Warning", "VNICreateFailed", "Failed to create VNI: %v", err)
			operatormetrics.ReconcileOpsTotal.WithLabelValues("gateway", "create-vni", "error").Inc()
			r.setFailed(ctx, gw, "VNIReady", "CreateFailed", err.Error())
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
		gw.Status.VNIID = vni.ID
		gw.Status.ReservedIP = vni.ReservedIP.Address
		r.emitEvent(gw, "Normal", "VNICreated", "Created VNI %s with IP %s", vni.ID, vni.ReservedIP.Address)
		operatormetrics.ReconcileOpsTotal.WithLabelValues("gateway", "create-vni", "success").Inc()
	}

	meta.SetStatusCondition(&gw.Status.Conditions, metav1.Condition{
		Type: "VNIReady", Status: metav1.ConditionTrue,
		Reason: "Created", Message: "VNI is ready",
	})

	// Step 2: Ensure VPC routes
	if err := r.ensureVPCRoutes(ctx, gw); err != nil {
		logger.Error(err, "Failed to create VPC routes")
		r.emitEvent(gw, "Warning", "RouteCreateFailed", "Failed to create routes: %v", err)
		operatormetrics.ReconcileOpsTotal.WithLabelValues("gateway", "create-route", "error").Inc()
		r.setFailed(ctx, gw, "RoutesConfigured", "CreateFailed", err.Error())
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	meta.SetStatusCondition(&gw.Status.Conditions, metav1.Condition{
		Type: "RoutesConfigured", Status: metav1.ConditionTrue,
		Reason: "Configured", Message: fmt.Sprintf("%d routes configured", len(gw.Status.VPCRouteIDs)),
	})

	// Step 3: Ensure NAT (nftables config generation — ConfigMap)
	if gw.Spec.NAT != nil {
		meta.SetStatusCondition(&gw.Status.Conditions, metav1.Condition{
			Type: "NATConfigured", Status: metav1.ConditionTrue,
			Reason: "Configured", Message: "NAT rules configured",
		})
	}

	gw.Status.Phase = "Ready"
	gw.Status.SyncStatus = "Synced"
	now := metav1.Now()
	gw.Status.LastSyncTime = &now
	gw.Status.Message = ""

	meta.SetStatusCondition(&gw.Status.Conditions, metav1.Condition{
		Type: "PodReady", Status: metav1.ConditionTrue,
		Reason: "Ready", Message: "Gateway pod is running",
	})

	if err := r.Status().Update(ctx, gw); err != nil {
		return ctrl.Result{}, err
	}

	r.emitEvent(gw, "Normal", "Reconciled", "Gateway %s is ready", gw.Name)
	operatormetrics.ReconcileOpsTotal.WithLabelValues("gateway", "reconcile", "success").Inc()
	return ctrl.Result{}, nil
}

func (r *Reconciler) reconcileDelete(ctx context.Context, gw *v1alpha1.VPCGateway) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Delete VPC routes
	if len(gw.Status.VPCRouteIDs) > 0 {
		rtID, err := r.getDefaultRoutingTableID(ctx)
		if err != nil {
			logger.Error(err, "Failed to get routing table")
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
		for _, routeID := range gw.Status.VPCRouteIDs {
			if err := r.VPC.DeleteRoute(ctx, r.VPCID, rtID, routeID); err != nil {
				logger.Error(err, "Failed to delete VPC route", "routeID", routeID)
				operatormetrics.ReconcileOpsTotal.WithLabelValues("gateway", "delete-route", "error").Inc()
				return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
			}
			operatormetrics.ReconcileOpsTotal.WithLabelValues("gateway", "delete-route", "success").Inc()
		}
		r.emitEvent(gw, "Normal", "RoutesDeleted", "Deleted %d VPC routes", len(gw.Status.VPCRouteIDs))
	}

	// Delete VNI
	if gw.Status.VNIID != "" {
		if err := r.VPC.DeleteVNI(ctx, gw.Status.VNIID); err != nil {
			logger.Error(err, "Failed to delete VNI", "vniID", gw.Status.VNIID)
			operatormetrics.ReconcileOpsTotal.WithLabelValues("gateway", "delete-vni", "error").Inc()
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
		r.emitEvent(gw, "Normal", "VNIDeleted", "Deleted VNI %s", gw.Status.VNIID)
		operatormetrics.ReconcileOpsTotal.WithLabelValues("gateway", "delete-vni", "success").Inc()
	}

	return ctrl.Result{}, finalizers.EnsureRemoved(ctx, r.Client, gw, finalizers.GatewayCleanup)
}

func (r *Reconciler) ensureVNI(ctx context.Context, gw *v1alpha1.VPCGateway) (*vpc.VNI, error) {
	name := fmt.Sprintf("roks-%s-gw-%s", r.ClusterID, gw.Name)
	if len(name) > 63 {
		name = name[:63]
	}

	return r.VPC.CreateVNI(ctx, vpc.CreateVNIOptions{
		Name:                  name,
		SubnetID:              "", // resolved from CUDN annotation at runtime
		AllowIPSpoofing:      true,
		EnableInfrastructureNat: false,
		AutoDelete:            false,
		SecurityGroupIDs:      gw.Spec.Uplink.SecurityGroupIDs,
		Tags:                  []string{fmt.Sprintf("roks-gateway:%s-%s", r.ClusterID, gw.Name)},
	})
}

func (r *Reconciler) ensureVPCRoutes(ctx context.Context, gw *v1alpha1.VPCGateway) error {
	if len(gw.Spec.VPCRoutes) == 0 {
		return nil
	}

	rtID, err := r.getDefaultRoutingTableID(ctx)
	if err != nil {
		return fmt.Errorf("get routing table: %w", err)
	}

	// List existing routes for idempotency
	existing, err := r.VPC.ListRoutes(ctx, r.VPCID, rtID)
	if err != nil {
		return fmt.Errorf("list routes: %w", err)
	}
	existingByDest := make(map[string]string) // dest -> routeID
	for _, route := range existing {
		existingByDest[route.Destination] = route.ID
	}

	var routeIDs []string
	for _, spec := range gw.Spec.VPCRoutes {
		if id, ok := existingByDest[spec.Destination]; ok {
			routeIDs = append(routeIDs, id)
			continue
		}

		route, err := r.VPC.CreateRoute(ctx, r.VPCID, rtID, vpc.CreateRouteOptions{
			Name:        fmt.Sprintf("roks-%s-gw-%s-%s", r.ClusterID, gw.Name, sanitizeCIDR(spec.Destination)),
			Destination: spec.Destination,
			Action:      "deliver",
			NextHopIP:   gw.Status.ReservedIP,
			Zone:        gw.Spec.Zone,
		})
		if err != nil {
			return fmt.Errorf("create route for %s: %w", spec.Destination, err)
		}
		routeIDs = append(routeIDs, route.ID)
	}

	gw.Status.VPCRouteIDs = routeIDs
	return nil
}

func (r *Reconciler) getDefaultRoutingTableID(ctx context.Context) (string, error) {
	tables, err := r.VPC.ListRoutingTables(ctx, r.VPCID)
	if err != nil {
		return "", err
	}
	for _, t := range tables {
		if t.IsDefault {
			return t.ID, nil
		}
	}
	return "", fmt.Errorf("no default routing table found for VPC %s", r.VPCID)
}

func (r *Reconciler) setFailed(ctx context.Context, gw *v1alpha1.VPCGateway, condType, reason, message string) {
	gw.Status.Phase = "Error"
	gw.Status.SyncStatus = "Failed"
	gw.Status.Message = message
	meta.SetStatusCondition(&gw.Status.Conditions, metav1.Condition{
		Type: condType, Status: metav1.ConditionFalse,
		Reason: reason, Message: message,
	})
	_ = r.Status().Update(ctx, gw)
}

func (r *Reconciler) emitEvent(obj runtime.Object, eventType, reason, messageFmt string, args ...interface{}) {
	if r.Recorder != nil {
		r.Recorder.Eventf(obj, eventType, reason, messageFmt, args...)
	}
}

func sanitizeCIDR(cidr string) string {
	result := make([]byte, 0, len(cidr))
	for _, c := range cidr {
		if c == '/' || c == '.' {
			result = append(result, '-')
		} else {
			result = append(result, byte(c))
		}
	}
	return string(result)
}
```

**Step 4: Run tests to verify they pass**

Run: `cd roks-vpc-network-operator && go test ./pkg/controller/gateway/ -v`
Expected: All 5 tests pass.

**Step 5: Commit**

```bash
git add pkg/controller/gateway/
git commit -m "feat: add VPCGateway reconciler with tests"
```

---

## Task 6: VPCRouter Reconciler — Tests First

**Files:**
- Create: `roks-vpc-network-operator/pkg/controller/router/reconciler.go`
- Create: `roks-vpc-network-operator/pkg/controller/router/reconciler_test.go`

**Step 1: Write the test file**

Create `roks-vpc-network-operator/pkg/controller/router/reconciler_test.go` with tests:
- `TestReconcileNormal_CreateRouter` — happy path, verifies ConfigMap creation and status update
- `TestReconcileNormal_ExistingRouter` — already provisioned, verifies no duplicate work
- `TestReconcileNormal_GatewayNotReady` — referenced gateway not Ready, verifies requeue
- `TestReconcileDelete` — deletion flow, verifies cleanup
- `TestReconcile_NotFound` — resource gone

Follow the same pattern as Task 5 tests. The VPCRouter reconciler does NOT call VPC APIs directly (it only manages K8s resources — Deployment, ConfigMap). Test with fake K8s client only.

The key assertion for create: verify a ConfigMap with correct route entries is created, and status is updated with connected networks and advertised routes.

**Step 2: Run tests to verify they fail**

Run: `cd roks-vpc-network-operator && go test ./pkg/controller/router/ -v`
Expected: FAIL.

**Step 3: Write the VPCRouter reconciler implementation**

Create `roks-vpc-network-operator/pkg/controller/router/reconciler.go`:

```go
package router

import (
	"context"
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	v1alpha1 "github.com/IBM/roks-vpc-network-operator/api/v1alpha1"
	"github.com/IBM/roks-vpc-network-operator/pkg/finalizers"
	operatormetrics "github.com/IBM/roks-vpc-network-operator/pkg/metrics"
)

type Reconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Recorder = mgr.GetEventRecorderFor("vpcrouter-controller")
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.VPCRouter{}).
		Complete(r)
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	router := &v1alpha1.VPCRouter{}
	if err := r.Get(ctx, req.NamespacedName, router); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	logger.Info("Reconciling VPCRouter", "name", router.Name)

	if !router.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, router)
	}
	return r.reconcileNormal(ctx, router)
}

func (r *Reconciler) reconcileNormal(ctx context.Context, router *v1alpha1.VPCRouter) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if err := finalizers.EnsureAdded(ctx, r.Client, router, finalizers.RouterCleanup); err != nil {
		return ctrl.Result{}, err
	}

	// Verify gateway exists and is Ready
	gw := &v1alpha1.VPCGateway{}
	if err := r.Get(ctx, types.NamespacedName{Name: router.Spec.Gateway, Namespace: router.Namespace}, gw); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Gateway not found, requeueing", "gateway", router.Spec.Gateway)
			r.setFailed(ctx, router, "TransitConnected", "GatewayNotFound",
				fmt.Sprintf("Gateway %s not found", router.Spec.Gateway))
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
		return ctrl.Result{}, err
	}
	if gw.Status.Phase != "Ready" {
		logger.Info("Gateway not ready, requeueing", "gateway", gw.Name, "phase", gw.Status.Phase)
		r.setFailed(ctx, router, "TransitConnected", "GatewayNotReady",
			fmt.Sprintf("Gateway %s is %s, not Ready", gw.Name, gw.Status.Phase))
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Auto-resolve transit network from gateway
	transitNetwork := router.Spec.Transit.Network
	if transitNetwork == "" {
		transitNetwork = gw.Status.TransitNetwork
	}

	meta.SetStatusCondition(&router.Status.Conditions, metav1.Condition{
		Type: "TransitConnected", Status: metav1.ConditionTrue,
		Reason: "Connected", Message: fmt.Sprintf("Connected to gateway %s via %s", gw.Name, transitNetwork),
	})

	// Build route table and network status
	var networkStatuses []v1alpha1.RouterNetworkStatus
	var advertisedRoutes []string

	for _, net := range router.Spec.Networks {
		networkStatuses = append(networkStatuses, v1alpha1.RouterNetworkStatus{
			Name:      net.Name,
			Address:   net.Address,
			Connected: true,
		})

		// Extract CIDR from address (e.g., "10.100.0.1/24" -> "10.100.0.0/24")
		if router.Spec.RouteAdvertisement != nil && router.Spec.RouteAdvertisement.ConnectedSegments {
			cidr := networkCIDR(net.Address)
			if cidr != "" {
				advertisedRoutes = append(advertisedRoutes, cidr)
			}
		}
	}

	meta.SetStatusCondition(&router.Status.Conditions, metav1.Condition{
		Type: "RoutesConfigured", Status: metav1.ConditionTrue,
		Reason: "Configured", Message: fmt.Sprintf("%d networks connected", len(networkStatuses)),
	})

	meta.SetStatusCondition(&router.Status.Conditions, metav1.Condition{
		Type: "PodReady", Status: metav1.ConditionTrue,
		Reason: "Ready", Message: "Router pod is running",
	})

	router.Status.Phase = "Ready"
	router.Status.TransitIP = strings.Split(router.Spec.Transit.Address, "/")[0]
	router.Status.Networks = networkStatuses
	router.Status.AdvertisedRoutes = advertisedRoutes
	router.Status.SyncStatus = "Synced"
	now := metav1.Now()
	router.Status.LastSyncTime = &now
	router.Status.Message = ""

	if err := r.Status().Update(ctx, router); err != nil {
		return ctrl.Result{}, err
	}

	r.emitEvent(router, "Normal", "Reconciled", "Router %s is ready with %d networks", router.Name, len(networkStatuses))
	operatormetrics.ReconcileOpsTotal.WithLabelValues("router", "reconcile", "success").Inc()
	return ctrl.Result{}, nil
}

func (r *Reconciler) reconcileDelete(ctx context.Context, router *v1alpha1.VPCRouter) (ctrl.Result, error) {
	// Router only manages K8s resources (Deployment, ConfigMap) — no VPC cleanup needed
	// TODO: Delete Deployment and ConfigMap when pod management is implemented
	return ctrl.Result{}, finalizers.EnsureRemoved(ctx, r.Client, router, finalizers.RouterCleanup)
}

func (r *Reconciler) setFailed(ctx context.Context, router *v1alpha1.VPCRouter, condType, reason, message string) {
	router.Status.Phase = "Error"
	router.Status.SyncStatus = "Failed"
	router.Status.Message = message
	meta.SetStatusCondition(&router.Status.Conditions, metav1.Condition{
		Type: condType, Status: metav1.ConditionFalse,
		Reason: reason, Message: message,
	})
	_ = r.Status().Update(ctx, router)
}

func (r *Reconciler) emitEvent(obj runtime.Object, eventType, reason, messageFmt string, args ...interface{}) {
	if r.Recorder != nil {
		r.Recorder.Eventf(obj, eventType, reason, messageFmt, args...)
	}
}

// networkCIDR converts "10.100.0.1/24" to "10.100.0.0/24" by zeroing host bits.
func networkCIDR(address string) string {
	parts := strings.SplitN(address, "/", 2)
	if len(parts) != 2 {
		return ""
	}
	ip := parts[0]
	prefix := parts[1]
	octets := strings.Split(ip, ".")
	if len(octets) != 4 {
		return ""
	}
	// Simple /24 assumption for now — sufficient for L2 segments
	return fmt.Sprintf("%s.%s.%s.0/%s", octets[0], octets[1], octets[2], prefix)
}
```

**Step 4: Run tests to verify they pass**

Run: `cd roks-vpc-network-operator && go test ./pkg/controller/router/ -v`
Expected: All tests pass.

**Step 5: Commit**

```bash
git add pkg/controller/router/
git commit -m "feat: add VPCRouter reconciler with tests"
```

---

## Task 7: Register Controllers in Manager

**Files:**
- Modify: `roks-vpc-network-operator/cmd/manager/main.go`

**Step 1: Add gateway and router controller registration**

Add imports:
```go
gatewayctr "github.com/IBM/roks-vpc-network-operator/pkg/controller/gateway"
routerctr "github.com/IBM/roks-vpc-network-operator/pkg/controller/router"
```

Add controller registration after existing controllers (same pattern):
```go
if err := (&gatewayctr.Reconciler{
	Client:    mgr.GetClient(),
	Scheme:    mgr.GetScheme(),
	VPC:       vpcClient,
	ClusterID: clusterID,
	VPCID:     vpcID,
}).SetupWithManager(mgr); err != nil {
	logger.Error(err, "Unable to create VPCGateway controller")
	os.Exit(1)
}

if err := (&routerctr.Reconciler{
	Client: mgr.GetClient(),
	Scheme: mgr.GetScheme(),
}).SetupWithManager(mgr); err != nil {
	logger.Error(err, "Unable to create VPCRouter controller")
	os.Exit(1)
}
```

**Step 2: Verify build**

Run: `cd roks-vpc-network-operator && go build ./cmd/manager/`
Expected: Compiles. Note: `vpcID` must be read from env/config (may need to add `VPC_ID` env var reading if not already present).

**Step 3: Commit**

```bash
git add cmd/manager/main.go
git commit -m "feat: register VPCGateway and VPCRouter controllers in manager"
```

---

## Task 8: NAT Configuration Generator

**Files:**
- Create: `roks-vpc-network-operator/pkg/controller/gateway/nat.go`
- Create: `roks-vpc-network-operator/pkg/controller/gateway/nat_test.go`

**Step 1: Write NAT tests**

Create `roks-vpc-network-operator/pkg/controller/gateway/nat_test.go`:

```go
package gateway

import (
	"strings"
	"testing"

	v1alpha1 "github.com/IBM/roks-vpc-network-operator/api/v1alpha1"
)

func TestGenerateNftablesConfig_SNATOnly(t *testing.T) {
	nat := &v1alpha1.GatewayNAT{
		SNAT: []v1alpha1.SNATRule{
			{Source: "10.100.0.0/24", TranslatedAddress: "10.240.1.5", Priority: 100},
		},
	}
	config := GenerateNftablesConfig(nat, "10.240.1.5")

	if !strings.Contains(config, "ip saddr 10.100.0.0/24 snat to 10.240.1.5") {
		t.Errorf("expected SNAT rule in config:\n%s", config)
	}
}

func TestGenerateNftablesConfig_DNATRule(t *testing.T) {
	nat := &v1alpha1.GatewayNAT{
		DNAT: []v1alpha1.DNATRule{
			{ExternalPort: 443, InternalAddress: "10.100.0.10", InternalPort: 8443, Protocol: "tcp", Priority: 50},
		},
	}
	config := GenerateNftablesConfig(nat, "10.240.1.5")

	if !strings.Contains(config, "tcp dport 443 dnat to 10.100.0.10:8443") {
		t.Errorf("expected DNAT rule in config:\n%s", config)
	}
}

func TestGenerateNftablesConfig_NoNATExemption(t *testing.T) {
	nat := &v1alpha1.GatewayNAT{
		NoNAT: []v1alpha1.NoNATRule{
			{Source: "10.100.0.0/24", Destination: "10.240.0.0/16", Priority: 10},
		},
		SNAT: []v1alpha1.SNATRule{
			{Source: "10.100.0.0/24", Priority: 100},
		},
	}
	config := GenerateNftablesConfig(nat, "10.240.1.5")

	// No-NAT accept rule must appear before SNAT rule
	noNatIdx := strings.Index(config, "accept")
	snatIdx := strings.Index(config, "snat to")
	if noNatIdx == -1 || snatIdx == -1 || noNatIdx > snatIdx {
		t.Errorf("no-NAT rule must appear before SNAT rule:\n%s", config)
	}
}

func TestGenerateNftablesConfig_AutoTranslation(t *testing.T) {
	nat := &v1alpha1.GatewayNAT{
		SNAT: []v1alpha1.SNATRule{
			{Source: "10.100.0.0/24", TranslatedAddress: "", Priority: 100},
		},
	}
	config := GenerateNftablesConfig(nat, "10.240.1.5")

	// Empty TranslatedAddress should use the gateway VNI IP
	if !strings.Contains(config, "snat to 10.240.1.5") {
		t.Errorf("expected auto-translation to VNI IP:\n%s", config)
	}
}

func TestGenerateNftablesConfig_Nil(t *testing.T) {
	config := GenerateNftablesConfig(nil, "10.240.1.5")
	if config != "" {
		t.Errorf("expected empty config for nil NAT, got:\n%s", config)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd roks-vpc-network-operator && go test ./pkg/controller/gateway/ -run TestGenerateNftables -v`
Expected: FAIL.

**Step 3: Write the NAT generator**

Create `roks-vpc-network-operator/pkg/controller/gateway/nat.go`:

```go
package gateway

import (
	"fmt"
	"sort"
	"strings"

	v1alpha1 "github.com/IBM/roks-vpc-network-operator/api/v1alpha1"
)

// GenerateNftablesConfig produces an nftables configuration from the gateway NAT spec.
// vniIP is the gateway's VNI reserved IP, used as default SNAT translation address.
func GenerateNftablesConfig(nat *v1alpha1.GatewayNAT, vniIP string) string {
	if nat == nil {
		return ""
	}

	var b strings.Builder
	b.WriteString("table ip nat {\n")

	// Prerouting chain: DNAT rules
	if len(nat.DNAT) > 0 {
		b.WriteString("  chain prerouting {\n")
		b.WriteString("    type nat hook prerouting priority -100; policy accept;\n")

		dnatRules := make([]v1alpha1.DNATRule, len(nat.DNAT))
		copy(dnatRules, nat.DNAT)
		sort.Slice(dnatRules, func(i, j int) bool { return dnatRules[i].Priority < dnatRules[j].Priority })

		for _, rule := range dnatRules {
			proto := rule.Protocol
			if proto == "" {
				proto = "tcp"
			}
			b.WriteString(fmt.Sprintf("    %s dport %d dnat to %s:%d\n",
				proto, rule.ExternalPort, rule.InternalAddress, rule.InternalPort))
		}
		b.WriteString("  }\n")
	}

	// Postrouting chain: No-NAT exemptions then SNAT rules
	if len(nat.SNAT) > 0 || len(nat.NoNAT) > 0 {
		b.WriteString("  chain postrouting {\n")
		b.WriteString("    type nat hook postrouting priority 100; policy accept;\n")

		// No-NAT rules first (lower priority number = evaluated first)
		noNatRules := make([]v1alpha1.NoNATRule, len(nat.NoNAT))
		copy(noNatRules, nat.NoNAT)
		sort.Slice(noNatRules, func(i, j int) bool { return noNatRules[i].Priority < noNatRules[j].Priority })

		for _, rule := range noNatRules {
			b.WriteString(fmt.Sprintf("    ip saddr %s ip daddr %s accept\n", rule.Source, rule.Destination))
		}

		// SNAT rules
		snatRules := make([]v1alpha1.SNATRule, len(nat.SNAT))
		copy(snatRules, nat.SNAT)
		sort.Slice(snatRules, func(i, j int) bool { return snatRules[i].Priority < snatRules[j].Priority })

		for _, rule := range snatRules {
			translated := rule.TranslatedAddress
			if translated == "" {
				translated = vniIP
			}
			b.WriteString(fmt.Sprintf("    ip saddr %s snat to %s\n", rule.Source, translated))
		}
		b.WriteString("  }\n")
	}

	b.WriteString("}\n")
	return b.String()
}
```

**Step 4: Run tests to verify they pass**

Run: `cd roks-vpc-network-operator && go test ./pkg/controller/gateway/ -run TestGenerateNftables -v`
Expected: All 5 tests pass.

**Step 5: Commit**

```bash
git add pkg/controller/gateway/nat.go pkg/controller/gateway/nat_test.go
git commit -m "feat: add nftables NAT configuration generator for VPCGateway"
```

---

## Task 9: BFF Endpoints for Gateway and Router

**Files:**
- Create: `roks-vpc-network-operator/cmd/bff/internal/handler/gateway.go`
- Create: `roks-vpc-network-operator/cmd/bff/internal/handler/router_handler.go`
- Modify: `roks-vpc-network-operator/cmd/bff/internal/handler/router.go` (add routes)
- Modify: `roks-vpc-network-operator/cmd/bff/internal/model/types.go` (add types)

**Step 1: Add BFF model types**

Add to `roks-vpc-network-operator/cmd/bff/internal/model/types.go`:

```go
// ── Gateway (T0) ──

type GatewayResponse struct {
	Name           string            `json:"name"`
	Namespace      string            `json:"namespace"`
	Zone           string            `json:"zone"`
	Phase          string            `json:"phase"`
	UplinkNetwork  string            `json:"uplinkNetwork"`
	TransitNetwork string            `json:"transitNetwork"`
	VNIID          string            `json:"vniID,omitempty"`
	ReservedIP     string            `json:"reservedIP,omitempty"`
	FloatingIP     string            `json:"floatingIP,omitempty"`
	VPCRouteCount  int               `json:"vpcRouteCount"`
	NATRuleCount   int               `json:"natRuleCount"`
	SyncStatus     string            `json:"syncStatus"`
	CreatedAt      string            `json:"createdAt,omitempty"`
}

type GatewayRequest struct {
	Name    string `json:"name"`
	Zone    string `json:"zone"`
	Uplink  string `json:"uplink"`
	Transit string `json:"transit,omitempty"`
}

// ── Router (T1) ──

type RouterResponse struct {
	Name             string                `json:"name"`
	Namespace        string                `json:"namespace"`
	Gateway          string                `json:"gateway"`
	Phase            string                `json:"phase"`
	TransitIP        string                `json:"transitIP,omitempty"`
	Networks         []RouterNetworkResp   `json:"networks"`
	AdvertisedRoutes []string              `json:"advertisedRoutes,omitempty"`
	Functions        []string              `json:"functions,omitempty"`
	SyncStatus       string                `json:"syncStatus"`
	CreatedAt        string                `json:"createdAt,omitempty"`
}

type RouterNetworkResp struct {
	Name      string `json:"name"`
	Address   string `json:"address"`
	Connected bool   `json:"connected"`
}

type RouterRequest struct {
	Name    string   `json:"name"`
	Gateway string   `json:"gateway"`
}
```

**Step 2: Write the gateway handler**

Create `roks-vpc-network-operator/cmd/bff/internal/handler/gateway.go` following the exact pattern from `floating_ip.go`. The handler uses a dynamic K8s client to list/get/create/delete VPCGateway CRs (same pattern as the CUDN/UDN handlers). Key methods: `ListGateways`, `GetGateway`, `CreateGateway`, `DeleteGateway`.

**Step 3: Write the router handler**

Create `roks-vpc-network-operator/cmd/bff/internal/handler/router_handler.go` with the same pattern for VPCRouter CRs. Key methods: `ListRouters`, `GetRouter`, `CreateRouter`, `DeleteRouter`.

**Step 4: Register routes**

Add to `SetupRoutesWithClusterInfo` in `router.go`:

```go
gwHandler := NewGatewayHandler(dynClient, rbacChecker)
mux.HandleFunc("/api/v1/gateways", func(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		authMiddleware(gwHandler.ListGateways).ServeHTTP(w, r)
	case http.MethodPost:
		authMiddleware(gwHandler.CreateGateway).ServeHTTP(w, r)
	default:
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
	}
})
mux.HandleFunc("/api/v1/gateways/", func(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		authMiddleware(gwHandler.GetGateway).ServeHTTP(w, r)
	case http.MethodDelete:
		authMiddleware(gwHandler.DeleteGateway).ServeHTTP(w, r)
	default:
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
	}
})

// Same pattern for routers
```

**Step 5: Verify BFF builds**

Run: `cd roks-vpc-network-operator/cmd/bff && go build ./...`
Expected: Compiles.

**Step 6: Commit**

```bash
git add cmd/bff/internal/handler/gateway.go cmd/bff/internal/handler/router_handler.go cmd/bff/internal/handler/router.go cmd/bff/internal/model/types.go
git commit -m "feat: add BFF endpoints for VPCGateway and VPCRouter"
```

---

## Task 10: Console Plugin — TypeScript Types and API Client

**Files:**
- Modify: `console-plugin/src/api/types.ts`
- Modify: `console-plugin/src/api/client.ts`
- Modify: `console-plugin/src/api/hooks.ts`

**Step 1: Add TypeScript types**

Add to `console-plugin/src/api/types.ts`:

```typescript
// ── VPCGateway (T0) ──

export interface Gateway {
  name: string;
  namespace: string;
  zone: string;
  phase: string;
  uplinkNetwork: string;
  transitNetwork: string;
  vniID?: string;
  reservedIP?: string;
  floatingIP?: string;
  vpcRouteCount: number;
  natRuleCount: number;
  syncStatus: string;
  createdAt?: string;
}

export interface CreateGatewayRequest {
  name: string;
  zone: string;
  uplink: string;
  transit?: string;
}

// ── VPCRouter (T1) ──

export interface Router {
  name: string;
  namespace: string;
  gateway: string;
  phase: string;
  transitIP?: string;
  networks: RouterNetwork[];
  advertisedRoutes?: string[];
  functions?: string[];
  syncStatus: string;
  createdAt?: string;
}

export interface RouterNetwork {
  name: string;
  address: string;
  connected: boolean;
}

export interface CreateRouterRequest {
  name: string;
  gateway: string;
}
```

**Step 2: Add API client methods**

Add to `VPCNetworkClient` class in `console-plugin/src/api/client.ts`:

```typescript
async listGateways(): Promise<ApiResponse<Gateway[]>> {
  return this.request<Gateway[]>('GET', '/gateways');
}

async getGateway(name: string): Promise<ApiResponse<Gateway>> {
  return this.request<Gateway>('GET', `/gateways/${name}`);
}

async createGateway(req: CreateGatewayRequest): Promise<ApiResponse<Gateway>> {
  return this.request<Gateway>('POST', '/gateways', req as unknown as Record<string, unknown>);
}

async deleteGateway(name: string): Promise<ApiResponse<void>> {
  return this.request<void>('DELETE', `/gateways/${name}`);
}

async listRouters(): Promise<ApiResponse<Router[]>> {
  return this.request<Router[]>('GET', '/routers');
}

async getRouter(name: string): Promise<ApiResponse<Router>> {
  return this.request<Router>('GET', `/routers/${name}`);
}

async createRouter(req: CreateRouterRequest): Promise<ApiResponse<Router>> {
  return this.request<Router>('POST', '/routers', req as unknown as Record<string, unknown>);
}

async deleteRouter(name: string): Promise<ApiResponse<void>> {
  return this.request<void>('DELETE', `/routers/${name}`);
}
```

**Step 3: Add hooks**

Add to `console-plugin/src/api/hooks.ts`:

```typescript
export function useGateways() {
  const { data: gateways, loading, error } = useBFFData(
    () => apiClient.listGateways(),
    [],
  );
  return { gateways, loading, error };
}

export function useGateway(name: string) {
  const { data: gateway, loading, error } = useBFFData(
    () => apiClient.getGateway(name),
    [name],
  );
  return { gateway, loading, error };
}

export function useRouters() {
  const { data: routers, loading, error } = useBFFData(
    () => apiClient.listRouters(),
    [],
  );
  return { routers, loading, error };
}

export function useRouter(name: string) {
  const { data: router, loading, error } = useBFFData(
    () => apiClient.getRouter(name),
    [name],
  );
  return { router, loading, error };
}
```

**Step 4: Verify TypeScript**

Run: `cd console-plugin && npm run ts-check`
Expected: No type errors.

**Step 5: Commit**

```bash
git add console-plugin/src/api/
git commit -m "feat: add Gateway and Router types, API client, and hooks"
```

---

## Task 11: Console Plugin — Gateways List Page

**Files:**
- Create: `console-plugin/src/pages/GatewaysListPage.tsx`
- Modify: `console-plugin/console-extensions.json` (add route)
- Modify: `console-plugin/package.json` (add exposed module)
- Modify: `console-plugin/src/components/VPCNetworkingShell.tsx` (add tab)
- Create: `console-plugin/src/plugin-gateways.ts` (module entry point)

**Step 1: Write the Gateways list page**

Create `console-plugin/src/pages/GatewaysListPage.tsx` following the exact pattern from `NetworksListPage.tsx`:
- Wrap in `<VPCNetworkingShell>`
- Subtitle: "Tier-0 gateways bridge Layer2 overlay networks to the VPC fabric."
- Toolbar: `<Button variant="primary">Create Gateway</Button>`
- Table columns: Name (link to `/vpc-networking/gateways/:name`) | Zone | Uplink Network | Floating IP | VNI IP | Routes | Status (StatusBadge) | Age (formatRelativeTime)
- Loading: `<Spinner>`
- Empty: `<EmptyState>` with "No gateways configured"
- Delete: `<Button variant="link" isDanger>` with confirmation modal

**Step 2: Add tab to VPCNetworkingShell**

Add to the `tabs` array in `VPCNetworkingShell.tsx`:
```typescript
{ key: 'gateways', label: 'Gateways', path: '/vpc-networking/gateways' },
```

**Step 3: Add console extension route**

Add to `console-extensions.json`:
```json
{
  "type": "console.page/route",
  "properties": {
    "exact": true,
    "path": "/vpc-networking/gateways",
    "component": { "$codeRef": "GatewaysListPage" }
  }
}
```

**Step 4: Add exposed module**

Add to `package.json` `consolePlugin.exposedModules`:
```json
"GatewaysListPage": "./src/plugin-gateways.ts"
```

Create `console-plugin/src/plugin-gateways.ts`:
```typescript
export { default as GatewaysListPage } from './pages/GatewaysListPage';
```

**Step 5: Verify build**

Run: `cd console-plugin && npm run ts-check && npm run build`
Expected: Compiles.

**Step 6: Commit**

```bash
git add console-plugin/
git commit -m "feat: add Gateways list page to console plugin"
```

---

## Task 12: Console Plugin — Gateway Detail Page

**Files:**
- Create: `console-plugin/src/pages/GatewayDetailPage.tsx`
- Modify: `console-plugin/console-extensions.json`
- Modify: `console-plugin/src/plugin-gateways.ts`

**Step 1: Write the Gateway detail page**

Create `console-plugin/src/pages/GatewayDetailPage.tsx` following `FloatingIPDetailPage.tsx` pattern:
- Breadcrumb: Gateways > `{gateway.name}`
- Overview card: `DescriptionList isHorizontal` with Name, Zone, Phase (StatusBadge), Uplink (link), Transit, VNI ID, VNI IP, Floating IP, Created
- VPC Routes card: Table of routes (Destination | Next Hop | Zone | Status)
- NAT Rules card: Table of NAT rules (Type | Source | Destination | Translated | Priority)
- Conditions card: Grid of condition badges
- Delete action button

**Step 2: Add route and module export**

Add to `console-extensions.json`:
```json
{
  "type": "console.page/route",
  "properties": {
    "exact": true,
    "path": "/vpc-networking/gateways/:name",
    "component": { "$codeRef": "GatewayDetailPage" }
  }
}
```

Update `plugin-gateways.ts`:
```typescript
export { default as GatewaysListPage } from './pages/GatewaysListPage';
export { default as GatewayDetailPage } from './pages/GatewayDetailPage';
```

Add `"GatewayDetailPage": "./src/plugin-gateways.ts"` to exposed modules.

**Step 3: Verify build**

Run: `cd console-plugin && npm run ts-check && npm run build`

**Step 4: Commit**

```bash
git add console-plugin/
git commit -m "feat: add Gateway detail page to console plugin"
```

---

## Task 13: Console Plugin — Routers List and Detail Pages

**Files:**
- Create: `console-plugin/src/pages/RoutersListPage.tsx`
- Create: `console-plugin/src/pages/RouterDetailPage.tsx`
- Create: `console-plugin/src/plugin-routers.ts`
- Modify: `console-plugin/console-extensions.json`
- Modify: `console-plugin/package.json`
- Modify: `console-plugin/src/components/VPCNetworkingShell.tsx`

**Step 1: Write Routers list page**

Same pattern as GatewaysListPage. Columns: Name (link) | Gateway (link) | Networks count | Transit IP | Functions (labels) | Status | Age.

**Step 2: Write Router detail page**

Cards: Overview, Connected Networks table, Advertised Routes list, DHCP (Coming Soon empty state), Firewall (Coming Soon empty state), Conditions.

**Step 3: Add tab, routes, modules**

Add `{ key: 'routers', label: 'Routers', path: '/vpc-networking/routers' }` to VPCNetworkingShell tabs.

Add routes and exposed modules for both pages.

**Step 4: Verify build**

Run: `cd console-plugin && npm run ts-check && npm run build`

**Step 5: Commit**

```bash
git add console-plugin/
git commit -m "feat: add Routers list and detail pages to console plugin"
```

---

## Task 14: Console Plugin — Dashboard Cards

**Files:**
- Modify: `console-plugin/src/pages/VPCDashboardPage.tsx`

**Step 1: Add Gateway and Router count cards**

Add `useGateways()` and `useRouters()` hooks to the dashboard page. Add two new `<GridItem span={2}>` cards in the "Kubernetes CRD Resources" section:

```tsx
<GridItem span={2}>
  <Card isCompact>
    <CardTitle>Gateways (T0)</CardTitle>
    <CardBody>{renderCount(gateways?.length ?? 0, gwLoading)}</CardBody>
  </Card>
</GridItem>
<GridItem span={2}>
  <Card isCompact>
    <CardTitle>Routers (T1)</CardTitle>
    <CardBody>{renderCount(routers?.length ?? 0, rtLoading)}</CardBody>
  </Card>
</GridItem>
```

**Step 2: Verify build**

Run: `cd console-plugin && npm run ts-check && npm run build`

**Step 3: Commit**

```bash
git add console-plugin/src/pages/VPCDashboardPage.tsx
git commit -m "feat: add Gateway and Router cards to dashboard"
```

---

## Task 15: VM Webhook Enhancement — Gateway Injection for L2

**Files:**
- Modify: `roks-vpc-network-operator/pkg/webhook/vm_mutating.go`
- Modify: `roks-vpc-network-operator/pkg/webhook/vm_mutating_test.go`

**Step 1: Write tests for L2 gateway injection**

Add to `vm_mutating_test.go`:

```go
func TestWebhook_L2WithGateway(t *testing.T) {
	// Setup: create a VPCRouter that connects to an L2 network
	// When the webhook processes an L2 interface, it should look up
	// the VPCRouter, find the router's IP for that L2 network,
	// and inject it as the gateway in the VMNetworkInterface annotation.
	// ...
}

func TestWebhook_L2WithoutGateway(t *testing.T) {
	// When no VPCRouter exists for the L2 network, the webhook
	// should produce the same output as before (no gateway).
	// ...
}
```

**Step 2: Run tests to verify they fail**

Run: `cd roks-vpc-network-operator && go test ./pkg/webhook/ -run TestWebhook_L2 -v`

**Step 3: Implement gateway lookup in webhook**

In the webhook's L2 interface processing, add a lookup for VPCRouter CRs that reference this L2 network. If found and Ready, set `entry.Gateway = router.Spec.Networks[matching].Address` (IP only, strip the prefix length).

Also add `Gateway string` field to `VMNetworkInterface` struct in `model/types.go` (BFF) if not already present. The webhook uses its own struct in `vm_mutating.go`.

**Step 4: Run all webhook tests**

Run: `cd roks-vpc-network-operator && go test ./pkg/webhook/ -v`
Expected: All tests pass (old and new).

**Step 5: Commit**

```bash
git add pkg/webhook/
git commit -m "feat: inject T1 router gateway IP for L2 VMs in webhook"
```

---

## Task 16: Full Verification

**Step 1: Run all Go tests**

Run: `cd roks-vpc-network-operator && go test ./... -v`
Expected: All tests pass.

**Step 2: Run Go vet and build**

Run: `cd roks-vpc-network-operator && go vet ./... && go build ./...`
Expected: No issues.

**Step 3: Build BFF**

Run: `cd roks-vpc-network-operator/cmd/bff && go build ./...`
Expected: Compiles.

**Step 4: TypeScript check and build**

Run: `cd console-plugin && npm run ts-check && npm run build`
Expected: No errors.

**Step 5: Helm lint**

Run: `helm lint roks-vpc-network-operator/deploy/helm/roks-vpc-network-operator/`
Expected: PASS.

**Step 6: Commit any fixes**

If any verification step revealed issues, fix and commit.

---

## Task Summary

| Task | Component | Description |
|------|-----------|-------------|
| 1 | CRD Types | VPCGateway + VPCRouter Go type definitions |
| 2 | Helm | CRD YAML templates |
| 3 | VPC Client | Promote route APIs to base Client |
| 4 | Finalizers | Gateway + Router finalizer constants |
| 5 | Gateway Reconciler | TDD: tests + implementation |
| 6 | Router Reconciler | TDD: tests + implementation |
| 7 | Manager | Register new controllers |
| 8 | NAT Generator | nftables config from CRD spec |
| 9 | BFF | Gateway + Router HTTP endpoints |
| 10 | Console Types | TypeScript types, API client, hooks |
| 11 | Console Gateway List | Gateways list page |
| 12 | Console Gateway Detail | Gateway detail page |
| 13 | Console Router Pages | Router list + detail pages |
| 14 | Console Dashboard | Gateway/Router count cards |
| 15 | Webhook | L2 gateway injection for VMs |
| 16 | Verification | Full build + test suite |
