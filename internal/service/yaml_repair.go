package service

import (
	"regexp"
	"strings"
)

var (
	// [0 0] без запятой — частая ошибка LLM.
	yamlBracketPairRe = regexp.MustCompile(`\[\s*(-?\d+(?:\.\d+)?)\s+(-?\d+(?:\.\d+)?)\s*\]`)
	// at: {16:20} или at: {16:20 MSK}
	yamlAtBraceRe = regexp.MustCompile(`(?m)^(\s*at:\s*)\{(\d{1,2}:\d{2})(?:\s+[^}]+)?\}\s*$`)
	// network tiktok → network: tiktok (в params и на уровне шага)
	yamlParamMissingColonRe = regexp.MustCompile(
		`(?m)^(\s+)(network|behavior|query|package|count|duration_sec|like_probability|skip_launch|profile|phase|until|action|id|name)\s+([^:#\n][^\n]*)$`,
	)
	yamlStepFlagMissingColonRe = regexp.MustCompile(`(?m)^(\s+)(after_previous|after_failure)\s+(true|false)\s*$`)
	yamlNumberedPromptLineRe    = regexp.MustCompile(`^\d+\.\s+\w`)
)

// repairLLMYAML — правки до yaml.Unmarshal (типичные ошибки генерации).
func repairLLMYAML(raw string) string {
	s := strings.ReplaceAll(raw, "\r\n", "\n")
	s = stripPromptGarbageYAML(s)
	s = yamlBracketPairRe.ReplaceAllString(s, `[$1, $2]`)
	s = yamlAtBraceRe.ReplaceAllString(s, `$1"$2"`)
	s = yamlStepFlagMissingColonRe.ReplaceAllString(s, `$1$2: $3`)
	s = yamlParamMissingColonRe.ReplaceAllString(s, `$1$2: $3`)
	return strings.TrimSpace(s) + "\n"
}

// stripPromptGarbageYAML — LLM иногда копирует текст промпта в scenario_yaml.
func stripPromptGarbageYAML(s string) string {
	cutMarkers := []string{
		"\nТребования к YAML",
		"\nЛогирование:",
		"\nЦепочка шагов",
		"\nvariables.yaml полный",
		"\nЕжедневный sequential",
		"\nВ массивах обязательно",
	}
	for _, m := range cutMarkers {
		if i := strings.Index(s, m); i >= 0 {
			s = s[:i]
		}
	}
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		t := strings.TrimSpace(line)
		if t == "" {
			out = append(out, line)
			continue
		}
		if yamlNumberedPromptLineRe.MatchString(t) {
			continue
		}
		if strings.HasPrefix(t, "без ") && !strings.Contains(line, ":") {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}
