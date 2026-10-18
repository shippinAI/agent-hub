package xlsx

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"
)

// Sheet is one worksheet entry.
type Sheet struct {
	Name string `json:"name"`
	Path string `json:"-"`
	Rows int    `json:"rowCount"`
	Cols int    `json:"colCount"`
}

// Book is an opened workbook.
type Book struct {
	z       *zip.ReadCloser
	Sheets  []Sheet
	strings []string
}

// Open reads the workbook index and shared strings.
func Open(p string) (*Book, error) {
	z, err := zip.OpenReader(p)
	if err != nil {
		return nil, fmt.Errorf("not an xlsx workbook: %w", err)
	}
	b := &Book{z: z}
	var wb struct {
		Sheets []struct {
			Name string `xml:"name,attr"`
			RID  string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
		} `xml:"sheets>sheet"`
	}
	var rels struct {
		Rel []struct {
			ID     string `xml:"Id,attr"`
			Target string `xml:"Target,attr"`
		} `xml:"Relationship"`
	}
	if err := b.decode("xl/workbook.xml", &wb); err != nil {
		z.Close()
		return nil, err
	}
	_ = b.decode("xl/_rels/workbook.xml.rels", &rels)
	for _, s := range wb.Sheets {
		for _, r := range rels.Rel {
			if r.ID == s.RID {
				t := strings.TrimPrefix(r.Target, "/")
				if !strings.HasPrefix(t, "xl/") {
					t = path.Join("xl", t)
				}
				b.Sheets = append(b.Sheets, Sheet{Name: s.Name, Path: t})
			}
		}
	}
	var sst struct {
		SI []struct {
			T string   `xml:"t"`
			R []string `xml:"r>t"`
		} `xml:"si"`
	}
	_ = b.decode("xl/sharedStrings.xml", &sst)
	for _, si := range sst.SI {
		b.strings = append(b.strings, si.T+strings.Join(si.R, ""))
	}
	return b, nil
}

// Close releases the file.
func (b *Book) Close() error { return b.z.Close() }

func (b *Book) decode(name string, v any) error {
	f, err := b.z.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()
	return xml.NewDecoder(io.LimitReader(f, 256<<20)).Decode(v)
}

// Find resolves a sheet by name or 0-based index string ("" = first).
func (b *Book) Find(name string) (Sheet, error) {
	if name == "" && len(b.Sheets) > 0 {
		return b.Sheets[0], nil
	}
	for _, s := range b.Sheets {
		if s.Name == name {
			return s, nil
		}
	}
	if i, err := strconv.Atoi(name); err == nil && i >= 0 && i < len(b.Sheets) {
		return b.Sheets[i], nil
	}
	return Sheet{}, fmt.Errorf("no sheet %q", name)
}
