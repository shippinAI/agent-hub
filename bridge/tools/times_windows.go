package tools

import (
	"os"
	"syscall"
	"time"
)

func times(_ string, st os.FileInfo) (time.Time, time.Time, bool) {
	d, ok := st.Sys().(*syscall.Win32FileAttributeData)
	if !ok {
		return time.Time{}, time.Time{}, false
	}
	return time.Unix(0, d.CreationTime.Nanoseconds()), time.Unix(0, d.LastAccessTime.Nanoseconds()), true
}
