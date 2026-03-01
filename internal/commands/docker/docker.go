package docker

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/shipq/shipq/cli"
	"github.com/shipq/shipq/codegen"
	"github.com/shipq/shipq/inifile"
	shipqdag "github.com/shipq/shipq/internal/dag"
	"github.com/shipq/shipq/project"
)

// ─── Docker Hub API types ───────────────────────────────────────────

// tagResult represents a single tag from the Docker Hub v2 tags API.
type tagResult struct {
	Name   string     `json:"name"`
	Images []tagImage `json:"images"`
}

// tagImage represents an image entry within a tag result.
type tagImage struct {
	Architecture string `json:"architecture"`
	OS           string `json:"os"`
}

// tagsPage represents a page of results from the Docker Hub v2 tags API.
type tagsPage struct {
	Next    *string     `json:"next"`
	Results []tagResult `json:"results"`
}

// alpineCandidate is a parsed semver-minor Alpine tag.
type alpineCandidate struct {
	Tag   string
	Major int
	Minor int
	Score int // Major*1000 + Minor
}

// ─── Template data types ────────────────────────────────────────────

// dockerfileData holds template variables for Dockerfile generation.
type dockerfileData struct {
	GoVersion        string
	AlpineVersion    string
	BinaryName       string
	CmdPath          string
	Port             string
	ExtraApkPackages string
}

// dockerignoreData is intentionally empty — the template is static.
type dockerignoreData struct{}

// adocData holds template variables for DOCKERFILE.adoc generation.
type adocData struct {
	GoVersion     string
	AlpineVersion string
	ProjectName   string
	Dialect       string
	HasWorker     bool
}

// ─── Entry point ────────────────────────────────────────────────────

// DockerCmd is the exported entry point for the "shipq docker" command.
func DockerCmd() {
	// ── Step 1: Locate the project ──────────────────────────────────
	roots, err := project.FindProjectRoots()
	if err != nil {
		cli.FatalErr("failed to find project", err)
	}

	// DAG prerequisite check (alongside existing checks)
	if !shipqdag.CheckPrerequisites(shipqdag.CmdDocker, roots.ShipqRoot) {
		os.Exit(1)
	}

	// ── Step 2: Read project context ────────────────────────────────
	shipqIniPath := filepath.Join(roots.ShipqRoot, project.ShipqIniFile)
	ini, err := inifile.ParseFile(shipqIniPath)
	if err != nil {
		cli.FatalErr("failed to parse shipq.ini", err)
	}
	dialect := ini.Get("db", "dialect")

	moduleInfo, err := codegen.GetModuleInfo(roots.GoModRoot, roots.ShipqRoot)
	if err != nil {
		cli.FatalErr("failed to read module path", err)
	}
	_ = moduleInfo // used implicitly via project name

	goVersion, err := extractGoVersion(roots.GoModRoot)
	if err != nil {
		cli.FatalErr("failed to extract Go version from go.mod", err)
	}

	projectName := project.GetProjectName(roots.ShipqRoot)

	hasWorker := dirExists(filepath.Join(roots.ShipqRoot, "cmd", "worker"))

	// ── Step 3: Resolve latest stable Alpine + validate golang tag ──
	client := &http.Client{Timeout: 60 * time.Second}

	cli.Info("Fetching Alpine tags from Docker Hub...")
	candidates, err := fetchAlpineCandidates(client)
	if err != nil {
		cli.FatalErr("failed to fetch Alpine tags", err)
	}

	alpineVersion, err := resolveAlpineVersion(client, candidates, goVersion)
	if err != nil {
		cli.FatalErr("failed to resolve Alpine version", err)
	}
	cli.Infof("Resolved Alpine %s with golang:%s-alpine%s", alpineVersion, goVersion, alpineVersion)

	// ── Step 4: Generate Dockerfile.server ───────────────────────────
	serverData := dockerfileData{
		GoVersion:        goVersion,
		AlpineVersion:    alpineVersion,
		BinaryName:       "server",
		CmdPath:          "./cmd/server",
		Port:             "8080",
		ExtraApkPackages: "",
	}
	serverContent, err := renderDockerfile(serverData)
	if err != nil {
		cli.FatalErr("failed to render Dockerfile.server", err)
	}
	serverPath := filepath.Join(roots.ShipqRoot, "Dockerfile.server")
	if err := os.WriteFile(serverPath, []byte(serverContent), 0644); err != nil {
		cli.FatalErr("failed to write Dockerfile.server", err)
	}
	cli.Success("Wrote " + serverPath)

	// ── Step 4b: Generate Dockerfile.worker (conditional) ────────────
	if hasWorker {
		workerData := dockerfileData{
			GoVersion:        goVersion,
			AlpineVersion:    alpineVersion,
			BinaryName:       "worker",
			CmdPath:          "./cmd/worker",
			Port:             "",
			ExtraApkPackages: "",
		}
		workerContent, err := renderDockerfile(workerData)
		if err != nil {
			cli.FatalErr("failed to render Dockerfile.worker", err)
		}
		workerPath := filepath.Join(roots.ShipqRoot, "Dockerfile.worker")
		if err := os.WriteFile(workerPath, []byte(workerContent), 0644); err != nil {
			cli.FatalErr("failed to write Dockerfile.worker", err)
		}
		cli.Success("Wrote " + workerPath)
	}

	// ── Step 5: Generate .dockerignore ───────────────────────────────
	diContent, err := renderDockerignore()
	if err != nil {
		cli.FatalErr("failed to render .dockerignore", err)
	}
	diPath := filepath.Join(roots.ShipqRoot, ".dockerignore")
	if err := os.WriteFile(diPath, []byte(diContent), 0644); err != nil {
		cli.FatalErr("failed to write .dockerignore", err)
	}
	cli.Success("Wrote " + diPath)

	// ── Step 6: Generate DOCKERFILE.adoc ─────────────────────────────
	ad := adocData{
		GoVersion:     goVersion,
		AlpineVersion: alpineVersion,
		ProjectName:   projectName,
		Dialect:       dialect,
		HasWorker:     hasWorker,
	}
	adocContent, err := renderDockerAdoc(ad)
	if err != nil {
		cli.FatalErr("failed to render DOCKERFILE.adoc", err)
	}
	adocPath := filepath.Join(roots.ShipqRoot, "DOCKERFILE.adoc")
	if err := os.WriteFile(adocPath, []byte(adocContent), 0644); err != nil {
		cli.FatalErr("failed to write DOCKERFILE.adoc", err)
	}
	cli.Success("Wrote " + adocPath)
}

// ─── Alpine version resolution ──────────────────────────────────────

// fetchAlpineCandidates paginates the Docker Hub v2 tags API for the
// library/alpine repository and returns semver-minor candidates sorted
// descending by version.
func fetchAlpineCandidates(client *http.Client) ([]alpineCandidate, error) {
	return fetchAlpineCandidatesFromURL(
		client,
		"https://hub.docker.com/v2/repositories/library/alpine/tags/?page_size=100",
	)
}

// fetchAlpineCandidatesFromURL is the internal implementation that follows
// pagination links. It is separated for testability.
func fetchAlpineCandidatesFromURL(client *http.Client, startURL string) ([]alpineCandidate, error) {
	re := regexp.MustCompile(`^(\d+)\.(\d+)$`)

	var all []tagResult
	url := startURL
	for url != "" {
		resp, err := client.Get(url)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch Alpine tags: %w", err)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("Docker Hub API error: %s\n%s", resp.Status, string(body))
		}

		var page tagsPage
		if err := json.Unmarshal(body, &page); err != nil {
			return nil, fmt.Errorf("failed to parse Alpine tags JSON: %w", err)
		}

		all = append(all, page.Results...)

		if page.Next != nil && *page.Next != "" {
			url = *page.Next
		} else {
			url = ""
		}
	}

	var candidates []alpineCandidate
	for _, tag := range all {
		m := re.FindStringSubmatch(tag.Name)
		if m == nil {
			continue
		}
		major := atoiSimple(m[1])
		minor := atoiSimple(m[2])

		// Validate that there is an amd64 image entry.
		if !hasAmd64Image(tag) {
			continue
		}

		candidates = append(candidates, alpineCandidate{
			Tag:   tag.Name,
			Major: major,
			Minor: minor,
			Score: major*1000 + minor,
		})
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no valid Alpine X.Y tags found")
	}

	return candidates, nil
}

// hasAmd64Image returns true if the tag contains at least one linux/amd64
// image entry.
func hasAmd64Image(tag tagResult) bool {
	for _, img := range tag.Images {
		if img.Architecture == "amd64" {
			return true
		}
	}
	return false
}

// pickLatestAlpine returns the highest-version Alpine candidate from a
// pre-sorted (descending) list. This is a convenience for tests.
func pickLatestAlpine(candidates []alpineCandidate) (string, error) {
	if len(candidates) == 0 {
		return "", fmt.Errorf("no Alpine candidates")
	}
	return candidates[0].Tag, nil
}

// resolveAlpineVersion iterates through Alpine candidates (highest first)
// and returns the first one for which a matching golang:<goVersion>-alpine<X.Y>
// tag exists on Docker Hub.
func resolveAlpineVersion(client *http.Client, candidates []alpineCandidate, goVersion string) (string, error) {
	for _, c := range candidates {
		exists, err := golangAlpineTagExists(client, goVersion, c.Tag)
		if err != nil {
			// Network error — skip this candidate but keep trying.
			continue
		}
		if exists {
			return c.Tag, nil
		}
	}
	return "", fmt.Errorf(
		"no valid golang:%s-alpine<X.Y> tag found after trying %d Alpine candidates",
		goVersion, len(candidates),
	)
}

// golangAlpineTagExists checks whether the Docker Hub tag
// golang:<goVersion>-alpine<alpineVersion> exists and has an amd64 image.
func golangAlpineTagExists(client *http.Client, goVersion, alpineVersion string) (bool, error) {
	return golangAlpineTagExistsFromBase(
		client,
		"https://hub.docker.com/v2/repositories/library/golang/tags/",
		goVersion,
		alpineVersion,
	)
}

// golangAlpineTagExistsFromBase is the internal implementation that accepts
// a base URL for testability.
func golangAlpineTagExistsFromBase(client *http.Client, baseURL, goVersion, alpineVersion string) (bool, error) {
	tagName := goVersion + "-alpine" + alpineVersion
	url := baseURL + tagName
	resp, err := client.Get(url)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return false, nil
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return false, fmt.Errorf("Docker Hub API error for golang tag %q: %s\n%s", tagName, resp.Status, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	var tag tagResult
	if err := json.Unmarshal(body, &tag); err != nil {
		return false, err
	}

	return hasAmd64Image(tag), nil
}

// ─── go.mod helpers ─────────────────────────────────────────────────

// extractGoVersion reads go.mod and returns the Go version from the
// `go X.Y` or `go X.Y.Z` directive as the `X.Y` portion only.
func extractGoVersion(goModRoot string) (string, error) {
	goModPath := filepath.Join(goModRoot, "go.mod")
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return "", fmt.Errorf("failed to read go.mod: %w", err)
	}
	return parseGoVersionFromMod(string(data))
}

// parseGoVersionFromMod parses the `go X.Y` directive from go.mod content
// and returns the `X.Y` portion (stripping any patch version).
func parseGoVersionFromMod(content string) (string, error) {
	re := regexp.MustCompile(`(?m)^go\s+(\d+\.\d+)`)
	m := re.FindStringSubmatch(content)
	if m == nil {
		return "", fmt.Errorf("go directive not found in go.mod")
	}
	return m[1], nil
}

// ─── Misc helpers ───────────────────────────────────────────────────

// dirExists returns true if the path is an existing directory.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// atoiSimple converts a decimal string to an int. It is only called on
// strings that have already been validated by a regex, so it does not
// return an error.
func atoiSimple(s string) int {
	n := 0
	for _, c := range s {
		n = n*10 + int(c-'0')
	}
	return n
}

// ─── Dockerfile template ────────────────────────────────────────────

const dockerfileTmpl = `# ── Build stage ──
FROM --platform=linux/amd64 golang:{{ .GoVersion }}-alpine{{ .AlpineVersion }} AS builder
RUN apk add --no-cache git ca-certificates
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" \
    -o /bin/{{ .BinaryName }} {{ .CmdPath }}

# ── Runtime stage ──
FROM --platform=linux/amd64 alpine:{{ .AlpineVersion }}
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S app && adduser -S app -G app
COPY --from=builder /bin/{{ .BinaryName }} /usr/local/bin/{{ .BinaryName }}
USER app
{{- if .Port }}
EXPOSE {{ .Port }}
{{- end }}
ENTRYPOINT ["{{ .BinaryName }}"]
`

// renderDockerfile renders the Dockerfile template with the given data.
func renderDockerfile(data dockerfileData) (string, error) {
	tmpl, err := template.New("Dockerfile").Parse(dockerfileTmpl)
	if err != nil {
		return "", fmt.Errorf("failed to parse Dockerfile template: %w", err)
	}
	var sb strings.Builder
	if err := tmpl.Execute(&sb, data); err != nil {
		return "", fmt.Errorf("failed to execute Dockerfile template: %w", err)
	}
	return sb.String(), nil
}

// ─── .dockerignore template ─────────────────────────────────────────

const dockerignoreContent = `.git
.shipq
node_modules
test_results
*.test
Justfile
shell.nix
README.*
TODO.*
LICENSE.*
DEVOPS_PLAN.*
DOCKERFILE.*
`

// renderDockerignore returns the static .dockerignore content.
func renderDockerignore() (string, error) {
	return dockerignoreContent, nil
}

// ─── DOCKERFILE.adoc template ───────────────────────────────────────

const dockerAdocTmpl = `= Building & Running the Docker Images

These Dockerfiles were generated by ` + "`" + `shipq docker` + "`" + `.
All images target *linux/amd64*.

== Prerequisites

* Docker ≥ 20.10 (BuildKit enabled by default)

== Build

    # Build the server image
    docker build -f Dockerfile.server -t {{ .ProjectName }}-server .
{{ if .HasWorker }}
    # Build the worker image
    docker build -f Dockerfile.worker -t {{ .ProjectName }}-worker .
{{ end }}
To be explicit about the target platform (e.g. when building on an ARM Mac):

    docker build --platform linux/amd64 -f Dockerfile.server -t {{ .ProjectName }}-server .
{{- if .HasWorker }}
    docker build --platform linux/amd64 -f Dockerfile.worker -t {{ .ProjectName }}-worker .
{{- end }}

== Run

    # Server
    docker run -p 8080:8080 \
      -e DATABASE_URL="{{ .Dialect }}://..." \
      -e COOKIE_SECRET="change-me" \
      {{ .ProjectName }}-server
{{ if .HasWorker }}
    # Worker
    docker run \
      -e DATABASE_URL="{{ .Dialect }}://..." \
      -e REDIS_URL="redis://..." \
      {{ .ProjectName }}-worker
{{ end }}
== Image Details

[cols="1,2"]
|===
| Base (build)   | golang:{{ .GoVersion }}-alpine{{ .AlpineVersion }}
| Base (runtime) | alpine:{{ .AlpineVersion }}
| Architecture   | linux/amd64
| User           | app (non-root)
|===

== Re-generating

Run ` + "`" + `shipq docker` + "`" + ` again at any time. It will re-query Docker Hub for the
latest stable Alpine and overwrite all Dockerfile artifacts.
`

// renderDockerAdoc renders the DOCKERFILE.adoc template.
func renderDockerAdoc(data adocData) (string, error) {
	tmpl, err := template.New("DOCKERFILE.adoc").Parse(dockerAdocTmpl)
	if err != nil {
		return "", fmt.Errorf("failed to parse DOCKERFILE.adoc template: %w", err)
	}
	var sb strings.Builder
	if err := tmpl.Execute(&sb, data); err != nil {
		return "", fmt.Errorf("failed to execute DOCKERFILE.adoc template: %w", err)
	}
	return sb.String(), nil
}
