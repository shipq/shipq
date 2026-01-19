package portapi

import (
	"errors"
	"reflect"
	"regexp"
	"runtime"
	"strings"
)

// MiddlewareRef represents a stable identity reference to a middleware function.
type MiddlewareRef struct {
	Pkg  string // import path of the package containing the middleware
	Name string // symbol name of the middleware function
	Fn   any    // the actual middleware function value
}

// Endpoint represents a registered HTTP endpoint.
type Endpoint struct {
	Method      string          // normalized: GET, POST, PUT, DELETE
	Path        string          // e.g. "/pets/{id}"
	HandlerPkg  string          // import path, e.g. "example.com/app/pets"
	HandlerName string          // symbol name, e.g. "CreatePet"
	Handler     any             // the actual handler value (used during discovery)
	HandlerInfo *HandlerInfo    // validated handler metadata (populated during validation)
	Middlewares []MiddlewareRef // ordered list of middleware applied to this endpoint
}

// newMiddlewareRef creates a MiddlewareRef from a middleware function.
func newMiddlewareRef(fn any) MiddlewareRef {
	if fn == nil {
		panic("middleware function cannot be nil")
	}

	fnVal := reflect.ValueOf(fn)
	ptr := fnVal.Pointer()
	fnInfo := runtime.FuncForPC(ptr)

	fullName := fnInfo.Name()
	// Extract package path and function name
	// Format is typically: "path/to/package.FuncName" or "path/to/package.(*Type).Method"
	lastSlash := strings.LastIndex(fullName, "/")
	lastDot := strings.LastIndex(fullName, ".")

	var pkg, name string
	if lastDot > lastSlash {
		pkg = fullName[:lastDot]
		name = fullName[lastDot+1:]
	} else {
		// No package path (e.g., main package or builtin)
		pkg = ""
		name = fullName
	}

	return MiddlewareRef{
		Pkg:  pkg,
		Name: name,
		Fn:   fn,
	}
}

var allowedMethods = map[string]bool{
	"GET": true, "POST": true, "PUT": true, "DELETE": true,
}

// NewEndpoint creates a new Endpoint with normalized method and path.
// Returns an error if the method is unsupported or the path is invalid.
func NewEndpoint(method, path string, handler any) (Endpoint, error) {
	// Normalize method
	m := strings.ToUpper(strings.TrimSpace(method))
	if m == "" {
		return Endpoint{}, errors.New("method cannot be empty")
	}
	if !allowedMethods[m] {
		return Endpoint{}, errors.New("unsupported method: " + method)
	}

	// Validate and normalize path
	p := strings.TrimSpace(path)
	if p == "" {
		return Endpoint{}, errors.New("path cannot be empty")
	}
	if !strings.HasPrefix(p, "/") {
		return Endpoint{}, errors.New("path must start with /")
	}
	// Remove trailing slash (except for root)
	if len(p) > 1 && strings.HasSuffix(p, "/") {
		p = strings.TrimSuffix(p, "/")
	}

	return Endpoint{Method: m, Path: p, Handler: handler}, nil
}

// pathVarRegex matches {name} or {name...} path variables
var pathVarRegex = regexp.MustCompile(`\{([^}]+?)(?:\.\.\.)?\}`)

// PathVariables returns the names of path variables in the endpoint path.
// For example, "/users/{user_id}/posts/{post_id}" returns ["user_id", "post_id"].
// Wildcard suffixes like {path...} are returned as just "path".
func (e Endpoint) PathVariables() []string {
	matches := pathVarRegex.FindAllStringSubmatch(e.Path, -1)
	if len(matches) == 0 {
		return nil
	}

	vars := make([]string, len(matches))
	for i, match := range matches {
		vars[i] = match[1]
	}
	return vars
}

// MuxPattern returns the Go 1.22+ ServeMux pattern string.
// Format: "METHOD /path" (e.g., "GET /pets/{id}")
func (e Endpoint) MuxPattern() string {
	return e.Method + " " + e.Path
}
