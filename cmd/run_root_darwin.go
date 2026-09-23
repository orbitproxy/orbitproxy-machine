//go:build darwin

package main

import (
	"fmt"
	"os"
)

func refuseRootForeground() error {
	if os.Geteuid() != 0 {
		return nil
	}
	return fmt.Errorf("orbitproxy run must not be started with sudo. Run it as your user. Use sudo only for orbitproxy service install, start, stop, and restart")
}
