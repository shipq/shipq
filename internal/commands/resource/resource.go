package resource

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shipq/shipq/codegen"
	"github.com/shipq/shipq/codegen/crud"
	"github.com/shipq/shipq/codegen/crudquerydefs"
	"github.com/shipq/shipq/codegen/handlergen"
	"github.com/shipq/shipq/codegen/resourcegen"
	"github.com/shipq/shipq/db/portsql/migrate"
	"github.com/shipq/shipq/dburl"
	"github.com/shipq/shipq/inifile"
	"github.com/shipq/shipq/internal/commands/db"
	"github.com/shipq/shipq/internal/commands/migrate/up"
	"github.com/shipq/shipq/project"
	"github.com/shipq/shipq/registry"
)

// ValidOperations lists the accepted operation names for `shipq resource <table> <op>`.
var ValidOperations = []string{"create", "get_one", "list", "update", "delete", "all"}

// ResourceCmd handles `shipq resource <table> <operation>`.
func ResourceCmd(tableName, operation string, extraArgs []string) {
	isPublic := false
	for _, arg := range extraArgs {
		if arg == "--public" {
			isPublic = true
		}
	}

	if err := generateResource(tableName, operation, isPublic); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func generateResource(tableName, operation string, isPublic bool) error {
	// Find project roots
	roots, err := project.FindProjectRoots()
	if err != nil {
		return fmt.Errorf("failed to find project: %w", err)
	}

	// Step 1: Run migrations
	fmt.Println("Running migrations...")
	up.MigrateUpCmd()

	// Step 2: Read config
	moduleInfo, err := codegen.GetModuleInfo(roots.GoModRoot, roots.ShipqRoot)
	if err != nil {
		return fmt.Errorf("%w\nMake sure you're in a Go project with a go.mod file.", err)
	}
	modulePath := moduleInfo.FullImportPath("")

	requireAuth := false
	if !isPublic {
		shipqIniPath := filepath.Join(roots.ShipqRoot, project.ShipqIniFile)
		ini, iniErr := inifile.ParseFile(shipqIniPath)
		if iniErr == nil {
			protectByDefault := strings.ToLower(ini.Get("auth", "protect_by_default"))
			requireAuth = (protectByDefault == "true")
		}
	}

	if requireAuth {
		fmt.Println("Auth protection enabled (protect_by_default = true)")
	} else if isPublic {
		fmt.Println("Auth protection disabled (--public flag)")
	}

	// Read dialect + test URL from shipq.ini
	dialect := ""
	testDatabaseURL := ""
	shipqIniPath := filepath.Join(roots.ShipqRoot, project.ShipqIniFile)
	if ini, iniErr := inifile.ParseFile(shipqIniPath); iniErr == nil {
		if u := ini.Get("db", "database_url"); u != "" {
			if d, dErr := dburl.InferDialectFromDBUrl(u); dErr == nil {
				dialect = d
			}
			testDatabaseURL, _ = dburl.TestDatabaseURL(u)
		}
	}

	// Load schema
	schemaPath := filepath.Join(roots.ShipqRoot, "shipq", "db", "migrate", "schema.json")
	schemaData, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("failed to read schema.json: %w\nMake sure migrations have been run.", err)
	}

	plan, err := migrate.PlanFromJSON(schemaData)
	if err != nil {
		return fmt.Errorf("failed to parse schema.json: %w", err)
	}

	table, ok := plan.Schema.Tables[tableName]
	if !ok {
		return fmt.Errorf("table %q not found in schema.json.\nAvailable tables: %s",
			tableName, strings.Join(handlergen.SortedTableNames(plan.Schema.Tables), ", "))
	}

	if table.IsJunctionTable {
		return fmt.Errorf("table %q is a junction table and doesn't need handlers", tableName)
	}

	// Load CRUD config for scope settings
	allTableNames := make([]string, 0, len(plan.Schema.Tables))
	for name := range plan.Schema.Tables {
		allTableNames = append(allTableNames, name)
	}

	scopeColumn := ""
	crudCfg, crudErr := crud.LoadCRUDConfigWithTables(roots.ShipqRoot, allTableNames, plan.Schema.Tables)
	if crudErr == nil {
		if opts, ok := crudCfg.TableOpts[tableName]; ok {
			scopeColumn = opts.ScopeColumn
		}
	}

	// Generate CRUD querydefs (DSL code the user can inspect and customise)
	querydefsDir := filepath.Join(roots.ShipqRoot, "querydefs", tableName)
	if err := codegen.EnsureDir(querydefsDir); err != nil {
		return fmt.Errorf("failed to create querydefs directory: %w", err)
	}

	querydefsCfg := crudquerydefs.Config{
		ModulePath:  modulePath,
		TableName:   tableName,
		Table:       table,
		ScopeColumn: scopeColumn,
		Schema:      plan.Schema.Tables,
	}
	querydefsBytes, err := crudquerydefs.GenerateCRUDQueryDefs(querydefsCfg)
	if err != nil {
		return fmt.Errorf("failed to generate CRUD querydefs: %w", err)
	}
	querydefsPath := filepath.Join(querydefsDir, "queries.go")
	querydefsChanged, err := codegen.WriteFileIfChanged(querydefsPath, querydefsBytes)
	if err != nil {
		return fmt.Errorf("failed to write querydefs: %w", err)
	}
	if querydefsChanged {
		fmt.Printf("  Generated querydefs/%s/queries.go\n", tableName)
	}

	// Recompile queries now that CRUD querydefs are in place
	fmt.Println("")
	fmt.Println("Recompiling queries...")
	db.DBCompileCmd()

	// Determine operations to generate
	var ops []handlergen.Operation
	if operation == "all" {
		ops = handlergen.AllOperations()
	} else {
		op := handlergen.Operation(operation)
		ops = []handlergen.Operation{op}
	}

	cfg := handlergen.HandlerGenConfig{
		ModulePath:  modulePath,
		TableName:   tableName,
		Table:       table,
		Schema:      plan.Schema.Tables,
		ScopeColumn: scopeColumn,
		RequireAuth: requireAuth,
	}

	// Create api/<table> directory
	apiDir := filepath.Join(roots.ShipqRoot, "api", tableName)
	if err := codegen.EnsureDir(apiDir); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", apiDir, err)
	}

	// Generate handler files for each operation
	fmt.Println("")
	fmt.Printf("Generating %s handlers for %s...\n", operation, tableName)
	relations := handlergen.AnalyzeRelationships(table, plan.Schema.Tables)

	for _, op := range ops {
		handlerBytes, err := generateSingleHandler(cfg, op, relations)
		if err != nil {
			return fmt.Errorf("failed to generate %s handler: %w", op, err)
		}

		filename := string(op) + ".go"
		filePath := filepath.Join(apiDir, filename)
		changed, err := codegen.WriteFileIfChanged(filePath, handlerBytes)
		if err != nil {
			return fmt.Errorf("failed to write %s: %w", filePath, err)
		}
		if changed {
			fmt.Printf("  Generated %s\n", filename)
		}
	}

	// Generate types.go for shared type declarations (e.g. AuthorEmbed)
	// when the table has author_account_id, so the struct is defined once
	// instead of being redeclared in every handler file.
	if handlergen.TableHasAuthorAccountID(table) {
		typesBytes, err := handlergen.GenerateTypesFile(cfg)
		if err != nil {
			return fmt.Errorf("failed to generate types.go: %w", err)
		}
		typesPath := filepath.Join(apiDir, "types.go")
		changed, err := codegen.WriteFileIfChanged(typesPath, typesBytes)
		if err != nil {
			return fmt.Errorf("failed to write types.go: %w", err)
		}
		if changed {
			fmt.Println("  Generated types.go")
		}
	}

	// Generate/update register.go
	registerPath := filepath.Join(apiDir, "register.go")
	registerBytes, err := handlergen.GenerateIncrementalRegister(registerPath, modulePath, tableName, ops, requireAuth)
	if err != nil {
		return fmt.Errorf("failed to generate register.go: %w", err)
	}
	changed, err := codegen.WriteFileIfChanged(registerPath, registerBytes)
	if err != nil {
		return fmt.Errorf("failed to write register.go: %w", err)
	}
	if changed {
		fmt.Println("  Generated register.go")
	}

	// Write .shipq-no-regen marker
	markerPath := filepath.Join(apiDir, ".shipq-no-regen")
	if _, err := os.Stat(markerPath); os.IsNotExist(err) {
		markerContent := "# This file prevents shipq from regenerating handlers in this directory.\n# Delete this file if you want shipq to regenerate the handlers.\n"
		if err := os.WriteFile(markerPath, []byte(markerContent), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", markerPath, err)
		}
	}

	// Generate fixture package
	fmt.Println("  Generating fixture...")
	fixtureDir := filepath.Join(apiDir, "fixture")
	if err := codegen.EnsureDir(fixtureDir); err != nil {
		return fmt.Errorf("failed to create fixture directory: %w", err)
	}

	fixtureCfg := resourcegen.FixtureGenConfig{
		ModulePath:  modulePath,
		TableName:   tableName,
		Table:       table,
		Schema:      plan.Schema.Tables,
		Dialect:     dialect,
		ScopeColumn: scopeColumn,
	}
	fixtureBytes, err := resourcegen.GenerateFixture(fixtureCfg)
	if err != nil {
		return fmt.Errorf("failed to generate fixture: %w", err)
	}
	fixturePath := filepath.Join(fixtureDir, "fixture.go")
	if _, err := codegen.WriteFileIfChanged(fixturePath, fixtureBytes); err != nil {
		return fmt.Errorf("failed to write fixture: %w", err)
	}

	// Generate per-operation test files
	fmt.Println("  Generating tests...")
	testDir := filepath.Join(roots.ShipqRoot, "api", tableName, "spec")
	if err := codegen.EnsureDir(testDir); err != nil {
		return fmt.Errorf("failed to create test directory: %w", err)
	}

	testCfg := resourcegen.PerOpTestGenConfig{
		ModulePath:      modulePath,
		TableName:       tableName,
		Table:           table,
		Schema:          plan.Schema.Tables,
		RequireAuth:     requireAuth,
		Dialect:         dialect,
		TestDatabaseURL: testDatabaseURL,
		ScopeColumn:     scopeColumn,
	}

	// Generate shared helpers file (parseDatabaseURL, isLocalhostURL)
	helpersBytes, err := resourcegen.GenerateTestHelpers(testCfg)
	if err != nil {
		return fmt.Errorf("failed to generate test helpers: %w", err)
	}
	helpersPath := filepath.Join(testDir, "helpers_test.go")
	if _, err := codegen.WriteFileIfChanged(helpersPath, helpersBytes); err != nil {
		return fmt.Errorf("failed to write helpers_test.go: %w", err)
	}

	// Reassign testCfg for clarity (already assigned above)
	testCfg = resourcegen.PerOpTestGenConfig{
		ModulePath:      modulePath,
		TableName:       tableName,
		Table:           table,
		Schema:          plan.Schema.Tables,
		RequireAuth:     requireAuth,
		Dialect:         dialect,
		TestDatabaseURL: testDatabaseURL,
		ScopeColumn:     scopeColumn,
	}

	for _, op := range ops {
		testBytes, err := generateSingleTest(testCfg, op)
		if err != nil {
			return fmt.Errorf("failed to generate %s test: %w", op, err)
		}

		testFilename := string(op) + "_test.go"
		testFilePath := filepath.Join(testDir, testFilename)
		if _, err := codegen.WriteFileIfChanged(testFilePath, testBytes); err != nil {
			return fmt.Errorf("failed to write %s: %w", testFilePath, err)
		}
		fmt.Printf("  Generated %s\n", testFilename)
	}

	// Compile the registry
	fmt.Println("")
	fmt.Println("Compiling handler registry...")
	if err := registry.Run(roots.ShipqRoot, roots.GoModRoot); err != nil {
		return fmt.Errorf("failed to compile registry: %w", err)
	}

	fmt.Println("")
	fmt.Printf("Done! Generated %s for %s.\n", operation, tableName)

	return nil
}

func generateSingleHandler(cfg handlergen.HandlerGenConfig, op handlergen.Operation, relations []handlergen.RelationshipInfo) ([]byte, error) {
	switch op {
	case handlergen.OpCreate:
		return handlergen.GenerateCreateHandler(cfg, relations)
	case handlergen.OpGetOne:
		// Pass nil relations: the query runner does not yet support
		// WithRelations, so we cannot embed relation data in get-one.
		return handlergen.GenerateGetOneHandler(cfg, nil)
	case handlergen.OpList:
		return handlergen.GenerateListHandler(cfg, relations)
	case handlergen.OpUpdate:
		return handlergen.GenerateUpdateHandler(cfg, relations)
	case handlergen.OpDelete:
		return handlergen.GenerateSoftDeleteHandler(cfg, relations)
	default:
		return nil, fmt.Errorf("unknown operation: %s", op)
	}
}

func generateSingleTest(cfg resourcegen.PerOpTestGenConfig, op handlergen.Operation) ([]byte, error) {
	switch op {
	case handlergen.OpCreate:
		return resourcegen.GenerateCreateTest(cfg)
	case handlergen.OpGetOne:
		return resourcegen.GenerateGetOneTest(cfg)
	case handlergen.OpList:
		return resourcegen.GenerateListTest(cfg)
	case handlergen.OpUpdate:
		return resourcegen.GenerateUpdateTest(cfg)
	case handlergen.OpDelete:
		return resourcegen.GenerateSoftDeleteTest(cfg)
	default:
		return nil, fmt.Errorf("unknown operation: %s", op)
	}
}
