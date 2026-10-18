package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"devagotchi/bridge/sandbox"
)

func cmdProbe(_ []string) error {
	h, _ := os.Hostname()
	b, _ := json.MarshalIndent(map[string]string{"sandbox": sandbox.Kind(), "host": h}, "", "  ")
	fmt.Println(string(b))
	return nil
}
