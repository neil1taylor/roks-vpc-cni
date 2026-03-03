package router

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1alpha1 "github.com/IBM/roks-vpc-network-operator/api/v1alpha1"
)

func newTestRouter() *v1alpha1.VPCRouter {
	return &v1alpha1.VPCRouter{
		ObjectMeta: metav1.ObjectMeta{Name: "test-router", Namespace: "default"},
		Spec: v1alpha1.VPCRouterSpec{
			Gateway: "gw-test",
			Networks: []v1alpha1.RouterNetwork{
				{Name: "net-a", Address: "10.100.0.1/24"},
			},
		},
	}
}

func newTestGateway() *v1alpha1.VPCGateway {
	return &v1alpha1.VPCGateway{
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
}

func TestBuildRouterPod_WithResources(t *testing.T) {
	router := newTestRouter()
	router.Spec.Pod = &v1alpha1.RouterPodSpec{
		Resources: &corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("2"),
				corev1.ResourceMemory: resource.MustParse("1Gi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("4"),
				corev1.ResourceMemory: resource.MustParse("2Gi"),
			},
		},
	}
	gw := newTestGateway()

	pod := buildRouterPod(router, gw)

	res := pod.Spec.Containers[0].Resources
	if res.Requests.Cpu().Cmp(resource.MustParse("2")) != 0 {
		t.Errorf("expected CPU request = 2, got %s", res.Requests.Cpu())
	}
	if res.Requests.Memory().Cmp(resource.MustParse("1Gi")) != 0 {
		t.Errorf("expected memory request = 1Gi, got %s", res.Requests.Memory())
	}
	if res.Limits.Cpu().Cmp(resource.MustParse("4")) != 0 {
		t.Errorf("expected CPU limit = 4, got %s", res.Limits.Cpu())
	}
	if res.Limits.Memory().Cmp(resource.MustParse("2Gi")) != 0 {
		t.Errorf("expected memory limit = 2Gi, got %s", res.Limits.Memory())
	}
}

func TestBuildRouterPod_WithNodeSelector(t *testing.T) {
	router := newTestRouter()
	router.Spec.Pod = &v1alpha1.RouterPodSpec{
		NodeSelector: map[string]string{
			"node-role.kubernetes.io/worker":                    "",
			"feature.node.kubernetes.io/network-sriov.capable": "true",
		},
	}
	gw := newTestGateway()

	pod := buildRouterPod(router, gw)

	if len(pod.Spec.NodeSelector) != 2 {
		t.Fatalf("expected 2 nodeSelector entries, got %d", len(pod.Spec.NodeSelector))
	}
	if v, ok := pod.Spec.NodeSelector["node-role.kubernetes.io/worker"]; !ok || v != "" {
		t.Errorf("expected nodeSelector worker key, got %v", pod.Spec.NodeSelector)
	}
	if v, ok := pod.Spec.NodeSelector["feature.node.kubernetes.io/network-sriov.capable"]; !ok || v != "true" {
		t.Errorf("expected nodeSelector sriov key = true, got %v", pod.Spec.NodeSelector)
	}
}

func TestBuildRouterPod_WithTolerations(t *testing.T) {
	router := newTestRouter()
	router.Spec.Pod = &v1alpha1.RouterPodSpec{
		Tolerations: []corev1.Toleration{
			{
				Key:      "dedicated",
				Operator: corev1.TolerationOpEqual,
				Value:    "network",
				Effect:   corev1.TaintEffectNoSchedule,
			},
		},
	}
	gw := newTestGateway()

	pod := buildRouterPod(router, gw)

	if len(pod.Spec.Tolerations) != 1 {
		t.Fatalf("expected 1 toleration, got %d", len(pod.Spec.Tolerations))
	}
	if pod.Spec.Tolerations[0].Key != "dedicated" || pod.Spec.Tolerations[0].Value != "network" {
		t.Errorf("unexpected toleration: %+v", pod.Spec.Tolerations[0])
	}
}

func TestBuildRouterPod_WithRuntimeClassName(t *testing.T) {
	router := newTestRouter()
	rc := "performance"
	router.Spec.Pod = &v1alpha1.RouterPodSpec{
		RuntimeClassName: &rc,
	}
	gw := newTestGateway()

	pod := buildRouterPod(router, gw)

	if pod.Spec.RuntimeClassName == nil || *pod.Spec.RuntimeClassName != "performance" {
		t.Errorf("expected runtimeClassName = performance, got %v", pod.Spec.RuntimeClassName)
	}
}

func TestBuildRouterPod_WithPriorityClassName(t *testing.T) {
	router := newTestRouter()
	router.Spec.Pod = &v1alpha1.RouterPodSpec{
		PriorityClassName: "system-node-critical",
	}
	gw := newTestGateway()

	pod := buildRouterPod(router, gw)

	if pod.Spec.PriorityClassName != "system-node-critical" {
		t.Errorf("expected priorityClassName = system-node-critical, got %q", pod.Spec.PriorityClassName)
	}
}

func TestBuildRouterPod_NilPodSpec(t *testing.T) {
	router := newTestRouter()
	router.Spec.Pod = nil
	gw := newTestGateway()

	pod := buildRouterPod(router, gw)

	if len(pod.Spec.NodeSelector) != 0 {
		t.Errorf("expected no nodeSelector, got %v", pod.Spec.NodeSelector)
	}
	if len(pod.Spec.Tolerations) != 0 {
		t.Errorf("expected no tolerations, got %v", pod.Spec.Tolerations)
	}
	if pod.Spec.RuntimeClassName != nil {
		t.Errorf("expected nil runtimeClassName, got %v", pod.Spec.RuntimeClassName)
	}
	if pod.Spec.PriorityClassName != "" {
		t.Errorf("expected empty priorityClassName, got %q", pod.Spec.PriorityClassName)
	}
	// Resources should be empty (zero value)
	res := pod.Spec.Containers[0].Resources
	if len(res.Requests) != 0 || len(res.Limits) != 0 {
		t.Errorf("expected empty resources, got requests=%v limits=%v", res.Requests, res.Limits)
	}
}

func TestBuildRouterPod_WithMetricsSidecar(t *testing.T) {
	router := newTestRouter()
	router.Spec.Metrics = &v1alpha1.RouterMetrics{Enabled: true}
	gw := newTestGateway()

	pod := buildRouterPod(router, gw)

	// Should have router + metrics-exporter = 2 containers
	if len(pod.Spec.Containers) != 2 {
		t.Fatalf("expected 2 containers, got %d", len(pod.Spec.Containers))
	}
	if pod.Spec.Containers[1].Name != "metrics-exporter" {
		t.Errorf("second container name = %q, want 'metrics-exporter'", pod.Spec.Containers[1].Name)
	}

	// Should have dnsmasq-leases volume
	foundVolume := false
	for _, v := range pod.Spec.Volumes {
		if v.Name == "dnsmasq-leases" {
			foundVolume = true
		}
	}
	if !foundVolume {
		t.Error("expected dnsmasq-leases volume")
	}

	// Router container should have lease volume mount
	foundMount := false
	for _, vm := range pod.Spec.Containers[0].VolumeMounts {
		if vm.Name == "dnsmasq-leases" {
			foundMount = true
		}
	}
	if !foundMount {
		t.Error("expected dnsmasq-leases volume mount on router container")
	}
}

func TestBuildRouterPod_MetricsAndSuricata(t *testing.T) {
	router := newTestRouter()
	router.Spec.IDS = &v1alpha1.RouterIDS{Enabled: true, Mode: "ids"}
	router.Spec.Metrics = &v1alpha1.RouterMetrics{Enabled: true}
	gw := newTestGateway()

	pod := buildRouterPod(router, gw)

	// Should have router + suricata + metrics-exporter = 3 containers
	if len(pod.Spec.Containers) != 3 {
		t.Fatalf("expected 3 containers, got %d", len(pod.Spec.Containers))
	}
	if pod.Spec.Containers[0].Name != "router" {
		t.Errorf("container[0] = %q, want 'router'", pod.Spec.Containers[0].Name)
	}
	if pod.Spec.Containers[1].Name != "suricata" {
		t.Errorf("container[1] = %q, want 'suricata'", pod.Spec.Containers[1].Name)
	}
	if pod.Spec.Containers[2].Name != "metrics-exporter" {
		t.Errorf("container[2] = %q, want 'metrics-exporter'", pod.Spec.Containers[2].Name)
	}
}

func TestBuildRouterPod_MetricsDisabled(t *testing.T) {
	router := newTestRouter()
	router.Spec.Metrics = &v1alpha1.RouterMetrics{Enabled: false}
	gw := newTestGateway()

	pod := buildRouterPod(router, gw)

	// Should have only router container
	if len(pod.Spec.Containers) != 1 {
		t.Fatalf("expected 1 container when metrics disabled, got %d", len(pod.Spec.Containers))
	}
}

func TestBuildRouterPod_AllPodSpecFields(t *testing.T) {
	router := newTestRouter()
	rc := "performance"
	router.Spec.Pod = &v1alpha1.RouterPodSpec{
		Resources: &corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("2"),
			},
		},
		NodeSelector: map[string]string{"zone": "eu-de-1"},
		Tolerations: []corev1.Toleration{
			{Key: "dedicated", Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule},
		},
		RuntimeClassName:  &rc,
		PriorityClassName: "high-priority",
	}
	gw := newTestGateway()

	pod := buildRouterPod(router, gw)

	// All fields should be set
	if pod.Spec.Containers[0].Resources.Requests.Cpu().Cmp(resource.MustParse("2")) != 0 {
		t.Error("resources not applied")
	}
	if pod.Spec.NodeSelector["zone"] != "eu-de-1" {
		t.Error("nodeSelector not applied")
	}
	if len(pod.Spec.Tolerations) != 1 {
		t.Error("tolerations not applied")
	}
	if pod.Spec.RuntimeClassName == nil || *pod.Spec.RuntimeClassName != "performance" {
		t.Error("runtimeClassName not applied")
	}
	if pod.Spec.PriorityClassName != "high-priority" {
		t.Error("priorityClassName not applied")
	}
}

// ---------------------------------------------------------------------------
// Fast-Path Mode Tests
// ---------------------------------------------------------------------------

func TestBuildRouterPod_StandardMode(t *testing.T) {
	router := newTestRouter()
	router.Spec.Mode = "standard"
	gw := newTestGateway()

	pod := buildRouterPod(router, gw)

	if pod.Spec.Containers[0].Command[0] != "/bin/bash" {
		t.Errorf("standard mode should use /bin/bash command, got %v", pod.Spec.Containers[0].Command)
	}
	if pod.Spec.Containers[0].Image != "quay.io/fedora/fedora:40" {
		t.Errorf("standard mode should use fedora image, got %s", pod.Spec.Containers[0].Image)
	}
}

func TestBuildRouterPod_EmptyModeIsStandard(t *testing.T) {
	router := newTestRouter()
	router.Spec.Mode = "" // empty = standard
	gw := newTestGateway()

	pod := buildRouterPod(router, gw)

	if pod.Spec.Containers[0].Command[0] != "/bin/bash" {
		t.Errorf("empty mode should use /bin/bash command, got %v", pod.Spec.Containers[0].Command)
	}
}

func TestBuildRouterPod_FastpathMode(t *testing.T) {
	router := newTestRouter()
	router.Spec.Mode = "fast-path"
	gw := newTestGateway()

	pod := buildRouterPod(router, gw)

	// Should use /vpc-router command
	if len(pod.Spec.Containers[0].Command) != 1 || pod.Spec.Containers[0].Command[0] != "/vpc-router" {
		t.Errorf("fast-path mode should use /vpc-router command, got %v", pod.Spec.Containers[0].Command)
	}

	// Should use fast-path image (default)
	if pod.Spec.Containers[0].Image != "de.icr.io/roks/vpc-router-fastpath:latest" {
		t.Errorf("fast-path mode should use fastpath image, got %s", pod.Spec.Containers[0].Image)
	}

	// Should have HTTP probes
	if pod.Spec.Containers[0].LivenessProbe.HTTPGet == nil {
		t.Error("fast-path mode should have HTTP liveness probe")
	}
	if pod.Spec.Containers[0].LivenessProbe.HTTPGet.Path != "/healthz" {
		t.Errorf("liveness probe path = %q, want /healthz", pod.Spec.Containers[0].LivenessProbe.HTTPGet.Path)
	}
	if pod.Spec.Containers[0].ReadinessProbe.HTTPGet == nil {
		t.Error("fast-path mode should have HTTP readiness probe")
	}
	if pod.Spec.Containers[0].ReadinessProbe.HTTPGet.Path != "/readyz" {
		t.Errorf("readiness probe path = %q, want /readyz", pod.Spec.Containers[0].ReadinessProbe.HTTPGet.Path)
	}

	// Should have fast-path env vars
	envMap := map[string]string{}
	for _, e := range pod.Spec.Containers[0].Env {
		envMap[e.Name] = e.Value
	}
	if envMap["ROUTER_MODE"] != "fast-path" {
		t.Errorf("ROUTER_MODE = %q, want fast-path", envMap["ROUTER_MODE"])
	}
	if envMap["XDP_ENABLED"] != "true" {
		t.Errorf("XDP_ENABLED = %q, want true", envMap["XDP_ENABLED"])
	}
	if envMap["HEALTH_PORT"] != "8080" {
		t.Errorf("HEALTH_PORT = %q, want 8080", envMap["HEALTH_PORT"])
	}
	if envMap["NETWORK_CONFIG"] == "" {
		t.Error("NETWORK_CONFIG should be non-empty")
	}
}

func TestBuildRouterPod_FastpathModeWithSidecars(t *testing.T) {
	router := newTestRouter()
	router.Spec.Mode = "fast-path"
	router.Spec.IDS = &v1alpha1.RouterIDS{Enabled: true, Mode: "ids"}
	router.Spec.Metrics = &v1alpha1.RouterMetrics{Enabled: true}
	gw := newTestGateway()

	pod := buildRouterPod(router, gw)

	// Should have 3 containers: router + suricata + metrics-exporter
	if len(pod.Spec.Containers) != 3 {
		t.Fatalf("expected 3 containers, got %d", len(pod.Spec.Containers))
	}
	if pod.Spec.Containers[0].Name != "router" {
		t.Errorf("container[0] = %q, want 'router'", pod.Spec.Containers[0].Name)
	}
	if pod.Spec.Containers[1].Name != "suricata" {
		t.Errorf("container[1] = %q, want 'suricata'", pod.Spec.Containers[1].Name)
	}
	if pod.Spec.Containers[2].Name != "metrics-exporter" {
		t.Errorf("container[2] = %q, want 'metrics-exporter'", pod.Spec.Containers[2].Name)
	}

	// Router container should still use /vpc-router command
	if pod.Spec.Containers[0].Command[0] != "/vpc-router" {
		t.Errorf("fast-path mode should use /vpc-router, got %v", pod.Spec.Containers[0].Command)
	}
}

func TestBuildRouterPod_FastpathDefaultImage(t *testing.T) {
	router := newTestRouter()
	router.Spec.Mode = "fast-path"
	gw := newTestGateway()

	pod := buildRouterPod(router, gw)

	expected := "de.icr.io/roks/vpc-router-fastpath:latest"
	if pod.Spec.Containers[0].Image != expected {
		t.Errorf("default fast-path image = %q, want %q", pod.Spec.Containers[0].Image, expected)
	}
}

func TestBuildRouterPod_FastpathWithPodSpec(t *testing.T) {
	router := newTestRouter()
	router.Spec.Mode = "fast-path"
	rc := "performance"
	router.Spec.Pod = &v1alpha1.RouterPodSpec{
		NodeSelector:      map[string]string{"zone": "eu-de-1"},
		RuntimeClassName:  &rc,
		PriorityClassName: "high-priority",
	}
	gw := newTestGateway()

	pod := buildRouterPod(router, gw)

	// Pod spec overrides should apply to fast-path mode too
	if pod.Spec.NodeSelector["zone"] != "eu-de-1" {
		t.Error("nodeSelector not applied in fast-path mode")
	}
	if pod.Spec.RuntimeClassName == nil || *pod.Spec.RuntimeClassName != "performance" {
		t.Error("runtimeClassName not applied in fast-path mode")
	}
	if pod.Spec.PriorityClassName != "high-priority" {
		t.Error("priorityClassName not applied in fast-path mode")
	}
}

func TestBuildRouterPod_ModeChangeTriggersDrift(t *testing.T) {
	router := newTestRouter()
	gw := newTestGateway()

	// Build standard pod
	router.Spec.Mode = "standard"
	standardPod := buildRouterPod(router, gw)

	// Build fast-path pod
	router.Spec.Mode = "fast-path"
	fastpathPod := buildRouterPod(router, gw)

	// Image should be different
	if standardPod.Spec.Containers[0].Image == fastpathPod.Spec.Containers[0].Image {
		t.Error("standard and fast-path should use different images")
	}

	// Command should be different
	if standardPod.Spec.Containers[0].Command[0] == fastpathPod.Spec.Containers[0].Command[0] {
		t.Error("standard and fast-path should use different commands")
	}
}

func TestBuildNetworkConfigJSON(t *testing.T) {
	router := newTestRouter()
	router.Spec.Networks = []v1alpha1.RouterNetwork{
		{Name: "net-a", Address: "10.100.0.1/24"},
		{Name: "net-b", Address: "10.200.0.1/24"},
	}

	json := buildNetworkConfigJSON(router)

	// Should contain interface entries
	if !strings.Contains(json, `"net0"`) || !strings.Contains(json, `"net1"`) {
		t.Errorf("NETWORK_CONFIG should contain net0 and net1, got %s", json)
	}
	if !strings.Contains(json, `"10.100.0.1/24"`) || !strings.Contains(json, `"10.200.0.1/24"`) {
		t.Errorf("NETWORK_CONFIG should contain addresses, got %s", json)
	}
}
