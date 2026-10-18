//go:build !linux

package cli

import "fmt"

func cmdGUIAgent(_ []string) error {
	return fmt.Errorf("gui-agent is only implemented on linux in this build")
}
