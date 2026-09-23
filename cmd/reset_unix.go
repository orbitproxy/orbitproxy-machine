//go:build unix

package main

import (
	"os"
	"syscall"
	"time"
)

func terminatePID(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return nil
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		if processGone(err) {
			return nil
		}
		return err
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if err := proc.Signal(syscall.Signal(0)); err != nil {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	_ = proc.Kill()
	return nil
}

func processGone(err error) bool {
	return err == os.ErrProcessDone || err == syscall.ESRCH
}
