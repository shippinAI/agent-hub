package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteReadEditHash(t *testing.T) {
	e, root := newEnv(t)
	f := filepath.Join(root, "a.txt")
	must(t, call(t, e, "write_file", map[string]any{"path": f, "content": "one\ntwo\nthree\n"}))
	must(t, call(t, e, "read_file", map[string]any{"path": f, "offset": -1}), "three")
	sha := data(call(t, e, "get_file_info", map[string]any{"path": f}))["sha256"].(string)
	code(t, call(t, e, "edit_block", map[string]any{"file_path": f, "old_string": "two", "new_string": "2", "expect_sha256": "00"}), "hash_mismatch")
	must(t, call(t, e, "edit_block", map[string]any{"file_path": f, "old_string": "two", "new_string": "2", "expect_sha256": sha}))
	b, _ := os.ReadFile(f)
	if string(b) != "one\n2\nthree\n" {
		t.Fatalf("got %q", b)
	}
	o := call(t, e, "edit_block", map[string]any{"file_path": f, "old_string": "thre", "new_string": "x", "expected_replacements": 2})
	if o.Code == "" {
		t.Fatal("expected count mismatch")
	}
	nm := call(t, e, "edit_block", map[string]any{"file_path": f, "old_string": "Three", "new_string": "x"})
	if nm.Code == "" || !strings.Contains(nm.Text, "line 3") {
		t.Fatalf("near-miss: %s", nm.Text)
	}
	must(t, call(t, e, "write_file", map[string]any{"path": f, "content": "four\n", "mode": "append"}))
	must(t, call(t, e, "read_multiple_files", map[string]any{"paths": []string{f, filepath.Join(root, "nope")}}), "four")
}

func TestGateRefusals(t *testing.T) {
	e, root := newEnv(t)
	code(t, call(t, e, "read_file", map[string]any{"path": "/etc/hostname"}), "grant_required")
	code(t, call(t, e, "write_file", map[string]any{"path": filepath.Join(root, "../x"), "content": "x"}), "grant_required")
	os.Symlink("/etc", filepath.Join(root, "esc"))
	code(t, call(t, e, "read_file", map[string]any{"path": filepath.Join(root, "esc/hostname")}), "grant_required")
	code(t, call(t, e, "set_config_value", map[string]any{"key": "roots", "value": []string{"/"}}), "policy_denied")
	code(t, call(t, e, "read_file", map[string]any{"path": "https://example.com", "isUrl": true}), "policy_denied")
	code(t, call(t, e, "kill_process", map[string]any{"pid": os.Getpid()}), "policy_denied")
	code(t, call(t, e, "delete_file", map[string]any{"path": filepath.Join(root, "x")}), "invalid")
}

func TestDirsMoveDelete(t *testing.T) {
	e, root := newEnv(t)
	d := filepath.Join(root, "d/e")
	must(t, call(t, e, "create_directory", map[string]any{"path": d}))
	os.WriteFile(filepath.Join(d, "f.go"), []byte("package f\n"), 0o644)
	must(t, call(t, e, "list_directory", map[string]any{"path": root, "depth": 3}), "[DIR]", "[FILE]", "f.go")
	dst := filepath.Join(root, "g.go")
	must(t, call(t, e, "move_file", map[string]any{"source": filepath.Join(d, "f.go"), "destination": dst}))
	sha := data(call(t, e, "get_file_info", map[string]any{"path": dst}))["sha256"]
	must(t, call(t, e, "delete_file", map[string]any{"path": dst, "expect_sha256": sha}))
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Fatal("not deleted")
	}
}

func TestOfficeAndPDF(t *testing.T) {
	e, root := newEnv(t)
	x := filepath.Join(root, "b.xlsx")
	must(t, call(t, e, "write_file", map[string]any{"path": x, "content": `[["name","n"],["a",1],["b",2]]`}))
	must(t, call(t, e, "read_file", map[string]any{"path": x}), "name")
	must(t, call(t, e, "edit_block", map[string]any{"file_path": x, "range": "Sheet1!B2", "content": [][]any{{42}}}))
	must(t, call(t, e, "read_file", map[string]any{"path": x, "range": "Sheet1!B2"}), "42")
	dx := filepath.Join(root, "c.docx")
	must(t, call(t, e, "write_file", map[string]any{"path": dx, "content": "# Title\n\nHello world"}))
	must(t, call(t, e, "read_file", map[string]any{"path": dx}), "Hello world")
	pdf := filepath.Join(root, "d.pdf")
	must(t, call(t, e, "write_pdf", map[string]any{"path": pdf, "content": "# Report\n\nBody text"}))
	must(t, call(t, e, "read_file", map[string]any{"path": pdf}), "Body text")
	out := filepath.Join(root, "e.pdf")
	must(t, call(t, e, "write_pdf", map[string]any{"path": pdf, "outputPath": out,
		"content": []map[string]any{{"type": "insert", "pageIndex": 1, "markdown": "# Appendix"}}}), "2 pages")
}
