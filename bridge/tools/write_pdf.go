package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"devagotchi/bridge/pdfdoc"
	"devagotchi/bridge/policy"
)

type writePDFIn struct {
	Path       string `json:"path" jsonschema:"PDF to create, or the source PDF to modify"`
	Content    any    `json:"content" jsonschema:"markdown string (create) or an array of operations (modify): {type:delete,pageIndexes:[0]} / {type:insert,pageIndex:1,markdown:'# New'} / {type:insert,pageIndex:0,sourcePdfPath:'/x.pdf'}"`
	OutputPath string `json:"outputPath,omitempty" jsonschema:"where to write; required for modify (must differ from path, the original is kept)"`
	Overwrite  bool   `json:"overwrite,omitempty" jsonschema:"allow replacing an existing output file"`
}

func writePDF() Tool {
	return def(Tool{Name: "write_pdf", Title: "Write/Modify PDF", Cap: policy.CapWrite, Destructive: true,
		Desc: "Create a PDF from markdown (headings, paragraphs, **bold**/*italic*/`code`, lists, code blocks, tables, " +
			"images from allowed paths, page breaks via <div style=\"page-break-before: always;\"></div>) with no browser " +
			"download, or modify an existing PDF (delete/insert pages) into a new outputPath. Core fonts: characters outside " +
			"Windows-1252 print as '?'; HTML/CSS styling and inline SVG are not rendered."},
		func(ctx context.Context, e *Env, in writePDFIn) Out {
			imageOK := func(p string) (string, bool) {
				r, err := e.Gate.Path(policy.CapRead, p)
				_, serr := os.Stat(r)
				return r, err == nil && serr == nil
			}
			ops, isOps := pdfOps(in.Content)
			out := in.OutputPath
			if out == "" && !isOps {
				out = in.Path
			}
			if out == "" {
				return fail("invalid", "outputPath is required when modifying a PDF")
			}
			dst, err := e.Gate.Path(policy.CapWrite, out)
			if err != nil {
				return errOut(err)
			}
			if _, err := os.Stat(dst); err == nil && !in.Overwrite {
				return fail("exists", "%s exists; pass overwrite=true or choose a new outputPath", dst)
			}
			if !isOps {
				md, _ := in.Content.(string)
				if err := pdfdoc.Create(dst, md, imageOK); err != nil {
					return errOut(err)
				}
				n, _ := pdfdoc.PageCount(dst)
				return ok(fmt.Sprintf("Created %s (%d pages)", dst, n), map[string]any{"path": dst, "pages": n, "sha256": fileSHA(dst, 1<<40)})
			}
			src, err := e.Gate.Path(policy.CapRead, in.Path)
			if err != nil {
				return errOut(err)
			}
			if src == dst {
				return fail("invalid", "outputPath must differ from path; the original is kept")
			}
			for i, op := range ops {
				if op.SourcePdfPath != "" {
					if ops[i].SourcePdfPath, err = e.Gate.Path(policy.CapRead, op.SourcePdfPath); err != nil {
						return errOut(err)
					}
				}
			}
			if err := pdfdoc.Modify(src, dst, ops, imageOK); err != nil {
				return errOut(err)
			}
			n, _ := pdfdoc.PageCount(dst)
			return ok(fmt.Sprintf("Wrote %s from %s (%d operations, %d pages)", dst, src, len(ops), n),
				map[string]any{"original": src, "path": dst, "pages": n, "sha256": fileSHA(dst, 1<<40)})
		})
}

// pdfOps accepts an array, or a JSON string of an array (DC accepts both).
func pdfOps(c any) ([]pdfdoc.Op, bool) {
	raw, _ := json.Marshal(c)
	if s, ok := c.(string); ok {
		raw = []byte(s)
	}
	var ops []pdfdoc.Op
	if len(raw) > 0 && raw[0] == '[' && json.Unmarshal(raw, &ops) == nil {
		return ops, true
	}
	return nil, false
}
