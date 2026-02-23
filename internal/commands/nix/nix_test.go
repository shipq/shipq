package nix

import (
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
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

func TestSha256SRIFormat(t *testing.T) {
	// Serve a known payload and verify the SRI string.
	payload := "hello nixpkgs"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(payload))
	}))
	defer ts.Close()

	client := ts.Client()
	sri, err := sha256SRI(client, ts.URL)
	if err != nil {
		t.Fatalf("sha256SRI returned error: %v", err)
	}

	// Must start with "sha256-"
	if !strings.HasPrefix(sri, "sha256-") {
		t.Fatalf("SRI string %q does not start with sha256-", sri)
	}

	// Decode the base64 portion and compare to expected digest.
	b64Part := strings.TrimPrefix(sri, "sha256-")
	decoded, err := base64.StdEncoding.DecodeString(b64Part)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}
	if len(decoded) != sha256.Size {
		t.Fatalf("decoded hash length = %d, want %d", len(decoded), sha256.Size)
	}

	expected := sha256.Sum256([]byte(payload))
	for i := range decoded {
		if decoded[i] != expected[i] {
			t.Fatalf("hash mismatch at byte %d", i)
		}
	}
}

func TestSha256SRIServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	client := ts.Client()
	_, err := sha256SRI(client, ts.URL)
	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}
}

func TestTemplateRender(t *testing.T) {
	data := shellNixData{
		Rev:            "abc123def456",
		SHA256:         "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
		DBPackages:     "postgresql_18",
		WorkerPackages: "valkey\n      centrifugo\n      minio\n      minio-client",
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

	// Verify all always-included packages are present.
	alwaysPackages := []string{
		"go", "git", "git-lfs", "glab", "gh", "opentofu", "goreman",
		"tmux", "lf", "tokei", "nix", "nixfmt-rfc-style", "ripgrep",
		"just", "neovim", "nodejs_24", "awscli2",
	}
	for _, pkg := range alwaysPackages {
		if !strings.Contains(rendered, pkg) {
			t.Errorf("rendered template missing always-included package %q", pkg)
		}
	}

	// Verify database package.
	if !strings.Contains(rendered, "postgresql_18") {
		t.Error("rendered template missing DB package postgresql_18")
	}

	// Verify worker packages.
	for _, pkg := range []string{"valkey", "centrifugo", "minio", "minio-client"} {
		if !strings.Contains(rendered, pkg) {
			t.Errorf("rendered template missing worker package %q", pkg)
		}
	}

	// Verify postgres DATABASE_URL appears.
	if !strings.Contains(rendered, "postgres://") {
		t.Error("rendered template missing postgres DATABASE_URL")
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

func TestTemplateRenderMySQL(t *testing.T) {
	data := shellNixData{
		Rev:            "deadbeef",
		SHA256:         "sha256-TEST=",
		DBPackages:     "mysql80",
		WorkerPackages: "",
	}

	rendered, err := renderShellNix(data)
	if err != nil {
		t.Fatalf("renderShellNix returned error: %v", err)
	}

	if !strings.Contains(rendered, "mysql80") {
		t.Error("rendered template missing mysql80 package")
	}
	if !strings.Contains(rendered, "mysql://") {
		t.Error("rendered template missing mysql DATABASE_URL")
	}

	// Worker packages section should NOT appear.
	if strings.Contains(rendered, "valkey") {
		t.Error("rendered template should not include worker packages")
	}
	if strings.Contains(rendered, "Workers / Centrifugo") {
		t.Error("rendered template should not include worker section header")
	}
}

func TestTemplateRenderSQLite(t *testing.T) {
	data := shellNixData{
		Rev:            "deadbeef",
		SHA256:         "sha256-TEST=",
		DBPackages:     "sqlite",
		WorkerPackages: "",
	}

	rendered, err := renderShellNix(data)
	if err != nil {
		t.Fatalf("renderShellNix returned error: %v", err)
	}

	if !strings.Contains(rendered, "sqlite") {
		t.Error("rendered template missing sqlite package")
	}
	if !strings.Contains(rendered, "sqlite://") {
		t.Error("rendered template missing sqlite DATABASE_URL")
	}
}

func TestDbPackagesForDialect(t *testing.T) {
	tests := []struct {
		dialect string
		want    string
	}{
		{"postgres", "postgresql_18"},
		{"mysql", "mysql80"},
		{"sqlite", "sqlite"},
		{"", "sqlite"},
		{"unknown", "sqlite"},
	}

	for _, tt := range tests {
		t.Run(tt.dialect, func(t *testing.T) {
			got := dbPackagesForDialect(tt.dialect)
			if got != tt.want {
				t.Errorf("dbPackagesForDialect(%q) = %q, want %q", tt.dialect, got, tt.want)
			}
		})
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
