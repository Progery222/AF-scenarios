package service

import (
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestRepairLLMYAML_BracketPairs(t *testing.T) {
	raw := `warmup_feed:
  like_probability: [0 0]
  scroll_interval_sec: [3 12]
`
	fixed := repairLLMYAML(raw)
	if strings.Contains(fixed, "[0 0]") {
		t.Fatalf("not repaired: %s", fixed)
	}
	var root map[string]any
	if err := yaml.Unmarshal([]byte(fixed), &root); err != nil {
		t.Fatal(err)
	}
}

func TestNormalizeScenarioYAML_MissingColonsInParams(t *testing.T) {
	now := time.Date(2026, 6, 30, 12, 0, 0, 0, time.FixedZone("MSK", 3*3600))
	raw := `id: tiktok-football-daily
name: TikTok Football
serial: PHONE
schedule:
  type: daily_recurring
  execution: sequential
steps:
  - id: open_tiktok
    at: "17:00"
    action: open_app
    params:
      package: com.zhiliaoapp.musically
  - id: warmup_feed
    after_previous true
    action: warmup_feed
    params:
      profile tiktok_daily
      phase pre_publish
      duration_sec 60
  - id: search_football
    after_previous: true
    action: social_action
    params:
      network tiktok
      behavior search-feed
      query Football
      count 12
Требования к YAML:
- at только в кавычках
`
	norm, warns := NormalizeScenarioYAML(raw, "PHONE", now)
	for _, w := range warns {
		if strings.Contains(w, "не парсится") {
			t.Fatalf("parse failed: %v\n%s", warns, norm)
		}
	}
	if strings.Contains(norm, "Требования к YAML") {
		t.Fatalf("prompt garbage not stripped:\n%s", norm)
	}
	if !strings.Contains(norm, "network: tiktok") {
		t.Fatalf("missing colon not fixed:\n%s", norm)
	}
}

func TestNormalizeScenarioYAML_AtWithBraces(t *testing.T) {
	now := time.Date(2026, 6, 30, 12, 0, 0, 0, time.FixedZone("MSK", 3*3600))
	raw := `id: test
serial: PHONE
steps:
  - id: open
    at: {16:20}
    action: open_app
`
	norm, warns := NormalizeScenarioYAML(raw, "PHONE", now)
	if strings.Contains(norm, "{16:20}") {
		t.Fatalf("braces not fixed:\n%s", norm)
	}
	for _, w := range warns {
		if strings.Contains(w, "не парсится") {
			t.Fatalf("parse failed: %v", warns)
		}
	}
	if !strings.Contains(norm, `at: "16:20"`) && !strings.Contains(norm, "at: 16:20") {
		t.Fatalf("expected at time in output:\n%s", norm)
	}
}
