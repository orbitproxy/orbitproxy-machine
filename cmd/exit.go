package main

import (
	"errors"
	"fmt"
)

// exitError 表示输出已经写完，只需要以 code 退出。
type exitError struct{ code int }

func (err *exitError) Error() string { return fmt.Sprintf("exit status %d", err.code) }

func exitWith(code int) error {
	if code == 0 {
		return nil
	}
	return &exitError{code: code}
}

func exitCodeOf(err error) (int, bool) {
	var exitErr *exitError
	if errors.As(err, &exitErr) {
		return exitErr.code, true
	}
	return 0, false
}
