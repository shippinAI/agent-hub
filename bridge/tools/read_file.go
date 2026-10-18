package tools

import (
	"context"
	"fmt"
	"os"
	"time"

	"devagotchi/bridge/policy"
	"devagotchi/bridge/textio"
)

type readFileIn struct {
	Path   string `json:"path" jsonschema:"absolute path, or a URL when isUrl is true"`
	IsURL  bool   `json:"isUrl,omitempty" jsonschema:"fetch an http(s) URL (host must be in url_hosts; 30 s default timeout)"`
	Offset int    `json:"offset,omitempty" jsonschema:"text: start line (0-based; negative = last N lines). PDF: page. Excel: row"`
	Length int    `json:"length,omitempty" jsonschema:"max lines/pages/rows (default file_read_line_limit, 1000)"`
	Sheet  string `json:"sheet,omitempty" jsonschema:"Excel sheet name or 0-based index as a string"`
	Range  string `json:"range,omitempty" jsonschema:"Excel range such as A1:D100"`
}

func readFile() Tool {
	return def(Tool{Name: "read_file", Title: "Read File or URL", Cap: policy.CapRead, ReadOnly: true, OpenWorld: true,
		Desc: "Read a text file by line window (offset/length; negative offset = tail), an image (returned as image content), " +
			"a PDF (page text + embedded images), Excel .xlsx/.xlsm (JSON 2D array; sheet, range), DOCX (outline; with offset>0 the " +
			"pretty XML lines for edit_block), or a URL (isUrl). Returns sha256 of the file: pass it to edit_block/write_file as " +
			"expect_sha256 so a changed file is never overwritten. Paths must be inside the policy roots (symlinks resolved)."},
		func(ctx context.Context, e *Env, in readFileIn) Out {
			if in.IsURL {
				return readURL(ctx, e, in.Path)
			}
			p, err := e.Gate.Path(policy.CapRead, in.Path)
			if err != nil {
				return errOut(err)
			}
			st, err := os.Stat(p)
			if err != nil {
				return errOut(err)
			}
			if st.IsDir() {
				return fail("invalid", "%s is a directory; use list_directory", p)
			}
			if in.Length <= 0 {
				in.Length = e.Gate.Limits().FileReadLineLimit
			}
			switch x := ext(p); {
			case imageMimes[x] != "":
				return readImage(e, p, imageMimes[x])
			case x == ".pdf":
				return readPDF(e, p, in)
			case x == ".xlsx" || x == ".xlsm":
				return readXLSX(p, in)
			case x == ".docx":
				return readDOCX(p, in)
			case x == ".xls":
				return fail("unsupported", "legacy .xls (BIFF) is not supported; save it as .xlsx")
			}
			return readText(e, p, st, in)
		})
}

func readText(e *Env, p string, st os.FileInfo, in readFileIn) Out {
	l := e.Gate.Limits()
	w, err := textio.ReadLines(p, in.Offset, in.Length, l.MaxReadBytes)
	if err != nil {
		if err == textio.ErrBinary {
			return fail("binary", "%s: %v", p, err)
		}
		return errOut(err)
	}
	total := "unknown"
	if w.Total >= 0 {
		total = fmt.Sprint(w.Total)
	}
	head := fmt.Sprintf("[Reading %d lines from line %d (total: %s lines, %d remaining)]\n", w.Returned, w.Start, total, w.Remaining)
	if w.Truncated {
		head += "[window cut at max_read_bytes; read a smaller range]\n"
	}
	data := map[string]any{"path": p, "size": st.Size(), "mtime": st.ModTime().UTC().Format(time.RFC3339),
		"sha256": fileSHA(p, l.MaxReadBytes), "window": w}
	return ok(head+w.Text, data)
}

func readImage(e *Env, p, mime string) Out {
	b, err := readCapped(p, e.Gate.Limits().MaxReadBytes)
	if err != nil {
		return errOut(err)
	}
	return Out{Text: fmt.Sprintf("Image %s (%s, %d bytes)", p, mime, len(b)), Images: []Image{{Mime: mime, Data: b}},
		Data: map[string]any{"path": p, "mime": mime, "size": len(b), "sha256": shaOf(b)}}
}
