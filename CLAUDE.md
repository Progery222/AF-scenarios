# CLAUDE.md

Руководство для AI-агентов в репозитории **AF-scenarios** — планировщик сценариев платформы **AF**.

## Назначение

**AF-scenarios** — движок сценариев для Android-фермы:

1. Хранит сценарии в **MinIO** (по папке телефона)
2. Раз в минуту проверяет расписание и текущий шаг
3. Вызывает **phone-orchestrator** для исполнения (generic DSL)
4. Пишет **логи** и **state** в MinIO
5. Генерирует черновики сценариев через **LLM** (`GenerateFromPrompt`)

> **Главный инвариант:** scenarios **не тапает** и **не публикует** — только планирует и делегирует orchestrator.

## Стек

| Слой | Технология |
|------|------------|
| Язык | Go 1.22+ |
| API | gRPC `:50059`; HTTP только health/ready `:9099` |
| Storage | MinIO (сценарии, variables, state, logs) |
| Scheduler | goroutine, interval `1m` |
| LLM | OpenAI / Claude (как recovery-engine) |
| Observability | slog JSON |

## Место в платформе AF

```
MinIO scenarios/{serial}/{id}/
         │
AF-scenarios (scheduler)
         │ gRPC RunScenarioStep
         ▼
phone-orchestrator ──► executor / observer / content-distributor / video-generator
         │
Frontend /scenarios (через orchestrator proxy)
```

| Сервис | Порт (dev) | Роль |
|--------|------------|------|
| **AF-scenarios** | gRPC `:50059`, health `:9099` | Планировщик, CRUD, ИИ |
| phone-orchestrator | gRPC `:50050` | Исполнение шагов DSL |
| phone-observer | gRPC `:50053` | Скриншоты (через orchestrator) |
| video-generator | gRPC `:50056` | Видео из скринов |
| content-distributor | REST/NATS | Доставка на телефон |

## Архитектура (Hexagonal)

```
AF-scenarios/
├── cmd/server/main.go
├── internal/
│   ├── config/
│   ├── domain/          # Scenario, Step, Goal, ContentSources
│   ├── port/            # ScenarioStore, LogWriter, OrchestratorClient, LLM
│   ├── service/         # Scheduler, ScenarioCRUD, Generator
│   └── adapter/
│       ├── handler/     # gRPC + health
│       ├── repository/  # MinIO
│       └── client/      # orchestrator, llm
├── proto/               # af.scenarios.v1 (целевой контракт)
├── examples/
└── docs/
```

## Формат сценария

См. `TZ.md` и `examples/saxonov-tiktok-daily/scenario.yaml`.

Блоки: `goal`, `content_sources`, `schedule`, `steps`, отдельно `variables.yaml`.

## Прогрев

Правила из мануалов: `docs/warmup-manual.md`. Профили в `variables.yaml` → `warmup_profiles`.

## Git

GitHub Flow, Conventional Commits. Не пушить в main без PR.
