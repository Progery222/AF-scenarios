package service_test

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/mobilefarm/af/scenarios/internal/adapter/repository"
	"github.com/mobilefarm/af/scenarios/internal/service"
)

func TestRunNow_BypassesTimeAndDone(t *testing.T) {
	store := repository.NewMemoryStore()
	// 11:30 — до at: "18:00"
	clock := fixedClock{t: time.Date(2026, 6, 30, 11, 30, 0, 0, time.FixedZone("MSK", 3*3600))}
	svc := service.NewScenarioService(store, clock, nil)
	orch := &recordingOrch{}
	sched := service.NewScheduler(store, svc, orch, clock, time.Minute, slog.New(slog.NewTextHandler(os.Stderr, nil)))
	ctx := context.Background()
	yaml := `id: chain
serial: PHONE-1
valid_from: "2026-06-01T00:00:00+03:00"
valid_until: "2026-12-31T23:59:59+03:00"
schedule:
  type: daily_recurring
  execution: sequential
steps:
  - id: open
    at: "18:00"
    action: open_app
  - id: warmup
    after_previous: true
    action: warmup_feed
`
	_ = svc.Put(ctx, "PHONE-1", "chain", service.ScenarioFiles{ScenarioYAML: yaml})
	_ = svc.MarkStepDone(ctx, "PHONE-1", "chain", "open")
	_ = svc.MarkStepDone(ctx, "PHONE-1", "chain", "warmup")

	_, due, _ := svc.DueStep(ctx, "PHONE-1", "chain")
	if due {
		t.Fatal("scheduler should not run before at and when done")
	}

	out := sched.RunNow(ctx, "PHONE-1", []string{"chain"})
	if len(out) != 1 {
		t.Fatalf("results: %d", len(out))
	}
	if out[0].Status != "completed" {
		t.Fatalf("status=%s err=%s steps=%v", out[0].Status, out[0].Error, out[0].StepsRun)
	}
	if len(out[0].StepsRun) != 2 {
		t.Fatalf("steps_run: %v", out[0].StepsRun)
	}
	if len(orch.calls) != 2 {
		t.Fatalf("orch calls: %v", orch.calls)
	}
}

func TestRunNow_MultipleScenarios(t *testing.T) {
	store := repository.NewMemoryStore()
	clock := fixedClock{t: time.Date(2026, 6, 30, 12, 0, 0, 0, time.FixedZone("MSK", 3*3600))}
	svc := service.NewScenarioService(store, clock, nil)
	orch := &recordingOrch{}
	sched := service.NewScheduler(store, svc, orch, clock, time.Minute, slog.New(slog.NewTextHandler(os.Stderr, nil)))
	ctx := context.Background()
	yamlA := `id: a
serial: PHONE-1
valid_from: "2026-06-01T00:00:00+03:00"
valid_until: "2026-12-31T23:59:59+03:00"
steps:
  - id: step_a
    at: "12:00"
    action: wait
`
	yamlB := `id: b
serial: PHONE-1
valid_from: "2026-06-01T00:00:00+03:00"
valid_until: "2026-12-31T23:59:59+03:00"
steps:
  - id: step_b
    at: "20:00"
    action: wait
`
	_ = svc.Put(ctx, "PHONE-1", "a", service.ScenarioFiles{ScenarioYAML: yamlA})
	_ = svc.Put(ctx, "PHONE-1", "b", service.ScenarioFiles{ScenarioYAML: yamlB})

	out := sched.RunNow(ctx, "PHONE-1", []string{"a", "b"})
	if len(out) != 2 {
		t.Fatalf("results: %d", len(out))
	}
	if out[0].ScenarioID != "a" || out[1].ScenarioID != "b" {
		t.Fatalf("order: %+v", out)
	}
	if len(orch.calls) != 2 {
		t.Fatalf("orch calls: %v", orch.calls)
	}
}
