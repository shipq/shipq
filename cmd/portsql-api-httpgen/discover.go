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
// If middlewarePkgPath is non-empty, it also executes RegisterMiddleware from that package.
func Discover(pkgPath string, middlewarePkgPath string) (*Manifest, error) {
	// 1. Create temp dir
	tmpDir, err := os.MkdirTemp("", "portsql-api-httpgen-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// 2. Write runner main.go that imports pkgPath and optionally middlewarePkgPath
	runnerCode := GenerateRunnerCode(pkgPath, middlewarePkgPath)
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
// - optionally calls RegisterMiddleware(reg) if middlewarePkgPath is non-empty
// - validates all endpoints and middleware
// - prints JSON manifest to stdout
func GenerateRunnerCode(pkgPath string, middlewarePkgPath string) string {
	// Extract the package alias from the import path
	parts := strings.Split(pkgPath, "/")
	pkgAlias := parts[len(parts)-1]

	// Build imports section
	imports := fmt.Sprintf("\t\"github.com/shipq/shipq/api/portapi\"\n\t%s %q\n", pkgAlias, pkgPath)

	var mwAlias string
	if middlewarePkgPath != "" {
		mwParts := strings.Split(middlewarePkgPath, "/")
		mwAlias = mwParts[len(mwParts)-1]
		imports += fmt.Sprintf("\t%s %q\n", mwAlias, middlewarePkgPath)
	}

	// Build middleware registration code
	mwRegistration := ""
	if middlewarePkgPath != "" {
		mwRegistration = fmt.Sprintf(`
	// Register middleware
	reg := &portapi.MiddlewareRegistry{}
	%s.RegisterMiddleware(reg)

	// Validate strict mode
	middlewareConfigured := true
	if err := portapi.ValidateStrictMiddlewareDeclaration(endpoints, reg, middlewareConfigured); err != nil {
		fmt.Fprintf(os.Stderr, "middleware validation error: %%v\n", err)
		os.Exit(1)
	}

	// Add middleware to manifest
	manifest.Middlewares = make([]ManifestMiddleware, 0, len(reg.Middlewares()))
	for _, mw := range reg.Middlewares() {
		manifest.Middlewares = append(manifest.Middlewares, ManifestMiddleware{
			Pkg:  mw.Pkg,
			Name: mw.Name,
		})
	}

	// Add context keys to manifest
	providedKeys := reg.ProvidedKeys()
	manifest.ContextKeys = make([]ManifestContextKey, 0, len(providedKeys))
	for _, pk := range providedKeys {
		manifest.ContextKeys = append(manifest.ContextKeys, ManifestContextKey{
			Key:  pk.Key,
			Type: pk.Type,
		})
	}

	// Add middleware metadata to manifest
	manifest.MiddlewareMetadata = make(map[string]*ManifestMiddlewareMetadata)
	for _, mw := range reg.Middlewares() {
		key := mw.Pkg + "." + mw.Name
		meta := reg.GetMetadata(mw.Fn)
		if meta != nil {
			mmeta := &ManifestMiddlewareMetadata{
				RequiredHeaders:   meta.RequiredHeaders,
				RequiredCookies:   meta.RequiredCookies,
				SecuritySchemes:   meta.SecuritySchemes,
			}
			for _, status := range meta.MayReturnStatuses {
				mmeta.MayReturnStatuses = append(mmeta.MayReturnStatuses, ManifestMayReturnStatus{
					Status:      status.Status,
					Description: status.Description,
				})
			}
			manifest.MiddlewareMetadata[key] = mmeta
		}
	}
`, mwAlias)
	} else {
		mwRegistration = `
	// No middleware package configured - validate that no middleware is used
	middlewareConfigured := false
	if err := portapi.ValidateStrictMiddlewareDeclaration(endpoints, nil, middlewareConfigured); err != nil {
		fmt.Fprintf(os.Stderr, "middleware validation error: %v\n", err)
		os.Exit(1)
	}
`
	}

	return fmt.Sprintf(`// Code generated by portsql-api-httpgen. DO NOT EDIT.

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	goruntime "runtime"
	"strings"

%s)

type Manifest struct {
	Endpoints          []ManifestEndpoint                     `+"`json:\"endpoints\"`"+`
	Middlewares        []ManifestMiddleware                   `+"`json:\"middlewares,omitempty\"`"+`
	ContextKeys        []ManifestContextKey                   `+"`json:\"context_keys,omitempty\"`"+`
	MiddlewareMetadata map[string]*ManifestMiddlewareMetadata `+"`json:\"middleware_metadata,omitempty\"`"+`
}

type ManifestMiddleware struct {
	Pkg  string `+"`json:\"pkg\"`"+`
	Name string `+"`json:\"name\"`"+`
}

type ManifestContextKey struct {
	Key  string `+"`json:\"key\"`"+`
	Type string `+"`json:\"type\"`"+`
}

type ManifestMiddlewareMetadata struct {
	RequiredHeaders   []string                  `+"`json:\"required_headers,omitempty\"`"+`
	RequiredCookies   []string                  `+"`json:\"required_cookies,omitempty\"`"+`
	SecuritySchemes   []string                  `+"`json:\"security_schemes,omitempty\"`"+`
	MayReturnStatuses []ManifestMayReturnStatus `+"`json:\"may_return_statuses,omitempty\"`"+`
}

type ManifestMayReturnStatus struct {
	Status      int    `+"`json:\"status\"`"+`
	Description string `+"`json:\"description\"`"+`
}

type ManifestEndpoint struct {
	Method      string               `+"`json:\"method\"`"+`
	Path        string               `+"`json:\"path\"`"+`
	HandlerPkg  string               `+"`json:\"handler_pkg\"`"+`
	HandlerName string               `+"`json:\"handler_name\"`"+`
	Shape       string               `+"`json:\"shape\"`"+`
	ReqType     string               `+"`json:\"req_type,omitempty\"`"+`
	RespType    string               `+"`json:\"resp_type,omitempty\"`"+`
	Middlewares []ManifestMiddleware `+"`json:\"middlewares,omitempty\"`"+`
	Bindings    *BindingInfo         `+"`json:\"bindings,omitempty\"`"+`
}

type BindingInfo struct {
	HasJSONBody    bool           `+"`json:\"has_json_body\"`"+`
	PathBindings   []FieldBinding `+"`json:\"path_bindings,omitempty\"`"+`
	QueryBindings  []FieldBinding `+"`json:\"query_bindings,omitempty\"`"+`
	HeaderBindings []FieldBinding `+"`json:\"header_bindings,omitempty\"`"+`
}

type FieldBinding struct {
	FieldName string `+"`json:\"field_name\"`"+`
	TagValue  string `+"`json:\"tag_value\"`"+`
	TypeKind  string `+"`json:\"type_kind\"`"+`
	IsPointer bool   `+"`json:\"is_pointer\"`"+`
	IsSlice   bool   `+"`json:\"is_slice\"`"+`
	ElemKind  string `+"`json:\"elem_kind,omitempty\"`"+`
}

func main() {
	app := &portapi.App{}
	%s.Register(app)

	endpoints := app.Endpoints()
	manifest := Manifest{
		Endpoints: make([]ManifestEndpoint, 0, len(endpoints)),
	}
%s

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

			// Analyze bindings for the request type
			bindingInfo, err := portapi.AnalyzeBindings(ep.Path, info.ReqType)
			if err != nil {
				fmt.Fprintf(os.Stderr, "binding analysis error: %%v\n", err)
				os.Exit(1)
			}

			me.Bindings = convertBindingInfo(bindingInfo)
		}
		if info.RespType != nil {
			me.RespType = info.RespType.String()
		}

		// Add endpoint middlewares
		me.Middlewares = make([]ManifestMiddleware, 0, len(ep.Middlewares))
		for _, mw := range ep.Middlewares {
			me.Middlewares = append(me.Middlewares, ManifestMiddleware{
				Pkg:  mw.Pkg,
				Name: mw.Name,
			})
		}

		manifest.Endpoints = append(manifest.Endpoints, me)
	}

	enc := json.NewEncoder(os.Stdout)
	if err := enc.Encode(manifest); err != nil {
		fmt.Fprintf(os.Stderr, "failed to encode manifest: %%v\n", err)
		os.Exit(1)
	}
}

func convertBindingInfo(info *portapi.BindingInfo) *BindingInfo {
	if info == nil {
		return nil
	}

	bi := &BindingInfo{
		HasJSONBody: info.HasJSONBody,
	}

	for _, fb := range info.PathBindings {
		bi.PathBindings = append(bi.PathBindings, convertFieldBinding(fb))
	}
	for _, fb := range info.QueryBindings {
		bi.QueryBindings = append(bi.QueryBindings, convertFieldBinding(fb))
	}
	for _, fb := range info.HeaderBindings {
		bi.HeaderBindings = append(bi.HeaderBindings, convertFieldBinding(fb))
	}

	return bi
}

func convertFieldBinding(fb portapi.FieldBinding) FieldBinding {
	typeKind, isPtr, isSlice, elemKind := analyzeType(fb.FieldType)
	return FieldBinding{
		FieldName: fb.FieldName,
		TagValue:  fb.TagValue,
		TypeKind:  typeKind,
		IsPointer: isPtr,
		IsSlice:   isSlice,
		ElemKind:  elemKind,
	}
}

func analyzeType(t reflect.Type) (typeKind string, isPointer, isSlice bool, elemKind string) {
	// Handle pointer types
	if t.Kind() == reflect.Ptr {
		isPointer = true
		t = t.Elem()
	}

	// Handle slice types
	if t.Kind() == reflect.Slice {
		isSlice = true
		elemType := t.Elem()
		elemKind = elemType.Kind().String()
		if elemType.String() == "time.Time" {
			elemKind = "time.Time"
		}
		typeKind = "[]" + elemKind
		return
	}

	// Scalar type
	typeKind = t.Kind().String()
	if t.String() == "time.Time" {
		typeKind = "time.Time"
	}
	return
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
`, imports, pkgAlias, mwRegistration)
}
