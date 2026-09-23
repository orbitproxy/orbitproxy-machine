package main

import (
	"fmt"
	"os/exec"
)

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if len(output) == 0 {
			return fmt.Errorf("%s %v: %w", name, args, err)
		}
		return fmt.Errorf("%s %v: %w\n%s", name, args, err, string(output))
	}
	return nil
}
