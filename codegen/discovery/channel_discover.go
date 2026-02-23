package discovery

import (
	"os"
	"path/filepath"
	"strings"
)

// DiscoverChannelPackages finds all Go packages under the channels directory
// that contain a register.go file (indicating they have a Register function
// for channel registration).
// Returns a slice of import paths relative to the module.
//
// Parameters:
//   - goModRoot: directory containing go.mod
//   - shipqRoot: directory containing shipq.ini (where channels/ directory lives)
//   - modulePath: module path from go.mod
//
// For example, in a monorepo where goModRoot is "/monorepo", shipqRoot is "/monorepo/services/myservice",
// and modulePath is "github.com/company/monorepo", it will return paths like:
//   - "github.com/company/monorepo/services/myservice/channels/email"
//   - "github.com/company/monorepo/services/myservice/channels/chatbot"
func DiscoverChannelPackages(goModRoot, shipqRoot, modulePath string) ([]string, error) {
	allPkgs, err := DiscoverPackages(goModRoot, shipqRoot, "channels", modulePath)
	if err != nil {
		return nil, err
	}

	// Filter to only packages that have a register.go file, since the channel
	// compile program calls Register() on every discovered package.
	// This excludes the root channels/ directory (which may contain shared
	// types but no Register function) and any other non-channel packages.
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
