# ТЗ: AF-scenarios (scenarios-engine)

> Составлено: 2026-06-29 на основе обсуждения в чате.  
> Прогрев: мануалы `прогрев+выкладка`, `Мануал по прогреву аккаунтов` → [docs/warmup-manual.md](docs/warmup-manual.md).

## Назначение

**AF-scenarios** — микросервис планирования и хранения сценариев для Android-телефонов платформы AF.

1. Сценарии лежат в **MinIO** в папке конкретного телефона
2. Сценарий действует в окне **valid_from / valid_until** (ежедневное расписание шагов)
3. Фоновый **tick раз в минуту**: активен ли сценарий, какой шаг сейчас
4. **Лог** сценария в той же папке MinIO (отладка, передача другим сервисам)
5. **variables.yaml** — интервалы таймингов (не константы)
6. **ИИ-генерация** черновиков по промпту (`GenerateFromPrompt`)
7. Исполнение шагов — **только через phone-orchestrator**

**Не входит:** прямые ADB-жесты, FSM телефонов, публикация без orchestrator.

## Место в экосистеме AF

```
MinIO scenarios/{serial}/{scenario_id}/
         │
AF-scenarios (scheduler, 1 min)
         │ gRPC RunScenarioStep
         ▼
phone-orchestrator ──┬──► phone-action-executor
                     ├──► phone-observer (скрины, UI)
                     ├──► content-distributor
                     └──► video-generator

AF-frontend /scenarios ──HTTP──► orchestrator (proxy) ──► AF-scenarios
```

| Сервис | Транспорт | Роль |
|--------|-----------|------|
| **AF-scenarios** | gRPC **`:50059`**, health `:9099` | CRUD, scheduler, logs, ИИ |
| **phone-orchestrator** | gRPC `:50050` | Исполнение generic DSL |
| **MinIO** | S3 API | Единственное хранилище (MVP) |
| **video-generator** | gRPC `:50056` | Видео из скринов (через orchestrator) |
| **phone-observer** | gRPC `:50053` | Скриншоты браузера/экрана |
| **AF-frontend** | REST | Вкладка «Сценарии» в сайдбаре |

## Принятые решения

| Вопрос | Решение |
|--------|---------|
| Исполнение | Через **orchestrator** |
| Расписание | **Ежедневно** в `valid_from`/`valid_until`, шаги по `at` / `window` |
| Часовой пояс | **Europe/Moscow**, ISO с `+03:00` |
| Несколько сценариев на телефон | Да; при пересечении — **очередь** (не параллельно) |
| Формат файлов | `scenario.yaml` + `variables.yaml` |
| Логи | Ротация по дням: `logs/YYYY-MM-DD.jsonl` |
| Хранение | **Только MinIO** (без Postgres в MVP) |
| ИИ | Часть AF-scenarios, RPC `GenerateFromPrompt` |
| DSL | **Универсальный** (любое приложение через executor) |
| Браузер | **Chrome** (`com.android.chrome`) |
| Google | Локальный домен по **гео прокси телефона** |
| Переход на сайт | Только **органическая** ссылка из SERP |
| Скрины | За сессию; **каждый день уникальная сессия** |
| Видео для поста | Из скринов **того же дня** |
| Orchestrator RPC | Отдельная задача; MVP scenarios + **заглушка** orchestrator допустима |

## Структура MinIO

```
{bucket}/scenarios/{serial}/{scenario_id}/
├── scenario.yaml      # метаданные, goal, content_sources, steps
├── variables.yaml     # интервалы, warmup_profiles
├── state.json         # индексы ротации, шаги за день, session_seed
└── logs/
    └── 2026-06-29.jsonl
```

Префикс bucket: env `MINIO_BUCKET` (по умолчанию `af-scenarios`).

## Формат scenario.yaml

### Корневые поля

```yaml
id: saxonov-tiktok-daily
name: "Саксонов: браузер + TikTok"
serial: "R5CY331L8NF"
timezone: "Europe/Moscow"

valid_from: "2026-06-29T00:00:00+03:00"
valid_until: "2026-07-29T23:59:59+03:00"

schedule:
  type: daily_recurring
```

### goal — цель постинга

```yaml
goal:
  platform: tiktok
  theme: "Дмитрий Саксонов / blockchain sports"
  video:
    source: same_day_screenshots
    min_screenshots: 5
    profile: reels_1080x1920
  hashtags:
    count: [3, 7]
    rotate: true
```

### content_sources — откуда брать скрины

```yaml
content_sources:
  browser_research:
    enabled: true
    frequency: daily
    window: "10:00-11:30"
    browser_package: com.android.chrome

    google:
      locale_from: phone_proxy    # orchestrator: proxy → google.tld

    search_keys:
      rotation: even
      items:
        - "Дмитрий Саксонов"
        - "Дмитрий Саксонов биография"
        - "Дмитрий Саксонов кто это"
        - "Дмитрий Саксонов инстаграмм"
        - "Дмитрий Саксонов блокчен"
        - "Дмитрий Саксонов предприниматель"
        - "Дмитрий Саксонов blockchain sports"

    target_domains:
      rotation: random
      match: organic_serp_only
      max_attempts: 3
      items:
        - vc.ru
        - geekwire.com
        - kino.mail.ru
        - kinopoisk.ru
        - kino-teatr.ru

    session_uniqueness:
      vary_key_daily: true
      vary_scroll_pattern: true
      vary_capture_offsets: true

    capture:
      minio_prefix: "screenshots/{serial}/{scenario_id}/{date}/"
      moments: [after_serp, during_scroll, before_close]
```

**Процесс browser_research (orchestrator):**

1. Выбрать ключ (`even` rotation из `state.json`)
2. Открыть Chrome, Google с локалью по прокси телефона
3. Ввести ключ, просмотр выдачи 5–10 сек (из variables)
4. Клик **органической** ссылки на целевой домен из SERP
5. Если домен не совпал — до `max_attempts` с другим доменом из списка
6. Dwell на странице 60–120 сек, прокрутка, скрины в MinIO
7. Закрыть браузер

### steps — расписание дня

```yaml
steps:
  - id: browser_research_session
    at: "10:00"
    window: "10:00-11:30"
    action: browser_research
    uses: content_sources.browser_research

  - id: build_video
    at: "17:50"
    action: create_video_from_screenshots
    params:
      screenshot_prefix: "screenshots/{serial}/{scenario_id}/{today}/"
      min_count: 5

  - id: open_tiktok
    at: "17:55"
    action: open_app
    params: { package: com.zhiliaoapp.musically }

  - id: warmup_pre
    at: "17:56"
    action: warmup_feed
    params:
      profile: tiktok_daily
      phase: pre_publish
      until: "18:10"

  - id: post_video
    at: "18:10"
    action: publish_content
    params:
      from_step: build_video
      platform: tiktok

  - id: warmup_post
    at: "18:11"
    action: warmup_feed
    params:
      profile: tiktok_daily
      phase: post_publish
      until: "18:25"

  - id: close_tiktok
    at: "18:25"
    action: close_app
    params: { package: com.zhiliaoapp.musically }
```

## Формат variables.yaml

Все тайминги — **диапазоны** `[min, max]`; сервис/orchestrator выбирает случайное значение.

```yaml
browser_research:
  search_results_view_sec: [5, 10]
  page_load_timeout_sec: [15, 30]
  page_dwell_sec: [60, 120]
  scroll_interval_sec: [8, 20]
  scroll_count: [3, 8]
  capture_interval_sec: [12, 25]

warmup_profiles:
  tiktok_daily:
    pre_publish:
      mode: recommendations_feed
      duration_sec: [300, 600]
      likes_max: [0, 2]
      saves: forbidden
    post_publish:
      duration_sec: [300, 600]
      likes_max: [1, 3]
      saves: forbidden

warmup_feed:
  scroll_interval_sec: [3, 12]
  view_duration_sec: [5, 25]
  like_probability: [0.05, 0.12]
  comment_probability: [0, 0.02]
  comment_cooldown_sec: [120, 300]
```

## state.json

```json
{
  "date": "2026-06-29",
  "search_key_index": 3,
  "search_key_used": "Дмитрий Саксонов блокчен",
  "target_domains_tried": ["vc.ru"],
  "session_seed": 847291,
  "screenshot_keys": [],
  "steps_done_today": ["browser_research_session"],
  "step_idempotency": {
    "browser_research_session": "2026-06-29"
  }
}
```

`session_seed` + дата → уникальные интервалы прокрутки и моменты скринов каждый день.

## Лог (JSONL)

```json
{"ts":"2026-06-29T10:15:22+03:00","msk":"10:15:22","serial":"R5CY...","scenario_id":"saxonov-tiktok-daily","step_id":"browser_research_session","status":"started","action":"browser_research"}
{"ts":"2026-06-29T10:17:05+03:00","msk":"10:17:05","status":"completed","duration_sec":103,"vars_used":{"page_dwell_sec":87},"screenshot_count":6}
```

## Планировщик

- Интервал: **1 минута** (`SCHEDULER_INTERVAL`, default `1m`)
- На каждом tick:
  1. List `scenarios/*/*/scenario.yaml`
  2. Фильтр: сегодня ∈ `[valid_from, valid_until]`
  3. Для каждого serial — **очередь** (один активный сценарий)
  4. Определить шаг по `at` / `window` и `state.json`
  5. Если шаг не выполнен сегодня → `orchestrator.RunScenarioStep`
  6. Append log, update `state.json`

## gRPC API (`af.scenarios.v1.ScenariosService`)

| RPC | Описание |
|-----|----------|
| `ListScenarios` | По `serial` или все |
| `GetScenario` | `scenario.yaml` + `variables.yaml` |
| `PutScenario` | Создать/обновить в MinIO |
| `DeleteScenario` | Удалить папку |
| `GetScenarioStatus` | Текущий шаг, valid, last_tick |
| `GetScenarioLogs` | `logs/{date}.jsonl` |
| `GenerateFromPrompt` | ИИ → preview YAML |
| `TriggerStep` | Ручной запуск (dev) |

HTTP: только `/health`, `/ready` на `:9099`.

### GenerateFromPrompt

```
Request: { serial, prompt, platform?, save: bool }
Response: { scenario_yaml, variables_yaml, warnings[] }
```

Системный промпт: схема DSL, правило диапазонов в variables, timezone MSK, ссылка на warmup-manual.

## Generic DSL — действия (orchestrator)

| action | Описание |
|--------|----------|
| `open_app` / `close_app` | Запуск/закрытие по package |
| `browser_research` | Chrome + Google + SERP + dwell + скрины |
| `create_video_from_screenshots` | video-generator |
| `warmup_feed` | Прогрев ленты по профилю из variables |
| `publish_content` | content-distributor + executor UI |
| `wait` | Пауза |
| `custom_execute` | Batch executor actions |

Orchestrator проверяет FSM (`StateWorking`) перед шагом.

## Google locale от прокси

```
orchestrator.GetPhone(serial).proxy_country →
  RU → google.ru
  US → google.com
  DE → google.de
  ...
```

Если прокси не задан — `google.com` + warning в лог.

## Фронтенд (AF-frontend)

- Маршрут `/scenarios`, пункт в левом сайдбаре
- MVP: список по телефону, статус, редактор YAML, ИИ-промпт, просмотр логов
- API через orchestrator proxy (не напрямую в scenarios)

## Переменные окружения

| Переменная | По умолчанию | Описание |
|------------|--------------|----------|
| `GRPC_ADDR` | `:50059` | gRPC API |
| `HEALTH_ADDR` | `:9099` | health/ready |
| `MINIO_ENDPOINT` | `localhost:9000` | MinIO |
| `MINIO_BUCKET` | `af-scenarios` | Bucket |
| `ORCHESTRATOR_GRPC_ADDR` | `localhost:50050` | Клиент orchestrator |
| `SCHEDULER_INTERVAL` | `1m` | Tick планировщика |
| `LLM_PROVIDER` | `openai` | ИИ-генератор |
| `LLM_API_KEY` | — | Ключ API |

## Этапы реализации

| # | Этап | Результат |
|---|------|-----------|
| 1 | Bootstrap + TZ + примеры | ✓ этот репозиторий |
| 2 | Domain + MinIO CRUD + proto | CRUD сценариев |
| 3 | Scheduler + state + logs | Фоновый tick |
| 4 | Orchestrator `RunScenarioStep` | 2–3 action |
| 5 | `browser_research` + capture | Скрины в MinIO |
| 6 | `create_video` + `publish_content` | E2E день |
| 7 | `GenerateFromPrompt` | ИИ |
| 8 | Frontend `/scenarios` | UI |

## Итог

**AF-scenarios:** «Для телефона #7 сегодня 18:10 — шаг post_video»  
**Orchestrator:** «Скачал видео, открыл TikTok, опубликовал, прогрел ленту»  
**MinIO:** сценарий, variables, state, logs — всё в одной папке телефона.
