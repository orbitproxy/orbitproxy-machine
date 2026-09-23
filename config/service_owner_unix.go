//go:build unix

package config

import (
	"fmt"
	"os"
	"os/user"
	"strings"
)

// ResolveServiceOwner returns the user that should own the service process.
// Under sudo this is SUDO_USER, not root.
func ResolveServiceOwner() (*ServiceOwner, error) {
	if os.Geteuid() == 0 {
		if name := strings.TrimSpace(os.Getenv("SUDO_USER")); name != "" && name != "root" {
			return lookupOwner(name)
		}
	}
	current, err := user.Current()
	if err != nil {
		return nil, fmt.Errorf("resolve current user: %w", err)
	}
	return ownerFromUser(current)
}

func lookupOwner(name string) (*ServiceOwner, error) {
	u, err := user.Lookup(name)
	if err != nil {
		return nil, fmt.Errorf("lookup user %s: %w", name, err)
	}
	return ownerFromUser(u)
}

func ownerFromUser(u *user.User) (*ServiceOwner, error) {
	owner := &ServiceOwner{
		Username: u.Username,
		Home:     u.HomeDir,
	}
	if u.Gid != "" {
		if g, err := user.LookupGroupId(u.Gid); err == nil {
			owner.Group = g.Name
		}
	}
	if strings.TrimSpace(owner.Home) == "" {
		return nil, fmt.Errorf("user %s has no home directory", u.Username)
	}
	return owner, nil
}
