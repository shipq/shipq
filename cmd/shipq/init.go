package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/shipq/shipq/cli"
	"github.com/shipq/shipq/inifile"
	"github.com/shipq/shipq/project"
)

// initCmd implements the "shipq init" command.
// It initializes a new shipq project by creating go.mod and shipq.ini if needed.
func initCmd() {
	cwd, err := os.Getwd()
	if err != nil {
		cli.FatalErr("failed to get current directory", err)
	}

	projectName := project.GetProjectName(cwd)
	createdGoMod := false
	createdShipqIni := false
	updatedGitignore := false

	// Create go.mod if it doesn't exist
	if !project.HasGoMod(cwd) {
		if err := createGoMod(cwd, projectName); err != nil {
			cli.FatalErr("failed to create go.mod", err)
		}
		createdGoMod = true
	}

	// Create shipq.ini if it doesn't exist
	if !project.HasShipqIni(cwd) {
		if err := createShipqIni(cwd); err != nil {
			cli.FatalErr("failed to create shipq.ini", err)
		}
		createdShipqIni = true
	}

	// Create or update .gitignore to include .shipq/
	updated, err := ensureGitignore(cwd)
	if err != nil {
		cli.FatalErr("failed to update .gitignore", err)
	}
	updatedGitignore = updated

	// Print results
	if createdGoMod && createdShipqIni {
		cli.Success("Initialized new shipq project")
		cli.Infof("  Created go.mod (module: com.%s)", projectName)
		cli.Info("  Created shipq.ini")
		if updatedGitignore {
			cli.Info("  Updated .gitignore")
		}
	} else if createdGoMod {
		cli.Success("Created go.mod")
		cli.Infof("  Module: com.%s", projectName)
		if updatedGitignore {
			cli.Info("  Updated .gitignore")
		}
	} else if createdShipqIni {
		cli.Success("Created shipq.ini")
		if updatedGitignore {
			cli.Info("  Updated .gitignore")
		}
	} else if updatedGitignore {
		cli.Success("Updated .gitignore")
	} else {
		cli.Info("Project already initialized (go.mod and shipq.ini exist)")
	}
}

// createGoMod creates a new go.mod file with the module name "com.<projectName>"
func createGoMod(dir, projectName string) error {
	goVersion := getGoVersion()
	moduleName := fmt.Sprintf("com.%s", projectName)

	content := fmt.Sprintf("module %s\n\ngo %s\n", moduleName, goVersion)

	goModPath := filepath.Join(dir, project.GoModFile)
	return os.WriteFile(goModPath, []byte(content), 0644)
}

// createShipqIni creates a new shipq.ini file with an empty [db] section
func createShipqIni(dir string) error {
	f := &inifile.File{}
	// Create empty db section by setting a placeholder that we'll leave empty
	// The Set function will create the section
	f.Sections = append(f.Sections, inifile.Section{Name: "db"})

	shipqIniPath := filepath.Join(dir, project.ShipqIniFile)
	return f.WriteFile(shipqIniPath)
}

// ensureGitignore creates or updates .gitignore to include .shipq/
// Returns true if the file was created or modified, false if already up-to-date.
func ensureGitignore(dir string) (bool, error) {
	gitignorePath := filepath.Join(dir, ".gitignore")
	shipqEntry := ".shipq/"

	// Read existing .gitignore if it exists
	content, err := os.ReadFile(gitignorePath)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}

	// Check if .shipq/ is already in the file
	if err == nil {
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			// Check for exact match or common variations
			if trimmed == shipqEntry || trimmed == ".shipq" || trimmed == "/.shipq/" || trimmed == "/.shipq" {
				// Already present
				return false, nil
			}
		}
	}

	// Append .shipq/ to the file
	var newContent string
	if len(content) == 0 {
		// New file - add header and entry
		newContent = "# shipq generated files\n" + shipqEntry + "\n"
	} else {
		// Existing file - append with appropriate newline handling
		existingContent := string(content)
		// Ensure there's a newline before our entry
		if !strings.HasSuffix(existingContent, "\n") {
			existingContent += "\n"
		}
		newContent = existingContent + "\n# shipq generated files\n" + shipqEntry + "\n"
	}

	if err := os.WriteFile(gitignorePath, []byte(newContent), 0644); err != nil {
		return false, err
	}

	return true, nil
}

// getGoVersion returns the current Go version in "X.Y" format
func getGoVersion() string {
	version := runtime.Version()
	// runtime.Version() returns something like "go1.21.5"
	// We want "1.21"
	if len(version) > 2 && version[:2] == "go" {
		version = version[2:]
	}

	// Extract major.minor
	dotCount := 0
	for i, c := range version {
		if c == '.' {
			dotCount++
			if dotCount == 2 {
				return version[:i]
			}
		}
	}
	return version
}
