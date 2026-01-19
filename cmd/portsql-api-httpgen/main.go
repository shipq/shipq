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

	// 3. Resolve middleware package path (if configured)
	middlewarePkgPath := ""
	if cfg.MiddlewarePackage != "" {
		if strings.HasPrefix(cfg.MiddlewarePackage, "./") || strings.HasPrefix(cfg.MiddlewarePackage, "../") {
			// Relative path - convert to full module path
			resolved, err := resolvePackagePath(cfg.MiddlewarePackage)
			if err != nil {
				return fmt.Errorf("resolving middleware package path %s: %w", cfg.MiddlewarePackage, err)
			}
			middlewarePkgPath = resolved
		} else {
			// Already a full module path
			middlewarePkgPath = cfg.MiddlewarePackage
		}
	}

	// 4. Discover endpoints
	manifest, err := Discover(pkgPath, middlewarePkgPath)
	if err != nil {
		return fmt.Errorf("discovering endpoints: %w", err)
	}

	// 5. Generate code
	pkgName := filepath.Base(outputPath)
	code, err := Generate(*manifest, pkgName, pkgPath)
	if err != nil {
		return fmt.Errorf("generating code: %w", err)
	}

	// 6. Write output
	outPath := filepath.Join(outputPath, "zz_generated_http.go")
	if err := os.WriteFile(outPath, []byte(code), 0644); err != nil {
		return fmt.Errorf("writing output %s: %w", outPath, err)
	}

	fmt.Printf("Generated %s with %d endpoint(s)\n", outPath, len(manifest.Endpoints))

	// 7. Generate middleware context helpers if context keys are provided
	if len(manifest.ContextKeys) > 0 && middlewarePkgPath != "" {
		// Resolve local path for middleware package
		middlewareLocalPath := ""
		if strings.HasPrefix(cfg.MiddlewarePackage, "./") || strings.HasPrefix(cfg.MiddlewarePackage, "../") {
			// Relative path
			absPath, err := filepath.Abs(cfg.MiddlewarePackage)
			if err != nil {
				return fmt.Errorf("resolving middleware package path: %w", err)
			}
			middlewareLocalPath = absPath
		} else {
			// Full module path - resolve to local
			localPath, err := resolveLocalPath(middlewarePkgPath)
			if err != nil {
				return fmt.Errorf("resolving local path for middleware package: %w", err)
			}
			middlewareLocalPath = localPath
		}

		middlewarePkgName := filepath.Base(middlewareLocalPath)
		contextCode, err := generateMiddlewareContextFile(middlewarePkgName, manifest.ContextKeys)
		if err != nil {
			return fmt.Errorf("generating middleware context helpers: %w", err)
		}

		if contextCode != "" {
			contextOutPath := filepath.Join(middlewareLocalPath, "zz_generated_middleware_context.go")
			if err := os.WriteFile(contextOutPath, []byte(contextCode), 0644); err != nil {
				return fmt.Errorf("writing middleware context output %s: %w", contextOutPath, err)
			}
			fmt.Printf("Generated %s with %d context helper(s)\n", contextOutPath, len(manifest.ContextKeys))
		}
	}

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
