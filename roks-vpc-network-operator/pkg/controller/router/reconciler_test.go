package router

import (
	"context"
	"encoding/json"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1alpha1 "github.com/IBM/roks-vpc-network-operator/api/v1alpha1"
)

func newTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(s)
	_ = v1alpha1.AddToScheme(s)
	return s
}

// TestReconcileNormal_CreateRouter tests the happy path: gateway exists and is Ready,
// router connects to L2 networks, status is updated correctly.
func TestReconcileNormal_CreateRouter(t *testing.T) {
	scheme := newTestScheme()

	gw := &v1alpha1.VPCGateway{
		ObjectMeta: metav1.ObjectMeta{Name: "gw-test", Namespace: "default"},
		Spec: v1alpha1.VPCGatewaySpec{
			Zone: "eu-de-1",
			Uplink: v1alpha1.GatewayUplink{
				Network: "uplink-net",
			},
		},
		Status: v1alpha1.VPCGatewayStatus{
			Phase:          "Ready",
			VNIID:          "vni-gw-123",
			MACAddress:     "fa:16:3e:aa:bb:cc",
			ReservedIP:     "10.240.1.5",
			TransitNetwork: "gw-test-transit",
		},
	}

	router := &v1alpha1.VPCRouter{
		ObjectMeta: metav1.ObjectMeta{Name: "router-test", Namespace: "default"},
		Spec: v1alpha1.VPCRouterSpec{
			Gateway: "gw-test",
			Transit: &v1alpha1.RouterTransit{Address: "172.16.0.2/24"},
			Networks: []v1alpha1.RouterNetwork{
				{Name: "l2-app", Address: "10.100.0.1/24"},
				{Name: "l2-db", Address: "10.200.0.1/24"},
			},
			RouteAdvertisement: &v1alpha1.RouteAdvertisement{ConnectedSegments: true},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(gw, router).
		WithStatusSubresource(gw, router).
		Build()

	r := &Reconciler{
		Client: fakeClient,
		Scheme: scheme,
	}

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "router-test", Namespace: "default"},
	})

	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	// Pod was just created, not Running yet — should requeue
	if result.RequeueAfter == 0 {
		t.Errorf("expected requeue after pod creation, got 0")
	}

	// Verify the updated router status
	updated := &v1alpha1.VPCRouter{}
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "router-test", Namespace: "default"}, updated); err != nil {
		t.Fatalf("Failed to get updated VPCRouter: %v", err)
	}

	// Phase should be Ready
	if updated.Status.Phase != "Ready" {
		t.Errorf("expected Phase = 'Ready', got %q", updated.Status.Phase)
	}

	// SyncStatus should be Synced
	if updated.Status.SyncStatus != "Synced" {
		t.Errorf("expected SyncStatus = 'Synced', got %q", updated.Status.SyncStatus)
	}

	// TransitIP should be the IP stripped of prefix
	if updated.Status.TransitIP != "172.16.0.2" {
		t.Errorf("expected TransitIP = '172.16.0.2', got %q", updated.Status.TransitIP)
	}

	// Networks should have 2 entries, all connected
	if len(updated.Status.Networks) != 2 {
		t.Fatalf("expected 2 network statuses, got %d", len(updated.Status.Networks))
	}
	for i, ns := range updated.Status.Networks {
		if !ns.Connected {
			t.Errorf("expected network %d (%s) to be connected", i, ns.Name)
		}
	}
	if updated.Status.Networks[0].Name != "l2-app" {
		t.Errorf("expected network[0].Name = 'l2-app', got %q", updated.Status.Networks[0].Name)
	}
	if updated.Status.Networks[0].Address != "10.100.0.1/24" {
		t.Errorf("expected network[0].Address = '10.100.0.1/24', got %q", updated.Status.Networks[0].Address)
	}
	if updated.Status.Networks[1].Name != "l2-db" {
		t.Errorf("expected network[1].Name = 'l2-db', got %q", updated.Status.Networks[1].Name)
	}

	// AdvertisedRoutes should contain CIDRs derived from spec.networks addresses
	if len(updated.Status.AdvertisedRoutes) != 2 {
		t.Fatalf("expected 2 advertised routes, got %d", len(updated.Status.AdvertisedRoutes))
	}
	if updated.Status.AdvertisedRoutes[0] != "10.100.0.0/24" {
		t.Errorf("expected advertisedRoutes[0] = '10.100.0.0/24', got %q", updated.Status.AdvertisedRoutes[0])
	}
	if updated.Status.AdvertisedRoutes[1] != "10.200.0.0/24" {
		t.Errorf("expected advertisedRoutes[1] = '10.200.0.0/24', got %q", updated.Status.AdvertisedRoutes[1])
	}

	// Conditions: TransitConnected and RoutesConfigured should be True
	conditionMap := make(map[string]metav1.Condition)
	for _, c := range updated.Status.Conditions {
		conditionMap[c.Type] = c
	}
	for _, cType := range []string{"TransitConnected", "RoutesConfigured"} {
		c, ok := conditionMap[cType]
		if !ok {
			t.Errorf("expected condition %q to be present", cType)
			continue
		}
		if c.Status != metav1.ConditionTrue {
			t.Errorf("expected condition %q status = True, got %v", cType, c.Status)
		}
	}
	// PodReady should be False (pod just created, not Running)
	if podCond, ok := conditionMap["PodReady"]; ok {
		if podCond.Status != metav1.ConditionFalse {
			t.Errorf("expected PodReady status = False (pod just created), got %v", podCond.Status)
		}
	} else {
		t.Error("expected PodReady condition to be present")
	}

	// Verify the router pod was created
	pod := &corev1.Pod{}
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "router-test-pod", Namespace: "default"}, pod); err != nil {
		t.Fatalf("Expected router pod to be created: %v", err)
	}

	// Verify pod labels
	if pod.Labels["vpc.roks.ibm.com/router"] != "router-test" {
		t.Errorf("expected router label = 'router-test', got %q", pod.Labels["vpc.roks.ibm.com/router"])
	}

	// Verify Multus annotation contains uplink with MAC and workload networks
	multusAnnotation := pod.Annotations["k8s.v1.cni.cncf.io/networks"]
	var attachments []multusNetworkAttachment
	if err := json.Unmarshal([]byte(multusAnnotation), &attachments); err != nil {
		t.Fatalf("Failed to parse Multus annotation: %v", err)
	}
	if len(attachments) != 3 { // uplink + 2 workloads
		t.Fatalf("expected 3 Multus attachments, got %d", len(attachments))
	}
	if attachments[0].Interface != "uplink" || attachments[0].MAC != "fa:16:3e:aa:bb:cc" {
		t.Errorf("expected uplink with MAC fa:16:3e:aa:bb:cc, got interface=%q mac=%q",
			attachments[0].Interface, attachments[0].MAC)
	}
	if attachments[1].Name != "l2-app" || attachments[1].Interface != "net0" {
		t.Errorf("expected net0 = l2-app, got %q/%q", attachments[1].Name, attachments[1].Interface)
	}
	if attachments[2].Name != "l2-db" || attachments[2].Interface != "net1" {
		t.Errorf("expected net1 = l2-db, got %q/%q", attachments[2].Name, attachments[2].Interface)
	}

	// Verify the container is privileged
	if !*pod.Spec.Containers[0].SecurityContext.Privileged {
		t.Error("expected router container to be privileged")
	}

	// Verify owner reference
	if len(pod.OwnerReferences) != 1 || pod.OwnerReferences[0].Name != "router-test" {
		t.Errorf("expected owner reference to router-test, got %v", pod.OwnerReferences)
	}
}

// TestReconcileNormal_GatewayNotReady tests that when the referenced gateway
// is not Ready, the reconciler requeues and sets Failed status.
func TestReconcileNormal_GatewayNotReady(t *testing.T) {
	scheme := newTestScheme()

	gw := &v1alpha1.VPCGateway{
		ObjectMeta: metav1.ObjectMeta{Name: "gw-pending", Namespace: "default"},
		Status: v1alpha1.VPCGatewayStatus{
			Phase: "Pending",
		},
	}

	router := &v1alpha1.VPCRouter{
		ObjectMeta: metav1.ObjectMeta{Name: "router-wait", Namespace: "default"},
		Spec: v1alpha1.VPCRouterSpec{
			Gateway: "gw-pending",
			Transit: &v1alpha1.RouterTransit{Address: "172.16.0.2/24"},
			Networks: []v1alpha1.RouterNetwork{
				{Name: "l2-app", Address: "10.100.0.1/24"},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(gw, router).
		WithStatusSubresource(gw, router).
		Build()

	r := &Reconciler{
		Client: fakeClient,
		Scheme: scheme,
	}

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "router-wait", Namespace: "default"},
	})

	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if result.RequeueAfter == 0 {
		t.Error("expected requeue when gateway is not ready")
	}

	updated := &v1alpha1.VPCRouter{}
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "router-wait", Namespace: "default"}, updated); err != nil {
		t.Fatalf("Failed to get updated VPCRouter: %v", err)
	}

	if updated.Status.SyncStatus != "Failed" {
		t.Errorf("expected SyncStatus = 'Failed', got %q", updated.Status.SyncStatus)
	}
	if updated.Status.Phase != "Error" {
		t.Errorf("expected Phase = 'Error', got %q", updated.Status.Phase)
	}
}

// TestReconcileNormal_GatewayNotFound tests that when the referenced gateway
// doesn't exist, the reconciler requeues and sets Failed status.
func TestReconcileNormal_GatewayNotFound(t *testing.T) {
	scheme := newTestScheme()

	router := &v1alpha1.VPCRouter{
		ObjectMeta: metav1.ObjectMeta{Name: "router-orphan", Namespace: "default"},
		Spec: v1alpha1.VPCRouterSpec{
			Gateway: "gw-missing",
			Transit: &v1alpha1.RouterTransit{Address: "172.16.0.2/24"},
			Networks: []v1alpha1.RouterNetwork{
				{Name: "l2-app", Address: "10.100.0.1/24"},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(router).
		WithStatusSubresource(router).
		Build()

	r := &Reconciler{
		Client: fakeClient,
		Scheme: scheme,
	}

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "router-orphan", Namespace: "default"},
	})

	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if result.RequeueAfter == 0 {
		t.Error("expected requeue when gateway is not found")
	}

	updated := &v1alpha1.VPCRouter{}
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "router-orphan", Namespace: "default"}, updated); err != nil {
		t.Fatalf("Failed to get updated VPCRouter: %v", err)
	}

	if updated.Status.SyncStatus != "Failed" {
		t.Errorf("expected SyncStatus = 'Failed', got %q", updated.Status.SyncStatus)
	}
	if updated.Status.Phase != "Error" {
		t.Errorf("expected Phase = 'Error', got %q", updated.Status.Phase)
	}
}

// TestReconcileDelete tests that when a VPCRouter has a DeletionTimestamp set,
// the reconciler deletes the router pod and removes the finalizer.
func TestReconcileDelete(t *testing.T) {
	scheme := newTestScheme()

	now := metav1.Now()
	router := &v1alpha1.VPCRouter{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "router-delete",
			Namespace:         "default",
			DeletionTimestamp: &now,
			Finalizers:        []string{"vpc.roks.ibm.com/router-cleanup"},
		},
		Spec: v1alpha1.VPCRouterSpec{
			Gateway: "gw-test",
			Networks: []v1alpha1.RouterNetwork{
				{Name: "l2-app", Address: "10.100.0.1/24"},
			},
		},
	}

	// Create an existing router pod that should be deleted
	isTrue := true
	existingPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "router-delete-pod",
			Namespace: "default",
			Labels: map[string]string{
				"vpc.roks.ibm.com/router": "router-delete",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "vpc.roks.ibm.com/v1alpha1",
					Kind:       "VPCRouter",
					Name:       "router-delete",
					Controller: &isTrue,
				},
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "router", Image: "quay.io/fedora/fedora:40"},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(router, existingPod).
		WithStatusSubresource(router).
		Build()

	r := &Reconciler{
		Client: fakeClient,
		Scheme: scheme,
	}

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "router-delete", Namespace: "default"},
	})

	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if result.Requeue {
		t.Error("should not requeue after successful delete")
	}

	// Verify the router pod was deleted
	pod := &corev1.Pod{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{Name: "router-delete-pod", Namespace: "default"}, pod)
	if err == nil {
		t.Error("expected router pod to be deleted")
	} else if !errors.IsNotFound(err) {
		t.Fatalf("unexpected error checking for deleted pod: %v", err)
	}

	// Verify finalizer was removed.
	// With the fake client, removing the last finalizer from an object with a
	// DeletionTimestamp causes the object to be garbage-collected, so a
	// not-found error is the expected successful outcome.
	updated := &v1alpha1.VPCRouter{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{Name: "router-delete", Namespace: "default"}, updated)
	if err != nil {
		// Object was fully deleted (expected with fake client when last finalizer is removed)
		if !errors.IsNotFound(err) {
			t.Fatalf("unexpected error getting VPCRouter after delete: %v", err)
		}
	} else {
		// If the object still exists, verify the finalizer was removed
		for _, f := range updated.Finalizers {
			if f == "vpc.roks.ibm.com/router-cleanup" {
				t.Error("expected finalizer to be removed after deletion")
			}
		}
	}
}

// TestReconcile_NotFound tests that when the VPCRouter resource is gone,
// the reconciler returns no error and does not requeue.
func TestReconcile_NotFound(t *testing.T) {
	scheme := newTestScheme()

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	r := &Reconciler{
		Client: fakeClient,
		Scheme: scheme,
	}

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "nonexistent", Namespace: "default"},
	})

	if err != nil {
		t.Fatalf("Reconcile() error = %v for not-found resource", err)
	}
	if result.Requeue {
		t.Error("should not requeue for not-found resource")
	}
}

// TestBuildRouterPod_NoNAT tests that when the gateway has no NAT rules,
// the router pod has no NFTABLES_CONFIG env var (pure routing, no MASQUERADE).
func TestBuildRouterPod_NoNAT(t *testing.T) {
	router := &v1alpha1.VPCRouter{
		ObjectMeta: metav1.ObjectMeta{Name: "rt-nonat", Namespace: "default"},
		Spec: v1alpha1.VPCRouterSpec{
			Gateway: "gw-test",
			Networks: []v1alpha1.RouterNetwork{
				{Name: "l2-app", Address: "10.100.0.1/24"},
			},
		},
	}
	gw := &v1alpha1.VPCGateway{
		ObjectMeta: metav1.ObjectMeta{Name: "gw-test", Namespace: "default"},
		Spec: v1alpha1.VPCGatewaySpec{
			Zone: "eu-de-1",
			Uplink: v1alpha1.GatewayUplink{Network: "uplink-net"},
			// No NAT configured — VPC routes handle return traffic
		},
		Status: v1alpha1.VPCGatewayStatus{
			Phase:      "Ready",
			MACAddress: "fa:16:3e:aa:bb:cc",
			ReservedIP: "10.240.1.5",
		},
	}

	pod := buildRouterPod(router, gw)

	// Should NOT have NFTABLES_CONFIG env var
	for _, env := range pod.Spec.Containers[0].Env {
		if env.Name == "NFTABLES_CONFIG" {
			t.Error("expected no NFTABLES_CONFIG when gateway has no NAT rules")
		}
	}

	// Script should contain ip_forward but not masquerade
	script := pod.Spec.Containers[0].Command[2]
	if !containsString(script, "ip_forward") {
		t.Error("expected init script to enable IP forwarding")
	}
}

// TestBuildRouterPod_WithExplicitNAT tests that when the gateway has explicit
// SNAT rules, the NFTABLES_CONFIG env var is present.
func TestBuildRouterPod_WithExplicitNAT(t *testing.T) {
	router := &v1alpha1.VPCRouter{
		ObjectMeta: metav1.ObjectMeta{Name: "rt-nat", Namespace: "default"},
		Spec: v1alpha1.VPCRouterSpec{
			Gateway: "gw-test",
			Networks: []v1alpha1.RouterNetwork{
				{Name: "l2-app", Address: "10.100.0.1/24"},
			},
		},
	}
	gw := &v1alpha1.VPCGateway{
		ObjectMeta: metav1.ObjectMeta{Name: "gw-test", Namespace: "default"},
		Spec: v1alpha1.VPCGatewaySpec{
			Zone:   "eu-de-1",
			Uplink: v1alpha1.GatewayUplink{Network: "uplink-net"},
			NAT: &v1alpha1.GatewayNAT{
				SNAT: []v1alpha1.SNATRule{
					{Source: "10.100.0.0/24", Priority: 100},
				},
			},
		},
		Status: v1alpha1.VPCGatewayStatus{
			Phase:      "Ready",
			MACAddress: "fa:16:3e:aa:bb:cc",
			ReservedIP: "10.240.1.5",
		},
	}

	pod := buildRouterPod(router, gw)

	// Should have NFTABLES_CONFIG env var with SNAT rule
	found := false
	for _, env := range pod.Spec.Containers[0].Env {
		if env.Name == "NFTABLES_CONFIG" {
			found = true
			if !containsString(env.Value, "snat") {
				t.Errorf("expected NFTABLES_CONFIG to contain snat rule, got %q", env.Value)
			}
			if !containsString(env.Value, "10.240.1.5") {
				t.Errorf("expected SNAT to use VNI IP 10.240.1.5, got %q", env.Value)
			}
		}
	}
	if !found {
		t.Error("expected NFTABLES_CONFIG env var when gateway has explicit SNAT rules")
	}
}

// TestBuildRouterPod_WithDHCP tests that DHCP_ENABLED is set correctly.
func TestBuildRouterPod_WithDHCP(t *testing.T) {
	router := &v1alpha1.VPCRouter{
		ObjectMeta: metav1.ObjectMeta{Name: "rt-dhcp", Namespace: "default"},
		Spec: v1alpha1.VPCRouterSpec{
			Gateway: "gw-test",
			Networks: []v1alpha1.RouterNetwork{
				{Name: "l2-app", Address: "10.100.0.1/24"},
			},
			DHCP: &v1alpha1.RouterDHCP{Enabled: true},
		},
	}
	gw := &v1alpha1.VPCGateway{
		ObjectMeta: metav1.ObjectMeta{Name: "gw-test", Namespace: "default"},
		Spec: v1alpha1.VPCGatewaySpec{
			Zone:   "eu-de-1",
			Uplink: v1alpha1.GatewayUplink{Network: "uplink-net"},
		},
		Status: v1alpha1.VPCGatewayStatus{
			Phase:      "Ready",
			MACAddress: "fa:16:3e:aa:bb:cc",
			ReservedIP: "10.240.1.5",
		},
	}

	pod := buildRouterPod(router, gw)

	for _, env := range pod.Spec.Containers[0].Env {
		if env.Name == "DHCP_ENABLED" {
			if env.Value != "true" {
				t.Errorf("expected DHCP_ENABLED = 'true', got %q", env.Value)
			}
			return
		}
	}
	t.Error("expected DHCP_ENABLED env var to be present")
}

// TestBuildRouterPod_Probes tests that liveness and readiness probes are present.
func TestBuildRouterPod_Probes(t *testing.T) {
	router := &v1alpha1.VPCRouter{
		ObjectMeta: metav1.ObjectMeta{Name: "rt-probes", Namespace: "default"},
		Spec: v1alpha1.VPCRouterSpec{
			Gateway: "gw-test",
			Networks: []v1alpha1.RouterNetwork{
				{Name: "l2-app", Address: "10.100.0.1/24"},
			},
		},
	}
	gw := &v1alpha1.VPCGateway{
		ObjectMeta: metav1.ObjectMeta{Name: "gw-test", Namespace: "default"},
		Spec: v1alpha1.VPCGatewaySpec{
			Zone:   "eu-de-1",
			Uplink: v1alpha1.GatewayUplink{Network: "uplink-net"},
		},
		Status: v1alpha1.VPCGatewayStatus{
			Phase:      "Ready",
			MACAddress: "fa:16:3e:aa:bb:cc",
			ReservedIP: "10.240.1.5",
		},
	}

	pod := buildRouterPod(router, gw)
	container := pod.Spec.Containers[0]

	// Verify liveness probe
	if container.LivenessProbe == nil {
		t.Fatal("expected liveness probe to be present")
	}
	if container.LivenessProbe.Exec == nil {
		t.Fatal("expected liveness probe to use exec handler")
	}
	if len(container.LivenessProbe.Exec.Command) == 0 {
		t.Fatal("expected liveness probe command to be non-empty")
	}
	if container.LivenessProbe.Exec.Command[0] != "sysctl" {
		t.Errorf("expected liveness probe command = 'sysctl', got %q", container.LivenessProbe.Exec.Command[0])
	}
	if container.LivenessProbe.InitialDelaySeconds != 30 {
		t.Errorf("expected liveness InitialDelaySeconds = 30, got %d", container.LivenessProbe.InitialDelaySeconds)
	}

	// Verify readiness probe
	if container.ReadinessProbe == nil {
		t.Fatal("expected readiness probe to be present")
	}
	if container.ReadinessProbe.Exec == nil {
		t.Fatal("expected readiness probe to use exec handler")
	}
	if container.ReadinessProbe.InitialDelaySeconds != 10 {
		t.Errorf("expected readiness InitialDelaySeconds = 10, got %d", container.ReadinessProbe.InitialDelaySeconds)
	}
}

// TestComputeSubnetGateway tests the subnet gateway derivation helper.
func TestComputeSubnetGateway(t *testing.T) {
	tests := []struct {
		ip   string
		want string
	}{
		{"10.240.1.5", "10.240.1.1"},
		{"192.168.0.100", "192.168.0.1"},
		{"invalid", ""},
	}
	for _, tc := range tests {
		got := computeSubnetGateway(tc.ip)
		if got != tc.want {
			t.Errorf("computeSubnetGateway(%q) = %q, want %q", tc.ip, got, tc.want)
		}
	}
}

// TestComputeDHCPRange tests the DHCP range derivation helper.
func TestComputeDHCPRange(t *testing.T) {
	tests := []struct {
		address string
		want    string
	}{
		{"10.100.0.1/24", "10.100.0.10,10.100.0.254,255.255.255.0,12h"},
		{"192.168.1.1/24", "192.168.1.10,192.168.1.254,255.255.255.0,12h"},
		{"10.200.0.1/20", "10.200.0.10,10.200.15.254,255.255.240.0,12h"},
		{"172.16.0.1/28", "172.16.0.10,172.16.0.14,255.255.255.240,12h"},
		{"10.100.0.1", ""}, // no prefix
		{"invalid", ""},    // invalid address
	}
	for _, tc := range tests {
		got := computeDHCPRange(tc.address)
		if got != tc.want {
			t.Errorf("computeDHCPRange(%q) = %q, want %q", tc.address, got, tc.want)
		}
	}
}

// TestBuildRouterPod_WithIDS tests that the Suricata sidecar container
// is added when IDS is enabled.
func TestBuildRouterPod_WithIDS(t *testing.T) {
	router := &v1alpha1.VPCRouter{
		ObjectMeta: metav1.ObjectMeta{Name: "rt-ids", Namespace: "default"},
		Spec: v1alpha1.VPCRouterSpec{
			Gateway: "gw-test",
			Networks: []v1alpha1.RouterNetwork{
				{Name: "l2-app", Address: "10.100.0.1/24"},
			},
			IDS: &v1alpha1.RouterIDS{
				Enabled:    true,
				Mode:       "ids",
				Interfaces: "all",
			},
		},
	}
	gw := &v1alpha1.VPCGateway{
		ObjectMeta: metav1.ObjectMeta{Name: "gw-test", Namespace: "default"},
		Spec: v1alpha1.VPCGatewaySpec{
			Zone:   "eu-de-1",
			Uplink: v1alpha1.GatewayUplink{Network: "uplink-net"},
		},
		Status: v1alpha1.VPCGatewayStatus{
			Phase:      "Ready",
			MACAddress: "fa:16:3e:aa:bb:cc",
			ReservedIP: "10.240.1.5",
		},
	}

	pod := buildRouterPod(router, gw)

	// Should have 2 containers: router + suricata
	if len(pod.Spec.Containers) != 2 {
		t.Fatalf("expected 2 containers, got %d", len(pod.Spec.Containers))
	}

	suricata := pod.Spec.Containers[1]
	if suricata.Name != "suricata" {
		t.Errorf("expected second container name = 'suricata', got %q", suricata.Name)
	}

	// Verify Suricata image
	if suricata.Image != "docker.io/jasonish/suricata:7.0" {
		t.Errorf("expected default suricata image, got %q", suricata.Image)
	}

	// Verify privileged security context
	if suricata.SecurityContext == nil || !*suricata.SecurityContext.Privileged {
		t.Error("expected suricata container to be privileged")
	}

	// Verify volume mounts
	if len(suricata.VolumeMounts) != 3 {
		t.Fatalf("expected 3 volume mounts, got %d", len(suricata.VolumeMounts))
	}

	// Verify volumes were added to pod
	if len(pod.Spec.Volumes) != 3 {
		t.Fatalf("expected 3 volumes, got %d", len(pod.Spec.Volumes))
	}
	volNames := make(map[string]bool)
	for _, v := range pod.Spec.Volumes {
		volNames[v.Name] = true
	}
	for _, expected := range []string{"suricata-config", "suricata-rules", "suricata-log"} {
		if !volNames[expected] {
			t.Errorf("expected volume %q to be present", expected)
		}
	}

	// Verify liveness probe
	if suricata.LivenessProbe == nil {
		t.Fatal("expected suricata liveness probe")
	}
	if suricata.LivenessProbe.InitialDelaySeconds != 60 {
		t.Errorf("expected suricata liveness InitialDelaySeconds = 60, got %d", suricata.LivenessProbe.InitialDelaySeconds)
	}

	// Verify SURICATA_MODE env var
	modeFound := false
	for _, env := range suricata.Env {
		if env.Name == "SURICATA_MODE" && env.Value == "ids" {
			modeFound = true
		}
	}
	if !modeFound {
		t.Error("expected SURICATA_MODE=ids env var on suricata container")
	}

	// Router container should NOT have IPS_NFQUEUE_CONFIG in IDS mode
	for _, env := range pod.Spec.Containers[0].Env {
		if env.Name == "IPS_NFQUEUE_CONFIG" {
			t.Error("expected no IPS_NFQUEUE_CONFIG in IDS mode")
		}
	}
}

// TestBuildRouterPod_WithIPS tests that IPS mode adds NFQUEUE env var and
// configures Suricata for NFQUEUE.
func TestBuildRouterPod_WithIPS(t *testing.T) {
	queueNum := int32(0)
	router := &v1alpha1.VPCRouter{
		ObjectMeta: metav1.ObjectMeta{Name: "rt-ips", Namespace: "default"},
		Spec: v1alpha1.VPCRouterSpec{
			Gateway: "gw-test",
			Networks: []v1alpha1.RouterNetwork{
				{Name: "l2-app", Address: "10.100.0.1/24"},
			},
			IDS: &v1alpha1.RouterIDS{
				Enabled:    true,
				Mode:       "ips",
				NFQueueNum: &queueNum,
			},
		},
	}
	gw := &v1alpha1.VPCGateway{
		ObjectMeta: metav1.ObjectMeta{Name: "gw-test", Namespace: "default"},
		Spec: v1alpha1.VPCGatewaySpec{
			Zone:   "eu-de-1",
			Uplink: v1alpha1.GatewayUplink{Network: "uplink-net"},
		},
		Status: v1alpha1.VPCGatewayStatus{
			Phase:      "Ready",
			MACAddress: "fa:16:3e:aa:bb:cc",
			ReservedIP: "10.240.1.5",
		},
	}

	pod := buildRouterPod(router, gw)

	// Router container should have IPS_NFQUEUE_CONFIG
	nfqFound := false
	for _, env := range pod.Spec.Containers[0].Env {
		if env.Name == "IPS_NFQUEUE_CONFIG" {
			nfqFound = true
			if !containsString(env.Value, "table ip suricata") {
				t.Error("expected IPS_NFQUEUE_CONFIG to contain nftables suricata table")
			}
		}
	}
	if !nfqFound {
		t.Error("expected IPS_NFQUEUE_CONFIG env var in IPS mode")
	}

	// Suricata container should have SURICATA_MODE=ips
	suricata := pod.Spec.Containers[1]
	for _, env := range suricata.Env {
		if env.Name == "SURICATA_MODE" && env.Value != "ips" {
			t.Errorf("expected SURICATA_MODE=ips, got %q", env.Value)
		}
	}
}

// TestBuildRouterPod_WithoutIDS tests that no sidecar is added when IDS is not configured.
func TestBuildRouterPod_WithoutIDS(t *testing.T) {
	router := &v1alpha1.VPCRouter{
		ObjectMeta: metav1.ObjectMeta{Name: "rt-noids", Namespace: "default"},
		Spec: v1alpha1.VPCRouterSpec{
			Gateway: "gw-test",
			Networks: []v1alpha1.RouterNetwork{
				{Name: "l2-app", Address: "10.100.0.1/24"},
			},
		},
	}
	gw := &v1alpha1.VPCGateway{
		ObjectMeta: metav1.ObjectMeta{Name: "gw-test", Namespace: "default"},
		Spec: v1alpha1.VPCGatewaySpec{
			Zone:   "eu-de-1",
			Uplink: v1alpha1.GatewayUplink{Network: "uplink-net"},
		},
		Status: v1alpha1.VPCGatewayStatus{
			Phase:      "Ready",
			MACAddress: "fa:16:3e:aa:bb:cc",
			ReservedIP: "10.240.1.5",
		},
	}

	pod := buildRouterPod(router, gw)

	if len(pod.Spec.Containers) != 1 {
		t.Errorf("expected 1 container without IDS, got %d", len(pod.Spec.Containers))
	}
	if len(pod.Spec.Volumes) != 0 {
		t.Errorf("expected 0 volumes without IDS, got %d", len(pod.Spec.Volumes))
	}
}

// TestPodNeedsRecreation_IDSAdded tests that adding IDS to a router triggers
// pod recreation (container count mismatch).
func TestPodNeedsRecreation_IDSAdded(t *testing.T) {
	gw := &v1alpha1.VPCGateway{
		ObjectMeta: metav1.ObjectMeta{Name: "gw-test", Namespace: "default"},
		Spec: v1alpha1.VPCGatewaySpec{
			Zone:   "eu-de-1",
			Uplink: v1alpha1.GatewayUplink{Network: "uplink-net"},
		},
		Status: v1alpha1.VPCGatewayStatus{
			Phase:      "Ready",
			MACAddress: "fa:16:3e:aa:bb:cc",
			ReservedIP: "10.240.1.5",
		},
	}

	// Old router without IDS
	oldRouter := &v1alpha1.VPCRouter{
		ObjectMeta: metav1.ObjectMeta{Name: "rt-test", Namespace: "default"},
		Spec: v1alpha1.VPCRouterSpec{
			Gateway:  "gw-test",
			Networks: []v1alpha1.RouterNetwork{{Name: "l2-app", Address: "10.100.0.1/24"}},
		},
	}
	existingPod := buildRouterPod(oldRouter, gw)

	// New router with IDS enabled
	newRouter := &v1alpha1.VPCRouter{
		ObjectMeta: metav1.ObjectMeta{Name: "rt-test", Namespace: "default"},
		Spec: v1alpha1.VPCRouterSpec{
			Gateway:  "gw-test",
			Networks: []v1alpha1.RouterNetwork{{Name: "l2-app", Address: "10.100.0.1/24"}},
			IDS:      &v1alpha1.RouterIDS{Enabled: true, Mode: "ids"},
		},
	}

	r := &Reconciler{}
	if !r.podNeedsRecreation(existingPod, newRouter, gw) {
		t.Error("expected pod recreation when IDS is added (container count change)")
	}
}

// TestPodNeedsRecreation_IDSModeChanged tests that changing from IDS to IPS
// triggers pod recreation (env var and suricata config change).
func TestPodNeedsRecreation_IDSModeChanged(t *testing.T) {
	gw := &v1alpha1.VPCGateway{
		ObjectMeta: metav1.ObjectMeta{Name: "gw-test", Namespace: "default"},
		Spec: v1alpha1.VPCGatewaySpec{
			Zone:   "eu-de-1",
			Uplink: v1alpha1.GatewayUplink{Network: "uplink-net"},
		},
		Status: v1alpha1.VPCGatewayStatus{
			Phase:      "Ready",
			MACAddress: "fa:16:3e:aa:bb:cc",
			ReservedIP: "10.240.1.5",
		},
	}

	// Old router with IDS mode
	oldRouter := &v1alpha1.VPCRouter{
		ObjectMeta: metav1.ObjectMeta{Name: "rt-test", Namespace: "default"},
		Spec: v1alpha1.VPCRouterSpec{
			Gateway:  "gw-test",
			Networks: []v1alpha1.RouterNetwork{{Name: "l2-app", Address: "10.100.0.1/24"}},
			IDS:      &v1alpha1.RouterIDS{Enabled: true, Mode: "ids"},
		},
	}
	existingPod := buildRouterPod(oldRouter, gw)

	// New router with IPS mode
	newRouter := &v1alpha1.VPCRouter{
		ObjectMeta: metav1.ObjectMeta{Name: "rt-test", Namespace: "default"},
		Spec: v1alpha1.VPCRouterSpec{
			Gateway:  "gw-test",
			Networks: []v1alpha1.RouterNetwork{{Name: "l2-app", Address: "10.100.0.1/24"}},
			IDS:      &v1alpha1.RouterIDS{Enabled: true, Mode: "ips"},
		},
	}

	r := &Reconciler{}
	if !r.podNeedsRecreation(existingPod, newRouter, gw) {
		t.Error("expected pod recreation when IDS mode changes from ids to ips")
	}
}

// ---------------------------------------------------------------------------
// DHCP Tests
// ---------------------------------------------------------------------------

// TestResolvedDHCPConfig_GlobalOnly tests global enabled with no per-network overrides.
func TestResolvedDHCPConfig_GlobalOnly(t *testing.T) {
	global := &v1alpha1.RouterDHCP{Enabled: true}
	net := v1alpha1.RouterNetwork{Name: "app", Address: "10.100.0.1/24"}

	cfg := resolvedDHCPConfig(global, net)
	if cfg == nil {
		t.Fatal("expected resolved config, got nil")
	}
	if cfg.LeaseTime != "12h" {
		t.Errorf("expected default lease 12h, got %q", cfg.LeaseTime)
	}
}

// TestResolvedDHCPConfig_PerNetworkDisable tests that per-network enabled=false overrides global.
func TestResolvedDHCPConfig_PerNetworkDisable(t *testing.T) {
	global := &v1alpha1.RouterDHCP{Enabled: true}
	boolFalse := false
	net := v1alpha1.RouterNetwork{
		Name:    "isolated",
		Address: "10.200.0.1/24",
		DHCP:    &v1alpha1.NetworkDHCP{Enabled: &boolFalse},
	}

	cfg := resolvedDHCPConfig(global, net)
	if cfg != nil {
		t.Error("expected nil (disabled), got config")
	}
}

// TestResolvedDHCPConfig_PerNetworkEnable tests that per-network enabled=true works even when global is disabled.
func TestResolvedDHCPConfig_PerNetworkEnable(t *testing.T) {
	global := &v1alpha1.RouterDHCP{Enabled: false}
	boolTrue := true
	net := v1alpha1.RouterNetwork{
		Name:    "special",
		Address: "10.200.0.1/24",
		DHCP:    &v1alpha1.NetworkDHCP{Enabled: &boolTrue},
	}

	cfg := resolvedDHCPConfig(global, net)
	if cfg == nil {
		t.Fatal("expected config when per-network enables DHCP")
	}
}

// TestResolvedDHCPConfig_LeaseTimeMerge tests that per-network lease time overrides global.
func TestResolvedDHCPConfig_LeaseTimeMerge(t *testing.T) {
	global := &v1alpha1.RouterDHCP{Enabled: true, LeaseTime: "6h"}
	net := v1alpha1.RouterNetwork{
		Name:    "fast",
		Address: "10.100.0.1/24",
		DHCP:    &v1alpha1.NetworkDHCP{LeaseTime: "1h"},
	}

	cfg := resolvedDHCPConfig(global, net)
	if cfg == nil {
		t.Fatal("expected config, got nil")
	}
	if cfg.LeaseTime != "1h" {
		t.Errorf("expected per-network lease 1h, got %q", cfg.LeaseTime)
	}
}

// TestResolvedDHCPConfig_DNSMerge tests that per-network DNS replaces global DNS wholesale.
func TestResolvedDHCPConfig_DNSMerge(t *testing.T) {
	global := &v1alpha1.RouterDHCP{
		Enabled: true,
		DNS: &v1alpha1.DHCPDNSConfig{
			Nameservers: []string{"8.8.8.8", "1.1.1.1"},
		},
	}
	net := v1alpha1.RouterNetwork{
		Name:    "custom-dns",
		Address: "10.100.0.1/24",
		DHCP: &v1alpha1.NetworkDHCP{
			DNS: &v1alpha1.DHCPDNSConfig{
				Nameservers: []string{"10.100.0.1"},
				LocalDomain: "vm.local",
			},
		},
	}

	cfg := resolvedDHCPConfig(global, net)
	if cfg == nil {
		t.Fatal("expected config, got nil")
	}
	if len(cfg.DNS.Nameservers) != 1 || cfg.DNS.Nameservers[0] != "10.100.0.1" {
		t.Errorf("expected per-network DNS to replace global, got %v", cfg.DNS.Nameservers)
	}
	if cfg.DNS.LocalDomain != "vm.local" {
		t.Errorf("expected localDomain vm.local, got %q", cfg.DNS.LocalDomain)
	}
}

// TestResolvedDHCPConfig_NilGlobal tests that per-network DHCP works when global spec.dhcp is nil.
func TestResolvedDHCPConfig_NilGlobal(t *testing.T) {
	boolTrue := true
	net := v1alpha1.RouterNetwork{
		Name:    "standalone",
		Address: "10.100.0.1/24",
		DHCP:    &v1alpha1.NetworkDHCP{Enabled: &boolTrue, LeaseTime: "30m"},
	}

	cfg := resolvedDHCPConfig(nil, net)
	if cfg == nil {
		t.Fatal("expected config with nil global but per-network enabled")
	}
	if cfg.LeaseTime != "30m" {
		t.Errorf("expected 30m, got %q", cfg.LeaseTime)
	}
}

// TestGenerateDnsmasqCommand_Basic tests auto-computed range with default lease.
func TestGenerateDnsmasqCommand_Basic(t *testing.T) {
	cfg := &resolvedDHCP{LeaseTime: "12h"}
	cmd := generateDnsmasqCommand("net0", "10.100.0.1/24", cfg)

	if !containsString(cmd, "--interface=net0") {
		t.Error("expected --interface=net0")
	}
	if !containsString(cmd, "--dhcp-range=10.100.0.10,10.100.0.254,255.255.255.0,12h") {
		t.Errorf("expected auto-computed range, got %q", cmd)
	}
	if !containsString(cmd, "--bind-interfaces") {
		t.Error("expected --bind-interfaces")
	}
	if !containsString(cmd, "--pid-file=/var/run/dnsmasq-net0.pid") {
		t.Error("expected --pid-file for net0")
	}
}

// TestGenerateDnsmasqCommand_CustomRange tests custom start/end in output.
func TestGenerateDnsmasqCommand_CustomRange(t *testing.T) {
	cfg := &resolvedDHCP{
		LeaseTime: "1h",
		Range:     &v1alpha1.NetworkDHCPRange{Start: "10.100.0.20", End: "10.100.0.200"},
	}
	cmd := generateDnsmasqCommand("net0", "10.100.0.1/24", cfg)

	if !containsString(cmd, "--dhcp-range=10.100.0.20,10.100.0.200,255.255.255.0,1h") {
		t.Errorf("expected custom range, got %q", cmd)
	}
}

// TestGenerateDnsmasqCommand_Reservations tests --dhcp-host flags.
func TestGenerateDnsmasqCommand_Reservations(t *testing.T) {
	cfg := &resolvedDHCP{
		LeaseTime: "12h",
		Reservations: []v1alpha1.DHCPStaticReservation{
			{MAC: "fa:16:3e:aa:bb:cc", IP: "10.100.0.10", Hostname: "db-server"},
			{MAC: "fa:16:3e:dd:ee:ff", IP: "10.100.0.11"},
		},
	}
	cmd := generateDnsmasqCommand("net0", "10.100.0.1/24", cfg)

	if !containsString(cmd, "--dhcp-host=fa:16:3e:aa:bb:cc,10.100.0.10,db-server") {
		t.Errorf("expected reservation with hostname, got %q", cmd)
	}
	if !containsString(cmd, "--dhcp-host=fa:16:3e:dd:ee:ff,10.100.0.11") {
		t.Errorf("expected reservation without hostname, got %q", cmd)
	}
}

// TestGenerateDnsmasqCommand_AllDNS tests nameservers, search domains, and local domain flags.
func TestGenerateDnsmasqCommand_AllDNS(t *testing.T) {
	cfg := &resolvedDHCP{
		LeaseTime: "12h",
		DNS: &v1alpha1.DHCPDNSConfig{
			Nameservers:   []string{"8.8.8.8", "1.1.1.1"},
			SearchDomains: []string{"example.com", "test.local"},
			LocalDomain:   "vm.local",
		},
	}
	cmd := generateDnsmasqCommand("net0", "10.100.0.1/24", cfg)

	if !containsString(cmd, "--dhcp-option=6,8.8.8.8,1.1.1.1") {
		t.Errorf("expected nameservers option, got %q", cmd)
	}
	if !containsString(cmd, "--dhcp-option=119,example.com,test.local") {
		t.Errorf("expected search domains option, got %q", cmd)
	}
	if !containsString(cmd, "--domain=vm.local") {
		t.Errorf("expected local domain, got %q", cmd)
	}
	if !containsString(cmd, "--expand-hosts") {
		t.Errorf("expected --expand-hosts with local domain, got %q", cmd)
	}
}

// TestGenerateDnsmasqCommand_AllOptions tests router, MTU, NTP, and custom option flags.
func TestGenerateDnsmasqCommand_AllOptions(t *testing.T) {
	mtu := int32(1400)
	cfg := &resolvedDHCP{
		LeaseTime: "12h",
		Options: &v1alpha1.DHCPOptions{
			Router:     "10.100.0.1",
			MTU:        &mtu,
			NTPServers: []string{"pool.ntp.org"},
			Custom:     []string{"15,example.com"},
		},
	}
	cmd := generateDnsmasqCommand("net0", "10.100.0.1/24", cfg)

	if !containsString(cmd, "--dhcp-option=3,10.100.0.1") {
		t.Errorf("expected router option, got %q", cmd)
	}
	if !containsString(cmd, "--dhcp-option=26,1400") {
		t.Errorf("expected MTU option, got %q", cmd)
	}
	if !containsString(cmd, "--dhcp-option=42,pool.ntp.org") {
		t.Errorf("expected NTP option, got %q", cmd)
	}
	if !containsString(cmd, "--dhcp-option=15,example.com") {
		t.Errorf("expected custom option, got %q", cmd)
	}
}

// TestComputeDHCPRangeWithLease tests parameterized lease times.
func TestComputeDHCPRangeWithLease(t *testing.T) {
	tests := []struct {
		address   string
		leaseTime string
		want      string
	}{
		{"10.100.0.1/24", "1h", "10.100.0.10,10.100.0.254,255.255.255.0,1h"},
		{"10.100.0.1/24", "30m", "10.100.0.10,10.100.0.254,255.255.255.0,30m"},
		{"10.200.0.1/20", "6h", "10.200.0.10,10.200.15.254,255.255.240.0,6h"},
		{"invalid", "12h", ""},
	}
	for _, tc := range tests {
		got := computeDHCPRangeWithLease(tc.address, tc.leaseTime)
		if got != tc.want {
			t.Errorf("computeDHCPRangeWithLease(%q, %q) = %q, want %q", tc.address, tc.leaseTime, got, tc.want)
		}
	}
}

// TestBuildInitScript_PerNetworkDHCP tests that the init script generates
// dnsmasq commands only for networks with DHCP enabled.
func TestBuildInitScript_PerNetworkDHCP(t *testing.T) {
	boolFalse := false
	router := &v1alpha1.VPCRouter{
		ObjectMeta: metav1.ObjectMeta{Name: "rt-dhcp-mix", Namespace: "default"},
		Spec: v1alpha1.VPCRouterSpec{
			Gateway: "gw-test",
			DHCP:    &v1alpha1.RouterDHCP{Enabled: true},
			Networks: []v1alpha1.RouterNetwork{
				{Name: "net-a", Address: "10.100.0.1/24"}, // inherits global → enabled
				{Name: "net-b", Address: "10.200.0.1/24", DHCP: &v1alpha1.NetworkDHCP{Enabled: &boolFalse}}, // disabled
				{Name: "net-c", Address: "10.300.0.1/24", DHCP: &v1alpha1.NetworkDHCP{
					Range: &v1alpha1.NetworkDHCPRange{Start: "10.300.0.50", End: "10.300.0.100"},
				}}, // enabled with custom range
			},
		},
	}
	gw := &v1alpha1.VPCGateway{
		ObjectMeta: metav1.ObjectMeta{Name: "gw-test", Namespace: "default"},
		Spec: v1alpha1.VPCGatewaySpec{
			Zone:   "eu-de-1",
			Uplink: v1alpha1.GatewayUplink{Network: "uplink-net"},
		},
		Status: v1alpha1.VPCGatewayStatus{
			Phase:      "Ready",
			MACAddress: "fa:16:3e:aa:bb:cc",
			ReservedIP: "10.240.1.5",
		},
	}

	script := buildInitScript(router, gw)

	// net0 (net-a) should have dnsmasq
	if !containsString(script, "--interface=net0") {
		t.Error("expected dnsmasq for net0 (globally enabled)")
	}

	// net1 (net-b) should NOT have dnsmasq
	if containsString(script, "--interface=net1") {
		t.Error("expected NO dnsmasq for net1 (per-network disabled)")
	}

	// net2 (net-c) should have dnsmasq with custom range
	if !containsString(script, "--interface=net2") {
		t.Error("expected dnsmasq for net2 (enabled with custom range)")
	}
}

// TestBuildRouterPod_DHCPConfigHash tests that DHCP_CONFIG_HASH env var is present
// and changes when config changes.
func TestBuildRouterPod_DHCPConfigHash(t *testing.T) {
	gw := &v1alpha1.VPCGateway{
		ObjectMeta: metav1.ObjectMeta{Name: "gw-test", Namespace: "default"},
		Spec: v1alpha1.VPCGatewaySpec{
			Zone:   "eu-de-1",
			Uplink: v1alpha1.GatewayUplink{Network: "uplink-net"},
		},
		Status: v1alpha1.VPCGatewayStatus{
			Phase:      "Ready",
			MACAddress: "fa:16:3e:aa:bb:cc",
			ReservedIP: "10.240.1.5",
		},
	}

	router1 := &v1alpha1.VPCRouter{
		ObjectMeta: metav1.ObjectMeta{Name: "rt-hash1", Namespace: "default"},
		Spec: v1alpha1.VPCRouterSpec{
			Gateway:  "gw-test",
			DHCP:     &v1alpha1.RouterDHCP{Enabled: true, LeaseTime: "12h"},
			Networks: []v1alpha1.RouterNetwork{{Name: "app", Address: "10.100.0.1/24"}},
		},
	}

	router2 := &v1alpha1.VPCRouter{
		ObjectMeta: metav1.ObjectMeta{Name: "rt-hash2", Namespace: "default"},
		Spec: v1alpha1.VPCRouterSpec{
			Gateway:  "gw-test",
			DHCP:     &v1alpha1.RouterDHCP{Enabled: true, LeaseTime: "1h"},
			Networks: []v1alpha1.RouterNetwork{{Name: "app", Address: "10.100.0.1/24"}},
		},
	}

	pod1 := buildRouterPod(router1, gw)
	pod2 := buildRouterPod(router2, gw)

	hash1 := ""
	hash2 := ""
	for _, env := range pod1.Spec.Containers[0].Env {
		if env.Name == "DHCP_CONFIG_HASH" {
			hash1 = env.Value
		}
	}
	for _, env := range pod2.Spec.Containers[0].Env {
		if env.Name == "DHCP_CONFIG_HASH" {
			hash2 = env.Value
		}
	}

	if hash1 == "" {
		t.Fatal("expected DHCP_CONFIG_HASH env var on pod1")
	}
	if hash2 == "" {
		t.Fatal("expected DHCP_CONFIG_HASH env var on pod2")
	}
	if hash1 == hash2 {
		t.Error("expected different hashes for different DHCP configs")
	}
}

// TestPodNeedsRecreation_DHCPConfigChanged tests that changing DHCP lease time
// triggers pod recreation via DHCP_CONFIG_HASH change.
func TestPodNeedsRecreation_DHCPConfigChanged(t *testing.T) {
	gw := &v1alpha1.VPCGateway{
		ObjectMeta: metav1.ObjectMeta{Name: "gw-test", Namespace: "default"},
		Spec: v1alpha1.VPCGatewaySpec{
			Zone:   "eu-de-1",
			Uplink: v1alpha1.GatewayUplink{Network: "uplink-net"},
		},
		Status: v1alpha1.VPCGatewayStatus{
			Phase:      "Ready",
			MACAddress: "fa:16:3e:aa:bb:cc",
			ReservedIP: "10.240.1.5",
		},
	}

	oldRouter := &v1alpha1.VPCRouter{
		ObjectMeta: metav1.ObjectMeta{Name: "rt-test", Namespace: "default"},
		Spec: v1alpha1.VPCRouterSpec{
			Gateway:  "gw-test",
			DHCP:     &v1alpha1.RouterDHCP{Enabled: true, LeaseTime: "12h"},
			Networks: []v1alpha1.RouterNetwork{{Name: "app", Address: "10.100.0.1/24"}},
		},
	}
	existingPod := buildRouterPod(oldRouter, gw)

	newRouter := &v1alpha1.VPCRouter{
		ObjectMeta: metav1.ObjectMeta{Name: "rt-test", Namespace: "default"},
		Spec: v1alpha1.VPCRouterSpec{
			Gateway:  "gw-test",
			DHCP:     &v1alpha1.RouterDHCP{Enabled: true, LeaseTime: "1h"},
			Networks: []v1alpha1.RouterNetwork{{Name: "app", Address: "10.100.0.1/24"}},
		},
	}

	r := &Reconciler{}
	if !r.podNeedsRecreation(existingPod, newRouter, gw) {
		t.Error("expected pod recreation when DHCP lease time changes")
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
