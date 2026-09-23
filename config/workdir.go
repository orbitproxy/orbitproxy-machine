package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/orbitproxy/orbitproxy-go/appdir"
)

const configFileName = "orbitproxy.yml"

// ResolveRoot resolves the OrbitProxy data root (--workdir).
// Empty explicit uses ~/.orbitproxy on all OS.
func ResolveRoot(explicit string) (string, error) {
	return appdir.ResolveRoot(explicit)
}

// ResolveWorkDir is an alias for ResolveRoot (CLI --workdir = data root).
func ResolveWorkDir(explicit string) (string, error) {
	return ResolveRoot(explicit)
}

// MachineDir is <root>/<machineKey>.
func MachineDir(root, machineKey string) (string, error) {
	return appdir.MachineDir(root, machineKey)
}

// LogsDir is <machineDir>/logs.
func LogsDir(machineDir string) string {
	return appdir.LogsDir(machineDir)
}

// Path returns <machineDir>/orbitproxy.yml.
func Path(machineDir string) string {
	return filepath.Join(machineDir, configFileName)
}

// LogPath returns <machineDir>/logs/orbitproxy.log.
func LogPath(machineDir string) string {
	return filepath.Join(LogsDir(machineDir), "orbitproxy.log")
}

// ResolveMachineDir picks <root>/<machineKey> for CLI run.
// If machineKey is empty, requires exactly one machine directory under root that contains orbitproxy.yml.
func ResolveMachineDir(root, machineKey string) (string, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		var err error
		root, err = ResolveRoot("")
		if err != nil {
			return "", err
		}
	}
	machineKey = strings.TrimSpace(machineKey)
	if machineKey != "" {
		return MachineDir(root, machineKey)
	}
	key, err := DiscoverSoleMachineKey(root)
	if err != nil {
		return "", err
	}
	return MachineDir(root, key)
}

// ListMachineKeys returns root children that contain orbitproxy.yml, sorted.
func ListMachineKeys(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no machine config under %s; run `orbitproxy authtoken` first", root)
		}
		return nil, fmt.Errorf("list workdir %s: %w", root, err)
	}

	var keys []string
	for _, ent := range entries {
		if !ent.IsDir() {
			continue
		}
		name := ent.Name()
		if name == "." || name == ".." {
			continue
		}
		if _, err := os.Stat(Path(filepath.Join(root, name))); err != nil {
			continue
		}
		keys = append(keys, name)
	}
	sort.Strings(keys)
	return keys, nil
}

// DiscoverSoleMachineKey lists root children that contain orbitproxy.yml.
func DiscoverSoleMachineKey(root string) (string, error) {
	keys, err := ListMachineKeys(root)
	if err != nil {
		return "", err
	}
	switch len(keys) {
	case 0:
		return "", fmt.Errorf("no machine config under %s; run `orbitproxy authtoken` first", root)
	case 1:
		return keys[0], nil
	default:
		return "", fmt.Errorf("multiple machines under %s (%s); pass --machine-key", root, strings.Join(keys, ", "))
	}
}
