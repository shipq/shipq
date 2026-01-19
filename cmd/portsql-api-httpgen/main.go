package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// outputPath holds the local directory path for writing output.
// This is separate from pkgPath which is the full import path.
var outputPath string

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// 1. Find and load config
	cfgPath, err := FindConfig()
	if err != nil {
		return err
	}
	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		return fmt.Errorf("loading config %s: %w", cfgPath, err)
	}

	// 2. Resolve the package path
	pkgPath := cfg.Package
	outputPath = cfg.Package // Default: output to same path as config specifies

	if strings.HasPrefix(pkgPath, "./") || strings.HasPrefix(pkgPath, "../") {
		// Relative path - convert to full module path for discovery
		// but keep the relative path for output
		resolved, err := resolvePackagePath(pkgPath)
		if err != nil {
			return fmt.Errorf("resolving package path %s: %w", pkgPath, err)
		}
		pkgPath = resolved
	} else {
		// Full module path - need to find the local directory
		localPath, err := resolveLocalPath(pkgPath)
		if err != nil {
			return fmt.Errorf("resolving local path for %s: %w", pkgPath, err)
		}
		outputPath = localPath
	}

	// 3. Discover endpoints
	manifest, err := Discover(pkgPath)
	if err != nil {
		return fmt.Errorf("discovering endpoints: %w", err)
	}

	// 4. Generate code
	pkgName := filepath.Base(outputPath)
	code, err := Generate(*manifest, pkgName)
	if err != nil {
		return fmt.Errorf("generating code: %w", err)
	}

	// 5. Write output
	outPath := filepath.Join(outputPath, "zz_generated_http.go")
	if err := os.WriteFile(outPath, []byte(code), 0644); err != nil {
		return fmt.Errorf("writing output %s: %w", outPath, err)
	}

	fmt.Printf("Generated %s with %d endpoint(s)\n", outPath, len(manifest.Endpoints))
	return nil
}

// resolvePackagePath converts a relative package path to a full module path.
func resolvePackagePath(relPath string) (string, error) {
	// Get the current module info
	cmd := exec.Command("go", "list", "-m", "-json")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to get module info: %w", err)
	}

	var mod struct {
		Path string `json:"Path"`
		Dir  string `json:"Dir"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &mod); err != nil {
		return "", fmt.Errorf("failed to parse module info: %w", err)
	}

	// Convert relative path to absolute
	absPath, err := filepath.Abs(relPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Calculate the relative path from module root
	relFromMod, err := filepath.Rel(mod.Dir, absPath)
	if err != nil {
		return "", fmt.Errorf("failed to get relative path from module: %w", err)
	}

	// Combine module path with relative path
	// Convert path separators to forward slashes for Go import paths
	relFromMod = filepath.ToSlash(relFromMod)
	return mod.Path + "/" + relFromMod, nil
}

// resolveLocalPath converts a full module package path to a local directory path.
func resolveLocalPath(pkgPath string) (string, error) {
	// Get the current module info
	cmd := exec.Command("go", "list", "-m", "-json")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to get module info: %w", err)
	}

	var mod struct {
		Path string `json:"Path"`
		Dir  string `json:"Dir"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &mod); err != nil {
		return "", fmt.Errorf("failed to parse module info: %w", err)
	}

	// If the package is within the current module, calculate the local path
	if strings.HasPrefix(pkgPath, mod.Path) {
		relPath := strings.TrimPrefix(pkgPath, mod.Path)
		relPath = strings.TrimPrefix(relPath, "/")
		return filepath.Join(mod.Dir, relPath), nil
	}

	// Package is not in current module - this is an error for now
	return "", fmt.Errorf("package %s is not within current module %s", pkgPath, mod.Path)
}
