package discovery

import (
	"os"
	"path/filepath"
)

// DiscoverPackages finds all Go packages under a directory.
// Returns a slice of import paths relative to the module.
//
// Parameters:
//   - goModRoot: directory containing go.mod
//   - shipqRoot: directory containing shipq.ini (may be same as goModRoot or a subdirectory)
//   - dir: subdirectory to search within shipqRoot (e.g., "querydefs")
//   - modulePath: module path from go.mod
//
// For example, in a monorepo where goModRoot is "/monorepo", shipqRoot is "/monorepo/services/myservice",
// dir is "querydefs", and modulePath is "github.com/company/monorepo", it will return paths like:
//   - "github.com/company/monorepo/services/myservice/querydefs"
//   - "github.com/company/monorepo/services/myservice/querydefs/users"
//   - "github.com/company/monorepo/services/myservice/querydefs/orders"
func DiscoverPackages(goModRoot, shipqRoot, dir, modulePath string) ([]string, error) {
	var packages []string
	baseDir := filepath.Join(shipqRoot, dir)

	// Check if the directory exists
	if _, err := os.Stat(baseDir); os.IsNotExist(err) {
		// Directory doesn't exist - return empty list (not an error)
		return packages, nil
	}

	err := filepath.WalkDir(baseDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}

		// Skip hidden directories (like .git)
		if d.Name() != "." && d.Name()[0] == '.' {
			return filepath.SkipDir
		}

		// Skip vendor directories
		if d.Name() == "vendor" {
			return filepath.SkipDir
		}

		// Skip testdata directories
		if d.Name() == "testdata" {
			return filepath.SkipDir
		}

		// Check if directory contains Go files
		hasGoFiles, err := containsGoFiles(path)
		if err != nil {
			return err
		}
		if !hasGoFiles {
			return nil
		}

		// Convert to import path - must be relative to goModRoot for correct Go imports
		relPath, err := filepath.Rel(goModRoot, path)
		if err != nil {
			return err
		}
		importPath := modulePath + "/" + filepath.ToSlash(relPath)
		packages = append(packages, importPath)

		return nil
	})

	return packages, err
}

// containsGoFiles checks if a directory contains any .go files
// (excluding _test.go files since we only want production code).
func containsGoFiles(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if filepath.Ext(name) == ".go" {
			// Skip test files
			if len(name) > 8 && name[len(name)-8:] == "_test.go" {
				continue
			}
			return true, nil
		}
	}
	return false, nil
}

// DiscoverQuerydefsPackages is a convenience function that discovers
// packages in the standard "querydefs" directory.
func DiscoverQuerydefsPackages(goModRoot, shipqRoot, modulePath string) ([]string, error) {
	return DiscoverPackages(goModRoot, shipqRoot, "querydefs", modulePath)
}
