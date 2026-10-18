package mcpserve

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const onboarding = `You have Devagotchi tools on this machine. Start with get_config to see the roots you may use,
the sandbox, and the limits. Then: list_directory and start_search to orient; read_file (note its sha256) before
write_file/edit_block with expect_sha256; start_process + interact_with_process for REPLs and long jobs (a timeout
never kills a session; read_process_output continues). If a call says grant_required or policy_denied, tell me the
command it suggests instead of trying to work around it.`

// addPrompts registers the onboarding prompt plus every prompts/*.md file in the config dir
// (the file name is the prompt name; the first line starting with "# " is its description).
func addPrompts(s *mcp.Server, dir string) {
	add(s, "devagotchi_onboarding", "How to use these tools well", onboarding)
	files, _ := filepath.Glob(filepath.Join(dir, "prompts", "*.md"))
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil || len(b) > 64<<10 {
			continue
		}
		name := strings.TrimSuffix(filepath.Base(f), ".md")
		desc, _, _ := strings.Cut(string(b), "\n")
		add(s, name, strings.TrimPrefix(desc, "# "), string(b))
	}
}

func add(s *mcp.Server, name, desc, text string) {
	s.AddPrompt(&mcp.Prompt{Name: name, Description: desc}, func(context.Context, *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		return &mcp.GetPromptResult{Description: desc, Messages: []*mcp.PromptMessage{{Role: "user", Content: &mcp.TextContent{Text: text}}}}, nil
	})
}
