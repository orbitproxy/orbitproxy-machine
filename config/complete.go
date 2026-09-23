package config

const defaultLogLevel = "info"

// Complete 为未设置的字段填充默认值。
func (cfg *Config) Complete() error {
	cfg.Logger.Complete()
	return nil
}

func (cfg *LoggerConfig) Complete() {
	cfg.Level = emptyOr(cfg.Level, defaultLogLevel)
}
