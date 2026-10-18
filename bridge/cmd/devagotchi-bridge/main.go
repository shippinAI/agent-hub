// Command devagotchi-bridge is the per-machine agent: it enrolls, connects to the hub, and runs signed calls
// inside an OS sandbox. Subcommands: enroll, run, probe, install, gui-agent (desktop session), and the
// internal sandbox-init / sandbox-fsop re-exec modes.
package main

import (
	"fmt"
	"os"
)

const version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "enroll":
		err = cmdEnroll(os.Args[2:])
	case "run":
		err = cmdRun(os.Args[2:])
	case "probe", "--probe":
		err = cmdProbe(os.Args[2:])
	case "install":
		err = cmdInstall(os.Args[2:])
	case "gui-agent":
		err = cmdGUIAgent(os.Args[2:])
	case "sandbox-init":
		err = sandboxInit(os.Args[2:])
	case "sandbox-fsop":
		err = sandboxFsop(os.Args[2:])
	case "version", "--version":
		fmt.Println("devagotchi-bridge", version)
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `devagotchi-bridge <command>

  enroll --hub URL        Enroll this machine (prompts for the code on stdin; never a flag)
  run                     Connect to the hub and run granted calls (the service entry point)
  probe                   Run the sandbox self-test and print the result
  install [--user]        Install the background service (systemd user unit / launchd / Windows service)
  gui-agent               Run the desktop GUI helper in the logged-in session
  version                 Print the version

Config lives in the per-OS config dir; the machine token is stored 0600 and never printed.
`)
}
