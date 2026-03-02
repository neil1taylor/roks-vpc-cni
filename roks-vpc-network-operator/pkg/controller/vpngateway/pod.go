package vpngateway

import (
	"encoding/json"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1alpha1 "github.com/IBM/roks-vpc-network-operator/api/v1alpha1"
)

const (
	defaultVPNImage       = "registry.access.redhat.com/ubi9/ubi:latest"
	defaultStrongSwanImage = "strongx509/strongswan:6.0.0"
	defaultTunnelMTU      = int32(1420)
	defaultListenPort     = int32(51820)
)

// multusNetworkAttachment represents one entry in the Multus
// k8s.v1.cni.cncf.io/networks annotation JSON array.
type multusNetworkAttachment struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
	Interface string `json:"interface"`
}

// vpnPodName returns the deterministic pod name for a VPCVPNGateway.
func vpnPodName(name string) string {
	return "vpngw-" + name
}

// computeMSS returns the maximum segment size for the given tunnel MTU.
// Subtracts 40 bytes for IP (20) + TCP (20) headers.
func computeMSS(mtu int32) int32 {
	return mtu - 40
}

// buildWireGuardPod constructs the Pod spec for a WireGuard VPN gateway.
func buildWireGuardPod(vpn *v1alpha1.VPCVPNGateway, gw *v1alpha1.VPCGateway) *corev1.Pod {
	podName := vpnPodName(vpn.Name)
	image := resolveVPNImage(vpn)
	script := buildWireGuardInitScript(vpn)
	envVars := buildWireGuardEnvVars(vpn)
	multusJSON := buildVPNMultusAnnotation(vpn, gw)

	isTrue := true
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: vpn.Namespace,
			Labels: map[string]string{
				"app":                            "vpngateway",
				"vpc.roks.ibm.com/vpngateway":    vpn.Name,
			},
			Annotations: map[string]string{
				"k8s.v1.cni.cncf.io/networks": multusJSON,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         "vpc.roks.ibm.com/v1alpha1",
					Kind:               "VPCVPNGateway",
					Name:               vpn.Name,
					UID:                vpn.UID,
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
							SecretName: vpn.Spec.WireGuard.PrivateKey.Name,
						},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name:    "vpn",
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
								Command: []string{"wg", "show", "wg0"},
							},
						},
						InitialDelaySeconds: 30,
						PeriodSeconds:       30,
					},
					ReadinessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							Exec: &corev1.ExecAction{
								Command: []string{"sh", "-c", "wg show wg0 | grep -q handshake"},
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

// buildWireGuardInitScript generates the bash init script for the WireGuard VPN pod.
func buildWireGuardInitScript(vpn *v1alpha1.VPCVPNGateway) string {
	var sb strings.Builder
	sb.WriteString("set -e\n\n")

	// Install tools
	sb.WriteString("# Install tools\n")
	sb.WriteString("dnf install -y iproute nftables wireguard-tools jq 2>/dev/null || ")
	sb.WriteString("yum install -y iproute nftables wireguard-tools jq 2>/dev/null || ")
	sb.WriteString("apt-get update && apt-get install -y iproute2 nftables wireguard-tools jq 2>/dev/null || true\n\n")

	// Uplink via DHCP
	sb.WriteString("# Uplink via DHCP\n")
	sb.WriteString("dhclient net0 2>/dev/null || true\n\n")

	// Enable IP forwarding
	sb.WriteString("# Enable IP forwarding\n")
	sb.WriteString("sysctl -w net.ipv4.ip_forward=1\n\n")

	// Create WireGuard interface
	sb.WriteString("# Create WireGuard interface\n")
	sb.WriteString("ip link add dev wg0 type wireguard\n")
	sb.WriteString("wg set wg0 listen-port ${WG_LISTEN_PORT} private-key /run/secrets/wireguard/${WG_PRIVATE_KEY_FILE}\n\n")

	// Add peers from VPN_TUNNELS JSON env var
	sb.WriteString("# Add peers from VPN_TUNNELS\n")
	sb.WriteString("echo \"${VPN_TUNNELS}\" | jq -c '.[]' | while read -r tunnel; do\n")
	sb.WriteString("  PEER_KEY=$(echo \"$tunnel\" | jq -r '.peerPublicKey')\n")
	sb.WriteString("  ENDPOINT=$(echo \"$tunnel\" | jq -r '.remoteEndpoint')\n")
	sb.WriteString("  ALLOWED_IPS=$(echo \"$tunnel\" | jq -r '.remoteNetworks | join(\",\")')\n")
	sb.WriteString("  LOCAL_ADDR=$(echo \"$tunnel\" | jq -r '.tunnelAddressLocal')\n")
	sb.WriteString("  wg set wg0 peer \"$PEER_KEY\" endpoint \"${ENDPOINT}:${WG_LISTEN_PORT}\" allowed-ips \"$ALLOWED_IPS\" persistent-keepalive 25\n")
	sb.WriteString("  ip addr add \"$LOCAL_ADDR\" dev wg0 2>/dev/null || true\n")
	sb.WriteString("done\n\n")

	sb.WriteString("ip link set wg0 up\n\n")

	// Add routes for remote networks
	sb.WriteString("# Add routes for remote networks\n")
	sb.WriteString("echo \"${VPN_TUNNELS}\" | jq -r '.[].remoteNetworks[]' | sort -u | while read -r cidr; do\n")
	sb.WriteString("  ip route add \"$cidr\" dev wg0 2>/dev/null || true\n")
	sb.WriteString("done\n\n")

	// MSS clamping (conditional)
	if isMSSClampEnabled(vpn) {
		sb.WriteString("# MSS clamping\n")
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

// buildStrongSwanPod constructs the Pod spec for an IPsec/StrongSwan VPN gateway.
func buildStrongSwanPod(vpn *v1alpha1.VPCVPNGateway, gw *v1alpha1.VPCGateway) *corev1.Pod {
	podName := vpnPodName(vpn.Name)
	image := resolveStrongSwanImage(vpn)
	swanctlConf := generateSwanctlConf(vpn)
	multusJSON := buildVPNMultusAnnotation(vpn, gw)

	isTrue := true

	// Build volumes: one per tunnel PSK secret
	volumes := []corev1.Volume{}
	volumeMounts := []corev1.VolumeMount{}
	for _, tunnel := range vpn.Spec.Tunnels {
		if tunnel.PresharedKey != nil {
			volName := "ipsec-psk-" + tunnel.Name
			volumes = append(volumes, corev1.Volume{
				Name: volName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: tunnel.PresharedKey.Name,
					},
				},
			})
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name:      volName,
				MountPath: "/run/secrets/ipsec/" + tunnel.Name,
				ReadOnly:  true,
			})
		}
	}

	// Build init script
	script := buildStrongSwanInitScript(vpn, swanctlConf)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: vpn.Namespace,
			Labels: map[string]string{
				"app":                            "vpngateway",
				"vpc.roks.ibm.com/vpngateway":    vpn.Name,
			},
			Annotations: map[string]string{
				"k8s.v1.cni.cncf.io/networks": multusJSON,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         "vpc.roks.ibm.com/v1alpha1",
					Kind:               "VPCVPNGateway",
					Name:               vpn.Name,
					UID:                vpn.UID,
					Controller:         &isTrue,
					BlockOwnerDeletion: &isTrue,
				},
			},
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyAlways,
			Volumes:       volumes,
			Containers: []corev1.Container{
				{
					Name:         "vpn",
					Image:        image,
					Command:      []string{"/bin/sh", "-c", script},
					VolumeMounts: volumeMounts,
					SecurityContext: &corev1.SecurityContext{
						Privileged: &isTrue,
						Capabilities: &corev1.Capabilities{
							Add: []corev1.Capability{"NET_ADMIN", "NET_RAW"},
						},
					},
					LivenessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							Exec: &corev1.ExecAction{
								Command: []string{"swanctl", "--list-conns"},
							},
						},
						InitialDelaySeconds: 30,
						PeriodSeconds:       30,
					},
					ReadinessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							Exec: &corev1.ExecAction{
								Command: []string{"sh", "-c", "swanctl --list-sas | grep -q ESTABLISHED"},
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

// buildStrongSwanInitScript generates the init script for the StrongSwan pod.
func buildStrongSwanInitScript(vpn *v1alpha1.VPCVPNGateway, swanctlConf string) string {
	var sb strings.Builder
	sb.WriteString("set -e\n\n")

	// Uplink via DHCP
	sb.WriteString("# Uplink via DHCP\n")
	sb.WriteString("dhclient net0 2>/dev/null || true\n\n")

	// Enable IP forwarding
	sb.WriteString("# Enable IP forwarding\n")
	sb.WriteString("sysctl -w net.ipv4.ip_forward=1\n\n")

	// Write swanctl.conf
	sb.WriteString("# Write swanctl.conf\n")
	sb.WriteString("mkdir -p /etc/swanctl/conf.d\n")
	sb.WriteString(fmt.Sprintf("cat > /etc/swanctl/conf.d/tunnels.conf << 'SWANEOF'\n%s\nSWANEOF\n\n", swanctlConf))

	// MSS clamping (conditional)
	if isMSSClampEnabled(vpn) {
		mtu := resolveTunnelMTU(vpn)
		mss := computeMSS(mtu)
		sb.WriteString("# MSS clamping\n")
		sb.WriteString("nft add table inet mangle 2>/dev/null || true\n")
		sb.WriteString("nft add chain inet mangle forward '{ type filter hook forward priority -150; }' 2>/dev/null || true\n")
		sb.WriteString(fmt.Sprintf("nft add rule inet mangle forward tcp flags syn / syn,rst tcp option maxseg size set %d\n\n", mss))
	}

	// Start charon and load config
	sb.WriteString("# Start StrongSwan\n")
	sb.WriteString("charon-systemd &\n")
	sb.WriteString("sleep 2\n")
	sb.WriteString("swanctl --load-all\n\n")

	// Keep alive
	sb.WriteString("# Keep alive\n")
	sb.WriteString("wait\n")

	return sb.String()
}

// generateSwanctlConf generates a swanctl.conf for StrongSwan from the VPN spec.
func generateSwanctlConf(vpn *v1alpha1.VPCVPNGateway) string {
	var sb strings.Builder
	sb.WriteString("connections {\n")

	for _, tunnel := range vpn.Spec.Tunnels {
		sb.WriteString(fmt.Sprintf("  %s {\n", tunnel.Name))
		sb.WriteString(fmt.Sprintf("    remote_addrs = %s\n", tunnel.RemoteEndpoint))
		sb.WriteString("    local {\n")
		sb.WriteString("      auth = psk\n")
		sb.WriteString("    }\n")
		sb.WriteString("    remote {\n")
		sb.WriteString("      auth = psk\n")
		sb.WriteString("    }\n")
		sb.WriteString("    children {\n")
		sb.WriteString(fmt.Sprintf("      %s {\n", tunnel.Name))
		sb.WriteString(fmt.Sprintf("        remote_ts = %s\n", strings.Join(tunnel.RemoteNetworks, ",")))
		sb.WriteString("        start_action = start\n")
		sb.WriteString("        dpd_action = restart\n")
		sb.WriteString("      }\n")
		sb.WriteString("    }\n")

		// IKE/IPsec policy
		if vpn.Spec.IPsec != nil && vpn.Spec.IPsec.IKEPolicy != nil {
			policy := vpn.Spec.IPsec.IKEPolicy
			if policy.Encryption != "" {
				sb.WriteString(fmt.Sprintf("    proposals = %s-%s-modp%s\n",
					policy.Encryption, resolveIntegrity(policy), resolveDHGroup(policy)))
			}
		}

		sb.WriteString("  }\n")
	}

	sb.WriteString("}\n\n")

	// Secrets
	sb.WriteString("secrets {\n")
	for _, tunnel := range vpn.Spec.Tunnels {
		if tunnel.PresharedKey != nil {
			sb.WriteString(fmt.Sprintf("  ike-%s {\n", tunnel.Name))
			sb.WriteString(fmt.Sprintf("    file = /run/secrets/ipsec/%s/%s\n", tunnel.Name, tunnel.PresharedKey.Key))
			sb.WriteString("  }\n")
		}
	}
	sb.WriteString("}\n")

	return sb.String()
}

// resolveIntegrity returns the integrity algorithm string for swanctl proposals.
func resolveIntegrity(policy *v1alpha1.VPNIPsecPolicy) string {
	if policy.Integrity != "" {
		return policy.Integrity
	}
	return "sha256"
}

// resolveDHGroup returns the DH group string for swanctl proposals.
func resolveDHGroup(policy *v1alpha1.VPNIPsecPolicy) string {
	if policy.DHGroup != nil {
		return fmt.Sprintf("%d", *policy.DHGroup)
	}
	return "14"
}

// buildWireGuardEnvVars constructs environment variables for the WireGuard VPN container.
func buildWireGuardEnvVars(vpn *v1alpha1.VPCVPNGateway) []corev1.EnvVar {
	listenPort := defaultListenPort
	if vpn.Spec.WireGuard != nil && vpn.Spec.WireGuard.ListenPort != nil {
		listenPort = *vpn.Spec.WireGuard.ListenPort
	}

	tunnelMTU := resolveTunnelMTU(vpn)

	mssClamp := "true"
	if vpn.Spec.MTU != nil && vpn.Spec.MTU.MSSClamp != nil && !*vpn.Spec.MTU.MSSClamp {
		mssClamp = "false"
	}

	privateKeyFile := "privateKey"
	if vpn.Spec.WireGuard != nil {
		privateKeyFile = vpn.Spec.WireGuard.PrivateKey.Key
	}

	// Build VPN_TUNNELS JSON
	tunnelsJSON := buildTunnelsJSON(vpn)

	return []corev1.EnvVar{
		{Name: "WG_LISTEN_PORT", Value: fmt.Sprintf("%d", listenPort)},
		{Name: "WG_PRIVATE_KEY_FILE", Value: privateKeyFile},
		{Name: "VPN_TUNNELS", Value: tunnelsJSON},
		{Name: "TUNNEL_MTU", Value: fmt.Sprintf("%d", tunnelMTU)},
		{Name: "MSS_CLAMP", Value: mssClamp},
	}
}

// buildTunnelsJSON serializes the VPN tunnels as a JSON array for the init script.
func buildTunnelsJSON(vpn *v1alpha1.VPCVPNGateway) string {
	type tunnelJSON struct {
		Name                string   `json:"name"`
		RemoteEndpoint      string   `json:"remoteEndpoint"`
		RemoteNetworks      []string `json:"remoteNetworks"`
		PeerPublicKey       string   `json:"peerPublicKey,omitempty"`
		TunnelAddressLocal  string   `json:"tunnelAddressLocal,omitempty"`
		TunnelAddressRemote string   `json:"tunnelAddressRemote,omitempty"`
	}

	tunnels := make([]tunnelJSON, 0, len(vpn.Spec.Tunnels))
	for _, t := range vpn.Spec.Tunnels {
		tunnels = append(tunnels, tunnelJSON{
			Name:                t.Name,
			RemoteEndpoint:      t.RemoteEndpoint,
			RemoteNetworks:      t.RemoteNetworks,
			PeerPublicKey:       t.PeerPublicKey,
			TunnelAddressLocal:  t.TunnelAddressLocal,
			TunnelAddressRemote: t.TunnelAddressRemote,
		})
	}

	data, _ := json.Marshal(tunnels)
	return string(data)
}

// buildVPNMultusAnnotation returns the JSON Multus network annotation for the
// VPN gateway's uplink network (from the referenced VPCGateway).
func buildVPNMultusAnnotation(vpn *v1alpha1.VPCVPNGateway, gw *v1alpha1.VPCGateway) string {
	attachments := []multusNetworkAttachment{
		{
			Name:      gw.Spec.Uplink.Network,
			Namespace: gw.Spec.Uplink.Namespace,
			Interface: "net0",
		},
	}
	data, _ := json.Marshal(attachments)
	return string(data)
}

// resolveVPNImage determines the container image for the WireGuard VPN pod.
func resolveVPNImage(vpn *v1alpha1.VPCVPNGateway) string {
	if vpn.Spec.Pod != nil && vpn.Spec.Pod.Image != "" {
		return vpn.Spec.Pod.Image
	}
	return defaultVPNImage
}

// resolveStrongSwanImage determines the container image for the StrongSwan VPN pod.
func resolveStrongSwanImage(vpn *v1alpha1.VPCVPNGateway) string {
	if vpn.Spec.IPsec != nil && vpn.Spec.IPsec.Image != "" {
		return vpn.Spec.IPsec.Image
	}
	if vpn.Spec.Pod != nil && vpn.Spec.Pod.Image != "" {
		return vpn.Spec.Pod.Image
	}
	return defaultStrongSwanImage
}

// resolveTunnelMTU returns the tunnel MTU from the VPN spec or the default.
func resolveTunnelMTU(vpn *v1alpha1.VPCVPNGateway) int32 {
	if vpn.Spec.MTU != nil && vpn.Spec.MTU.TunnelMTU != nil {
		return *vpn.Spec.MTU.TunnelMTU
	}
	return defaultTunnelMTU
}

// isMSSClampEnabled checks whether MSS clamping is enabled on the VPN gateway.
// Defaults to true if not explicitly set.
func isMSSClampEnabled(vpn *v1alpha1.VPCVPNGateway) bool {
	if vpn.Spec.MTU != nil && vpn.Spec.MTU.MSSClamp != nil {
		return *vpn.Spec.MTU.MSSClamp
	}
	return true // default
}

// vpnPodNeedsRecreation checks whether the existing pod is outdated compared
// to the desired pod spec (protocol, image, tunnels, gateway).
func vpnPodNeedsRecreation(existing *corev1.Pod, desired *corev1.Pod) bool {
	if len(existing.Spec.Containers) == 0 || len(desired.Spec.Containers) == 0 {
		return true
	}

	// Check image
	if existing.Spec.Containers[0].Image != desired.Spec.Containers[0].Image {
		return true
	}

	// Check Multus annotation (gateway uplink changed)
	existingMultus := existing.Annotations["k8s.v1.cni.cncf.io/networks"]
	desiredMultus := desired.Annotations["k8s.v1.cni.cncf.io/networks"]
	if existingMultus != desiredMultus {
		return true
	}

	// Check env vars (tunnel config changed)
	existingEnv := envMapFromContainer(existing.Spec.Containers[0])
	desiredEnv := envMapFromContainer(desired.Spec.Containers[0])
	for k, v := range desiredEnv {
		if existingEnv[k] != v {
			return true
		}
	}

	return false
}

// envMapFromContainer extracts env vars into a map for comparison.
func envMapFromContainer(c corev1.Container) map[string]string {
	m := make(map[string]string, len(c.Env))
	for _, e := range c.Env {
		m[e.Name] = e.Value
	}
	return m
}
