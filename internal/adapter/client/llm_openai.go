package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mobilefarm/af/scenarios/internal/config"
	"github.com/mobilefarm/af/scenarios/internal/port"
)

type OpenAILLM struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
	log     port.Logger
}

func NewOpenAILLM(cfg config.Config, log port.Logger) *OpenAILLM {
	base := strings.TrimRight(firstNonEmpty(cfg.OpenAIBaseURL, "https://api.openai.com/v1"), "/")
	model := firstNonEmpty(cfg.OpenAIModel, "gpt-4o-mini")
	key := firstNonEmpty(cfg.OpenAIAPIKey, cfg.LLMAPIKey)
	return &OpenAILLM{
		apiKey: key, baseURL: base, model: model,
		client: &http.Client{Timeout: 120 * time.Second},
		log:    log,
	}
}

func (c *OpenAILLM) GenerateScenario(ctx context.Context, prompt, serial string) (string, string, []string, error) {
	if strings.TrimSpace(c.apiKey) == "" {
		return "", "", []string{"LLM_API_KEY / OPENAI_API_KEY не задан — используйте шаблон"}, fmt.Errorf("llm api key missing")
	}
	system := `Ты генерируешь YAML для AF-scenarios (Android Farm).
Верни JSON: {"scenario_yaml":"...","variables_yaml":"..."} без markdown.
scenario_yaml: id, name, serial, timezone Europe/Moscow, valid_from/valid_until (RFC3339 +03:00), schedule daily_recurring, content_sources.browser_research при необходимости, steps с at/window/action/params.
variables.yaml: warmup_feed, warmup_profiles, browser_research с диапазонами [min,max].
Действия: wait, open_app, close_app, warmup_feed, browser_research, create_video_from_screenshots, publish_content.`
	user := fmt.Sprintf("serial: %q\nПромпт: %s", serial, prompt)
	body, _ := json.Marshal(map[string]any{
		"model": c.model,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
		"temperature": 0.4,
		"response_format": map[string]string{"type": "json_object"},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", "", nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return "", "", nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", "", nil, fmt.Errorf("openai HTTP %d: %s", resp.StatusCode, string(raw))
	}
	var envelope struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return "", "", nil, err
	}
	if len(envelope.Choices) == 0 {
		return "", "", nil, fmt.Errorf("пустой ответ LLM")
	}
	var out struct {
		ScenarioYAML  string `json:"scenario_yaml"`
		VariablesYAML string `json:"variables_yaml"`
	}
	if err := json.Unmarshal([]byte(envelope.Choices[0].Message.Content), &out); err != nil {
		return "", "", nil, fmt.Errorf("parse llm json: %w", err)
	}
	warnings := []string{"проверьте сгенерированный YAML перед сохранением"}
	return out.ScenarioYAML, out.VariablesYAML, warnings, nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

var _ port.LLMClient = (*OpenAILLM)(nil)
