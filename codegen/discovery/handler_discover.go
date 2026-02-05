package discovery

// DiscoverAPIPackages finds all Go packages under the api directory
// that contain a Register function.
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
	return DiscoverPackages(goModRoot, shipqRoot, "api", modulePath)
}
