package config

import (
	"strings"
)

// Validate 校验配置完整性。
func Validate(cfg *Config) error {
	var missing []string
	if strings.TrimSpace(cfg.MachineID) == "" {
		missing = append(missing, "machine_id")
	}
	if strings.TrimSpace(cfg.PrivateKeyPEM) == "" {
		missing = append(missing, "private_key_pem")
	}
	if strings.TrimSpace(cfg.EdgeAddr) == "" {
		missing = append(missing, "edge_addr")
	}
	if len(missing) > 0 {
		return &LoadError{
			Kind:       "incomplete",
			Missing:    missing,
			ConfigPath: cfg.ConfigFile,
			WorkDir:    cfg.WorkDir,
		}
	}
	return nil
}
