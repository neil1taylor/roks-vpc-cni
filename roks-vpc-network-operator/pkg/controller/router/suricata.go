package router

import (
	"fmt"
	"os"
	"strings"

	v1alpha1 "github.com/IBM/roks-vpc-network-operator/api/v1alpha1"
)

const (
	defaultSuricataImage = "docker.io/jasonish/suricata:7.0"
	suricataImageEnv     = "SURICATA_IMAGE"
)

// generateSuricataConfig builds a suricata.yaml configuration string.
// In IDS mode, it uses AF_PACKET for passive capture.
// In IPS mode, it uses NFQUEUE for inline inspection.
func generateSuricataConfig(ids *v1alpha1.RouterIDS, networks []v1alpha1.RouterNetwork) string {
	if ids == nil || !ids.Enabled {
		return ""
	}

	interfaces := suricataInterfaces(ids, networks)

	var sb strings.Builder
	sb.WriteString("%YAML 1.1\n---\n\n")

	// Vars
	sb.WriteString("vars:\n")
	sb.WriteString("  address-groups:\n")
	sb.WriteString("    HOME_NET: \"[192.168.0.0/16,10.0.0.0/8,172.16.0.0/12]\"\n")
	sb.WriteString("    EXTERNAL_NET: \"!$HOME_NET\"\n")
	sb.WriteString("  port-groups:\n")
	sb.WriteString("    HTTP_PORTS: \"80\"\n")
	sb.WriteString("    SSH_PORTS: \"22\"\n\n")

	// Default rule path
	sb.WriteString("default-rule-path: /var/lib/suricata/rules\n")
	sb.WriteString("rule-files:\n")
	sb.WriteString("  - suricata.rules\n")
	sb.WriteString("  - custom.rules\n\n")

	// Capture method
	if ids.Mode == "ips" {
		// NFQUEUE mode for inline IPS
		queueNum := int32(0)
		if ids.NFQueueNum != nil {
			queueNum = *ids.NFQueueNum
		}
		sb.WriteString("nfq:\n")
		sb.WriteString(fmt.Sprintf("  - queue-num: %d\n", queueNum))
		sb.WriteString("    fail-open: yes\n\n")
	} else {
		// AF_PACKET mode for passive IDS
		sb.WriteString("af-packet:\n")
		for _, iface := range interfaces {
			sb.WriteString(fmt.Sprintf("  - interface: %s\n", iface))
			sb.WriteString("    cluster-id: 99\n")
			sb.WriteString("    cluster-type: cluster_flow\n")
			sb.WriteString("    defrag: yes\n")
		}
		sb.WriteString("\n")
	}

	// Outputs — EVE JSON to stdout + optional syslog
	sb.WriteString("outputs:\n")
	sb.WriteString("  - eve-log:\n")
	sb.WriteString("      enabled: yes\n")
	sb.WriteString("      filetype: regular\n")
	sb.WriteString("      filename: /var/log/suricata/eve.json\n")
	sb.WriteString("      types:\n")
	sb.WriteString("        - alert:\n")
	sb.WriteString("            payload: yes\n")
	sb.WriteString("            payload-printable: yes\n")
	sb.WriteString("        - http\n")
	sb.WriteString("        - dns\n")
	sb.WriteString("        - tls\n")
	sb.WriteString("        - flow\n")
	sb.WriteString("        - stats:\n")
	sb.WriteString("            totals: yes\n")
	sb.WriteString("            threads: no\n")

	if ids.SyslogTarget != "" {
		sb.WriteString("  - eve-log:\n")
		sb.WriteString("      enabled: yes\n")
		sb.WriteString("      filetype: syslog\n")
		sb.WriteString(fmt.Sprintf("      identity: suricata-%s\n", ids.Mode))
		sb.WriteString("      facility: local5\n")
		sb.WriteString("      level: info\n")
		sb.WriteString("      types:\n")
		sb.WriteString("        - alert\n")
	}

	sb.WriteString("\n")

	// Logging
	sb.WriteString("logging:\n")
	sb.WriteString("  default-log-level: notice\n")
	sb.WriteString("  outputs:\n")
	sb.WriteString("    - console:\n")
	sb.WriteString("        enabled: yes\n")

	return sb.String()
}

// generateSuricataStartScript builds the startup script for the Suricata sidecar container.
// It updates ET Open rules, writes custom rules, starts Suricata, and tails EVE JSON to stdout.
func generateSuricataStartScript(ids *v1alpha1.RouterIDS) string {
	if ids == nil || !ids.Enabled {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("#!/bin/sh\nset -e\n\n")

	// Write suricata.yaml from env
	sb.WriteString("# Write Suricata config from env\n")
	sb.WriteString("mkdir -p /etc/suricata /var/log/suricata /var/lib/suricata/rules /var/run\n")
	sb.WriteString("echo \"$SURICATA_YAML\" > /etc/suricata/suricata.yaml\n\n")

	// Update ET Open rules
	sb.WriteString("# Update ET Open rules\n")
	sb.WriteString("suricata-update --no-test --no-reload 2>/dev/null || echo 'suricata-update not available, using bundled rules'\n\n")

	// Write custom rules from env
	sb.WriteString("# Write custom rules (if any)\n")
	sb.WriteString("if [ -n \"$CUSTOM_RULES\" ]; then\n")
	sb.WriteString("  echo \"$CUSTOM_RULES\" > /var/lib/suricata/rules/custom.rules\n")
	sb.WriteString("else\n")
	sb.WriteString("  touch /var/lib/suricata/rules/custom.rules\n")
	sb.WriteString("fi\n\n")

	// Start Suricata
	sb.WriteString("# Start Suricata in background\n")
	if ids.Mode == "ips" {
		queueNum := int32(0)
		if ids.NFQueueNum != nil {
			queueNum = *ids.NFQueueNum
		}
		sb.WriteString(fmt.Sprintf("suricata -c /etc/suricata/suricata.yaml -q %d -D\n", queueNum))
	} else {
		sb.WriteString("suricata -c /etc/suricata/suricata.yaml --af-packet -D\n")
	}
	sb.WriteString("\n")

	// Tail EVE JSON to stdout for log collection
	sb.WriteString("# Stream EVE JSON to stdout for log collection (IBM Cloud Logging on ROKS)\n")
	sb.WriteString("exec tail -F /var/log/suricata/eve.json\n")

	return sb.String()
}

// generateNFQueueRules generates nftables rules that redirect traffic through
// Suricata's NFQUEUE for inline IPS inspection. Returns empty string if IDS mode
// or IDS is disabled.
func generateNFQueueRules(ids *v1alpha1.RouterIDS, networks []v1alpha1.RouterNetwork) string {
	if ids == nil || !ids.Enabled || ids.Mode != "ips" {
		return ""
	}

	queueNum := int32(0)
	if ids.NFQueueNum != nil {
		queueNum = *ids.NFQueueNum
	}

	var sb strings.Builder
	sb.WriteString("table ip suricata {\n")
	sb.WriteString("  chain forward_ips {\n")
	sb.WriteString("    type filter hook forward priority -10; policy accept;\n")
	sb.WriteString("    ct state established,related counter accept\n")
	sb.WriteString(fmt.Sprintf("    counter queue num %d bypass\n", queueNum))
	sb.WriteString("  }\n")
	sb.WriteString("}\n")

	return sb.String()
}

// suricataInterfaces returns the list of network interface names that Suricata
// should monitor, based on the ids.Interfaces setting.
func suricataInterfaces(ids *v1alpha1.RouterIDS, networks []v1alpha1.RouterNetwork) []string {
	if ids == nil {
		return nil
	}

	switch ids.Interfaces {
	case "uplink":
		return []string{"uplink"}
	case "workload":
		ifaces := make([]string, 0, len(networks))
		for i := range networks {
			ifaces = append(ifaces, fmt.Sprintf("net%d", i))
		}
		return ifaces
	default: // "all" or empty
		ifaces := []string{"uplink"}
		for i := range networks {
			ifaces = append(ifaces, fmt.Sprintf("net%d", i))
		}
		return ifaces
	}
}

// resolveSuricataImage determines the Suricata container image, checking in order:
// ids.Image > SURICATA_IMAGE env > default
func resolveSuricataImage(router *v1alpha1.VPCRouter) string {
	if router.Spec.IDS != nil && router.Spec.IDS.Image != "" {
		return router.Spec.IDS.Image
	}
	if img := os.Getenv(suricataImageEnv); img != "" {
		return img
	}
	return defaultSuricataImage
}
