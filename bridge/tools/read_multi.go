package tools

import (
	"context"
	"fmt"
	"strings"

	"devagotchi/bridge/policy"
)

type readMultiIn struct {
	Paths []string `json:"paths"`
}

func readMultipleFiles() Tool {
	return def(Tool{Name: "read_multiple_files", Title: "Read Multiple Files", Cap: policy.CapRead, ReadOnly: true,
		Desc: "Read several files at once (text: first file_read_line_limit lines; images as image content; PDF/Excel/DOCX as " +
			"read_file renders them). A failing file is reported inline and does not stop the others."},
		func(ctx context.Context, e *Env, in readMultiIn) Out {
			var b strings.Builder
			out := Out{}
			results := []map[string]any{}
			for _, p := range in.Paths {
				r := readFile().Run(ctx, e, mustJSON(map[string]any{"path": p}))
				fmt.Fprintf(&b, "=== %s ===\n%s\n\n", p, strings.TrimRight(r.Text, "\n"))
				out.Images = append(out.Images, r.Images...)
				results = append(results, map[string]any{"path": p, "ok": r.Code == "", "error": r.Code, "data": r.Data})
			}
			out.Text, out.Data = b.String(), map[string]any{"files": results}
			return out
		})
}

