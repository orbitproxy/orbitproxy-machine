package config

// Config is persisted under the workdir as orbitproxy.yml.
type Config struct {
	Logger LoggerConfig `yaml:"logger,omitempty" json:"logger"`

	MachineKey    string `yaml:"machine_key" json:"machine_key"`
	Type          string `yaml:"type,omitempty" json:"type,omitempty"`
	EdgeAddr      string `yaml:"edge_addr,omitempty" json:"edge_addr,omitempty"`
	MachineCACert string `yaml:"machine_ca_cert,omitempty" json:"machine_ca_cert,omitempty"`
	TLSServerName string `yaml:"tls_server_name,omitempty" json:"tls_server_name,omitempty"`
	PublicKeyPEM  string `yaml:"public_key_pem" json:"public_key_pem"`
	PrivateKeyPEM string `yaml:"private_key_pem" json:"private_key_pem"`

	// 运行时元数据，不从配置文件加载。
	WorkDir    string `yaml:"-" json:"-"`
	ConfigFile string `yaml:"-" json:"-"`
}

type LoggerConfig struct {
	Level string `yaml:"level" json:"level"`
}
