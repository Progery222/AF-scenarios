package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mobilefarm/af/scenarios/internal/domain"
	"github.com/mobilefarm/af/scenarios/internal/port"
	"gopkg.in/yaml.v3"
)

type ScenarioService struct {
	repo  port.ScenarioRepository
	clock port.Clock
	llm   port.LLMClient
}

func NewScenarioService(repo port.ScenarioRepository, clock port.Clock, llm port.LLMClient) *ScenarioService {
	return &ScenarioService{repo: repo, clock: clock, llm: llm}
}

type ScenarioFiles struct {
	ScenarioYAML  string `json:"scenario_yaml"`
	VariablesYAML string `json:"variables_yaml"`
}

type ScenarioStatus struct {
	Serial       string   `json:"serial"`
	ScenarioID   string   `json:"scenario_id"`
	Active       bool     `json:"active"`
	CurrentStep  string   `json:"current_step,omitempty"`
	NextStep     string   `json:"next_step,omitempty"`
	StepsDone    []string `json:"steps_done_today,omitempty"`
	CheckedAt    string   `json:"checked_at"`
	Timezone     string   `json:"timezone,omitempty"`
}

type ScenarioListResult struct {
	Serial           string                  `json:"serial"`
	ActiveScenarioID string                  `json:"active_scenario_id,omitempty"`
	Items            []domain.ScenarioSummary `json:"items"`
}

func (s *ScenarioService) List(ctx context.Context, serial string) (ScenarioListResult, error) {
	items, err := s.repo.List(ctx, serial)
	if err != nil {
		return ScenarioListResult{}, err
	}
	active, _ := s.repo.GetActiveScenarioID(ctx, serial)
	if active == "" && len(items) == 1 {
		_ = s.repo.SetActiveScenarioID(ctx, serial, items[0].ID)
		active = items[0].ID
	}
	for i := range items {
		items[i].IsActive = items[i].ID == active
	}
	return ScenarioListResult{Serial: serial, ActiveScenarioID: active, Items: items}, nil
}

func (s *ScenarioService) GetActive(ctx context.Context, serial string) (string, error) {
	return s.repo.GetActiveScenarioID(ctx, serial)
}

func (s *ScenarioService) SetActive(ctx context.Context, serial, scenarioID string) error {
	if err := s.repo.SetActiveScenarioID(ctx, serial, scenarioID); err != nil {
		return err
	}
	return nil
}

func (s *ScenarioService) Get(ctx context.Context, serial, scenarioID string) (ScenarioFiles, error) {
	sc, vars, err := s.repo.GetFiles(ctx, serial, scenarioID)
	if err != nil {
		return ScenarioFiles{}, err
	}
	return ScenarioFiles{ScenarioYAML: string(sc), VariablesYAML: string(vars)}, nil
}

func (s *ScenarioService) Put(ctx context.Context, serial, scenarioID string, files ScenarioFiles) error {
	norm, _ := NormalizeScenarioYAML(files.ScenarioYAML, serial, s.clock.Now())
	var doc domain.ScenarioDoc
	if err := yaml.Unmarshal([]byte(norm), &doc); err != nil {
		return fmt.Errorf("%w: %v", domain.ErrInvalidYAML, err)
	}
	if strings.TrimSpace(doc.ID) == "" {
		return domain.ErrMissingID
	}
	varYAML, _ := MergeVariablesYAML(files.VariablesYAML)
	if err := s.repo.Put(ctx, serial, scenarioID, []byte(norm), []byte(varYAML)); err != nil {
		return err
	}
	active, _ := s.repo.GetActiveScenarioID(ctx, serial)
	if active == "" {
		_ = s.repo.SetActiveScenarioID(ctx, serial, scenarioID)
	}
	return nil
}

func (s *ScenarioService) Validate(ctx context.Context, scenarioYAML, variablesYAML, serial string, normalize bool) ValidateResult {
	return ValidateScenarioYAML(scenarioYAML, variablesYAML, serial, s.clock.Now(), normalize)
}

func (s *ScenarioService) Delete(ctx context.Context, serial, scenarioID string) error {
	return s.repo.Delete(ctx, serial, scenarioID)
}

func (s *ScenarioService) GetLogs(ctx context.Context, serial, scenarioID, date string) (string, error) {
	data, err := s.repo.GetLogs(ctx, serial, scenarioID, date)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *ScenarioService) Status(ctx context.Context, serial, scenarioID string) (ScenarioStatus, error) {
	doc, err := s.repo.ParseScenario(ctx, serial, scenarioID)
	if err != nil {
		return ScenarioStatus{}, err
	}
	now := s.clock.Now()
	loc := time.FixedZone("MSK", 3*3600)
	if doc.Timezone == "Europe/Moscow" || doc.Timezone == "" {
		loc = time.FixedZone("MSK", 3*3600)
	}
	localNow := now.In(loc)
	state, _ := s.repo.GetState(ctx, serial, scenarioID)
	active := inValidRange(localNow, doc.ValidFrom, doc.ValidUntil)
	cur, next := resolveSteps(doc.Steps, localNow, state, IsSequentialExecution(doc))
	return ScenarioStatus{
		Serial:      serial,
		ScenarioID:  scenarioID,
		Active:      active,
		CurrentStep: cur,
		NextStep:    next,
		StepsDone:   state.StepsDoneToday,
		CheckedAt:   localNow.Format(time.RFC3339),
		Timezone:    doc.Timezone,
	}, nil
}

func inValidRange(now time.Time, from, until string) bool {
	if from != "" {
		t, err := time.Parse(time.RFC3339, from)
		if err == nil && now.Before(t) {
			return false
		}
	}
	if until != "" {
		t, err := time.Parse(time.RFC3339, until)
		if err == nil && now.After(t) {
			return false
		}
	}
	return true
}

func resolveSteps(steps []domain.StepDoc, now time.Time, state domain.DayState, sequential bool) (current, next string) {
	if sequential {
		for i, st := range steps {
			if stepDoneToday(st.ID, state) {
				continue
			}
			if sequentialStepDue(st, i, steps, now, state) {
				current = st.ID
				for j := i + 1; j < len(steps); j++ {
					if !stepDoneToday(steps[j].ID, state) {
						next = steps[j].ID
						break
					}
				}
				return current, next
			}
			return "", st.ID
		}
		return "", ""
	}
	today := now.Format("15:04")
	for _, st := range steps {
		if stepDoneToday(st.ID, state) {
			continue
		}
		if st.At != "" && st.At <= today {
			current = st.ID
		}
		if st.At != "" && st.At > today && next == "" {
			next = st.ID
		}
	}
	return current, next
}

func stepDoneToday(stepID string, state domain.DayState) bool {
	for _, id := range state.StepsDoneToday {
		if id == stepID {
			return true
		}
	}
	if state.StepIdempotency != nil {
		if _, ok := state.StepIdempotency[stepID]; ok {
			return true
		}
	}
	return false
}

func (s *ScenarioService) AppendLogEntry(ctx context.Context, serial, scenarioID string, entry domain.LogEntry) error {
	line, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	date := s.clock.Now().Format("2006-01-02")
	return s.repo.AppendLog(ctx, serial, scenarioID, date, append(line, '\n'))
}

func (s *ScenarioService) MarkStepDone(ctx context.Context, serial, scenarioID, stepID string) error {
	return s.markStepFinished(ctx, serial, scenarioID, stepID, false)
}

func (s *ScenarioService) MarkStepFailed(ctx context.Context, serial, scenarioID, stepID string) error {
	return s.markStepFinished(ctx, serial, scenarioID, stepID, true)
}

func (s *ScenarioService) SetStepRunning(ctx context.Context, serial, scenarioID, stepID string) error {
	state, _ := s.repo.GetState(ctx, serial, scenarioID)
	date := s.clock.Now().Format("2006-01-02")
	if state.Date != date {
		state = domain.DayState{Date: date, StepIdempotency: map[string]string{}}
	}
	state.StepRunning = stepID
	return s.repo.PutState(ctx, serial, scenarioID, state)
}

func (s *ScenarioService) markStepFinished(ctx context.Context, serial, scenarioID, stepID string, failed bool) error {
	state, _ := s.repo.GetState(ctx, serial, scenarioID)
	date := s.clock.Now().Format("2006-01-02")
	if state.Date != date {
		state = domain.DayState{Date: date, StepIdempotency: map[string]string{}}
	}
	if state.StepIdempotency == nil {
		state.StepIdempotency = map[string]string{}
	}
	state.StepIdempotency[stepID] = date
	state.StepRunning = ""
	found := false
	for _, id := range state.StepsDoneToday {
		if id == stepID {
			found = true
			break
		}
	}
	if !found {
		state.StepsDoneToday = append(state.StepsDoneToday, stepID)
	}
	if failed {
		ff := false
		for _, id := range state.StepsFailedToday {
			if id == stepID {
				ff = true
				break
			}
		}
		if !ff {
			state.StepsFailedToday = append(state.StepsFailedToday, stepID)
		}
	}
	return s.repo.PutState(ctx, serial, scenarioID, state)
}

func (s *ScenarioService) ApplyStepResult(ctx context.Context, serial, scenarioID, stepID string, result port.RunStepResult) error {
	if err := s.MarkStepDone(ctx, serial, scenarioID, stepID); err != nil {
		return err
	}
	if len(result.ScreenshotKeys) == 0 && result.VideoOutputKey == "" && result.VideoJobID == "" {
		return nil
	}
	state, _ := s.repo.GetState(ctx, serial, scenarioID)
	if len(result.ScreenshotKeys) > 0 {
		state.ScreenshotKeys = result.ScreenshotKeys
	}
	if result.VideoOutputKey != "" {
		state.VideoOutputKey = result.VideoOutputKey
	}
	if result.VideoJobID != "" {
		state.VideoJobID = result.VideoJobID
	}
	return s.repo.PutState(ctx, serial, scenarioID, state)
}

func FormatMSK(t time.Time) string {
	loc := time.FixedZone("MSK", 3*3600)
	return t.In(loc).Format("15:04:05")
}

func StepDueNow(step domain.StepDoc, now time.Time) bool {
	if step.At == "" {
		return false
	}
	at, err := time.Parse("15:04", step.At)
	if err != nil {
		return false
	}
	nowMin := now.Hour()*60 + now.Minute()
	stepMin := at.Hour()*60 + at.Minute()
	return nowMin == stepMin
}

func StepInWindow(step domain.StepDoc, now time.Time) bool {
	if step.Window == "" {
		return StepDueNow(step, now)
	}
	parts := splitWindow(step.Window)
	if len(parts) != 2 {
		return StepDueNow(step, now)
	}
	start, err1 := time.Parse("15:04", parts[0])
	end, err2 := time.Parse("15:04", parts[1])
	if err1 != nil || err2 != nil {
		return StepDueNow(step, now)
	}
	nowMin := now.Hour()*60 + now.Minute()
	return nowMin >= start.Hour()*60+start.Minute() && nowMin <= end.Hour()*60+end.Minute()
}

func splitWindow(w string) []string {
	for i := 0; i < len(w); i++ {
		if w[i] == '-' {
			return []string{w[:i], w[i+1:]}
		}
	}
	return nil
}

func (s *ScenarioService) DueStep(ctx context.Context, serial, scenarioID string) (domain.StepDoc, bool, error) {
	doc, err := s.repo.ParseScenario(ctx, serial, scenarioID)
	if err != nil {
		return domain.StepDoc{}, false, err
	}
	now := s.clock.Now().In(time.FixedZone("MSK", 3*3600))
	if !inValidRange(now, doc.ValidFrom, doc.ValidUntil) {
		return domain.StepDoc{}, false, nil
	}
	state, _ := s.repo.GetState(ctx, serial, scenarioID)
	if state.StepRunning != "" {
		return domain.StepDoc{}, false, nil
	}
	if IsSequentialExecution(doc) {
		for i, step := range doc.Steps {
			if sequentialStepDue(step, i, doc.Steps, now, state) {
				return step, true, nil
			}
		}
		return domain.StepDoc{}, false, nil
	}
	for _, step := range doc.Steps {
		if stepDoneToday(step.ID, state) {
			continue
		}
		if StepDueNow(step, now) || (step.Window != "" && StepInWindow(step, now) && step.At != "" && step.At <= now.Format("15:04")) {
			return step, true, nil
		}
	}
	return domain.StepDoc{}, false, nil
}

func (s *ScenarioService) GeneratePreview(ctx context.Context, prompt, serial string) (ScenarioFiles, []string, []StepIssue, error) {
	var rawFiles ScenarioFiles
	var baseWarnings []string
	if s.llm != nil {
		scYAML, varYAML, warnings, err := s.llm.GenerateScenario(ctx, prompt, serial)
		if err == nil && strings.TrimSpace(scYAML) != "" {
			rawFiles = ScenarioFiles{ScenarioYAML: scYAML, VariablesYAML: varYAML}
			baseWarnings = warnings
		} else if err != nil {
			files, w2, _ := s.generateTemplate(prompt, serial)
			return files, append([]string{err.Error()}, w2...), nil, nil
		}
	}
	if rawFiles.ScenarioYAML == "" {
		files, w2, _ := s.generateTemplate(prompt, serial)
		vr := ValidateScenarioYAML(files.ScenarioYAML, files.VariablesYAML, serial, s.clock.Now(), true)
		return ScenarioFiles{ScenarioYAML: vr.NormalizedScenarioYAML, VariablesYAML: files.VariablesYAML},
			append(w2, vr.Warnings...), vr.StepIssues, nil
	}
	vr := ValidateScenarioYAML(rawFiles.ScenarioYAML, rawFiles.VariablesYAML, serial, s.clock.Now(), true)
	allWarnings := append(baseWarnings, vr.Warnings...)
	for _, iss := range vr.StepIssues {
		if iss.Level == "warning" {
			allWarnings = append(allWarnings, fmt.Sprintf("%s (%s): %s", iss.StepID, iss.Action, iss.Message))
		}
	}
	scYAML := vr.NormalizedScenarioYAML
	if scYAML == "" {
		scYAML = rawFiles.ScenarioYAML
	}
	varYAML, mergeWarns := MergeVariablesYAML(rawFiles.VariablesYAML)
	allWarnings = append(allWarnings, mergeWarns...)
	return ScenarioFiles{ScenarioYAML: scYAML, VariablesYAML: varYAML}, allWarnings, vr.StepIssues, nil
}

func (s *ScenarioService) generateTemplate(prompt, serial string) (ScenarioFiles, []string, error) {
	warnings := []string{"ИИ-генератор недоступен; возвращён шаблон. Задайте LLM_PROVIDER=ollama или OPENAI_API_KEY."}
	files := ScenarioFiles{
		ScenarioYAML: fmt.Sprintf(`id: generated-%s
name: "Сгенерировано из промпта"
serial: %q
timezone: "Europe/Moscow"
valid_from: %q
valid_until: %q
schedule:
  type: daily_recurring
steps:
  - id: sample_step
    at: "12:00"
    action: wait
    params:
      note: %q
`, serial, serial, time.Now().Format(time.RFC3339), time.Now().AddDate(0, 1, 0).Format(time.RFC3339), prompt),
		VariablesYAML: DefaultTikTokVariablesYAML(),
	}
	return files, warnings, nil
}
