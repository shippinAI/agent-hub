// hub: a thin CLI over the agent-hub REST API for shell agents and humans.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

const usage = `usage: hub <command> [args] [--flag value]
  whoami | ready [--project P] | ls [--state S] [--mine] | show ID | search QUERY
  new TITLE [--project P] [--desc D] [--labels a,b] [--priority N]
  claim [ID] [--ttl SECONDS]        (no ID = claim_next)
  beat ID | note ID TEXT [--kind K] | done ID SUMMARY | handoff ID NOTE [--to AGENT] | release ID [REASON]
  events [--after SEQ] | verify | protocol
env: HUB_URL, HUB_TOKEN or HUB_TOKEN_FILE, HUB_STATE (claim cache). --request-id reuses an idempotency key.`

func parse(args []string) (pos []string, flags map[string]string) {
	flags = map[string]string{}
	for i := 0; i < len(args); i++ {
		if a := args[i]; strings.HasPrefix(a, "--") {
			k := a[2:]
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
				flags[k] = args[i+1]
				i++
			} else {
				flags[k] = "true"
			}
			continue
		}
		pos = append(pos, args[i])
	}
	return pos, flags
}

func main() {
	if len(os.Args) < 2 || os.Args[1] == "-h" || os.Args[1] == "help" {
		fmt.Println(usage)
		return
	}
	pos, flags := parse(os.Args[2:])
	c, err := newClient(flags["request-id"])
	if err == nil {
		err = run(c, os.Args[1], pos, flags)
	}
	if err != nil {
		var ae *apiError
		if errors.As(err, &ae) && (ae.Code == "stale_fence" || ae.Code == "not_holder" || ae.Code == "lease_expired") && len(pos) > 0 {
			loadState().drop(pos[0])
			fmt.Fprintln(os.Stderr, "you no longer hold this task; stop working on it")
		}
		fmt.Fprintln(os.Stderr, "hub:", err)
		os.Exit(1)
	}
}

func need(pos []string, n int) error {
	if len(pos) < n {
		return errors.New("missing argument\n" + usage)
	}
	return nil
}

func show(v any) {
	b, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(b))
}
