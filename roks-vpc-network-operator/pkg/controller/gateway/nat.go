package gateway

import (
	"fmt"
	"sort"
	"strings"

	v1alpha1 "github.com/IBM/roks-vpc-network-operator/api/v1alpha1"
)

// GenerateNftablesConfig generates an nftables configuration string from the
// gateway's NAT specification. The vniIP parameter is the reserved IP on the
// uplink VNI, used as a fallback TranslatedAddress for SNAT rules that leave
// the TranslatedAddress field empty.
//
// The generated config follows this structure:
//
//	table ip nat {
//	  chain prerouting {
//	    type nat hook prerouting priority -100; policy accept;
//	    <DNAT rules sorted by priority>
//	  }
//	  chain postrouting {
//	    type nat hook postrouting priority 100; policy accept;
//	    <NoNAT accept rules sorted by priority>
//	    <SNAT rules sorted by priority>
//	  }
//	}
//
// Rules within each category are sorted by priority (lower number = higher
// priority = appears first). NoNAT accept rules always appear before SNAT
// rules in the postrouting chain to ensure traffic exemptions are evaluated
// before source translation.
func GenerateNftablesConfig(nat *v1alpha1.GatewayNAT, vniIP string) string {
	if nat == nil {
		return ""
	}

	hasDNAT := len(nat.DNAT) > 0
	hasPostrouting := len(nat.SNAT) > 0 || len(nat.NoNAT) > 0

	if !hasDNAT && !hasPostrouting {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("table ip nat {\n")

	// Prerouting chain (DNAT rules)
	if hasDNAT {
		sb.WriteString("  chain prerouting {\n")
		sb.WriteString("    type nat hook prerouting priority -100; policy accept;\n")

		// Sort DNAT rules by priority (lower first)
		dnatRules := make([]v1alpha1.DNATRule, len(nat.DNAT))
		copy(dnatRules, nat.DNAT)
		sort.Slice(dnatRules, func(i, j int) bool {
			return dnatRules[i].Priority < dnatRules[j].Priority
		})

		for _, rule := range dnatRules {
			protocol := rule.Protocol
			if protocol == "" {
				protocol = "tcp"
			}
			sb.WriteString(fmt.Sprintf("    %s dport %d dnat to %s:%d\n",
				protocol, rule.ExternalPort, rule.InternalAddress, rule.InternalPort))
		}

		sb.WriteString("  }\n")
	}

	// Postrouting chain (NoNAT + SNAT rules)
	if hasPostrouting {
		sb.WriteString("  chain postrouting {\n")
		sb.WriteString("    type nat hook postrouting priority 100; policy accept;\n")

		// NoNAT accept rules first (sorted by priority)
		if len(nat.NoNAT) > 0 {
			nonatRules := make([]v1alpha1.NoNATRule, len(nat.NoNAT))
			copy(nonatRules, nat.NoNAT)
			sort.Slice(nonatRules, func(i, j int) bool {
				return nonatRules[i].Priority < nonatRules[j].Priority
			})

			for _, rule := range nonatRules {
				sb.WriteString(fmt.Sprintf("    ip saddr %s ip daddr %s accept\n",
					rule.Source, rule.Destination))
			}
		}

		// SNAT rules after NoNAT (sorted by priority)
		if len(nat.SNAT) > 0 {
			snatRules := make([]v1alpha1.SNATRule, len(nat.SNAT))
			copy(snatRules, nat.SNAT)
			sort.Slice(snatRules, func(i, j int) bool {
				return snatRules[i].Priority < snatRules[j].Priority
			})

			for _, rule := range snatRules {
				translatedAddr := rule.TranslatedAddress
				if translatedAddr == "" {
					translatedAddr = vniIP
				}
				sb.WriteString(fmt.Sprintf("    ip saddr %s snat to %s\n",
					rule.Source, translatedAddr))
			}
		}

		sb.WriteString("  }\n")
	}

	sb.WriteString("}\n")
	return sb.String()
}
