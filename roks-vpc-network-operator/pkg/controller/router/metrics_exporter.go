package router

import (
	"fmt"
	"net"
	"os"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	v1alpha1 "github.com/IBM/roks-vpc-network-operator/api/v1alpha1"
)

const (
	defaultMetricsExporterImage = "de.icr.io/roks/vpc-router-metrics-exporter:latest"
	metricsExporterImageEnv     = "METRICS_EXPORTER_IMAGE"
	defaultMetricsPort          = int32(9100)
)

// resolveMetricsExporterImage determines the metrics exporter container image,
// checking in order: spec.metrics.image > METRICS_EXPORTER_IMAGE env > default
func resolveMetricsExporterImage(router *v1alpha1.VPCRouter) string {
	if router.Spec.Metrics != nil && router.Spec.Metrics.Image != "" {
		return router.Spec.Metrics.Image
	}
	if img := os.Getenv(metricsExporterImageEnv); img != "" {
		return img
	}
	return defaultMetricsExporterImage
}

// metricsExporterPort returns the configured port or the default.
func metricsExporterPort(router *v1alpha1.VPCRouter) int32 {
	if router.Spec.Metrics != nil && router.Spec.Metrics.Port != nil {
		return *router.Spec.Metrics.Port
	}
	return defaultMetricsPort
}

// buildMetricsExporterContainer constructs the metrics exporter sidecar container spec.
func buildMetricsExporterContainer(router *v1alpha1.VPCRouter) corev1.Container {
	image := resolveMetricsExporterImage(router)
	port := metricsExporterPort(router)

	envVars := []corev1.EnvVar{
		{Name: "METRICS_PORT", Value: fmt.Sprintf("%d", port)},
		{Name: "DNSMASQ_LEASE_DIR", Value: "/var/lib/misc"},
	}

	// Pass DHCP pool size env vars so the exporter knows pool capacities
	envVars = append(envVars, buildDHCPPoolEnvVars(router)...)

	return corev1.Container{
		Name:  "metrics-exporter",
		Image: image,
		Env:   envVars,
		Ports: []corev1.ContainerPort{
			{
				Name:          "metrics",
				ContainerPort: port,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		SecurityContext: &corev1.SecurityContext{
			Capabilities: &corev1.Capabilities{
				Add: []corev1.Capability{"NET_ADMIN"},
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{Name: "dnsmasq-leases", MountPath: "/var/lib/misc", ReadOnly: true},
		},
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/healthz",
					Port: intstr.FromInt32(port),
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       30,
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/healthz",
					Port: intstr.FromInt32(port),
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       10,
		},
	}
}

// buildDHCPPoolEnvVars creates DHCP_POOL_<IFACE>_SIZE env vars for each network
// that has DHCP enabled, so the metrics exporter knows the pool capacity.
func buildDHCPPoolEnvVars(router *v1alpha1.VPCRouter) []corev1.EnvVar {
	var envVars []corev1.EnvVar
	for i, netSpec := range router.Spec.Networks {
		cfg := resolvedDHCPConfig(router.Spec.DHCP, netSpec)
		if cfg == nil {
			continue
		}

		ifName := fmt.Sprintf("NET%d", i)
		poolSize := computePoolSize(netSpec.Address, cfg)
		if poolSize > 0 {
			envVars = append(envVars, corev1.EnvVar{
				Name:  fmt.Sprintf("DHCP_POOL_%s_SIZE", ifName),
				Value: fmt.Sprintf("%d", poolSize),
			})
		}
	}
	return envVars
}

// computePoolSize calculates the number of IPs in a DHCP pool.
func computePoolSize(address string, cfg *resolvedDHCP) int {
	if cfg.Range != nil {
		start := net.ParseIP(cfg.Range.Start)
		end := net.ParseIP(cfg.Range.End)
		if start == nil || end == nil {
			return 0
		}
		return ipToInt(end) - ipToInt(start) + 1
	}

	// Auto-computed range: .10 to broadcast-1
	_, ipNet, err := net.ParseCIDR(address)
	if err != nil {
		return 0
	}
	start := make(net.IP, len(ipNet.IP))
	copy(start, ipNet.IP)
	start[len(start)-1] += 10
	end := broadcastIP(ipNet)
	end[len(end)-1]--
	return ipToInt(end) - ipToInt(start) + 1
}

// ipToInt converts an IPv4 address to an integer for pool size calculation.
func ipToInt(ip net.IP) int {
	ip = ip.To4()
	if ip == nil {
		return 0
	}
	return int(ip[0])<<24 | int(ip[1])<<16 | int(ip[2])<<8 | int(ip[3])
}
