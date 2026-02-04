package codegen

// DiscoverAPIPackages finds all Go packages under the api directory
// that contain a Register function.
// Returns a slice of import paths relative to the module.
//
// For example, if projectRoot is "/home/user/myapp", and modulePath is
// "github.com/user/myapp", it will return paths like:
//   - "github.com/user/myapp/api/posts"
//   - "github.com/user/myapp/api/users"
//   - "github.com/user/myapp/api/comments"
func DiscoverAPIPackages(projectRoot, modulePath string) ([]string, error) {
	return DiscoverPackages(projectRoot, "api", modulePath)
}
