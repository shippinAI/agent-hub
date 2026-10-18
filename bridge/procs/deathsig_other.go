//go:build !linux && !windows

package procs

import "syscall"

func setDeathSig(dst, src *syscall.SysProcAttr) {}
