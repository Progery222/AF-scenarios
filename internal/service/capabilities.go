package service

// Поддерживаемые действия ScenarioRunner (orchestrator).
var validActions = map[string]struct{}{
	"wait":                          {},
	"open_app":                      {},
	"close_app":                     {},
	"warmup_feed":                   {},
	"browser_research":                {},
	"create_video_from_screenshots":   {},
	"publish_content":                 {},
	"social_action":                   {},
	"custom_execute":                  {},
}

// actionAliases — маппинг частых галлюцинаций LLM на реальный DSL.
var actionAliases = map[string]string{
	"scroll_down":                  "warmup_feed",
	"scroll_up":                    "warmup_feed",
	"scroll_feed":                  "warmup_feed",
	"scroll":                       "warmup_feed",
	"watch_video":                  "social_action",
	"watch_feed":                   "social_action",
	"search_in_browser_research":   "social_action",
	"search_in_app":                "social_action",
	"search_tiktok":                "social_action",
	"search":                       "social_action",
	"like_video":                   "social_action",
	"comment":                      "social_action",
	"launch_app":                   "open_app",
	"stop_app":                     "close_app",
}

// knownTikTokPackages — региональные пакеты TikTok.
var knownTikTokPackages = []string{
	"com.zhiliaoapp.musically",
	"com.ss.android.ugc.trill",
	"com.ss.android.ugc.aweme",
}

// socialBehaviors — допустимые behavior для social_action (behavior-engine).
var socialBehaviors = map[string]struct{}{
	"launch":       {},
	"open-tab":     {},
	"feed":         {},
	"search-feed":  {},
	"chat":         {},
	"comment":      {},
}
