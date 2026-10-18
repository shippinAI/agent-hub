package docx

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Replace swaps old for new in the pretty-printed XML of the body (then headers/footers when the body has
// no match). old must match exactly count times in that part, or nothing is written.
func Replace(path, old, new string, count int) (string, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return "", fmt.Errorf("not a docx: %w", err)
	}
	defer zr.Close()
	part, pretty := "", ""
	for _, name := range Parts(&zr.Reader) {
		x, err := Part(&zr.Reader, name)
		if err != nil {
			continue
		}
		p := Pretty(x)
		n := strings.Count(p, old)
		if n == 0 {
			continue
		}
		if n != count {
			return "", fmt.Errorf("edit_ambiguous: matched %d times in %s, expected %d", n, name, count)
		}
		part, pretty = name, strings.ReplaceAll(p, old, new)
		break
	}
	if part == "" {
		return "", fmt.Errorf("edit_no_match: text not found in the document XML (copy it from read_file with offset/length)")
	}
	return part, rewrite(path, &zr.Reader, map[string]string{part: Compact(pretty)})
}

// rewrite copies the package with some parts replaced, via temp file + rename.
func rewrite(path string, zr *zip.Reader, repl map[string]string) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".devagotchi-*.docx")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	z := zip.NewWriter(tmp)
	for _, f := range zr.File {
		if body, ok := repl[f.Name]; ok {
			w, err := z.CreateHeader(&zip.FileHeader{Name: f.Name, Method: zip.Deflate, Modified: f.Modified})
			if err != nil {
				return err
			}
			if _, err := io.WriteString(w, body); err != nil {
				return err
			}
			continue
		}
		r, err := f.OpenRaw()
		if err != nil {
			return err
		}
		w, err := z.CreateRaw(&f.FileHeader)
		if err != nil {
			return err
		}
		if _, err := io.Copy(w, r); err != nil {
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
	return os.Rename(tmp.Name(), path)
}
