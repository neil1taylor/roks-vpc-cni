package vpc

import (
	"strings"
	"testing"
)

func TestBuildTags_AllFields(t *testing.T) {
	tags := BuildTags("cluster123", "subnet", "cudn", "localnet-1")

	expected := []string{
		"roks-operator:true",
		"roks-cluster:cluster123",
		"roks-resource-type:subnet",
		"roks-owner:cudn/localnet-1",
	}
	if len(tags) != len(expected) {
		t.Fatalf("expected %d tags, got %d: %v", len(expected), len(tags), tags)
	}
	for i, tag := range tags {
		if tag != expected[i] {
			t.Errorf("tag[%d]: expected %q, got %q", i, expected[i], tag)
		}
	}
}

func TestBuildTags_ClusterOnly(t *testing.T) {
	tags := BuildTags("cluster123", "", "", "")

	if len(tags) != 2 {
		t.Fatalf("expected 2 tags, got %d: %v", len(tags), tags)
	}
	if tags[0] != TagOperatorManaged {
		t.Errorf("first tag should be %q, got %q", TagOperatorManaged, tags[0])
	}
	if tags[1] != "roks-cluster:cluster123" {
		t.Errorf("second tag should be cluster tag, got %q", tags[1])
	}
}

func TestBuildTags_NoOwnerKindSkipsOwnerTag(t *testing.T) {
	tags := BuildTags("c1", "fip", "", "my-fip")

	for _, tag := range tags {
		if strings.HasPrefix(tag, TagOwnerPrefix) {
			t.Errorf("should not have owner tag when ownerKind is empty, got %q", tag)
		}
	}
}

func TestBuildTags_NoOwnerNameSkipsOwnerTag(t *testing.T) {
	tags := BuildTags("c1", "fip", "gateway", "")

	for _, tag := range tags {
		if strings.HasPrefix(tag, TagOwnerPrefix) {
			t.Errorf("should not have owner tag when ownerName is empty, got %q", tag)
		}
	}
}

func TestBuildTags_Empty(t *testing.T) {
	tags := BuildTags("", "", "", "")

	if len(tags) != 1 || tags[0] != TagOperatorManaged {
		t.Errorf("expected only operator tag, got %v", tags)
	}
}

func TestSanitizeTagValue_LowercaseAndSpecialChars(t *testing.T) {
	cases := []struct {
		input, expected string
	}{
		{"simple", "simple"},
		{"MixedCase", "mixedcase"},
		{"has spaces", "has-spaces"},
		{"special!@#$%chars", "special-----chars"},
		{"dots.are.ok", "dots.are.ok"},
		{"dashes-ok", "dashes-ok"},
		{"under_score", "under_score"},
		{"colons:ok", "colons:ok"},
	}

	for _, tc := range cases {
		got := sanitizeTagValue(tc.input)
		if got != tc.expected {
			t.Errorf("sanitizeTagValue(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestSanitizeTagValue_Truncation(t *testing.T) {
	long := strings.Repeat("a", 200)
	got := sanitizeTagValue(long)
	if len(got) != 128 {
		t.Errorf("expected length 128, got %d", len(got))
	}
}
