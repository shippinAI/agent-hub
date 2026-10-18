//go:build darwin

package sandbox

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

func kind() string {
	if exec.Command("/usr/bin/sandbox-exec", "-p", "(version 1)(allow default)", "/usr/bin/true").Run() != nil {
		return "none"
	}
	return "seatbelt"
}

// Profile generates the Seatbelt profile: deny by default, read system locations, read (or read+write) the
// grant paths, and network only to the loopback proxy when the grant names hosts. sandbox-exec is deprecated
// but still shipped (02 "Unverified"); D9 is the fallback.
func Profile(s Spec) string {
	var b strings.Builder
	b.WriteString("(version 1)\n(deny default)\n")
	b.WriteString("(allow process-exec process-fork signal)\n(allow sysctl-read mach-lookup ipc-posix-shm)\n")
	b.WriteString("(allow file-read* (subpath \"/usr\") (subpath \"/bin\") (subpath \"/sbin\") (subpath \"/System\") (subpath \"/Library\") (subpath \"/private/etc\") (subpath \"/opt\") (literal \"/dev/null\") (literal \"/dev/urandom\") (subpath \"/private/tmp\") (subpath \"/private/var/folders\"))\n")
	for _, p := range s.Paths {
		b.WriteString(fmt.Sprintf("(allow file-read* (subpath %q))\n", p))
		if s.Level == "workspace-write" {
			b.WriteString(fmt.Sprintf("(allow file-write* (subpath %q))\n", p))
		}
	}
	if s.ProxyTCP != "" {
		b.WriteString(fmt.Sprintf("(allow network-outbound (remote tcp %q))\n", s.ProxyTCP))
		b.WriteString("(allow network-outbound (remote unix-socket))\n")
	}
	return b.String()
}

func build(s Spec) (*exec.Cmd, error) {
	argv := append([]string{"-p", Profile(s)}, s.Argv...)
	c := exec.Command("/usr/bin/sandbox-exec", argv...)
	c.Dir = s.Cwd
	env := s.Env
	if s.ProxyTCP != "" {
		if env == nil {
			env = map[string]string{}
		}
		for _, k := range []string{"HTTPS_PROXY", "HTTP_PROXY", "https_proxy", "http_proxy"} {
			env[k] = "http://" + s.ProxyTCP
		}
	}
	c.Env = envList(env, os.Getenv("HOME"))
	detach(c)
	return c, nil
}

func detach(c *exec.Cmd) { c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} }

// Kill ends the confined process group.
func Kill(c *exec.Cmd) {
	if c.Process != nil {
		_ = syscall.Kill(-c.Process.Pid, syscall.SIGKILL)
		_ = c.Process.Kill()
	}
}
