package codegen

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GetModulePath reads go.mod and extracts the module path.
func GetModulePath(projectRoot string) (string, error) {
	goModPath := filepath.Join(projectRoot, "go.mod")
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return "", fmt.Errorf("failed to read go.mod: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			modulePath := strings.TrimPrefix(line, "module ")
			return strings.TrimSpace(modulePath), nil
		}
	}
	return "", fmt.Errorf("module declaration not found in go.mod")
}

// EnsureDir creates a directory and all parent directories if they don't exist.
func EnsureDir(path string) error {
	return os.MkdirAll(path, 0755)
}

// WriteFileIfChanged writes content to a file only if it differs from existing content.
// Returns true if the file was written, false if unchanged.
func WriteFileIfChanged(path string, content []byte) (bool, error) {
	existing, err := os.ReadFile(path)
	if err == nil && string(existing) == string(content) {
		return false, nil
	}

	if err := os.WriteFile(path, content, 0644); err != nil {
		return false, err
	}
	return true, nil
}
