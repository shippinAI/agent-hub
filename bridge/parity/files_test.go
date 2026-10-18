package parity

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// TestParityFiles covers matrix rows R01-R15 (config, filesystem, office/PDF, edits) over stdio.
func TestParityFiles(t *testing.T) {
	web := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, "hello from the web") }))
	defer web.Close()
	cfg, root := fixture(t, "url_hosts = [\"127.0.0.1\"]\n")
	cs := connect(t, cfg)
	p := func(n string) string { return filepath.Join(root, n) }
	var sha string
	steps := []struct {
		id string
		fn func(t *testing.T)
	}{
		{"R01 get_config", func(t *testing.T) { want(t, callTool(t, cs, "get_config", nil), "", root, "telemetry: false", "bubblewrap") }},
		{"R02 set_config_value", func(t *testing.T) {
			want(t, callTool(t, cs, "set_config_value", map[string]any{"key": "roots", "value": []string{"/"}}), "policy_denied", "devagotchi config set")
		}},
		{"R03 read_file text+tail", func(t *testing.T) {
			r := want(t, callTool(t, cs, "read_file", map[string]any{"path": p("notes.txt"), "offset": 10, "length": 5}), "", "line 11", "line 15")
			want(t, callTool(t, cs, "read_file", map[string]any{"path": p("notes.txt"), "offset": -2}), "", "line 50")
			sha, _ = r.data["sha256"].(string)
		}},
		{"R04 read_file url", func(t *testing.T) {
			want(t, callTool(t, cs, "read_file", map[string]any{"path": web.URL, "isUrl": true}), "", "hello from the web")
		}},
		{"R05 read_file image", func(t *testing.T) {
			if r := want(t, callTool(t, cs, "read_file", map[string]any{"path": p("pic.png")}), ""); r.imgs != 1 {
				t.Fatalf("want 1 image, got %d", r.imgs)
			}
		}},
		{"R06 read_multiple_files", func(t *testing.T) {
			want(t, callTool(t, cs, "read_multiple_files", map[string]any{"paths": []string{p("notes.txt"), p("pic.png"), p("missing")}}), "", "line 1", "missing")
		}},
		{"R07 write_file text+hash", func(t *testing.T) {
			want(t, callTool(t, cs, "write_file", map[string]any{"path": p("notes.txt"), "content": "x", "expect_sha256": "deadbeef"}), "hash_mismatch")
			want(t, callTool(t, cs, "write_file", map[string]any{"path": p("notes.txt"), "content": "line 51\n", "mode": "append", "expect_sha256": sha}), "")
		}},
		{"R08 write_file+read_file xlsx", func(t *testing.T) {
			want(t, callTool(t, cs, "write_file", map[string]any{"path": p("t.xlsx"), "content": `{"Data":[["k","v"],["a",1]]}`}), "")
			want(t, callTool(t, cs, "read_file", map[string]any{"path": p("t.xlsx"), "sheet": "Data"}), "", "k", "a")
		}},
		{"R09 write_file+read_file docx", func(t *testing.T) {
			want(t, callTool(t, cs, "write_file", map[string]any{"path": p("t.docx"), "content": "# Plan\n\nShip it"}), "")
			want(t, callTool(t, cs, "read_file", map[string]any{"path": p("t.docx")}), "", "Ship it")
		}},
		{"R10 write_pdf create+modify, read pdf", func(t *testing.T) {
			want(t, callTool(t, cs, "write_pdf", map[string]any{"path": p("r.pdf"), "content": "# Report\n\n| a | b |\n|---|---|\n| 1 | 2 |"}), "")
			want(t, callTool(t, cs, "write_pdf", map[string]any{"path": p("r.pdf"), "outputPath": p("r2.pdf"),
				"content": []map[string]any{{"type": "insert", "pageIndex": 0, "markdown": "# Cover"}}}), "", "2 pages")
			want(t, callTool(t, cs, "read_file", map[string]any{"path": p("r2.pdf")}), "", "Cover", "Report")
		}},
		{"R11 create_directory", func(t *testing.T) { want(t, callTool(t, cs, "create_directory", map[string]any{"path": p("a/b/c")}), "") }},
		{"R12 list_directory", func(t *testing.T) {
			want(t, callTool(t, cs, "list_directory", map[string]any{"path": root, "depth": 2}), "", "[DIR] sub", "[FILE] notes.txt")
		}},
		{"R13 move_file", func(t *testing.T) {
			want(t, callTool(t, cs, "move_file", map[string]any{"source": p("t.docx"), "destination": p("a/t.docx")}), "")
			want(t, callTool(t, cs, "move_file", map[string]any{"source": p("a/t.docx"), "destination": "/tmp/escape.docx"}), "grant_required")
		}},
		{"R14 get_file_info", func(t *testing.T) {
			want(t, callTool(t, cs, "get_file_info", map[string]any{"path": p("notes.txt")}), "", "sha256", "lineCount")
		}},
		{"R15 edit_block text+xlsx+docx", func(t *testing.T) {
			want(t, callTool(t, cs, "edit_block", map[string]any{"file_path": p("notes.txt"), "old_string": "line 2\n", "new_string": "LINE 2\n"}), "")
			want(t, callTool(t, cs, "edit_block", map[string]any{"file_path": p("notes.txt"), "old_string": "line 9x", "new_string": "?"}), "edit_no_match", "Closest")
			want(t, callTool(t, cs, "edit_block", map[string]any{"file_path": p("t.xlsx"), "range": "Data!B2", "content": [][]any{{7}}}), "")
			want(t, callTool(t, cs, "edit_block", map[string]any{"file_path": p("a/t.docx"), "old_string": "Ship it", "new_string": "Shipped"}), "")
			want(t, callTool(t, cs, "read_file", map[string]any{"path": p("a/t.docx")}), "", "Shipped")
		}},
		{"X01 delete_file (hash-checked)", func(t *testing.T) {
			r := want(t, callTool(t, cs, "get_file_info", map[string]any{"path": p("r.pdf")}), "")
			want(t, callTool(t, cs, "delete_file", map[string]any{"path": p("r.pdf"), "expect_sha256": r.data["sha256"]}), "")
			if _, err := os.Stat(p("r.pdf")); !os.IsNotExist(err) {
				t.Fatal("still there")
			}
		}},
	}
	for _, s := range steps {
		t.Run(s.id, s.fn)
	}
}
