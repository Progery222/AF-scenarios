package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/mobilefarm/af/scenarios/internal/adapter/repository"
	"github.com/mobilefarm/af/scenarios/internal/port"
	"github.com/mobilefarm/af/scenarios/internal/service"
)

type fixedClock struct{ t time.Time }

func (f fixedClock) Now() time.Time { return f.t }

func TestScenarioService_PutGetList(t *testing.T) {
	store := repository.NewMemoryStore()
	clock := fixedClock{t: time.Date(2026, 6, 29, 12, 0, 0, 0, time.FixedZone("MSK", 3*3600))}
	svc := service.NewScenarioService(store, clock)
	ctx := context.Background()

	yaml := `id: test-1
name: Test
serial: PHONE-1
valid_from: "2026-06-29T00:00:00+03:00"
valid_until: "2026-07-29T23:59:59+03:00"
steps:
  - id: step1
    at: "12:00"
    action: wait
`
	if err := svc.Put(ctx, "PHONE-1", "test-1", service.ScenarioFiles{
		ScenarioYAML:  yaml,
		VariablesYAML: "x: 1\n",
	}); err != nil {
		t.Fatal(err)
	}
	files, err := svc.Get(ctx, "PHONE-1", "test-1")
	if err != nil {
		t.Fatal(err)
	}
	if files.ScenarioYAML == "" {
		t.Fatal("empty scenario")
	}
	items, err := svc.List(ctx, "PHONE-1")
	if err != nil || len(items) != 1 {
		t.Fatalf("list: %v len=%d", err, len(items))
	}
}

func TestScenarioService_StatusDueStep(t *testing.T) {
	store := repository.NewMemoryStore()
	clock := fixedClock{t: time.Date(2026, 6, 29, 12, 0, 0, 0, time.FixedZone("MSK", 3*3600))}
	svc := service.NewScenarioService(store, clock)
	ctx := context.Background()
	yaml := `id: daily
serial: PHONE-1
valid_from: "2026-06-01T00:00:00+03:00"
valid_until: "2026-12-31T23:59:59+03:00"
steps:
  - id: noon
    at: "12:00"
    action: open_app
`
	_ = svc.Put(ctx, "PHONE-1", "daily", service.ScenarioFiles{ScenarioYAML: yaml, VariablesYAML: ""})
	st, err := svc.Status(ctx, "PHONE-1", "daily")
	if err != nil {
		t.Fatal(err)
	}
	if !st.Active {
		t.Fatal("expected active")
	}
	step, due, err := svc.DueStep(ctx, "PHONE-1", "daily")
	if err != nil || !due || step.ID != "noon" {
		t.Fatalf("due step: due=%v step=%+v err=%v", due, step, err)
	}
}

func TestScheduler_RunsStep(t *testing.T) {
	store := repository.NewMemoryStore()
	clock := fixedClock{t: time.Date(2026, 6, 29, 12, 0, 0, 0, time.FixedZone("MSK", 3*3600))}
	svc := service.NewScenarioService(store, clock)
	orch := &recordingOrch{}
	ctx := context.Background()
	yaml := `id: daily
serial: PHONE-1
valid_from: "2026-06-01T00:00:00+03:00"
valid_until: "2026-12-31T23:59:59+03:00"
steps:
  - id: noon
    at: "12:00"
    action: wait
`
	_ = svc.Put(ctx, "PHONE-1", "daily", service.ScenarioFiles{ScenarioYAML: yaml})
	step, due, _ := svc.DueStep(ctx, "PHONE-1", "daily")
	if !due {
		t.Fatal("step not due")
	}
	_ = orch.RunScenarioStep(ctx, "PHONE-1", "daily", step.ID, step.Action, nil)
	_ = svc.MarkStepDone(ctx, "PHONE-1", "daily", step.ID)
	if len(orch.calls) != 1 {
		t.Fatalf("orch calls: %d", len(orch.calls))
	}
	step2, due2, _ := svc.DueStep(ctx, "PHONE-1", "daily")
	if due2 {
		t.Fatalf("step should be done, got %+v", step2)
	}
}

type recordingOrch struct {
	calls []string
}

func (r *recordingOrch) RunScenarioStep(_ context.Context, serial, scenarioID, stepID, action string, _ map[string]string) error {
	r.calls = append(r.calls, serial+"/"+scenarioID+"/"+stepID+"/"+action)
	return nil
}

var _ port.OrchestratorClient = (*recordingOrch)(nil)
