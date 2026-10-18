package docx

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// Parts lists the XML parts edits may touch: the body first, then headers and footers.
func Parts(z *zip.Reader) []string {
	out := []string{"word/document.xml"}
	for _, f := range z.File {
		if strings.HasPrefix(f.Name, "word/header") || strings.HasPrefix(f.Name, "word/footer") {
			out = append(out, f.Name)
		}
	}
	return out
}

// Part reads one zip part.
func Part(z *zip.Reader, name string) (string, error) {
	f, err := z.Open(name)
	if err != nil {
		return "", err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, 128<<20))
	return string(b), err
}

// Outline renders body elements with their index: paragraphs with style and text, tables with cell text,
// and image references. Shapes and drawings without text are skipped (as DC does).
func Outline(z *zip.Reader) (string, error) {
	doc, err := Part(z, "word/document.xml")
	if err != nil {
		return "", err
	}
	d := xml.NewDecoder(strings.NewReader(doc))
	var b strings.Builder
	idx, depth, inBody := 0, 0, false
	var para, style strings.Builder
	var cells []string
	var row, trows []string
	inTable, images := 0, 0
	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		} else if err != nil {
			return b.String(), err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			switch t.Name.Local {
			case "body":
				inBody = true
			case "tbl":
				inTable++
			case "p":
				para.Reset()
				style.Reset()
				images = 0
			case "pStyle":
				style.WriteString(attrVal(t, "val"))
			case "blip":
				images++
			case "tab":
				para.WriteString("\t")
			}
		case xml.CharData:
			if inBody {
				para.Write(t)
			}
		case xml.EndElement:
			depth--
			switch t.Name.Local {
			case "p":
				txt := strings.TrimSpace(para.String())
				if inTable > 0 {
					cells = append(cells, txt)
				} else if txt != "" || images > 0 {
					fmt.Fprintf(&b, "[%d]%s %s%s\n", idx, styleTag(style.String()), txt, strings.Repeat(" [image]", images))
					idx++
				}
			case "tc":
				row = append(row, strings.Join(cells, " "))
				cells = nil
			case "tr":
				trows = append(trows, "    | "+strings.Join(row, " | ")+" |")
				row = nil
			case "tbl":
				inTable--
				if inTable == 0 {
					fmt.Fprintf(&b, "[%d] TABLE %d rows\n%s\n", idx, len(trows), strings.Join(trows, "\n"))
					trows = nil
					idx++
				}
			}
		}
	}
	return b.String(), nil
}

func styleTag(s string) string {
	if s == "" {
		return ""
	}
	return " (" + s + ")"
}

func attrVal(t xml.StartElement, local string) string {
	for _, a := range t.Attr {
		if a.Name.Local == local {
			return a.Value
		}
	}
	return ""
}
