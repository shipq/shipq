package docker

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ─── TestPickLatestAlpine ───────────────────────────────────────────

func TestPickLatestAlpine(t *testing.T) {
	tests := []struct {
		name       string
		candidates []alpineCandidate
		want       string
		wantErr    bool
	}{
		{
			name: "single candidate",
			candidates: []alpineCandidate{
				{Tag: "3.20", Major: 3, Minor: 20, Score: 3020},
			},
			want: "3.20",
		},
		{
			name: "picks highest version",
			candidates: []alpineCandidate{
				{Tag: "3.22", Major: 3, Minor: 22, Score: 3022},
				{Tag: "3.21", Major: 3, Minor: 21, Score: 3021},
				{Tag: "3.20", Major: 3, Minor: 20, Score: 3020},
			},
			want: "3.22",
		},
		{
			name:       "empty list returns error",
			candidates: []alpineCandidate{},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := pickLatestAlpine(tt.candidates)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error but got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("pickLatestAlpine() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ─── TestFetchAlpineCandidates ──────────────────────────────────────

func TestFetchAlpineCandidatesFiltersTags(t *testing.T) {
	tags := tagsPage{
		Next: nil,
		Results: []tagResult{
			{
				Name:   "3.22",
				Images: []tagImage{{Architecture: "amd64", OS: "linux"}},
			},
			{
				Name:   "3.21",
				Images: []tagImage{{Architecture: "amd64", OS: "linux"}, {Architecture: "arm64", OS: "linux"}},
			},
			{
				Name:   "3.22.1",
				Images: []tagImage{{Architecture: "amd64", OS: "linux"}},
			},
			{
				Name:   "20231219",
				Images: []tagImage{{Architecture: "amd64", OS: "linux"}},
			},
			{
				Name:   "edge",
				Images: []tagImage{{Architecture: "amd64", OS: "linux"}},
			},
			{
				Name:   "latest",
				Images: []tagImage{{Architecture: "amd64", OS: "linux"}},
			},
			{
				Name:   "3.18",
				Images: []tagImage{{Architecture: "amd64", OS: "linux"}},
			},
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tags)
	}))
	defer ts.Close()

	client := ts.Client()
	candidates, err := fetchAlpineCandidatesFromURL(client, ts.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(candidates) != 3 {
		t.Fatalf("got %d candidates, want 3; candidates: %v", len(candidates), candidates)
	}

	if candidates[0].Tag != "3.22" {
		t.Errorf("candidates[0].Tag = %q, want %q", candidates[0].Tag, "3.22")
	}
	if candidates[1].Tag != "3.21" {
		t.Errorf("candidates[1].Tag = %q, want %q", candidates[1].Tag, "3.21")
	}
	if candidates[2].Tag != "3.18" {
		t.Errorf("candidates[2].Tag = %q, want %q", candidates[2].Tag, "3.18")
	}
}

func TestFetchAlpineCandidatesPagination(t *testing.T) {
	mux := http.NewServeMux()
	var ts *httptest.Server

	mux.HandleFunc("/page1", func(w http.ResponseWriter, r *http.Request) {
		next := ts.URL + "/page2"
		page := tagsPage{
			Next: &next,
			Results: []tagResult{
				{
					Name:   "3.20",
					Images: []tagImage{{Architecture: "amd64", OS: "linux"}},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(page)
	})
	mux.HandleFunc("/page2", func(w http.ResponseWriter, r *http.Request) {
		page := tagsPage{
			Next: nil,
			Results: []tagResult{
				{
					Name:   "3.21",
					Images: []tagImage{{Architecture: "amd64", OS: "linux"}},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(page)
	})

	ts = httptest.NewServer(mux)
	defer ts.Close()

	client := ts.Client()
	candidates, err := fetchAlpineCandidatesFromURL(client, ts.URL+"/page1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(candidates) != 2 {
		t.Fatalf("got %d candidates, want 2", len(candidates))
	}

	if candidates[0].Tag != "3.21" {
		t.Errorf("candidates[0].Tag = %q, want %q", candidates[0].Tag, "3.21")
	}
	if candidates[1].Tag != "3.20" {
		t.Errorf("candidates[1].Tag = %q, want %q", candidates[1].Tag, "3.20")
	}
}

// ─── TestAmd64Validation ────────────────────────────────────────────

func TestAmd64Validation(t *testing.T) {
	tags := tagsPage{
		Next: nil,
		Results: []tagResult{
			{
				Name:   "3.22",
				Images: []tagImage{{Architecture: "arm64", OS: "linux"}},
			},
			{
				Name:   "3.21",
				Images: []tagImage{{Architecture: "amd64", OS: "linux"}},
			},
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tags)
	}))
	defer ts.Close()

	client := ts.Client()
	candidates, err := fetchAlpineCandidatesFromURL(client, ts.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(candidates) != 1 {
		t.Fatalf("got %d candidates, want 1", len(candidates))
	}
	if candidates[0].Tag != "3.21" {
		t.Errorf("candidates[0].Tag = %q, want %q", candidates[0].Tag, "3.21")
	}
}

func TestHasAmd64Image(t *testing.T) {
	tests := []struct {
		name   string
		tag    tagResult
		expect bool
	}{
		{
			name: "has amd64",
			tag: tagResult{
				Name:   "3.22",
				Images: []tagImage{{Architecture: "amd64", OS: "linux"}, {Architecture: "arm64", OS: "linux"}},
			},
			expect: true,
		},
		{
			name: "only arm64",
			tag: tagResult{
				Name:   "3.22",
				Images: []tagImage{{Architecture: "arm64", OS: "linux"}},
			},
			expect: false,
		},
		{
			name: "empty images",
			tag: tagResult{
				Name:   "3.22",
				Images: []tagImage{},
			},
			expect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasAmd64Image(tt.tag)
			if got != tt.expect {
				t.Errorf("hasAmd64Image() = %v, want %v", got, tt.expect)
			}
		})
	}
}

// ─── TestGolangAlpineTagCheck ───────────────────────────────────────

func TestGolangAlpineTagExists(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "1.25-alpine3.22") {
			tag := tagResult{
				Name:   "1.25-alpine3.22",
				Images: []tagImage{{Architecture: "amd64", OS: "linux"}},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(tag)
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	client := ts.Client()

	exists, err := golangAlpineTagExistsFromBase(client, ts.URL+"/", "1.25", "3.22")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Error("expected tag to exist but it did not")
	}

	exists, err = golangAlpineTagExistsFromBase(client, ts.URL+"/", "1.25", "3.21")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Error("expected tag not to exist but it did")
	}
}

func TestGolangAlpineTagFallback(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "1.25-alpine3.21") {
			tag := tagResult{
				Name:   "1.25-alpine3.21",
				Images: []tagImage{{Architecture: "amd64", OS: "linux"}},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(tag)
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	candidates := []alpineCandidate{
		{Tag: "3.22", Major: 3, Minor: 22, Score: 3022},
		{Tag: "3.21", Major: 3, Minor: 21, Score: 3021},
		{Tag: "3.20", Major: 3, Minor: 20, Score: 3020},
	}

	client := ts.Client()
	var resolved string
	for _, c := range candidates {
		exists, err := golangAlpineTagExistsFromBase(client, ts.URL+"/", "1.25", c.Tag)
		if err != nil {
			continue
		}
		if exists {
			resolved = c.Tag
			break
		}
	}

	if resolved != "3.21" {
		t.Errorf("resolved = %q, want %q", resolved, "3.21")
	}
}

func TestGolangAlpineTagNoMatch(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer ts.Close()

	candidates := []alpineCandidate{
		{Tag: "3.22", Major: 3, Minor: 22, Score: 3022},
		{Tag: "3.21", Major: 3, Minor: 21, Score: 3021},
	}

	client := ts.Client()
	var resolved string
	for _, c := range candidates {
		exists, err := golangAlpineTagExistsFromBase(client, ts.URL+"/", "1.25", c.Tag)
		if err != nil {
			continue
		}
		if exists {
			resolved = c.Tag
			break
		}
	}

	if resolved != "" {
		t.Errorf("expected no resolution, got %q", resolved)
	}
}

func TestGolangAlpineTagMissingAmd64(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tag := tagResult{
			Name:   "1.25-alpine3.22",
			Images: []tagImage{{Architecture: "arm64", OS: "linux"}},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tag)
	}))
	defer ts.Close()

	client := ts.Client()
	exists, err := golangAlpineTagExistsFromBase(client, ts.URL+"/", "1.25", "3.22")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Error("expected tag to not be valid (no amd64) but it was reported as existing")
	}
}

// ─── Template rendering tests ───────────────────────────────────────

func TestRenderDockerfileServer(t *testing.T) {
	data := dockerfileData{
		GoVersion:        "1.25",
		AlpineVersion:    "3.22",
		BinaryName:       "server",
		CmdPath:          "./cmd/server",
		Port:             "8080",
		ExtraApkPackages: "",
	}

	content, err := renderDockerfile(data)
	if err != nil {
		t.Fatalf("renderDockerfile returned error: %v", err)
	}

	checks := []struct {
		label    string
		contains string
	}{
		{"FROM build platform", "FROM --platform=linux/amd64 golang:1.25-alpine3.22"},
		{"FROM runtime platform", "FROM --platform=linux/amd64 alpine:3.22"},
		{"GOARCH", "GOARCH=amd64"},
		{"GOOS", "GOOS=linux"},
		{"CGO_ENABLED", "CGO_ENABLED=0"},
		{"COPY go.mod", "COPY go.mod go.sum"},
		{"go mod download", "go mod download"},
		{"binary name", "-o /bin/server"},
		{"cmd path", "./cmd/server"},
		{"ENTRYPOINT", `ENTRYPOINT ["server"]`},
		{"EXPOSE", "EXPOSE 8080"},
		{"USER app", "USER app"},
		{"adduser", "adduser -S app -G app"},
		{"COPY binary", "COPY --from=builder /bin/server /usr/local/bin/server"},
		{"ca-certificates build", "apk add --no-cache git ca-certificates"},
		{"ca-certificates runtime", "apk add --no-cache ca-certificates tzdata"},
		{"trimpath", "-trimpath"},
		{"ldflags", `-ldflags="-s -w"`},
	}

	for _, c := range checks {
		t.Run(c.label, func(t *testing.T) {
			if !strings.Contains(content, c.contains) {
				t.Errorf("Dockerfile missing %q:\n%s", c.contains, content)
			}
		})
	}
}

func TestRenderDockerfileWorkerNoExpose(t *testing.T) {
	data := dockerfileData{
		GoVersion:        "1.25",
		AlpineVersion:    "3.22",
		BinaryName:       "worker",
		CmdPath:          "./cmd/worker",
		Port:             "",
		ExtraApkPackages: "",
	}

	content, err := renderDockerfile(data)
	if err != nil {
		t.Fatalf("renderDockerfile returned error: %v", err)
	}

	if strings.Contains(content, "EXPOSE") {
		t.Error("worker Dockerfile should not contain EXPOSE directive")
	}

	if !strings.Contains(content, `ENTRYPOINT ["worker"]`) {
		t.Error("worker Dockerfile missing correct ENTRYPOINT")
	}

	if !strings.Contains(content, "-o /bin/worker") {
		t.Error("worker Dockerfile missing correct binary output path")
	}

	if !strings.Contains(content, "./cmd/worker") {
		t.Error("worker Dockerfile missing correct cmd path")
	}
}

func TestRenderDockerignore(t *testing.T) {
	content, err := renderDockerignore()
	if err != nil {
		t.Fatalf("renderDockerignore returned error: %v", err)
	}

	expected := []string{
		".git",
		".shipq",
		"node_modules",
		"test_results",
		"*.test",
		"Justfile",
		"shell.nix",
		"README.*",
		"TODO.*",
		"LICENSE.*",
		"DEVOPS_PLAN.*",
		"DOCKERFILE.*",
	}

	for _, e := range expected {
		if !strings.Contains(content, e) {
			t.Errorf(".dockerignore missing entry %q", e)
		}
	}
}

func TestRenderDockerAdocWithWorker(t *testing.T) {
	data := adocData{
		GoVersion:     "1.25",
		AlpineVersion: "3.22",
		ProjectName:   "myapp",
		Dialect:       "postgres",
		HasWorker:     true,
	}

	content, err := renderDockerAdoc(data)
	if err != nil {
		t.Fatalf("renderDockerAdoc returned error: %v", err)
	}

	checks := []struct {
		label    string
		contains string
	}{
		{"title", "Building & Running the Docker Images"},
		{"shipq docker mention", "shipq docker"},
		{"linux/amd64", "linux/amd64"},
		{"server build", "docker build -f Dockerfile.server -t myapp-server ."},
		{"worker build", "docker build -f Dockerfile.worker -t myapp-worker ."},
		{"server run", "myapp-server"},
		{"worker run", "myapp-worker"},
		{"golang base", "golang:1.25-alpine3.22"},
		{"alpine base", "alpine:3.22"},
		{"dialect in DATABASE_URL", `DATABASE_URL="postgres://..."`},
		{"non-root user", "app (non-root)"},
		{"re-generating", "Re-generating"},
		{"platform flag server", "--platform linux/amd64 -f Dockerfile.server"},
		{"platform flag worker", "--platform linux/amd64 -f Dockerfile.worker"},
		{"REDIS_URL", "REDIS_URL"},
	}

	for _, c := range checks {
		t.Run(c.label, func(t *testing.T) {
			if !strings.Contains(content, c.contains) {
				t.Errorf("DOCKERFILE.adoc missing %q", c.contains)
			}
		})
	}
}

func TestRenderDockerAdocWithoutWorker(t *testing.T) {
	data := adocData{
		GoVersion:     "1.25",
		AlpineVersion: "3.22",
		ProjectName:   "myapp",
		Dialect:       "sqlite",
		HasWorker:     false,
	}

	content, err := renderDockerAdoc(data)
	if err != nil {
		t.Fatalf("renderDockerAdoc returned error: %v", err)
	}

	if strings.Contains(content, "Dockerfile.worker") {
		t.Error("DOCKERFILE.adoc should not mention Dockerfile.worker when HasWorker is false")
	}

	if !strings.Contains(content, "Dockerfile.server") {
		t.Error("DOCKERFILE.adoc should mention Dockerfile.server")
	}

	if !strings.Contains(content, `DATABASE_URL="sqlite://..."`) {
		t.Error("DOCKERFILE.adoc should use sqlite dialect in DATABASE_URL")
	}
}

func TestRenderDockerAdocVersionsInlined(t *testing.T) {
	data := adocData{
		GoVersion:     "1.24",
		AlpineVersion: "3.21",
		ProjectName:   "testproj",
		Dialect:       "mysql",
		HasWorker:     false,
	}

	content, err := renderDockerAdoc(data)
	if err != nil {
		t.Fatalf("renderDockerAdoc returned error: %v", err)
	}

	if !strings.Contains(content, "golang:1.24-alpine3.21") {
		t.Error("DOCKERFILE.adoc does not inline correct golang version")
	}
	if !strings.Contains(content, "alpine:3.21") {
		t.Error("DOCKERFILE.adoc does not inline correct alpine version")
	}
	if !strings.Contains(content, `DATABASE_URL="mysql://..."`) {
		t.Error("DOCKERFILE.adoc should use mysql dialect in DATABASE_URL")
	}
}

// ─── Go version extraction tests ────────────────────────────────────

func TestParseGoVersionFromMod(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
		wantErr bool
	}{
		{
			name:    "simple go directive",
			content: "module example.com/app\n\ngo 1.25\n",
			want:    "1.25",
		},
		{
			name:    "go directive with patch version",
			content: "module example.com/app\n\ngo 1.25.4\n",
			want:    "1.25",
		},
		{
			name:    "go directive surrounded by requires",
			content: "module example.com/app\n\ngo 1.24\n\nrequire (\n\tgithub.com/foo/bar v1.0.0\n)\n",
			want:    "1.24",
		},
		{
			name:    "no go directive",
			content: "module example.com/app\n\nrequire (\n\tgithub.com/foo/bar v1.0.0\n)\n",
			wantErr: true,
		},
		{
			name:    "empty file",
			content: "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseGoVersionFromMod(tt.content)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error but got version %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("parseGoVersionFromMod() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractGoVersion(t *testing.T) {
	dir := t.TempDir()
	goModPath := filepath.Join(dir, "go.mod")
	content := "module example.com/test\n\ngo 1.25.4\n\nrequire (\n)\n"
	if err := os.WriteFile(goModPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp go.mod: %v", err)
	}

	got, err := extractGoVersion(dir)
	if err != nil {
		t.Fatalf("extractGoVersion returned error: %v", err)
	}
	if got != "1.25" {
		t.Errorf("extractGoVersion() = %q, want %q", got, "1.25")
	}
}

func TestExtractGoVersionMissing(t *testing.T) {
	dir := t.TempDir()
	_, err := extractGoVersion(dir)
	if err == nil {
		t.Fatal("expected error for missing go.mod, got nil")
	}
}

// ─── atoiSimple tests ───────────────────────────────────────────────

func TestAtoiSimple(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"0", 0},
		{"3", 3},
		{"10", 10},
		{"22", 22},
		{"100", 100},
		{"999", 999},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := atoiSimple(tt.input)
			if got != tt.want {
				t.Errorf("atoiSimple(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

// ─── cmdPathWithSubPath tests ───────────────────────────────────────

func TestCmdPathWithSubPath_EmptySubPath(t *testing.T) {
	got := cmdPathWithSubPath("", "./cmd/server")
	if got != "./cmd/server" {
		t.Errorf("expected ./cmd/server, got %q", got)
	}

	got = cmdPathWithSubPath("", "./cmd/worker")
	if got != "./cmd/worker" {
		t.Errorf("expected ./cmd/worker, got %q", got)
	}
}

func TestCmdPathWithSubPath_SingleLevel(t *testing.T) {
	got := cmdPathWithSubPath("services/api", "./cmd/server")
	if got != "./services/api/cmd/server" {
		t.Errorf("expected ./services/api/cmd/server, got %q", got)
	}

	got = cmdPathWithSubPath("services/api", "./cmd/worker")
	if got != "./services/api/cmd/worker" {
		t.Errorf("expected ./services/api/cmd/worker, got %q", got)
	}
}

func TestCmdPathWithSubPath_MultipleLevel(t *testing.T) {
	got := cmdPathWithSubPath("internal/apps/api", "./cmd/server")
	if got != "./internal/apps/api/cmd/server" {
		t.Errorf("expected ./internal/apps/api/cmd/server, got %q", got)
	}
}

// ─── Monorepo Dockerfile rendering tests ────────────────────────────

func TestRenderDockerfile_MonorepoCmdPath(t *testing.T) {
	data := dockerfileData{
		GoVersion:        "1.25",
		AlpineVersion:    "3.22",
		BinaryName:       "server",
		CmdPath:          cmdPathWithSubPath("services/api", "./cmd/server"),
		Port:             "8080",
		ExtraApkPackages: "",
		SubPath:          "services/api",
	}

	content, err := renderDockerfile(data)
	if err != nil {
		t.Fatalf("renderDockerfile returned error: %v", err)
	}

	// The go build target must include the subpath
	if !strings.Contains(content, "./services/api/cmd/server") {
		t.Errorf("Dockerfile missing monorepo cmd path ./services/api/cmd/server:\n%s", content)
	}

	// COPY go.mod should still be present (unchanged)
	if !strings.Contains(content, "COPY go.mod go.sum ./") {
		t.Errorf("Dockerfile missing COPY go.mod go.sum ./:\n%s", content)
	}
}

func TestRenderDockerfile_SameDir_NoBehaviorChange(t *testing.T) {
	data := dockerfileData{
		GoVersion:        "1.25",
		AlpineVersion:    "3.22",
		BinaryName:       "server",
		CmdPath:          cmdPathWithSubPath("", "./cmd/server"),
		Port:             "8080",
		ExtraApkPackages: "",
		SubPath:          "",
	}

	content, err := renderDockerfile(data)
	if err != nil {
		t.Fatalf("renderDockerfile returned error: %v", err)
	}

	// CmdPath must be the default ./cmd/server (no subpath prefix)
	if !strings.Contains(content, "./cmd/server") {
		t.Errorf("Dockerfile missing ./cmd/server:\n%s", content)
	}
	// Must NOT contain a double-slash or stray subpath
	if strings.Contains(content, ".//cmd/server") {
		t.Errorf("Dockerfile has malformed cmd path .//cmd/server:\n%s", content)
	}
}

func TestMonorepoDockerfileOutputPaths(t *testing.T) {
	// Simulate a monorepo layout:
	//   tmpDir/           <- GoModRoot (go.mod lives here)
	//   tmpDir/services/api/  <- ShipqRoot (shipq.ini lives here)
	tmpDir := t.TempDir()
	goModRoot := tmpDir
	shipqRoot := filepath.Join(tmpDir, "services", "api")

	if err := os.MkdirAll(shipqRoot, 0755); err != nil {
		t.Fatalf("failed to create shipqRoot: %v", err)
	}

	subPath := "services/api"

	// Render and write Dockerfile.server to GoModRoot
	serverData := dockerfileData{
		GoVersion:        "1.25",
		AlpineVersion:    "3.22",
		BinaryName:       "server",
		CmdPath:          cmdPathWithSubPath(subPath, "./cmd/server"),
		Port:             "8080",
		ExtraApkPackages: "",
		SubPath:          subPath,
	}
	serverContent, err := renderDockerfile(serverData)
	if err != nil {
		t.Fatalf("renderDockerfile error: %v", err)
	}

	serverPath := filepath.Join(goModRoot, "Dockerfile.server")
	if err := os.WriteFile(serverPath, []byte(serverContent), 0644); err != nil {
		t.Fatalf("failed to write Dockerfile.server: %v", err)
	}

	// Assert Dockerfile.server is at GoModRoot, not ShipqRoot
	if _, err := os.Stat(serverPath); err != nil {
		t.Fatalf("Dockerfile.server not found at GoModRoot: %v", err)
	}
	if _, err := os.Stat(filepath.Join(shipqRoot, "Dockerfile.server")); err == nil {
		t.Error("Dockerfile.server should NOT be at ShipqRoot in monorepo layout")
	}

	// Assert the content references the subpath in the go build target
	written, err := os.ReadFile(serverPath)
	if err != nil {
		t.Fatalf("failed to read Dockerfile.server: %v", err)
	}
	if !strings.Contains(string(written), "./services/api/cmd/server") {
		t.Errorf("Dockerfile.server missing monorepo cmd path:\n%s", written)
	}
}

func TestRenderDockerAdoc_MonorepoNote(t *testing.T) {
	ad := adocData{
		GoVersion:     "1.25",
		AlpineVersion: "3.22",
		ProjectName:   "myapp",
		Dialect:       "postgres",
		HasWorker:     false,
		SubPath:       "services/api",
	}

	content, err := renderDockerAdoc(ad)
	if err != nil {
		t.Fatalf("renderDockerAdoc error: %v", err)
	}

	if !strings.Contains(content, "monorepo") {
		t.Errorf("DOCKERFILE.adoc should mention monorepo when SubPath is set:\n%s", content)
	}
	if !strings.Contains(content, "go.mod") {
		t.Errorf("DOCKERFILE.adoc should mention go.mod root directory:\n%s", content)
	}
}

func TestRenderDockerAdoc_SameDir_NoMonorepoNote(t *testing.T) {
	ad := adocData{
		GoVersion:     "1.25",
		AlpineVersion: "3.22",
		ProjectName:   "myapp",
		Dialect:       "postgres",
		HasWorker:     false,
		SubPath:       "",
	}

	content, err := renderDockerAdoc(ad)
	if err != nil {
		t.Fatalf("renderDockerAdoc error: %v", err)
	}

	if strings.Contains(content, "monorepo") {
		t.Errorf("DOCKERFILE.adoc should NOT mention monorepo when SubPath is empty:\n%s", content)
	}
}
