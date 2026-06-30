package service

import (
	"context"
	"time"

	"github.com/mobilefarm/af/scenarios/internal/domain"
)

// RunNowOutcome — результат ручного запуска одного сценария.
type RunNowOutcome struct {
	ScenarioID string   `json:"scenario_id"`
	Status     string   `json:"status"`
	StepsRun   []string `json:"steps_run,omitempty"`
	Error      string   `json:"error,omitempty"`
}

// ResetDayProgress сбрасывает дневной прогресс шагов (для «Запустить сейчас»).
func (s *ScenarioService) ResetDayProgress(ctx context.Context, serial, scenarioID string) error {
	state, _ := s.repo.GetState(ctx, serial, scenarioID)
	date := s.clock.Now().Format("2006-01-02")
	if state.Date != date {
		state = domain.DayState{Date: date, StepIdempotency: map[string]string{}}
	}
	state.StepsDoneToday = nil
	state.StepsFailedToday = nil
	state.StepRunning = ""
	state.StepIdempotency = map[string]string{}
	return s.repo.PutState(ctx, serial, scenarioID, state)
}

// NextStepRunNow — следующий шаг sequential без проверки at и steps_done_today.
func (s *ScenarioService) NextStepRunNow(ctx context.Context, serial, scenarioID string) (domain.StepDoc, bool, error) {
	doc, err := s.repo.ParseScenario(ctx, serial, scenarioID)
	if err != nil {
		return domain.StepDoc{}, false, err
	}
	state, _ := s.repo.GetState(ctx, serial, scenarioID)
	if state.StepRunning != "" {
		return domain.StepDoc{}, false, nil
	}
	if IsSequentialExecution(doc) {
		for i, step := range doc.Steps {
			if sequentialStepDueRunNow(step, i, doc.Steps, state) {
				return step, true, nil
			}
		}
		return domain.StepDoc{}, false, nil
	}
	for _, step := range doc.Steps {
		if !stepDoneToday(step.ID, state) {
			return step, true, nil
		}
	}
	return domain.StepDoc{}, false, nil
}

// RunNow запускает полную цепочку выбранных сценариев на одном телефоне (последовательно).
func (s *Scheduler) RunNow(ctx context.Context, serial string, scenarioIDs []string) []RunNowOutcome {
	out := make([]RunNowOutcome, 0, len(scenarioIDs))
	for _, id := range scenarioIDs {
		if id == "" {
			continue
		}
		out = append(out, s.runScenarioNow(ctx, serial, id))
	}
	return out
}

func (s *Scheduler) runScenarioNow(ctx context.Context, serial, scenarioID string) RunNowOutcome {
	result := RunNowOutcome{ScenarioID: scenarioID, Status: "completed"}
	doc, err := s.repo.ParseScenario(ctx, serial, scenarioID)
	if err != nil {
		result.Status = "failed"
		result.Error = err.Error()
		return result
	}
	now := s.clock.Now().In(time.FixedZone("MSK", 3*3600))
	if !inValidRange(now, doc.ValidFrom, doc.ValidUntil) {
		result.Status = "skipped"
		result.Error = "сценарий вне valid_from/valid_until"
		return result
	}
	if len(doc.Steps) == 0 {
		result.Status = "skipped"
		result.Error = "нет шагов"
		return result
	}
	if err := s.scenario.ResetDayProgress(ctx, serial, scenarioID); err != nil {
		result.Status = "failed"
		result.Error = err.Error()
		return result
	}
	_ = s.scenario.AppendLogEntry(ctx, serial, scenarioID, domain.LogEntry{
		TS:         now.Format(time.RFC3339),
		MSK:        FormatMSK(now),
		Serial:     serial,
		ScenarioID: scenarioID,
		Status:     "run_now_started",
		Action:     "scenario",
	})
	sequential := IsSequentialExecution(doc)
	const maxChain = 16
	failed := false
	for n := 0; n < maxChain; n++ {
		step, due, err := s.scenario.NextStepRunNow(ctx, serial, scenarioID)
		if err != nil {
			result.Status = "failed"
			result.Error = err.Error()
			return result
		}
		if !due {
			break
		}
		result.StepsRun = append(result.StepsRun, step.ID)
		cont := s.runStep(ctx, serial, scenarioID, step)
		state, _ := s.repo.GetState(ctx, serial, scenarioID)
		if stepFailedToday(step.ID, state) {
			failed = true
		}
		if !sequential || !cont {
			break
		}
	}
	if len(result.StepsRun) == 0 {
		result.Status = "skipped"
		result.Error = "ни один шаг не выполнен"
		return result
	}
	if failed {
		result.Status = "failed"
		result.Error = "цепочка завершилась с ошибкой (см. логи)"
	}
	_ = s.scenario.AppendLogEntry(ctx, serial, scenarioID, domain.LogEntry{
		TS:         s.clock.Now().Format(time.RFC3339),
		MSK:        FormatMSK(s.clock.Now()),
		Serial:     serial,
		ScenarioID: scenarioID,
		Status:     "run_now_finished",
		Action:     "scenario",
		Error:      result.Error,
	})
	return result
}
