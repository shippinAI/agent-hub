package textio

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadLines(t *testing.T) {
	p := filepath.Join(t.TempDir(), "f.txt")
	var b strings.Builder
	for i := 0; i < 100; i++ {
		b.WriteString("line" + string(rune('0'+i%10)) + "\n")
	}
	_ = os.WriteFile(p, []byte(b.String()), 0o644)
	w, err := ReadLines(p, 10, 5, 1<<20)
	if err != nil || w.Returned != 5 || w.Total != 100 || w.Remaining != 85 || !strings.HasPrefix(w.Text, "line0\n") {
		t.Fatalf("%+v %v", w, err)
	}
	w, _ = ReadLines(p, -3, 0, 1<<20)
	if w.Returned != 3 || w.Text != "line7\nline8\nline9\n" || w.Start != 97 {
		t.Fatalf("tail %+v", w)
	}
	_ = os.WriteFile(p, []byte("a\nb"), 0o644)
	if n, _ := CountLines(p); n != 2 {
		t.Fatal(n)
	}
	_ = os.WriteFile(p, []byte{1, 0, 2}, 0o644)
	if _, err := ReadLines(p, 0, 10, 100); err != ErrBinary {
		t.Fatal("binary not detected")
	}
}
