package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"devagotchi/bridge/policy"
	"devagotchi/bridge/search"
	"devagotchi/bridge/xlsx"
)

type startSearchIn struct {
	Path             string `json:"path"`
	Pattern          string `json:"pattern"`
	SearchType       string `json:"searchType,omitempty" jsonschema:"files (default: match file names, globs like *.ts) or content (regex in file text)"`
	FilePattern      string `json:"filePattern,omitempty" jsonschema:"limit to files like *.js|*.ts"`
	IgnoreCase       *bool  `json:"ignoreCase,omitempty" jsonschema:"default true"`
	MaxResults       int    `json:"maxResults,omitempty" jsonschema:"0 = up to search_max_results"`
	IncludeHidden    bool   `json:"includeHidden,omitempty"`
	ContextLines     *int   `json:"contextLines,omitempty" jsonschema:"content search context lines (default 5)"`
	TimeoutMS        int    `json:"timeout_ms,omitempty" jsonschema:"0 = no time limit"`
	EarlyTermination *bool  `json:"earlyTermination,omitempty" jsonschema:"files: stop at an exact name match (default true for files)"`
	LiteralSearch    bool   `json:"literalSearch,omitempty" jsonschema:"treat pattern as a plain string"`
}

func startSearch() Tool {
	return def(Tool{Name: "start_search", Title: "Start Search", Cap: policy.CapRead, ReadOnly: true,
		Desc: "Start a background search and return the first results at once. searchType=files matches names (glob or " +
			"substring); content searches text with a regex (or literalSearch). Honors .gitignore/.ignore, skips hidden files " +
			"(unless includeHidden), binaries and files over 64 MiB; Excel cells are searched too. Built in (no ripgrep " +
			"download). Page with get_more_search_results; stop with stop_search."},
		func(ctx context.Context, e *Env, in startSearchIn) Out {
			root, err := e.Gate.Path(policy.CapRead, in.Path)
			if err != nil {
				return errOut(err)
			}
			if in.SearchType == "" {
				in.SearchType = "files"
			}
			q := search.Query{Root: root, Pattern: in.Pattern, Type: in.SearchType, FilePattern: in.FilePattern,
				IgnoreCase: truthy(in.IgnoreCase, true), MaxResults: in.MaxResults, IncludeHidden: in.IncludeHidden,
				Literal: in.LiteralSearch, EarlyStop: truthy(in.EarlyTermination, in.SearchType == "files")}
			if in.ContextLines != nil {
				q.ContextLines = *in.ContextLines
			} else if q.Type == "content" {
				q.ContextLines = 5
			}
			allow := func(p string) bool { _, err := e.Gate.Path(policy.CapRead, p); return err == nil }
			s := e.Searches.Start(q, time.Duration(in.TimeoutMS)*time.Millisecond, e.Gate.Limits().SearchMaxResults, search.Hooks{Allow: allow, Extract: excelLines})
			s.Wait(300 * time.Millisecond)
			hits, st := s.Page(0, 100)
			return ok(fmt.Sprintf("Started search %s\n%s", s.ID, pageText(hits, st, 0)), map[string]any{"status": st, "results": hits})
		})
}

func excelLines(p string) ([]string, bool) {
	if x := ext(p); x != ".xlsx" && x != ".xlsm" {
		return nil, false
	}
	b, err := xlsx.Open(p)
	if err != nil {
		return nil, false
	}
	defer b.Close()
	var out []string
	for _, s := range b.Sheets {
		g, _ := b.Grid(s)
		for r, row := range g {
			for c, v := range row {
				if v != nil {
					out = append(out, fmt.Sprintf("%s!%s: %v", s.Name, xlsx.Ref{Col: c + 1, Row: r + 1}, v))
				}
			}
		}
	}
	return out, true
}

func pageText(hits []search.Hit, st search.Status, offset int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[%s search for %q: %d results so far, complete=%v, %d ms]\n", st.Type, st.Pattern, st.Total, st.Complete, st.RuntimeMS)
	if st.Error != "" {
		fmt.Fprintf(&b, "[note: %s]\n", st.Error)
	}
	for _, h := range hits {
		if h.Line > 0 {
			fmt.Fprintf(&b, "%s:%d: %s\n", h.File, h.Line, strings.TrimSpace(h.Match))
		} else {
			fmt.Fprintf(&b, "%s\n", h.File)
		}
	}
	if !st.Complete || offset+len(hits) < st.Total {
		fmt.Fprintf(&b, "[more: get_more_search_results sessionId=%s offset=%d]\n", st.ID, offset+len(hits))
	}
	return b.String()
}
