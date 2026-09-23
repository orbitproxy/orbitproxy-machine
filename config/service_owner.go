package config

import (
	"path/filepath"
	"strings"
)

const homeRootDirName = ".orbitproxy"

// ServiceOwner is the user whose home directory owns the machine config.
type ServiceOwner struct {
	Username string
	Group    string
	Home     string
}

// ResolveServiceRoot is the data root the OS service should use.
// Explicit --workdir wins; otherwise <owner.Home>/.orbitproxy.
func ResolveServiceRoot(explicit string, owner *ServiceOwner) (string, error) {
	if strings.TrimSpace(explicit) != "" {
		return ResolveRoot(explicit)
	}
	if owner != nil && strings.TrimSpace(owner.Home) != "" {
		return filepath.Abs(filepath.Join(owner.Home, homeRootDirName))
	}
	return ResolveRoot("")
}
