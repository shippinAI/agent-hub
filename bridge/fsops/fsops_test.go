package fsops

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteEditDelete(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "f.txt")
	r := Do(Op{Kind: "write", Path: p, Content: "hello world", ExpectAbsent: true})
	if r.Error != "" {
		t.Fatal(r)
	}
	if Do(Op{Kind: "write", Path: p, Content: "x", ExpectAbsent: true}).Error != "hash_mismatch" {
		t.Fatal("expect_absent on existing file")
	}
	if Do(Op{Kind: "write", Path: p, Content: "x", ExpectSHA256: "deadbeef"}).Error != "hash_mismatch" {
		t.Fatal("stale sha")
	}
	if Do(Op{Kind: "edit", Path: p, ExpectSHA256: r.SHA256, Edits: []Edit{{Old: "world", New: "there"}}}).Applied != 1 {
		t.Fatal("edit")
	}
	if b, _ := os.ReadFile(p); string(b) != "hello there" {
		t.Fatalf("content %q", b)
	}
	if Do(Op{Kind: "edit", Path: p, Edits: []Edit{{Old: "l", New: "L"}}}).Error != "edit_ambiguous" {
		t.Fatal("ambiguous")
	}
	if Do(Op{Kind: "edit", Path: p, Edits: []Edit{{Old: "zzz", New: "x"}}}).Error != "edit_no_match" {
		t.Fatal("no match")
	}
	cur := Do(Op{Kind: "read", Path: p})
	if Do(Op{Kind: "delete", Path: p}).Error != "invalid" {
		t.Fatal("delete needs sha")
	}
	if Do(Op{Kind: "delete", Path: p, ExpectSHA256: cur.SHA256}).Error != "" {
		t.Fatal("delete")
	}
	if _, err := os.Stat(p); err == nil {
		t.Fatal("file still there")
	}
}
