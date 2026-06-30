package domain

import "time"

type ScenarioSummary struct {
	ID         string `json:"id"`                   // ключ хранения (папка в MinIO)
	YamlID     string `json:"yaml_id,omitempty"`    // id из scenario.yaml, если отличается
	Name       string `json:"name"`
	Serial     string `json:"serial"`
	ValidFrom  string `json:"valid_from,omitempty"`
	ValidUntil string `json:"valid_until,omitempty"`
	IsActive   bool   `json:"is_active,omitempty"`
}

type PhoneScenarioMeta struct {
	ActiveScenarioID string `json:"active_scenario_id"`
}

type ScenarioRef struct {
	Serial     string
	ScenarioID string
}

type ScenarioDoc struct {
	ID         string         `yaml:"id"`
	Name       string         `yaml:"name"`
	Serial     string         `yaml:"serial"`
	Timezone   string         `yaml:"timezone"`
	ValidFrom  string         `yaml:"valid_from"`
	ValidUntil string         `yaml:"valid_until"`
	Schedule   ScheduleConfig `yaml:"schedule"`
	Steps      []StepDoc      `yaml:"steps"`
}

type StepDoc struct {
	ID                    string            `yaml:"id"`
	At                    string            `yaml:"at"`
	Window                string            `yaml:"window"`
	AfterPrevious         bool              `yaml:"after_previous"`
	RequiresPreviousSuccess bool            `yaml:"requires_previous_success"`
	AfterFailure          bool              `yaml:"after_failure"`
	Action                string            `yaml:"action"`
	Uses                  string            `yaml:"uses"`
	Params                map[string]string `yaml:"params"`
}

type ScheduleType string

const (
	ScheduleDailyRecurring ScheduleType = "daily_recurring"
)

type Scenario struct {
	ID         string
	Name       string
	Serial     string
	Timezone   string
	ValidFrom  time.Time
	ValidUntil time.Time
	Schedule   ScheduleConfig
	Goal       Goal
	Content    ContentSources
	Steps      []StepDoc
}

type ScheduleConfig struct {
	Type      ScheduleType `yaml:"type"`
	Execution string       `yaml:"execution"` // scheduled (default) | sequential
}

const (
	ScheduleExecutionScheduled  = "scheduled"
	ScheduleExecutionSequential = "sequential"
)

type Goal struct {
	Platform string
	Theme    string
	Video    VideoGoal
	Hashtags HashtagGoal
}

type VideoGoal struct {
	Source         string
	MinScreenshots int
	Profile        string
}

type HashtagGoal struct {
	Count  [2]int
	Rotate bool
}

type ContentSources struct {
	BrowserResearch *BrowserResearch
}

type BrowserResearch struct {
	Enabled         bool
	Frequency       string
	Window          string
	BrowserPackage  string
	Google          GoogleConfig
	SearchKeys      RotationList
	TargetDomains   TargetDomains
	SessionUnique   SessionUniqueness
	Capture         CaptureConfig
}

type GoogleConfig struct {
	LocaleFrom string
}

type RotationList struct {
	Rotation string
	Items    []string
}

type TargetDomains struct {
	Rotation    string
	Match       string
	MaxAttempts int
	Items       []string
}

type SessionUniqueness struct {
	VaryKeyDaily          bool
	VaryScrollPattern     bool
	VaryCaptureOffsets    bool
}

type CaptureConfig struct {
	MinioPrefix string
	Moments     []string
}


type DayState struct {
	Date              string            `json:"date"`
	SearchKeyIndex    int               `json:"search_key_index"`
	SearchKeyUsed     string            `json:"search_key_used,omitempty"`
	TargetDomainsTried []string         `json:"target_domains_tried,omitempty"`
	SessionSeed       int64             `json:"session_seed"`
	ScreenshotKeys    []string          `json:"screenshot_keys,omitempty"`
	VideoOutputKey    string            `json:"video_output_key,omitempty"`
	VideoJobID        string            `json:"video_job_id,omitempty"`
	StepsDoneToday    []string          `json:"steps_done_today,omitempty"`
	StepsFailedToday  []string          `json:"steps_failed_today,omitempty"`
	StepRunning       string            `json:"step_running,omitempty"`
	StepIdempotency   map[string]string `json:"step_idempotency,omitempty"`
}

type LogEntry struct {
	TS          string `json:"ts"`
	MSK         string `json:"msk,omitempty"`
	Serial      string `json:"serial"`
	ScenarioID  string `json:"scenario_id"`
	StepID      string `json:"step_id,omitempty"`
	Status      string `json:"status"`
	Action      string `json:"action,omitempty"`
	Event       string `json:"event,omitempty"`
	Detail      string `json:"detail,omitempty"`
	DurationSec int    `json:"duration_sec,omitempty"`
	Error       string `json:"error,omitempty"`
}
