package cli

import (
	"bytes"
	"fmt"
	"go/format"
	"sort"
	"strings"
	"text/template"
)

// TestClientGeneratorFromManifest generates test client code from manifest data.
// This is different from gen.TestClientGenerator which uses reflection.
type TestClientGeneratorFromManifest struct {
	// PackageName is the Go package name for the generated file.
	PackageName string

	// Endpoints from the manifest.
	Endpoints []ManifestEndpoint
}

// Generate produces the test client Go source code.
func (g *TestClientGeneratorFromManifest) Generate() ([]byte, error) {
	if g.PackageName == "" {
		return nil, fmt.Errorf("PackageName is required")
	}

	// Sort endpoints for deterministic output
	sortedEndpoints := make([]ManifestEndpoint, len(g.Endpoints))
	copy(sortedEndpoints, g.Endpoints)
	sort.Slice(sortedEndpoints, func(i, j int) bool {
		if sortedEndpoints[i].Method != sortedEndpoints[j].Method {
			return sortedEndpoints[i].Method < sortedEndpoints[j].Method
		}
		return sortedEndpoints[i].Path < sortedEndpoints[j].Path
	})

	// Analyze endpoints
	infos := make([]manifestEndpointInfo, 0, len(sortedEndpoints))
	for _, ep := range sortedEndpoints {
		info := g.analyzeEndpoint(ep)
		infos = append(infos, info)
	}

	// Build template data
	data := g.buildTemplateData(infos)

	// Execute template
	var buf bytes.Buffer
	if err := manifestTestClientTemplate.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("execute template: %w", err)
	}

	// Format the output
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return buf.Bytes(), fmt.Errorf("format source: %w\n\nUnformatted source:\n%s", err, buf.String())
	}

	return formatted, nil
}

// manifestEndpointInfo contains analyzed information about an endpoint for code generation.
type manifestEndpointInfo struct {
	MethodName       string
	HTTPMethod       string
	Path             string
	HasRequest       bool
	HasResponse      bool
	RequestTypeName  string
	ResponseTypeName string
	Bindings         *BindingInfo
	PathVars         []string
}

// analyzeEndpoint extracts information from a manifest endpoint.
func (g *TestClientGeneratorFromManifest) analyzeEndpoint(ep ManifestEndpoint) manifestEndpointInfo {
	info := manifestEndpointInfo{
		HTTPMethod: ep.Method,
		Path:       ep.Path,
		Bindings:   ep.Bindings,
	}

	// Derive method name from handler name
	info.MethodName = ep.HandlerName

	// Determine request/response types from shape
	switch ep.Shape {
	case "ctx_req_resp_err":
		info.HasRequest = true
		info.HasResponse = true
	case "ctx_req_err":
		info.HasRequest = true
		info.HasResponse = false
	case "ctx_resp_err":
		info.HasRequest = false
		info.HasResponse = true
	case "ctx_err":
		info.HasRequest = false
		info.HasResponse = false
	}

	// Get type names
	if ep.ReqType != "" {
		info.RequestTypeName = extractTypeName(ep.ReqType)
	}
	if ep.RespType != "" {
		info.ResponseTypeName = extractTypeName(ep.RespType)
	}

	// Extract path variables
	info.PathVars = extractPathVars(ep.Path)

	return info
}

// extractTypeName extracts the short type name from a fully qualified type.
// e.g., "github.com/example/api.CreatePetReq" -> "CreatePetReq"
func extractTypeName(fullType string) string {
	// Handle pointer types
	if strings.HasPrefix(fullType, "*") {
		return "*" + extractTypeName(fullType[1:])
	}

	// Handle slice types
	if strings.HasPrefix(fullType, "[]") {
		return "[]" + extractTypeName(fullType[2:])
	}

	// Find the last dot
	lastDot := strings.LastIndex(fullType, ".")
	if lastDot == -1 {
		return fullType
	}
	return fullType[lastDot+1:]
}

// extractPathVars extracts path variable names from a path pattern.
// e.g., "/pets/{id}" -> ["id"]
func extractPathVars(path string) []string {
	var vars []string
	start := 0
	for {
		openIdx := strings.Index(path[start:], "{")
		if openIdx == -1 {
			break
		}
		openIdx += start

		closeIdx := strings.Index(path[openIdx:], "}")
		if closeIdx == -1 {
			break
		}
		closeIdx += openIdx

		varName := path[openIdx+1 : closeIdx]
		varName = strings.TrimSuffix(varName, "...")
		vars = append(vars, varName)

		start = closeIdx + 1
	}
	return vars
}

// manifestTemplateData contains all data needed to render the template.
type manifestTemplateData struct {
	PackageName string
	Imports     []manifestImportSpec
	Endpoints   []manifestEndpointInfo
}

type manifestImportSpec struct {
	Alias string
	Path  string
}

// buildTemplateData creates the data structure for template rendering.
func (g *TestClientGeneratorFromManifest) buildTemplateData(endpoints []manifestEndpointInfo) manifestTemplateData {
	data := manifestTemplateData{
		PackageName: g.PackageName,
		Endpoints:   endpoints,
	}

	// Collect required imports
	imports := make(map[string]string)
	imports["context"] = ""
	imports["net/http"] = ""
	imports["net/url"] = ""
	imports["github.com/shipq/shipq/api/portapi"] = ""
	imports["io"] = ""

	// Convert to sorted slice
	var paths []string
	for path := range imports {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	for _, path := range paths {
		data.Imports = append(data.Imports, manifestImportSpec{
			Alias: imports[path],
			Path:  path,
		})
	}

	return data
}

// Template functions for the manifest-based generator
var manifestTemplateFuncs = template.FuncMap{
	"lower":       strings.ToLower,
	"bindingCode": generateManifestBindingCode,
	"formatValue": formatManifestValueCode,
}

// generateManifestBindingCode generates code for adding a binding to query or headers.
func generateManifestBindingCode(b FieldBinding, target, prefix string) string {
	fieldRef := prefix + b.FieldName
	tagValue := b.TagValue

	var buf strings.Builder

	if b.IsPointer {
		buf.WriteString(fmt.Sprintf("if %s != nil {\n", fieldRef))
		fieldRef = "*" + fieldRef
	}

	if b.IsSlice {
		buf.WriteString(fmt.Sprintf("for _, v := range %s {\n", fieldRef))
		if target == "query" {
			buf.WriteString(fmt.Sprintf("\tportapi.AddQuery(%s, %q, %s)\n", target, tagValue, formatManifestValue("v", b.ElemKind)))
		} else {
			buf.WriteString(fmt.Sprintf("\t%s.Add(%q, %s)\n", target, tagValue, formatManifestValue("v", b.ElemKind)))
		}
		buf.WriteString("}\n")
	} else {
		if target == "query" {
			buf.WriteString(fmt.Sprintf("portapi.AddQuery(%s, %q, %s)\n", target, tagValue, formatManifestValue(fieldRef, b.TypeKind)))
		} else {
			if b.IsPointer {
				buf.WriteString(fmt.Sprintf("\t%s.Set(%q, %s)\n", target, tagValue, formatManifestValue(fieldRef, b.TypeKind)))
			} else {
				buf.WriteString(fmt.Sprintf("%s.Set(%q, %s)\n", target, tagValue, formatManifestValue(fieldRef, b.TypeKind)))
			}
		}
	}

	if b.IsPointer && !b.IsSlice {
		buf.WriteString("}\n")
	}

	return buf.String()
}

// formatManifestValue returns the code to format a value of the given type kind as a string.
func formatManifestValue(varName, typeKind string) string {
	switch typeKind {
	case "string":
		return varName
	case "bool":
		return fmt.Sprintf("portapi.FormatBool(%s)", varName)
	case "int":
		return fmt.Sprintf("portapi.FormatInt(%s)", varName)
	case "int8", "int16", "int32":
		return fmt.Sprintf("portapi.FormatInt64(int64(%s))", varName)
	case "int64":
		return fmt.Sprintf("portapi.FormatInt64(%s)", varName)
	case "uint":
		return fmt.Sprintf("portapi.FormatUint(%s)", varName)
	case "uint8", "uint16", "uint32":
		return fmt.Sprintf("portapi.FormatUint64(uint64(%s))", varName)
	case "uint64":
		return fmt.Sprintf("portapi.FormatUint64(%s)", varName)
	case "float32":
		return fmt.Sprintf("portapi.FormatFloat32(%s)", varName)
	case "float64":
		return fmt.Sprintf("portapi.FormatFloat64(%s)", varName)
	case "time.Time":
		return fmt.Sprintf("portapi.FormatTime(%s)", varName)
	default:
		return fmt.Sprintf("fmt.Sprint(%s)", varName)
	}
}

// formatManifestValueCode generates code to format a field value as a string (for path vars).
func formatManifestValueCode(b FieldBinding, prefix string) string {
	return formatManifestValue(prefix+b.FieldName, b.TypeKind)
}

// manifestTestClientTemplate is the main template for generating test client code from manifest.
var manifestTestClientTemplate = template.Must(template.New("testclient").Funcs(manifestTemplateFuncs).Parse(`// Code generated by portapi testclient generator. DO NOT EDIT.
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
	{{- if .Bindings}}
	{{- range .Bindings.QueryBindings}}
	{{bindingCode . "query" "req."}}
	{{- end}}
	{{- end}}

	// Build headers
	headers := make(http.Header)
	{{- if .Bindings}}
	{{- range .Bindings.HeaderBindings}}
	{{bindingCode . "headers" "req."}}
	{{- end}}
	{{- end}}

	// Build body
	{{- if and .Bindings .Bindings.HasJSONBody}}
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

// TestHarnessGeneratorFromManifest generates test harness helpers from manifest data.
type TestHarnessGeneratorFromManifest struct {
	// PackageName is the Go package name for the generated file.
	PackageName string

	// HasNewMux indicates whether a NewMux() function exists.
	HasNewMux bool
}

// Generate produces the test harness Go source code.
func (g *TestHarnessGeneratorFromManifest) Generate() ([]byte, error) {
	if g.PackageName == "" {
		return nil, fmt.Errorf("PackageName is required")
	}

	// Build template data
	data := struct {
		PackageName string
		Imports     []manifestImportSpec
		HasNewMux   bool
	}{
		PackageName: g.PackageName,
		HasNewMux:   g.HasNewMux,
	}

	// Collect required imports
	imports := []string{"net/http", "net/http/httptest", "testing"}
	sort.Strings(imports)

	for _, path := range imports {
		data.Imports = append(data.Imports, manifestImportSpec{Path: path})
	}

	// Execute template
	var buf bytes.Buffer
	if err := manifestTestHarnessTemplate.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("execute template: %w", err)
	}

	// Format the output
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return buf.Bytes(), fmt.Errorf("format source: %w\n\nUnformatted source:\n%s", err, buf.String())
	}

	return formatted, nil
}

// manifestTestHarnessTemplate is the template for generating test harness code.
var manifestTestHarnessTemplate = template.Must(template.New("testharness").Parse(`// Code generated by portapi testharness generator. DO NOT EDIT.
// This file is test-only and will not be included in production builds.

package {{.PackageName}}

import (
{{- range .Imports}}
	{{if .Alias}}{{.Alias}} {{end}}"{{.Path}}"
{{- end}}
)

// NewTestClient creates a test client configured for the given httptest.Server.
//
// It uses ts.Client() to get an HTTP client properly configured for the test server
// (handling cookies, redirects, and TLS settings), and ts.URL as the base URL.
//
// This function panics if ts is nil, as that indicates a bug in the test setup.
//
// Example usage:
//
//	ts := httptest.NewServer(NewMux())
//	defer ts.Close()
//	client := NewTestClient(ts)
//	resp, err := client.SomeEndpoint(ctx, req)
func NewTestClient(ts *httptest.Server) *Client {
	if ts == nil {
		panic("NewTestClient: nil httptest.Server")
	}
	return &Client{
		BaseURL: ts.URL,
		HTTP:    ts.Client(),
	}
}

{{if .HasNewMux}}
// NewTestServer creates an httptest.Server with the generated API routes.
//
// It uses NewMux() to create the server mux with all routes registered.
// The server is automatically closed when the test completes via t.Cleanup.
//
// Example usage:
//
//	ts := NewTestServer(t)
//	client := NewTestClient(ts)
//	resp, err := client.SomeEndpoint(ctx, req)
func NewTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := NewMux()
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts
}
{{else}}
// NewTestServer creates an httptest.Server from the provided handler.
//
// The server is automatically closed when the test completes via t.Cleanup.
//
// Example usage:
//
//	mux := http.NewServeMux()
//	// ... register your handlers ...
//	ts := NewTestServer(t, mux)
//	client := NewTestClient(ts)
//	resp, err := client.SomeEndpoint(ctx, req)
func NewTestServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	return ts
}
{{end}}
`))
