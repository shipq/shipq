package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Discover executes a temporary runner program to discover endpoints from a package.
// It returns a manifest describing all registered endpoints.
func Discover(pkgPath string) (*Manifest, error) {
	// 1. Create temp dir
	tmpDir, err := os.MkdirTemp("", "portsql-api-httpgen-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// 2. Write runner main.go that imports pkgPath
	runnerCode := GenerateRunnerCode(pkgPath)
	runnerPath := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(runnerPath, []byte(runnerCode), 0644); err != nil {
		return nil, fmt.Errorf("failed to write runner: %w", err)
	}

	// 3. Write go.mod that requires the main module
	// Get the current module path from go list
	modPath, modDir, err := getCurrentModule()
	if err != nil {
		return nil, fmt.Errorf("failed to get current module: %w", err)
	}

	// Read existing replace directives from the current module's go.mod
	replaceDirectives, err := readReplaceDirectives(modDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read replace directives: %w", err)
	}

	// Extract the module part of the target package path
	// pkgPath could be "example.com/testapp/api" and we need "example.com/testapp"
	targetModPath := extractModulePath(pkgPath, modPath)

	// Build go.mod with replace directives
	// We need to replace both the target module AND any modules it depends on
	var goModBuilder strings.Builder
	goModBuilder.WriteString("module portsql-api-httpgen-runner\n\ngo 1.22\n\n")

	// Require the target module
	goModBuilder.WriteString(fmt.Sprintf("require %s v0.0.0\n", targetModPath))

	// Replace directive for the target module
	goModBuilder.WriteString(fmt.Sprintf("replace %s => %s\n", targetModPath, modDir))

	// Copy all replace directives from the current module
	// This ensures that any local dependencies (like github.com/shipq/shipq) are also replaced
	for mod, dir := range replaceDirectives {
		if mod != targetModPath { // Don't duplicate the target module replace
			goModBuilder.WriteString(fmt.Sprintf("require %s v0.0.0\n", mod))
			goModBuilder.WriteString(fmt.Sprintf("replace %s => %s\n", mod, dir))
		}
	}

	// If the target module is not the shipq module, we also need to add the portapi dependency
	// The runner imports portapi, so we need to make sure it's available
	const portapiModule = "github.com/shipq/shipq"
	if targetModPath != portapiModule && !strings.HasPrefix(targetModPath, portapiModule+"/") {
		// Check if we already have a replace for it from the current module
		if _, hasReplace := replaceDirectives[portapiModule]; !hasReplace {
			// Check if portapi is in the same module we're running from
			if modPath == portapiModule || strings.HasPrefix(modPath, portapiModule+"/") {
				// We're running from within the shipq module
				goModBuilder.WriteString(fmt.Sprintf("require %s v0.0.0\n", portapiModule))
				goModBuilder.WriteString(fmt.Sprintf("replace %s => %s\n", portapiModule, modDir))
			}
		}
	}

	goModPath := filepath.Join(tmpDir, "go.mod")
	if err := os.WriteFile(goModPath, []byte(goModBuilder.String()), 0644); err != nil {
		return nil, fmt.Errorf("failed to write go.mod: %w", err)
	}

	// 4. Write empty go.sum to avoid checksum issues
	goSumPath := filepath.Join(tmpDir, "go.sum")
	if err := os.WriteFile(goSumPath, []byte(""), 0644); err != nil {
		return nil, fmt.Errorf("failed to write go.sum: %w", err)
	}

	// 5. Execute `go run` and capture stdout
	cmd := exec.Command("go", "run", ".")
	cmd.Dir = tmpDir
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("runner failed: %w\nstderr: %s", err, stderr.String())
	}

	// 6. Parse JSON manifest from stdout
	var manifest Manifest
	if err := json.Unmarshal(stdout.Bytes(), &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w\noutput: %s", err, stdout.String())
	}

	return &manifest, nil
}

// readReplaceDirectives reads replace directives from a go.mod file.
// Returns a map of module path -> replacement path.
func readReplaceDirectives(modDir string) (map[string]string, error) {
	goModPath := filepath.Join(modDir, "go.mod")
	f, err := os.Open(goModPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	replaces := make(map[string]string)
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "replace ") {
			// Parse: replace module/path => /local/path
			// or: replace module/path => other/module version
			line = strings.TrimPrefix(line, "replace ")
			parts := strings.Split(line, " => ")
			if len(parts) == 2 {
				modPath := strings.TrimSpace(parts[0])
				// Remove version from modPath if present
				if idx := strings.Index(modPath, " "); idx != -1 {
					modPath = modPath[:idx]
				}
				replacement := strings.TrimSpace(parts[1])
				// Only include local path replacements (not version replacements)
				if !strings.Contains(replacement, " ") && (strings.HasPrefix(replacement, "/") || strings.HasPrefix(replacement, ".")) {
					replaces[modPath] = replacement
				}
			}
		}
	}

	return replaces, scanner.Err()
}

// extractModulePath tries to determine the module path from a package path.
// If the package is within the current module, it returns the current module path.
// Otherwise, it tries to extract a reasonable module path from the package path.
func extractModulePath(pkgPath, currentModPath string) string {
	// If the package is within the current module, use the current module
	if strings.HasPrefix(pkgPath, currentModPath) {
		return currentModPath
	}

	// Otherwise, try to guess the module path
	// Common patterns: github.com/org/repo, example.com/name
	parts := strings.Split(pkgPath, "/")
	if len(parts) >= 3 {
		// Assume first 3 parts are the module (e.g., github.com/org/repo)
		return strings.Join(parts[:3], "/")
	}
	if len(parts) >= 2 {
		return strings.Join(parts[:2], "/")
	}
	return pkgPath
}

// getCurrentModule returns the current module path and directory.
func getCurrentModule() (string, string, error) {
	cmd := exec.Command("go", "list", "-m", "-json")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return "", "", err
	}

	var mod struct {
		Path string `json:"Path"`
		Dir  string `json:"Dir"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &mod); err != nil {
		return "", "", err
	}

	return mod.Path, mod.Dir, nil
}

// GenerateRunnerCode generates the main.go source for the discovery runner.
// The generated program:
// - imports the target package
// - creates a portapi.App
// - calls Register(app)
// - validates all endpoints
// - prints JSON manifest to stdout
func GenerateRunnerCode(pkgPath string) string {
	// Extract the package alias from the import path
	parts := strings.Split(pkgPath, "/")
	pkgAlias := parts[len(parts)-1]

	return fmt.Sprintf(`// Code generated by portsql-api-httpgen. DO NOT EDIT.

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	goruntime "runtime"
	"strings"

	"github.com/shipq/shipq/api/portapi"
	%s %q
)

type Manifest struct {
	Endpoints []ManifestEndpoint `+"`json:\"endpoints\"`"+`
}

type ManifestEndpoint struct {
	Method      string `+"`json:\"method\"`"+`
	Path        string `+"`json:\"path\"`"+`
	HandlerPkg  string `+"`json:\"handler_pkg\"`"+`
	HandlerName string `+"`json:\"handler_name\"`"+`
	Shape       string `+"`json:\"shape\"`"+`
	ReqType     string `+"`json:\"req_type,omitempty\"`"+`
	RespType    string `+"`json:\"resp_type,omitempty\"`"+`
}

func main() {
	app := &portapi.App{}
	%s.Register(app)

	endpoints := app.Endpoints()
	manifest := Manifest{
		Endpoints: make([]ManifestEndpoint, 0, len(endpoints)),
	}

	for _, ep := range endpoints {
		// Validate the endpoint
		if err := portapi.ValidateEndpoint(ep); err != nil {
			fmt.Fprintf(os.Stderr, "validation error: %%v\n", err)
			os.Exit(1)
		}

		// Get handler info
		info, err := portapi.ValidateHandler(ep.Handler)
		if err != nil {
			fmt.Fprintf(os.Stderr, "handler validation error: %%v\n", err)
			os.Exit(1)
		}

		// Extract handler function name and package
		handlerPkg, handlerName := extractHandlerInfo(ep.Handler)

		me := ManifestEndpoint{
			Method:      ep.Method,
			Path:        ep.Path,
			HandlerPkg:  handlerPkg,
			HandlerName: handlerName,
			Shape:       shapeToString(info.Shape),
		}

		if info.ReqType != nil {
			me.ReqType = info.ReqType.String()
		}
		if info.RespType != nil {
			me.RespType = info.RespType.String()
		}

		manifest.Endpoints = append(manifest.Endpoints, me)
	}

	enc := json.NewEncoder(os.Stdout)
	if err := enc.Encode(manifest); err != nil {
		fmt.Fprintf(os.Stderr, "failed to encode manifest: %%v\n", err)
		os.Exit(1)
	}
}

func extractHandlerInfo(handler any) (pkg, name string) {
	v := reflect.ValueOf(handler)
	if v.Kind() != reflect.Func {
		return "", ""
	}

	ptr := v.Pointer()
	fn := goruntime.FuncForPC(ptr)
	if fn == nil {
		return "", ""
	}

	fullName := fn.Name()
	// fullName is like "github.com/example/pkg.FuncName" or "github.com/example/pkg.(*T).Method"
	lastDot := strings.LastIndex(fullName, ".")
	if lastDot == -1 {
		return "", fullName
	}

	pkg = fullName[:lastDot]
	name = fullName[lastDot+1:]

	// Handle method receivers like "(*T).Method"
	if strings.HasPrefix(name, "(") {
		// Find the actual method name after "(*T)."
		if idx := strings.LastIndex(name, "."); idx != -1 {
			name = name[idx+1:]
		}
	}

	return pkg, name
}

func shapeToString(shape portapi.HandlerShape) string {
	switch shape {
	case portapi.ShapeCtxReqRespErr:
		return "ctx_req_resp_err"
	case portapi.ShapeCtxReqErr:
		return "ctx_req_err"
	case portapi.ShapeCtxRespErr:
		return "ctx_resp_err"
	case portapi.ShapeCtxErr:
		return "ctx_err"
	default:
		return "unknown"
	}
}
`, pkgAlias, pkgPath, pkgAlias)
}
