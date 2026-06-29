# AGENTS.md

Краткая шпаргалка для AI-агентов в **AF-scenarios**.

## Что это

Планировщик сценариев по телефонам: YAML в MinIO, tick раз в минуту, вызов orchestrator для шагов, ИИ-генерация черновиков.

## Ключевые файлы

| Файл | Зачем |
|------|-------|
| `TZ.md` | Полное ТЗ |
| `examples/saxonov-tiktok-daily/` | Референсный сценарий |
| `docs/warmup-manual.md` | Правила прогрева (из мануалов) |
| `internal/domain/scenario.go` | Модель сценария |
| `internal/service/scheduler.go` | Tick + выбор шага |
| `internal/port/ports.go` | Интерфейсы MinIO, orchestrator, LLM |
| `cmd/server/main.go` | Wire + запуск |

## Команды

```bash
make build
make run
make test
```

## Не делать

- Не вызывать executor / observer напрямую — только orchestrator
- Не хранить сценарии в Postgres (MVP: MinIO only)
- Не коммитить без явной просьбы

## Соседи

- Orchestrator gRPC `:50050` — `RunScenarioStep`
- MinIO `:9000` — bucket `af-scenarios` (или из env)
- video-generator `:50056` — через orchestrator (`create_video_from_screenshots`)
- Frontend — прокси через orchestrator, вкладка `/scenarios`
