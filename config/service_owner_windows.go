//go:build windows

package config

import (
	"fmt"
	"os"
	"os/user"
	"strings"
)

// ResolveServiceOwner returns the current Windows user.
func ResolveServiceOwner() (*ServiceOwner, error) {
	current, err := user.Current()
	if err != nil {
		return nil, fmt.Errorf("resolve current user: %w", err)
	}
	home := strings.TrimSpace(current.HomeDir)
	if home == "" {
		home, err = os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("resolve home directory: %w", err)
		}
	}
	return &ServiceOwner{
		Username: current.Username,
		Home:     home,
	}, nil
}
