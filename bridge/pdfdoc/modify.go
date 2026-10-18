package pdfdoc

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/pdfcpu/pdfcpu/pkg/api"
)

// Op is one write_pdf modification (DC operation shape).
type Op struct {
	Type          string `json:"type"` // delete | insert
	PageIndexes   []int  `json:"pageIndexes,omitempty"`
	PageIndex     int    `json:"pageIndex,omitempty"`
	Markdown      string `json:"markdown,omitempty"`
	SourcePdfPath string `json:"sourcePdfPath,omitempty"`
}

// Modify applies ops to src and writes out (which must differ from src; the original is kept).
// Insert sources are markdown (rendered with Create) or another PDF (sourcePdfPath, already policy-checked).
func Modify(src, out string, ops []Op, imageOK func(string) (string, bool)) error {
	work, err := os.MkdirTemp(filepath.Dir(out), ".devagotchi-pdf-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	ctx := context.Background()
	cur := filepath.Join(work, "cur.pdf")
	if n, err := api.PageCountFile(ctx, src); err != nil || n == 0 {
		return fmt.Errorf("cannot read %s as a PDF: %v", src, err)
	}
	if b, err := os.ReadFile(src); err != nil || os.WriteFile(cur, b, 0o600) != nil {
		return fmt.Errorf("cannot copy %s: %v", src, err)
	}
	for i, op := range ops {
		next := filepath.Join(work, "step"+strconv.Itoa(i)+".pdf")
		switch op.Type {
		case "delete":
			sel := []string{}
			for _, p := range op.PageIndexes {
				sel = append(sel, strconv.Itoa(p+1))
			}
			if err := api.RemovePagesFile(ctx, cur, next, sel, nil); err != nil {
				return fmt.Errorf("delete pages: %w", err)
			}
		case "insert":
			ins := op.SourcePdfPath
			if ins == "" {
				ins = filepath.Join(work, "ins"+strconv.Itoa(i)+".pdf")
				if err := Create(ins, op.Markdown, imageOK); err != nil {
					return err
				}
			}
			if err := insertAt(ctx, work, cur, ins, next, op.PageIndex); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown operation %q (delete or insert)", op.Type)
		}
		cur = next
	}
	return os.Rename(cur, out)
}

func insertAt(ctx context.Context, work, cur, ins, out string, at int) error {
	n, err := api.PageCountFile(ctx, cur)
	if err != nil {
		return err
	}
	m, err := api.PageCountFile(ctx, ins)
	if err != nil {
		return err
	}
	at = max(0, min(at, n))
	merged := filepath.Join(work, "merged.pdf")
	if err := api.MergeCreateFile(ctx, []string{cur, ins}, merged, false, nil); err != nil {
		return fmt.Errorf("merge: %w", err)
	}
	var sel []string
	if at > 0 {
		sel = append(sel, "1-"+strconv.Itoa(at))
	}
	sel = append(sel, strconv.Itoa(n+1)+"-"+strconv.Itoa(n+m))
	if at < n {
		sel = append(sel, strconv.Itoa(at+1)+"-"+strconv.Itoa(n))
	}
	return api.CollectFile(ctx, merged, out, sel, nil)
}
