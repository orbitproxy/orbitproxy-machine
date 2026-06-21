package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const configFileName = "orbitlink.yml"

// ResolveWorkDir 解析 orbitlink 工作目录。
func ResolveWorkDir(explicit string) (string, error) {
	explicit = strings.TrimSpace(explicit)
	if explicit != "" {
		return filepath.Abs(explicit)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home dir: %w", err)
	}

	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(homeDir, "Library", "Application Support", "orbitlink"), nil
	case "windows":
		localAppData := strings.TrimSpace(os.Getenv("LocalAppData"))
		if localAppData == "" {
			localAppData = filepath.Join(homeDir, "AppData", "Local")
		}
		return filepath.Join(localAppData, "orbitlink"), nil
	default:
		return filepath.Join(homeDir, ".config", "orbitlink"), nil
	}
}

// Path 返回工作目录内的配置文件路径。
func Path(workDir string) string {
	return filepath.Join(workDir, configFileName)
}

// LogPath 返回工作目录内的日志文件路径。
func LogPath(workDir string) string {
	return filepath.Join(workDir, "orbitlink.log")
}
