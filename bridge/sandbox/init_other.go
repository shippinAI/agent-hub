//go:build !linux

package sandbox

import "fmt"

// Init is only used on Linux, where bwrap starts it as the first program in the sandbox.
func Init([]string) error { return fmt.Errorf("sandbox-init is only used on Linux") }
