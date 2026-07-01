package service

import (
	"context"
	"encoding/json"
	"sync"
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

	runSerialMu sync.Mutex
	runSerial   map[string]struct{} // serial → run-now или цепочка в процессе
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
		runSerial: make(map[string]struct{}),
	}
}

func (s *Scheduler) tryAcquireSerial(serial string) bool {
	s.runSerialMu.Lock()
	defer s.runSerialMu.Unlock()
	if _, busy := s.runSerial[serial]; busy {
		return false
	}
	s.runSerial[serial] = struct{}{}
	return true
}

func (s *Scheduler) releaseSerial(serial string) {
	s.runSerialMu.Lock()
	delete(s.runSerial, serial)
	s.runSerialMu.Unlock()
}

func (s *Scheduler) serialBusy(serial string) bool {
	s.runSerialMu.Lock()
	defer s.runSerialMu.Unlock()
	_, busy := s.runSerial[serial]
	return busy
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
	for _, ref := range refs {
		if s.serialBusy(ref.Serial) {
			continue
		}
		active, _ := s.repo.GetActiveScenarioID(ctx, ref.Serial)
		if active != "" && ref.ScenarioID != active {
			continue
		}
		if active == "" {
			continue
		}
		doc, err := s.repo.ParseScenario(ctx, ref.Serial, ref.ScenarioID)
		if err != nil {
			continue
		}
		sequential := IsSequentialExecution(doc)
		const maxChain = 16
		for n := 0; n < maxChain; n++ {
			step, due, err := s.scenario.DueStep(ctx, ref.Serial, ref.ScenarioID)
			if err != nil || !due {
				break
			}
			cont := s.runStep(ctx, ref.Serial, ref.ScenarioID, step)
			if !sequential || !cont {
				break
			}
		}
	}
}

func (s *Scheduler) runStep(ctx context.Context, serial, scenarioID string, step domain.StepDoc) bool {
	now := s.clock.Now()
	files, _ := s.scenario.Get(ctx, serial, scenarioID)
	state, _ := s.repo.GetState(ctx, serial, scenarioID)
	_ = s.scenario.SetStepRunning(ctx, serial, scenarioID, step.ID)
	s.log.Info("запуск шага сценария", "serial", serial, "scenario", scenarioID, "step", step.ID, "action", step.Action)
	entry := domain.LogEntry{
		TS:         now.Format(time.RFC3339),
		MSK:        FormatMSK(now),
		Serial:     serial,
		ScenarioID: scenarioID,
		StepID:     step.ID,
		Status:     "started",
		Action:     step.Action,
	}
	_ = s.scenario.AppendLogEntry(ctx, serial, scenarioID, entry)

	params := step.Params
	if params == nil {
		params = map[string]string{}
	}
	result, err := s.orch.RunScenarioStep(ctx, port.RunStepInput{
		Serial:         serial,
		ScenarioID:     scenarioID,
		StepID:         step.ID,
		Action:         step.Action,
		Params:         params,
		Uses:           step.Uses,
		VariablesYAML:  files.VariablesYAML,
		ScenarioYAML:   files.ScenarioYAML,
		ScreenshotKeys: state.ScreenshotKeys,
		VideoOutputKey: state.VideoOutputKey,
	})
	if err != nil {
		s.log.Error("RunScenarioStep", "error", err, "step", step.ID)
		fail := entry
		fail.Status = "failed"
		fail.Error = err.Error()
		_ = s.scenario.AppendLogEntry(ctx, serial, scenarioID, fail)
		_ = s.scenario.MarkStepFailed(ctx, serial, scenarioID, step.ID)
		line, _ := json.Marshal(fail)
		_ = s.repo.AppendLog(ctx, serial, scenarioID, now.Format("2006-01-02"), append(line, '\n'))
		return true // в sequential цепочке продолжаем (например close_app)
	}
	_ = s.scenario.ApplyStepResult(ctx, serial, scenarioID, step.ID, result)
	done := entry
	done.Status = "completed"
	if result.Message != "" {
		done.Error = result.Message
	}
	line, _ := json.Marshal(done)
	_ = s.repo.AppendLog(ctx, serial, scenarioID, now.Format("2006-01-02"), append(line, '\n'))
	return true
}
