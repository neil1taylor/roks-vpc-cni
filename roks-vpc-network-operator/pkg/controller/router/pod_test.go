package router

import (
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
