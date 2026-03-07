package nix

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/shipq/shipq/cli"
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
	Rev    string
	SHA256 string
}

// NixCmd is the exported entry point for the "shipq nix" command.
func NixCmd() {
	// ── Step 1: Locate the project ──────────────────────────────────
	roots, err := project.FindProjectRoots()
	if err != nil {
		cli.FatalErr("failed to find project", err)
	}

	// ── Step 2: Resolve latest stable nixpkgs ───────────────────────
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

	// ── Step 3: Compute the unpacked tarball hash via nix-prefetch-url ──
	tarURL := fmt.Sprintf("https://github.com/%s/%s/archive/%s.tar.gz", owner, repo, rev)

	cli.Info("Prefetching tarball and computing hash (this may take a moment)...")
	sri, err := nixPrefetchSRI(tarURL)
	if err != nil {
		cli.FatalErr("failed to compute hash of tarball", err)
	}
	cli.Infof("SRI hash: %s", sri)

	// ── Step 4: Render and write shell.nix ──────────────────────────
	data := shellNixData{
		Rev:    rev,
		SHA256: sri,
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

// ─── Hash helpers ───────────────────────────────────────────────────

// nixPrefetchSRI uses nix-prefetch-url --unpack to download and hash the
// unpacked tarball contents (matching what fetchTarball expects), then
// converts the resulting Nix base32 hash to SRI format via nix-hash --to-sri.
func nixPrefetchSRI(tarURL string) (string, error) {
	nix32, err := nixPrefetchURL(tarURL)
	if err != nil {
		return "", err
	}

	sri, err := nixHashToSRI(nix32)
	if err != nil {
		return "", err
	}

	return sri, nil
}

// nixPrefetchURL shells out to nix-prefetch-url --unpack --type sha256 and
// returns the Nix base32 hash of the unpacked archive contents.
var nixPrefetchURL = func(tarURL string) (string, error) {
	cmd := exec.Command("nix-prefetch-url", "--unpack", "--type", "sha256", tarURL)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("nix-prefetch-url failed: %s\n%s", err, string(exitErr.Stderr))
		}
		return "", fmt.Errorf("nix-prefetch-url failed: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// nixHashToSRI shells out to nix-hash --to-sri --type sha256 to convert
// a Nix base32 hash into SRI format (e.g. "sha256-<base64>").
var nixHashToSRI = func(nix32 string) (string, error) {
	cmd := exec.Command("nix-hash", "--to-sri", "--type", "sha256", nix32)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("nix-hash failed: %s\n%s", err, string(exitErr.Stderr))
		}
		return "", fmt.Errorf("nix-hash failed: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// ─── Template ───────────────────────────────────────────────────────

const shellNixTemplate = `let
  pkgs = import (fetchTarball {
    url = "https://github.com/NixOS/nixpkgs/archive/{{ .Rev }}.tar.gz";
    sha256 = "{{ .SHA256 }}";
  }) {};
in
pkgs.mkShell {
  packages = [
  ];

  shellHook = ''
    export PROJECT_ROOT="$(pwd)"
    export PORT=8080
    export GO_ENV=development
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
