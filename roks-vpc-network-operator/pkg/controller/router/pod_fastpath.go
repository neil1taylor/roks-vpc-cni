package router

import (
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	v1alpha1 "github.com/IBM/roks-vpc-network-operator/api/v1alpha1"
)

// networkInterfaceConfig is one entry in the NETWORK_CONFIG JSON consumed by the fast-path router binary.
type networkInterfaceConfig struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

// networkConfig is the top-level JSON structure for NETWORK_CONFIG.
type networkConfig struct {
	Interfaces []networkInterfaceConfig `json:"interfaces"`
}

// buildFastpathRouterPod constructs the Pod spec for a VPCRouter in fast-path mode.
// It uses the purpose-built Go binary (/vpc-router) instead of the bash init script,
// with HTTP health probes and a NETWORK_CONFIG JSON env var.
// Sidecars (Suricata, metrics-exporter) are appended identically to standard mode.
func buildFastpathRouterPod(router *v1alpha1.VPCRouter, gw *v1alpha1.VPCGateway) *corev1.Pod {
	podName := routerPodName(router)

	// Build Multus network attachments (same as standard mode)
	attachments := buildMultusAttachments(router, gw)
	multusJSON, _ := json.Marshal(attachments)

	// Determine fast-path image
	image := resolveFastpathImage()

	// Build environment variables (same base set as standard mode)
	envVars := buildEnvVars(router, gw)

	// Add fast-path specific env vars
	envVars = append(envVars,
		corev1.EnvVar{Name: "ROUTER_MODE", Value: "fast-path"},
		corev1.EnvVar{Name: "XDP_ENABLED", Value: "true"},
		corev1.EnvVar{Name: "HEALTH_PORT", Value: "8080"},
		corev1.EnvVar{Name: "NETWORK_CONFIG", Value: buildNetworkConfigJSON(router)},
	)

	isTrue := true
	healthPort := intstr.FromInt32(8080)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: router.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":      "vpc-router",
				"app.kubernetes.io/component": "router-pod",
				"vpc.roks.ibm.com/router":     router.Name,
			},
			Annotations: map[string]string{
				"k8s.v1.cni.cncf.io/networks": string(multusJSON),
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         "vpc.roks.ibm.com/v1alpha1",
					Kind:               "VPCRouter",
					Name:               router.Name,
					UID:                router.UID,
					Controller:         &isTrue,
					BlockOwnerDeletion: &isTrue,
				},
			},
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyAlways,
			Containers: []corev1.Container{
				{
					Name:    "router",
					Image:   image,
					Command: []string{"/vpc-router"},
					Env:     envVars,
					SecurityContext: &corev1.SecurityContext{
						Privileged: &isTrue,
						Capabilities: &corev1.Capabilities{
							Add: []corev1.Capability{"NET_ADMIN", "NET_RAW"},
						},
					},
					LivenessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							HTTPGet: &corev1.HTTPGetAction{
								Path: "/healthz",
								Port: healthPort,
							},
						},
						InitialDelaySeconds: 5,
						PeriodSeconds:       15,
					},
					ReadinessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							HTTPGet: &corev1.HTTPGetAction{
								Path: "/readyz",
								Port: healthPort,
							},
						},
						InitialDelaySeconds: 3,
						PeriodSeconds:       10,
					},
				},
			},
		},
	}

	// Apply pod-level scheduling and resource overrides from spec.pod
	if router.Spec.Pod != nil {
		if router.Spec.Pod.Resources != nil {
			pod.Spec.Containers[0].Resources = *router.Spec.Pod.Resources
		}
		if len(router.Spec.Pod.NodeSelector) > 0 {
			pod.Spec.NodeSelector = router.Spec.Pod.NodeSelector
		}
		if len(router.Spec.Pod.Tolerations) > 0 {
			pod.Spec.Tolerations = router.Spec.Pod.Tolerations
		}
		if router.Spec.Pod.RuntimeClassName != nil {
			pod.Spec.RuntimeClassName = router.Spec.Pod.RuntimeClassName
		}
		if router.Spec.Pod.PriorityClassName != "" {
			pod.Spec.PriorityClassName = router.Spec.Pod.PriorityClassName
		}
	}

	// Append Suricata sidecar container and volumes when IDS/IPS is enabled
	if router.Spec.IDS != nil && router.Spec.IDS.Enabled {
		pod.Spec.Containers = append(pod.Spec.Containers, buildSuricataContainer(router))
		pod.Spec.Volumes = append(pod.Spec.Volumes,
			corev1.Volume{
				Name:         "suricata-config",
				VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
			},
			corev1.Volume{
				Name:         "suricata-rules",
				VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
			},
			corev1.Volume{
				Name:         "suricata-log",
				VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
			},
		)
	}

	// Append metrics exporter sidecar when metrics are enabled
	if router.Spec.Metrics != nil && router.Spec.Metrics.Enabled {
		pod.Spec.Containers = append(pod.Spec.Containers, buildMetricsExporterContainer(router))
		// Shared volume for dnsmasq lease files — PVC if persistence enabled, emptyDir otherwise
		leaseVolume := corev1.Volume{Name: "dnsmasq-leases"}
		if router.Spec.DHCP != nil && router.Spec.DHCP.LeasePersistence != nil && router.Spec.DHCP.LeasePersistence.Enabled {
			leaseVolume.VolumeSource = corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: leasePVCName(router.Name),
				},
			}
		} else {
			leaseVolume.VolumeSource = corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			}
		}
		pod.Spec.Volumes = append(pod.Spec.Volumes, leaseVolume)
		pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts,
			corev1.VolumeMount{Name: "dnsmasq-leases", MountPath: "/var/lib/misc"},
		)
	}

	return pod
}

// buildNetworkConfigJSON generates the NETWORK_CONFIG JSON env var value.
func buildNetworkConfigJSON(router *v1alpha1.VPCRouter) string {
	cfg := networkConfig{
		Interfaces: make([]networkInterfaceConfig, 0, len(router.Spec.Networks)),
	}
	for i, net := range router.Spec.Networks {
		cfg.Interfaces = append(cfg.Interfaces, networkInterfaceConfig{
			Name:    fmt.Sprintf("net%d", i),
			Address: net.Address,
		})
	}
	data, _ := json.Marshal(cfg)
	return string(data)
}
