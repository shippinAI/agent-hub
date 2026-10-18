package docx

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"devagotchi/bridge/markdown"
)

const nsW = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"

var esc = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")

// Create writes a new .docx from markdown: headings use Heading1-6 styles, lists, code, tables and page breaks.
func Create(path, md string) error {
	var body strings.Builder
	for _, b := range markdown.Parse(md) {
		switch b.Kind {
		case markdown.Heading:
			body.WriteString(para(fmt.Sprintf("Heading%d", b.Level), runs(b.Text)))
		case markdown.Bullet:
			body.WriteString(para("ListParagraph", run("• ", false, false, false)+runs(b.Text)))
		case markdown.Numbered:
			body.WriteString(para("ListParagraph", runs(b.Text)))
		case markdown.Code:
			for _, l := range strings.Split(b.Text, "\n") {
				body.WriteString(para("Code", run(l, false, false, true)))
			}
		case markdown.Table:
			body.WriteString(table(b.Rows))
		case markdown.PageBreak:
			body.WriteString(`<w:p><w:r><w:br w:type="page"/></w:r></w:p>`)
		case markdown.Image:
			body.WriteString(para("", run("[image: "+b.Text+"]", false, true, false)))
		case markdown.Rule:
			body.WriteString(`<w:p><w:pPr><w:pBdr><w:bottom w:val="single" w:sz="6" w:space="1" w:color="auto"/></w:pBdr></w:pPr></w:p>`)
		default:
			body.WriteString(para("", runs(b.Text)))
		}
	}
	parts := [][2]string{
		{"[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/><Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/></Types>`},
		{"_rels/.rels", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`},
		{"word/_rels/document.xml.rels", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/></Relationships>`},
		{"word/document.xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><w:document xmlns:w="` + nsW + `"><w:body>` + body.String() + `<w:sectPr/></w:body></w:document>`},
		{"word/styles.xml", styles()},
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".devagotchi-*.docx")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	z := zip.NewWriter(tmp)
	for _, p := range parts {
		w, err := z.Create(p[0])
		if err != nil {
			return err
		}
		if _, err := io.WriteString(w, p[1]); err != nil {
			return err
		}
	}
	if err := z.Close(); err != nil {
		return err
	}
	tmp.Close()
	return os.Rename(tmp.Name(), path)
}

func para(style, runs string) string {
	pPr := ""
	if style != "" {
		pPr = `<w:pPr><w:pStyle w:val="` + style + `"/></w:pPr>`
	}
	return `<w:p>` + pPr + runs + `</w:p>`
}

func runs(text string) string {
	var b strings.Builder
	for _, s := range markdown.Spans(text) {
		b.WriteString(run(s.Text, s.Bold, s.Italic, s.Code))
	}
	return b.String()
}

func run(t string, bold, italic, code bool) string {
	rPr := ""
	if bold || italic || code {
		rPr = "<w:rPr>" + map[bool]string{true: "<w:b/>"}[bold] + map[bool]string{true: "<w:i/>"}[italic] +
			map[bool]string{true: `<w:rFonts w:ascii="Courier New" w:hAnsi="Courier New"/>`}[code] + "</w:rPr>"
	}
	return `<w:r>` + rPr + `<w:t xml:space="preserve">` + esc.Replace(t) + `</w:t></w:r>`
}

func table(rows [][]string) string {
	var b strings.Builder
	b.WriteString(`<w:tbl><w:tblPr><w:tblStyle w:val="TableGrid"/><w:tblW w:w="0" w:type="auto"/><w:tblBorders>`)
	for _, s := range []string{"top", "left", "bottom", "right", "insideH", "insideV"} {
		b.WriteString(`<w:` + s + ` w:val="single" w:sz="4" w:space="0" w:color="auto"/>`)
	}
	b.WriteString(`</w:tblBorders></w:tblPr>`)
	for i, r := range rows {
		b.WriteString(`<w:tr>`)
		for _, c := range r {
			b.WriteString(`<w:tc><w:p>` + run(markdown.Plain(c), i == 0, false, false) + `</w:p></w:tc>`)
		}
		b.WriteString(`</w:tr>`)
	}
	return b.String() + `</w:tbl>`
}

func styles() string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><w:styles xmlns:w="` + nsW + `"><w:style w:type="paragraph" w:default="1" w:styleId="Normal"><w:name w:val="Normal"/><w:rPr><w:sz w:val="22"/></w:rPr></w:style>`)
	for i, sz := range []int{36, 30, 26, 24, 22, 22} {
		fmt.Fprintf(&b, `<w:style w:type="paragraph" w:styleId="Heading%d"><w:name w:val="heading %d"/><w:basedOn w:val="Normal"/><w:pPr><w:keepNext/><w:outlineLvl w:val="%d"/></w:pPr><w:rPr><w:b/><w:sz w:val="%d"/></w:rPr></w:style>`, i+1, i+1, i, sz)
	}
	b.WriteString(`<w:style w:type="paragraph" w:styleId="ListParagraph"><w:name w:val="List Paragraph"/><w:basedOn w:val="Normal"/><w:pPr><w:ind w:left="720"/></w:pPr></w:style><w:style w:type="paragraph" w:styleId="Code"><w:name w:val="Code"/><w:basedOn w:val="Normal"/><w:pPr><w:spacing w:after="0"/></w:pPr><w:rPr><w:rFonts w:ascii="Courier New" w:hAnsi="Courier New"/><w:sz w:val="20"/></w:rPr></w:style></w:styles>`)
	return b.String()
}
