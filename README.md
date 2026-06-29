# AF-scenarios

Микросервис **планирования и хранения сценариев** для телефонов платформы AF: расписание, переменные прогрева, сбор скринов через браузер, генерация сценариев через ИИ.

## Роль

| Зона | Да | Нет |
|------|----|-----|
| Хранение сценариев в MinIO | ✓ | |
| Планировщик (tick 1 мин) | ✓ | |
| Логи сценария в MinIO | ✓ | |
| ИИ-генерация черновиков | ✓ | |
| Прямые tap/swipe на телефоне | | ✓ (orchestrator → executor) |
| FSM телефонов | | ✓ (orchestrator) |

## Поток

```
MinIO (scenario.yaml + variables.yaml)
         │
AF-scenarios (scheduler) ──gRPC──► phone-orchestrator ──► executor / observer / content-distributor / video-generator
         │
         └── logs + state.json в MinIO
```

## Структура в MinIO

```
scenarios/{serial}/{scenario_id}/
├── scenario.yaml
├── variables.yaml
├── state.json
└── logs/2026-06-29.jsonl
```

## Запуск (локально, скелет)

```powershell
$env:GRPC_ADDR=":50059"
$env:HEALTH_ADDR=":9099"
$env:MINIO_ENDPOINT="localhost:9000"
$env:ORCHESTRATOR_GRPC_ADDR="localhost:50050"
$env:SCHEDULER_INTERVAL="1m"
go run ./cmd/server
```

## Пример сценария

[examples/saxonov-tiktok-daily/](examples/saxonov-tiktok-daily/) — браузерная сессия + TikTok-выкладка.

Подробнее: [TZ.md](TZ.md), прогрев: [docs/warmup-manual.md](docs/warmup-manual.md).

## Порты (dev)

| Сервис | gRPC | health |
|--------|------|--------|
| **AF-scenarios** | `:50059` | `:9099` |
| phone-orchestrator | `:50050` | `:9090` |
