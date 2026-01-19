package main

import (
	"errors"
	"os"

	"github.com/shipq/shipq/inifile"
)

// Config holds the parsed configuration for portsql-api-httpgen.
type Config struct {
	Package string // e.g. "./api"
}

// FindConfig locates the configuration file.
// It first checks the PORTSQL_API_HTTPGEN_CONFIG environment variable,
// then falls back to ./portsql-api-httpgen.ini in the current directory.
func FindConfig() (string, error) {
	if p := os.Getenv("PORTSQL_API_HTTPGEN_CONFIG"); p != "" {
		return p, nil
	}
	if _, err := os.Stat("./portsql-api-httpgen.ini"); err == nil {
		return "./portsql-api-httpgen.ini", nil
	}
	return "", errors.New("config not found: set PORTSQL_API_HTTPGEN_CONFIG or create ./portsql-api-httpgen.ini")
}

// LoadConfig reads and parses a config file from the given path.
func LoadConfig(path string) (*Config, error) {
	f, err := inifile.ParseFile(path)
	if err != nil {
		return nil, err
	}

	pkg := f.Get("httpgen", "package")
	if pkg == "" {
		return nil, errors.New("missing [httpgen] section with 'package' key")
	}

	return &Config{Package: pkg}, nil
}
