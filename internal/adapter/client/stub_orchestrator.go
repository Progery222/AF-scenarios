package client

import (
	"context"

	"github.com/mobilefarm/af/scenarios/internal/port"
)

// StubOrchestrator — заглушка до RunScenarioStep в orchestrator.
type StubOrchestrator struct {
	log port.Logger
}

func NewStubOrchestrator(log port.Logger) *StubOrchestrator {
	return &StubOrchestrator{log: log}
}

func (s *StubOrchestrator) RunScenarioStep(ctx context.Context, in port.RunStepInput) (port.RunStepResult, error) {
	s.log.Info("RunScenarioStep (stub)", "serial", in.Serial, "scenario", in.ScenarioID, "step", in.StepID, "action", in.Action)
	return port.RunStepResult{Status: "completed", Message: "stub"}, nil
}
