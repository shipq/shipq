package codegen

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ModuleInfo contains module path and relative subpath information.
// In a monorepo setup, the shipq project may be in a subdirectory of the Go module.
type ModuleInfo struct {
	ModulePath string // Module path from go.mod (e.g., "github.com/company/monorepo")
	SubPath    string // Relative path from go.mod to shipq root (e.g., "services/myservice"), empty if same dir
}

// FullImportPath returns the full import path for a package within the shipq project.
// For example, if ModulePath is "github.com/company/monorepo", SubPath is "services/myservice",
// and pkgPath is "migrations", returns "github.com/company/monorepo/services/myservice/migrations".
func (m *ModuleInfo) FullImportPath(pkgPath string) string {
	parts := []string{m.ModulePath}
	if m.SubPath != "" {
		parts = append(parts, m.SubPath)
	}
	if pkgPath != "" {
		parts = append(parts, pkgPath)
	}
	return strings.Join(parts, "/")
}

// GetModulePath reads go.mod and extracts the module path.
// The goModRoot parameter should be the directory containing go.mod.
func GetModulePath(goModRoot string) (string, error) {
	goModPath := filepath.Join(goModRoot, "go.mod")
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

// GetModuleInfo reads go.mod and calculates the subpath for a shipq project.
// goModRoot is the directory containing go.mod, shipqRoot is the directory containing shipq.ini.
// In a monorepo, shipqRoot may be a subdirectory of goModRoot.
func GetModuleInfo(goModRoot, shipqRoot string) (*ModuleInfo, error) {
	modulePath, err := GetModulePath(goModRoot)
	if err != nil {
		return nil, err
	}

	// Calculate the subpath (relative path from go.mod to shipq root)
	subPath := ""
	if goModRoot != shipqRoot {
		rel, err := filepath.Rel(goModRoot, shipqRoot)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate relative path: %w", err)
		}
		// Convert to forward slashes for import paths
		subPath = filepath.ToSlash(rel)
	}

	return &ModuleInfo{
		ModulePath: modulePath,
		SubPath:    subPath,
	}, nil
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
