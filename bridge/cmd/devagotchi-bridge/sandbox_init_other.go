//go:build !linux

package main

import "fmt"

func sandboxInit(_ []string) error {
	return fmt.Errorf("sandbox-init is only used inside the Linux bubblewrap sandbox")
}

func sandboxFsop(_ []string) error {
	return fmt.Errorf("sandbox-fsop is only used inside the Linux bubblewrap sandbox")
}
