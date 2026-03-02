package router

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1alpha1 "github.com/IBM/roks-vpc-network-operator/api/v1alpha1"
)

func TestBuildMetricsExporterContainer_DefaultPort(t *testing.T) {
	router := &v1alpha1.VPCRouter{
		ObjectMeta: metav1.ObjectMeta{Name: "test-router", Namespace: "default"},
		Spec: v1alpha1.VPCRouterSpec{
			Gateway:  "gw-test",
			Networks: []v1alpha1.RouterNetwork{{Name: "net-a", Address: "10.100.0.1/24"}},
			Metrics:  &v1alpha1.RouterMetrics{Enabled: true},
		},
	}

	c := buildMetricsExporterContainer(router)

	if c.Name != "metrics-exporter" {
		t.Errorf("container name = %q, want 'metrics-exporter'", c.Name)
	}
	if len(c.Ports) != 1 || c.Ports[0].ContainerPort != 9100 {
		t.Errorf("container port = %v, want 9100", c.Ports)
	}
	if c.Ports[0].Name != "metrics" {
		t.Errorf("port name = %q, want 'metrics'", c.Ports[0].Name)
	}
}

func TestBuildMetricsExporterContainer_CustomPort(t *testing.T) {
	port := int32(9200)
	router := &v1alpha1.VPCRouter{
		ObjectMeta: metav1.ObjectMeta{Name: "test-router", Namespace: "default"},
		Spec: v1alpha1.VPCRouterSpec{
			Gateway:  "gw-test",
			Networks: []v1alpha1.RouterNetwork{{Name: "net-a", Address: "10.100.0.1/24"}},
			Metrics:  &v1alpha1.RouterMetrics{Enabled: true, Port: &port},
		},
	}

	c := buildMetricsExporterContainer(router)

	if c.Ports[0].ContainerPort != 9200 {
		t.Errorf("container port = %d, want 9200", c.Ports[0].ContainerPort)
	}
}

func TestBuildMetricsExporterContainer_CustomImage(t *testing.T) {
	router := &v1alpha1.VPCRouter{
		ObjectMeta: metav1.ObjectMeta{Name: "test-router", Namespace: "default"},
		Spec: v1alpha1.VPCRouterSpec{
			Gateway:  "gw-test",
			Networks: []v1alpha1.RouterNetwork{{Name: "net-a", Address: "10.100.0.1/24"}},
			Metrics:  &v1alpha1.RouterMetrics{Enabled: true, Image: "custom/image:v1"},
		},
	}

	c := buildMetricsExporterContainer(router)

	if c.Image != "custom/image:v1" {
		t.Errorf("image = %q, want 'custom/image:v1'", c.Image)
	}
}

func TestBuildMetricsExporterContainer_ImageFromEnv(t *testing.T) {
	t.Setenv("METRICS_EXPORTER_IMAGE", "env-override:latest")

	router := &v1alpha1.VPCRouter{
		ObjectMeta: metav1.ObjectMeta{Name: "test-router", Namespace: "default"},
		Spec: v1alpha1.VPCRouterSpec{
			Gateway:  "gw-test",
			Networks: []v1alpha1.RouterNetwork{{Name: "net-a", Address: "10.100.0.1/24"}},
			Metrics:  &v1alpha1.RouterMetrics{Enabled: true},
		},
	}

	img := resolveMetricsExporterImage(router)
	if img != "env-override:latest" {
		t.Errorf("image = %q, want 'env-override:latest'", img)
	}
}

func TestBuildMetricsExporterContainer_Capabilities(t *testing.T) {
	router := &v1alpha1.VPCRouter{
		ObjectMeta: metav1.ObjectMeta{Name: "test-router", Namespace: "default"},
		Spec: v1alpha1.VPCRouterSpec{
			Gateway:  "gw-test",
			Networks: []v1alpha1.RouterNetwork{{Name: "net-a", Address: "10.100.0.1/24"}},
			Metrics:  &v1alpha1.RouterMetrics{Enabled: true},
		},
	}

	c := buildMetricsExporterContainer(router)

	if c.SecurityContext == nil || c.SecurityContext.Capabilities == nil {
		t.Fatal("expected security context with capabilities")
	}
	found := false
	for _, cap := range c.SecurityContext.Capabilities.Add {
		if cap == "NET_ADMIN" {
			found = true
		}
	}
	if !found {
		t.Error("expected NET_ADMIN capability")
	}
}

func TestBuildMetricsExporterContainer_Probes(t *testing.T) {
	router := &v1alpha1.VPCRouter{
		ObjectMeta: metav1.ObjectMeta{Name: "test-router", Namespace: "default"},
		Spec: v1alpha1.VPCRouterSpec{
			Gateway:  "gw-test",
			Networks: []v1alpha1.RouterNetwork{{Name: "net-a", Address: "10.100.0.1/24"}},
			Metrics:  &v1alpha1.RouterMetrics{Enabled: true},
		},
	}

	c := buildMetricsExporterContainer(router)

	if c.LivenessProbe == nil {
		t.Fatal("expected liveness probe")
	}
	if c.LivenessProbe.HTTPGet == nil || c.LivenessProbe.HTTPGet.Path != "/healthz" {
		t.Error("expected HTTP GET liveness probe on /healthz")
	}
	if c.ReadinessProbe == nil {
		t.Fatal("expected readiness probe")
	}
	if c.ReadinessProbe.HTTPGet == nil || c.ReadinessProbe.HTTPGet.Path != "/healthz" {
		t.Error("expected HTTP GET readiness probe on /healthz")
	}
}

func TestBuildMetricsExporterContainer_VolumeMount(t *testing.T) {
	router := &v1alpha1.VPCRouter{
		ObjectMeta: metav1.ObjectMeta{Name: "test-router", Namespace: "default"},
		Spec: v1alpha1.VPCRouterSpec{
			Gateway:  "gw-test",
			Networks: []v1alpha1.RouterNetwork{{Name: "net-a", Address: "10.100.0.1/24"}},
			Metrics:  &v1alpha1.RouterMetrics{Enabled: true},
		},
	}

	c := buildMetricsExporterContainer(router)

	if len(c.VolumeMounts) != 1 {
		t.Fatalf("expected 1 volume mount, got %d", len(c.VolumeMounts))
	}
	if c.VolumeMounts[0].Name != "dnsmasq-leases" {
		t.Errorf("volume mount name = %q, want 'dnsmasq-leases'", c.VolumeMounts[0].Name)
	}
	if !c.VolumeMounts[0].ReadOnly {
		t.Error("expected read-only volume mount")
	}
}

func TestBuildDHCPPoolEnvVars(t *testing.T) {
	dhcpEnabled := true
	router := &v1alpha1.VPCRouter{
		ObjectMeta: metav1.ObjectMeta{Name: "test-router", Namespace: "default"},
		Spec: v1alpha1.VPCRouterSpec{
			Gateway: "gw-test",
			DHCP:    &v1alpha1.RouterDHCP{Enabled: true},
			Networks: []v1alpha1.RouterNetwork{
				{Name: "net-a", Address: "10.100.0.1/24", DHCP: &v1alpha1.NetworkDHCP{Enabled: &dhcpEnabled}},
				{Name: "net-b", Address: "10.200.0.1/24"},
			},
		},
	}

	envVars := buildDHCPPoolEnvVars(router)

	// Should have env vars for both networks (both have DHCP: net-a explicit, net-b via global)
	if len(envVars) != 2 {
		t.Fatalf("expected 2 env vars, got %d", len(envVars))
	}
	if envVars[0].Name != "DHCP_POOL_NET0_SIZE" {
		t.Errorf("env var name = %q, want 'DHCP_POOL_NET0_SIZE'", envVars[0].Name)
	}
}

func TestComputePoolSize_AutoRange(t *testing.T) {
	cfg := &resolvedDHCP{LeaseTime: "12h"}
	size := computePoolSize("10.100.0.1/24", cfg)

	// Range: .10 to .254 = 245 addresses
	if size != 245 {
		t.Errorf("pool size = %d, want 245", size)
	}
}

func TestComputePoolSize_CustomRange(t *testing.T) {
	cfg := &resolvedDHCP{
		LeaseTime: "12h",
		Range:     &v1alpha1.NetworkDHCPRange{Start: "10.100.0.100", End: "10.100.0.200"},
	}
	size := computePoolSize("10.100.0.1/24", cfg)

	if size != 101 {
		t.Errorf("pool size = %d, want 101", size)
	}
}

func TestMetricsExporterPort_Default(t *testing.T) {
	router := &v1alpha1.VPCRouter{
		Spec: v1alpha1.VPCRouterSpec{
			Gateway:  "gw-test",
			Networks: []v1alpha1.RouterNetwork{{Name: "net-a", Address: "10.100.0.1/24"}},
			Metrics:  &v1alpha1.RouterMetrics{Enabled: true},
		},
	}

	port := metricsExporterPort(router)
	if port != 9100 {
		t.Errorf("port = %d, want 9100", port)
	}
}
