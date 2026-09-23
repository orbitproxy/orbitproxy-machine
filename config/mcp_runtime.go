package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	mcpRuntimeFileName = "mcp-runtime.env"
	npmRootEnvName     = "ORBITPROXY_NPM_ROOT_G"
)

// McpRuntime is the login-shell snapshot written at service install.
type McpRuntime struct {
	Home     string
	Path     string
	NpmRootG string
}

// McpRuntimePath is <machineDir>/mcp-runtime.env.
func McpRuntimePath(machineDir string) string {
	return filepath.Join(machineDir, mcpRuntimeFileName)
}

// WriteMcpRuntime writes KEY=VALUE lines. Existing file is replaced.
func WriteMcpRuntime(machineDir string, rt McpRuntime) error {
	if strings.TrimSpace(machineDir) == "" {
		return fmt.Errorf("machine dir is empty")
	}
	if err := os.MkdirAll(machineDir, 0o700); err != nil {
		return err
	}
	var b strings.Builder
	if rt.Home != "" {
		fmt.Fprintf(&b, "HOME=%s\n", rt.Home)
	}
	if rt.Path != "" {
		fmt.Fprintf(&b, "PATH=%s\n", rt.Path)
	}
	if rt.NpmRootG != "" {
		fmt.Fprintf(&b, "NPM_ROOT_G=%s\n", rt.NpmRootG)
	}
	return os.WriteFile(McpRuntimePath(machineDir), []byte(b.String()), 0o600)
}

// ApplyMcpRuntimeEnv loads the snapshot. HOME/PATH are set only when the process is missing them.
func ApplyMcpRuntimeEnv(machineDir string) error {
	path := McpRuntimePath(machineDir)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok || key == "" {
			continue
		}
		switch key {
		case "HOME", "PATH":
			if strings.TrimSpace(os.Getenv(key)) == "" {
				_ = os.Setenv(key, val)
			}
		case "NPM_ROOT_G":
			if strings.TrimSpace(os.Getenv(npmRootEnvName)) == "" && val != "" {
				_ = os.Setenv(npmRootEnvName, val)
			}
		}
	}
	return sc.Err()
}
