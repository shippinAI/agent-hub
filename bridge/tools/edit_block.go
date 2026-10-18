package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"devagotchi/bridge/docx"
	"devagotchi/bridge/fsops"
	"devagotchi/bridge/policy"
	"devagotchi/bridge/xlsx"
)

type editBlockIn struct {
	FilePath     string `json:"file_path"`
	OldString    string `json:"old_string,omitempty" jsonschema:"exact text to replace (include 1-3 lines of context so it is unique)"`
	NewString    string `json:"new_string,omitempty"`
	Expected     int    `json:"expected_replacements,omitempty" jsonschema:"how many times old_string must occur (default 1); any other count changes nothing"`
	ExpectSHA256 string `json:"expect_sha256,omitempty" jsonschema:"sha256 from read_file; refuse the edit if the file changed"`
	Range        string `json:"range,omitempty" jsonschema:"Excel: Sheet!A1:C10"`
	Content      any    `json:"content,omitempty" jsonschema:"Excel: 2D array for range"`
}

func editBlock() Tool {
	return def(Tool{Name: "edit_block", Title: "Edit Block", Cap: policy.CapWrite, Destructive: true,
		Desc: "Exact find/replace in a text file: old_string must occur exactly expected_replacements times (default 1) or nothing " +
			"is written; no fuzzy matching is ever applied (DC #97). On a miss it shows the closest text as " +
			"prefix{-removed-}{+added+}suffix so you can fix old_string. LF in old_string matches a CRLF file. Optional " +
			"expect_sha256 guards against concurrent changes; the write is atomic and returns the new sha256. Excel: range + " +
			"content (2D array) updates cells. DOCX: old_string/new_string match the XML shown by read_file with offset>=1."},
		func(ctx context.Context, e *Env, in editBlockIn) Out {
			p, err := e.Gate.Path(policy.CapWrite, in.FilePath)
			if err != nil {
				return errOut(err)
			}
			if in.ExpectSHA256 != "" {
				if cur := fileSHA(p, e.Gate.Limits().MaxReadBytes); cur != in.ExpectSHA256 {
					return Out{Code: "hash_mismatch", Text: "Error (hash_mismatch): the file changed since you read it", Data: map[string]any{"current": cur}}
				}
			}
			if in.Expected <= 0 {
				in.Expected = 1
			}
			switch x := ext(p); {
			case (x == ".xlsx" || x == ".xlsm") && in.Range != "":
				return editXLSX(p, in)
			case x == ".docx":
				part, err := docx.Replace(p, in.OldString, in.NewString, in.Expected)
				if err != nil {
					code, _, _ := strings.Cut(err.Error(), ":")
					if code != "edit_no_match" && code != "edit_ambiguous" {
						code = "io_error"
					}
					return fail(code, "%v", err)
				}
				return ok(fmt.Sprintf("Edited %s in %s", part, p), map[string]any{"path": p, "part": part, "sha256": fileSHA(p, 1<<40)})
			}
			if in.OldString == "" {
				return fail("invalid", "old_string is required for text files (or range + content for Excel)")
			}
			return editText(e, p, in)
		})
}

func editText(e *Env, p string, in editBlockIn) Out {
	b, err := readCapped(p, e.Gate.Limits().MaxReadBytes)
	if err != nil {
		return errOut(err)
	}
	s, old, nw, note := string(b), in.OldString, in.NewString, ""
	n := strings.Count(s, old)
	if n == 0 && strings.Contains(s, "\r\n") && !strings.Contains(old, "\r\n") {
		crOld := strings.ReplaceAll(old, "\n", "\r\n")
		if c := strings.Count(s, crOld); c > 0 {
			old, nw, n, note = crOld, strings.ReplaceAll(nw, "\n", "\r\n"), c, " (matched after converting LF to the file's CRLF)"
		}
	}
	if n == 0 {
		return Out{Code: "edit_no_match", Text: "Error (edit_no_match): old_string was not found; nothing was changed.\n" + nearMiss(s, old)}
	}
	if n != in.Expected {
		return fail("edit_ambiguous", "old_string occurs %d times but expected_replacements is %d; nothing was changed. Add context or set expected_replacements=%d", n, in.Expected, n)
	}
	line := strings.Count(s[:strings.Index(s, old)], "\n") + 1
	out := strings.ReplaceAll(s, old, nw)
	if err := fsops.AtomicWrite(p, []byte(out)); err != nil {
		return errOut(err)
	}
	sum := shaOf([]byte(out))
	warn := ""
	if lim := e.Gate.Limits().FileWriteLineLimit; lim > 0 && strings.Count(nw, "\n") > lim {
		warn = fmt.Sprintf(" Note: the replacement has more than file_write_line_limit (%d) lines.", lim)
	}
	return ok(fmt.Sprintf("Applied %d edit(s) to %s at line %d%s. New sha256 %s.%s", n, p, line, note, sum, warn),
		map[string]any{"path": p, "replacements": n, "line": line, "sha256": sum})
}

func editXLSX(p string, in editBlockIn) Out {
	r, err := xlsx.ParseRange(in.Range)
	if err != nil {
		return fail("invalid", "%v", err)
	}
	var grid [][]any
	raw, _ := json.Marshal(in.Content)
	if s, isStr := in.Content.(string); isStr {
		raw = []byte(s)
	}
	if err := json.Unmarshal(raw, &grid); err != nil {
		return fail("invalid", "content must be a 2D array: %v", err)
	}
	if err := xlsx.SetRange(p, r, grid); err != nil {
		return errOut(err)
	}
	return ok(fmt.Sprintf("Updated %s in %s", in.Range, p), map[string]any{"path": p, "range": in.Range, "sha256": fileSHA(p, 1<<40)})
}
