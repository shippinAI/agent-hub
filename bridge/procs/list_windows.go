package procs

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// listAll walks a Toolhelp32 snapshot. CPU and memory are not sampled on Windows (they would need a
// second snapshot interval and psapi); those fields stay zero rather than guessed.
func listAll() ([]Proc, error) {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(snap)
	var e windows.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	var out []Proc
	for err = windows.Process32First(snap, &e); err == nil; err = windows.Process32Next(snap, &e) {
		name := windows.UTF16ToString(e.ExeFile[:])
		out = append(out, Proc{PID: int(e.ProcessID), PPID: int(e.ParentProcessID), Name: name, Command: name})
	}
	return out, nil
}

func killPID(pid int, _ bool) error {
	h, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	return windows.TerminateProcess(h, 1)
}
