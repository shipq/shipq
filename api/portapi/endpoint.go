package portapi

import (
	"errors"
	"regexp"
	"strings"
)

// Endpoint represents a registered HTTP endpoint.
type Endpoint struct {
	Method      string       // normalized: GET, POST, PUT, DELETE
	Path        string       // e.g. "/pets/{id}"
	HandlerPkg  string       // import path, e.g. "example.com/app/pets"
	HandlerName string       // symbol name, e.g. "CreatePet"
	Handler     any          // the actual handler value (used during discovery)
	HandlerInfo *HandlerInfo // validated handler metadata (populated during validation)
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
