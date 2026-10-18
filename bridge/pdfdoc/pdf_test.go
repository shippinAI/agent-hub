package pdfdoc

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateReadModify(t *testing.T) {
	d := t.TempDir()
	a := filepath.Join(d, "a.pdf")
	no := func(string) (string, bool) { return "", false }
	md := "# One\n\nfirst page\n<div style=\"page-break-before: always;\"></div>\n# Two\n\nsecond\n<div style=\"page-break-before: always;\"></div>\n# Three\n\n| a | b |\n|---|---|\n| 1 | 2 |\n"
	if err := Create(a, md, no); err != nil {
		t.Fatal(err)
	}
	doc, err := Read(a, 0, 0, 2)
	if err != nil || doc.Pages != 3 || !strings.Contains(doc.Text, "second") {
		t.Fatalf("%v %+v", err, doc)
	}
	b := filepath.Join(d, "b.pdf")
	err = Modify(a, b, []Op{{Type: "delete", PageIndexes: []int{0}}, {Type: "insert", PageIndex: 1, Markdown: "# Inserted"}}, no)
	if err != nil {
		t.Fatal(err)
	}
	doc, _ = Read(b, 0, 0, 0)
	i1, i2, i3 := strings.Index(doc.Text, "Two"), strings.Index(doc.Text, "Inserted"), strings.Index(doc.Text, "Three")
	if doc.Pages != 3 || !(i1 < i2 && i2 < i3) || strings.Contains(doc.Text, "One") {
		t.Fatalf("%d pages\n%s", doc.Pages, doc.Text)
	}
	if tail, _ := Read(b, -1, 0, 0); tail.From != 2 {
		t.Fatalf("%+v", tail)
	}
}
