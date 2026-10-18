package fsops

import (
	"os"
	"path/filepath"
)

// AtomicWrite writes b to path via a temp file in the same directory, fsync, then rename, so readers see
// the old or the new file and never a half-written one (DC #433). An existing file keeps its mode.
func AtomicWrite(path string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	mode := os.FileMode(0o644)
	if st, err := os.Stat(path); err == nil {
		mode = st.Mode().Perm()
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".devagotchi-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	_ = os.Chmod(tmp.Name(), mode)
	return os.Rename(tmp.Name(), path)
}

func atomicWrite(path string, b []byte) error { return AtomicWrite(path, b) }

func trunc(s string) string {
	if len(s) > 40 {
		return s[:40] + "…"
	}
	return s
}
