//go:build windows

package procs

import (
	"io"
	"os"
	"os/exec"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// conProc is a process attached to a Windows pseudo console (ConPTY), so console programs such as ssh.exe,
// REPLs and installers see a real console (DC #702) without ever opening a visible window.
type conProc struct {
	pi  windows.ProcessInformation
	hpc windows.Handle
	in  *os.File
	out *os.File
	job windows.Handle
	pid int
	eof chan struct{}
}

func (p *conProc) Pid() int         { return p.pid }
func (p *conProc) Stdin() io.Writer { return p.in }

func (p *conProc) Wait() (int, error) {
	_, err := windows.WaitForSingleObject(p.pi.Process, windows.INFINITE)
	var code uint32
	_ = windows.GetExitCodeProcess(p.pi.Process, &code)
	windows.ClosePseudoConsole(p.hpc) // flushes the last output, then the reader sees EOF
	select {
	case <-p.eof:
	case <-time.After(time.Second):
	}
	_ = p.in.Close()
	_ = windows.CloseHandle(p.pi.Process)
	_ = windows.CloseHandle(p.pi.Thread)
	return int(code), err
}

func (p *conProc) Kill() error {
	if p.job != 0 {
		_ = windows.TerminateJobObject(p.job, 1)
	}
	return windows.TerminateProcess(p.pi.Process, 1)
}

func startConPTY(cmd *exec.Cmd, out io.Writer) (handle, error) {
	var inR, inW, outR, outW windows.Handle
	if err := windows.CreatePipe(&inR, &inW, nil, 0); err != nil {
		return nil, err
	}
	if err := windows.CreatePipe(&outR, &outW, nil, 0); err != nil {
		return nil, err
	}
	var hpc windows.Handle
	if err := windows.CreatePseudoConsole(windows.Coord{X: 200, Y: 50}, inR, outW, 0, &hpc); err != nil {
		return nil, err
	}
	_ = windows.CloseHandle(inR)
	_ = windows.CloseHandle(outW)
	attrs, err := windows.NewProcThreadAttributeList(1)
	if err != nil {
		return nil, err
	}
	defer attrs.Delete()
	// The attribute value is the HPCON itself, not a pointer to it (CreatePseudoConsole docs).
	if err := attrs.Update(windows.PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE, *(*unsafe.Pointer)(unsafe.Pointer(&hpc)), unsafe.Sizeof(hpc)); err != nil {
		return nil, err
	}
	si := windows.StartupInfoEx{ProcThreadAttributeList: attrs.List()}
	si.Cb = uint32(unsafe.Sizeof(si))
	line, _ := windows.UTF16PtrFromString(windows.ComposeCommandLine(cmd.Args))
	var dir *uint16
	if cmd.Dir != "" {
		dir, _ = windows.UTF16PtrFromString(cmd.Dir)
	}
	p := &conProc{hpc: hpc, in: os.NewFile(uintptr(inW), "conin"), out: os.NewFile(uintptr(outR), "conout")}
	flags := uint32(windows.EXTENDED_STARTUPINFO_PRESENT | windows.CREATE_UNICODE_ENVIRONMENT)
	if err := windows.CreateProcess(nil, line, nil, nil, false, flags, envBlock(cmd.Env), dir, &si.StartupInfo, &p.pi); err != nil {
		windows.ClosePseudoConsole(hpc)
		return nil, err
	}
	p.pid = int(p.pi.ProcessId)
	if job, err := newJob(); err == nil && windows.AssignProcessToJobObject(job, p.pi.Process) == nil {
		p.job = job
	}
	p.eof = make(chan struct{})
	go func() { _, _ = io.Copy(out, p.out); close(p.eof) }()
	return p, nil
}
