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
	// DBDialect is intentionally NOT defaulted to "mysql" here.
	// If dialect inference failed (e.g. the user's database_url format
	// isn't recognized), we want CompileRegistry to surface a clear error
	// rather than silently importing the wrong driver.
}
