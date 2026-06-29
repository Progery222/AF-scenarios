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

func (s *StubOrchestrator) RunScenarioStep(ctx context.Context, serial, scenarioID, stepID, action string, params map[string]string) error {
	s.log.Info("RunScenarioStep (stub)", "serial", serial, "scenario", scenarioID, "step", stepID, "action", action)
	return nil
}
