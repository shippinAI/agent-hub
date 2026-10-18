package policy

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Resolve makes p absolute and clean, expands "~", and resolves symlinks on the longest existing prefix, so a
// link inside a root that points outside it is judged by where it really goes (DC #403, #791).
func Resolve(p string) string {
	if p == "" {
		return ""
	}
	if p == "~" || strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`) {
		home, _ := os.UserHomeDir()
		p = filepath.Join(home, p[1:])
	}
	if abs, err := filepath.Abs(p); err == nil {
		p = abs
	}
	p = filepath.Clean(p)
	rest := ""
	for cur := p; ; {
		if real, err := filepath.EvalSymlinks(cur); err == nil {
			return filepath.Join(real, rest)
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return p
		}
		rest = filepath.Join(filepath.Base(cur), rest)
		cur = parent
	}
}

// Within reports whether p is root or below it. Comparison folds case on Windows and macOS, whose default
// filesystems are case-insensitive (the DC #791 bypass).
func Within(p, root string) bool {
	if root == "" || p == "" {
		return false
	}
	if foldCase() {
		p, root = strings.ToLower(p), strings.ToLower(root)
	}
	if p == root {
		return true
	}
	sep := string(filepath.Separator)
	return strings.HasPrefix(p, strings.TrimSuffix(root, sep)+sep)
}

func foldCase() bool { return runtime.GOOS == "windows" || runtime.GOOS == "darwin" }
