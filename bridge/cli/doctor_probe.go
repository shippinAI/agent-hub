package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"devagotchi/bridge/policy"
	"devagotchi/bridge/tools"
)

// sandboxCheck runs a real confined process: writing inside the root must work, outside must fail.
func sandboxCheck(kind string) []check {
	base, err := os.MkdirTemp("", "dg-doctor-")
	if err != nil {
		return []check{{"fail", "sandbox", err.Error()}}
	}
	defer os.RemoveAll(base)
	in, out := filepath.Join(base, "in"), filepath.Join(base, "out")
	os.Mkdir(in, 0o700)
	os.Mkdir(out, 0o700)
	p := policy.Defaults()
	p.Roots = []string{in}
	o := doctorRun(p, "run_command", map[string]any{"argv": []string{"sh", "-c", "echo a > " + in + "/a; echo b > " + out + "/b 2>/dev/null; true"}})
	_, inErr := os.Stat(filepath.Join(in, "a"))
	_, outErr := os.Stat(filepath.Join(out, "b"))
	switch {
	case o.Code != "":
		return []check{{"fail", "sandbox", kind + ": " + o.Text}}
	case inErr != nil:
		return []check{{"fail", "sandbox", kind + ": a confined process could not write inside its root"}}
	case outErr == nil:
		return []check{{"fail", "sandbox", kind + ": a confined process wrote OUTSIDE its root"}}
	}
	return []check{{"ok", "sandbox", kind + ": confined process wrote inside its root and was blocked outside it"}}
}

// ptyCheck starts a short command on a pseudo-terminal (ConPTY on Windows).
func ptyCheck() check {
	p := policy.Defaults()
	p.SandboxLevel = "unsandboxed"
	argv := []string{"sh", "-c", "echo pty-ok"}
	if runtime.GOOS == "windows" {
		argv = []string{"cmd", "/c", "echo pty-ok"}
	}
	o := doctorRun(p, "start_process", map[string]any{"argv": argv, "pty": true, "timeout_ms": 5000})
	if o.Code != "" || !strings.Contains(o.Text, "pty-ok") {
		return check{"warn", "pty", "pseudo-terminal start failed: " + firstLine(o.Text)}
	}
	return check{"ok", "pty", "pseudo-terminal works"}
}

func doctorRun(p policy.Policy, name string, args map[string]any) tools.Out {
	self, _ := os.Executable()
	e := tools.NewEnv(policy.NewGate(p), "doctor", self, nil, nil)
	defer e.Close()
	t, _ := tools.Find(name)
	raw, _ := json.Marshal(args)
	return t.Run(context.Background(), e, raw)
}

func firstLine(s string) string {
	l, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	return l
}
