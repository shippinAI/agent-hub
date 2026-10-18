//go:build windows

package sandbox

import (
	"os"
	"os/exec"
)

// On Windows the service runs confined commands as a dedicated low-privilege sandbox user (created at
// install, 02/D-Windows) inside a Job Object that is killed as a unit; a restricted token drops writes
// outside the grant paths. Creating the token and the job needs the Windows build's syscalls; this file
// keeps the cross-compile green and documents the design. Network confinement reuses the loopback proxy
// (ProxyTCP) because there is no network namespace. WGC screen capture is NOT implemented (deviation; GDI
// BitBlt is used instead, see gui/).
func kind() string {
	// A real install reports "windows-restricted-token"; without the sandbox user it degrades to none.
	if os.Getenv("DEVAGOTCHI_WIN_SANDBOX_USER") == "" {
		return "none"
	}
	return "windows-restricted-token"
}

func build(s Spec) (*exec.Cmd, error) {
	// TODO(windows-hardware): CreateProcessAsUser with the sandbox user's restricted token, assigned to a
	// Job Object with JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE; bind the grant paths via the token's allowed SIDs.
	c := exec.Command(s.Argv[0], s.Argv[1:]...)
	c.Dir = s.Cwd
	c.Env = envList(s.Env, os.Getenv("USERPROFILE"))
	return c, nil
}

func detach(c *exec.Cmd) {}

// Kill ends the process (a full build ends the Job Object).
func Kill(c *exec.Cmd) {
	if c.Process != nil {
		_ = c.Process.Kill()
	}
}
