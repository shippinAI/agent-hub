// Command devagotchi is the Devagotchi bridge: standalone MCP server, hub agent and admin CLI in one binary.
package main

import (
	"os"

	"devagotchi/bridge/cli"
)

func main() { os.Exit(cli.Main(os.Args[1:])) }
