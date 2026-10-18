package xlsx

import (
	"archive/zip"
	"encoding/xml"
	"io"
	"sort"
)

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
