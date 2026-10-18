package tools

import (
	"context"
	"os"
	"time"

	"devagotchi/bridge/pdfdoc"
	"devagotchi/bridge/policy"
	"devagotchi/bridge/textio"
	"devagotchi/bridge/xlsx"
)

func getFileInfo() Tool {
	return def(Tool{Name: "get_file_info", Title: "Get File Information", Cap: policy.CapRead, ReadOnly: true,
		Desc: "Metadata for a file or directory: size, created (where the OS records it), modified, accessed, permissions, " +
			"type, sha256 (files up to max_read_bytes), and for text files lineCount, lastLine and appendPosition; Excel " +
			"files list sheets with rowCount/colCount; PDFs report pages."},
		func(ctx context.Context, e *Env, in pathIn) Out {
			p, err := e.Gate.Path(policy.CapRead, in.Path)
			if err != nil {
				return errOut(err)
			}
			st, err := os.Lstat(p)
			if err != nil {
				return errOut(err)
			}
			info := map[string]any{"path": p, "size": st.Size(), "modified": st.ModTime().UTC().Format(time.RFC3339),
				"permissions": st.Mode().Perm().String(), "mode_octal": "0" + itoa8(uint32(st.Mode().Perm()))}
			if c, a, ok := times(p, st); ok {
				info["accessed"] = a.UTC().Format(time.RFC3339)
				if !c.IsZero() {
					info["created"] = c.UTC().Format(time.RFC3339)
				}
			}
			switch {
			case st.Mode()&os.ModeSymlink != 0:
				info["type"] = "symlink"
			case st.IsDir():
				info["type"] = "directory"
			default:
				info["type"] = "file"
				fileExtras(e, p, info)
			}
			return ok(render(info), info)
		})
}

func fileExtras(e *Env, p string, info map[string]any) {
	info["sha256"] = fileSHA(p, e.Gate.Limits().MaxReadBytes)
	switch ext(p) {
	case ".xlsx", ".xlsm":
		if b, err := xlsx.Open(p); err == nil {
			var sheets []xlsx.Sheet
			for _, s := range b.Sheets {
				if g, err := b.Grid(s); err == nil {
					s.Rows = len(g)
					for _, r := range g {
						s.Cols = max(s.Cols, len(r))
					}
				}
				sheets = append(sheets, s)
			}
			b.Close()
			info["sheets"] = sheets
		}
	case ".pdf":
		if n, err := pdfdoc.PageCount(p); err == nil {
			info["pages"] = n
		}
	default:
		if f, err := os.Open(p); err == nil {
			bin, _ := textio.IsBinary(f)
			f.Close()
			if n, err := textio.CountLines(p); !bin && err == nil && n >= 0 {
				info["lineCount"], info["lastLine"], info["appendPosition"] = n, max(0, n-1), n
			}
		}
	}
}

func itoa8(n uint32) string {
	s := ""
	for ; n > 0; n /= 8 {
		s = string(rune('0'+n%8)) + s
	}
	if s == "" {
		return "0"
	}
	return s
}
