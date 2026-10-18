// Package providers is the provider schema v1 (docs/devagotchi/34-provider-schema.md, ADR 0002): one row shape
// for every AI client or API, validated here, stored in the providers table, edited as data in the UI.
// Nothing vendor-specific lives in this package; argv, paths, endpoints and regexes are row data.
package providers

import "encoding/json"

// SchemaVersion is the newest row shape this hub understands. Newer rows are refused; older rows are converted.
const SchemaVersion = 1

// Provider is one row. JSON names are the column and API names (snake_case, rule 1 of doc 34).
type Provider struct {
	SchemaVersion    int          `json:"schema_version"`
	ID               string       `json:"id"`
	Vendor           string       `json:"vendor"`
	Product          string       `json:"product"`
	Label            string       `json:"label"`
	IconRef          string       `json:"icon_ref"`
	DocsURL          string       `json:"docs_url,omitempty"`
	AuthKinds        []string     `json:"auth_kinds"`
	Runner           []string     `json:"runner"`
	Editions         []string     `json:"editions"`
	Adapter          *Adapter     `json:"adapter"`
	FallbackAdapters []Adapter    `json:"fallback_adapters,omitempty"`
	HostFor          string       `json:"host_for,omitempty"`
	Models           Models       `json:"models"`
	Usage            Usage        `json:"usage"`
	State            State        `json:"state"`
	Detect           Detect       `json:"detect"`
	Capabilities     Capabilities `json:"capabilities"`
	TermsURL         string       `json:"terms_url,omitempty"`
	TermsNote        string       `json:"terms_note,omitempty"`
}

// Adapter names one of the five adapters and its per-kind config (validated by kind, see adapters.go).
type Adapter struct {
	Kind   string          `json:"kind"`
	Config json.RawMessage `json:"config,omitempty"`
}

// Models says where the model list comes from; the list itself is cached in provider_models.
type Models struct {
	Source         string `json:"source"`
	Ref            string `json:"ref,omitempty"`
	RefreshMinutes int    `json:"refresh_minutes"`
}

// Usage says where token counts and quota are read and how cost is based.
type Usage struct {
	Tokens    string `json:"tokens"`
	Quota     string `json:"quota"`
	QuotaRef  string `json:"quota_ref,omitempty"`
	CostBasis string `json:"cost_basis"`
}

// State says how past sessions are read (doc 32: official surface first, files last, behind a parser).
type State struct {
	Read   string              `json:"read"`
	Ref    string              `json:"ref,omitempty"`
	Paths  map[string][]string `json:"paths,omitempty"`
	Parser string              `json:"parser,omitempty"`
}

// Detect holds the detection rules per OS (linux, macos, windows).
type Detect map[string]DetectRule

// DetectRule is run by the bridge under fs.read plus exec of the version and status commands only.
type DetectRule struct {
	Bins         []string `json:"bins,omitempty"`
	Dirs         []string `json:"dirs,omitempty"`
	AppIDs       []string `json:"app_ids,omitempty"`
	VersionCmd   []string `json:"version_cmd,omitempty"`
	VersionRegex string   `json:"version_regex,omitempty"`
	SignedInCmd  []string `json:"signed_in_cmd,omitempty"`
}

// Capabilities are the nine feature flags every row states explicitly.
type Capabilities struct {
	Streaming       *bool `json:"streaming"`
	Tools           *bool `json:"tools"`
	MCPClient       *bool `json:"mcp_client"`
	MCPHost         *bool `json:"mcp_host"`
	Resume          *bool `json:"resume"`
	Fork            *bool `json:"fork"`
	Images          *bool `json:"images"`
	ReasoningEffort *bool `json:"reasoning_effort"`
	Approvals       *bool `json:"approvals"`
}
