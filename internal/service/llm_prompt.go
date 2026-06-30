package service

import (
	"fmt"
	"time"
)

// BuildLLMSystemPrompt — system prompt с актуальной датой и few-shot.
func BuildLLMSystemPrompt(now time.Time) string {
	loc := time.FixedZone("MSK", 3*3600)
	today := now.In(loc)
	validFrom := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, loc).Format(time.RFC3339)
	validUntil := time.Date(today.Year(), today.Month(), today.Day(), 23, 59, 59, 0, loc).AddDate(0, 1, 0).Format(time.RFC3339)
	dateHint := today.Format("2006-01-02")

	return fmt.Sprintf(`Ты генерируешь YAML для AF-scenarios (Android Farm).
Верни ТОЛЬКО JSON: {"scenario_yaml":"...","variables_yaml":"..."} без markdown.

СЕГОДНЯ (MSK): %s. valid_from=%q, valid_until=%q — НЕ используй даты из прошлого.

Схема scenario.yaml:
- id, name, serial, timezone: Europe/Moscow
- valid_from, valid_until: RFC3339 +03:00
- schedule: { type: daily_recurring, execution: sequential | scheduled }
  - sequential: шаги цепочкой — каждый следующий ТОЛЬКО после завершения предыдущего
  - scheduled: каждый шаг в своё время at (HH:MM)
- content_sources.browser_research (объект, НЕ boolean) при необходимости
- steps[]: id, action, params (строки)
  - sequential: только ПЕРВЫЙ шаг имеет at (время старта цепочки); остальные — after_previous: true, БЕЗ at
  - scheduled: каждый шаг с at (HH:MM)

Допустимые action (ТОЛЬКО эти, без выдумок):
- wait (params.duration_sec)
- open_app / close_app (params.package)
- warmup_feed (params.profile, params.phase, params.until) — свайпы FYP TikTok
- browser_research (uses: content_sources.browser_research) — Chrome+Google, НЕ TikTok
- social_action (params.network: tiktok|instagram|youtube, params.behavior: launch|feed|search-feed|comment|open-tab|chat, params.query, params.count, params.like_probability, params.duration_sec) — поиск/лента через behavior-engine
- create_video_from_screenshots, publish_content
- custom_execute (params.steps_json — JSON-массив [{type,sec,x,y,...}])

TikTok package: com.zhiliaoapp.musically или com.ss.android.ugc.trill (регион).

variables.yaml: ОБЯЗАТЕЛЬНО включи полный блок (не только scroll_interval_sec):
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

Пример sequential (TikTok, старт 13:26 МСК, цепочка без фиксированных минут):
scenario_yaml:
id: tiktok-daily-12pm
name: "TikTok ежедневно 13:26"
serial: "PHONE"
timezone: "Europe/Moscow"
valid_from: "%s"
valid_until: "%s"
schedule:
  type: daily_recurring
  execution: sequential
steps:
  - id: open_tiktok
    at: "13:26"
    action: open_app
    params:
      package: com.zhiliaoapp.musically
  - id: warmup_feed
    after_previous: true
    action: warmup_feed
    params:
      profile: tiktok_daily
      phase: pre_publish
  - id: search_football
    after_previous: true
    action: social_action
    params:
      network: tiktok
      behavior: search-feed
      query: футбол
      count: "5"
      duration_sec: "120"
      skip_launch: "true"
  - id: close_tiktok
    after_previous: true
    after_failure: true
    action: close_app
    params:
      package: com.zhiliaoapp.musically

variables_yaml:
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
`, dateHint, validFrom, validUntil, validFrom, validUntil)
}
