package nix

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"text/template"
	"time"

	"github.com/shipq/shipq/cli"
	"github.com/shipq/shipq/inifile"
	"github.com/shipq/shipq/project"
)

const (
	owner = "NixOS"
	repo  = "nixpkgs"
)

// branch mirrors the relevant fields from the GitHub branches API.
type branch struct {
	Name   string `json:"name"`
	Commit struct {
		SHA string `json:"sha"`
	} `json:"commit"`
}

// shellNixData holds the template variables for shell.nix generation.
type shellNixData struct {
	Rev            string
	SHA256         string
	DBPackages     string
	WorkerPackages string
}

// NixCmd is the exported entry point for the "shipq nix" command.
func NixCmd() {
	// ── Step 1: Locate the project ──────────────────────────────────
	roots, err := project.FindProjectRoots()
	if err != nil {
		cli.FatalErr("failed to find project", err)
	}

	// ── Step 2: Read project context from shipq.ini ─────────────────
	shipqIniPath := filepath.Join(roots.ShipqRoot, project.ShipqIniFile)
	ini, err := inifile.ParseFile(shipqIniPath)
	if err != nil {
		cli.FatalErr("failed to parse shipq.ini", err)
	}

	dialect := ini.Get("db", "dialect")
	dbPackages := dbPackagesForDialect(dialect)

	workerPackages := ""
	workerDir := filepath.Join(roots.ShipqRoot, "cmd", "worker")
	if dirExists(workerDir) {
		workerPackages = "valkey\n      centrifugo\n      minio\n      minio-client"
	}

	// ── Step 3: Resolve latest stable nixpkgs ───────────────────────
	client := &http.Client{Timeout: 60 * time.Second}

	cli.Info("Fetching nixpkgs branches from GitHub...")
	branches, err := listAllBranches(client)
	if err != nil {
		cli.FatalErr("failed to list nixpkgs branches", err)
	}

	stableName, rev, err := pickLatestStable(branches)
	if err != nil {
		cli.FatalErr("failed to pick latest stable branch", err)
	}
	cli.Infof("Latest stable branch: %s (rev %s)", stableName, rev[:12])

	tarURL := fmt.Sprintf("https://github.com/%s/%s/archive/%s.tar.gz", owner, repo, rev)

	cli.Info("Downloading tarball and computing SHA-256 (this may take a moment)...")
	sri, err := sha256SRI(client, tarURL)
	if err != nil {
		cli.FatalErr("failed to compute SHA-256 of tarball", err)
	}
	cli.Infof("SRI hash: %s", sri)

	// ── Step 4: Render and write shell.nix ──────────────────────────
	data := shellNixData{
		Rev:            rev,
		SHA256:         sri,
		DBPackages:     dbPackages,
		WorkerPackages: workerPackages,
	}

	rendered, err := renderShellNix(data)
	if err != nil {
		cli.FatalErr("failed to render shell.nix template", err)
	}

	outPath := filepath.Join(roots.ShipqRoot, "shell.nix")
	if err := os.WriteFile(outPath, []byte(rendered), 0644); err != nil {
		cli.FatalErr("failed to write shell.nix", err)
	}

	cli.Success("Wrote " + outPath)
}

// ─── Networking helpers ─────────────────────────────────────────────

// listAllBranches paginates the GitHub branches API for NixOS/nixpkgs.
func listAllBranches(client *http.Client) ([]branch, error) {
	var all []branch
	for page := 1; page <= 10; page++ {
		url := fmt.Sprintf(
			"https://api.github.com/repos/%s/%s/branches?per_page=100&page=%d",
			owner, repo, page,
		)

		resp, err := client.Get(url)
		if err != nil {
			return nil, err
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("GitHub API error: %s\n%s", resp.Status, string(body))
		}

		var pageBranches []branch
		if err := json.Unmarshal(body, &pageBranches); err != nil {
			return nil, err
		}

		if len(pageBranches) == 0 {
			break
		}

		all = append(all, pageBranches...)

		if len(pageBranches) < 100 {
			break
		}
	}

	if len(all) == 0 {
		return nil, errors.New("no branches found")
	}

	return all, nil
}

// candidate is a scored nixos-YY.MM branch.
type candidate struct {
	Name  string
	SHA   string
	Score int
}

// pickLatestStable selects the nixos-YY.MM branch with the highest
// year*100+month score. It returns (branchName, commitSHA, error).
func pickLatestStable(branches []branch) (string, string, error) {
	re := regexp.MustCompile(`^nixos-(\d{2})\.(\d{2})$`)

	var cands []candidate

	for _, b := range branches {
		m := re.FindStringSubmatch(b.Name)
		if m == nil {
			continue
		}

		yy := atoi2(m[1])
		mm := atoi2(m[2])
		score := yy*100 + mm

		cands = append(cands, candidate{
			Name:  b.Name,
			SHA:   b.Commit.SHA,
			Score: score,
		})
	}

	if len(cands) == 0 {
		return "", "", errors.New("no stable nixos-YY.MM branches found")
	}

	sort.Slice(cands, func(i, j int) bool {
		return cands[i].Score > cands[j].Score
	})

	return cands[0].Name, cands[0].SHA, nil
}

// atoi2 converts a two-digit decimal string to an int.
func atoi2(s string) int {
	return int(s[0]-'0')*10 + int(s[1]-'0')
}

// sha256SRI downloads the URL and returns its SHA-256 as an SRI string
// (e.g. "sha256-<base64>").
func sha256SRI(client *http.Client, url string) (string, error) {
	sum, err := sha256OfURL(client, url)
	if err != nil {
		return "", err
	}
	return "sha256-" + base64.StdEncoding.EncodeToString(sum[:]), nil
}

// sha256OfURL downloads the content at url and returns its SHA-256 digest.
func sha256OfURL(client *http.Client, url string) ([32]byte, error) {
	resp, err := client.Get(url)
	if err != nil {
		return [32]byte{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return [32]byte{}, fmt.Errorf("download failed: %s", resp.Status)
	}

	h := sha256.New()
	if _, err := io.Copy(h, resp.Body); err != nil {
		return [32]byte{}, err
	}

	var sum [32]byte
	copy(sum[:], h.Sum(nil))
	return sum, nil
}

// ─── Project-context helpers ────────────────────────────────────────

// dbPackagesForDialect returns the Nix package name(s) for the given dialect.
func dbPackagesForDialect(dialect string) string {
	switch dialect {
	case "postgres":
		return "postgresql_18"
	case "mysql":
		return "mysql80"
	case "sqlite":
		return "sqlite"
	default:
		// If dialect is empty or unknown, default to sqlite
		return "sqlite"
	}
}

// dirExists returns true if the path is an existing directory.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// ─── Template ───────────────────────────────────────────────────────

const shellNixTemplate = `let
  pkgs = import (fetchTarball {
    url = "https://github.com/NixOS/nixpkgs/archive/{{ .Rev }}.tar.gz";
    sha256 = "{{ .SHA256 }}";
  }) {};
in
pkgs.mkShell {
  packages = with pkgs; [
    # ── Always included ──
    go
    git
    git-lfs
    glab
    gh
    opentofu
    goreman
    tmux
    lf
    tokei
    nix
    nixfmt-rfc-style
    ripgrep
    just
    neovim
    nodejs_24
    awscli2

    # ── Database ──
    {{ .DBPackages }}
{{- if .WorkerPackages }}

    # ── Workers / Centrifugo ──
    {{ .WorkerPackages }}
{{- end }}
  ];

  shellHook = ''
    export PROJECT_ROOT="$(pwd)"
    export PORT=8080
    export GO_ENV=development
{{- if eq .DBPackages "postgresql_18" }}
    export DATABASE_URL="postgres://localhost:5432/dev?sslmode=disable"
{{- else if eq .DBPackages "mysql80" }}
    export DATABASE_URL="mysql://root@localhost:3306/dev"
{{- else }}
    export DATABASE_URL="sqlite://dev.db"
{{- end }}
    export COOKIE_SECRET=supersecret
    unset DEVELOPER_DIR
  '';
}
`

// renderShellNix renders the shell.nix template with the given data.
func renderShellNix(data shellNixData) (string, error) {
	tmpl, err := template.New("shell.nix").Parse(shellNixTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf []byte
	w := &byteWriter{buf: &buf}
	if err := tmpl.Execute(w, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return string(buf), nil
}

// byteWriter is a minimal io.Writer that appends to a byte slice.
type byteWriter struct {
	buf *[]byte
}

func (w *byteWriter) Write(p []byte) (int, error) {
	*w.buf = append(*w.buf, p...)
	return len(p), nil
}
