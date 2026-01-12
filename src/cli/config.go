package cli

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// Config holds the portsql configuration.
type Config struct {
	Database DatabaseConfig
	Paths    PathsConfig
}

// DatabaseConfig holds database connection settings.
type DatabaseConfig struct {
	URL string
}

// PathsConfig holds file path settings.
type PathsConfig struct {
	Migrations  string
	Schematypes string
	QueriesIn   string
	QueriesOut  string
}

// DefaultConfig returns a Config with default values.
func DefaultConfig() *Config {
	return &Config{
		Database: DatabaseConfig{
			URL: os.Getenv("DATABASE_URL"),
		},
		Paths: PathsConfig{
			Migrations:  "migrations",
			Schematypes: "schematypes",
			QueriesIn:   "querydef",
			QueriesOut:  "queries",
		},
	}
}

// LoadConfig reads portsql.ini if present, falls back to defaults + DATABASE_URL.
// The configPath parameter specifies the directory to look for portsql.ini.
// If empty, it defaults to the current working directory.
func LoadConfig(configPath string) (*Config, error) {
	cfg := DefaultConfig()

	if configPath == "" {
		var err error
		configPath, err = os.Getwd()
		if err != nil {
			return nil, err
		}
	}

	iniPath := filepath.Join(configPath, "portsql.ini")
	if _, err := os.Stat(iniPath); os.IsNotExist(err) {
		// No config file, use defaults
		return cfg, nil
	}

	file, err := os.Open(iniPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	section := ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		// Section header
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(strings.Trim(line, "[]"))
			continue
		}

		// Key-value pair
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(strings.ToLower(parts[0]))
		value := strings.TrimSpace(parts[1])

		switch section {
		case "database":
			switch key {
			case "url":
				cfg.Database.URL = value
			}
		case "paths":
			switch key {
			case "migrations":
				cfg.Paths.Migrations = value
			case "schematypes":
				cfg.Paths.Schematypes = value
			case "queries_in":
				cfg.Paths.QueriesIn = value
			case "queries_out":
				cfg.Paths.QueriesOut = value
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// If database URL is still empty, try DATABASE_URL env var
	if cfg.Database.URL == "" {
		cfg.Database.URL = os.Getenv("DATABASE_URL")
	}

	return cfg, nil
}
