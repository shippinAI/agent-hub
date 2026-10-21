package cli

import "errors"

var errPending = errors.New("not built yet in this commit")

func cmdMCP([]string) error       { return errPending }
func cmdDoctor([]string) error    { return errPending }
func cmdConfig([]string) error    { return errPending }
func cmdGrant([]string) error     { return errPending }
func cmdUninstall([]string) error { return errPending }
