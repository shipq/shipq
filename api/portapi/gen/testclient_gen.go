// Package gen provides code generation utilities for PortAPI.
//
// This file implements the test client generator which produces type-safe
// HTTP client code for testing API endpoints.
package gen

import (
	"bytes"
	"fmt"
	"go/format"
	"reflect"
	"sort"
	"strings"
	"text/template"
	"unicode"

	"github.com/shipq/shipq/api/portapi"
)

// TestClientGenerator generates test client code from endpoint definitions.
type TestClientGenerator struct {
	// PackageName is the Go package name for the generated file.
	PackageName string

	// Endpoints are the endpoints to generate client methods for.
	Endpoints []portapi.Endpoint

	// TypesPackage is the import path for request/response types.
	// If empty, types are assumed to be in the same package.
	TypesPackage string

	// TypesAlias is the import alias for the types package.
	// If empty, no alias is used.
	TypesAlias string
}

// EndpointInfo contains analyzed information about an endpoint for code generation.
type EndpointInfo struct {
	// MethodName is the Go method name for this endpoint (e.g., "CreatePet").
	MethodName string

	// HTTPMethod is the HTTP method (GET, POST, etc.).
	HTTPMethod string

	// Path is the URL path pattern (e.g., "/pets/{id}").
	Path string

	// HasRequest indicates whether the endpoint takes a request parameter.
	HasRequest bool

	// HasResponse indicates whether the endpoint returns a JSON response.
	HasResponse bool

	// RequestTypeName is the Go type name for the request (e.g., "CreatePetReq").
	RequestTypeName string

	// ResponseTypeName is the Go type name for the response (e.g., "CreatePetResp").
	ResponseTypeName string

	// Bindings contains the analyzed binding information.
	Bindings *portapi.BindingInfo

	// PathVars are the path variable names in order.
	PathVars []string
}

// Generate produces the test client Go source code.
func (g *TestClientGenerator) Generate() ([]byte, error) {
	// Validate inputs
	if g.PackageName == "" {
		return nil, fmt.Errorf("PackageName is required")
	}

	// Analyze endpoints
	endpoints, err := g.analyzeEndpoints()
	if err != nil {
		return nil, fmt.Errorf("analyze endpoints: %w", err)
	}

	// Build template data
	data := g.buildTemplateData(endpoints)

	// Execute template
	var buf bytes.Buffer
	if err := testClientTemplate.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("execute template: %w", err)
	}

	// Format the output
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		// Return unformatted source for debugging
		return buf.Bytes(), fmt.Errorf("format source: %w\n\nUnformatted source:\n%s", err, buf.String())
	}

	return formatted, nil
}

// analyzeEndpoints validates and extracts information from endpoints.
func (g *TestClientGenerator) analyzeEndpoints() ([]EndpointInfo, error) {
	// Sort endpoints for deterministic output
	sorted := portapi.SortEndpoints(g.Endpoints)

	// Track method names to avoid collisions
	usedNames := make(map[string]int)

	var infos []EndpointInfo
	for _, ep := range sorted {
		info, err := g.analyzeEndpoint(ep, usedNames)
		if err != nil {
			return nil, fmt.Errorf("endpoint %s %s: %w", ep.Method, ep.Path, err)
		}
		infos = append(infos, info)
	}

	return infos, nil
}

// analyzeEndpoint extracts information from a single endpoint.
func (g *TestClientGenerator) analyzeEndpoint(ep portapi.Endpoint, usedNames map[string]int) (EndpointInfo, error) {
	info := EndpointInfo{
		HTTPMethod: ep.Method,
		Path:       ep.Path,
		PathVars:   ep.PathVariables(),
	}

	// Determine method name
	baseName := g.deriveMethodName(ep)
	info.MethodName = g.uniqueMethodName(baseName, usedNames)

	// Validate handler and get type info
	handlerInfo, err := portapi.ValidateHandler(ep.Handler)
	if err != nil {
		return info, err
	}

	// Set request/response info
	if handlerInfo.ReqType != nil {
		info.HasRequest = true
		info.RequestTypeName = g.typeName(handlerInfo.ReqType)

		// Analyze bindings
		bindings, err := portapi.AnalyzeBindings(ep.Path, handlerInfo.ReqType)
		if err != nil {
			return info, fmt.Errorf("analyze bindings: %w", err)
		}
		info.Bindings = bindings
	}

	if handlerInfo.RespType != nil {
		info.HasResponse = true
		info.ResponseTypeName = g.typeName(handlerInfo.RespType)
	}

	return info, nil
}

// deriveMethodName generates a method name from the endpoint.
func (g *TestClientGenerator) deriveMethodName(ep portapi.Endpoint) string {
	// Prefer handler name if available
	if ep.HandlerName != "" {
		return ep.HandlerName
	}

	// Derive from method + path
	// Example: GET /pets/{id} -> GetPetsById
	name := strings.Title(strings.ToLower(ep.Method))

	// Process path segments
	segments := strings.Split(ep.Path, "/")
	for _, seg := range segments {
		if seg == "" {
			continue
		}
		// Skip path variables
		if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
			varName := strings.TrimSuffix(strings.TrimPrefix(seg, "{"), "}")
			varName = strings.TrimSuffix(varName, "...")
			name += "By" + toPascalCase(varName)
		} else {
			name += toPascalCase(seg)
		}
	}

	return name
}

// uniqueMethodName ensures the method name is unique by adding a suffix if needed.
func (g *TestClientGenerator) uniqueMethodName(baseName string, usedNames map[string]int) string {
	count := usedNames[baseName]
	usedNames[baseName] = count + 1

	if count == 0 {
		return baseName
	}
	return fmt.Sprintf("%s_%d", baseName, count+1)
}

// typeName returns the Go type name for a reflect.Type.
func (g *TestClientGenerator) typeName(t reflect.Type) string {
	// Handle pointer types
	if t.Kind() == reflect.Ptr {
		return "*" + g.typeName(t.Elem())
	}

	name := t.Name()
	if name == "" {
		// Anonymous type - use full string representation
		return t.String()
	}

	// Add package prefix if types are in a different package
	if g.TypesPackage != "" {
		alias := g.TypesAlias
		if alias == "" {
			// Use the last component of the package path as alias
			parts := strings.Split(g.TypesPackage, "/")
			alias = parts[len(parts)-1]
		}
		return alias + "." + name
	}

	return name
}

// toPascalCase converts a string to PascalCase.
func toPascalCase(s string) string {
	if s == "" {
		return ""
	}

	var result strings.Builder
	capitalizeNext := true

	for _, r := range s {
		if r == '_' || r == '-' || r == ' ' {
			capitalizeNext = true
			continue
		}
		if capitalizeNext {
			result.WriteRune(unicode.ToUpper(r))
			capitalizeNext = false
		} else {
			result.WriteRune(r)
		}
	}

	return result.String()
}

// templateData contains all data needed to render the template.
type templateData struct {
	PackageName  string
	Imports      []importSpec
	Endpoints    []EndpointInfo
	TypesPackage string
	TypesAlias   string
}

type importSpec struct {
	Alias string
	Path  string
}

// buildTemplateData creates the data structure for template rendering.
func (g *TestClientGenerator) buildTemplateData(endpoints []EndpointInfo) templateData {
	data := templateData{
		PackageName:  g.PackageName,
		Endpoints:    endpoints,
		TypesPackage: g.TypesPackage,
		TypesAlias:   g.TypesAlias,
	}

	// Collect required imports
	imports := make(map[string]string) // path -> alias

	// Always needed
	imports["context"] = ""
	imports["net/http"] = ""
	imports["net/url"] = ""
	imports["github.com/shipq/shipq/api/portapi"] = ""

	// Always need io for the do() method signature
	imports["io"] = ""

	// Add types package if needed
	if g.TypesPackage != "" {
		imports[g.TypesPackage] = g.TypesAlias
	}

	// Convert to sorted slice
	var paths []string
	for path := range imports {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	for _, path := range paths {
		data.Imports = append(data.Imports, importSpec{
			Alias: imports[path],
			Path:  path,
		})
	}

	return data
}

// needsStrconvForType returns true if the type needs strconv for stringification.
func needsStrconvForType(t reflect.Type) bool {
	// Handle pointers
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	// Handle slices
	if t.Kind() == reflect.Slice {
		t = t.Elem()
	}

	switch t.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	}
	return false
}

// testClientTemplate is the main template for generating test client code.
var testClientTemplate = template.Must(template.New("testclient").Funcs(template.FuncMap{
	"lower":              strings.ToLower,
	"bindingCode":        generateBindingCode,
	"responseDecodeCode": generateResponseDecodeCode,
	"formatValue":        formatValueCode,
	"isPointer":          isPointerType,
	"isSlice":            isSliceType,
	"elemType":           elemTypeName,
	"baseType":           baseTypeName,
}).Parse(`// Code generated by portapi testclient generator. DO NOT EDIT.
// This file is test-only and will not be included in production builds.

package {{.PackageName}}

import (
{{- range .Imports}}
	{{if .Alias}}{{.Alias}} {{end}}"{{.Path}}"
{{- end}}
)

// Client is a test client for making HTTP requests to the API.
type Client struct {
	// BaseURL is the base URL for API requests (e.g., "http://localhost:8080").
	BaseURL string

	// HTTP is the HTTP client to use. If nil, http.DefaultClient is used.
	HTTP *http.Client
}

// NewClient creates a new test client with the given base URL.
func NewClient(baseURL string) *Client {
	return &Client{
		BaseURL: baseURL,
		HTTP:    http.DefaultClient,
	}
}

// httpClient returns the HTTP client to use, defaulting to http.DefaultClient.
func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return http.DefaultClient
}

// do executes an HTTP request and handles the response.
// For 2xx responses, it decodes the body into out (if out is not nil).
// For non-2xx responses, it returns a portapi.HTTPError (satisfying portapi.CodedError).
func (c *Client) do(ctx context.Context, method, path string, query url.Values, headers http.Header, body io.Reader, out any) error {
	// Build URL
	u := c.BaseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return err
	}

	// Set headers
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, vs := range headers {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}

	// Execute
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Read body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// Handle non-2xx
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return portapi.DecodeErrorEnvelope(resp.StatusCode, resp.Header, respBody)
	}

	// Decode success response
	if out != nil && len(respBody) > 0 {
		if err := portapi.DecodeJSONInto(respBody, out); err != nil {
			return err
		}
	}

	return nil
}

{{range .Endpoints}}
{{if .HasRequest}}
// {{.MethodName}} calls {{.HTTPMethod}} {{.Path}}
func (c *Client) {{.MethodName}}(ctx context.Context, req {{.RequestTypeName}}) ({{if .HasResponse}}{{.ResponseTypeName}}, {{end}}error) {
	{{- if .HasResponse}}
	var out {{.ResponseTypeName}}
	{{- end}}

	// Build path
	{{- if .PathVars}}
	pathVars := map[string]string{
		{{- range .Bindings.PathBindings}}
		"{{.TagValue}}": {{formatValue . "req."}},
		{{- end}}
	}
	path, pathErr := portapi.InterpolatePath("{{.Path}}", pathVars)
	if pathErr != nil {
		return {{if .HasResponse}}out, {{end}}pathErr
	}
	{{- else}}
	path := "{{.Path}}"
	{{- end}}

	// Build query
	query := url.Values{}
	{{- range .Bindings.QueryBindings}}
	{{bindingCode . "query" "req."}}
	{{- end}}

	// Build headers
	headers := make(http.Header)
	{{- range .Bindings.HeaderBindings}}
	{{bindingCode . "headers" "req."}}
	{{- end}}

	// Build body
	{{- if .Bindings.HasJSONBody}}
	bodyReader, encErr := portapi.EncodeJSON(req)
	if encErr != nil {
		return {{if .HasResponse}}out, {{end}}encErr
	}
	{{- else}}
	var bodyReader io.Reader
	{{- end}}

	// Execute request
	{{- if .HasResponse}}
	doErr := c.do(ctx, "{{.HTTPMethod}}", path, query, headers, bodyReader, &out)
	return out, doErr
	{{- else}}
	doErr := c.do(ctx, "{{.HTTPMethod}}", path, query, headers, bodyReader, nil)
	return doErr
	{{- end}}
}
{{else}}
// {{.MethodName}} calls {{.HTTPMethod}} {{.Path}}
func (c *Client) {{.MethodName}}(ctx context.Context) ({{if .HasResponse}}{{.ResponseTypeName}}, {{end}}error) {
	{{- if .HasResponse}}
	var out {{.ResponseTypeName}}
	{{- end}}

	path := "{{.Path}}"
	query := url.Values{}
	headers := make(http.Header)
	var bodyReader io.Reader

	// Execute request
	{{- if .HasResponse}}
	err := c.do(ctx, "{{.HTTPMethod}}", path, query, headers, bodyReader, &out)
	return out, err
	{{- else}}
	err := c.do(ctx, "{{.HTTPMethod}}", path, query, headers, bodyReader, nil)
	return err
	{{- end}}
}
{{end}}
{{end}}
`))

// generateBindingCode generates code for adding a binding to query or headers.
func generateBindingCode(b portapi.FieldBinding, target, prefix string) string {
	fieldRef := prefix + b.FieldName
	tagValue := b.TagValue
	t := b.FieldType

	var buf strings.Builder

	isPtr := t.Kind() == reflect.Ptr
	isSlice := t.Kind() == reflect.Slice

	if isPtr {
		buf.WriteString(fmt.Sprintf("if %s != nil {\n", fieldRef))
		fieldRef = "*" + fieldRef
		t = t.Elem()
	}

	if isSlice {
		elemT := t.Elem()
		buf.WriteString(fmt.Sprintf("for _, v := range %s {\n", fieldRef))
		if target == "query" {
			buf.WriteString(fmt.Sprintf("\tportapi.AddQuery(%s, %q, %s)\n", target, tagValue, formatValueForType("v", elemT)))
		} else {
			buf.WriteString(fmt.Sprintf("\t%s.Add(%q, %s)\n", target, tagValue, formatValueForType("v", elemT)))
		}
		buf.WriteString("}\n")
	} else {
		if target == "query" {
			buf.WriteString(fmt.Sprintf("portapi.AddQuery(%s, %q, %s)\n", target, tagValue, formatValueForType(fieldRef, t)))
		} else {
			buf.WriteString(fmt.Sprintf("portapi.SetHeader(&http.Request{Header: %s}, %q, %s)\n", target, tagValue, formatValueForType(fieldRef, t)))
			// Actually, we need to use headers.Set directly
			buf.Reset()
			if isPtr {
				buf.WriteString(fmt.Sprintf("if %s%s != nil {\n", prefix, b.FieldName))
				buf.WriteString(fmt.Sprintf("\t%s.Set(%q, %s)\n", target, tagValue, formatValueForType("*"+prefix+b.FieldName, t)))
				buf.WriteString("}\n")
				return buf.String()
			}
			buf.WriteString(fmt.Sprintf("%s.Set(%q, %s)\n", target, tagValue, formatValueForType(fieldRef, t)))
		}
	}

	if isPtr && !isSlice {
		buf.WriteString("}\n")
	}

	return buf.String()
}

// formatValueForType returns the code to format a value of the given type as a string.
func formatValueForType(varName string, t reflect.Type) string {
	switch t.Kind() {
	case reflect.String:
		return varName
	case reflect.Bool:
		return fmt.Sprintf("portapi.FormatBool(%s)", varName)
	case reflect.Int:
		return fmt.Sprintf("portapi.FormatInt(%s)", varName)
	case reflect.Int64:
		return fmt.Sprintf("portapi.FormatInt64(%s)", varName)
	case reflect.Int8, reflect.Int16, reflect.Int32:
		return fmt.Sprintf("portapi.FormatInt64(int64(%s))", varName)
	case reflect.Uint:
		return fmt.Sprintf("portapi.FormatUint(%s)", varName)
	case reflect.Uint64:
		return fmt.Sprintf("portapi.FormatUint64(%s)", varName)
	case reflect.Uint8, reflect.Uint16, reflect.Uint32:
		return fmt.Sprintf("portapi.FormatUint64(uint64(%s))", varName)
	case reflect.Float32:
		return fmt.Sprintf("portapi.FormatFloat32(%s)", varName)
	case reflect.Float64:
		return fmt.Sprintf("portapi.FormatFloat64(%s)", varName)
	default:
		if t.String() == "time.Time" {
			return fmt.Sprintf("portapi.FormatTime(%s)", varName)
		}
		return fmt.Sprintf("fmt.Sprint(%s)", varName)
	}
}

// formatValueCode generates code to format a field value as a string (for path vars).
func formatValueCode(b portapi.FieldBinding, prefix string) string {
	return formatValueForType(prefix+b.FieldName, b.FieldType)
}

// generateResponseDecodeCode generates the response decoding code.
func generateResponseDecodeCode(ep EndpointInfo) string {
	if !ep.HasResponse {
		return "return nil"
	}
	return "return out, err"
}

// isPointerType returns true if the type is a pointer.
func isPointerType(t reflect.Type) bool {
	return t.Kind() == reflect.Ptr
}

// isSliceType returns true if the type is a slice.
func isSliceType(t reflect.Type) bool {
	return t.Kind() == reflect.Slice
}

// elemTypeName returns the element type name for pointer/slice types.
func elemTypeName(t reflect.Type) string {
	if t.Kind() == reflect.Ptr || t.Kind() == reflect.Slice {
		return t.Elem().Name()
	}
	return t.Name()
}

// baseTypeName returns the base type name (unwrapping pointers and slices).
func baseTypeName(t reflect.Type) string {
	for t.Kind() == reflect.Ptr || t.Kind() == reflect.Slice {
		t = t.Elem()
	}
	return t.Name()
}
