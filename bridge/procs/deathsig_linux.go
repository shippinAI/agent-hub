package procs

import "syscall"

func setDeathSig(dst, src *syscall.SysProcAttr) { dst.Pdeathsig = src.Pdeathsig }
