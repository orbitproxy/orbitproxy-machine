//go:build windows

package main

import (
	"os"
	"os/exec"

	"github.com/inconshreveable/mousetrap"
)

// tryHandleExplorerLaunch 与 ZeroNews explorer_launch_windows.go 相同：
// cmd /C start cmd /k <exe> --help，然后当前进程返回。
func tryHandleExplorerLaunch() bool {
	if !mousetrap.StartedByExplorer() {
		return false
	}

	exePath, err := os.Executable()
	if err != nil || exePath == "" {
		exePath = os.Args[0]
	}

	cmd := exec.Command("cmd", "/C", "start", "cmd", "/k", exePath, "--help")
	if err := cmd.Start(); err != nil {
		return false
	}

	return true
}
