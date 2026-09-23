package console

import (
	"os"

	"golang.org/x/term"
)

// IsTerminal reports whether f is an interactive terminal.
func IsTerminal(f *os.File) bool {
	if f == nil {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}
