package service

import (
	"strings"
	"time"

	"github.com/mobilefarm/af/scenarios/internal/domain"
)

// IsSequentialExecution — цепочка шагов: каждый следующий после завершения предыдущего.
func IsSequentialExecution(doc domain.ScenarioDoc) bool {
	ex := strings.ToLower(strings.TrimSpace(doc.Schedule.Execution))
	if ex == domain.ScheduleExecutionSequential || ex == "chain" {
		return true
	}
	if len(doc.Steps) < 2 {
		return false
	}
	for i := 1; i < len(doc.Steps); i++ {
		if doc.Steps[i].AfterPrevious {
			return true
		}
	}
	return false
}

func stepFailedToday(stepID string, state domain.DayState) bool {
	for _, id := range state.StepsFailedToday {
		if id == stepID {
			return true
		}
	}
	return false
}

// StepAtOrPast — для sequential: время старта первого шага уже наступило (по минутам).
func StepAtOrPast(step domain.StepDoc, now time.Time) bool {
	if step.At == "" {
		return false
	}
	at, err := time.Parse("15:04", step.At)
	if err != nil {
		return false
	}
	nowMin := now.Hour()*60 + now.Minute()
	return nowMin >= at.Hour()*60+at.Minute()
}

func sequentialStepDue(step domain.StepDoc, index int, steps []domain.StepDoc, now time.Time, state domain.DayState) bool {
	if stepDoneToday(step.ID, state) {
		return false
	}
	if state.StepRunning != "" && state.StepRunning != step.ID {
		return false
	}
	if index == 0 {
		return StepAtOrPast(step, now)
	}
	prev := steps[index-1]
	if !stepDoneToday(prev.ID, state) {
		return false
	}
	if step.RequiresPreviousSuccess && stepFailedToday(prev.ID, state) {
		return false
	}
	if stepFailedToday(prev.ID, state) && !step.AfterFailure && step.Action != "close_app" {
		// после ошибки пропускаем шаги, кроме явного after_failure и close_app (cleanup)
		return false
	}
	return true
}
