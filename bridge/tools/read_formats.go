package tools

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"devagotchi/bridge/buildinfo"
	"devagotchi/bridge/docx"
	"devagotchi/bridge/pdfdoc"
	"devagotchi/bridge/policy"
	"devagotchi/bridge/scope"
	"devagotchi/bridge/webfetch"
	"devagotchi/bridge/xlsx"
)

// maxPDFImages bounds the images one read returns.
const maxPDFImages = 6

func readPDF(e *Env, p string, in readFileIn) Out {
	length := in.Length
	if length == e.Gate.Limits().FileReadLineLimit {
		length = 0 // pages, not lines: default is the whole document
	}
	d, err := pdfdoc.Read(p, in.Offset, length, maxPDFImages)
	if err != nil {
		return errOut(err)
	}
	out := Out{Text: fmt.Sprintf("[PDF %s: pages %d-%d of %d]\n\n%s", p, d.From+1, d.To, d.Pages, d.Text),
		Data: map[string]any{"path": p, "pages": d.Pages, "from": d.From, "to": d.To, "images": len(d.Images), "sha256": fileSHA(p, e.Gate.Limits().MaxReadBytes)}}
	for _, im := range d.Images {
		out.Images = append(out.Images, Image{Mime: im.Mime, Data: im.Data})
	}
	return out
}

func readXLSX(p string, in readFileIn) Out {
	b, err := xlsx.Open(p)
	if err != nil {
		return errOut(err)
	}
	defer b.Close()
	sh, err := b.Find(in.Sheet)
	if err != nil {
		return fail("not_found", "%v", err)
	}
	g, err := b.Grid(sh)
	if err != nil {
		return errOut(err)
	}
	if in.Range != "" {
		r, err := xlsx.ParseRange(in.Range)
		if err != nil {
			return fail("invalid", "%v", err)
		}
		g = cut(g, r)
	} else {
		from := min(max(in.Offset, 0), len(g))
		g = g[from:min(len(g), from+in.Length)]
	}
	js, _ := json.Marshal(g)
	names := []string{}
	for _, s := range b.Sheets {
		names = append(names, s.Name)
	}
	return ok(fmt.Sprintf("[Excel %s, sheet %q (sheets: %s), %d rows]\n%s", p, sh.Name, strings.Join(names, ", "), len(g), js),
		map[string]any{"path": p, "sheet": sh.Name, "sheets": names, "rows": g})
}

func cut(g [][]any, r xlsx.Range) [][]any {
	var out [][]any
	for row := r.From.Row; row <= r.To.Row && row <= len(g); row++ {
		var line []any
		for col := r.From.Col; col <= r.To.Col; col++ {
			var v any
			if col <= len(g[row-1]) {
				v = g[row-1][col-1]
			}
			line = append(line, v)
		}
		out = append(out, line)
	}
	return out
}

func readDOCX(p string, in readFileIn) Out {
	z, err := zip.OpenReader(p)
	if err != nil {
		return fail("invalid", "not a docx: %v", err)
	}
	defer z.Close()
	if in.Offset == 0 {
		o, err := docx.Outline(&z.Reader)
		if err != nil {
			return errOut(err)
		}
		return ok("[DOCX outline "+p+"; read with offset>=1 for the XML that edit_block matches]\n"+o, map[string]any{"path": p})
	}
	x, err := docx.Part(&z.Reader, "word/document.xml")
	if err != nil {
		return errOut(err)
	}
	lines := strings.SplitAfter(docx.Pretty(x), "\n")
	from := min(in.Offset-1, len(lines))
	to := min(len(lines), from+in.Length)
	return ok(fmt.Sprintf("[DOCX XML %s lines %d-%d of %d]\n%s", p, from+1, to, len(lines), strings.Join(lines[from:to], "")),
		map[string]any{"path": p, "total_lines": len(lines)})
}

func readURL(ctx context.Context, e *Env, raw string) Out {
	l := e.Gate.Limits()
	named := func(h string) bool {
		for _, x := range e.Gate.P.URLHosts {
			if x != "*" && scope.HostOK(h, x) {
				return true
			}
		}
		return false
	}
	pg, err := webfetch.Fetch(ctx, raw, webfetch.Rules{HostOK: e.Gate.Host, Named: named,
		Timeout: time.Duration(l.URLTimeoutMS) * time.Millisecond, MaxBytes: l.URLMaxBytes, UserAgent: "devagotchi/" + buildinfo.Version})
	if err != nil {
		if _, perr := url.Parse(raw); perr != nil {
			return fail("invalid", "%v", perr)
		}
		if e.Gate.Need(policy.CapNet) != nil || strings.HasPrefix(err.Error(), "grant_required") {
			return errOut(err)
		}
		return fail("fetch_failed", "%v", err)
	}
	if strings.HasPrefix(pg.Mime, "image/") {
		return Out{Text: fmt.Sprintf("Image from %s (%s, %d bytes)", pg.URL, pg.Mime, len(pg.Body)), Images: []Image{{Mime: pg.Mime, Data: pg.Body}}, Data: pg}
	}
	note := ""
	if pg.Truncated {
		note = " (truncated at url_max_bytes)"
	}
	return ok(fmt.Sprintf("[%s HTTP %d %s%s]\n%s", pg.URL, pg.Status, pg.Mime, note, pg.Body), pg)
}
