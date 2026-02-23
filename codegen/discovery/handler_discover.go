package discovery

import (
	"os"
	"path/filepath"
	"strings"
)

// DiscoverAPIPackages finds all Go packages under the api directory
// that contain a Register function (indicated by the presence of a
// register.go file).
// Returns a slice of import paths relative to the module.
//
// Parameters:
//   - goModRoot: directory containing go.mod
//   - shipqRoot: directory containing shipq.ini (where api/ directory lives)
//   - modulePath: module path from go.mod
//
// For example, in a monorepo where goModRoot is "/monorepo", shipqRoot is "/monorepo/services/myservice",
// and modulePath is "github.com/company/monorepo", it will return paths like:
//   - "github.com/company/monorepo/services/myservice/api/posts"
//   - "github.com/company/monorepo/services/myservice/api/users"
//   - "github.com/company/monorepo/services/myservice/api/comments"
func DiscoverAPIPackages(goModRoot, shipqRoot, modulePath string) ([]string, error) {
	allPkgs, err := DiscoverPackages(goModRoot, shipqRoot, "api", modulePath)
	if err != nil {
		return nil, err
	}

	// Filter to only packages that have a register.go file, since the handler
	// compile program calls Register() on every discovered package.
	// This excludes the root api/ directory (which contains generated server
	// files but no Register function) and any other non-handler packages.
	var filtered []string
	for _, pkg := range allPkgs {
		// Convert import path back to filesystem path
		relImport := strings.TrimPrefix(pkg, modulePath+"/")
		dirPath := filepath.Join(goModRoot, relImport)
		registerPath := filepath.Join(dirPath, "register.go")
		if _, err := os.Stat(registerPath); err == nil {
			filtered = append(filtered, pkg)
		}
	}

	return filtered, nil
}
