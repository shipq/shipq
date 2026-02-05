package registry

// setDefaults applies default values to the CompileConfig.
// It modifies the config in place, setting defaults for any empty fields.
func setDefaults(cfg *CompileConfig) {
	if cfg.OutputPkg == "" {
		cfg.OutputPkg = "api"
	}
	if cfg.OutputDir == "" {
		cfg.OutputDir = "api"
	}
	if cfg.DBDialect == "" {
		cfg.DBDialect = "mysql"
	}
}
