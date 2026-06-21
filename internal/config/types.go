package config

// Config is persisted under the workdir as orbitlink.yml.
type Config struct {
	Logger LoggerConfig `yaml:"logger,omitempty" json:"logger"`

	MachineID     string `yaml:"machine_id" json:"machine_id"`
	EdgeAddr      string `yaml:"edge_addr,omitempty" json:"edge_addr,omitempty"`
	TLSServerName string `yaml:"tls_server_name,omitempty" json:"tls_server_name,omitempty"`
	TLSCAPEM      string `yaml:"tls_ca_pem,omitempty" json:"tls_ca_pem,omitempty"`
	PublicKeyPEM  string `yaml:"public_key_pem" json:"public_key_pem"`
	PrivateKeyPEM string `yaml:"private_key_pem" json:"private_key_pem"`

	// 运行时元数据，不从配置文件加载。
	WorkDir    string `yaml:"-" json:"-"`
	ConfigFile string `yaml:"-" json:"-"`
}

type LoggerConfig struct {
	Level string `yaml:"level" json:"level"`
}
