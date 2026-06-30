package service

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mobilefarm/af/scenarios/internal/domain"
	"gopkg.in/yaml.v3"
)

var durationSuffixRe = regexp.MustCompile(`^(\d+)\s*s(?:ec)?$`)

// NormalizeScenarioYAML исправляет типичные ошибки LLM и приводит YAML к ожидаемой схеме.
func NormalizeScenarioYAML(raw, serial string, now time.Time) (string, []string) {
	warnings := []string{}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return raw, warnings
	}

	var root map[string]any
	if err := yaml.Unmarshal([]byte(raw), &root); err != nil {
		return raw, []string{"YAML не парсится: " + err.Error()}
	}

	if serial != "" {
		root["serial"] = serial
	}
	if root["timezone"] == nil || root["timezone"] == "" {
		root["timezone"] = "Europe/Moscow"
	}

	root["schedule"] = normalizeSchedule(root["schedule"], &warnings)
	normalizeValidity(root, now, &warnings)

	steps, _ := root["steps"].([]any)
	if len(steps) == 0 {
		warnings = append(warnings, "нет шагов steps[]")
	} else {
		sequential := isSequentialYAML(root, steps)
		root["steps"] = normalizeSteps(steps, root, now, sequential, &warnings)
		if sequential {
			sched, _ := root["schedule"].(map[string]any)
			if sched == nil {
				sched = map[string]any{"type": string(domainScheduleDaily)}
				root["schedule"] = sched
			}
			sched["execution"] = domain.ScheduleExecutionSequential
		}
	}

	normalizeContentSources(root, &warnings)

	out, err := yaml.Marshal(root)
	if err != nil {
		return raw, append(warnings, "не удалось сериализовать YAML: "+err.Error())
	}
	return string(out), warnings
}

func normalizeSchedule(v any, warnings *[]string) map[string]any {
	switch t := v.(type) {
	case map[string]any:
		if t["type"] == nil || t["type"] == "" {
			t["type"] = string(domainScheduleDaily)
			*warnings = append(*warnings, "schedule.type установлен в daily_recurring")
		}
		if ex, _ := t["execution"].(string); ex != "" {
			t["execution"] = strings.ToLower(strings.TrimSpace(ex))
		}
		return t
	case string:
		if t != "" {
			*warnings = append(*warnings, "schedule исправлен: daily_recurring → type: daily_recurring")
		}
		return map[string]any{"type": t}
	default:
		return map[string]any{"type": string(domainScheduleDaily)}
	}
}

const domainScheduleDaily = "daily_recurring"

func normalizeValidity(root map[string]any, now time.Time, warnings *[]string) {
	loc := time.FixedZone("MSK", 3*3600)
	today := now.In(loc)
	startOfDay := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, loc)
	endDefault := startOfDay.AddDate(0, 1, 0).Add(23*time.Hour + 59*time.Minute + 59*time.Second)

	fromStr, _ := root["valid_from"].(string)
	untilStr, _ := root["valid_until"].(string)

	fromT := parseRFC3339(fromStr)
	untilT := parseRFC3339(untilStr)

	if fromT.IsZero() || fromT.Before(startOfDay.AddDate(0, 0, -1)) {
		root["valid_from"] = startOfDay.Format(time.RFC3339)
		*warnings = append(*warnings, fmt.Sprintf("valid_from заменён на сегодня (%s)", root["valid_from"]))
	}
	if untilT.IsZero() || !untilT.After(today) {
		root["valid_until"] = endDefault.Format(time.RFC3339)
		*warnings = append(*warnings, fmt.Sprintf("valid_until продлён до %s", root["valid_until"]))
	}
}

func parseRFC3339(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

func normalizeContentSources(root map[string]any, warnings *[]string) {
	cs, ok := root["content_sources"].(map[string]any)
	if !ok {
		return
	}
	br, ok := cs["browser_research"]
	if !ok {
		return
	}
	switch v := br.(type) {
	case bool:
		if v {
			cs["browser_research"] = map[string]any{
				"enabled": true,
				"browser_package": "com.android.chrome",
				"search_keys": map[string]any{
					"rotation": "even",
					"items":    []any{"запрос"},
				},
			}
			*warnings = append(*warnings, "content_sources.browser_research: true → объект с enabled")
		} else {
			delete(cs, "browser_research")
		}
	}
}

func isSequentialYAML(root map[string]any, steps []any) bool {
	if sched, ok := root["schedule"].(map[string]any); ok {
		ex, _ := sched["execution"].(string)
		ex = strings.ToLower(strings.TrimSpace(ex))
		if ex == domain.ScheduleExecutionSequential || ex == "chain" {
			return true
		}
	}
	for i := 1; i < len(steps); i++ {
		m, ok := steps[i].(map[string]any)
		if !ok {
			continue
		}
		if ap, _ := m["after_previous"].(bool); ap {
			return true
		}
	}
	return false
}

func normalizeSteps(steps []any, root map[string]any, now time.Time, sequential bool, warnings *[]string) []any {
	loc := time.FixedZone("MSK", 3*3600)
	baseAt := inferBaseAt(root, now.In(loc))
	out := make([]any, 0, len(steps))

	for i, rawStep := range steps {
		m, ok := rawStep.(map[string]any)
		if !ok {
			continue
		}
		stepWarnings := []string{}
		normalizeStepParams(m, &stepWarnings)
		for _, w := range stepWarnings {
			*warnings = append(*warnings, fmt.Sprintf("шаг %d: %s", i+1, w))
		}

		action, _ := m["action"].(string)
		action = strings.TrimSpace(action)
		if alias, ok := actionAliases[action]; ok {
			*warnings = append(*warnings, fmt.Sprintf("шаг %d: action %q → %q", i+1, action, alias))
			mapActionAlias(m, action, alias)
			action = alias
		}
		m["action"] = action

		if id, _ := m["id"].(string); strings.TrimSpace(id) == "" {
			m["id"] = fmt.Sprintf("step_%d", i+1)
			*warnings = append(*warnings, fmt.Sprintf("шаг %d: добавлен id=%s", i+1, m["id"]))
		}

		if sequential {
			if i == 0 {
				if at, _ := m["at"].(string); strings.TrimSpace(at) == "" {
					m["at"] = minutesToAt(baseAt)
					*warnings = append(*warnings, fmt.Sprintf("шаг %d: добавлен at=%s (старт цепочки)", i+1, m["at"]))
				}
			} else {
				m["after_previous"] = true
				delete(m, "at")
				if action == "close_app" {
					m["after_failure"] = true
				}
			}
		} else if at, _ := m["at"].(string); strings.TrimSpace(at) == "" {
			m["at"] = minutesToAt(baseAt + i)
			*warnings = append(*warnings, fmt.Sprintf("шаг %d: добавлен at=%s", i+1, m["at"]))
		}
		out = append(out, m)
	}
	return out
}

func inferBaseAt(root map[string]any, now time.Time) int {
	if fromStr, _ := root["valid_from"].(string); fromStr != "" {
		if t := parseRFC3339(fromStr); !t.IsZero() {
			return t.Hour()*60 + t.Minute()
		}
	}
	return now.Hour()*60 + now.Minute()
}

func minutesToAt(minutes int) string {
	if minutes < 0 {
		minutes = 0
	}
	h := minutes / 60
	m := minutes % 60
	return fmt.Sprintf("%02d:%02d", h%24, m)
}

func normalizeStepParams(m map[string]any, warnings *[]string) {
	var params map[string]any
	switch p := m["params"].(type) {
	case map[string]any:
		params = p
	case map[string]string:
		params = make(map[string]any, len(p))
		for k, v := range p {
			params[k] = v
		}
	default:
		params = map[string]any{}
	}
	if params == nil {
		params = map[string]any{}
	}

	// app_package_name → package
	if pkg, ok := params["app_package_name"].(string); ok && pkg != "" {
		params["package"] = pkg
		delete(params, "app_package_name")
		*warnings = append(*warnings, "app_package_name → package")
	}
	if pkg, ok := params["package_name"].(string); ok && pkg != "" {
		if params["package"] == nil || params["package"] == "" {
			params["package"] = pkg
		}
		delete(params, "package_name")
	}

	normalizeDurationParam(params, "duration", "duration_sec", warnings)
	normalizeDurationParam(params, "wait_time", "duration_sec", warnings)
	normalizeDurationParam(params, "video_duration", "duration_sec", warnings)

	if query, ok := params["query"].(string); ok && query != "" {
		action, _ := m["action"].(string)
		if action == "social_action" || action == "search" || action == "search_in_app" {
			params["query"] = query
		}
	}
	if behavior, _ := params["behavior"].(string); behavior == "search-feed" {
		if params["skip_launch"] == nil || params["skip_launch"] == "" {
			params["skip_launch"] = "true"
		}
	}

	delete(params, "note")
	m["params"] = stringifyParams(params)
}

func normalizeDurationParam(params map[string]any, fromKey, toKey string, warnings *[]string) {
	v, ok := params[fromKey]
	if !ok {
		return
	}
	sec := parseDurationValue(v)
	if sec <= 0 {
		delete(params, fromKey)
		return
	}
	params[toKey] = strconv.Itoa(sec)
	if fromKey != toKey {
		delete(params, fromKey)
		*warnings = append(*warnings, fmt.Sprintf("%s → %s=%d", fromKey, toKey, sec))
	}
}

func parseDurationValue(v any) int {
	switch t := v.(type) {
	case int:
		return t
	case int64:
		return int(t)
	case float64:
		return int(t)
	case string:
		s := strings.TrimSpace(t)
		if m := durationSuffixRe.FindStringSubmatch(s); len(m) == 2 {
			n, _ := strconv.Atoi(m[1])
			return n
		}
		if n, err := strconv.Atoi(s); err == nil {
			return n
		}
	}
	return 0
}

func stringifyParams(params map[string]any) map[string]string {
	out := make(map[string]string, len(params))
	for k, v := range params {
		switch t := v.(type) {
		case string:
			out[k] = t
		case int:
			out[k] = strconv.Itoa(t)
		case int64:
			out[k] = strconv.Itoa(int(t))
		case float64:
			out[k] = strconv.Itoa(int(t))
		case bool:
			if t {
				out[k] = "true"
			} else {
				out[k] = "false"
			}
		default:
			out[k] = fmt.Sprint(v)
		}
	}
	return out
}

func mapActionAlias(m map[string]any, from, to string) {
	m["action"] = to
	params := ensureParamsMap(m)

	switch from {
	case "scroll_down", "scroll_up", "scroll_feed", "scroll":
		if params["profile"] == "" {
			params["profile"] = "tiktok_daily"
		}
		if params["phase"] == "" {
			params["phase"] = "pre_publish"
		}
	case "watch_video", "watch_feed":
		params["network"] = firstNonEmptyStr(params["network"], "tiktok")
		params["behavior"] = "feed"
		if params["count"] == "" && params["duration_sec"] != "" {
			if sec, _ := strconv.Atoi(params["duration_sec"]); sec > 0 {
				params["count"] = strconv.Itoa(max(1, sec/12))
			}
		}
	case "search_in_browser_research", "search_in_app", "search_tiktok", "search":
		params["network"] = firstNonEmptyStr(params["network"], "tiktok")
		params["behavior"] = "search-feed"
		if params["skip_launch"] == "" {
			params["skip_launch"] = "true"
		}
	case "comment":
		params["network"] = firstNonEmptyStr(params["network"], "tiktok")
		params["behavior"] = "comment"
	case "launch_app":
		// open_app — package уже в params
	case "stop_app":
		// close_app
	}
	m["params"] = params
}

func ensureParamsMap(m map[string]any) map[string]string {
	if p, ok := m["params"].(map[string]string); ok {
		return p
	}
	if p, ok := m["params"].(map[string]any); ok {
		return stringifyParams(p)
	}
	return map[string]string{}
}

func firstNonEmptyStr(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
