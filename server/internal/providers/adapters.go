package providers

import (
	"regexp"
	"strings"
)

// Overrides are the best-practice defaults every adapter ships (enums.go) that a row may override.
type Overrides struct {
	TimeoutMS  *int `json:"timeout_ms,omitempty"`
	MaxRetries *int `json:"max_retries,omitempty"`
	BackoffMS  *int `json:"backoff_ms,omitempty"`
}

// AcpConfig drives any ACP agent over stdio.
type AcpConfig struct {
	Argv        []string          `json:"argv"`
	Env         map[string]string `json:"env,omitempty"`
	SessionLoad bool              `json:"session_load,omitempty"`
	Overrides
}

// CodexAppServerConfig drives the Codex JSON-RPC app-server.
type CodexAppServerConfig struct {
	Argv       []string          `json:"argv"`
	Env        map[string]string `json:"env,omitempty"`
	RateLimits bool              `json:"rate_limits,omitempty"`
	Overrides
}

// HeadlessJSONConfig runs one process per turn and reads JSON or JSONL from stdout.
type HeadlessJSONConfig struct {
	Argv          []string          `json:"argv"`
	Env           map[string]string `json:"env,omitempty"`
	PromptArg     string            `json:"prompt_arg,omitempty"`
	ResumeArg     string            `json:"resume_arg,omitempty"`
	Output        []string          `json:"output,omitempty"`
	Framing       string            `json:"framing,omitempty"`
	SessionIDPath string            `json:"session_id_path,omitempty"`
	TextPath      string            `json:"text_path,omitempty"`
	UsagePath     string            `json:"usage_path,omitempty"`
	EventMap      map[string]string `json:"event_map,omitempty"`
	Overrides
}

// Router picks how an API adapter reaches the vendor: direct keys (default) or a router endpoint row.
type Router struct {
	Mode       string `json:"mode"`
	EndpointID string `json:"endpoint_id,omitempty"`
}

// APIConfig is the config of openai_compatible (API selects chat_completions or responses) and
// anthropic_compatible (Messages; API must be empty).
type APIConfig struct {
	BaseURL   string  `json:"base_url,omitempty"`
	API       string  `json:"api,omitempty"`
	SecretRef string  `json:"secret_ref,omitempty"`
	Router    *Router `json:"router,omitempty"`
	Overrides
}

var jsonPathRe = regexp.MustCompile(`^\$(\.[A-Za-z_][A-Za-z0-9_]*|\[[0-9]+\])*$`)

// AdapterConfig validates one adapter (kind + config) at path.
func AdapterConfig(errs *Errs, path string, a Adapter) {
	enum(errs, path+".kind", a.Kind, AdapterKinds)
	cfg := []byte(a.Config)
	if len(cfg) == 0 || string(cfg) == "null" {
		cfg = []byte("{}")
	}
	cp := path + ".config"
	switch a.Kind {
	case "acp":
		var c AcpConfig
		if decodeStrict(cfg, &c, cp, errs) {
			argv(errs, cp+".argv", c.Argv, true)
			env(errs, cp+".env", c.Env)
			overrides(errs, cp, c.Overrides)
		}
	case "codex_app_server":
		var c CodexAppServerConfig
		if decodeStrict(cfg, &c, cp, errs) {
			argv(errs, cp+".argv", c.Argv, true)
			env(errs, cp+".env", c.Env)
			overrides(errs, cp, c.Overrides)
		}
	case "headless_json":
		var c HeadlessJSONConfig
		if decodeStrict(cfg, &c, cp, errs) {
			headless(errs, cp, c)
		}
	case "openai_compatible", "anthropic_compatible":
		var c APIConfig
		if decodeStrict(cfg, &c, cp, errs) {
			api(errs, cp, a.Kind, c)
		}
	}
}

func headless(errs *Errs, cp string, c HeadlessJSONConfig) {
	argv(errs, cp+".argv", c.Argv, true)
	argv(errs, cp+".output", c.Output, false)
	env(errs, cp+".env", c.Env)
	if c.Framing != "" {
		enum(errs, cp+".framing", c.Framing, HeadlessFrames)
	}
	for k, v := range map[string]string{"session_id_path": c.SessionIDPath, "text_path": c.TextPath, "usage_path": c.UsagePath} {
		if v != "" && !jsonPathRe.MatchString(v) {
			errs.add(cp+"."+k, "must be a JSON path such as $.session_id")
		}
	}
	overrides(errs, cp, c.Overrides)
}

func api(errs *Errs, cp, kind string, c APIConfig) {
	link(errs, cp+".base_url", c.BaseURL)
	if kind == "openai_compatible" && c.API != "" {
		enum(errs, cp+".api", c.API, OpenAIAPIs)
	}
	if kind == "anthropic_compatible" && c.API != "" {
		errs.add(cp+".api", "is only for openai_compatible")
	}
	if c.SecretRef != "" && (!refNameRe.MatchString(c.SecretRef) || strings.HasPrefix(c.SecretRef, "sk-")) {
		errs.add(cp+".secret_ref", "must name a stored secret (lower-case id), never hold the key itself")
	}
	r := Router{Mode: "direct"}
	if c.Router != nil {
		r = *c.Router
	}
	enum(errs, cp+".router.mode", r.Mode, RouterModes)
	switch {
	case r.Mode == "direct" && r.EndpointID != "":
		errs.add(cp+".router.endpoint_id", "must be empty when router.mode is direct")
	case r.Mode == "direct" && c.BaseURL == "":
		errs.add(cp+".base_url", "is required when router.mode is direct")
	case r.Mode != "direct" && r.EndpointID == "":
		errs.add(cp+".router.endpoint_id", "is required when router.mode is %s", r.Mode)
	}
	overrides(errs, cp, c.Overrides)
}

func overrides(errs *Errs, cp string, o Overrides) {
	for k, v := range map[string]*int{"timeout_ms": o.TimeoutMS, "max_retries": o.MaxRetries, "backoff_ms": o.BackoffMS} {
		if v != nil && (*v < 0 || *v > 3600000) {
			errs.add(cp+"."+k, "must be between 0 and 3600000")
		}
	}
}
