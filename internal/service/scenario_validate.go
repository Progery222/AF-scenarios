package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/mobilefarm/af/scenarios/internal/domain"
	"gopkg.in/yaml.v3"
)

type StepIssue struct {
	StepID  string `json:"step_id"`
	Action  string `json:"action"`
	Level   string `json:"level"`
	Message string `json:"message"`
}

type ValidateResult struct {
	Valid                  bool        `json:"valid"`
	Errors                 []string    `json:"errors"`
	Warnings               []string    `json:"warnings"`
	StepIssues             []StepIssue `json:"step_issues"`
	NormalizedScenarioYAML string      `json:"normalized_scenario_yaml,omitempty"`
	StepsCount             int         `json:"steps_count"`
	RunnableByScheduler    bool        `json:"runnable_by_scheduler"`
}

// ValidateScenarioYAML проверяет сценарий и опционально нормализует.
func ValidateScenarioYAML(scenarioYAML, variablesYAML string, serial string, now time.Time, normalize bool) ValidateResult {
	result := ValidateResult{Valid: true, RunnableByScheduler: true}

	raw := scenarioYAML
	if normalize {
		norm, normWarns := NormalizeScenarioYAML(scenarioYAML, serial, now)
		result.NormalizedScenarioYAML = norm
		result.Warnings = append(result.Warnings, normWarns...)
		raw = norm
	}

	var doc domain.ScenarioDoc
	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		result.Valid = false
		result.RunnableByScheduler = false
		result.Errors = append(result.Errors, "некорректный YAML: "+err.Error())
		return result
	}

	if strings.TrimSpace(doc.ID) == "" {
		result.Valid = false
		result.Errors = append(result.Errors, "отсутствует id")
	}
	if serial != "" && doc.Serial != "" && doc.Serial != serial {
		result.Warnings = append(result.Warnings, fmt.Sprintf("serial в YAML (%s) ≠ запрошенный (%s)", doc.Serial, serial))
	}

	loc := time.FixedZone("MSK", 3*3600)
	localNow := now.In(loc)
	if !inValidRange(localNow, doc.ValidFrom, doc.ValidUntil) {
		result.Warnings = append(result.Warnings, "сценарий неактивен по valid_from/valid_until на текущую дату")
		result.RunnableByScheduler = false
	}

	if doc.Schedule.Type == "" {
		result.Warnings = append(result.Warnings, "schedule.type не задан — планировщик может не сработать")
	}

	if len(doc.Steps) == 0 {
		result.Valid = false
		result.RunnableByScheduler = false
		result.Errors = append(result.Errors, "нет шагов")
	}
	result.StepsCount = len(doc.Steps)

	sequential := IsSequentialExecution(doc)

	for i, step := range doc.Steps {
		stepLabel := step.ID
		if stepLabel == "" {
			stepLabel = fmt.Sprintf("#%d", i+1)
		}
		if step.ID == "" {
			result.RunnableByScheduler = false
			result.StepIssues = append(result.StepIssues, StepIssue{
				StepID: stepLabel, Action: step.Action, Level: "error",
				Message: "отсутствует id",
			})
		}
		if step.At == "" && (!sequential || i == 0) {
			result.RunnableByScheduler = false
			result.StepIssues = append(result.StepIssues, StepIssue{
				StepID: stepLabel, Action: step.Action, Level: "error",
				Message: "отсутствует at — планировщик не запустит шаг",
			})
		}
		if sequential && i > 0 && !step.AfterPrevious {
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s: рекомендуется after_previous: true в sequential-режиме", stepLabel))
		}
		if step.Action == "" {
			result.Valid = false
			result.StepIssues = append(result.StepIssues, StepIssue{
				StepID: stepLabel, Level: "error", Message: "отсутствует action",
			})
			continue
		}
		if _, ok := validActions[step.Action]; !ok {
			result.Valid = false
			result.RunnableByScheduler = false
			result.StepIssues = append(result.StepIssues, StepIssue{
				StepID: stepLabel, Action: step.Action, Level: "error",
				Message: fmt.Sprintf("неизвестное действие: %s", step.Action),
			})
			continue
		}
		validateStepParams(step, &result)
	}

	if strings.TrimSpace(variablesYAML) != "" {
		var vars map[string]any
		if err := yaml.Unmarshal([]byte(variablesYAML), &vars); err != nil {
			result.Warnings = append(result.Warnings, "variables.yaml не парсится: "+err.Error())
		}
	}

	return result
}

func validateStepParams(step domain.StepDoc, result *ValidateResult) {
	p := step.Params
	if p == nil {
		p = map[string]string{}
	}
	switch step.Action {
	case "open_app", "close_app":
		if p["package"] == "" {
			result.StepIssues = append(result.StepIssues, StepIssue{
				StepID: step.ID, Action: step.Action, Level: "error",
				Message: "params.package обязателен",
			})
			result.Valid = false
		} else if !isKnownPackage(p["package"]) {
			result.StepIssues = append(result.StepIssues, StepIssue{
				StepID: step.ID, Action: step.Action, Level: "warning",
				Message: fmt.Sprintf("пакет %q — проверьте на устройстве", p["package"]),
			})
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s: нестандартный package %s", step.ID, p["package"]))
		}
	case "warmup_feed":
		if p["profile"] == "" && p["until"] == "" {
			result.StepIssues = append(result.StepIssues, StepIssue{
				StepID: step.ID, Action: step.Action, Level: "warning",
				Message: "нет profile/phase — будут дефолты из warmup_feed в variables",
			})
		}
	case "browser_research":
		if step.Uses == "" {
			result.StepIssues = append(result.StepIssues, StepIssue{
				StepID: step.ID, Action: step.Action, Level: "warning",
				Message: "рекомендуется uses: content_sources.browser_research",
			})
		}
	case "social_action":
		network := p["network"]
		if network == "" {
			network = "tiktok"
		}
		behavior := p["behavior"]
		if behavior == "" {
			behavior = "feed"
		}
		if _, ok := socialBehaviors[behavior]; !ok {
			result.StepIssues = append(result.StepIssues, StepIssue{
				StepID: step.ID, Action: step.Action, Level: "error",
				Message: fmt.Sprintf("неизвестный behavior: %s", behavior),
			})
			result.Valid = false
		}
		if behavior == "search-feed" && p["query"] == "" {
			result.StepIssues = append(result.StepIssues, StepIssue{
				StepID: step.ID, Action: step.Action, Level: "warning",
				Message: "search-feed без query",
			})
		}
	case "custom_execute":
		if p["steps_json"] == "" {
			result.StepIssues = append(result.StepIssues, StepIssue{
				StepID: step.ID, Action: step.Action, Level: "error",
				Message: "params.steps_json обязателен",
			})
			result.Valid = false
		}
	case "create_video_from_screenshots":
		if p["screenshot_prefix"] == "" && p["min_count"] == "" {
			result.StepIssues = append(result.StepIssues, StepIssue{
				StepID: step.ID, Action: step.Action, Level: "warning",
				Message: "укажите screenshot_prefix или min_count",
			})
		}
	}
}

func isKnownPackage(pkg string) bool {
	known := []string{
		"com.android.chrome", "com.zhiliaoapp.musically", "com.ss.android.ugc.trill",
		"com.ss.android.ugc.aweme", "com.google.android.gm", "com.android.settings",
	}
	for _, k := range known {
		if pkg == k {
			return true
		}
	}
	return strings.Contains(pkg, ".")
}
