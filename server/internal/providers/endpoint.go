package providers

import (
	"encoding/json"
	"strings"
)

// Target is where an API adapter's request goes and in which wire shape.
type Target struct {
	URL    string // full request URL
	Shape  string // "chat_completions", "responses" or "messages"
	Router string // router mode used
}

// Resolve picks the request target for an API adapter. Direct mode uses the row's base_url (the vendor
// SDK's base URL); a router mode uses the router endpoint's URL, which is always OpenAI-compatible
// (LiteLLM, OpenRouter, custom). routerURL looks an endpoint id up; ok=false means not an API adapter.
func Resolve(a *Adapter, routerURL func(id string) string) (Target, bool) {
	if a == nil || !IsAPI(a.Kind) {
		return Target{}, false
	}
	var c APIConfig
	if json.Unmarshal(a.Config, &c) != nil {
		return Target{}, false
	}
	r := Router{Mode: "direct"}
	if c.Router != nil {
		r = *c.Router
	}
	if r.Mode != "direct" {
		base := routerURL(r.EndpointID)
		if base == "" {
			return Target{}, false
		}
		return Target{URL: strings.TrimRight(base, "/") + "/chat/completions", Shape: "chat_completions", Router: r.Mode}, true
	}
	base := strings.TrimRight(c.BaseURL, "/")
	if base == "" {
		return Target{}, false
	}
	if a.Kind == "anthropic_compatible" {
		return Target{URL: base + "/v1/messages", Shape: "messages", Router: "direct"}, true
	}
	if c.API == "responses" {
		return Target{URL: base + "/responses", Shape: "responses", Router: "direct"}, true
	}
	return Target{URL: base + "/chat/completions", Shape: "chat_completions", Router: "direct"}, true
}
