//go:build !windows

package daemon

import (
	"os"
	"os/exec"
	"syscall"
)

func spawnDetached(executable string, args []string, outputPath string) (int, error) {
	cmd := exec.Command(executable, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return startDetached(cmd, outputPath)
}

func processAlive(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}

func terminate(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return process.Signal(syscall.SIGTERM)
}
