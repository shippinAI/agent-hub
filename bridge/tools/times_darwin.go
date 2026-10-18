package tools

import (
	"os"
	"syscall"
	"time"
)

func times(_ string, st os.FileInfo) (time.Time, time.Time, bool) {
	s, ok := st.Sys().(*syscall.Stat_t)
	if !ok {
		return time.Time{}, time.Time{}, false
	}
	return time.Unix(s.Birthtimespec.Unix()), time.Unix(s.Atimespec.Unix()), true
}
