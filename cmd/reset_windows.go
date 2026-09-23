//go:build windows

package main

import (
	"os"
	"time"
)

func terminatePID(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return nil
	}
	if err := proc.Kill(); err != nil {
		if err == os.ErrProcessDone {
			return nil
		}
		return err
	}
	deadline := time.Now().Add(3 * time.Second)
	done := make(chan struct{})
	go func() {
		_, _ = proc.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-time.After(time.Until(deadline)):
		return nil
	}
}
