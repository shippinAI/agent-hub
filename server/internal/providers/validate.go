package providers

import (
	"fmt"
	"slices"
)

// Parse decodes and validates one row from JSON. Unknown fields, wrong types and every schema rule are
// reported together as field errors. Older schema versions are converted first (convert.go).
func Parse(raw []byte) (Provider, Errs) {
	var errs Errs
	raw, err := Convert(raw)
	if err != nil {
		errs.add("schema_version", "%s", err.Error())
		return Provider{}, errs
	}
	var p Provider
	if !decodeStrict(raw, &p, "", &errs) {
		return p, errs
	}
	if p.Models.RefreshMinutes == 0 {
		p.Models.RefreshMinutes = DefaultRefreshMinutes
	}
	return p, Validate(p)
}

// Validate checks a decoded row against schema v1 (doc 34).
func Validate(p Provider) Errs {
	var errs Errs
	if p.SchemaVersion != SchemaVersion {
		errs.add("schema_version", "must be %d", SchemaVersion)
	}
	switch {
	case len(p.ID) > 64:
		errs.add("id", "is longer than 64 characters")
	case !idRe.MatchString(p.ID):
		errs.add("id", "must be vendor-product in lower case (e.g. anthropic-claude-code)")
	}
	text(&errs, "vendor", p.Vendor, true, 120)
	text(&errs, "product", p.Product, true, 120)
	text(&errs, "label", p.Label, true, 80)
	if !iconRe.MatchString(p.IconRef) {
		errs.add("icon_ref", "must be a lower-case icon path such as icons/xai")
	}
	link(&errs, "docs_url", p.DocsURL)
	link(&errs, "terms_url", p.TermsURL)
	text(&errs, "terms_note", p.TermsNote, false, 2000)
	set(&errs, "auth_kinds", p.AuthKinds, AuthKinds, true)
	set(&errs, "runner", p.Runner, Runners, true)
	set(&errs, "editions", p.Editions, Editions, true)
	if p.HostFor != "" {
		enum(&errs, "host_for", p.HostFor, HostFor)
	}
	adapters(&errs, p)
	models(&errs, p.Models)
	usage(&errs, p.Usage)
	state(&errs, p.State)
	detect(&errs, p.Detect)
	capabilities(&errs, p.Capabilities)
	return errs
}

func adapters(errs *Errs, p Provider) {
	api := false
	if p.Adapter == nil {
		if p.Capabilities.MCPHost == nil || !*p.Capabilities.MCPHost {
			errs.add("adapter", "is required unless the row is an MCP host only (capabilities.mcp_host)")
		}
		if len(p.FallbackAdapters) > 0 {
			errs.add("fallback_adapters", "need a preferred adapter first")
		}
	} else {
		AdapterConfig(errs, "adapter", *p.Adapter)
		api = IsAPI(p.Adapter.Kind)
	}
	seen := map[string]bool{}
	if p.Adapter != nil {
		seen[p.Adapter.Kind] = true
	}
	for i, a := range p.FallbackAdapters {
		path := fmt.Sprintf("fallback_adapters[%d]", i)
		AdapterConfig(errs, path, a)
		if seen[a.Kind] {
			errs.add(path+".kind", "%s is already the preferred or an earlier fallback adapter", a.Kind)
		}
		seen[a.Kind] = true
		api = api || IsAPI(a.Kind)
	}
	if slices.Contains(p.Runner, "hub") && !api {
		errs.add("runner", "hub is only for API adapters (openai_compatible, anthropic_compatible)")
	}
}
