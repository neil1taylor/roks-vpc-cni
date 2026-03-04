package router

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net"
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

	defaultFastpathImage    = "de.icr.io/roks/vpc-router-fastpath:latest"
	routerPodFastpathImgEnv = "ROUTER_POD_FASTPATH_IMAGE"
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
	// Dispatch to fast-path mode if configured
	if router.Spec.Mode == "fast-path" {
		return buildFastpathRouterPod(router, gw)
	}

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
					LivenessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							Exec: &corev1.ExecAction{
								Command: []string{"sysctl", "-n", "net.ipv4.ip_forward"},
							},
						},
						InitialDelaySeconds: 30,
						PeriodSeconds:       30,
					},
					ReadinessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							Exec: &corev1.ExecAction{
								Command: []string{"sh", "-c", "ip route show default | grep -q uplink && ip link show uplink | grep -q UP"},
							},
						},
						InitialDelaySeconds: 10,
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
		// Shared volume for dnsmasq lease files
		pod.Spec.Volumes = append(pod.Spec.Volumes,
			corev1.Volume{
				Name:         "dnsmasq-leases",
				VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
			},
		)
		// Mount in router container so dnsmasq writes leases there
		pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts,
			corev1.VolumeMount{Name: "dnsmasq-leases", MountPath: "/var/lib/misc"},
		)
	}

	return pod
}

// buildSuricataContainer constructs the Suricata sidecar container spec.
func buildSuricataContainer(router *v1alpha1.VPCRouter) corev1.Container {
	ids := router.Spec.IDS
	image := resolveSuricataImage(router)
	startScript := generateSuricataStartScript(ids)
	suricataYAML := generateSuricataConfig(ids, router.Spec.Networks)

	envVars := []corev1.EnvVar{
		{Name: "SURICATA_MODE", Value: ids.Mode},
		{Name: "SURICATA_YAML", Value: suricataYAML},
	}
	if ids.CustomRules != "" {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "CUSTOM_RULES",
			Value: ids.CustomRules,
		})
	}

	isTrue := true
	return corev1.Container{
		Name:    "suricata",
		Image:   image,
		Command: []string{"/bin/sh", "-c", startScript},
		Env:     envVars,
		SecurityContext: &corev1.SecurityContext{
			Privileged: &isTrue,
			Capabilities: &corev1.Capabilities{
				Add: []corev1.Capability{"NET_ADMIN", "NET_RAW", "SYS_NICE"},
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{Name: "suricata-config", MountPath: "/etc/suricata"},
			{Name: "suricata-rules", MountPath: "/var/lib/suricata/rules"},
			{Name: "suricata-log", MountPath: "/var/log/suricata"},
		},
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				Exec: &corev1.ExecAction{
					Command: []string{"test", "-f", "/var/run/suricata.pid"},
				},
			},
			InitialDelaySeconds: 60,
			PeriodSeconds:       30,
		},
	}
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

	// Configure uplink interface via DHCP with static IP fallback.
	// The VNI MAC is set by Multus, so VPC's DHCP server will assign the reserved IP.
	// If DHCP fails, fall back to static assignment from gateway status.
	sb.WriteString("# Configure uplink interface via DHCP (with static fallback)\n")
	sb.WriteString("ip link set uplink up\n")
	sb.WriteString("if ! dhclient -v -timeout 30 uplink 2>/dev/null; then\n")
	sb.WriteString("  echo 'DHCP failed on uplink, falling back to static IP from gateway'\n")
	sb.WriteString("  if [ -n \"${GW_RESERVED_IP}\" ]; then\n")
	sb.WriteString("    GW_SUBNET=$(echo ${GW_RESERVED_IP} | cut -d. -f1-3)\n")
	sb.WriteString("    ip addr add ${GW_RESERVED_IP}/24 dev uplink 2>/dev/null || true\n")
	sb.WriteString("    ip route replace default via ${GW_SUBNET}.1 dev uplink 2>/dev/null || true\n")
	sb.WriteString("  else\n")
	sb.WriteString("    echo 'ERROR: DHCP failed and no GW_RESERVED_IP set'\n")
	sb.WriteString("    exit 1\n")
	sb.WriteString("  fi\n")
	sb.WriteString("fi\n")
	sb.WriteString("echo 'Uplink DHCP complete:'\n")
	sb.WriteString("ip addr show dev uplink\n")
	sb.WriteString("ip route show dev uplink\n\n")

	// Configure workload interfaces
	sb.WriteString("# Configure workload interfaces\n")
	for i, net := range router.Spec.Networks {
		ifName := fmt.Sprintf("net%d", i)
		addr := net.Address
		if !strings.Contains(addr, "/") {
			addr = addr + "/24"
		}
		sb.WriteString(fmt.Sprintf("ip addr add %s dev %s 2>/dev/null || true\n", addr, ifName))
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

	// Apply IPS NFQUEUE rules if configured (Suricata inline mode)
	sb.WriteString("# Apply IPS NFQUEUE rules (Suricata inline inspection)\n")
	sb.WriteString("if [ -n \"$IPS_NFQUEUE_CONFIG\" ]; then\n")
	sb.WriteString("  echo \"$IPS_NFQUEUE_CONFIG\" | nft -f -\n")
	sb.WriteString("fi\n\n")

	// DHCP servers — per-network with merged global/local config
	sb.WriteString("# DHCP servers\n")
	for i, netSpec := range router.Spec.Networks {
		cfg := resolvedDHCPConfig(router.Spec.DHCP, netSpec)
		if cfg == nil {
			continue
		}
		ifName := fmt.Sprintf("net%d", i)
		sb.WriteString(generateDnsmasqCommand(ifName, netSpec.Address, cfg) + " &\n")
	}
	sb.WriteString("\n")

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

	// DHCP — check if any network has DHCP enabled (global or per-network)
	anyDHCP := false
	for _, netSpec := range router.Spec.Networks {
		if resolvedDHCPConfig(router.Spec.DHCP, netSpec) != nil {
			anyDHCP = true
			break
		}
	}
	dhcpEnabled := "false"
	if anyDHCP {
		dhcpEnabled = "true"
	}
	envVars = append(envVars, corev1.EnvVar{
		Name:  "DHCP_ENABLED",
		Value: dhcpEnabled,
	})

	// DHCP config hash — ensures podNeedsRecreation detects DHCP changes
	envVars = append(envVars, corev1.EnvVar{
		Name:  "DHCP_CONFIG_HASH",
		Value: dhcpConfigHash(router),
	})

	// IPS NFQUEUE nftables rules — applied by the router init container
	nfqConfig := generateNFQueueRules(router.Spec.IDS, router.Spec.Networks)
	if nfqConfig != "" {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "IPS_NFQUEUE_CONFIG",
			Value: nfqConfig,
		})
	}

	// Gateway reserved IP — used as DHCP fallback for uplink configuration
	if gw.Status.ReservedIP != "" {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "GW_RESERVED_IP",
			Value: gw.Status.ReservedIP,
		})
	}

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
	// Allow inter-VM traffic that doesn't traverse the uplink
	sb.WriteString("    iifname != \"uplink\" oifname != \"uplink\" accept\n")

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

		// Counter + Action (map CRD values to nftables syntax)
		sb.WriteString("counter ")
		switch rule.Action {
		case "deny":
			sb.WriteString("drop")
		case "allow":
			sb.WriteString("accept")
		default:
			sb.WriteString(rule.Action) // pass through accept/drop if already nftables syntax
		}
		sb.WriteString("\n")
	}

	sb.WriteString("  }\n")
	sb.WriteString("}\n")
	return sb.String()
}

// resolvedDHCP holds the fully merged DHCP config for a single network.
type resolvedDHCP struct {
	LeaseTime    string
	Range        *v1alpha1.NetworkDHCPRange
	Reservations []v1alpha1.DHCPStaticReservation
	DNS          *v1alpha1.DHCPDNSConfig
	Options      *v1alpha1.DHCPOptions
}

// resolvedDHCPConfig merges global spec.dhcp with per-network spec.networks[].dhcp.
// Returns nil if DHCP is disabled for this network.
func resolvedDHCPConfig(global *v1alpha1.RouterDHCP, net v1alpha1.RouterNetwork) *resolvedDHCP {
	globalEnabled := global != nil && global.Enabled

	// Per-network enabled overrides global
	if net.DHCP != nil && net.DHCP.Enabled != nil {
		if !*net.DHCP.Enabled {
			return nil
		}
		// per-network explicitly enabled
	} else if !globalEnabled {
		return nil
	}

	cfg := &resolvedDHCP{
		LeaseTime: "12h", // default
	}

	// Apply global settings
	if global != nil {
		if global.LeaseTime != "" {
			cfg.LeaseTime = global.LeaseTime
		}
		cfg.DNS = global.DNS
		cfg.Options = global.Options
	}

	// Apply per-network overrides (replace wholesale, not field-level merge)
	if net.DHCP != nil {
		if net.DHCP.LeaseTime != "" {
			cfg.LeaseTime = net.DHCP.LeaseTime
		}
		if net.DHCP.Range != nil {
			cfg.Range = net.DHCP.Range
		}
		if len(net.DHCP.Reservations) > 0 {
			cfg.Reservations = net.DHCP.Reservations
		}
		if net.DHCP.DNS != nil {
			cfg.DNS = net.DHCP.DNS
		}
		if net.DHCP.Options != nil {
			cfg.Options = net.DHCP.Options
		}
	}

	return cfg
}

// computeDHCPRangeWithLease derives a DHCP range from a network address with a parameterized lease time.
func computeDHCPRangeWithLease(address, leaseTime string) string {
	if !strings.Contains(address, "/") {
		address = address + "/24"
	}
	_, ipNet, err := net.ParseCIDR(address)
	if err != nil {
		return ""
	}

	start := make(net.IP, len(ipNet.IP))
	copy(start, ipNet.IP)
	start[len(start)-1] += 10

	end := broadcastIP(ipNet)
	end[len(end)-1]--

	mask := net.IP(ipNet.Mask)
	return fmt.Sprintf("%s,%s,%s,%s", start, end, mask, leaseTime)
}

// generateDnsmasqCommand maps a resolved DHCP config to a dnsmasq command line.
func generateDnsmasqCommand(ifName, address string, cfg *resolvedDHCP) string {
	var args []string

	// Interface binding — port=0 disables DNS listener to avoid conflicts
	// when multiple dnsmasq instances run for different networks.
	args = append(args,
		fmt.Sprintf("--interface=%s", ifName),
		"--bind-interfaces",
		"--port=0",
		"--no-daemon",
		"--log-dhcp",
		"--no-resolv",
		fmt.Sprintf("--pid-file=/var/run/dnsmasq-%s.pid", ifName),
		fmt.Sprintf("--dhcp-leasefile=/var/lib/dnsmasq/dnsmasq-%s.leases", ifName),
	)

	// DHCP range
	if cfg.Range != nil {
		// Custom range — need subnet mask from address
		addrCIDR := address
		if !strings.Contains(addrCIDR, "/") {
			addrCIDR = addrCIDR + "/24"
		}
		_, ipNet, err := net.ParseCIDR(addrCIDR)
		if err == nil {
			mask := net.IP(ipNet.Mask)
			args = append(args, fmt.Sprintf("--dhcp-range=%s,%s,%s,%s",
				cfg.Range.Start, cfg.Range.End, mask, cfg.LeaseTime))
		}
	} else {
		// Auto-computed range
		rangeStr := computeDHCPRangeWithLease(address, cfg.LeaseTime)
		if rangeStr != "" {
			args = append(args, fmt.Sprintf("--dhcp-range=%s", rangeStr))
		}
	}

	// Static reservations
	for _, res := range cfg.Reservations {
		if res.Hostname != "" {
			args = append(args, fmt.Sprintf("--dhcp-host=%s,%s,%s", res.MAC, res.IP, res.Hostname))
		} else {
			args = append(args, fmt.Sprintf("--dhcp-host=%s,%s", res.MAC, res.IP))
		}
	}

	// DNS settings
	if cfg.DNS != nil {
		if len(cfg.DNS.Nameservers) > 0 {
			args = append(args, fmt.Sprintf("--dhcp-option=6,%s", strings.Join(cfg.DNS.Nameservers, ",")))
		}
		if len(cfg.DNS.SearchDomains) > 0 {
			args = append(args, fmt.Sprintf("--dhcp-option=119,%s", strings.Join(cfg.DNS.SearchDomains, ",")))
		}
		if cfg.DNS.LocalDomain != "" {
			args = append(args, fmt.Sprintf("--domain=%s", cfg.DNS.LocalDomain), "--expand-hosts")
		}
	}

	// DHCP options
	if cfg.Options != nil {
		if cfg.Options.Router != "" {
			args = append(args, fmt.Sprintf("--dhcp-option=3,%s", cfg.Options.Router))
		}
		if cfg.Options.MTU != nil {
			args = append(args, fmt.Sprintf("--dhcp-option=26,%d", *cfg.Options.MTU))
		}
		if len(cfg.Options.NTPServers) > 0 {
			args = append(args, fmt.Sprintf("--dhcp-option=42,%s", strings.Join(cfg.Options.NTPServers, ",")))
		}
		for _, custom := range cfg.Options.Custom {
			args = append(args, fmt.Sprintf("--dhcp-option=%s", custom))
		}
	}

	return "dnsmasq " + strings.Join(args, " ")
}

// dhcpConfigHash computes a SHA256 hash of the DHCP configuration for change detection.
func dhcpConfigHash(router *v1alpha1.VPCRouter) string {
	data, _ := json.Marshal(struct {
		Global   *v1alpha1.RouterDHCP   `json:"global,omitempty"`
		Networks []v1alpha1.RouterNetwork `json:"networks"`
	}{
		Global:   router.Spec.DHCP,
		Networks: router.Spec.Networks,
	})
	return fmt.Sprintf("%x", sha256.Sum256(data))
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
// Supports arbitrary prefix lengths, not just /24.
// For example:
//
//	"10.100.0.1/24" → "10.100.0.10,10.100.0.254,255.255.255.0,12h"
//	"10.200.0.1/20" → "10.200.0.10,10.200.15.254,255.255.240.0,12h"
func computeDHCPRange(address string) string {
	return computeDHCPRangeWithLease(address, "12h")
}

// broadcastIP returns the broadcast address for the given network.
func broadcastIP(ipNet *net.IPNet) net.IP {
	ip := make(net.IP, len(ipNet.IP))
	for i := range ip {
		ip[i] = ipNet.IP[i] | ^ipNet.Mask[i]
	}
	return ip
}

// resolveFastpathImage determines the fast-path container image.
// Priority: ROUTER_POD_FASTPATH_IMAGE env > default
func resolveFastpathImage() string {
	if img := os.Getenv(routerPodFastpathImgEnv); img != "" {
		return img
	}
	return defaultFastpathImage
}
