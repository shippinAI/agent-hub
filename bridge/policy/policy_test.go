package policy

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGatePathsAndSymlinks(t *testing.T) {
	root, out := t.TempDir(), t.TempDir()
	_ = os.WriteFile(filepath.Join(out, "secret"), []byte("x"), 0o600)
	_ = os.Symlink(out, filepath.Join(root, "link"))
	p := Defaults()
	p.Roots = []string{root}
	g := NewGate(p)
	if _, err := g.Path(CapRead, filepath.Join(root, "a.txt")); err != nil {
		t.Fatal(err)
	}
	if _, err := g.Path(CapRead, filepath.Join(root, "link", "secret")); err == nil {
		t.Fatal("symlink escape allowed")
	}
	if _, err := g.Path(CapRead, filepath.Join(root, "..", filepath.Base(out))); err == nil {
		t.Fatal("dotdot escape allowed")
	}
	if g.Has(CapNet) || g.Host("example.com") == nil {
		t.Fatal("net should be off by default")
	}
	n := g.Narrow([]string{filepath.Join(root, "sub")})
	if _, err := n.Path(CapRead, filepath.Join(root, "a.txt")); err == nil {
		t.Fatal("client roots must narrow")
	}
}

func TestGrantsAndReadOnly(t *testing.T) {
	root, extra := t.TempDir(), t.TempDir()
	p := Defaults()
	p.SandboxLevel, p.Roots = "read-only", []string{root}
	p.Grants = []Grant{{ID: "g1", Capability: CapWrite, Paths: []string{extra}, Expires: time.Now().Add(time.Hour)},
		{ID: "g2", Capability: CapNet, Hosts: []string{"*.example.com"}, Expires: time.Now().Add(-time.Hour)}}
	g := NewGate(p)
	if _, err := g.Path(CapWrite, filepath.Join(root, "x")); err == nil {
		t.Fatal("read-only level allowed a write")
	}
	if _, err := g.Path(CapWrite, filepath.Join(extra, "x")); err != nil {
		t.Fatal(err)
	}
	if g.Has(CapNet) {
		t.Fatal("expired grant counted")
	}
}

func TestSetAndValidate(t *testing.T) {
	p := Defaults()
	p, err := Set(p, "limits.max_timeout_ms", "5000")
	if err != nil || p.Limits.MaxTimeoutMS != 5000 {
		t.Fatal(err, p.Limits.MaxTimeoutMS)
	}
	if p, err = Set(p, "roots", "/a,/b"); err != nil || len(p.Roots) != 2 {
		t.Fatal(err, p.Roots)
	}
	if _, err = Set(p, "sandbox_level", "yolo"); err == nil {
		t.Fatal("bad level accepted")
	}
	f := filepath.Join(t.TempDir(), FileName)
	if err := Save(f, p); err != nil {
		t.Fatal(err)
	}
	q, err := Load(f)
	if err != nil || q.Limits.MaxTimeoutMS != 5000 || q.Roots[1] != "/b" {
		t.Fatal(err, q)
	}
	_ = os.WriteFile(f, []byte("capabilities = [\"root.everything\"]\n"), 0o600)
	if _, err := Load(f); err == nil {
		t.Fatal("unknown capability must fail closed")
	}
}
