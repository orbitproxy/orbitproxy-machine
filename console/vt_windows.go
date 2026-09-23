//go:build windows

package console

import (
	"os"

	"golang.org/x/sys/windows"
)

// EnableVirtualTerminal turns on ANSI processing for a Windows console.
func EnableVirtualTerminal(f *os.File) {
	if f == nil {
		return
	}
	handle := windows.Handle(f.Fd())
	var mode uint32
	if err := windows.GetConsoleMode(handle, &mode); err != nil {
		return
	}
	_ = windows.SetConsoleMode(handle, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING)
}
