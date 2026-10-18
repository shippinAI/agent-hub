package docx

import (
	"archive/zip"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateOutlineReplace(t *testing.T) {
	p := filepath.Join(t.TempDir(), "d.docx")
	if err := Create(p, "# Title\n\nHello **bold** world.\n\n| a | b |\n|---|---|\n| 1 | 2 |\n"); err != nil {
		t.Fatal(err)
	}
	z, _ := zip.OpenReader(p)
	out, err := Outline(&z.Reader)
	x, _ := Part(&z.Reader, "word/document.xml")
	z.Close()
	if err != nil || !strings.Contains(out, "[0] (Heading1) Title") || !strings.Contains(out, "| 1 | 2 |") {
		t.Fatalf("%v\n%s", err, out)
	}
	if Compact(Pretty(x)) != strings.SplitN(x, "?>", 2)[0]+"?>"+strings.SplitN(x, "?>", 2)[1] {
		t.Fatal("pretty/compact not lossless")
	}
	if _, err := Replace(p, "<w:t xml:space=\"preserve\">Title</w:t>", "<w:t xml:space=\"preserve\">New</w:t>", 1); err != nil {
		t.Fatal(err)
	}
	if _, err := Replace(p, "nope", "x", 1); err == nil || !strings.HasPrefix(err.Error(), "edit_no_match") {
		t.Fatal(err)
	}
	z, _ = zip.OpenReader(p)
	out, _ = Outline(&z.Reader)
	z.Close()
	if !strings.Contains(out, "(Heading1) New") {
		t.Fatal(out)
	}
}
