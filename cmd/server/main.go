package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/mobilefarm/af/scenarios/internal/adapter/client"
	"github.com/mobilefarm/af/scenarios/internal/adapter/handler"
	"github.com/mobilefarm/af/scenarios/internal/adapter/repository"
	"github.com/mobilefarm/af/scenarios/internal/adapter/storage"
	"github.com/mobilefarm/af/scenarios/internal/config"
	"github.com/mobilefarm/af/scenarios/internal/pkg/logging"
	"github.com/mobilefarm/af/scenarios/internal/port"
	"github.com/mobilefarm/af/scenarios/internal/service"
)

func main() {
	cfg := config.Load()
	logger := logging.New(cfg.LogLevel).With("service", "af-scenarios")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var obj port.ObjectStorage
	var repo port.ScenarioRepository

	if strings.EqualFold(cfg.StorageMode, "minio") {
		minioClient, err := storage.NewMinIO(ctx, cfg)
		if err != nil {
			logger.Error("minio init", "error", err)
			os.Exit(1)
		}
		obj = minioClient
		repo = repository.NewMinIOStore(minioClient, cfg.MinioPrefix)
		logger.Info("storage mode minio", "bucket", cfg.MinioBucket)
	} else {
		mem := repository.NewMemoryStore()
		obj = repository.NewMemoryObjectStorage(mem)
		repo = mem
		logger.Info("storage mode memory")
	}

	clock := port.RealClock{}
	var llm port.LLMClient
	if strings.EqualFold(cfg.LLMProvider, "ollama") || cfg.OpenAIAPIKey != "" || cfg.LLMAPIKey != "" {
		llm = client.NewLLM(cfg, logger)
		logger.Info("llm enabled", "provider", cfg.LLMProvider, "model", firstLLMModel(cfg))
	}
	scenarioSvc := service.NewScenarioService(repo, clock, llm)

	var orch port.OrchestratorClient = client.NewOrchestratorHTTP(cfg, logger)
	if strings.EqualFold(os.Getenv("ORCHESTRATOR_MODE"), "stub") {
		orch = client.NewStubOrchestrator(logger)
	}

	sched := service.NewScheduler(repo, scenarioSvc, orch, clock, cfg.SchedulerInterval, logger)
	if cfg.SchedulerEnabled {
		go sched.Run(ctx)
	}

	api := handler.NewAPI(scenarioSvc, repo, orch, sched)
	health := handler.NewHealth(obj)

	apiServer := &http.Server{Addr: cfg.HTTPAddr, Handler: api.Routes()}
	healthServer := &http.Server{Addr: cfg.HealthAddr, Handler: health.Routes()}

	go func() {
		logger.Info("http api started", "addr", cfg.HTTPAddr)
		if err := apiServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("http serve", "error", err)
		}
	}()
	go func() {
		logger.Info("health server started", "addr", cfg.HealthAddr)
		if err := healthServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("health serve", "error", err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = apiServer.Shutdown(shutdownCtx)
	_ = healthServer.Shutdown(shutdownCtx)
	logger.Info("shutdown complete")
}

func firstLLMModel(cfg config.Config) string {
	if strings.EqualFold(cfg.LLMProvider, "ollama") {
		if cfg.OllamaModel != "" {
			return cfg.OllamaModel
		}
		return "qwen2.5:7b"
	}
	if cfg.OpenAIModel != "" {
		return cfg.OpenAIModel
	}
	return "gpt-4o-mini"
}
