package fsops

import (
	"io"
	"os"
	"time"
)

func doRead(op Op) Result {
	f, err := os.Open(op.Path)
	if err != nil {
		return fail("not_found", err.Error())
	}
	defer f.Close()
	st, _ := f.Stat()
	if op.Offset > 0 {
		_, _ = f.Seek(op.Offset, io.SeekStart)
	}
	lim := int64(maxRead)
	if op.Limit > 0 && op.Limit < lim {
		lim = op.Limit
	}
	b, err := io.ReadAll(io.LimitReader(f, lim))
	if err != nil {
		return fail("io_error", err.Error())
	}
	return Result{Content: string(b), Size: st.Size(), SHA256: sha(b), Mtime: st.ModTime().UTC().Format(time.RFC3339)}
}

func doList(op Op) Result {
	ents, err := os.ReadDir(op.Path)
	if err != nil {
		return fail("not_found", err.Error())
	}
	out := Result{Entries: []DirEntry{}}
	for _, e := range ents {
		info, _ := e.Info()
		var sz int64
		if info != nil {
			sz = info.Size()
		}
		out.Entries = append(out.Entries, DirEntry{Name: e.Name(), IsDir: e.IsDir(), Size: sz})
	}
	return out
}

func doStat(op Op) Result {
	st, err := os.Stat(op.Path)
	if err != nil {
		return fail("not_found", err.Error())
	}
	return Result{Size: st.Size(), Mtime: st.ModTime().UTC().Format(time.RFC3339), Extra: map[string]any{"is_dir": st.IsDir(), "mode": st.Mode().String()}}
}
