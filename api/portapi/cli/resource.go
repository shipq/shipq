// Package cli provides the command-line interface for the ShipQ API HTTP generator.
package cli

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"go/format"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/shipq/shipq/config"
	"github.com/shipq/shipq/db/portsql/ddl"
	"github.com/shipq/shipq/db/portsql/migrate"
)

//go:embed assets/resource.go.tmpl
var resourceTemplateText string

// ResourceOptions configures resource generation.
type ResourceOptions struct {
	TableName       string // The database table name (plural, snake_case)
	Prefix          string // URL prefix for endpoints (e.g., "/api/v1")
	OutDir          string // Output directory for generated code
	DBRunnerImport  string // Import path for DB runner package (e.g., "myapp/db/generated")
	DBRunnerPackage string // Directory where runner.go is generated (for deriving import)
	QueriesImport   string // Import path for queries package (e.g., "myapp/queries")
	QueriesOut      string // Directory where queries are generated (for deriving import)
}

// GenerateResource generates REST API endpoints for a database table.
func GenerateResource(opts ResourceOptions, stdout, stderr io.Writer) error {
	// 1. Load the schema from migrations/schema.json
	plan, err := loadMigrationSchema()
	if err != nil {
		return err
	}

	// 2. Find the table in the schema
	table, ok := plan.Schema.Tables[opts.TableName]
	if !ok {
		return fmt.Errorf("table %q not found in migrations/schema.json\n"+
			"  Hint: Run 'shipq db migrate up' first to generate the schema", opts.TableName)
	}

	// 3. Validate that the table is eligible for resource generation
	if !migrate.IsEligibleForResource(table) {
		return fmt.Errorf("table %q is not eligible for resource generation\n"+
			"  Resources can only be generated for tables created with plan.AddTable()\n"+
			"  (must include public_id and deleted_at columns).\n\n"+
			"  If you need a simple table without these columns, use plan.AddEmptyTable()\n"+
			"  but note that it cannot be used with 'shipq api resource'.", opts.TableName)
	}

	// 4. Analyze the table structure
	analysis := analyzeResourceTable(table)

	// 5. Generate the resource code
	code, err := generateResourceCode(opts, table, analysis)
	if err != nil {
		return fmt.Errorf("failed to generate resource code: %w", err)
	}

	// 6. Create output directory
	outputDir := filepath.Join(opts.OutDir, opts.TableName)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// 7. Write the handlers file
	handlersPath := filepath.Join(outputDir, "handlers.go")
	if err := os.WriteFile(handlersPath, code, 0644); err != nil {
		return fmt.Errorf("failed to write handlers.go: %w", err)
	}

	fmt.Fprintf(stdout, "Generated: %s\n", handlersPath)

	// 8. Generate/update the API root register file
	if err := generateOrUpdateAPIRootRegister(opts, stdout); err != nil {
		// Log warning but don't fail - the resource was still generated
		fmt.Fprintf(stderr, "Warning: could not update API root register: %v\n", err)
	}

	fmt.Fprintf(stdout, "\nEndpoints generated for %s:\n", opts.TableName)
	fmt.Fprintf(stdout, "  GET    %s/{public_id}  -> Get%s\n", pathWithPrefix(opts.Prefix, opts.TableName), analysis.SingularPascal)
	fmt.Fprintf(stdout, "  GET    %s              -> List%s\n", pathWithPrefix(opts.Prefix, opts.TableName), analysis.PluralPascal)
	fmt.Fprintf(stdout, "  POST   %s              -> Create%s\n", pathWithPrefix(opts.Prefix, opts.TableName), analysis.SingularPascal)
	fmt.Fprintf(stdout, "  PUT    %s/{public_id}  -> Update%s\n", pathWithPrefix(opts.Prefix, opts.TableName), analysis.SingularPascal)
	fmt.Fprintf(stdout, "  DELETE %s/{public_id}  -> Delete%s\n", pathWithPrefix(opts.Prefix, opts.TableName), analysis.SingularPascal)

	// Determine if DB integration is enabled
	hasDBIntegration := opts.QueriesImport != "" || opts.QueriesOut != ""

	if hasDBIntegration {
		fmt.Fprintf(stdout, "\nUsage:\n")
		fmt.Fprintf(stdout, "  Register with DB in your app:\n")
		fmt.Fprintf(stdout, "    %s.RegisterWithDB(app, db)\n", opts.TableName)
		fmt.Fprintf(stdout, "  Run migrations at startup:\n")
		fmt.Fprintf(stdout, "    generated.Run(ctx, db, dialect)\n")
	} else {
		fmt.Fprintf(stdout, "\nUsage:\n")
		fmt.Fprintf(stdout, "  Register in your app:\n")
		fmt.Fprintf(stdout, "    %s.Register(app)\n", opts.TableName)
		fmt.Fprintf(stdout, "\n")
		fmt.Fprintf(stdout, "  Tip: For DB-backed handlers, configure queries_out in shipq.ini:\n")
		fmt.Fprintf(stdout, "    [db]\n")
		fmt.Fprintf(stdout, "    queries_out = queries\n")
		fmt.Fprintf(stdout, "  Then run: shipq db compile && shipq api resource %s\n", opts.TableName)
	}

	return nil
}

// generateOrUpdateAPIRootRegister generates or updates the zz_generated_register.go file
// in the API root package to include all resource registrations.
func generateOrUpdateAPIRootRegister(opts ResourceOptions, stdout io.Writer) error {
	// Load shipq config to find the API root package
	shipqCfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	apiPkg := shipqCfg.API.Package
	if apiPkg == "" {
		return fmt.Errorf("no API package configured in shipq.ini")
	}

	// Normalize the path (remove ./ prefix if present)
	apiDir := strings.TrimPrefix(apiPkg, "./")

	// Ensure the API directory exists
	if err := os.MkdirAll(apiDir, 0755); err != nil {
		return fmt.Errorf("failed to create API directory: %w", err)
	}

	// Check if the API directory has any non-generated Go files
	// If not, create a minimal api.go to bootstrap the package
	if err := ensureAPIDirectoryAndPackage(apiDir, stdout); err != nil {
		return fmt.Errorf("failed to ensure API package exists: %w", err)
	}

	// Find all resource packages in the output directory
	resourcePackages, err := findResourcePackages(opts.OutDir)
	if err != nil {
		return fmt.Errorf("failed to find resource packages: %w", err)
	}

	// Get the module path for imports
	modulePath, err := getModulePathForResource()
	if err != nil {
		return fmt.Errorf("failed to get module path: %w", err)
	}

	// Determine the package name from existing files in the API directory
	pkgName, err := detectPackageName(apiDir)
	if err != nil {
		// Fall back to directory name
		pkgName = filepath.Base(apiDir)
	}

	// Generate the register file content
	registerCode, err := generateAPIRootRegisterCode(pkgName, modulePath, opts.OutDir, resourcePackages)
	if err != nil {
		return fmt.Errorf("failed to generate register code: %w", err)
	}

	// Write the register file
	registerPath := filepath.Join(apiDir, "zz_generated_register.go")
	if err := os.WriteFile(registerPath, registerCode, 0644); err != nil {
		return fmt.Errorf("failed to write register file: %w", err)
	}

	fmt.Fprintf(stdout, "Generated: %s\n", registerPath)
	return nil
}

// detectPackageName reads existing .go files in a directory to determine the package name.
func detectPackageName(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		// Skip generated files
		if strings.HasPrefix(entry.Name(), "zz_generated") {
			continue
		}

		// Read the file and find the package declaration
		filePath := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "package ") {
				pkgName := strings.TrimPrefix(line, "package ")
				// Remove any trailing comments
				if idx := strings.Index(pkgName, "//"); idx != -1 {
					pkgName = strings.TrimSpace(pkgName[:idx])
				}
				return pkgName, nil
			}
		}
	}

	return "", fmt.Errorf("no package declaration found in %s", dir)
}

// findResourcePackages finds all resource package directories in the output directory.
func findResourcePackages(outDir string) ([]string, error) {
	var packages []string

	entries, err := os.ReadDir(outDir)
	if err != nil {
		if os.IsNotExist(err) {
			return packages, nil
		}
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			// Check if it has a handlers.go file
			handlersPath := filepath.Join(outDir, entry.Name(), "handlers.go")
			if _, err := os.Stat(handlersPath); err == nil {
				packages = append(packages, entry.Name())
			}
		}
	}

	sort.Strings(packages)
	return packages, nil
}

// getModulePathForResource gets the module path by searching up the directory tree for go.mod.
// It also returns the module root directory.
func getModulePathForResource() (string, error) {
	// Start from current directory and walk up
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}

	for {
		goModPath := filepath.Join(dir, "go.mod")
		data, err := os.ReadFile(goModPath)
		if err == nil {
			// Found go.mod, parse it
			lines := strings.Split(string(data), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "module ") {
					modulePath := strings.TrimPrefix(line, "module ")
					// Calculate the relative path from module root to current dir
					cwd, _ := os.Getwd()
					relPath, err := filepath.Rel(dir, cwd)
					if err != nil {
						return modulePath, nil
					}
					if relPath != "." {
						// We're in a subdirectory, append it to the module path
						return modulePath + "/" + filepath.ToSlash(relPath), nil
					}
					return modulePath, nil
				}
			}
			return "", fmt.Errorf("could not find module declaration in go.mod")
		}

		// Move up one directory
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root without finding go.mod
			return "", fmt.Errorf("could not find go.mod in current directory or any parent")
		}
		dir = parent
	}
}

// generateAPIRootRegisterCode generates the content for zz_generated_register.go.
func generateAPIRootRegisterCode(pkgName, modulePath, resourcesDir string, resourcePackages []string) ([]byte, error) {
	data := struct {
		PackageName      string
		ModulePath       string
		ResourcesDir     string
		ResourcePackages []string
	}{
		PackageName:      pkgName,
		ModulePath:       modulePath,
		ResourcesDir:     resourcesDir,
		ResourcePackages: resourcePackages,
	}

	var buf bytes.Buffer
	if err := apiRootRegisterTemplate.Execute(&buf, data); err != nil {
		return nil, err
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return buf.Bytes(), fmt.Errorf("failed to format generated code: %w", err)
	}

	return formatted, nil
}

// apiRootRegisterTemplate is the template for the API root register file.
var apiRootRegisterTemplate = template.Must(template.New("apiRootRegister").Parse(`// Code generated by shipq api resource. DO NOT EDIT.

package {{.PackageName}}

import (
	"github.com/shipq/shipq/api/portapi"
{{- range .ResourcePackages}}
	"{{$.ModulePath}}/{{$.ResourcesDir}}/{{.}}"
{{- end}}
)

// Register registers all resource endpoints with the portapi App.
func Register(app *portapi.App) {
{{- range .ResourcePackages}}
	{{.}}.Register(app)
{{- end}}
}
`))

func pathWithPrefix(prefix, tableName string) string {
	if prefix == "" {
		return "/" + tableName
	}
	return strings.TrimSuffix(prefix, "/") + "/" + tableName
}

// loadMigrationSchema loads the schema from migrations/schema.json.
func loadMigrationSchema() (*migrate.MigrationPlan, error) {
	schemaPath := filepath.Join("migrations", "schema.json")

	data, err := os.ReadFile(schemaPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("schema.json not found at %s\n"+
				"  Hint: Run 'shipq db migrate up' first to generate the schema", schemaPath)
		}
		return nil, fmt.Errorf("failed to read schema.json: %w", err)
	}

	var plan migrate.MigrationPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, fmt.Errorf("failed to parse schema.json: %w", err)
	}

	return &plan, nil
}

// ResourceTableAnalysis contains analyzed information about a table for resource generation.
type ResourceTableAnalysis struct {
	TableName      string
	SingularName   string // e.g., "user"
	PluralName     string // e.g., "users"
	SingularPascal string // e.g., "User"
	PluralPascal   string // e.g., "Users"

	// Columns for the resource (excluding auto-managed columns)
	UserColumns []ddl.ColumnDefinition

	// All result columns (for Get response)
	ResultColumns []ddl.ColumnDefinition

	// Whether table has cursor pagination support
	SupportsCursor bool
}

// analyzeResourceTable analyzes a table for resource generation.
func analyzeResourceTable(table ddl.Table) ResourceTableAnalysis {
	singular := toSingular(table.Name)
	analysis := ResourceTableAnalysis{
		TableName:      table.Name,
		SingularName:   singular,
		PluralName:     table.Name,
		SingularPascal: toPascalCase(singular),
		PluralPascal:   toPascalCase(table.Name),
	}

	hasCreatedAt := false
	hasPublicID := false

	for _, col := range table.Columns {
		switch col.Name {
		case "created_at":
			hasCreatedAt = true
		case "public_id":
			hasPublicID = true
		}

		// User columns: exclude auto-managed columns
		if col.Name != "id" && col.Name != "public_id" &&
			col.Name != "created_at" && col.Name != "updated_at" &&
			col.Name != "deleted_at" {
			analysis.UserColumns = append(analysis.UserColumns, col)
		}

		// Result columns: exclude internal id and deleted_at
		if col.Name != "id" && col.Name != "deleted_at" {
			analysis.ResultColumns = append(analysis.ResultColumns, col)
		}
	}

	analysis.SupportsCursor = hasCreatedAt && hasPublicID

	return analysis
}

// toSingular converts a plural table name to singular.
func toSingular(s string) string {
	if strings.HasSuffix(s, "ies") {
		return s[:len(s)-3] + "y"
	}
	if strings.HasSuffix(s, "es") {
		return s[:len(s)-2]
	}
	if strings.HasSuffix(s, "s") {
		return s[:len(s)-1]
	}
	return s
}

// toPascalCase converts a snake_case string to PascalCase.
func toPascalCase(s string) string {
	parts := strings.Split(s, "_")
	var result []string
	for _, part := range parts {
		if len(part) > 0 {
			result = append(result, strings.ToUpper(part[:1])+part[1:])
		}
	}
	joined := strings.Join(result, "")
	if len(joined) > 0 && joined[0] >= '0' && joined[0] <= '9' {
		joined = "X" + joined
	}
	if joined == "" {
		return "X"
	}
	return joined
}

// ResourceTemplateData contains data for the resource templates.
type ResourceTemplateData struct {
	PackageName    string
	TableName      string
	SingularPascal string
	PluralPascal   string
	Prefix         string
	BasePath       string

	// Columns
	UserColumns   []ResourceColumn
	ResultColumns []ResourceColumn

	// Imports needed
	Imports []string

	// Pagination
	SupportsCursor bool

	// DB integration
	HasDBIntegration bool   // Whether DB runner is configured
	QueriesImport    string // Import path for queries types package (e.g., "myapp/queries")
	QueriesPkg       string // Package name for queries types (e.g., "queries")
	RunnerImport     string // Import path for dialect runner (e.g., "myapp/queries/sqlite")
	RunnerPkg        string // Package name for runner (e.g., "sqlite")
	DBGenImport      string // Import path for generated db package (e.g., "myapp/db/generated")
	DBGenPkg         string // Package name for generated db (e.g., "generated")

	// For template use
	SingularName string // e.g., "user"
}

// ResourceColumn represents a column for template generation.
type ResourceColumn struct {
	Name       string // snake_case
	FieldName  string // PascalCase
	GoType     string
	JSONTag    string
	IsNullable bool
}

// generateResourceCode generates the Go code for a resource.
func generateResourceCode(opts ResourceOptions, table ddl.Table, analysis ResourceTableAnalysis) ([]byte, error) {
	// Prepare template data
	data := ResourceTemplateData{
		PackageName:    table.Name,
		TableName:      table.Name,
		SingularPascal: analysis.SingularPascal,
		PluralPascal:   analysis.PluralPascal,
		Prefix:         opts.Prefix,
		BasePath:       pathWithPrefix(opts.Prefix, table.Name),
		SupportsCursor: analysis.SupportsCursor,
		SingularName:   analysis.SingularName,
	}

	// Determine if we have DB integration configured
	// Always attempt DB integration by deriving queries import from module path
	queriesImport := opts.QueriesImport
	if queriesImport == "" {
		modulePath, err := getModulePathForResource()
		if err == nil {
			if opts.QueriesOut != "" {
				// Use configured queries_out directory
				queriesOut := strings.TrimPrefix(opts.QueriesOut, "./")
				queriesImport = modulePath + "/" + queriesOut
			} else {
				// Default to "queries" directory
				queriesImport = modulePath + "/queries"
			}
		}
	}

	if queriesImport != "" {
		data.HasDBIntegration = true
		data.QueriesImport = queriesImport
		data.QueriesPkg = filepath.Base(strings.TrimSuffix(queriesImport, "/"))

		// Determine the dialect runner package
		// Default to sqlite if not specified; in future could read from config
		dialect := "sqlite"
		if opts.DBRunnerPackage != "" {
			dialect = opts.DBRunnerPackage
		}
		data.RunnerImport = queriesImport + "/" + dialect
		data.RunnerPkg = dialect

		// Determine the generated db package import
		// Default to db/generated if not specified
		dbGenDir := "db/generated"
		if opts.DBRunnerImport != "" {
			// DBRunnerImport may contain the full import path already
			data.DBGenImport = opts.DBRunnerImport
		} else {
			// Derive from module path
			modulePath, err := getModulePathForResource()
			if err == nil {
				data.DBGenImport = modulePath + "/" + dbGenDir
			}
		}
		if data.DBGenImport != "" {
			data.DBGenPkg = filepath.Base(data.DBGenImport)
		}
	}

	// Collect imports
	imports := make(map[string]bool)
	imports["context"] = true
	imports["github.com/shipq/shipq/api/portapi"] = true

	// Add DB-related imports if we have integration
	if data.HasDBIntegration {
		imports[data.QueriesImport] = true
		if data.DBGenImport != "" {
			imports[data.DBGenImport] = true
		}
	}

	// Convert columns
	for _, col := range analysis.UserColumns {
		rc := columnToResourceColumn(col)
		data.UserColumns = append(data.UserColumns, rc)
		if imp := goTypeImport(rc.GoType); imp != "" {
			imports[imp] = true
		}
	}

	for _, col := range analysis.ResultColumns {
		rc := columnToResourceColumn(col)
		data.ResultColumns = append(data.ResultColumns, rc)
		if imp := goTypeImport(rc.GoType); imp != "" {
			imports[imp] = true
		}
	}

	// Sort imports
	importList := make([]string, 0, len(imports))
	for imp := range imports {
		importList = append(importList, imp)
	}
	sort.Strings(importList)
	data.Imports = importList

	// Execute template
	var buf bytes.Buffer
	if err := resourceTemplate.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("template execution failed: %w", err)
	}

	// Format the code
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		// Return unformatted code with error for debugging
		return buf.Bytes(), fmt.Errorf("failed to format generated code: %w\n\nGenerated code:\n%s", err, buf.String())
	}

	return formatted, nil
}

// columnToResourceColumn converts a DDL column to a ResourceColumn.
func columnToResourceColumn(col ddl.ColumnDefinition) ResourceColumn {
	return ResourceColumn{
		Name:       col.Name,
		FieldName:  toPascalCase(col.Name),
		GoType:     mapColumnGoType(col),
		JSONTag:    col.Name,
		IsNullable: col.Nullable,
	}
}

// mapColumnGoType maps a DDL column to a Go type.
func mapColumnGoType(col ddl.ColumnDefinition) string {
	switch col.Type {
	case ddl.IntegerType:
		if col.Nullable {
			return "*int32"
		}
		return "int32"
	case ddl.BigintType:
		if col.Nullable {
			return "*int64"
		}
		return "int64"
	case ddl.DecimalType:
		if col.Nullable {
			return "*string"
		}
		return "string"
	case ddl.FloatType:
		if col.Nullable {
			return "*float64"
		}
		return "float64"
	case ddl.BooleanType:
		if col.Nullable {
			return "*bool"
		}
		return "bool"
	case ddl.StringType, ddl.TextType:
		if col.Nullable {
			return "*string"
		}
		return "string"
	case ddl.DatetimeType, ddl.TimestampType:
		if col.Nullable {
			return "*time.Time"
		}
		return "time.Time"
	case ddl.BinaryType:
		return "[]byte"
	case ddl.JSONType:
		if col.Nullable {
			return "json.RawMessage"
		}
		return "json.RawMessage"
	default:
		if col.Nullable {
			return "*string"
		}
		return "string"
	}
}

// goTypeImport returns the import path needed for a Go type.
func goTypeImport(goType string) string {
	if strings.Contains(goType, "time.Time") {
		return "time"
	}
	if strings.Contains(goType, "json.RawMessage") {
		return "encoding/json"
	}
	return ""
}

// resourceTemplate is the parsed template for generating resource handlers.
// It is initialized in init() using the embedded template text.
var resourceTemplate *template.Template

func init() {
	resourceTemplate = template.Must(template.New("resource").Parse(resourceTemplateText))
}
