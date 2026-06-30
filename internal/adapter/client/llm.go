package client

import (
	"strings"

	"github.com/mobilefarm/af/scenarios/internal/config"
	"github.com/mobilefarm/af/scenarios/internal/port"
)

// NewLLM создаёт клиент по LLM_PROVIDER (ollama | openai).
func NewLLM(cfg config.Config, log port.Logger) port.LLMClient {
	switch strings.ToLower(strings.TrimSpace(cfg.LLMProvider)) {
	case "ollama":
		ollama := cfg
		ollama.OpenAIBaseURL = strings.TrimRight(firstNonEmpty(cfg.OllamaURL, "http://host.docker.internal:11434"), "/") + "/v1"
		ollama.OpenAIModel = firstNonEmpty(cfg.OllamaModel, "qwen2.5:7b")
		ollama.LLMAPIKey = "ollama"
		return NewOpenAILLM(ollama, log)
	default:
		return NewOpenAILLM(cfg, log)
	}
}
