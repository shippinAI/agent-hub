// Package cli is the devagotchi command line: one binary that is a standalone MCP server (`mcp`), a
// hub-connected bridge (`connect`), and the human-only admin surface (config, grants, service install).
package cli

import (
	"fmt"
	"os"

	"devagotchi/bridge/buildinfo"
)

// version is reported to the hub at enroll and in logs.
var version = buildinfo.Version

type command struct {
	name string
	help string
	run  func([]string) error
}

func commands() []command {
	return []command{
		{"mcp", "Standalone local MCP server (stdio, or --http ADDR for streamable HTTP)", cmdMCP},
		{"connect", "Connect to the hub and run granted calls (alias: run)", cmdRun},
		{"enroll", "Enroll this machine with a hub (code read from stdin, never a flag)", cmdEnroll},
		{"doctor", "Self-check: OS support, sandbox, permissions, policy, PTY", cmdDoctor},
		{"version", "Print version, commit, build date and target", cmdVersion},
		{"config", "Human-only policy admin: path | show | init | set KEY VALUE", cmdConfig},
		{"grant", "Human-only local grants: list | add | revoke", cmdGrant},
		{"install", "Install the background service (--mode mcp-http|connect, --enable)", cmdInstall},
		{"uninstall", "Remove the background service files (--disable first if enabled)", cmdUninstall},
		{"probe", "Run the sandbox self-test and print the result", cmdProbe},
		{"gui-agent", "Run the desktop GUI helper in the logged-in session", cmdGUIAgent},
	}
}

// Main runs the CLI and returns the process exit code.
func Main(args []string) int {
	if len(args) < 1 {
		usage()
		return 2
	}
	name := args[0]
	switch name {
	case "run":
		name = "connect"
	case "--version", "-v":
		name = "version"
	case "--probe":
		name = "probe"
	case "sandbox-init":
		return exit(sandboxInit(args[1:]))
	case "sandbox-fsop":
		return exit(sandboxFsop(args[1:]))
	case "help", "-h", "--help":
		usage()
		return 0
	}
	for _, c := range commands() {
		if c.name == name {
			return exit(c.run(args[1:]))
		}
	}
	usage()
	return 2
}

func exit(err error) int {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	return 0
}

func cmdVersion(_ []string) error {
	fmt.Println(buildinfo.String())
	return nil
}

func usage() {
	fmt.Fprintln(os.Stderr, "devagotchi <command> [flags]\n\nCommands:")
	for _, c := range commands() {
		fmt.Fprintf(os.Stderr, "  %-10s %s\n", c.name, c.help)
	}
	fmt.Fprintln(os.Stderr, "\nConfig and policy live in the per-OS config dir (`devagotchi config path`). Tokens are 0600 and never printed.\nNo telemetry: the only network calls are to the hub you configure and URLs your policy allows.")
}
