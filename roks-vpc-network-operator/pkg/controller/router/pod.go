package router

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1alpha1 "github.com/IBM/roks-vpc-network-operator/api/v1alpha1"
	"github.com/IBM/roks-vpc-network-operator/pkg/controller/gateway"
)

const (
	defaultRouterImage = "quay.io/fedora/fedora:40"
	routerPodImageEnv  = "ROUTER_POD_IMAGE"
)

// multusNetworkAttachment represents one entry in the Multus
// k8s.v1.cni.cncf.io/networks annotation JSON array.
type multusNetworkAttachment struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
	Interface string `json:"interface"`
	MAC       string `json:"mac,omitempty"`
}

// routerPodName returns the deterministic pod name for a VPCRouter.
func routerPodName(router *v1alpha1.VPCRouter) string {
	return router.Name + "-pod"
}

// buildRouterPod constructs the Pod spec for a VPCRouter.
//
// The pod attaches to:
//   - The uplink network (using the gateway's VNI MAC) as interface "uplink"
//   - Each workload network from router.Spec.Networks as interfaces "net0", "net1", etc.
//
// The init script:
//  1. Configures IP addresses on all interfaces
//  2. Enables IP forwarding
//  3. Sets the default route via the uplink subnet gateway
//  4. Applies explicit NAT rules from the gateway's NAT spec (if any)
//  5. Applies firewall rules (if any)
//  6. Starts dnsmasq for DHCP (if enabled)
func buildRouterPod(router *v1alpha1.VPCRouter, gw *v1alpha1.VPCGateway) *corev1.Pod {
	podName := routerPodName(router)

	// Build Multus network attachments
	attachments := buildMultusAttachments(router, gw)
	multusJSON, _ := json.Marshal(attachments)

	// Determine container image
	image := resolveRouterImage(router, gw)

	// Build the init script
	script := buildInitScript(router, gw)

	// Build environment variables
	envVars := buildEnvVars(router, gw)

	isTrue := true
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
					Command: []string{"/bin/bash", "-c", script},
					Env:     envVars,
					SecurityContext: &corev1.SecurityContext{
						Privileged: &isTrue,
						Capabilities: &corev1.Capabilities{
							Add: []corev1.Capability{"NET_ADMIN", "NET_RAW"},
						},
					},
				},
			},
		},
	}

	return pod
}

// buildMultusAttachments creates the Multus network-attachment list.
func buildMultusAttachments(router *v1alpha1.VPCRouter, gw *v1alpha1.VPCGateway) []multusNetworkAttachment {
	attachments := make([]multusNetworkAttachment, 0, 1+len(router.Spec.Networks))

	// Uplink interface — uses the gateway's VNI MAC for identity
	attachments = append(attachments, multusNetworkAttachment{
		Name:      gw.Spec.Uplink.Network,
		Namespace: gw.Spec.Uplink.Namespace,
		Interface: "uplink",
		MAC:       gw.Status.MACAddress,
	})

	// Workload interfaces
	for i, net := range router.Spec.Networks {
		attachments = append(attachments, multusNetworkAttachment{
			Name:      net.Name,
			Namespace: net.Namespace,
			Interface: fmt.Sprintf("net%d", i),
		})
	}

	return attachments
}

// resolveRouterImage determines the container image, checking in priority order:
// router.Spec.Pod.Image > gw.Spec.Pod.Image > ROUTER_POD_IMAGE env > default
func resolveRouterImage(router *v1alpha1.VPCRouter, gw *v1alpha1.VPCGateway) string {
	if router.Spec.Pod != nil && router.Spec.Pod.Image != "" {
		return router.Spec.Pod.Image
	}
	if gw.Spec.Pod != nil && gw.Spec.Pod.Image != "" {
		return gw.Spec.Pod.Image
	}
	if img := os.Getenv(routerPodImageEnv); img != "" {
		return img
	}
	return defaultRouterImage
}

// buildInitScript generates the bash init script for the router pod.
func buildInitScript(router *v1alpha1.VPCRouter, gw *v1alpha1.VPCGateway) string {
	var sb strings.Builder
	sb.WriteString("set -e\n\n")

	// Install networking tools (including DHCP client for uplink)
	sb.WriteString("# Install networking tools\n")
	sb.WriteString("dnf install -y iproute nftables procps-ng dnsmasq dhcp-client 2>/dev/null || ")
	sb.WriteString("yum install -y iproute nftables procps-ng dnsmasq dhcp-client 2>/dev/null || ")
	sb.WriteString("apt-get update && apt-get install -y iproute2 nftables dnsmasq isc-dhcp-client 2>/dev/null || true\n\n")

	// Configure uplink interface via DHCP — the VNI MAC is set by Multus,
	// so VPC's DHCP server will assign the reserved IP and gateway, just like a VM.
	sb.WriteString("# Configure uplink interface via DHCP\n")
	sb.WriteString("ip link set uplink up\n")
	sb.WriteString("dhclient -v uplink\n")
	sb.WriteString("echo 'Uplink DHCP complete:'\n")
	sb.WriteString("ip addr show dev uplink\n")
	sb.WriteString("ip route show dev uplink\n\n")

	// Configure workload interfaces
	sb.WriteString("# Configure workload interfaces\n")
	for i, net := range router.Spec.Networks {
		ifName := fmt.Sprintf("net%d", i)
		sb.WriteString(fmt.Sprintf("ip addr add %s dev %s 2>/dev/null || true\n", net.Address, ifName))
		sb.WriteString(fmt.Sprintf("ip link set %s up\n", ifName))
	}
	sb.WriteString("\n")

	// Enable IP forwarding
	sb.WriteString("# Enable IP forwarding\n")
	sb.WriteString("sysctl -w net.ipv4.ip_forward=1\n\n")

	// Default route is set by DHCP on the uplink interface

	// Apply explicit NAT rules (only if gateway has nat.snat/dnat configured)
	sb.WriteString("# Apply NAT rules (only if explicitly configured on gateway)\n")
	sb.WriteString("if [ -n \"$NFTABLES_CONFIG\" ]; then\n")
	sb.WriteString("  echo \"$NFTABLES_CONFIG\" | nft -f -\n")
	sb.WriteString("fi\n\n")

	// Apply firewall rules if configured
	sb.WriteString("# Apply firewall rules\n")
	sb.WriteString("if [ -n \"$FIREWALL_CONFIG\" ]; then\n")
	sb.WriteString("  echo \"$FIREWALL_CONFIG\" | nft -f -\n")
	sb.WriteString("fi\n\n")

	// Optional DHCP server
	sb.WriteString("# Optional: start dnsmasq for DHCP\n")
	sb.WriteString("if [ \"$DHCP_ENABLED\" = \"true\" ]; then\n")
	for i, net := range router.Spec.Networks {
		ifName := fmt.Sprintf("net%d", i)
		dhcpRange := computeDHCPRange(net.Address)
		if dhcpRange != "" {
			sb.WriteString(fmt.Sprintf("  dnsmasq --interface=%s --dhcp-range=%s --no-daemon --log-dhcp &\n", ifName, dhcpRange))
		}
	}
	sb.WriteString("fi\n\n")

	sb.WriteString("# Keep the pod running\n")
	sb.WriteString("exec sleep infinity\n")

	return sb.String()
}

// buildEnvVars constructs the environment variables for the router container.
func buildEnvVars(router *v1alpha1.VPCRouter, gw *v1alpha1.VPCGateway) []corev1.EnvVar {
	envVars := []corev1.EnvVar{}

	// NAT config — only present if the gateway has explicit NAT rules.
	// If PAR is configured, pass the PAR CIDR so SNAT defaults to the first PAR IP.
	nftConfig := gateway.GenerateNftablesConfig(gw.Spec.NAT, gw.Status.ReservedIP, gw.Status.PublicAddressRangeCIDR)
	if nftConfig != "" {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "NFTABLES_CONFIG",
			Value: nftConfig,
		})
	}

	// Firewall config
	fwConfig := generateFirewallConfig(router.Spec.Firewall)
	if fwConfig != "" {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "FIREWALL_CONFIG",
			Value: fwConfig,
		})
	}

	// DHCP
	dhcpEnabled := "false"
	if router.Spec.DHCP != nil && router.Spec.DHCP.Enabled {
		dhcpEnabled = "true"
	}
	envVars = append(envVars, corev1.EnvVar{
		Name:  "DHCP_ENABLED",
		Value: dhcpEnabled,
	})

	return envVars
}

// generateFirewallConfig generates nftables firewall rules from the router's
// firewall spec.
func generateFirewallConfig(fw *v1alpha1.GatewayFirewall) string {
	if fw == nil || !fw.Enabled || len(fw.Rules) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("table ip filter {\n")
	sb.WriteString("  chain forward {\n")
	sb.WriteString("    type filter hook forward priority 0; policy drop;\n")
	sb.WriteString("    ct state established,related accept\n")

	for _, rule := range fw.Rules {
		sb.WriteString("    ")
		// Direction
		if rule.Direction == "ingress" {
			sb.WriteString("iifname \"uplink\" ")
		} else {
			sb.WriteString("oifname \"uplink\" ")
		}

		// Source/Destination
		if rule.Source != "" {
			sb.WriteString(fmt.Sprintf("ip saddr %s ", rule.Source))
		}
		if rule.Destination != "" {
			sb.WriteString(fmt.Sprintf("ip daddr %s ", rule.Destination))
		}

		// Protocol + port
		if rule.Protocol != "" && rule.Protocol != "any" {
			sb.WriteString(fmt.Sprintf("meta l4proto %s ", rule.Protocol))
			if rule.Port != nil {
				sb.WriteString(fmt.Sprintf("th dport %d ", *rule.Port))
			}
		}

		// Action
		sb.WriteString(rule.Action)
		sb.WriteString("\n")
	}

	sb.WriteString("  }\n")
	sb.WriteString("}\n")
	return sb.String()
}

// computeSubnetGateway derives the VPC subnet gateway IP from a host IP.
// VPC subnets always use the first usable address (x.x.x.1) as the gateway.
// For example, "10.240.1.5" → "10.240.1.1".
func computeSubnetGateway(ip string) string {
	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		return ""
	}
	parts[3] = "1"
	return strings.Join(parts, ".")
}

// computeDHCPRange derives a DHCP range from a network address with prefix.
// For example, "10.100.0.1/24" → "10.100.0.10,10.100.0.254,255.255.255.0,12h"
func computeDHCPRange(address string) string {
	parts := strings.SplitN(address, "/", 2)
	if len(parts) != 2 {
		return ""
	}
	ipParts := strings.Split(parts[0], ".")
	if len(ipParts) != 4 {
		return ""
	}

	// Simple /24 assumption for DHCP range
	base := strings.Join(ipParts[:3], ".")
	return fmt.Sprintf("%s.10,%s.254,255.255.255.0,12h", base, base)
}
