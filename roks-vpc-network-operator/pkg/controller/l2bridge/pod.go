package l2bridge

import (
	"encoding/json"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1alpha1 "github.com/IBM/roks-vpc-network-operator/api/v1alpha1"
)

const (
	defaultBridgeImage = "registry.access.redhat.com/ubi9/ubi:latest"
	defaultTunnelMTU   = int32(1400)
	defaultListenPort  = int32(51820)
)

// multusNetworkAttachment represents one entry in the Multus
// k8s.v1.cni.cncf.io/networks annotation JSON array.
type multusNetworkAttachment struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
	Interface string `json:"interface"`
}

// bridgePodName returns the deterministic pod name for a VPCL2Bridge.
func bridgePodName(name string) string {
	return "l2bridge-" + name
}

// computeMSS returns the maximum segment size for the given tunnel MTU.
// Subtracts 40 bytes for IP (20) + TCP (20) headers.
func computeMSS(mtu int32) int32 {
	return mtu - 40
}

// buildGRETAPPod constructs the Pod spec for a GRETAP-over-WireGuard L2 bridge.
//
// The pod attaches to:
//   - The workload network (from bridge.Spec.NetworkRef) as interface "net0"
//
// The pod creates:
//  1. A WireGuard tunnel to the remote peer
//  2. A GRETAP tunnel over WireGuard for L2 frames
//  3. A Linux bridge joining gretap0 and net0
//  4. Optional TCP MSS clamping via nftables
func buildGRETAPPod(bridge *v1alpha1.VPCL2Bridge, gw *v1alpha1.VPCGateway) *corev1.Pod {
	podName := bridgePodName(bridge.Name)

	// Build Multus annotation
	multusJSON := buildMultusAnnotation(bridge)

	// Determine container image
	image := resolveBridgeImage(bridge)

	// Build the init script
	script := buildGRETAPInitScript(bridge)

	// Build environment variables
	envVars := buildBridgeEnvVars(bridge)

	isTrue := true
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: bridge.Namespace,
			Labels: map[string]string{
				"app":                          "l2bridge",
				"vpc.roks.ibm.com/l2bridge":    bridge.Name,
			},
			Annotations: map[string]string{
				"k8s.v1.cni.cncf.io/networks": multusJSON,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         "vpc.roks.ibm.com/v1alpha1",
					Kind:               "VPCL2Bridge",
					Name:               bridge.Name,
					UID:                bridge.UID,
					Controller:         &isTrue,
					BlockOwnerDeletion: &isTrue,
				},
			},
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyAlways,
			Volumes: []corev1.Volume{
				{
					Name: "wireguard-key",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: bridge.Spec.Remote.WireGuard.PrivateKey.Name,
						},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name:    "bridge",
					Image:   image,
					Command: []string{"/bin/bash", "-c", script},
					Env:     envVars,
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "wireguard-key",
							MountPath: "/run/secrets/wireguard",
							ReadOnly:  true,
						},
					},
					SecurityContext: &corev1.SecurityContext{
						Privileged: &isTrue,
						Capabilities: &corev1.Capabilities{
							Add: []corev1.Capability{"NET_ADMIN", "NET_RAW"},
						},
					},
					LivenessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							Exec: &corev1.ExecAction{
								Command: []string{"ip", "link", "show", "br-l2"},
							},
						},
						InitialDelaySeconds: 30,
						PeriodSeconds:       30,
					},
					ReadinessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							Exec: &corev1.ExecAction{
								Command: []string{"sh", "-c", "ip link show wg0 | grep -q UP && ip link show br-l2 | grep -q UP"},
							},
						},
						InitialDelaySeconds: 10,
						PeriodSeconds:       10,
					},
				},
			},
		},
	}

	return pod
}

// buildGRETAPInitScript generates the bash init script for the GRETAP-over-WireGuard bridge pod.
// All values are passed via environment variables set on the container.
func buildGRETAPInitScript(bridge *v1alpha1.VPCL2Bridge) string {
	var sb strings.Builder
	sb.WriteString("set -e\n\n")

	// Install tools
	sb.WriteString("# Install tools\n")
	sb.WriteString("dnf install -y iproute nftables wireguard-tools 2>/dev/null || ")
	sb.WriteString("yum install -y iproute nftables wireguard-tools 2>/dev/null || ")
	sb.WriteString("apt-get update && apt-get install -y iproute2 nftables wireguard-tools 2>/dev/null || true\n\n")

	// Step 1: WireGuard
	sb.WriteString("# Step 1: WireGuard\n")
	sb.WriteString("ip link add dev wg0 type wireguard\n")
	sb.WriteString("ip addr add ${WG_LOCAL_ADDR} dev wg0\n")
	sb.WriteString("wg set wg0 private-key /run/secrets/wireguard/privateKey peer ${WG_PEER_PUBLIC_KEY} endpoint ${WG_REMOTE_ENDPOINT}:${WG_LISTEN_PORT} allowed-ips 0.0.0.0/0\n")
	sb.WriteString("ip link set wg0 up\n\n")

	// Step 2: GRETAP
	sb.WriteString("# Step 2: GRETAP\n")
	sb.WriteString("ip link add dev gretap0 type gretap local ${GRETAP_LOCAL} remote ${GRETAP_REMOTE} ttl 255\n")
	sb.WriteString("ip link set gretap0 mtu ${TUNNEL_MTU}\n")
	sb.WriteString("ip link set gretap0 up\n\n")

	// Step 3: Bridge
	sb.WriteString("# Step 3: Bridge\n")
	sb.WriteString("ip link add name br-l2 type bridge\n")
	sb.WriteString("ip link set gretap0 master br-l2\n")
	sb.WriteString("ip link set net0 master br-l2\n")
	sb.WriteString("ip link set br-l2 up\n\n")

	// Step 4: MSS clamping (conditional)
	mssEnabled := isMSSClampEnabled(bridge)
	if mssEnabled {
		sb.WriteString("# Step 4: MSS clamping\n")
		sb.WriteString("if [ \"${MSS_CLAMP}\" = \"true\" ]; then\n")
		sb.WriteString("  MSS=$((${TUNNEL_MTU} - 40))\n")
		sb.WriteString("  nft add table inet mangle\n")
		sb.WriteString("  nft add chain inet mangle forward '{ type filter hook forward priority -150; }'\n")
		sb.WriteString("  nft add rule inet mangle forward tcp flags syn / syn,rst tcp option maxseg size set ${MSS}\n")
		sb.WriteString("fi\n\n")
	}

	// Keep alive
	sb.WriteString("# Keep alive\n")
	sb.WriteString("exec sleep infinity\n")

	return sb.String()
}

// buildBridgeEnvVars constructs the environment variables for the bridge container.
func buildBridgeEnvVars(bridge *v1alpha1.VPCL2Bridge) []corev1.EnvVar {
	wg := bridge.Spec.Remote.WireGuard

	// Parse tunnel local address without prefix for GRETAP local
	gretapLocal := stripPrefix(wg.TunnelAddressLocal)
	// Parse tunnel remote address without prefix for GRETAP remote
	gretapRemote := stripPrefix(wg.TunnelAddressRemote)

	// Resolve tunnel MTU
	tunnelMTU := defaultTunnelMTU
	if bridge.Spec.MTU != nil && bridge.Spec.MTU.TunnelMTU != nil {
		tunnelMTU = *bridge.Spec.MTU.TunnelMTU
	}

	// Resolve listen port
	listenPort := defaultListenPort
	if wg.ListenPort != nil {
		listenPort = *wg.ListenPort
	}

	// Resolve MSS clamp
	mssClamp := "true"
	if bridge.Spec.MTU != nil && bridge.Spec.MTU.MSSClamp != nil && !*bridge.Spec.MTU.MSSClamp {
		mssClamp = "false"
	}

	return []corev1.EnvVar{
		{Name: "WG_LOCAL_ADDR", Value: wg.TunnelAddressLocal},
		{Name: "WG_REMOTE_ENDPOINT", Value: bridge.Spec.Remote.Endpoint},
		{Name: "WG_PEER_PUBLIC_KEY", Value: wg.PeerPublicKey},
		{Name: "WG_LISTEN_PORT", Value: fmt.Sprintf("%d", listenPort)},
		{Name: "GRETAP_LOCAL", Value: gretapLocal},
		{Name: "GRETAP_REMOTE", Value: gretapRemote},
		{Name: "TUNNEL_MTU", Value: fmt.Sprintf("%d", tunnelMTU)},
		{Name: "MSS_CLAMP", Value: mssClamp},
	}
}

// buildMultusAnnotation returns the JSON Multus network annotation for the
// workload network attachment.
func buildMultusAnnotation(bridge *v1alpha1.VPCL2Bridge) string {
	attachments := []multusNetworkAttachment{
		{
			Name:      bridge.Spec.NetworkRef.Name,
			Namespace: bridge.Spec.NetworkRef.Namespace,
			Interface: "net0",
		},
	}
	data, _ := json.Marshal(attachments)
	return string(data)
}

// buildL2VPNPod constructs the Pod spec for an NSX-T L2VPN bridge.
// This is a stub — full NSX Edge integration is planned for a future release.
func buildL2VPNPod(bridge *v1alpha1.VPCL2Bridge, gw *v1alpha1.VPCGateway) *corev1.Pod {
	podName := bridgePodName(bridge.Name)
	isTrue := true

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: bridge.Namespace,
			Labels: map[string]string{
				"app":                       "l2bridge",
				"vpc.roks.ibm.com/l2bridge": bridge.Name,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         "vpc.roks.ibm.com/v1alpha1",
					Kind:               "VPCL2Bridge",
					Name:               bridge.Name,
					UID:                bridge.UID,
					Controller:         &isTrue,
					BlockOwnerDeletion: &isTrue,
				},
			},
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyAlways,
			Containers: []corev1.Container{
				{
					Name:    "l2vpn-edge",
					Image:   resolveBridgeImage(bridge),
					Command: []string{"/bin/bash", "-c", "echo 'NSX L2VPN stub — not yet implemented'; exec sleep infinity"},
					SecurityContext: &corev1.SecurityContext{
						Privileged: &isTrue,
						Capabilities: &corev1.Capabilities{
							Add: []corev1.Capability{"NET_ADMIN"},
						},
					},
				},
			},
		},
	}
}

// buildEVPNPod constructs the Pod spec for an FRR-based EVPN-VXLAN bridge.
// This is a stub — full FRR EVPN integration is planned for a future release.
func buildEVPNPod(bridge *v1alpha1.VPCL2Bridge, gw *v1alpha1.VPCGateway) *corev1.Pod {
	podName := bridgePodName(bridge.Name)
	isTrue := true

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: bridge.Namespace,
			Labels: map[string]string{
				"app":                       "l2bridge",
				"vpc.roks.ibm.com/l2bridge": bridge.Name,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         "vpc.roks.ibm.com/v1alpha1",
					Kind:               "VPCL2Bridge",
					Name:               bridge.Name,
					UID:                bridge.UID,
					Controller:         &isTrue,
					BlockOwnerDeletion: &isTrue,
				},
			},
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyAlways,
			Containers: []corev1.Container{
				{
					Name:    "frr-evpn",
					Image:   resolveBridgeImage(bridge),
					Command: []string{"/bin/bash", "-c", "echo 'FRR EVPN-VXLAN stub — not yet implemented'; exec sleep infinity"},
					SecurityContext: &corev1.SecurityContext{
						Privileged: &isTrue,
						Capabilities: &corev1.Capabilities{
							Add: []corev1.Capability{"NET_ADMIN"},
						},
					},
				},
			},
		},
	}
}

// resolveBridgeImage determines the container image for the bridge pod.
// Uses bridge.Spec.Pod.Image if set, otherwise falls back to the default.
func resolveBridgeImage(bridge *v1alpha1.VPCL2Bridge) string {
	if bridge.Spec.Pod != nil && bridge.Spec.Pod.Image != "" {
		return bridge.Spec.Pod.Image
	}
	return defaultBridgeImage
}

// stripPrefix removes the CIDR prefix from an IP address string.
// For example, "10.99.0.1/30" -> "10.99.0.1".
func stripPrefix(addr string) string {
	if idx := strings.IndexByte(addr, '/'); idx >= 0 {
		return addr[:idx]
	}
	return addr
}

// isMSSClampEnabled checks whether MSS clamping is enabled on the bridge.
// Defaults to true if not explicitly set.
func isMSSClampEnabled(bridge *v1alpha1.VPCL2Bridge) bool {
	if bridge.Spec.MTU != nil && bridge.Spec.MTU.MSSClamp != nil {
		return *bridge.Spec.MTU.MSSClamp
	}
	return true // default
}
