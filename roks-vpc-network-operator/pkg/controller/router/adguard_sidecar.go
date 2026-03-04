package router

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"

	v1alpha1 "github.com/IBM/roks-vpc-network-operator/api/v1alpha1"
)

const defaultAdGuardImage = "adguard/adguardhome:latest"

// buildAdGuardSidecar constructs the AdGuard Home sidecar container and its volumes.
func buildAdGuardSidecar(policy *v1alpha1.VPCDNSPolicy) (corev1.Container, []corev1.Volume) {
	image := defaultAdGuardImage
	if policy.Spec.Image != "" {
		image = policy.Spec.Image
	}

	container := corev1.Container{
		Name:  "adguard-home",
		Image: image,
		Args:  []string{"--config", "/opt/adguardhome/conf/AdGuardHome.yaml", "--work-dir", "/opt/adguardhome/work", "--no-check-update"},
		Ports: []corev1.ContainerPort{
			{Name: "dns", ContainerPort: 5353, Protocol: corev1.ProtocolUDP},
			{Name: "dns-tcp", ContainerPort: 5353, Protocol: corev1.ProtocolTCP},
			{Name: "web-ui", ContainerPort: 3000, Protocol: corev1.ProtocolTCP},
		},
		VolumeMounts: []corev1.VolumeMount{
			{Name: "adguard-config", MountPath: "/opt/adguardhome/conf", ReadOnly: true},
			{Name: "adguard-work", MountPath: "/opt/adguardhome/work"},
		},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("50m"),
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("200m"),
				corev1.ResourceMemory: resource.MustParse("256Mi"),
			},
		},
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/control/status",
					Port: intstr.FromInt(3000),
				},
			},
			InitialDelaySeconds: 10,
			PeriodSeconds:       30,
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/control/status",
					Port: intstr.FromInt(3000),
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       10,
		},
	}

	volumes := []corev1.Volume{
		{
			Name: "adguard-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: policy.Status.ConfigMapName,
					},
				},
			},
		},
		{
			Name: "adguard-work",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}

	return container, volumes
}
