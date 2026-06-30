package service

import (
	"strings"

	"gopkg.in/yaml.v3"
)

// DefaultTikTokVariablesYAML — базовый variables.yaml для TikTok-сценариев.
func DefaultTikTokVariablesYAML() string {
	return `warmup_profiles:
  tiktok_daily:
    pre_publish:
      duration_sec: [55, 65]
      likes_max: [0, 0]
      saves: forbidden
warmup_feed:
  scroll_interval_sec: [3, 12]
  view_duration_sec: [5, 12]
  like_probability: [0, 0]
  swipe_pause_ms: [300, 800]
`
}

// MergeVariablesYAML накладывает пользовательский/LLM YAML на дефолты (дефолты заполняют пропуски).
func MergeVariablesYAML(generated string) (string, []string) {
	warnings := []string{}
	base := map[string]any{}
	if err := yaml.Unmarshal([]byte(DefaultTikTokVariablesYAML()), &base); err != nil {
		return strings.TrimSpace(generated), []string{"не удалось разобрать дефолтный variables.yaml"}
	}
	custom := map[string]any{}
	if strings.TrimSpace(generated) != "" {
		if err := yaml.Unmarshal([]byte(generated), &custom); err != nil {
			warnings = append(warnings, "variables.yaml от LLM не парсится — подставлен шаблон")
			out, err := yaml.Marshal(base)
			if err != nil {
				return DefaultTikTokVariablesYAML(), warnings
			}
			return string(out), warnings
		}
	} else {
		warnings = append(warnings, "variables.yaml пуст — подставлен шаблон TikTok")
	}
	merged := deepMergeMaps(base, custom)
	sanitizeVariablesMap(merged)
	out, err := yaml.Marshal(merged)
	if err != nil {
		return DefaultTikTokVariablesYAML(), warnings
	}
	return string(out), warnings
}

func deepMergeMaps(base, overlay map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range overlay {
		if existing, ok := out[k]; ok {
			baseMap, baseOK := asMapStringAny(existing)
			overMap, overOK := asMapStringAny(v)
			if baseOK && overOK {
				out[k] = deepMergeMaps(baseMap, overMap)
				continue
			}
		}
		out[k] = v
	}
	return out
}

func asMapStringAny(v any) (map[string]any, bool) {
	switch t := v.(type) {
	case map[string]any:
		return t, true
	case map[interface{}]interface{}:
		m := make(map[string]any, len(t))
		for k, val := range t {
			if s, ok := k.(string); ok {
				m[s] = val
			}
		}
		return m, len(m) > 0
	default:
		return nil, false
	}
}

// sanitizeVariablesMap убирает из warmup_profiles ключи не-профили (частая ошибка LLM)
// и переносит их в warmup_feed.
func sanitizeVariablesMap(m map[string]any) {
	wp, ok := asMapStringAny(m["warmup_profiles"])
	if !ok {
		return
	}
	wf, _ := asMapStringAny(m["warmup_feed"])
	if wf == nil {
		wf = map[string]any{}
		m["warmup_feed"] = wf
	}
	for k, v := range wp {
		if _, isProfile := asMapStringAny(v); isProfile {
			continue
		}
		delete(wp, k)
		if _, exists := wf[k]; !exists {
			wf[k] = v
		}
	}
}
