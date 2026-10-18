//go:build windows

package procs

import (
	"io"
	"os/exec"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// winProc wraps an exec.Cmd started hidden (CREATE_NO_WINDOW: no console window, no focus stealing, DC #852)
// and placed in a kill-on-close Job Object so Kill ends the whole tree.
type winProc struct {
	cmd *exec.Cmd
	in  io.Writer
	job windows.Handle
}

func (p *winProc) Pid() int         { return p.cmd.Process.Pid }
func (p *winProc) Stdin() io.Writer { return p.in }

func (p *winProc) Wait() (int, error) {
	err := p.cmd.Wait()
	return p.cmd.ProcessState.ExitCode(), err
}

func (p *winProc) Kill() error {
	if p.job != 0 {
		_ = windows.TerminateJobObject(p.job, 1)
	}
	return p.cmd.Process.Kill()
}

func start(cmd *exec.Cmd, usePTY bool, out io.Writer) (handle, error) {
	if usePTY {
		return startConPTY(cmd, out)
	}
	cmd.Stdout, cmd.Stderr = out, out
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW | windows.CREATE_NEW_PROCESS_GROUP}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	p := &winProc{cmd: cmd, in: in}
	if job, err := newJob(); err == nil {
		if h, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid)); err == nil {
			_ = windows.AssignProcessToJobObject(job, h)
			_ = windows.CloseHandle(h)
			p.job = job
		}
	}
	return p, nil
}

// newJob creates a Job Object that kills every member when its last handle closes or it is terminated.
func newJob() (windows.Handle, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return 0, err
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	_, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)))
	if err != nil {
		_ = windows.CloseHandle(job)
		return 0, err
	}
	return job, nil
}

// envBlock builds a CreateProcess UTF-16 environment block ("k=v\0...\0\0").
func envBlock(env []string) *uint16 {
	if len(env) == 0 {
		return nil
	}
	var b []uint16
	for _, kv := range env {
		u, _ := windows.UTF16FromString(kv)
		b = append(b, u...)
	}
	b = append(b, 0)
	return &b[0]
}
