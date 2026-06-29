package port

import (
	"context"
	"time"

	"github.com/mobilefarm/af/scenarios/internal/domain"
)

type ObjectStorage interface {
	Put(ctx context.Context, key string, data []byte, contentType string) error
	Get(ctx context.Context, key string) ([]byte, error)
	Delete(ctx context.Context, key string) error
	ListPrefix(ctx context.Context, prefix string) ([]string, error)
	Ping(ctx context.Context) error
}

type ScenarioRepository interface {
	List(ctx context.Context, serial string) ([]domain.ScenarioSummary, error)
	GetFiles(ctx context.Context, serial, scenarioID string) (scenarioYAML, variablesYAML []byte, err error)
	Put(ctx context.Context, serial, scenarioID string, scenarioYAML, variablesYAML []byte) error
	Delete(ctx context.Context, serial, scenarioID string) error
	GetState(ctx context.Context, serial, scenarioID string) (domain.DayState, error)
	PutState(ctx context.Context, serial, scenarioID string, state domain.DayState) error
	AppendLog(ctx context.Context, serial, scenarioID, date string, line []byte) error
	GetLogs(ctx context.Context, serial, scenarioID, date string) ([]byte, error)
	ListAllScenarioPaths(ctx context.Context) ([]domain.ScenarioRef, error)
	ParseScenario(ctx context.Context, serial, scenarioID string) (domain.ScenarioDoc, error)
}

type RunStepInput struct {
	Serial         string
	ScenarioID     string
	StepID         string
	Action         string
	Params         map[string]string
	Uses           string
	VariablesYAML  string
	ScenarioYAML   string
	ScreenshotKeys []string
	VideoOutputKey string
}

type RunStepResult struct {
	Status         string
	Message        string
	ScreenshotKeys []string
	VideoJobID     string
	VideoOutputKey string
}

type OrchestratorClient interface {
	RunScenarioStep(ctx context.Context, in RunStepInput) (RunStepResult, error)
}

type LLMClient interface {
	GenerateScenario(ctx context.Context, prompt string, serial string) (scenarioYAML, variablesYAML string, warnings []string, err error)
}

type Logger interface {
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
	Debug(msg string, args ...any)
}

type Clock interface {
	Now() time.Time
}

type RealClock struct{}

func (RealClock) Now() time.Time { return time.Now() }
