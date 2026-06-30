package service_test

import (
	"strings"
	"testing"
	"time"

	"github.com/mobilefarm/af/scenarios/internal/service"
)

func TestNormalizeScenarioYAML_SequentialStripsAt(t *testing.T) {
	now := time.Date(2026, 6, 30, 12, 0, 0, 0, time.FixedZone("MSK", 3*3600))
	raw := `id: tiktok
serial: PHONE
valid_from: "2026-06-30T00:00:00+03:00"
valid_until: "2026-12-31T23:59:59+03:00"
schedule:
  type: daily_recurring
  execution: sequential
steps:
  - id: open
    at: "12:00"
    action: open_app
    params:
      package: com.zhiliaoapp.musically
  - id: warmup
    at: "12:01"
    after_previous: true
    action: warmup_feed
  - id: close
    at: "12:02"
    after_previous: true
    action: close_app
    params:
      package: com.zhiliaoapp.musically
`
	norm, _ := service.NormalizeScenarioYAML(raw, "PHONE", now)
	if !strings.Contains(norm, "execution: sequential") {
		t.Fatalf("expected sequential execution:\n%s", norm)
	}
	if strings.Contains(norm, "at: \"12:01\"") || strings.Contains(norm, "at: \"12:02\"") {
		t.Fatalf("later steps should not keep at:\n%s", norm)
	}
	if !strings.Contains(norm, "after_previous: true") {
		t.Fatalf("expected after_previous:\n%s", norm)
	}
}
