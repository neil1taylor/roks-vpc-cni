package router

import (
	"testing"

	v1alpha1 "github.com/IBM/roks-vpc-network-operator/api/v1alpha1"
)

func TestBuildAdGuardSidecar(t *testing.T) {
	policy := &v1alpha1.VPCDNSPolicy{
		Spec: v1alpha1.VPCDNSPolicySpec{
			RouterRef: "my-router",
			Image:     "adguard/adguardhome:v0.107",
		},
		Status: v1alpha1.VPCDNSPolicyStatus{
			ConfigMapName: "test-dns-adguard-config",
		},
	}

	container, volumes := buildAdGuardSidecar(policy)

	if container.Name != "adguard-home" {
		t.Errorf("expected container name 'adguard-home', got %q", container.Name)
	}
	if container.Image != "adguard/adguardhome:v0.107" {
		t.Errorf("expected image 'adguard/adguardhome:v0.107', got %q", container.Image)
	}
	if len(container.Ports) != 3 {
		t.Fatalf("expected 3 ports (dns-udp, dns-tcp, web-ui), got %d", len(container.Ports))
	}
	if container.Ports[0].ContainerPort != 5353 {
		t.Errorf("expected DNS port 5353, got %d", container.Ports[0].ContainerPort)
	}
	if container.Ports[2].ContainerPort != 3000 {
		t.Errorf("expected web-ui port 3000, got %d", container.Ports[2].ContainerPort)
	}
	if len(container.VolumeMounts) != 2 {
		t.Fatalf("expected 2 volume mounts (config + work), got %d", len(container.VolumeMounts))
	}
	if container.LivenessProbe == nil || container.LivenessProbe.HTTPGet == nil {
		t.Error("expected HTTP liveness probe")
	}
	if container.ReadinessProbe == nil || container.ReadinessProbe.HTTPGet == nil {
		t.Error("expected HTTP readiness probe")
	}
	if len(volumes) != 2 {
		t.Fatalf("expected 2 volumes (config + work), got %d", len(volumes))
	}
	if volumes[0].ConfigMap == nil || volumes[0].ConfigMap.Name != "test-dns-adguard-config" {
		t.Error("expected ConfigMap volume source with name 'test-dns-adguard-config'")
	}
	if volumes[1].EmptyDir == nil {
		t.Error("expected emptyDir volume for adguard-work")
	}
}

func TestBuildAdGuardSidecar_DefaultImage(t *testing.T) {
	policy := &v1alpha1.VPCDNSPolicy{
		Spec:   v1alpha1.VPCDNSPolicySpec{RouterRef: "my-router"},
		Status: v1alpha1.VPCDNSPolicyStatus{ConfigMapName: "test-cm"},
	}

	container, _ := buildAdGuardSidecar(policy)

	if container.Image != defaultAdGuardImage {
		t.Errorf("expected default image %q, got %q", defaultAdGuardImage, container.Image)
	}
}

func TestBuildAdGuardSidecar_Resources(t *testing.T) {
	policy := &v1alpha1.VPCDNSPolicy{
		Spec:   v1alpha1.VPCDNSPolicySpec{RouterRef: "my-router"},
		Status: v1alpha1.VPCDNSPolicyStatus{ConfigMapName: "test-cm"},
	}

	container, _ := buildAdGuardSidecar(policy)

	if container.Resources.Requests.Cpu().String() != "50m" {
		t.Errorf("expected CPU request 50m, got %s", container.Resources.Requests.Cpu())
	}
	if container.Resources.Requests.Memory().String() != "128Mi" {
		t.Errorf("expected memory request 128Mi, got %s", container.Resources.Requests.Memory())
	}
	if container.Resources.Limits.Cpu().String() != "200m" {
		t.Errorf("expected CPU limit 200m, got %s", container.Resources.Limits.Cpu())
	}
	if container.Resources.Limits.Memory().String() != "256Mi" {
		t.Errorf("expected memory limit 256Mi, got %s", container.Resources.Limits.Memory())
	}
}

func TestBuildAdGuardSidecar_Args(t *testing.T) {
	policy := &v1alpha1.VPCDNSPolicy{
		Spec:   v1alpha1.VPCDNSPolicySpec{RouterRef: "my-router"},
		Status: v1alpha1.VPCDNSPolicyStatus{ConfigMapName: "test-cm"},
	}

	container, _ := buildAdGuardSidecar(policy)

	expectedArgs := []string{"--config", "/opt/adguardhome/conf/AdGuardHome.yaml", "--work-dir", "/opt/adguardhome/work", "--no-check-update"}
	if len(container.Args) != len(expectedArgs) {
		t.Fatalf("expected %d args, got %d", len(expectedArgs), len(container.Args))
	}
	for i, arg := range expectedArgs {
		if container.Args[i] != arg {
			t.Errorf("arg[%d] = %q, want %q", i, container.Args[i], arg)
		}
	}
}
