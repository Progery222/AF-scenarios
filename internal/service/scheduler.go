package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/mobilefarm/af/scenarios/internal/domain"
	"github.com/mobilefarm/af/scenarios/internal/port"
)

type Scheduler struct {
	repo     port.ScenarioRepository
	scenario *ScenarioService
	orch     port.OrchestratorClient
	clock    port.Clock
	interval time.Duration
	log      port.Logger
}

func NewScheduler(
	repo port.ScenarioRepository,
	scenario *ScenarioService,
	orch port.OrchestratorClient,
	clock port.Clock,
	interval time.Duration,
	log port.Logger,
) *Scheduler {
	return &Scheduler{
		repo: repo, scenario: scenario, orch: orch,
		clock: clock, interval: interval, log: log,
	}
}

func (s *Scheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	s.log.Info("планировщик сценариев запущен", "interval", s.interval.String())
	s.tick(ctx)
	for {
		select {
		case <-ctx.Done():
			s.log.Info("планировщик сценариев остановлен")
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

func (s *Scheduler) tick(ctx context.Context) {
	refs, err := s.repo.ListAllScenarioPaths(ctx)
	if err != nil {
		s.log.Error("list scenarios", "error", err)
		return
	}
	now := s.clock.Now()
	for _, ref := range refs {
		step, due, err := s.scenario.DueStep(ctx, ref.Serial, ref.ScenarioID)
		if err != nil || !due {
			continue
		}
		s.log.Info("запуск шага сценария", "serial", ref.Serial, "scenario", ref.ScenarioID, "step", step.ID, "action", step.Action)
		entry := domain.LogEntry{
			TS:         now.Format(time.RFC3339),
			MSK:        FormatMSK(now),
			Serial:     ref.Serial,
			ScenarioID: ref.ScenarioID,
			StepID:     step.ID,
			Status:     "started",
			Action:     step.Action,
		}
		_ = s.scenario.AppendLogEntry(ctx, ref.Serial, ref.ScenarioID, entry)

		params := step.Params
		if params == nil {
			params = map[string]string{}
		}
		if err := s.orch.RunScenarioStep(ctx, ref.Serial, ref.ScenarioID, step.ID, step.Action, params); err != nil {
			s.log.Error("RunScenarioStep", "error", err, "step", step.ID)
			fail := entry
			fail.Status = "failed"
			fail.Error = err.Error()
			_ = s.scenario.AppendLogEntry(ctx, ref.Serial, ref.ScenarioID, fail)
			continue
		}
		_ = s.scenario.MarkStepDone(ctx, ref.Serial, ref.ScenarioID, step.ID)
		done := entry
		done.Status = "completed"
		line, _ := json.Marshal(done)
		_ = s.repo.AppendLog(ctx, ref.Serial, ref.ScenarioID, now.Format("2006-01-02"), append(line, '\n'))
	}
}
