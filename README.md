# AF-scenarios

Микросервис **планирования и хранения сценариев** для телефонов платформы AF: расписание, переменные прогрева, ИИ-генерация YAML, валидация и запуск шагов через orchestrator.

## Роль

| Зона | Да | Нет |
|------|----|-----|
| Хранение сценариев в MinIO | ✓ | |
| Планировщик (tick 1 мин) | ✓ | |
| Логи сценария в MinIO (JSONL) | ✓ | |
| ИИ-генерация + нормализация YAML | ✓ | |
| Валидация шагов и capabilities | ✓ | |
| Прямые tap/swipe на телефоне | | ✓ (orchestrator → executor) |
| FSM телефонов | | ✓ (orchestrator) |

## Поток

```
MinIO (scenario.yaml + variables.yaml + state.json)
         │
AF-scenarios (HTTP :19095, scheduler 1 min)
         │
         ├── HTTP ──► phone-orchestrator (run-step, open_app, warmup_feed, social_action)
         │                    └── behavior-engine (search-feed, feed, open-tab)
         │
         └── logs + state.json → MinIO
```

UI: **AF-frontend** `/scenarios` → orchestrator proxy → scenarios-engine.

## Структура в MinIO

```
scenarios/{serial}/{scenario_id}/
├── scenario.yaml      # шаги, расписание, timezone
├── variables.yaml     # warmup_profiles, warmup_feed
├── state.json         # steps_done_today, session_seed
└── logs/2026-06-30.jsonl
```

**Важно:** `id` в списке UI = имя папки в MinIO (не дублировать один `id` в YAML в разных папках).

## Типы шагов (action)

| action | Описание |
|--------|----------|
| `open_app` / `close_app` | Запуск / force-stop пакета |
| `warmup_feed` | Свайпы ленты через executor (orchestrator) |
| `social_action` | behavior-engine: `launch`, `feed`, `search-feed`, `open-tab`, … |
| `browser_research` | Chrome + Google, скрины |
| `wait` | Пауза |
| `create_video_from_screenshots` / `publish_content` | Видео-пайплайн |

### sequential-сценарии

- `schedule.execution: sequential`
- Только **первый** шаг с `at: "HH:MM"` (старт цепочки)
- Остальные шаги: `after_previous: true`, без `at`
- `close_app` с `after_failure: true` — cleanup после ошибки

### variables.yaml

Шаблон TikTok (автоподстановка при генерации):

```yaml
warmup_profiles:
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
```

При сохранении вызывается `MergeVariablesYAML` + `sanitizeVariablesMap` (исправляет типичные ошибки LLM).

## HTTP API (scenarios-engine)

| Метод | Путь | Описание |
|-------|------|----------|
| POST | `/scenarios/generate` | ИИ-черновик + `valid`, `warnings`, `normalized_scenario_yaml` |
| POST | `/scenarios/validate` | Валидация YAML |
| GET/PUT/DELETE | `/scenarios/{serial}/{id}` | CRUD сценария |
| GET | `/scenarios/{serial}/{id}/status` | Статус, шаги за день |
| GET | `/scenarios/{serial}/{id}/logs?date=` | JSONL логи |
| POST | `/scenarios/{serial}/{id}/trigger/{step_id}` | Ручной запуск шага |

Health: `:9099` (`/health`, `/ready`).

## Запуск локально

```powershell
$env:HTTP_ADDR=":19095"
$env:HEALTH_ADDR=":9099"
$env:MINIO_ENDPOINT="localhost:9000"
$env:ORCHESTRATOR_HTTP_ADDR="http://127.0.0.1:9092"
$env:SCHEDULER_INTERVAL="1m"
$env:LLM_PROVIDER="ollama"   # или openai
go run ./cmd/server
```

Docker (из корня монорепо):

```powershell
docker compose up -d --build scenarios-engine
```

## Деплой на сервер

```powershell
powershell -ExecutionPolicy Bypass -File scripts\deploy-scenarios-stack.ps1
```

Или только scenarios-engine + frontend (без полного robocopy orchestrator):

```powershell
# sync AF-scenarios без .git, docker build scenarios-engine
```

Сервер по умолчанию: `10.16.93.169`, путь `E:\Dev\AF`. UI: http://10.16.93.169:3030/scenarios

## Примеры

- [examples/saxonov-tiktok-daily/](examples/saxonov-tiktok-daily/) — браузер + TikTok
- Sequential TikTok 14:00: open → warmup → search-feed «футбол» `skip_launch: true` → close
- TikTok + Instagram: после TikTok — open IG → reels → warmup → search → close

Подробнее: [TZ.md](TZ.md), прогрев: [docs/warmup-manual.md](docs/warmup-manual.md).

## Порты (docker-compose)

| Сервис | HTTP | health |
|--------|------|--------|
| **scenarios-engine** | `19095` | `19098` → `:9099` |
| phone-orchestrator | `9092` → `:9090` | `9092` |
| behavior-engine | `19096` | `19097` |

## Переменные окружения

| Переменная | По умолчанию | Описание |
|------------|--------------|----------|
| `MINIO_BUCKET` | `af-scenarios` | Bucket сценариев |
| `MINIO_PREFIX` | `scenarios` | Префикс ключей |
| `ORCHESTRATOR_HTTP_ADDR` | `http://phone-orchestrator:9090` | run-step |
| `SCHEDULER_INTERVAL` | `1m` | Интервал планировщика |
| `LLM_PROVIDER` | `ollama` | `ollama` / `openai` |
| `OLLAMA_URL` / `OPENAI_API_KEY` | — | ИИ-генерация |

## Связанные сервисы

| Сервис | Роль |
|--------|------|
| [AF-phone-orchestrator](../AF-phone-orchestrator/README.md) | run-step, warmup_feed, apps |
| [AF-behavior-engine](../AF-behavior-engine/README.md) | social_action |
| [AF-frontend](../AF-frontend/README.md) | UI `/scenarios` |
