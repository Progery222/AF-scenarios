package service_test

import (
	"strings"
	"testing"

	"github.com/mobilefarm/af/scenarios/internal/service"
	"gopkg.in/yaml.v3"
)

func TestMergeVariablesYAML_FillsDefaults(t *testing.T) {
	merged, warns := service.MergeVariablesYAML(`warmup_feed:
  scroll_interval_sec: [3, 12]
`)
	if len(warns) > 0 && !strings.Contains(warns[0], "пуст") {
		// ok
	}
	for _, want := range []string{
		"warmup_profiles:",
		"likes_max:",
		"view_duration_sec:",
		"like_probability:",
		"saves: forbidden",
	} {
		if !strings.Contains(merged, want) {
			t.Fatalf("missing %q in:\n%s", want, merged)
		}
	}
}

func TestMergeVariablesYAML_EmptyUsesTemplate(t *testing.T) {
	merged, _ := service.MergeVariablesYAML("")
	if !strings.Contains(merged, "like_probability:") || !strings.Contains(merged, "likes_max:") {
		t.Fatalf("expected template:\n%s", merged)
	}
}

func TestMergeVariablesYAML_SanitizesMisplacedWarmupKeys(t *testing.T) {
	broken := `warmup_profiles:
  like_probability: [0, 0]
  scroll_interval_sec: [3, 12]
  tiktok_daily:
    pre_publish:
      duration_sec: [55, 65]
      likes_max: [0, 0]
      saves: forbidden
warmup_feed:
  scroll_interval_sec: [3, 12]
`
	merged, _ := service.MergeVariablesYAML(broken)
	if strings.Contains(merged, "warmup_profiles:\n  like_probability") {
		t.Fatalf("like_probability must not stay under warmup_profiles:\n%s", merged)
	}
	if !strings.Contains(merged, "tiktok_daily:") {
		t.Fatalf("profile missing:\n%s", merged)
	}
	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(merged), &parsed); err != nil {
		t.Fatalf("parse merged: %v\n%s", err, merged)
	}
}
