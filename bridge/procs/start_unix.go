//go:build !windows

package procs

import (
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/creack/pty"
)

type unixProc struct {
	cmd *exec.Cmd
	in  io.Writer
	tty *os.File
	eof chan struct{} // closed when the PTY reader drained everything
}

func (p *unixProc) Pid() int         { return p.cmd.Process.Pid }
func (p *unixProc) Stdin() io.Writer { return p.in }

func (p *unixProc) Wait() (int, error) {
	err := p.cmd.Wait()
	if p.tty != nil {
		// Drain what the child wrote before exiting; a grandchild holding the PTY open must not block us.
		select {
		case <-p.eof:
		case <-time.After(time.Second):
		}
		_ = p.tty.Close()
	}
	return p.cmd.ProcessState.ExitCode(), err
}

// Kill signals the whole process group (sandbox.Command and the PTY path both make the child a group leader).
func (p *unixProc) Kill() error {
	_ = syscall.Kill(-p.cmd.Process.Pid, syscall.SIGKILL)
	return p.cmd.Process.Kill()
}

func start(cmd *exec.Cmd, usePTY bool, out io.Writer) (handle, error) {
	if usePTY {
		attrs := &syscall.SysProcAttr{Setsid: true, Setctty: true}
		if cmd.SysProcAttr != nil {
			setDeathSig(attrs, cmd.SysProcAttr)
		}
		f, err := pty.StartWithAttrs(cmd, &pty.Winsize{Rows: 50, Cols: 200}, attrs)
		if err != nil {
			return nil, err
		}
		p := &unixProc{cmd: cmd, in: f, tty: f, eof: make(chan struct{})}
		go func() { _, _ = io.Copy(out, f); close(p.eof) }()
		return p, nil
	}
	cmd.Stdout, cmd.Stderr = out, out
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &unixProc{cmd: cmd, in: in}, nil
}
