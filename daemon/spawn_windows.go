//go:build windows

package daemon

import (
	"os/exec"
	"syscall"
)

const (
	detachedProcess       = 0x00000008
	createNewProcessGroup = 0x00000200
)

func spawnDetached(executable string, args []string, outputPath string) (int, error) {
	cmd := exec.Command(executable, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: detachedProcess | createNewProcessGroup,
		HideWindow:    true,
	}
	return startDetached(cmd, outputPath)
}

const (
	synchronize      = 0x00100000
	waitObjectZero   = 0x00000000
	processQueryInfo = 0x00001000
)

func processAlive(pid int) bool {
	handle, err := syscall.OpenProcess(synchronize|processQueryInfo, false, uint32(pid))
	if err != nil {
		return false
	}
	defer func() { _ = syscall.CloseHandle(handle) }()
	event, err := syscall.WaitForSingleObject(handle, 0)
	return err == nil && event != waitObjectZero
}

// Windows 没有可投递给分离进程的 SIGTERM，直接结束进程。
func terminate(pid int) error {
	return kill(pid)
}
