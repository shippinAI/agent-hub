// Package policyrules evaluates data-defined policy rules against one agent tool call.
// Rules are data (a versioned JSON document); only the small check vocabulary below is code.
package policyrules

// Doc is one version of the whole policy: classifiers, the region table and the rules.
type Doc struct {
	Version   int64               `json:"version"`
	UpdatedAt string              `json:"updated_at"`
	UpdatedBy string              `json:"updated_by"`
	Classes   map[string][]string `json:"classes"` // class name -> regexes over the command text
	Regions   []Region            `json:"regions"`
	Rules     []Rule              `json:"rules"`
}

// Region maps host globs / CIDRs to a site code (WI, TX, VA ...).
type Region struct {
	Code  string   `json:"code"`
	Name  string   `json:"name"`
	Hosts []string `json:"hosts"` // shell globs over hostnames, or CIDRs
}

// Rule is one policy. Effect "require" denies unless every check passes; "deny" always denies when it
// matches; "allow" and "audit" never block; "collect" runs evidence collectors on failure events.
type Rule struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Hook     string         `json:"hook"` // which operating hook this mirrors (shown in the UI)
	Enabled  bool           `json:"enabled"`
	Priority int            `json:"priority"`
	Effect   string         `json:"effect"`
	When     When           `json:"when"`
	Require  []Check        `json:"require,omitempty"`
	Message  string         `json:"message"`
	Params   map[string]int `json:"params,omitempty"`  // runner knobs, e.g. verify_within_s, max_revert_s
	Collect  []string       `json:"collect,omitempty"` // evidence commands (read-only) for effect=collect
}

// When narrows a rule; every non-empty field must match. Empty When matches everything.
type When struct {
	Events    []string `json:"events,omitempty"`  // pre (default), post-failure
	Tools     []string `json:"tools,omitempty"`   // Bash, Write, hub:exec ... ("*" = any)
	Classes   []string `json:"classes,omitempty"` // any of these classes
	Regions   []string `json:"regions,omitempty"` // any of these region codes
	CommandRe string   `json:"command_re,omitempty"`
	ViaRunner *bool    `json:"via_runner,omitempty"`
}

// Check is one requirement. Kinds: ticket, field, list, number, approval, review, via_runner.
type Check struct {
	Kind       string `json:"check"`
	Field      string `json:"field,omitempty"`
	Pattern    string `json:"pattern,omitempty"`     // field/list items must match
	Not        string `json:"not,omitempty"`         // field must NOT match
	MinLen     int    `json:"min_len,omitempty"`     // field
	FileExists bool   `json:"file_exists,omitempty"` // field (or each list item that looks like a path)
	MaxAgeS    int    `json:"max_age_s,omitempty"`   // with file_exists
	ContainsID bool   `json:"contains_id,omitempty"` // file must mention the ticket id
	Min        *int   `json:"min,omitempty"`         // number value / list length
	Max        *int   `json:"max,omitempty"`
	Role       string `json:"role,omitempty"` // review: only reviewers holding this role count
	Msg        string `json:"msg,omitempty"`
}

// Request is one tool call, normalized from any source (Claude Code, Codex, wrapper, hub).
type Request struct {
	Event     string   `json:"event"` // pre | post-failure
	Source    string   `json:"source"`
	Tool      string   `json:"tool"`
	Command   string   `json:"command"`
	Paths     []string `json:"paths,omitempty"`
	Hosts     []string `json:"hosts,omitempty"`
	Principal string   `json:"principal,omitempty"`
	TicketID  string   `json:"ticket_id,omitempty"`
	ViaRunner bool     `json:"via_runner"`
}

// Decision is the engine's answer.
type Decision struct {
	Allow    bool           `json:"allow"`
	Version  int64          `json:"policy_version"`
	Classes  []string       `json:"classes"`
	Regions  []string       `json:"regions"`
	Matched  []string       `json:"matched"`
	Failures []Failure      `json:"failures,omitempty"`
	Collect  []string       `json:"collect,omitempty"`
	Params   map[string]int `json:"params,omitempty"`
	Ticket   string         `json:"ticket_id,omitempty"`
}

// Failure says which rule blocked and why.
type Failure struct {
	Rule    string   `json:"rule"`
	Hook    string   `json:"hook"`
	Message string   `json:"message"`
	Why     []string `json:"why"`
}
