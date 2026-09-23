//go:build !windows

package console

import "os"

// EnableVirtualTerminal is a no-op outside Windows.
func EnableVirtualTerminal(*os.File) {}
