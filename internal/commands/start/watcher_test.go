package start

import (
	"testing"
)

// ── shouldSkipDir ────────────────────────────────────────────────────────────

func TestShouldSkipDir(t *testing.T) {
	tests := []struct {
		name     string
		dirName  string
		wantSkip bool
	}{
		// Explicitly listed directories.
		{name: "skip .shipq", dirName: ".shipq", wantSkip: true},
		{name: "skip .git", dirName: ".git", wantSkip: true},
		{name: "skip node_modules", dirName: "node_modules", wantSkip: true},
		{name: "skip vendor", dirName: "vendor", wantSkip: true},
		{name: "skip test_results", dirName: "test_results", wantSkip: true},

		// Hidden directories (start with '.').
		{name: "skip .vscode", dirName: ".vscode", wantSkip: true},
		{name: "skip .idea", dirName: ".idea", wantSkip: true},
		{name: "skip .cache", dirName: ".cache", wantSkip: true},

		// Normal directories that should be watched.
		{name: "include cmd", dirName: "cmd", wantSkip: false},
		{name: "include internal", dirName: "internal", wantSkip: false},
		{name: "include api", dirName: "api", wantSkip: false},
		{name: "include pkg", dirName: "pkg", wantSkip: false},
		{name: "include handlers", dirName: "handlers", wantSkip: false},
		{name: "include db", dirName: "db", wantSkip: false},

		// Edge cases.
		{name: "empty string", dirName: "", wantSkip: false},
		{name: "single dot", dirName: ".", wantSkip: true},
		{name: "double dot", dirName: "..", wantSkip: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldSkipDir(tt.dirName)
			if got != tt.wantSkip {
				t.Errorf("shouldSkipDir(%q) = %v, want %v", tt.dirName, got, tt.wantSkip)
			}
		})
	}
}

// ── isGoFile ─────────────────────────────────────────────────────────────────

func TestIsGoFile(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		wantGo bool
	}{
		{name: "simple .go file", path: "main.go", wantGo: true},
		{name: "nested .go file", path: "internal/commands/start/watcher.go", wantGo: true},
		{name: "test file", path: "watcher_test.go", wantGo: true},

		{name: "json file", path: "config.json", wantGo: false},
		{name: "markdown file", path: "README.md", wantGo: false},
		{name: "go.mod file", path: "go.mod", wantGo: false},
		{name: "go.sum file", path: "go.sum", wantGo: false},
		{name: "vim swap file", path: "main.go.swp", wantGo: false},
		{name: "backup file", path: "main.go~", wantGo: false},
		{name: "empty string", path: "", wantGo: false},
		{name: "just .go", path: ".go", wantGo: true},
		{name: "directory named go", path: "go", wantGo: false},
		{name: "typescript file", path: "app.ts", wantGo: false},
		{name: "html template", path: "index.html", wantGo: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isGoFile(tt.path)
			if got != tt.wantGo {
				t.Errorf("isGoFile(%q) = %v, want %v", tt.path, got, tt.wantGo)
			}
		})
	}
}

// ── hasFlag ──────────────────────────────────────────────────────────────────

func TestHasFlag(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		flag    string
		wantHas bool
	}{
		{name: "flag present alone", args: []string{"--no-watch"}, flag: "--no-watch", wantHas: true},
		{name: "flag present with others", args: []string{"--verbose", "--no-watch"}, flag: "--no-watch", wantHas: true},
		{name: "flag not present", args: []string{"--verbose"}, flag: "--no-watch", wantHas: false},
		{name: "empty args", args: []string{}, flag: "--no-watch", wantHas: false},
		{name: "nil args", args: nil, flag: "--no-watch", wantHas: false},
		{name: "partial match is not a match", args: []string{"--no-watc"}, flag: "--no-watch", wantHas: false},
		{name: "superset is not a match", args: []string{"--no-watch-extra"}, flag: "--no-watch", wantHas: false},
		{name: "flag at end", args: []string{"--other", "--no-watch"}, flag: "--no-watch", wantHas: true},
		{name: "flag at start", args: []string{"--no-watch", "--other"}, flag: "--no-watch", wantHas: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasFlag(tt.args, tt.flag)
			if got != tt.wantHas {
				t.Errorf("hasFlag(%v, %q) = %v, want %v", tt.args, tt.flag, got, tt.wantHas)
			}
		})
	}
}

// ── skippedDirs completeness ─────────────────────────────────────────────────

func TestSkippedDirsNotEmpty(t *testing.T) {
	if len(skippedDirs) == 0 {
		t.Error("skippedDirs should not be empty")
	}

	required := []string{".shipq", ".git", "node_modules", "vendor", "test_results"}
	for _, dir := range required {
		if !skippedDirs[dir] {
			t.Errorf("skippedDirs missing required entry %q", dir)
		}
	}
}
