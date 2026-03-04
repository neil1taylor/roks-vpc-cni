package vpc

import (
	"regexp"
	"strings"
)

// Tag key constants for operator-managed VPC resources.
const (
	TagOperatorManaged = "roks-operator:true"
	TagClusterPrefix   = "roks-cluster:"
	TagResourcePrefix  = "roks-resource-type:"
	TagOwnerPrefix     = "roks-owner:"
)

// validTagValueRe matches IBM Cloud tag-safe characters: lowercase alphanumeric, dash, underscore, colon, dot.
var validTagValueRe = regexp.MustCompile(`[^a-z0-9\-_:.]`)

// BuildTags constructs the standard set of tags for an operator-managed VPC resource.
func BuildTags(clusterID, resourceType, ownerKind, ownerName string) []string {
	tags := []string{TagOperatorManaged}

	if clusterID != "" {
		tags = append(tags, TagClusterPrefix+sanitizeTagValue(clusterID))
	}
	if resourceType != "" {
		tags = append(tags, TagResourcePrefix+sanitizeTagValue(resourceType))
	}
	if ownerKind != "" && ownerName != "" {
		tags = append(tags, TagOwnerPrefix+sanitizeTagValue(ownerKind)+"/"+sanitizeTagValue(ownerName))
	}

	return tags
}

// sanitizeTagValue normalizes a value for IBM Cloud tag format compliance.
// IBM Cloud user tags must be lowercase, 1-128 characters, and may contain
// alphanumeric characters, dashes, underscores, colons, and dots.
func sanitizeTagValue(s string) string {
	s = strings.ToLower(s)
	s = validTagValueRe.ReplaceAllString(s, "-")
	if len(s) > 128 {
		s = s[:128]
	}
	return s
}
