package devapi

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"agenthub/internal/guides"
	"agenthub/internal/mcptools"
	"agenthub/internal/store"
)

type guideSearchIn struct {
	Query string `json:"query,omitempty"`
}

type guideGetIn struct {
	Guide string `json:"guide"`
}

func (a *API) guideTools(s *mcp.Server) {
	mcptools.Tool(s, "guides_search", "Find guides (title, goal, tags).", func(ctx context.Context, p *store.Principal, in guideSearchIn) (any, error) {
		return a.Guides.List(ctx, in.Query)
	})
	mcptools.Tool(s, "guide_get", "The effective published version of a guide (a tenant override wins) with its steps.", func(ctx context.Context, p *store.Principal, in guideGetIn) (any, error) {
		return a.Guides.Effective(ctx, in.Guide)
	})
	mcptools.Tool(s, "guide_run_start", "Start a guide run on a machine (or dry_run with a profile). Returns the run and its first step.", func(ctx context.Context, p *store.Principal, in guides.StartReq) (any, error) {
		out, err := a.Guides.Start(ctx, p, "", in)
		if err != nil {
			return nil, err
		}
		return a.runView(ctx, out)
	})
	mcptools.Tool(s, "guide_run_step", "Finish the current step: outcome ok|fail|skip|<confirm option>, or empty to have the hub run the step's op (agent steps) or file/check its access request. Operator and confirm steps are completed by the human in the UI.",
		func(ctx context.Context, p *store.Principal, in guides.StepReq) (any, error) {
			out, err := a.Guides.Step(ctx, p, "", in)
			if err != nil {
				return nil, err
			}
			view, _ := a.runView(ctx, map[string]any{"run_id": in.RunID})
			return map[string]any{"step_result": out, "now": view}, nil
		})
}

func (a *API) runView(ctx context.Context, out any) (any, error) {
	m, _ := out.(map[string]any)
	id, _ := m["run_id"].(string)
	run, err := a.Guides.GetRun(ctx, id)
	if err != nil {
		return out, nil
	}
	return a.Guides.View(ctx, run), nil
}

// guideResources exposes published guides as guide://<id>@<semver> resources and as prompts.
func (a *API) guideResources(ctx context.Context, s *mcp.Server) {
	list, err := a.Guides.List(ctx, "")
	if err != nil {
		return
	}
	for _, g := range list {
		if g.Semver == nil || !g.Enabled {
			continue
		}
		g := g
		uri := fmt.Sprintf("guide://%s@%s", g.ID, *g.Semver)
		s.AddResource(&mcp.Resource{URI: uri, Name: g.ID, Title: g.Title, Description: g.Goal, MIMEType: "application/json"},
			func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
				full, err := a.Guides.Get(ctx, g.ID)
				if err != nil {
					return nil, err
				}
				b, _ := json.Marshal(full)
				return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: uri, MIMEType: "application/json", Text: string(b)}}}, nil
			})
		s.AddPrompt(&mcp.Prompt{Name: g.ID, Title: g.Title, Description: g.Goal, Arguments: []*mcp.PromptArgument{{Name: "machine", Description: "machine id"}}},
			func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
				m := strings.TrimSpace(req.Params.Arguments["machine"])
				text := fmt.Sprintf("Follow the guide %q (%s) on machine %q: call guide_run_start {guide:%q, machine:%q}, explain each step to me in plain words, "+
					"ask for access with the step's request_access, and never ask me for a secret.", g.Title, uri, m, g.ID, m)
				return &mcp.GetPromptResult{Description: g.Goal, Messages: []*mcp.PromptMessage{{Role: "user", Content: &mcp.TextContent{Text: text}}}}, nil
			})
	}
}
