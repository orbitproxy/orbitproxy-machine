package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadError describes why machine config cannot be loaded.
type LoadError struct {
	Kind       string
	Missing    []string
	ConfigPath string
	WorkDir    string
}

func (err *LoadError) Error() string {
	switch err.Kind {
	case "not_found":
		return fmt.Sprintf(
			"machine config not found at %s; run `orbitlink-machine authtoken <token>` first",
			err.ConfigPath,
		)
	case "incomplete":
		return fmt.Sprintf(
			"machine config at %s is missing required fields: %s",
			err.ConfigPath,
			strings.Join(err.Missing, ", "),
		)
	default:
		return "machine config is invalid"
	}
}

// Load reads orbitlink.yml from workDir, completes defaults, and validates.
func Load(workDir string) (*Config, error) {
	configPath := Path(workDir)
	if _, err := os.Stat(configPath); err != nil {
		if os.IsNotExist(err) {
			return nil, &LoadError{
				Kind:       "not_found",
				ConfigPath: configPath,
				WorkDir:    workDir,
			}
		}
		return nil, fmt.Errorf("inspect machine config %s: %w", configPath, err)
	}

	cfg, err := LoadFromFile(configPath)
	if err != nil {
		return nil, err
	}
	cfg.WorkDir = workDir
	return cfg, nil
}

// LoadFromFile 读取 YAML 配置文件，并依次执行 Complete 与 Validate。
func LoadFromFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var raw struct {
		Logger        LoggerConfig   `yaml:"logger"`
		ClientID      string         `yaml:"client_id"`
		MachineID     string         `yaml:"machine_id"`
		EdgeAddr      string         `yaml:"edge_addr"`
		TLSServerName string         `yaml:"tls_server_name"`
		TLSCAPEM      string         `yaml:"tls_ca_pem"`
		PublicKeyPEM  string `yaml:"public_key_pem"`
		PrivateKeyPEM string `yaml:"private_key_pem"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse machine config %s: %w", path, err)
	}

	machineID := strings.TrimSpace(raw.MachineID)
	if machineID == "" {
		machineID = strings.TrimSpace(raw.ClientID)
	}

	cfg := &Config{
		Logger:        raw.Logger,
		MachineID:     machineID,
		EdgeAddr:      strings.TrimSpace(raw.EdgeAddr),
		TLSServerName: strings.TrimSpace(raw.TLSServerName),
		TLSCAPEM:      raw.TLSCAPEM,
		PublicKeyPEM:  raw.PublicKeyPEM,
		PrivateKeyPEM: raw.PrivateKeyPEM,
		ConfigFile:    path,
	}

	if err := cfg.Complete(); err != nil {
		return nil, err
	}
	if err := Validate(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Save persists orbitlink.yml under workDir.
func Save(workDir string, cfg Config) error {
	path := Path(workDir)
	payload, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal machine config: %w", err)
	}

	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return fmt.Errorf("create workdir: %w", err)
	}
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		return fmt.Errorf("write machine config %s: %w", path, err)
	}
	return nil
}
