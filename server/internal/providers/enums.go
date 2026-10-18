package providers

import "slices"

// The schema vocabulary (doc 34). These are the type's allowed words, not provider data; the web UI mirrors
// them in web/src/lib/providerTypes.ts (enums_ts_test.go keeps the two in step).
var (
	AdapterKinds   = []string{"acp", "codex_app_server", "headless_json", "openai_compatible", "anthropic_compatible"}
	APIAdapters    = []string{"openai_compatible", "anthropic_compatible"}
	RouterModes    = []string{"direct", "litellm", "openrouter", "custom"}
	AuthKinds      = []string{"client_login", "signin_oauth", "api_key", "plan_key"}
	Runners        = []string{"user_device", "hosted_workspace", "hub"}
	Editions       = []string{"local", "hosted"}
	OSes           = []string{"linux", "macos", "windows"}
	HostFor        = []string{"harness"}
	ModelSources   = []string{"adapter", "command", "http", "static"}
	TokenSources   = []string{"stream", "otel", "api_response", "none"}
	QuotaSources   = []string{"adapter", "statusline", "http", "observed", "none"}
	CostBases      = []string{"provider_charged", "computed", "client_estimate"}
	StateReads     = []string{"adapter", "command", "http", "file", "live_only"}
	OpenAIAPIs     = []string{"chat_completions", "responses"}
	HeadlessFrames = []string{"json", "jsonl"}
)

// Best-practice defaults the adapters apply when a row does not override them (rule 2 of doc 34).
const (
	DefaultRefreshMinutes = 360    // model list refresh (doc 29: model_list_refresh_minutes preset)
	DefaultTurnTimeoutMS  = 600000 // one agent turn
	DefaultDetectTimeout  = 5000   // version / status commands (doc 32)
	DefaultMaxRetries     = 2      // transient HTTP / spawn failures
	DefaultBackoffMS      = 1000   // first retry delay, doubled per retry
)

// IsAPI reports whether an adapter kind talks HTTP from the hub (the only kinds allowed runner "hub").
func IsAPI(kind string) bool { return slices.Contains(APIAdapters, kind) }
