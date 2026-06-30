package service_test

import (
	"strings"
	"testing"
	"time"

	"github.com/mobilefarm/af/scenarios/internal/service"
)

func TestNormalizeScenarioYAML_LLMGarbage(t *testing.T) {
	now := time.Date(2026, 6, 29, 12, 0, 0, 0, time.FixedZone("MSK", 3*3600))
	raw := `id: tiktok_futbol
name: Test
serial: "10.16.182.227:5555"
valid_from: '2023-10-04T10:35:00+03:00'
valid_until: '2023-10-04T11:35:00+03:00'
schedule: daily_recurring
steps:
  - action: open_app
    params:
      app_package_name: com.ss.android.ugc.trill
      wait_time: 10s
  - action: scroll_down
    params:
      duration: 60s
  - action: search_in_browser_research
    params:
      query: футбол
      duration: 60s
  - action: watch_video
    params:
      video_duration: 60s
  - action: close_app
    params:
      app_package_name: com.ss.android.ugc.trill
`
	norm, warns := service.NormalizeScenarioYAML(raw, "10.16.182.227:5555", now)
	if !strings.Contains(norm, "2026-06-29") {
		t.Fatalf("expected 2026 valid_from, got:\n%s", norm)
	}
	if !strings.Contains(norm, "action: warmup_feed") {
		t.Fatalf("scroll_down not mapped:\n%s", norm)
	}
	if !strings.Contains(norm, "action: social_action") {
		t.Fatalf("search/watch not mapped to social_action:\n%s", norm)
	}
	if !strings.Contains(norm, "package: com.ss.android.ugc.trill") {
		t.Fatalf("package not normalized:\n%s", norm)
	}
	if len(warns) == 0 {
		t.Fatal("expected normalization warnings")
	}
}

func TestValidateScenarioYAML_AfterNormalize(t *testing.T) {
	now := time.Date(2026, 6, 29, 12, 0, 0, 0, time.FixedZone("MSK", 3*3600))
	raw := `id: x
serial: PHONE
valid_from: "2026-06-29T00:00:00+03:00"
valid_until: "2026-12-31T23:59:59+03:00"
schedule:
  type: daily_recurring
steps:
  - id: s1
    at: "10:35"
    action: open_app
    params:
      package: com.zhiliaoapp.musically
`
	vr := service.ValidateScenarioYAML(raw, "", "PHONE", now, true)
	if !vr.Valid {
		t.Fatalf("expected valid, errors=%v issues=%+v", vr.Errors, vr.StepIssues)
	}
	if !vr.RunnableByScheduler {
		t.Fatal("expected runnable")
	}
}
