package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"devagotchi/bridge/config"
)

// cmdInstall writes the per-OS service definition to the config dir and prints the single command the user
// runs to enable it. It installs nothing system-wide on its own and never needs root for the Linux user unit.
func cmdInstall(args []string) error {
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	dir := fs.String("config-dir", config.Dir(), "config directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	self, _ := os.Executable()
	switch runtime.GOOS {
	case "linux":
		unit := fmt.Sprintf(`[Unit]
Description=Devagotchi bridge
After=network-online.target

[Service]
ExecStart=%s run
Restart=on-failure
NoNewPrivileges=yes

[Install]
WantedBy=default.target
`, self)
		path := filepath.Join(xdgUserUnitDir(), "devagotchi-bridge.service")
		if err := writeFile(path, unit); err != nil {
			return err
		}
		fmt.Printf("Wrote %s\nEnable it with:\n  systemctl --user daemon-reload && systemctl --user enable --now devagotchi-bridge\n", path)
	case "darwin":
		home, _ := os.UserHomeDir()
		plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>Label</key><string>com.devagotchi.bridge</string>
  <key>ProgramArguments</key><array><string>%s</string><string>run</string></array>
  <key>RunAtLoad</key><true/><key>KeepAlive</key><true/>
</dict></plist>
`, self)
		path := filepath.Join(home, "Library", "LaunchAgents", "com.devagotchi.bridge.plist")
		if err := writeFile(path, plist); err != nil {
			return err
		}
		fmt.Printf("Wrote %s\nEnable it with:\n  launchctl load -w %s\n", path, path)
	case "windows":
		path := filepath.Join(*dir, "install-service.ps1")
		ps := fmt.Sprintf("# Run in an elevated PowerShell to install the Devagotchi bridge service.\n"+
			"sc.exe create DevagotchiBridge binPath= '\"%s\" run' start= auto\n"+
			"sc.exe description DevagotchiBridge 'Devagotchi bridge'\nStart-Service DevagotchiBridge\n", self)
		if err := writeFile(path, ps); err != nil {
			return err
		}
		fmt.Printf("Wrote %s\nRun it in an elevated PowerShell to install the service.\n", path)
	default:
		return fmt.Errorf("install is not supported on %s", runtime.GOOS)
	}
	return nil
}

func xdgUserUnitDir() string {
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "systemd", "user")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "systemd", "user")
}

func writeFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strings.ReplaceAll(content, "\r\n", "\n")), 0o644)
}
