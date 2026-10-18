package tools

import (
	"context"

	"devagotchi/bridge/policy"
)

type sessionIn struct {
	SessionID string `json:"sessionId"`
}

type moreIn struct {
	SessionID string `json:"sessionId"`
	Offset    int    `json:"offset,omitempty" jsonschema:"result index (negative = last N)"`
	Length    int    `json:"length,omitempty" jsonschema:"max results (default 100)"`
}

func getMoreSearchResults() Tool {
	return def(Tool{Name: "get_more_search_results", Title: "Get Search Results", Cap: policy.CapRead, ReadOnly: true,
		Desc: "Page through a search's results (offset/length; negative offset = last N) with its status."},
		func(ctx context.Context, e *Env, in moreIn) Out {
			s, found := e.Searches.Get(in.SessionID)
			if !found {
				return fail("not_found", "no search %s (finished searches are kept 5 minutes)", in.SessionID)
			}
			if in.Length <= 0 {
				in.Length = 100
			}
			hits, st := s.Page(in.Offset, in.Length)
			return ok(pageText(hits, st, max(in.Offset, 0)), map[string]any{"status": st, "results": hits})
		})
}

func stopSearch() Tool {
	return def(Tool{Name: "stop_search", Title: "Stop Search", Cap: policy.CapRead,
		Desc: "Stop a running search; its results stay readable for 5 minutes."},
		func(ctx context.Context, e *Env, in sessionIn) Out {
			if !e.Searches.Stop(in.SessionID) {
				return fail("not_found", "no search %s", in.SessionID)
			}
			return ok("Stopped "+in.SessionID, map[string]any{"sessionId": in.SessionID})
		})
}

type noIn struct{}

func listSearches() Tool {
	return def(Tool{Name: "list_searches", Title: "List Active Searches", Cap: policy.CapRead, ReadOnly: true,
		Desc: "List searches with id, type, pattern, status and runtime."},
		func(ctx context.Context, e *Env, _ noIn) Out {
			l := e.Searches.List()
			return ok(render(map[string]any{"searches": l}), map[string]any{"searches": l})
		})
}
