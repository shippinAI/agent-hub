package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// service describes one OS service definition: the file we write and the commands that enable/disable it.
type service struct {
	path    string
	body    string
	enable  [][]string
	disable [][]string
}

var serviceArgs = map[string][]string{"connect": {"connect"}, "mcp-http": {"mcp", "--http", "--quiet"}}

func serviceFor(mode, self, dir string) (service, error) {
	args, ok := serviceArgs[mode]
	if !ok {
		return service{}, fmt.Errorf("--mode must be connect or mcp-http")
	}
	home, _ := os.UserHomeDir()
	name := "devagotchi-" + mode
	switch runtime.GOOS {
	case "linux":
		p := filepath.Join(xdgUserUnitDir(), name+".service")
		body := fmt.Sprintf("[Unit]\nDescription=Devagotchi (%s)\nAfter=network-online.target\n\n[Service]\nExecStart=%s %s\n"+
			"Restart=on-failure\nNoNewPrivileges=yes\n\n[Install]\nWantedBy=default.target\n", mode, quoteArg(self), strings.Join(args, " "))
		return service{p, body, [][]string{{"systemctl", "--user", "daemon-reload"}, {"systemctl", "--user", "enable", "--now", name}},
			[][]string{{"systemctl", "--user", "disable", "--now", name}}}, nil
	case "darwin":
		label := "com.devagotchi." + mode
		p := filepath.Join(home, "Library", "LaunchAgents", label+".plist")
		var argXML strings.Builder
		for _, a := range append([]string{self}, args...) {
			fmt.Fprintf(&argXML, "<string>%s</string>", xmlEscape(a))
		}
		body := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>Label</key><string>%s</string>
  <key>ProgramArguments</key><array>%s</array>
  <key>RunAtLoad</key><true/><key>KeepAlive</key><true/>
</dict></plist>
`, label, argXML.String())
		return service{p, body, [][]string{{"launchctl", "load", "-w", p}}, [][]string{{"launchctl", "unload", "-w", p}}}, nil
	case "windows":
		task := "Devagotchi-" + mode
		tr := fmt.Sprintf(`"%s" %s`, self, strings.Join(args, " "))
		p := filepath.Join(dir, name+".task.txt")
		body := "Scheduled task " + task + " (runs at logon, as you, no admin):\n" + tr + "\n"
		return service{p, body, [][]string{{"schtasks", "/Create", "/F", "/SC", "ONLOGON", "/RL", "LIMITED", "/TN", task, "/TR", tr}, {"schtasks", "/Run", "/TN", task}},
			[][]string{{"schtasks", "/End", "/TN", task}, {"schtasks", "/Delete", "/F", "/TN", task}}}, nil
	}
	return service{}, fmt.Errorf("install is not supported on %s", runtime.GOOS)
}

func xdgUserUnitDir() string {
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "systemd", "user")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "systemd", "user")
}

func quoteArg(s string) string {
	if strings.ContainsAny(s, " \t\"\\") {
		return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
	}
	return s
}

func xmlEscape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(s)
}
