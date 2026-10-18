//go:build linux

package sandbox

import (
	"os"
	"os/exec"
	"syscall"
)

// ProxyAddr is where the in-sandbox forwarder listens; HTTP(S)_PROXY point here.
const ProxyAddr = "127.0.0.1:3128"

func kind() string {
	if _, err := exec.LookPath("bwrap"); err != nil {
		return "none"
	}
	if exec.Command("bwrap", "--unshare-user", "--unshare-net", "--ro-bind", "/", "/", "--", "/bin/true").Run() != nil {
		return "none"
	}
	return "bubblewrap"
}

var systemDirs = []string{"/usr", "/bin", "/sbin", "/lib", "/lib32", "/lib64", "/libx32", "/etc", "/opt", "/nix", "/snap"}

func build(s Spec) (*exec.Cmd, error) {
	a := []string{"--die-with-parent", "--new-session", "--unshare-pid", "--unshare-ipc", "--unshare-uts", "--unshare-cgroup-try", "--unshare-net",
		"--unshare-user", "--dev", "/dev", "--proc", "/proc", "--tmpfs", "/tmp", "--tmpfs", "/run", "--dir", "/tmp/home"}
	if s.UsernsRoot {
		a = append(a, "--uid", "0", "--gid", "0")
	}
	for _, d := range systemDirs {
		if _, err := os.Stat(d); err == nil {
			a = append(a, "--ro-bind", d, d)
		}
	}
	bind := "--ro-bind"
	if s.Level == "workspace-write" {
		bind = "--bind"
	}
	area := s.Paths
	if len(area) == 0 && s.Cwd != "" {
		area = []string{s.Cwd}
	}
	for _, p := range area {
		if _, err := os.Stat(p); err == nil {
			a = append(a, bind, p, p)
		}
	}
	a = append(a, "--ro-bind", s.Self, "/run/devagotchi-init", "--clearenv")
	for _, kv := range envList(s.Env, "/tmp/home") {
		k, v, _ := cut(kv)
		a = append(a, "--setenv", k, v)
	}
	init := []string{"/run/devagotchi-init", "sandbox-init"}
	if s.ProxySock != "" {
		a = append(a, "--bind", s.ProxySock, "/run/devagotchi-proxy.sock")
		for _, k := range []string{"HTTPS_PROXY", "HTTP_PROXY", "https_proxy", "http_proxy"} {
			a = append(a, "--setenv", k, "http://"+ProxyAddr)
		}
		init = append(init, "--fwd", "/run/devagotchi-proxy.sock")
	}
	if s.Cwd != "" {
		a = append(a, "--chdir", s.Cwd)
	}
	a = append(a, "--")
	a = append(a, init...)
	a = append(a, "--")
	a = append(a, s.Argv...)
	c := exec.Command("bwrap", a...)
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
	return c, nil
}

func cut(kv string) (string, string, bool) {
	for i := 0; i < len(kv); i++ {
		if kv[i] == '=' {
			return kv[:i], kv[i+1:], true
		}
	}
	return kv, "", false
}

func detach(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
}

// Kill ends the whole confined process group (bwrap is pid 1 of its namespace, so everything inside dies).
func Kill(c *exec.Cmd) {
	if c.Process != nil {
		_ = syscall.Kill(-c.Process.Pid, syscall.SIGKILL)
		_ = c.Process.Kill()
	}
}
