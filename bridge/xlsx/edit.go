package xlsx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type xcell struct {
	Attrs []xml.Attr `xml:",any,attr"`
	Inner string     `xml:",innerxml"`
}

type xrow struct {
	Attrs []xml.Attr `xml:",any,attr"`
	Cells []xcell    `xml:"c"`
}

var (
	reSheetData = regexp.MustCompile(`(?s)<sheetData\s*/>|<sheetData[^>]*>.*</sheetData>`)
	reCalcRel   = regexp.MustCompile(`<Relationship [^>]*calcChain[^>]*/>`)
	reCalcCT    = regexp.MustCompile(`<Override [^>]*calcChain[^>]*/>`)
)

// SetRange writes values into rng of an existing workbook, keeping every other part, cell styles and
// formulas outside the range. The calc chain is dropped so Excel rebuilds it.
func SetRange(p string, rng Range, values [][]any) error {
	b, err := Open(p)
	if err != nil {
		return err
	}
	sh, err := b.Find(rng.Sheet)
	if err != nil {
		b.Close()
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), ".devagotchi-*.xlsx")
	if err != nil {
		b.Close()
		return err
	}
	defer os.Remove(tmp.Name())
	z := zip.NewWriter(tmp)
	for _, f := range b.z.File {
		switch f.Name {
		case "xl/calcChain.xml":
			continue
		case sh.Path, "[Content_Types].xml", "xl/_rels/workbook.xml.rels":
			raw, err := readAll(f)
			if err == nil && f.Name == sh.Path {
				raw, err = editSheet(raw, rng, values)
			} else if err == nil {
				raw = reCalcCT.ReplaceAll(reCalcRel.ReplaceAll(raw, nil), nil)
			}
			if err != nil {
				b.Close()
				return err
			}
			if err := put(z, f.Name, string(raw)); err != nil {
				b.Close()
				return err
			}
		default:
			if err := copyRaw(z, f); err != nil {
				b.Close()
				return err
			}
		}
	}
	b.Close()
	if err := z.Close(); err != nil {
		return err
	}
	tmp.Close()
	return os.Rename(tmp.Name(), p)
}

func editSheet(raw []byte, rng Range, values [][]any) ([]byte, error) {
	loc := reSheetData.FindIndex(raw)
	if loc == nil {
		return nil, fmt.Errorf("sheet has no sheetData")
	}
	var sd struct {
		Rows []xrow `xml:"row"`
	}
	if err := xml.Unmarshal(raw[loc[0]:loc[1]], &sd); err != nil {
		return nil, err
	}
	rows := map[int]map[int]string{}
	rowAttrs := map[int]string{}
	for i, r := range sd.Rows {
		rn := i + 1
		if v := attr(r.Attrs, "r"); v != "" {
			rn, _ = strconv.Atoi(v)
		}
		rowAttrs[rn] = attrs(r.Attrs, "r")
		rows[rn] = map[int]string{}
		for ci, c := range r.Cells {
			ref := Ref{Col: ci + 1, Row: rn}
			if v := attr(c.Attrs, "r"); v != "" {
				ref, _ = ParseRef(v)
			}
			rows[rn][ref.Col] = `<c r="` + ref.String() + `"` + attrs(c.Attrs, "r") + `>` + c.Inner + `</c>`
		}
	}
	for dr, vals := range values {
		rn := rng.From.Row + dr
		if rows[rn] == nil {
			rows[rn] = map[int]string{}
		}
		for dc, v := range vals {
			ref := Ref{Col: rng.From.Col + dc, Row: rn}
			style := ""
			if old, ok := rows[rn][ref.Col]; ok {
				if m := regexp.MustCompile(` s="\d+"`).FindString(old[:strings.Index(old, ">")]); m != "" {
					style = m
				}
			}
			rows[rn][ref.Col] = Cell(ref, v, style)
		}
	}
	var out bytes.Buffer
	out.WriteString("<sheetData>")
	for _, rn := range sortedKeys(rows) {
		out.WriteString(`<row r="` + strconv.Itoa(rn) + `"` + rowAttrs[rn] + `>`)
		for _, cn := range sortedKeys(rows[rn]) {
			out.WriteString(rows[rn][cn])
		}
		out.WriteString("</row>")
	}
	out.WriteString("</sheetData>")
	return append(append(append([]byte{}, raw[:loc[0]]...), out.Bytes()...), raw[loc[1]:]...), nil
}

func attr(as []xml.Attr, name string) string {
	for _, a := range as {
		if a.Name.Space == "" && a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}

// attrs re-renders un-namespaced attributes except skip (namespaced extras like x14ac:dyDescent are dropped).
func attrs(as []xml.Attr, skip string) string {
	s := ""
	for _, a := range as {
		if a.Name.Space == "" && a.Name.Local != skip {
			s += ` ` + a.Name.Local + `="` + esc(a.Value) + `"`
		}
	}
	return s
}

func sortedKeys[V any](m map[int]V) []int {
	k := make([]int, 0, len(m))
	for n := range m {
		k = append(k, n)
	}
	sort.Ints(k)
	return k
}

func readAll(f *zip.File) ([]byte, error) {
	r, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(io.LimitReader(r, 256<<20))
}

func copyRaw(z *zip.Writer, f *zip.File) error {
	r, err := f.OpenRaw()
	if err != nil {
		return err
	}
	w, err := z.CreateRaw(&f.FileHeader)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, r)
	return err
}
