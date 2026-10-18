package providers

import (
	"fmt"
	"regexp"
	"sort"
)

// Section checks for the nested parts of a row (models, usage, state, detect, capabilities).

func models(errs *Errs, m Models) {
	enum(errs, "models.source", m.Source, ModelSources)
	if (m.Source == "command" || m.Source == "http") && m.Ref == "" {
		errs.add("models.ref", "is required when models.source is %s", m.Source)
	}
	text(errs, "models.ref", m.Ref, false, 500)
	if m.RefreshMinutes < 1 || m.RefreshMinutes > 10080 {
		errs.add("models.refresh_minutes", "must be between 1 and 10080 (one week)")
	}
}

func usage(errs *Errs, u Usage) {
	enum(errs, "usage.tokens", u.Tokens, TokenSources)
	enum(errs, "usage.quota", u.Quota, QuotaSources)
	enum(errs, "usage.cost_basis", u.CostBasis, CostBases)
	if u.Quota == "http" && u.QuotaRef == "" {
		errs.add("usage.quota_ref", "is required when usage.quota is http")
	}
	text(errs, "usage.quota_ref", u.QuotaRef, false, 500)
}

func state(errs *Errs, s State) {
	enum(errs, "state.read", s.Read, StateReads)
	if (s.Read == "command" || s.Read == "http") && s.Ref == "" {
		errs.add("state.ref", "is required when state.read is %s", s.Read)
	}
	if s.Read == "file" && (len(s.Paths) == 0 || s.Parser == "") {
		errs.add("state", "read file needs paths and a parser (id@version)")
	}
	if s.Parser != "" && !parserRe.MatchString(s.Parser) {
		errs.add("state.parser", "must be id@version, e.g. claude-jsonl@1")
	}
	for _, os := range keys(s.Paths) {
		ps := s.Paths[os]
		enum(errs, "state.paths", os, OSes)
		for i, x := range ps {
			if x == "" || credentialPath(x) {
				errs.add(fmt.Sprintf("state.paths.%s[%d]", os, i), "must be a transcript path, never a credential store")
			}
		}
	}
}

func detect(errs *Errs, d Detect) {
	for _, os := range keys(d) {
		r, p := d[os], "detect."+os
		enum(errs, "detect", os, OSes)
		argv(errs, p+".version_cmd", r.VersionCmd, false)
		argv(errs, p+".signed_in_cmd", r.SignedInCmd, false)
		if _, err := regexp.Compile(r.VersionRegex); err != nil {
			errs.add(p+".version_regex", "does not compile: %v", err)
		}
		for i, dir := range r.Dirs {
			if dir == "" || credentialPath(dir) {
				errs.add(fmt.Sprintf("%s.dirs[%d]", p, i), "must be a config dir, never a credential store")
			}
		}
	}
}

func capabilities(errs *Errs, c Capabilities) {
	flags := []struct {
		name string
		v    *bool
	}{{"streaming", c.Streaming}, {"tools", c.Tools}, {"mcp_client", c.MCPClient}, {"mcp_host", c.MCPHost},
		{"resume", c.Resume}, {"fork", c.Fork}, {"images", c.Images}, {"reasoning_effort", c.ReasoningEffort},
		{"approvals", c.Approvals}}
	for _, f := range flags {
		if f.v == nil {
			errs.add("capabilities."+f.name, "must be true or false")
		}
	}
}

func keys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
