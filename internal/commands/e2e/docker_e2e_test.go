//go:build e2e

package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// requireDocker skips the test if docker is not on $PATH or the daemon is not
// reachable.
func requireDocker(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("skipping: docker not found on $PATH")
	}
	cmd := exec.Command("docker", "info")
	cmd.Env = cleanEnv()
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("skipping: docker daemon not reachable: %v\n%s", err, out)
	}
}

// setupProjectForDocker initialises a project directory through the full
// shipq pipeline (init → db setup → auth → go mod tidy) so that
// cmd/server/main.go and all embedded library packages exist. It returns the
// project directory (which is also the go.mod root in the single-repo case).
//
// Only SQLite is used — the Docker build test does not need a running database
// server; it only needs a compilable Go project.
func setupProjectForDocker(t *testing.T, shipq, baseDirName string) string {
	t.Helper()

	cleanDir := "/tmp/" + baseDirName
	projectName := filepath.Base(cleanDir)

	os.RemoveAll(cleanDir)
	if err := os.MkdirAll(cleanDir, 0755); err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}

	t.Log("Initializing project...")
	run(t, cleanDir, shipq, "init", "--sqlite")

	// Construct SQLite DATABASE_URL
	dbPath := filepath.Join(cleanDir, ".shipq", "data", projectName+".db")
	dbURL := "sqlite://" + dbPath
	dbEnv := []string{"DATABASE_URL=" + dbURL}

	t.Log("Setting up database (sqlite)...")
	runWithEnv(t, cleanDir, dbEnv, shipq, "db", "setup")

	// Auth generates migrations, runs migrate up (which embeds lib packages),
	// compiles queries, and compiles the handler registry — producing
	// cmd/server/main.go and the full api/ package.
	t.Log("Generating auth (triggers embed + handler compile)...")
	runWithEnv(t, cleanDir, dbEnv, shipq, "auth")
	run(t, cleanDir, "go", "mod", "tidy")

	// Verify cmd/server/main.go was generated
	mainPath := filepath.Join(cleanDir, "cmd", "server", "main.go")
	if _, err := os.Stat(mainPath); err != nil {
		t.Fatalf("cmd/server/main.go not generated: %v", err)
	}

	return cleanDir
}

// setupMonorepoForDocker creates a monorepo layout:
//
//	<root>/                 ← GoModRoot (go.mod lives here)
//	├── go.mod
//	├── go.sum
//	└── services/api/       ← ShipqRoot (shipq.ini lives here)
//	    ├── shipq.ini
//	    ├── api/health/
//	    ├── cmd/server/main.go
//	    └── ...
//
// It returns (goModRoot, shipqRoot).
func setupMonorepoForDocker(t *testing.T, shipq, baseDirName string) (string, string) {
	t.Helper()

	goModRoot := "/tmp/" + baseDirName
	shipqRoot := filepath.Join(goModRoot, "services", "api")
	projectName := filepath.Base(shipqRoot) // "api"

	os.RemoveAll(goModRoot)
	if err := os.MkdirAll(shipqRoot, 0755); err != nil {
		t.Fatalf("failed to create shipqRoot: %v", err)
	}

	// Create go.mod in the monorepo root FIRST so that `shipq init` (run from
	// services/api/) discovers it via FindGoModRootFrom and skips creating a
	// new go.mod.
	goVersion := goVersionForMod(t)
	goModContent := "module com." + filepath.Base(goModRoot) + "\n\ngo " + goVersion + "\n"
	if err := os.WriteFile(filepath.Join(goModRoot, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	t.Log("Initializing monorepo project (from services/api/)...")
	run(t, shipqRoot, shipq, "init", "--sqlite")

	// Verify shipq.ini was created in shipqRoot, NOT in goModRoot
	if _, err := os.Stat(filepath.Join(shipqRoot, "shipq.ini")); err != nil {
		t.Fatalf("shipq.ini not created in shipqRoot: %v", err)
	}

	// Construct SQLite DATABASE_URL
	dbPath := filepath.Join(shipqRoot, ".shipq", "data", projectName+".db")
	dbURL := "sqlite://" + dbPath
	dbEnv := []string{"DATABASE_URL=" + dbURL}

	t.Log("Setting up database (sqlite, monorepo)...")
	runWithEnv(t, shipqRoot, dbEnv, shipq, "db", "setup")

	t.Log("Generating auth (triggers embed + handler compile, monorepo)...")
	runWithEnv(t, shipqRoot, dbEnv, shipq, "auth")

	// go mod tidy must run from goModRoot (where go.mod lives)
	run(t, goModRoot, "go", "mod", "tidy")

	// Verify cmd/server/main.go was generated inside shipqRoot
	mainPath := filepath.Join(shipqRoot, "cmd", "server", "main.go")
	if _, err := os.Stat(mainPath); err != nil {
		t.Fatalf("cmd/server/main.go not generated in monorepo: %v", err)
	}

	return goModRoot, shipqRoot
}

// goVersionForMod returns the current Go version in "X.Y" format, suitable
// for a go.mod directive.
func goVersionForMod(t *testing.T) string {
	t.Helper()
	cmd := exec.Command("go", "env", "GOVERSION")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go env GOVERSION failed: %v\n%s", err, out)
	}
	// GOVERSION returns e.g. "go1.23.4"
	v := strings.TrimSpace(string(out))
	v = strings.TrimPrefix(v, "go")
	// Keep only X.Y
	parts := strings.SplitN(v, ".", 3)
	if len(parts) < 2 {
		t.Fatalf("unexpected GOVERSION format: %q", v)
	}
	return parts[0] + "." + parts[1]
}

// dockerBuild runs `docker build` with the given Dockerfile in the given
// context directory and fails the test if the build fails. The image is
// tagged with a unique name and cleaned up after the test.
func dockerBuild(t *testing.T, contextDir, dockerfilePath string) {
	t.Helper()

	// Use a unique tag so parallel runs don't collide
	tag := "shipq-e2e-docker-" + filepath.Base(contextDir) + ":test"

	t.Logf("Running: docker build -f %s -t %s %s", dockerfilePath, tag, contextDir)
	cmd := exec.Command("docker", "build",
		"-f", dockerfilePath,
		"-t", tag,
		contextDir,
	)
	cmd.Env = cleanEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("docker build failed:\n%s\n\nerror: %v", out, err)
	}
	t.Logf("docker build succeeded for %s", dockerfilePath)

	// Clean up the image after the test
	t.Cleanup(func() {
		rmi := exec.Command("docker", "rmi", "-f", tag)
		rmi.Env = cleanEnv()
		_ = rmi.Run() // best-effort cleanup
	})
}

// ─── Single-repo Docker build test ──────────────────────────────────

func TestEndToEnd_DockerBuild_SingleRepo(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping end-to-end test in short mode")
	}
	requireDocker(t)

	repoRoot := shipqRepoRoot(t)
	shipq := buildShipq(t, repoRoot)

	// Set up a fully compiled project (init → db setup → auth → tidy)
	projectDir := setupProjectForDocker(t, shipq, "shipq-e2e-docker-single")

	// Generate Dockerfiles via `shipq docker`
	t.Log("Running shipq docker...")
	run(t, projectDir, shipq, "docker")

	// Verify Dockerfile.server was written to the project root (single-repo:
	// GoModRoot == ShipqRoot == projectDir)
	serverDockerfile := filepath.Join(projectDir, "Dockerfile.server")
	if _, err := os.Stat(serverDockerfile); err != nil {
		t.Fatalf("Dockerfile.server not found at %s: %v", serverDockerfile, err)
	}

	// Read and sanity-check the Dockerfile content
	dfContent, err := os.ReadFile(serverDockerfile)
	if err != nil {
		t.Fatalf("failed to read Dockerfile.server: %v", err)
	}
	dfStr := string(dfContent)
	if !strings.Contains(dfStr, "COPY go.mod go.sum ./") {
		t.Error("Dockerfile.server missing 'COPY go.mod go.sum ./'")
	}
	// In single-repo mode CmdPath should be ./cmd/server (no subpath prefix)
	if !strings.Contains(dfStr, "./cmd/server") {
		t.Error("Dockerfile.server missing './cmd/server' build target")
	}

	// Verify .dockerignore was also written
	diPath := filepath.Join(projectDir, ".dockerignore")
	if _, err := os.Stat(diPath); err != nil {
		t.Fatalf(".dockerignore not found: %v", err)
	}

	// The main event: docker build must succeed
	t.Log("Building Docker image (single-repo)...")
	dockerBuild(t, projectDir, serverDockerfile)
	t.Log("Single-repo Docker build passed!")
}

// ─── Monorepo Docker build test ─────────────────────────────────────

func TestEndToEnd_DockerBuild_Monorepo(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping end-to-end test in short mode")
	}
	requireDocker(t)

	repoRoot := shipqRepoRoot(t)
	shipq := buildShipq(t, repoRoot)

	// Set up a monorepo layout with go.mod in root and shipq.ini in
	// services/api/
	goModRoot, shipqRoot := setupMonorepoForDocker(t, shipq, "shipq-e2e-docker-mono")

	// Generate Dockerfiles via `shipq docker` (run from shipqRoot)
	t.Log("Running shipq docker (monorepo)...")
	run(t, shipqRoot, shipq, "docker")

	// Dockerfile.server must be written to GoModRoot, NOT ShipqRoot
	serverDockerfile := filepath.Join(goModRoot, "Dockerfile.server")
	if _, err := os.Stat(serverDockerfile); err != nil {
		t.Fatalf("Dockerfile.server not found at GoModRoot (%s): %v", goModRoot, err)
	}
	if _, err := os.Stat(filepath.Join(shipqRoot, "Dockerfile.server")); err == nil {
		t.Error("Dockerfile.server should NOT be in ShipqRoot in monorepo layout")
	}

	// .dockerignore must also be at GoModRoot
	diPath := filepath.Join(goModRoot, ".dockerignore")
	if _, err := os.Stat(diPath); err != nil {
		t.Fatalf(".dockerignore not found at GoModRoot: %v", err)
	}

	// Read and verify the Dockerfile uses the subpath-prefixed CmdPath
	dfContent, err := os.ReadFile(serverDockerfile)
	if err != nil {
		t.Fatalf("failed to read Dockerfile.server: %v", err)
	}
	dfStr := string(dfContent)

	// In monorepo mode, CmdPath should be ./services/api/cmd/server
	if !strings.Contains(dfStr, "./services/api/cmd/server") {
		t.Errorf("Dockerfile.server should reference ./services/api/cmd/server, got:\n%s", dfStr)
	}

	// COPY go.mod should still be present
	if !strings.Contains(dfStr, "COPY go.mod go.sum ./") {
		t.Error("Dockerfile.server missing 'COPY go.mod go.sum ./'")
	}

	// Verify DOCKERFILE.adoc mentions monorepo
	adocPath := filepath.Join(shipqRoot, "DOCKERFILE.adoc")
	if _, err := os.Stat(adocPath); err != nil {
		t.Fatalf("DOCKERFILE.adoc not found in shipqRoot: %v", err)
	}
	adocContent, err := os.ReadFile(adocPath)
	if err != nil {
		t.Fatalf("failed to read DOCKERFILE.adoc: %v", err)
	}
	if !strings.Contains(string(adocContent), "monorepo") {
		t.Error("DOCKERFILE.adoc should contain monorepo note when SubPath is set")
	}

	// The main event: docker build from GoModRoot must succeed.
	// The build context is GoModRoot (where go.mod, go.sum, and Dockerfile
	// all live), and the go build target points into the subpath.
	t.Log("Building Docker image (monorepo)...")
	dockerBuild(t, goModRoot, serverDockerfile)
	t.Log("Monorepo Docker build passed!")
}
