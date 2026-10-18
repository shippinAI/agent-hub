package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"devagotchi/bridge/docx"
	"devagotchi/bridge/fsops"
	"devagotchi/bridge/policy"
	"devagotchi/bridge/xlsx"
)

type writeFileIn struct {
	Path         string `json:"path"`
	Content      string `json:"content"`
	Mode         string `json:"mode,omitempty" jsonschema:"rewrite (default) or append"`
	ExpectSHA256 string `json:"expect_sha256,omitempty" jsonschema:"sha256 from read_file; the write fails with hash_mismatch if the file changed since"`
	ExpectAbsent bool   `json:"expect_absent,omitempty" jsonschema:"fail if the file already exists"`
}

func writeFile() Tool {
	return def(Tool{Name: "write_file", Title: "Write File", Cap: policy.CapWrite, Destructive: true,
		Desc: "Write or append a file atomically (temp file + fsync + rename; the result is re-read and its sha256 verified, so a " +
			"'success' is never an empty file). Optional expect_sha256 / expect_absent make it a safe compare-and-swap. Excel " +
			"(.xlsx): content is a JSON 2D array or {\"Sheet\": [[...]]}. DOCX: markdown becomes styled paragraphs. PDFs: use write_pdf. " +
			"Long files can be written in chunks (rewrite, then append)."},
		func(ctx context.Context, e *Env, in writeFileIn) Out {
			p, err := e.Gate.Path(policy.CapWrite, in.Path)
			if err != nil {
				return errOut(err)
			}
			cur := fileSHA(p, e.Gate.Limits().MaxReadBytes)
			_, statErr := os.Stat(p)
			exists := statErr == nil
			if in.ExpectAbsent && exists {
				return Out{Code: "hash_mismatch", Text: "Error (hash_mismatch): the file already exists", Data: map[string]any{"current": cur}}
			}
			if in.ExpectSHA256 != "" && exists && cur != in.ExpectSHA256 {
				return Out{Code: "hash_mismatch", Text: "Error (hash_mismatch): the file changed since you read it; re-read and retry", Data: map[string]any{"current": cur}}
			}
			switch ext(p) {
			case ".xlsx", ".xlsm":
				return writeXLSX(p, in.Content)
			case ".docx":
				if in.Mode == "append" {
					return fail("unsupported", "append is not supported for DOCX; use edit_block")
				}
				if err := docx.Create(p, in.Content); err != nil {
					return errOut(err)
				}
				return ok("Wrote DOCX "+p, map[string]any{"path": p, "sha256": fileSHA(p, 1<<40)})
			case ".pdf":
				return fail("invalid", "use write_pdf for PDF files")
			}
			body := []byte(in.Content)
			if in.Mode == "append" && exists {
				old, err := readCapped(p, e.Gate.Limits().MaxReadBytes)
				if err != nil {
					return errOut(err)
				}
				body = append(old, body...)
			} else if in.Mode != "" && in.Mode != "rewrite" && in.Mode != "append" {
				return fail("invalid", "mode must be rewrite or append")
			}
			if err := fsops.AtomicWrite(p, body); err != nil {
				return errOut(err)
			}
			want := shaOf(body)
			if got := fileSHA(p, int64(len(body))+1); got != want {
				return fail("io_error", "wrote %s but the file on disk does not match (sha %s, expected %s)", p, got, want)
			}
			note := ""
			if lim := e.Gate.Limits().FileWriteLineLimit; lim > 0 && strings.Count(in.Content, "\n") > lim {
				note = fmt.Sprintf(" Note: %d lines in one call is above file_write_line_limit %d (a hint only; the write succeeded).", strings.Count(in.Content, "\n"), lim)
			}
			verb := map[bool]string{true: "Appended to", false: "Wrote"}[in.Mode == "append"]
			return ok(fmt.Sprintf("%s %s (%d bytes, sha256 %s, verified).%s", verb, p, len(body), want, note),
				map[string]any{"path": p, "sha256": want, "size": len(body), "previous_sha256": cur})
		})
}

func writeXLSX(p, content string) Out {
	var grid [][]any
	var sheets map[string][][]any
	var out []xlsx.NamedGrid
	if json.Unmarshal([]byte(content), &grid) == nil {
		out = []xlsx.NamedGrid{{Name: "Sheet1", Rows: grid}}
	} else if json.Unmarshal([]byte(content), &sheets) == nil {
		for name, g := range sheets {
			out = append(out, xlsx.NamedGrid{Name: name, Rows: g})
		}
	} else {
		return fail("invalid", "Excel content must be a JSON 2D array or an object of sheet name -> 2D array")
	}
	if err := xlsx.Write(p, out); err != nil {
		return errOut(err)
	}
	return ok(fmt.Sprintf("Wrote Excel %s (%d sheet(s))", p, len(out)), map[string]any{"path": p, "sha256": fileSHA(p, 1<<40)})
}
