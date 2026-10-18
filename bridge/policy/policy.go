// Package policy is the local, human-owned rule set for the standalone MCP server and the limits both modes
// share: roots, capabilities, sandbox level, URL hosts, shell profiles, prompt patterns, limits and time-boxed
// grants. It is data (policy.toml). No MCP tool can write it; only `devagotchi config|grant` or an editor can.
package policy

import "time"

// Capabilities a policy or grant can name. Tools map onto these (see tools.Cap*).
const (
	CapRead     = "fs.read"          // read, list, search, file info
	CapWrite    = "fs.write"         // write, edit, move, mkdir, pdf/docx/xlsx writes
	CapExec     = "exec"             // one-shot argv runs
	CapProc     = "proc.interactive" // start/interact/read/terminate own sessions
	CapProcList = "proc.list"        // list every process on the machine
	CapProcKill = "proc.kill"        // kill processes this server did not start
	CapNet      = "net.fetch"        // read_file isUrl (still limited to url_hosts)
)

// Policy is policy.toml.
type Policy struct {
	SandboxLevel   string         `toml:"sandbox_level"`   // read-only | workspace-write | unsandboxed
	Roots          []string       `toml:"roots"`           // read+write roots (read-only when the level is read-only)
	ReadOnlyRoots  []string       `toml:"read_only_roots"` // read-only roots
	Capabilities   []string       `toml:"capabilities"`    // always-on capabilities
	URLHosts       []string       `toml:"url_hosts"`       // hosts read_file may fetch ("*" = any); empty = none
	EgressHosts    []string       `toml:"egress_hosts"`    // hosts sandboxed processes may reach through the proxy
	Shell          string         `toml:"shell"`           // shell profile for string commands
	History        bool           `toml:"history"`         // keep local tool-call history (never sent anywhere)
	FeedbackURL    string         `toml:"feedback_url"`    // what the feedback tool points at
	HTTPAddr       string         `toml:"http_addr"`       // default listen address for `mcp --http`
	SecretPatterns []string       `toml:"secret_patterns"` // extra redaction prefixes for output and history
	PromptPatterns []string       `toml:"prompt_patterns"` // regexes that mean "waiting for input"
	Limits         Limits         `toml:"limits"`
	ShellProfiles  []ShellProfile `toml:"shell_profiles"`
	CodeRunners    []ShellProfile `toml:"code_runners"`
	Grants         []Grant        `toml:"grants"`
}

// Limits are every cap a tool applies; each tool description states them, nothing is hidden.
type Limits struct {
	FileReadLineLimit  int   `toml:"file_read_line_limit"`
	FileWriteLineLimit int   `toml:"file_write_line_limit"` // 0 = no note; never blocks a write
	MaxReadBytes       int64 `toml:"max_read_bytes"`
	DefaultTimeoutMS   int   `toml:"default_timeout_ms"`
	MaxTimeoutMS       int   `toml:"max_timeout_ms"`
	OutputBufferBytes  int   `toml:"output_buffer_bytes"`
	URLTimeoutMS       int   `toml:"url_timeout_ms"`
	URLMaxBytes        int64 `toml:"url_max_bytes"`
	NestedDirLimit     int   `toml:"nested_dir_limit"`
	SearchMaxResults   int   `toml:"search_max_results"`
	SessionIdleSeconds int   `toml:"session_idle_seconds"`
	HistoryEntries     int   `toml:"history_entries"`
	HistoryOutputBytes int   `toml:"history_output_bytes"`
}

// ShellProfile turns a command string into argv: Argv + [script]. PTY asks for a pseudo-terminal (ConPTY on Windows).
type ShellProfile struct {
	Name string   `toml:"name"`
	Argv []string `toml:"argv"`
	PTY  bool     `toml:"pty"`
}

// Grant is a time-boxed extra a human added with `devagotchi grant add`.
type Grant struct {
	ID         string    `toml:"id"`
	Capability string    `toml:"capability"`
	Paths      []string  `toml:"paths"`
	Hosts      []string  `toml:"hosts"`
	Expires    time.Time `toml:"expires"`
	Note       string    `toml:"note"`
}

// Live reports whether the grant has not expired (zero Expires = until revoked).
func (g Grant) Live(now time.Time) bool { return g.Expires.IsZero() || now.Before(g.Expires) }
