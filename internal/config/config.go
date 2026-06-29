package config

import (
	"log/slog"
	"os"
	"time"
)

type Config struct {
	HTTPAddr              string
	GRPCAddr              string
	HealthAddr            string
	StorageMode           string
	LogLevel              slog.Level
	MinioEndpoint         string
	MinioAccessKey        string
	MinioSecretKey        string
	MinioBucket           string
	MinioPrefix           string
	MinioUseSSL           bool
	OrchestratorGRPCAddr  string
	OrchestratorHTTPAddr  string
	SchedulerInterval     time.Duration
	SchedulerEnabled      bool
	LLMProvider           string
	LLMAPIKey             string
	OpenAIAPIKey          string
	OpenAIBaseURL         string
	OpenAIModel           string
}

func Load() Config {
	return Config{
		HTTPAddr:             env("HTTP_ADDR", ":19095"),
		GRPCAddr:             env("GRPC_ADDR", ":50059"),
		HealthAddr:           env("HEALTH_ADDR", ":9099"),
		StorageMode:          env("STORAGE_MODE", "memory"),
		LogLevel:             parseLogLevel(env("LOG_LEVEL", "info")),
		MinioEndpoint:        env("MINIO_ENDPOINT", "localhost:9000"),
		MinioAccessKey:       env("MINIO_ACCESS_KEY", "minioadmin"),
		MinioSecretKey:       env("MINIO_SECRET_KEY", "minioadmin"),
		MinioBucket:          env("MINIO_BUCKET", "af-scenarios"),
		MinioPrefix:          env("MINIO_PREFIX", "scenarios"),
		MinioUseSSL:          envBool("MINIO_USE_SSL", false),
		OrchestratorGRPCAddr: env("ORCHESTRATOR_GRPC_ADDR", "localhost:50050"),
		OrchestratorHTTPAddr: env("ORCHESTRATOR_HTTP_ADDR", "http://127.0.0.1:9092"),
		SchedulerInterval:    envDuration("SCHEDULER_INTERVAL", time.Minute),
		SchedulerEnabled:     envBool("SCHEDULER_ENABLED", true),
		LLMProvider:          env("LLM_PROVIDER", "openai"),
		LLMAPIKey:            env("LLM_API_KEY", ""),
		OpenAIAPIKey:         env("OPENAI_API_KEY", ""),
		OpenAIBaseURL:        env("OPENAI_BASE_URL", "https://api.openai.com/v1"),
		OpenAIModel:          env("OPENAI_MODEL", "gpt-4o-mini"),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v == "1" || v == "true" || v == "yes"
}

func envDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}

func parseLogLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
