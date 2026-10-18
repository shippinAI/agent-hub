package cli

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"devagotchi/bridge/config"
)

// cmdInstall writes a per-user service definition (systemd user unit, LaunchAgent, or a logon scheduled task)
// and prints the commands that enable it. Only --enable runs them. Nothing needs root or admin.
func cmdInstall(args []string) error {
	return serviceCmd("install", args)
}

// cmdUninstall disables (with --disable) and removes the service definition.
func cmdUninstall(args []string) error {
	return serviceCmd("uninstall", args)
}

func serviceCmd(op string, args []string) error {
	fs := flag.NewFlagSet(op, flag.ContinueOnError)
	mode := fs.String("mode", "connect", "connect (hub agent) or mcp-http (local streamable HTTP MCP server)")
	dir := fs.String("config-dir", config.UserDir(), "config directory")
	run := fs.Bool("enable", false, "install: also run the enable commands")
	fs.BoolVar(run, "disable", false, "uninstall: also run the disable commands")
	if err := fs.Parse(args); err != nil {
		return err
	}
	self, _ := os.Executable()
	self, _ = filepath.EvalSymlinks(self)
	s, err := serviceFor(*mode, self, *dir)
	if err != nil {
		return err
	}
	cmds := s.enable
	if op == "install" {
		if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(s.path, []byte(s.body), 0o644); err != nil {
			return err
		}
		fmt.Printf("Wrote %s\n", s.path)
	} else {
		cmds = s.disable
	}
	if !*run {
		fmt.Printf("To %s it, run:\n", map[string]string{"install": "enable", "uninstall": "disable"}[op])
		for _, c := range cmds {
			fmt.Println("  " + strings.Join(c, " "))
		}
	}
	for _, c := range cmds {
		if !*run {
			break
		}
		out, err := exec.Command(c[0], c[1:]...).CombinedOutput()
		fmt.Printf("$ %s\n%s", strings.Join(c, " "), out)
		if err != nil && op == "install" {
			return err
		}
	}
	if op == "uninstall" {
		if err := os.Remove(s.path); err != nil && !os.IsNotExist(err) {
			return err
		}
		fmt.Printf("Removed %s\n", s.path)
	}
	return nil
}
