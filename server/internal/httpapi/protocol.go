package httpapi

import (
	"fmt"
	"strings"
)

// AgentsMD is the claim/handoff protocol block agents paste into AGENTS.md / CLAUDE.md.
func AgentsMD(url string, beat int) string {
	return strings.TrimSpace(fmt.Sprintf(`
## Agent hub protocol (shared task board: %[1]s)
You coordinate with other agents through the hub (MCP server "hub", or the `+"`hub`"+` CLI). Identity comes from your token.
1. Start: `+"`list_ready`"+` (or `+"`hub ready`"+`), then `+"`claim_next`"+` / `+"`claim`"+`. Never work a task you have not claimed.
2. Keep the lease: `+"`heartbeat`"+` every ~%[2]d s with the `+"`fence`"+` from the claim. A 409 stale_fence / lease_expired / not_holder means you LOST the task: stop, do not write, re-read.
3. Every write on a held task sends `+"`fence`"+`; edits also send `+"`version`"+`. On version_conflict, re-read and retry.
4. Record decisions and blockers with `+"`add_note`"+` (kind decision|blocker|note) as you go.
5. Finish with `+"`complete`"+` (summary: what changed, how verified). If you stop early, `+"`handoff`"+` with a note: done so far, next step, gotchas; optionally `+"`to`"+` an agent.
6. Retrying after a timeout? Reuse the same `+"`request_id`"+` so the hub applies the write once.
7. Never force-release or reassign other agents' work; ask a human.`, url, beat))
}

// ConnectSnippets are per-client config lines for one principal (token stays in an env var).
func ConnectSnippets(url, id string) map[string]string {
	mcp := strings.TrimRight(url, "/") + "/mcp"
	return map[string]string{
		"claude_code": fmt.Sprintf(`claude mcp add --transport http hub %s --header "Authorization: Bearer ${HUB_TOKEN}"`, mcp),
		"codex":       fmt.Sprintf("# ~/.codex/config.toml\n[mcp_servers.hub]\nurl = %q\nbearer_token_env_var = \"HUB_TOKEN\"", mcp),
		"cursor":      fmt.Sprintf("{\n  \"mcpServers\": {\n    \"hub\": {\n      \"url\": %q,\n      \"headers\": { \"Authorization\": \"Bearer ${env:HUB_TOKEN}\" }\n    }\n  }\n}", mcp),
		"grok_cli":    fmt.Sprintf(`grok mcp add --transport http hub %s --header "Authorization: Bearer ${HUB_TOKEN}"`, mcp),
		"cli":         fmt.Sprintf("export HUB_URL=%s\nexport HUB_TOKEN_FILE=~/.config/hub/%s.token   # chmod 600\nhub whoami && hub ready", url, id),
	}
}
