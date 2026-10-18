package xlsx

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// NamedGrid is one sheet to write.
type NamedGrid struct {
	Name string
	Rows [][]any
}

const (
	nsMain = "http://schemas.openxmlformats.org/spreadsheetml/2006/main"
	nsRel  = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
	nsPkg  = "http://schemas.openxmlformats.org/package/2006/relationships"
)

// Write creates a new workbook at p (via a temp file + rename). Strings starting with "=" become formulas.
func Write(p string, sheets []NamedGrid) error {
	if len(sheets) == 0 {
		return fmt.Errorf("no sheets to write")
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), ".devagotchi-*.xlsx")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	z := zip.NewWriter(tmp)
	var ct, wb, rels strings.Builder
	ct.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>`)
	wb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><workbook xmlns="` + nsMain + `" xmlns:r="` + nsRel + `"><sheets>`)
	rels.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="` + nsPkg + `">`)
	for i, s := range sheets {
		n := strconv.Itoa(i + 1)
		ct.WriteString(`<Override PartName="/xl/worksheets/sheet` + n + `.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>`)
		wb.WriteString(`<sheet name="` + esc(s.Name) + `" sheetId="` + n + `" r:id="rId` + n + `"/>`)
		rels.WriteString(`<Relationship Id="rId` + n + `" Type="` + nsRel + `/worksheet" Target="worksheets/sheet` + n + `.xml"/>`)
		if err := put(z, "xl/worksheets/sheet"+n+".xml", sheetXML(s.Rows)); err != nil {
			return err
		}
	}
	rels.WriteString(`<Relationship Id="rIdS" Type="` + nsRel + `/styles" Target="styles.xml"/></Relationships>`)
	parts := map[string]string{
		"[Content_Types].xml":        ct.String() + `</Types>`,
		"_rels/.rels":                `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="` + nsPkg + `"><Relationship Id="rId1" Type="` + nsRel + `/officeDocument" Target="xl/workbook.xml"/></Relationships>`,
		"xl/workbook.xml":            wb.String() + `</sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": rels.String(),
		"xl/styles.xml":              `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><styleSheet xmlns="` + nsMain + `"><fonts count="1"><font><sz val="11"/><name val="Calibri"/></font></fonts><fills count="1"><fill><patternFill patternType="none"/></fill></fills><borders count="1"><border/></borders><cellStyleXfs count="1"><xf/></cellStyleXfs><cellXfs count="1"><xf xfId="0"/></cellXfs><cellStyles count="1"><cellStyle name="Normal" xfId="0" builtinId="0"/></cellStyles></styleSheet>`,
	}
	for _, name := range []string{"[Content_Types].xml", "_rels/.rels", "xl/workbook.xml", "xl/_rels/workbook.xml.rels", "xl/styles.xml"} {
		if err := put(z, name, parts[name]); err != nil {
			return err
		}
	}
	if err := z.Close(); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	tmp.Close()
	return os.Rename(tmp.Name(), p)
}

func sheetXML(rows [][]any) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><worksheet xmlns="` + nsMain + `"><sheetData>`)
	for r, row := range rows {
		b.WriteString(`<row r="` + strconv.Itoa(r+1) + `">`)
		for c, v := range row {
			b.WriteString(Cell(Ref{Col: c + 1, Row: r + 1}, v, ""))
		}
		b.WriteString(`</row>`)
	}
	b.WriteString(`</sheetData></worksheet>`)
	return b.String()
}

// Cell renders one <c> element; attrs carries extra attributes to keep (style).
func Cell(ref Ref, v any, attrs string) string {
	r := `<c r="` + ref.String() + `"` + attrs
	switch x := v.(type) {
	case nil:
		return r + `/>`
	case bool:
		return r + ` t="b"><v>` + map[bool]string{true: "1", false: "0"}[x] + `</v></c>`
	case float64:
		return r + `><v>` + strconv.FormatFloat(x, 'f', -1, 64) + `</v></c>`
	case int:
		return r + `><v>` + strconv.Itoa(x) + `</v></c>`
	case string:
		if strings.HasPrefix(x, "=") && len(x) > 1 {
			return r + `><f>` + esc(x[1:]) + `</f></c>`
		}
		return r + ` t="inlineStr"><is><t xml:space="preserve">` + esc(x) + `</t></is></c>`
	}
	return r + ` t="inlineStr"><is><t>` + esc(fmt.Sprint(v)) + `</t></is></c>`
}

func put(z *zip.Writer, name, body string) error {
	w, err := z.Create(name)
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, body)
	return err
}

var xmlEsc = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")

func esc(s string) string { return xmlEsc.Replace(s) }
