// Package pdfdoc reads PDFs as page-tagged text (plus embedded JPEG/PNG images), creates PDFs from markdown
// without a browser (DC downloads Chrome for this, #797), and deletes/inserts pages. Pure Go, no cgo.
package pdfdoc

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/ledongthuc/pdf"
	"github.com/pdfcpu/pdfcpu/pkg/api"
)

func init() { api.DisableConfigDir() } // pdfcpu must not create ~/.config/pdfcpu

// Img is one extracted image.
type Img struct {
	Page int
	Mime string
	Data []byte
}

// Doc is a read result.
type Doc struct {
	Pages    int
	From, To int // 0-based page window returned
	Text     string
	Images   []Img
}

// Read returns pages [offset, offset+length) as markdown text ("## Page N" headers). A negative offset
// reads the last -offset pages. Up to maxImages embedded JPEG/PNG images of those pages are returned.
func Read(path string, offset, length, maxImages int) (Doc, error) {
	f, r, err := pdf.Open(path)
	if err != nil {
		return Doc{}, fmt.Errorf("cannot parse PDF: %w", err)
	}
	defer f.Close()
	d := Doc{Pages: r.NumPage()}
	if offset < 0 {
		offset, length = max(0, d.Pages+offset), -offset
	}
	if length <= 0 {
		length = d.Pages
	}
	d.From, d.To = min(offset, d.Pages), min(d.Pages, offset+length)
	var b strings.Builder
	for i := d.From; i < d.To; i++ {
		txt, err := r.Page(i + 1).GetPlainText(nil)
		if err != nil {
			txt = "(text could not be extracted from this page: " + err.Error() + ")"
		}
		fmt.Fprintf(&b, "## Page %d\n\n%s\n\n", i+1, strings.TrimSpace(txt))
	}
	d.Text = b.String()
	if maxImages > 0 && d.To > d.From {
		d.Images = images(path, d.From+1, d.To, maxImages)
	}
	return d, nil
}

func images(path string, from, to, max int) []Img {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	sel := []string{strconv.Itoa(from) + "-" + strconv.Itoa(to)}
	pages, err := api.ExtractImagesRaw(context.Background(), f, sel, nil)
	if err != nil {
		return nil
	}
	var out []Img
	for _, m := range pages {
		for _, im := range m {
			mime := map[string]string{"jpg": "image/jpeg", "png": "image/png"}[im.FileType]
			if mime == "" || len(out) >= max {
				continue
			}
			var buf bytes.Buffer
			if _, err := io.Copy(&buf, io.LimitReader(im, 8<<20)); err == nil {
				out = append(out, Img{Page: im.PageNr, Mime: mime, Data: buf.Bytes()})
			}
		}
	}
	return out
}

// PageCount returns the number of pages.
func PageCount(path string) (int, error) { return api.PageCountFile(context.Background(), path) }
