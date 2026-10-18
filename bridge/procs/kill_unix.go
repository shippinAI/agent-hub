//go:build !windows

package procs

import (
	"syscall"
	"time"
)

func killPID(pid int, force bool) error {
	if force {
		return syscall.Kill(pid, syscall.SIGKILL)
	}
	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
		return err
	}
	for i := 0; i < 20; i++ {
		time.Sleep(100 * time.Millisecond)
		if syscall.Kill(pid, 0) != nil {
			return nil
		}
	}
	return syscall.Kill(pid, syscall.SIGKILL)
}
