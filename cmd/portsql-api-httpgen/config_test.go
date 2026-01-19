package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	t.Run("loads valid config", func(t *testing.T) {
		tmpDir := t.TempDir()
		cfgPath := filepath.Join(tmpDir, "test.ini")
		if err := os.WriteFile(cfgPath, []byte("[httpgen]\npackage = ./api\n"), 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		cfg, err := LoadConfig(cfgPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Package != "./api" {
			t.Errorf("got Package %q, want %q", cfg.Package, "./api")
		}
	})

	t.Run("missing section", func(t *testing.T) {
		tmpDir := t.TempDir()
		cfgPath := filepath.Join(tmpDir, "test.ini")
		if err := os.WriteFile(cfgPath, []byte("package = ./api\n"), 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		_, err := LoadConfig(cfgPath)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "[httpgen]") {
			t.Errorf("expected error to contain '[httpgen]', got %q", err.Error())
		}
	})

	t.Run("missing package key", func(t *testing.T) {
		tmpDir := t.TempDir()
		cfgPath := filepath.Join(tmpDir, "test.ini")
		if err := os.WriteFile(cfgPath, []byte("[httpgen]\n"), 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		_, err := LoadConfig(cfgPath)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "package") {
			t.Errorf("expected error to contain 'package', got %q", err.Error())
		}
	})

	t.Run("trims whitespace", func(t *testing.T) {
		tmpDir := t.TempDir()
		cfgPath := filepath.Join(tmpDir, "test.ini")
		if err := os.WriteFile(cfgPath, []byte("[httpgen]\npackage =   ./api  \n"), 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		cfg, err := LoadConfig(cfgPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Package != "./api" {
			t.Errorf("got Package %q, want %q", cfg.Package, "./api")
		}
	})

	t.Run("ignores other sections", func(t *testing.T) {
		tmpDir := t.TempDir()
		cfgPath := filepath.Join(tmpDir, "test.ini")
		if err := os.WriteFile(cfgPath, []byte("[other]\nfoo = bar\n[httpgen]\npackage = ./myapi\n[another]\nbaz = qux\n"), 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		cfg, err := LoadConfig(cfgPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Package != "./myapi" {
			t.Errorf("got Package %q, want %q", cfg.Package, "./myapi")
		}
	})

	t.Run("ignores comments", func(t *testing.T) {
		tmpDir := t.TempDir()
		cfgPath := filepath.Join(tmpDir, "test.ini")
		if err := os.WriteFile(cfgPath, []byte("# comment\n[httpgen]\n; another comment\npackage = ./api\n"), 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		cfg, err := LoadConfig(cfgPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Package != "./api" {
			t.Errorf("got Package %q, want %q", cfg.Package, "./api")
		}
	})

	t.Run("error if file not found", func(t *testing.T) {
		_, err := LoadConfig("/nonexistent/path/config.ini")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestFindConfig(t *testing.T) {
	t.Run("uses env var if set", func(t *testing.T) {
		// Create temp file
		tmpDir := t.TempDir()
		cfgPath := filepath.Join(tmpDir, "custom.ini")
		if err := os.WriteFile(cfgPath, []byte("[httpgen]\npackage = ./api\n"), 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		// Set env var
		t.Setenv("PORTSQL_API_HTTPGEN_CONFIG", cfgPath)

		path, err := FindConfig()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if path != cfgPath {
			t.Errorf("got path %q, want %q", path, cfgPath)
		}
	})

	t.Run("falls back to ./portsql-api-httpgen.ini", func(t *testing.T) {
		// Create temp dir with ini file
		tmpDir := t.TempDir()
		cfgPath := filepath.Join(tmpDir, "portsql-api-httpgen.ini")
		if err := os.WriteFile(cfgPath, []byte("[httpgen]\npackage = ./api\n"), 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		// Change to temp dir
		origDir, err := os.Getwd()
		if err != nil {
			t.Fatalf("failed to get working directory: %v", err)
		}
		defer os.Chdir(origDir)

		if err := os.Chdir(tmpDir); err != nil {
			t.Fatalf("failed to change directory: %v", err)
		}

		// Clear env var
		t.Setenv("PORTSQL_API_HTTPGEN_CONFIG", "")

		path, err := FindConfig()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if path != "./portsql-api-httpgen.ini" {
			t.Errorf("got path %q, want %q", path, "./portsql-api-httpgen.ini")
		}
	})

	t.Run("error if no config found", func(t *testing.T) {
		// Empty temp dir
		tmpDir := t.TempDir()

		// Change to temp dir
		origDir, err := os.Getwd()
		if err != nil {
			t.Fatalf("failed to get working directory: %v", err)
		}
		defer os.Chdir(origDir)

		if err := os.Chdir(tmpDir); err != nil {
			t.Fatalf("failed to change directory: %v", err)
		}

		// Clear env var
		t.Setenv("PORTSQL_API_HTTPGEN_CONFIG", "")

		_, err = FindConfig()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "config not found") {
			t.Errorf("expected error to contain 'config not found', got %q", err.Error())
		}
	})
}
