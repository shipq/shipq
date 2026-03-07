package nix

import (
	"errors"
	"strings"
	"testing"
)

func TestPickLatestStable(t *testing.T) {
	tests := []struct {
		name       string
		branches   []branch
		wantName   string
		wantSHA    string
		wantErr    bool
		errContain string
	}{
		{
			name: "single stable branch",
			branches: []branch{
				{Name: "nixos-24.05", Commit: struct {
					SHA string `json:"sha"`
				}{SHA: "abc123"}},
			},
			wantName: "nixos-24.05",
			wantSHA:  "abc123",
		},
		{
			name: "picks highest version",
			branches: []branch{
				{Name: "nixos-23.11", Commit: struct {
					SHA string `json:"sha"`
				}{SHA: "old111"}},
				{Name: "nixos-24.05", Commit: struct {
					SHA string `json:"sha"`
				}{SHA: "mid222"}},
				{Name: "nixos-24.11", Commit: struct {
					SHA string `json:"sha"`
				}{SHA: "new333"}},
			},
			wantName: "nixos-24.11",
			wantSHA:  "new333",
		},
		{
			name: "ignores non-stable branches",
			branches: []branch{
				{Name: "master", Commit: struct {
					SHA string `json:"sha"`
				}{SHA: "m1"}},
				{Name: "nixos-unstable", Commit: struct {
					SHA string `json:"sha"`
				}{SHA: "u1"}},
				{Name: "nixos-24.05", Commit: struct {
					SHA string `json:"sha"`
				}{SHA: "s1"}},
				{Name: "staging", Commit: struct {
					SHA string `json:"sha"`
				}{SHA: "st1"}},
				{Name: "nixos-24.05-small", Commit: struct {
					SHA string `json:"sha"`
				}{SHA: "sm1"}},
			},
			wantName: "nixos-24.05",
			wantSHA:  "s1",
		},
		{
			name: "year wrapping - higher year wins",
			branches: []branch{
				{Name: "nixos-23.11", Commit: struct {
					SHA string `json:"sha"`
				}{SHA: "a1"}},
				{Name: "nixos-25.05", Commit: struct {
					SHA string `json:"sha"`
				}{SHA: "b2"}},
			},
			wantName: "nixos-25.05",
			wantSHA:  "b2",
		},
		{
			name: "no stable branches at all",
			branches: []branch{{Name: "master", Commit: struct {
				SHA string `json:"sha"`
			}{SHA: "x"}}},
			wantErr:    true,
			errContain: "no stable nixos-YY.MM branches found",
		},
		{
			name:       "empty branch list",
			branches:   []branch{},
			wantErr:    true,
			errContain: "no stable nixos-YY.MM branches found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, sha, err := pickLatestStable(tt.branches)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error but got nil")
				}
				if tt.errContain != "" && !strings.Contains(err.Error(), tt.errContain) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errContain)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if name != tt.wantName {
				t.Errorf("branch name = %q, want %q", name, tt.wantName)
			}
			if sha != tt.wantSHA {
				t.Errorf("commit SHA = %q, want %q", sha, tt.wantSHA)
			}
		})
	}
}

func TestNixPrefetchSRI(t *testing.T) {
	// Save originals and restore after test.
	origPrefetch := nixPrefetchURL
	origHashToSRI := nixHashToSRI
	t.Cleanup(func() {
		nixPrefetchURL = origPrefetch
		nixHashToSRI = origHashToSRI
	})

	t.Run("success", func(t *testing.T) {
		nixPrefetchURL = func(tarURL string) (string, error) {
			if !strings.Contains(tarURL, "deadbeef") {
				t.Errorf("unexpected tarURL: %s", tarURL)
			}
			return "0v6bd1xk8a2aal83karlvc853x44dg1n4nk08jg3dajqyy0s98np", nil
		}
		nixHashToSRI = func(nix32 string) (string, error) {
			if nix32 != "0v6bd1xk8a2aal83karlvc853x44dg1n4nk08jg3dajqyy0s98np" {
				t.Errorf("unexpected nix32 hash: %s", nix32)
			}
			return "sha256-16KkgfdYqjaeRGBaYsNrhPRRENs0qzkQVUooNHtoy2w=", nil
		}

		sri, err := nixPrefetchSRI("https://github.com/NixOS/nixpkgs/archive/deadbeef.tar.gz")
		if err != nil {
			t.Fatalf("nixPrefetchSRI returned error: %v", err)
		}
		if !strings.HasPrefix(sri, "sha256-") {
			t.Errorf("SRI string %q does not start with sha256-", sri)
		}
		if sri != "sha256-16KkgfdYqjaeRGBaYsNrhPRRENs0qzkQVUooNHtoy2w=" {
			t.Errorf("unexpected SRI: %s", sri)
		}
	})

	t.Run("prefetch error", func(t *testing.T) {
		nixPrefetchURL = func(tarURL string) (string, error) {
			return "", errors.New("nix-prefetch-url not found")
		}
		nixHashToSRI = func(nix32 string) (string, error) {
			t.Fatal("nixHashToSRI should not be called when prefetch fails")
			return "", nil
		}

		_, err := nixPrefetchSRI("https://example.com/bad.tar.gz")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "nix-prefetch-url not found") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("hash-to-sri error", func(t *testing.T) {
		nixPrefetchURL = func(tarURL string) (string, error) {
			return "somehash", nil
		}
		nixHashToSRI = func(nix32 string) (string, error) {
			return "", errors.New("nix-hash not found")
		}

		_, err := nixPrefetchSRI("https://example.com/test.tar.gz")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "nix-hash not found") {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestTemplateRender(t *testing.T) {
	data := shellNixData{
		Rev:    "abc123def456",
		SHA256: "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
	}

	rendered, err := renderShellNix(data)
	if err != nil {
		t.Fatalf("renderShellNix returned error: %v", err)
	}

	// Verify the output contains the fetchTarball block.
	if !strings.Contains(rendered, "fetchTarball") {
		t.Error("rendered template missing fetchTarball")
	}

	// Verify rev and hash appear in the output.
	if !strings.Contains(rendered, data.Rev) {
		t.Errorf("rendered template missing rev %q", data.Rev)
	}
	if !strings.Contains(rendered, data.SHA256) {
		t.Errorf("rendered template missing SHA256 %q", data.SHA256)
	}

	// Verify sha256 attribute is present in fetchTarball.
	if !strings.Contains(rendered, "sha256 = ") {
		t.Error("rendered template missing sha256 attribute in fetchTarball")
	}

	// Verify packages list is empty.
	if strings.Contains(rendered, "packages = with pkgs") {
		t.Error("rendered template should not use 'with pkgs' (empty package list)")
	}
	if !strings.Contains(rendered, "packages = [") {
		t.Error("rendered template missing 'packages = ['")
	}

	// Verify no hardcoded packages appear.
	hardcodedPackages := []string{
		"go", "git-lfs", "glab", "gh", "opentofu", "goreman",
		"tmux", "lf", "tokei", "nixfmt-rfc-style", "ripgrep",
		"just", "neovim", "nodejs_24", "awscli2",
		"postgresql_18", "mysql80", "sqlite",
		"valkey", "centrifugo", "minio", "minio-client",
	}
	for _, pkg := range hardcodedPackages {
		if strings.Contains(rendered, pkg) {
			t.Errorf("rendered template should not contain package %q", pkg)
		}
	}

	// Verify no DATABASE_URL is set (no DB-specific logic).
	if strings.Contains(rendered, "DATABASE_URL") {
		t.Error("rendered template should not contain DATABASE_URL")
	}

	// Verify shellHook is present.
	if !strings.Contains(rendered, "shellHook") {
		t.Error("rendered template missing shellHook")
	}

	// Verify PROJECT_ROOT is exported.
	if !strings.Contains(rendered, "PROJECT_ROOT") {
		t.Error("rendered template missing PROJECT_ROOT")
	}

	// Verify COOKIE_SECRET.
	if !strings.Contains(rendered, "COOKIE_SECRET=supersecret") {
		t.Error("rendered template missing COOKIE_SECRET")
	}

	// Verify unset DEVELOPER_DIR.
	if !strings.Contains(rendered, "unset DEVELOPER_DIR") {
		t.Error("rendered template missing unset DEVELOPER_DIR")
	}

	// Verify basic Nix syntax: let ... in ... mkShell.
	if !strings.Contains(rendered, "let") {
		t.Error("rendered template missing 'let'")
	}
	if !strings.Contains(rendered, "pkgs.mkShell") {
		t.Error("rendered template missing 'pkgs.mkShell'")
	}
}

func TestAtoi2(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"00", 0},
		{"05", 5},
		{"11", 11},
		{"24", 24},
		{"99", 99},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := atoi2(tt.input)
			if got != tt.want {
				t.Errorf("atoi2(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}
