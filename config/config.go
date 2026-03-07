package config

import (
	"strings"

	"github.com/shipq/shipq/inifile"
)

// Config holds general project configuration.
type Config struct {
	// What is the URL of the database?
	DatabaseURL string
}

// ServerConfig holds the [server] section from shipq.ini.
type ServerConfig struct {
	// StripPrefix is the URL prefix to strip from incoming requests.
	// For example, "/api" means a request to "/api/posts" is routed as "/posts".
	StripPrefix string
}

// ParseServerConfig extracts the [server] section from a parsed INI file.
// Returns nil when the [server] section is absent.
func ParseServerConfig(ini *inifile.File) *ServerConfig {
	section := ini.Section("server")
	if section == nil {
		return nil
	}

	stripPrefix := strings.TrimRight(strings.TrimSpace(section.Get("strip_prefix")), "/")

	return &ServerConfig{
		StripPrefix: stripPrefix,
	}
}

// LLMConfig holds the [llm] section from shipq.ini.
type LLMConfig struct {
	// ToolPkgs is the list of Go import paths for packages that export
	// Register(app *llm.App) functions.
	ToolPkgs []string
}

// ParseLLMConfig extracts the [llm] section from a parsed INI file.
// Returns nil (not an error) when the [llm] section is absent, indicating
// that LLM support is not enabled.
func ParseLLMConfig(ini *inifile.File) *LLMConfig {
	section := ini.Section("llm")
	if section == nil {
		return nil
	}

	raw := section.Get("tool_pkgs")
	toolPkgs := parseCommaSeparatedList(raw)

	return &LLMConfig{
		ToolPkgs: toolPkgs,
	}
}

// parseCommaSeparatedList splits a comma-separated string into trimmed,
// non-empty tokens. An empty input yields a nil slice.
func parseCommaSeparatedList(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}

	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
