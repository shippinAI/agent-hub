package pdfdoc

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/go-pdf/fpdf"

	"devagotchi/bridge/markdown"
)

var headSize = []float64{0, 22, 18, 15, 13, 12, 11}

// Create renders markdown to a new PDF at path (A4, core fonts, cp1252 text; other characters become "?").
// imageOK confines embedded image paths to the policy roots.
func Create(path, md string, imageOK func(string) (string, bool)) error {
	p := fpdf.New("P", "mm", "A4", "")
	p.SetMargins(18, 18, 18)
	p.SetAutoPageBreak(true, 18)
	p.AddPage()
	tr := p.UnicodeTranslatorFromDescriptor("")
	write := func(text string, size float64) {
		for _, s := range markdown.Spans(text) {
			style, fam := "", "Helvetica"
			if s.Bold {
				style += "B"
			}
			if s.Italic {
				style += "I"
			}
			if s.Code {
				fam = "Courier"
			}
			p.SetFont(fam, style, size)
			p.Write(size*0.5, tr(s.Text))
		}
		p.Ln(size * 0.7)
	}
	for _, b := range markdown.Parse(md) {
		switch b.Kind {
		case markdown.Heading:
			p.Ln(2)
			write("**"+markdown.Plain(b.Text)+"**", headSize[b.Level])
		case markdown.Bullet, markdown.Numbered:
			p.SetX(24)
			write("- "+b.Text, 11)
		case markdown.Code:
			p.SetFont("Courier", "", 9.5)
			p.SetFillColor(242, 242, 242)
			p.MultiCell(0, 4.5, tr(b.Text), "", "L", true)
			p.Ln(2)
		case markdown.Table:
			table(p, tr, b.Rows)
		case markdown.PageBreak:
			p.AddPage()
		case markdown.Rule:
			y := p.GetY() + 2
			p.Line(18, y, 192, y)
			p.Ln(5)
		case markdown.Image:
			if real, ok := imageOK(b.Text); ok {
				p.ImageOptions(real, 18, p.GetY(), 120, 0, true, fpdf.ImageOptions{ReadDpi: true}, 0, "")
				p.Ln(2)
			} else {
				write("*[image not allowed or missing: "+b.Text+"]*", 10)
			}
		default:
			write(b.Text, 11)
			p.Ln(1.5)
		}
	}
	if err := p.Error(); err != nil {
		return err
	}
	tmp := filepath.Join(filepath.Dir(path), ".devagotchi-"+filepath.Base(path))
	if err := p.OutputFileAndClose(tmp); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func table(p *fpdf.Fpdf, tr func(string) string, rows [][]string) {
	if len(rows) == 0 {
		return
	}
	w := 174 / float64(len(rows[0]))
	for i, r := range rows {
		p.SetFont("Helvetica", map[bool]string{true: "B"}[i == 0], 10)
		for _, c := range r {
			p.CellFormat(w, 7, tr(strings.TrimSpace(markdown.Plain(c))), "1", 0, "L", false, 0, "")
		}
		p.Ln(-1)
	}
	p.Ln(3)
}
