package discovery

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverChannelPackages_FindsRegisterFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create channels/email with register.go
	emailDir := filepath.Join(tmpDir, "channels", "email")
	if err := os.MkdirAll(emailDir, 0755); err != nil {
		t.Fatalf("failed to create email dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(emailDir, "register.go"), []byte("package email\n"), 0644); err != nil {
		t.Fatalf("failed to create email register.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(emailDir, "types.go"), []byte("package email\n"), 0644); err != nil {
		t.Fatalf("failed to create email types.go: %v", err)
	}

	// Create channels/chatbot with register.go
	chatbotDir := filepath.Join(tmpDir, "channels", "chatbot")
	if err := os.MkdirAll(chatbotDir, 0755); err != nil {
		t.Fatalf("failed to create chatbot dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(chatbotDir, "register.go"), []byte("package chatbot\n"), 0644); err != nil {
		t.Fatalf("failed to create chatbot register.go: %v", err)
	}

	pkgs, err := DiscoverChannelPackages(tmpDir, tmpDir, "example.com/myapp")
	if err != nil {
		t.Fatalf("DiscoverChannelPackages failed: %v", err)
	}

	if len(pkgs) != 2 {
		t.Fatalf("expected 2 packages, got %d: %v", len(pkgs), pkgs)
	}

	expectedPkgs := map[string]bool{
		"example.com/myapp/channels/email":   false,
		"example.com/myapp/channels/chatbot": false,
	}

	for _, pkg := range pkgs {
		if _, ok := expectedPkgs[pkg]; !ok {
			t.Errorf("unexpected package: %s", pkg)
		}
		expectedPkgs[pkg] = true
	}

	for pkg, found := range expectedPkgs {
		if !found {
			t.Errorf("missing expected package: %s", pkg)
		}
	}
}

func TestDiscoverChannelPackages_SkipsWithoutRegister(t *testing.T) {
	tmpDir := t.TempDir()

	// Create channels/utils with helper.go but NO register.go
	utilsDir := filepath.Join(tmpDir, "channels", "utils")
	if err := os.MkdirAll(utilsDir, 0755); err != nil {
		t.Fatalf("failed to create utils dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(utilsDir, "helper.go"), []byte("package utils\n"), 0644); err != nil {
		t.Fatalf("failed to create helper.go: %v", err)
	}

	// Create channels/email WITH register.go
	emailDir := filepath.Join(tmpDir, "channels", "email")
	if err := os.MkdirAll(emailDir, 0755); err != nil {
		t.Fatalf("failed to create email dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(emailDir, "register.go"), []byte("package email\n"), 0644); err != nil {
		t.Fatalf("failed to create register.go: %v", err)
	}

	pkgs, err := DiscoverChannelPackages(tmpDir, tmpDir, "example.com/myapp")
	if err != nil {
		t.Fatalf("DiscoverChannelPackages failed: %v", err)
	}

	// Should find only email, not utils
	if len(pkgs) != 1 {
		t.Fatalf("expected 1 package, got %d: %v", len(pkgs), pkgs)
	}

	expected := "example.com/myapp/channels/email"
	if pkgs[0] != expected {
		t.Errorf("expected package %q, got %q", expected, pkgs[0])
	}
}

func TestDiscoverChannelPackages_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Create empty channels/ directory
	channelsDir := filepath.Join(tmpDir, "channels")
	if err := os.MkdirAll(channelsDir, 0755); err != nil {
		t.Fatalf("failed to create channels dir: %v", err)
	}

	pkgs, err := DiscoverChannelPackages(tmpDir, tmpDir, "example.com/myapp")
	if err != nil {
		t.Fatalf("DiscoverChannelPackages failed: %v", err)
	}

	if len(pkgs) != 0 {
		t.Errorf("expected 0 packages, got %d: %v", len(pkgs), pkgs)
	}
}

func TestDiscoverChannelPackages_MissingDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Don't create channels/ directory at all
	pkgs, err := DiscoverChannelPackages(tmpDir, tmpDir, "example.com/myapp")
	if err != nil {
		t.Fatalf("DiscoverChannelPackages failed: %v", err)
	}

	if len(pkgs) != 0 {
		t.Errorf("expected 0 packages, got %d: %v", len(pkgs), pkgs)
	}
}

func TestDiscoverChannelPackages_SkipsRootChannelsDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Create channels/ root directory with a Go file but NO register.go.
	// This simulates a shared types file in the channels root.
	channelsDir := filepath.Join(tmpDir, "channels")
	if err := os.MkdirAll(channelsDir, 0755); err != nil {
		t.Fatalf("failed to create channels dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(channelsDir, "shared.go"), []byte("package channels\n"), 0644); err != nil {
		t.Fatalf("failed to create shared.go: %v", err)
	}

	// Create channels/email WITH register.go
	emailDir := filepath.Join(channelsDir, "email")
	if err := os.MkdirAll(emailDir, 0755); err != nil {
		t.Fatalf("failed to create email dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(emailDir, "register.go"), []byte("package email\n"), 0644); err != nil {
		t.Fatalf("failed to create register.go: %v", err)
	}

	pkgs, err := DiscoverChannelPackages(tmpDir, tmpDir, "example.com/myapp")
	if err != nil {
		t.Fatalf("DiscoverChannelPackages failed: %v", err)
	}

	// Should find only email, not the root channels/ dir
	if len(pkgs) != 1 {
		t.Fatalf("expected 1 package, got %d: %v", len(pkgs), pkgs)
	}

	expected := "example.com/myapp/channels/email"
	if pkgs[0] != expected {
		t.Errorf("expected package %q, got %q", expected, pkgs[0])
	}
}

func TestDiscoverChannelPackages_MonorepoSetup(t *testing.T) {
	tmpDir := t.TempDir()

	goModRoot := tmpDir
	shipqRoot := filepath.Join(tmpDir, "services", "myservice")

	// Create directories
	emailDir := filepath.Join(shipqRoot, "channels", "email")
	if err := os.MkdirAll(emailDir, 0755); err != nil {
		t.Fatalf("failed to create directories: %v", err)
	}

	// Create go.mod in root
	if err := os.WriteFile(filepath.Join(goModRoot, "go.mod"), []byte("module github.com/company/monorepo\n"), 0644); err != nil {
		t.Fatalf("failed to create go.mod: %v", err)
	}

	// Create register.go
	if err := os.WriteFile(filepath.Join(emailDir, "register.go"), []byte("package email\n"), 0644); err != nil {
		t.Fatalf("failed to create register.go: %v", err)
	}

	modulePath := "github.com/company/monorepo"
	pkgs, err := DiscoverChannelPackages(goModRoot, shipqRoot, modulePath)
	if err != nil {
		t.Fatalf("DiscoverChannelPackages failed: %v", err)
	}

	if len(pkgs) != 1 {
		t.Fatalf("expected 1 package, got %d: %v", len(pkgs), pkgs)
	}

	expected := "github.com/company/monorepo/services/myservice/channels/email"
	if pkgs[0] != expected {
		t.Errorf("expected package %q, got %q", expected, pkgs[0])
	}
}
