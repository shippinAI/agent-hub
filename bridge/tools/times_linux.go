package tools

import (
	"os"
	"time"

	"golang.org/x/sys/unix"
)

// times returns created (statx birth time; zero when the filesystem does not record it) and accessed.
func times(p string, _ os.FileInfo) (time.Time, time.Time, bool) {
	var x unix.Statx_t
	if unix.Statx(unix.AT_FDCWD, p, unix.AT_SYMLINK_NOFOLLOW, unix.STATX_BTIME|unix.STATX_ATIME, &x) != nil {
		return time.Time{}, time.Time{}, false
	}
	var born time.Time
	if x.Mask&unix.STATX_BTIME != 0 {
		born = time.Unix(x.Btime.Sec, int64(x.Btime.Nsec))
	}
	return born, time.Unix(x.Atime.Sec, int64(x.Atime.Nsec)), true
}
