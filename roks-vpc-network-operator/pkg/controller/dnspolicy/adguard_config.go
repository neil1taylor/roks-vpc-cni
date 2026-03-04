package dnspolicy

import (
	"fmt"
	"strings"

	v1alpha1 "github.com/IBM/roks-vpc-network-operator/api/v1alpha1"
)

// generateAdGuardConfig produces a YAML config string for AdGuard Home.
func generateAdGuardConfig(spec *v1alpha1.VPCDNSPolicySpec) string {
	var sb strings.Builder

	sb.WriteString("bind_host: 127.0.0.1\n")
	sb.WriteString("bind_port: 5353\n")

	sb.WriteString("http:\n")
	sb.WriteString("  address: 127.0.0.1:3000\n")

	sb.WriteString("dns:\n")
	sb.WriteString("  upstream_dns:\n")
	if spec.Upstream != nil && len(spec.Upstream.Servers) > 0 {
		for _, s := range spec.Upstream.Servers {
			sb.WriteString(fmt.Sprintf("    - %s\n", s.URL))
		}
	} else {
		sb.WriteString("    - 8.8.8.8\n")
		sb.WriteString("    - 1.1.1.1\n")
	}

	filteringEnabled := spec.Filtering != nil && spec.Filtering.Enabled
	sb.WriteString(fmt.Sprintf("  filtering_enabled: %t\n", filteringEnabled))

	if filteringEnabled && spec.Filtering != nil {
		if len(spec.Filtering.Allowlist) > 0 || len(spec.Filtering.Denylist) > 0 {
			sb.WriteString("  user_rules:\n")
			for _, allow := range spec.Filtering.Allowlist {
				sb.WriteString(fmt.Sprintf("    - '@@||%s^'\n", allow))
			}
			for _, deny := range spec.Filtering.Denylist {
				sb.WriteString(fmt.Sprintf("    - '||%s^'\n", deny))
			}
		}
	}

	if filteringEnabled && spec.Filtering != nil && len(spec.Filtering.Blocklists) > 0 {
		sb.WriteString("filters:\n")
		for i, bl := range spec.Filtering.Blocklists {
			sb.WriteString(fmt.Sprintf("  - enabled: true\n"))
			sb.WriteString(fmt.Sprintf("    url: %s\n", bl))
			sb.WriteString(fmt.Sprintf("    name: blocklist-%d\n", i+1))
			sb.WriteString(fmt.Sprintf("    id: %d\n", i+1))
		}
	}

	if spec.LocalDNS != nil && spec.LocalDNS.Enabled && spec.LocalDNS.Domain != "" {
		sb.WriteString(fmt.Sprintf("  local_domain_name: %s\n", spec.LocalDNS.Domain))
	}

	return sb.String()
}
