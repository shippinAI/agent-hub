package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"devagotchi/bridge/policy"
)

type pathIn struct {
	Path string `json:"path"`
}

func createDirectory() Tool {
	return def(Tool{Name: "create_directory", Title: "Create Directory", Cap: policy.CapWrite,
		Desc: "Create a directory (and parents) inside the policy roots. Succeeds if it already exists."},
		func(ctx context.Context, e *Env, in pathIn) Out {
			p, err := e.Gate.Path(policy.CapWrite, in.Path)
			if err != nil {
				return errOut(err)
			}
			if err := os.MkdirAll(p, 0o755); err != nil {
				return errOut(err)
			}
			return ok("Directory ready: "+p, map[string]any{"path": p})
		})
}

type listDirIn struct {
	Path  string `json:"path"`
	Depth int    `json:"depth,omitempty" jsonschema:"levels to show (default 2)"`
}

func listDirectory() Tool {
	return def(Tool{Name: "list_directory", Title: "List Directory Contents", Cap: policy.CapRead, ReadOnly: true,
		Desc: "List a directory as [DIR]/[FILE] lines with paths relative to it, recursing depth levels (default 2). Nested " +
			"directories show at most nested_dir_limit (100) entries with a [WARNING] for the rest. Unreadable entries show " +
			"[DENIED]. Symlinked directories are listed but never followed."},
		func(ctx context.Context, e *Env, in listDirIn) Out {
			p, err := e.Gate.Path(policy.CapRead, in.Path)
			if err != nil {
				return errOut(err)
			}
			if st, err := os.Stat(p); err != nil {
				return fail("not_found", "[NOT_FOUND] %s", p)
			} else if !st.IsDir() {
				return fail("invalid", "%s is not a directory", p)
			}
			if in.Depth <= 0 {
				in.Depth = 2
			}
			var b strings.Builder
			n := walkList(&b, p, "", 1, in.Depth, e.Gate.Limits().NestedDirLimit)
			return ok(b.String(), map[string]any{"path": p, "entries": n})
		})
}

func walkList(b *strings.Builder, root, rel string, level, depth, limit int) int {
	ents, err := os.ReadDir(filepath.Join(root, rel))
	if err != nil {
		fmt.Fprintf(b, "[DENIED] %s\n", rel)
		return 0
	}
	count := 0
	for i, e := range ents {
		if level > 1 && i >= limit {
			fmt.Fprintf(b, "[WARNING] %s: %d items hidden (showing first %d of %d total)\n", rel, len(ents)-limit, limit, len(ents))
			break
		}
		r := filepath.ToSlash(filepath.Join(rel, e.Name()))
		count++
		switch {
		case e.Type()&os.ModeSymlink != 0:
			fmt.Fprintf(b, "[LINK] %s\n", r)
		case e.IsDir():
			fmt.Fprintf(b, "[DIR] %s\n", r)
			if level < depth {
				count += walkList(b, root, r, level+1, depth, limit)
			}
		default:
			fmt.Fprintf(b, "[FILE] %s\n", r)
		}
	}
	return count
}
