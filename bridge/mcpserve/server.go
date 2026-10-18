// Package mcpserve exposes the tools package as a local MCP server over stdio or streamable HTTP.
// Logs go to stderr only (stdout is the protocol on stdio). Nothing is sent anywhere else.
package mcpserve

import (
	"context"
	"path/filepath"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"devagotchi/bridge/buildinfo"
	"devagotchi/bridge/history"
	"devagotchi/bridge/policy"
	"devagotchi/bridge/redact"
	"devagotchi/bridge/tools"
)

// Config is everything the standalone server needs; all of it comes from data (policy file, flags).
type Config struct {
	Policy     policy.Policy
	ConfigPath string   // policy.toml shown by get_config
	ConfigDir  string   // history, prompts/, http-token live here
	ExtraRoots []string // --root flags written into the client config by a human
	Self       string   // this binary, used as the in-sandbox init
	Log        func(string)
}

const instructions = "Devagotchi: local file, search, office/PDF and terminal tools (Desktop Commander-compatible names). " +
	"Access is limited to the policy roots and capabilities (see get_config); errors say grant_required or policy_denied " +
	"with the command a human can run. Prefer argv over shell strings, pass expect_sha256 from read_file to write_file/" +
	"edit_block, and use start_process + interact_with_process for REPLs. The model cannot change this policy."

// Server is one MCP server plus the shared tool environment.
type Server struct {
	MCP   *mcp.Server
	env   *tools.Env
	roots *rootCache
}

// New builds the server: tools, prompts and resources, with history recording.
func New(c Config) *Server {
	if c.Log == nil {
		c.Log = func(string) {}
	}
	var hist *history.Log
	if c.Policy.History {
		l := c.Policy.Limits
		hist = history.Open(filepath.Join(c.ConfigDir, "history"), true, l.HistoryEntries, l.HistoryOutputBytes, redact.New(c.Policy.SecretPatterns))
	}
	env := tools.NewEnv(policy.NewGate(c.Policy, c.ExtraRoots...), "standalone", c.Self, hist, c.Log)
	env.ConfigPath = c.ConfigPath
	s := &Server{env: env, roots: newRootCache()}
	s.MCP = mcp.NewServer(&mcp.Implementation{Name: "devagotchi", Title: "Devagotchi", Version: buildinfo.Version},
		&mcp.ServerOptions{Instructions: instructions,
			InitializedHandler:      func(_ context.Context, r *mcp.InitializedRequest) { s.roots.refresh(r.Session) },
			RootsListChangedHandler: func(_ context.Context, r *mcp.RootsListChangedRequest) { s.rootsChanged(r.Session) }})
	for _, t := range tools.All() {
		s.MCP.AddTool(describe(t), s.handler(t))
	}
	addPrompts(s.MCP, c.ConfigDir)
	addResources(s.MCP, s.env)
	return s
}

// Close kills every session the server started.
func (s *Server) Close() { s.env.Close() }

// RunStdio serves one client on stdin/stdout until it disconnects or ctx ends.
func (s *Server) RunStdio(ctx context.Context) error {
	return s.MCP.Run(ctx, &mcp.StdioTransport{})
}

func describe(t tools.Tool) *mcp.Tool {
	open := t.OpenWorld
	destr := t.Destructive
	return &mcp.Tool{Name: t.Name, Title: t.Title, Description: t.Desc, InputSchema: t.Schema,
		Annotations: &mcp.ToolAnnotations{Title: t.Title, ReadOnlyHint: t.ReadOnly, DestructiveHint: &destr, OpenWorldHint: &open}}
}

func (s *Server) handler(t tools.Tool) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		env := *s.env
		g, input := s.roots.resolve(ctx, req, s.env.Gate)
		if input != nil {
			return input, nil
		}
		env.Gate = g
		if ip := req.Session.InitializeParams(); ip != nil && ip.ClientInfo != nil {
			env.Client = ip.ClientInfo.Name
		}
		start := time.Now()
		out := t.Run(ctx, &env, req.Params.Arguments)
		s.record(t.Name, req.Params.Arguments, out, env.Client, start)
		return result(out), nil
	}
}

func (s *Server) rootsChanged(ss *mcp.ServerSession) {
	if ip := ss.InitializeParams(); ip != nil && ip.ProtocolVersion >= newProtocol {
		s.roots.forget(ss)
		return
	}
	s.roots.refresh(ss)
}
