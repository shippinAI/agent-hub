package tools

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func getUsageStats() Tool {
	return def(Tool{Name: "get_usage_stats", Title: "Get Usage Statistics", ReadOnly: true,
		Desc: "Local usage statistics (calls per tool, failures, sessions, uptime). Stored only on this machine; never sent anywhere."},
		func(ctx context.Context, e *Env, _ noIn) Out {
			if e.History == nil {
				return fail("disabled", "history is off in the policy (history = false)")
			}
			s := map[string]any{"usage": e.History.Summary(), "uptime_s": int(time.Since(e.Started).Seconds()),
				"live_sessions": len(e.Procs.List()), "searches": len(e.Searches.List())}
			return ok(render(s), s)
		})
}

type recentIn struct {
	MaxResults int    `json:"maxResults,omitempty" jsonschema:"default 50, max 1000"`
	ToolName   string `json:"toolName,omitempty"`
	Since      string `json:"since,omitempty" jsonschema:"RFC 3339 timestamp"`
}

func getRecentToolCalls() Tool {
	return def(Tool{Name: "get_recent_tool_calls", Title: "Get Recent Tool Calls", ReadOnly: true,
		Desc: "Recent tool calls with arguments and outcomes (secrets redacted, outputs capped), kept locally across restarts " +
			"for context recovery."},
		func(ctx context.Context, e *Env, in recentIn) Out {
			if e.History == nil {
				return fail("disabled", "history is off in the policy (history = false)")
			}
			if in.MaxResults <= 0 {
				in.MaxResults = 50
			}
			var since time.Time
			if in.Since != "" {
				t, err := time.Parse(time.RFC3339, in.Since)
				if err != nil {
					return fail("invalid", "since must be RFC 3339")
				}
				since = t
			}
			calls := e.History.Recent(min(in.MaxResults, 1000), in.ToolName, since)
			var b strings.Builder
			for _, c := range calls {
				fmt.Fprintf(&b, "%s %s ok=%v %dms %s\n", c.At.Format(time.RFC3339), c.Tool, c.OK, c.DurationMS, c.Args)
			}
			return ok(fmt.Sprintf("%d calls\n%s", len(calls), b.String()), map[string]any{"calls": calls})
		})
}

func giveFeedback() Tool {
	return def(Tool{Name: "give_feedback_to_desktop_commander", Title: "Give Feedback", ReadOnly: true,
		Desc: "Returns where to send feedback (feedback_url in the policy). Nothing is opened or sent automatically."},
		func(ctx context.Context, e *Env, _ noIn) Out {
			u := e.Gate.P.FeedbackURL
			if u == "" {
				return ok("No feedback_url is configured. Nothing was sent; this build has no telemetry.", map[string]any{"url": ""})
			}
			return ok("Send feedback at "+u+" (nothing was opened or sent automatically).", map[string]any{"url": u})
		})
}
