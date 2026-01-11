package codegen

import (
	"bytes"
	"fmt"
	"go/format"
	"sort"
	"strings"

	"github.com/portsql/portsql/src/ddl"
	"github.com/portsql/portsql/src/migrate"
)

// Dialect represents a database dialect for SQL generation.
type Dialect string

const (
	DialectPostgres Dialect = "postgres"
	DialectMySQL    Dialect = "mysql"
	DialectSQLite   Dialect = "sqlite"
)

// CRUDSQLSet contains all generated SQL strings for a single table.
type CRUDSQLSet struct {
	TableName string

	// Get by public_id (or id if no public_id)
	GetSQL string

	// List with pagination
	ListSQL string

	// Insert with auto-filled columns
	InsertSQL string

	// Update with auto-filled updated_at
	UpdateSQL string

	// Soft delete (sets deleted_at)
	DeleteSQL string

	// Hard delete (actual DELETE)
	HardDeleteSQL string
}

// GenerateCRUDSQL generates SQL strings for all CRUD operations for a table.
func GenerateCRUDSQL(table ddl.Table, dialect Dialect, opts CRUDOptions) CRUDSQLSet {
	analysis := AnalyzeTable(table)
	set := CRUDSQLSet{TableName: table.Name}

	set.GetSQL = generateGetSQL(table, analysis, dialect, opts)
	set.ListSQL = generateListSQL(table, analysis, dialect, opts)
	set.InsertSQL = generateInsertSQL(table, analysis, dialect, opts)
	set.UpdateSQL = generateUpdateSQL(table, analysis, dialect, opts)
	set.DeleteSQL = generateDeleteSQL(table, analysis, dialect)
	if analysis.HasDeletedAt {
		set.HardDeleteSQL = generateHardDeleteSQL(table, analysis, dialect)
	}

	return set
}

// quoteIdentifier quotes an identifier based on dialect.
func quoteIdentifier(name string, dialect Dialect) string {
	switch dialect {
	case DialectMySQL:
		return "`" + name + "`"
	default: // Postgres, SQLite use double quotes
		return `"` + name + `"`
	}
}

// placeholder returns the parameter placeholder for the given index (1-based).
func placeholder(index int, dialect Dialect) string {
	switch dialect {
	case DialectPostgres:
		return fmt.Sprintf("$%d", index)
	default: // MySQL, SQLite use ?
		return "?"
	}
}

// nowFunc returns the NOW() function for the dialect.
func nowFunc(dialect Dialect) string {
	switch dialect {
	case DialectSQLite:
		return "datetime('now')"
	default: // Postgres, MySQL
		return "NOW()"
	}
}

// generateGetSQL generates SELECT ... WHERE public_id = ? (or id = ?)
func generateGetSQL(table ddl.Table, analysis TableAnalysis, dialect Dialect, opts CRUDOptions) string {
	var b strings.Builder

	b.WriteString("SELECT ")

	// Select result columns
	first := true
	for _, col := range analysis.ResultColumns {
		if !first {
			b.WriteString(", ")
		}
		b.WriteString(quoteIdentifier(col.Name, dialect))
		first = false
	}

	b.WriteString(" FROM ")
	b.WriteString(quoteIdentifier(table.Name, dialect))
	b.WriteString(" WHERE ")

	paramIdx := 1

	// Use public_id if available, otherwise id
	if analysis.HasPublicID {
		b.WriteString(quoteIdentifier("public_id", dialect))
		b.WriteString(" = ")
		b.WriteString(placeholder(paramIdx, dialect))
		paramIdx++
	} else {
		b.WriteString(quoteIdentifier("id", dialect))
		b.WriteString(" = ")
		b.WriteString(placeholder(paramIdx, dialect))
		paramIdx++
	}

	// Add scope column if configured
	if opts.ScopeColumn != "" {
		b.WriteString(" AND ")
		b.WriteString(quoteIdentifier(opts.ScopeColumn, dialect))
		b.WriteString(" = ")
		b.WriteString(placeholder(paramIdx, dialect))
		paramIdx++
	}

	// Exclude soft-deleted records
	if analysis.HasDeletedAt {
		b.WriteString(" AND ")
		b.WriteString(quoteIdentifier("deleted_at", dialect))
		b.WriteString(" IS NULL")
	}

	return b.String()
}

// generateListSQL generates SELECT ... ORDER BY ... LIMIT ? OFFSET ?
func generateListSQL(table ddl.Table, analysis TableAnalysis, dialect Dialect, opts CRUDOptions) string {
	var b strings.Builder

	b.WriteString("SELECT ")

	// Select result columns (excluding updated_at for list brevity)
	first := true
	for _, col := range analysis.ResultColumns {
		if col.Name == "updated_at" {
			continue
		}
		if !first {
			b.WriteString(", ")
		}
		b.WriteString(quoteIdentifier(col.Name, dialect))
		first = false
	}

	b.WriteString(" FROM ")
	b.WriteString(quoteIdentifier(table.Name, dialect))
	b.WriteString(" WHERE ")

	paramIdx := 1

	// Add scope column if configured
	if opts.ScopeColumn != "" {
		b.WriteString(quoteIdentifier(opts.ScopeColumn, dialect))
		b.WriteString(" = ")
		b.WriteString(placeholder(paramIdx, dialect))
		paramIdx++
		b.WriteString(" AND ")
	}

	// Exclude soft-deleted records
	if analysis.HasDeletedAt {
		b.WriteString(quoteIdentifier("deleted_at", dialect))
		b.WriteString(" IS NULL")
	} else {
		b.WriteString("1 = 1") // Always true if no deleted_at
	}

	// Order by created_at DESC if available, otherwise by id
	b.WriteString(" ORDER BY ")
	if analysis.HasCreatedAt {
		b.WriteString(quoteIdentifier("created_at", dialect))
		b.WriteString(" DESC")
	} else if analysis.HasPublicID {
		b.WriteString(quoteIdentifier("public_id", dialect))
		b.WriteString(" DESC")
	} else {
		b.WriteString(quoteIdentifier("id", dialect))
		b.WriteString(" DESC")
	}

	b.WriteString(" LIMIT ")
	b.WriteString(placeholder(paramIdx, dialect))
	paramIdx++
	b.WriteString(" OFFSET ")
	b.WriteString(placeholder(paramIdx, dialect))

	return b.String()
}

// generateInsertSQL generates INSERT with auto-filled columns.
func generateInsertSQL(table ddl.Table, analysis TableAnalysis, dialect Dialect, opts CRUDOptions) string {
	var b strings.Builder

	b.WriteString("INSERT INTO ")
	b.WriteString(quoteIdentifier(table.Name, dialect))
	b.WriteString(" (")

	// Build column list and values list
	var columns []string
	var values []string
	paramIdx := 1

	// public_id is passed as a parameter (generated in Go code)
	if analysis.HasPublicID {
		columns = append(columns, quoteIdentifier("public_id", dialect))
		values = append(values, placeholder(paramIdx, dialect))
		paramIdx++
	}

	// Add scope column if configured
	if opts.ScopeColumn != "" {
		columns = append(columns, quoteIdentifier(opts.ScopeColumn, dialect))
		values = append(values, placeholder(paramIdx, dialect))
		paramIdx++
	}

	// User-provided columns
	for _, col := range analysis.UserColumns {
		// Skip scope column if already added
		if opts.ScopeColumn != "" && col.Name == opts.ScopeColumn {
			continue
		}
		columns = append(columns, quoteIdentifier(col.Name, dialect))
		values = append(values, placeholder(paramIdx, dialect))
		paramIdx++
	}

	// Auto-filled timestamp columns
	if analysis.HasCreatedAt {
		columns = append(columns, quoteIdentifier("created_at", dialect))
		values = append(values, nowFunc(dialect))
	}
	if analysis.HasUpdatedAt {
		columns = append(columns, quoteIdentifier("updated_at", dialect))
		values = append(values, nowFunc(dialect))
	}

	b.WriteString(strings.Join(columns, ", "))
	b.WriteString(") VALUES (")
	b.WriteString(strings.Join(values, ", "))
	b.WriteString(")")

	// RETURNING clause for Postgres and SQLite
	if analysis.HasPublicID && (dialect == DialectPostgres || dialect == DialectSQLite) {
		b.WriteString(" RETURNING ")
		b.WriteString(quoteIdentifier("public_id", dialect))
	}

	return b.String()
}

// generateUpdateSQL generates UPDATE with auto-filled updated_at.
func generateUpdateSQL(table ddl.Table, analysis TableAnalysis, dialect Dialect, opts CRUDOptions) string {
	var b strings.Builder

	b.WriteString("UPDATE ")
	b.WriteString(quoteIdentifier(table.Name, dialect))
	b.WriteString(" SET ")

	// Build SET clause
	var setClauses []string
	paramIdx := 1

	// User-provided columns
	for _, col := range analysis.UserColumns {
		// Skip scope column in SET clause
		if opts.ScopeColumn != "" && col.Name == opts.ScopeColumn {
			continue
		}
		setClauses = append(setClauses,
			quoteIdentifier(col.Name, dialect)+" = "+placeholder(paramIdx, dialect))
		paramIdx++
	}

	// Auto-fill updated_at
	if analysis.HasUpdatedAt {
		setClauses = append(setClauses,
			quoteIdentifier("updated_at", dialect)+" = "+nowFunc(dialect))
	}

	b.WriteString(strings.Join(setClauses, ", "))

	// WHERE clause
	b.WriteString(" WHERE ")

	if analysis.HasPublicID {
		b.WriteString(quoteIdentifier("public_id", dialect))
		b.WriteString(" = ")
		b.WriteString(placeholder(paramIdx, dialect))
		paramIdx++
	} else {
		b.WriteString(quoteIdentifier("id", dialect))
		b.WriteString(" = ")
		b.WriteString(placeholder(paramIdx, dialect))
		paramIdx++
	}

	// Add scope column if configured
	if opts.ScopeColumn != "" {
		b.WriteString(" AND ")
		b.WriteString(quoteIdentifier(opts.ScopeColumn, dialect))
		b.WriteString(" = ")
		b.WriteString(placeholder(paramIdx, dialect))
		paramIdx++
	}

	// Exclude soft-deleted records
	if analysis.HasDeletedAt {
		b.WriteString(" AND ")
		b.WriteString(quoteIdentifier("deleted_at", dialect))
		b.WriteString(" IS NULL")
	}

	return b.String()
}

// generateDeleteSQL generates soft delete (UPDATE ... SET deleted_at = NOW()).
func generateDeleteSQL(table ddl.Table, analysis TableAnalysis, dialect Dialect) string {
	var b strings.Builder

	if analysis.HasDeletedAt {
		// Soft delete: UPDATE ... SET deleted_at = NOW()
		b.WriteString("UPDATE ")
		b.WriteString(quoteIdentifier(table.Name, dialect))
		b.WriteString(" SET ")
		b.WriteString(quoteIdentifier("deleted_at", dialect))
		b.WriteString(" = ")
		b.WriteString(nowFunc(dialect))
		b.WriteString(" WHERE ")

		if analysis.HasPublicID {
			b.WriteString(quoteIdentifier("public_id", dialect))
			b.WriteString(" = ")
			b.WriteString(placeholder(1, dialect))
		} else {
			b.WriteString(quoteIdentifier("id", dialect))
			b.WriteString(" = ")
			b.WriteString(placeholder(1, dialect))
		}

		// Only soft-delete if not already deleted
		b.WriteString(" AND ")
		b.WriteString(quoteIdentifier("deleted_at", dialect))
		b.WriteString(" IS NULL")
	} else {
		// No deleted_at column - do a hard delete
		b.WriteString("DELETE FROM ")
		b.WriteString(quoteIdentifier(table.Name, dialect))
		b.WriteString(" WHERE ")

		if analysis.HasPublicID {
			b.WriteString(quoteIdentifier("public_id", dialect))
			b.WriteString(" = ")
			b.WriteString(placeholder(1, dialect))
		} else {
			b.WriteString(quoteIdentifier("id", dialect))
			b.WriteString(" = ")
			b.WriteString(placeholder(1, dialect))
		}
	}

	return b.String()
}

// generateHardDeleteSQL generates actual DELETE statement.
func generateHardDeleteSQL(table ddl.Table, analysis TableAnalysis, dialect Dialect) string {
	var b strings.Builder

	b.WriteString("DELETE FROM ")
	b.WriteString(quoteIdentifier(table.Name, dialect))
	b.WriteString(" WHERE ")

	if analysis.HasPublicID {
		b.WriteString(quoteIdentifier("public_id", dialect))
		b.WriteString(" = ")
		b.WriteString(placeholder(1, dialect))
	} else {
		b.WriteString(quoteIdentifier("id", dialect))
		b.WriteString(" = ")
		b.WriteString(placeholder(1, dialect))
	}

	return b.String()
}

// =============================================================================
// CRUD Runner Code Generation
// =============================================================================

// GenerateCRUDRunner generates the Go code for a QueryRunner with CRUD methods.
func GenerateCRUDRunner(plan *migrate.MigrationPlan, packageName string, tableOpts map[string]CRUDOptions) ([]byte, error) {
	var buf bytes.Buffer

	// Sort tables for deterministic output
	tableNames := make([]string, 0, len(plan.Schema.Tables))
	for name := range plan.Schema.Tables {
		tableNames = append(tableNames, name)
	}
	sort.Strings(tableNames)

	// Collect imports
	imports := []string{
		"context",
		"database/sql",
		"github.com/portsql/nanoid",
	}

	// Check if we need time import
	needsTime := false
	for _, tableName := range tableNames {
		table := plan.Schema.Tables[tableName]
		for _, col := range table.Columns {
			mapping := MapColumnType(col)
			if mapping.NeedsImport == "time" {
				needsTime = true
				break
			}
		}
		if needsTime {
			break
		}
	}
	if needsTime {
		imports = append(imports, "time")
	}

	// Write package and imports
	buf.WriteString("// Code generated by orm codegen. DO NOT EDIT.\n")
	buf.WriteString(fmt.Sprintf("package %s\n\n", packageName))

	buf.WriteString("import (\n")
	sort.Strings(imports)
	for _, imp := range imports {
		buf.WriteString(fmt.Sprintf("\t%q\n", imp))
	}
	buf.WriteString(")\n\n")

	// Write Dialect type
	buf.WriteString("// Dialect identifies the target database.\n")
	buf.WriteString("type Dialect int\n\n")
	buf.WriteString("const (\n")
	buf.WriteString("\tPostgres Dialect = iota\n")
	buf.WriteString("\tMySQL\n")
	buf.WriteString("\tSQLite\n")
	buf.WriteString(")\n\n")

	// Write Querier interface
	buf.WriteString("// Querier is the interface for executing queries.\n")
	buf.WriteString("type Querier interface {\n")
	buf.WriteString("\tExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)\n")
	buf.WriteString("\tQueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)\n")
	buf.WriteString("\tQueryRowContext(ctx context.Context, query string, args ...any) *sql.Row\n")
	buf.WriteString("}\n\n")

	// Write QueryRunner struct
	buf.WriteString("// QueryRunner holds pre-compiled SQL strings for a specific dialect.\n")
	buf.WriteString("type QueryRunner struct {\n")
	buf.WriteString("\tdialect Dialect\n")
	buf.WriteString("\tdb      Querier\n\n")

	// Add SQL string fields for each table
	for _, tableName := range tableNames {
		singular := toSingular(tableName)
		singularCamel := toLowerCamel(singular)
		buf.WriteString(fmt.Sprintf("\t// %s SQL strings\n", toPascalCase(tableName)))
		buf.WriteString(fmt.Sprintf("\tget%sSQL    string\n", toPascalCase(singular)))
		buf.WriteString(fmt.Sprintf("\tlist%sSQL   string\n", toPascalCase(tableName)))
		buf.WriteString(fmt.Sprintf("\tinsert%sSQL string\n", toPascalCase(singular)))
		buf.WriteString(fmt.Sprintf("\tupdate%sSQL string\n", toPascalCase(singular)))
		buf.WriteString(fmt.Sprintf("\tdelete%sSQL string\n", toPascalCase(singular)))

		table := plan.Schema.Tables[tableName]
		analysis := AnalyzeTable(table)
		if analysis.HasDeletedAt {
			buf.WriteString(fmt.Sprintf("\thardDelete%sSQL string\n", toPascalCase(singular)))
		}
		_ = singularCamel // silence unused variable
		buf.WriteString("\n")
	}
	buf.WriteString("}\n\n")

	// Write NewQueryRunner constructor
	buf.WriteString("// NewQueryRunner creates a runner for the given dialect.\n")
	buf.WriteString("func NewQueryRunner(db Querier, dialect Dialect) *QueryRunner {\n")
	buf.WriteString("\tr := &QueryRunner{\n")
	buf.WriteString("\t\tdialect: dialect,\n")
	buf.WriteString("\t\tdb:      db,\n")
	buf.WriteString("\t}\n\n")

	buf.WriteString("\tswitch dialect {\n")

	// Generate SQL initialization for each dialect
	for _, d := range []Dialect{DialectPostgres, DialectMySQL, DialectSQLite} {
		dialectName := "Postgres"
		if d == DialectMySQL {
			dialectName = "MySQL"
		} else if d == DialectSQLite {
			dialectName = "SQLite"
		}

		buf.WriteString(fmt.Sprintf("\tcase %s:\n", dialectName))

		for _, tableName := range tableNames {
			table := plan.Schema.Tables[tableName]
			opts := tableOpts[tableName]
			sqlSet := GenerateCRUDSQL(table, d, opts)

			singular := toSingular(tableName)
			buf.WriteString(fmt.Sprintf("\t\tr.get%sSQL = %q\n", toPascalCase(singular), sqlSet.GetSQL))
			buf.WriteString(fmt.Sprintf("\t\tr.list%sSQL = %q\n", toPascalCase(tableName), sqlSet.ListSQL))
			buf.WriteString(fmt.Sprintf("\t\tr.insert%sSQL = %q\n", toPascalCase(singular), sqlSet.InsertSQL))
			buf.WriteString(fmt.Sprintf("\t\tr.update%sSQL = %q\n", toPascalCase(singular), sqlSet.UpdateSQL))
			buf.WriteString(fmt.Sprintf("\t\tr.delete%sSQL = %q\n", toPascalCase(singular), sqlSet.DeleteSQL))

			analysis := AnalyzeTable(table)
			if analysis.HasDeletedAt {
				buf.WriteString(fmt.Sprintf("\t\tr.hardDelete%sSQL = %q\n", toPascalCase(singular), sqlSet.HardDeleteSQL))
			}
		}
	}

	buf.WriteString("\t}\n\n")
	buf.WriteString("\treturn r\n")
	buf.WriteString("}\n\n")

	// Write WithTx method
	buf.WriteString("// WithTx returns a new QueryRunner using the given transaction.\n")
	buf.WriteString("func (r *QueryRunner) WithTx(tx *sql.Tx) *QueryRunner {\n")
	buf.WriteString("\treturn &QueryRunner{\n")
	buf.WriteString("\t\tdialect: r.dialect,\n")
	buf.WriteString("\t\tdb:      tx,\n")
	for _, tableName := range tableNames {
		singular := toSingular(tableName)
		buf.WriteString(fmt.Sprintf("\t\tget%sSQL:    r.get%sSQL,\n", toPascalCase(singular), toPascalCase(singular)))
		buf.WriteString(fmt.Sprintf("\t\tlist%sSQL:   r.list%sSQL,\n", toPascalCase(tableName), toPascalCase(tableName)))
		buf.WriteString(fmt.Sprintf("\t\tinsert%sSQL: r.insert%sSQL,\n", toPascalCase(singular), toPascalCase(singular)))
		buf.WriteString(fmt.Sprintf("\t\tupdate%sSQL: r.update%sSQL,\n", toPascalCase(singular), toPascalCase(singular)))
		buf.WriteString(fmt.Sprintf("\t\tdelete%sSQL: r.delete%sSQL,\n", toPascalCase(singular), toPascalCase(singular)))

		table := plan.Schema.Tables[tableName]
		analysis := AnalyzeTable(table)
		if analysis.HasDeletedAt {
			buf.WriteString(fmt.Sprintf("\t\thardDelete%sSQL: r.hardDelete%sSQL,\n", toPascalCase(singular), toPascalCase(singular)))
		}
	}
	buf.WriteString("\t}\n")
	buf.WriteString("}\n\n")

	// Generate CRUD methods for each table
	for _, tableName := range tableNames {
		table := plan.Schema.Tables[tableName]
		opts := tableOpts[tableName]
		generateTableCRUDMethods(&buf, table, opts)
	}

	// Format the code
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return buf.Bytes(), fmt.Errorf("failed to format generated code: %w", err)
	}

	return formatted, nil
}

// generateTableCRUDMethods generates CRUD methods for a single table.
func generateTableCRUDMethods(buf *bytes.Buffer, table ddl.Table, opts CRUDOptions) {
	analysis := AnalyzeTable(table)
	singular := toSingular(table.Name)
	singularPascal := toPascalCase(singular)
	pluralPascal := toPascalCase(table.Name)

	// --- Get ---
	generateGetMethod(buf, table, analysis, singularPascal, opts)

	// --- List ---
	generateListMethod(buf, table, analysis, singularPascal, pluralPascal, opts)

	// --- Insert ---
	generateInsertMethod(buf, table, analysis, singularPascal, opts)

	// --- Update ---
	generateUpdateMethod(buf, table, analysis, singularPascal, opts)

	// --- Delete ---
	generateDeleteMethod(buf, table, analysis, singularPascal)

	// --- HardDelete ---
	if analysis.HasDeletedAt {
		generateHardDeleteMethod(buf, table, analysis, singularPascal)
	}
}

func generateGetMethod(buf *bytes.Buffer, table ddl.Table, analysis TableAnalysis, singularPascal string, opts CRUDOptions) {
	buf.WriteString(fmt.Sprintf("// Get%s fetches a single %s by its identifier.\n", singularPascal, toSingular(table.Name)))
	buf.WriteString(fmt.Sprintf("func (r *QueryRunner) Get%s(ctx context.Context, params Get%sParams) (*Get%sResult, error) {\n",
		singularPascal, singularPascal, singularPascal))

	// Build args list
	buf.WriteString("\tvar args []any\n")
	if analysis.HasPublicID {
		buf.WriteString("\targs = append(args, params.PublicID)\n")
	} else {
		buf.WriteString("\targs = append(args, params.ID)\n")
	}
	if opts.ScopeColumn != "" {
		buf.WriteString(fmt.Sprintf("\targs = append(args, params.%s)\n", toPascalCase(opts.ScopeColumn)))
	}

	buf.WriteString(fmt.Sprintf("\n\trow := r.db.QueryRowContext(ctx, r.get%sSQL, args...)\n", singularPascal))
	buf.WriteString(fmt.Sprintf("\n\tvar result Get%sResult\n", singularPascal))
	buf.WriteString("\terr := row.Scan(\n")

	// Scan result columns
	for _, col := range analysis.ResultColumns {
		buf.WriteString(fmt.Sprintf("\t\t&result.%s,\n", toPascalCase(col.Name)))
	}
	buf.WriteString("\t)\n")
	buf.WriteString("\tif err == sql.ErrNoRows {\n")
	buf.WriteString("\t\treturn nil, nil\n")
	buf.WriteString("\t}\n")
	buf.WriteString("\tif err != nil {\n")
	buf.WriteString("\t\treturn nil, err\n")
	buf.WriteString("\t}\n")
	buf.WriteString("\treturn &result, nil\n")
	buf.WriteString("}\n\n")
}

func generateListMethod(buf *bytes.Buffer, table ddl.Table, analysis TableAnalysis, singularPascal, pluralPascal string, opts CRUDOptions) {
	buf.WriteString(fmt.Sprintf("// List%s fetches a paginated list of %s.\n", pluralPascal, table.Name))
	buf.WriteString(fmt.Sprintf("func (r *QueryRunner) List%s(ctx context.Context, params List%sParams) ([]List%sResult, error) {\n",
		pluralPascal, pluralPascal, pluralPascal))

	// Build args list
	buf.WriteString("\tvar args []any\n")
	if opts.ScopeColumn != "" {
		buf.WriteString(fmt.Sprintf("\targs = append(args, params.%s)\n", toPascalCase(opts.ScopeColumn)))
	}
	buf.WriteString("\targs = append(args, params.Limit, params.Offset)\n")

	buf.WriteString(fmt.Sprintf("\n\trows, err := r.db.QueryContext(ctx, r.list%sSQL, args...)\n", pluralPascal))
	buf.WriteString("\tif err != nil {\n")
	buf.WriteString("\t\treturn nil, err\n")
	buf.WriteString("\t}\n")
	buf.WriteString("\tdefer rows.Close()\n")

	buf.WriteString(fmt.Sprintf("\n\tvar results []List%sResult\n", pluralPascal))
	buf.WriteString("\tfor rows.Next() {\n")
	buf.WriteString(fmt.Sprintf("\t\tvar item List%sResult\n", pluralPascal))
	buf.WriteString("\t\terr := rows.Scan(\n")

	// Scan result columns (excluding updated_at)
	for _, col := range analysis.ResultColumns {
		if col.Name == "updated_at" {
			continue
		}
		buf.WriteString(fmt.Sprintf("\t\t\t&item.%s,\n", toPascalCase(col.Name)))
	}
	buf.WriteString("\t\t)\n")
	buf.WriteString("\t\tif err != nil {\n")
	buf.WriteString("\t\t\treturn nil, err\n")
	buf.WriteString("\t\t}\n")
	buf.WriteString("\t\tresults = append(results, item)\n")
	buf.WriteString("\t}\n")

	buf.WriteString("\tif err := rows.Err(); err != nil {\n")
	buf.WriteString("\t\treturn nil, err\n")
	buf.WriteString("\t}\n")
	buf.WriteString("\treturn results, nil\n")
	buf.WriteString("}\n\n")
}

func generateInsertMethod(buf *bytes.Buffer, table ddl.Table, analysis TableAnalysis, singularPascal string, opts CRUDOptions) {
	// Determine return type
	returnType := "error"
	if analysis.HasPublicID {
		returnType = "(string, error)"
	}

	buf.WriteString(fmt.Sprintf("// Insert%s inserts a new %s and returns its public ID.\n", singularPascal, toSingular(table.Name)))
	buf.WriteString(fmt.Sprintf("func (r *QueryRunner) Insert%s(ctx context.Context, params Insert%sParams) %s {\n",
		singularPascal, singularPascal, returnType))

	// Generate public_id if needed
	if analysis.HasPublicID {
		buf.WriteString("\tpublicID := nanoid.New()\n\n")
	}

	// Build args list
	buf.WriteString("\tvar args []any\n")
	if analysis.HasPublicID {
		buf.WriteString("\targs = append(args, publicID)\n")
	}
	if opts.ScopeColumn != "" {
		buf.WriteString(fmt.Sprintf("\targs = append(args, params.%s)\n", toPascalCase(opts.ScopeColumn)))
	}
	for _, col := range analysis.UserColumns {
		if opts.ScopeColumn != "" && col.Name == opts.ScopeColumn {
			continue
		}
		buf.WriteString(fmt.Sprintf("\targs = append(args, params.%s)\n", toPascalCase(col.Name)))
	}

	buf.WriteString("\n")

	if analysis.HasPublicID {
		// Handle dialect-specific RETURNING behavior
		buf.WriteString("\tif r.dialect == MySQL {\n")
		buf.WriteString(fmt.Sprintf("\t\t_, err := r.db.ExecContext(ctx, r.insert%sSQL, args...)\n", singularPascal))
		buf.WriteString("\t\tif err != nil {\n")
		buf.WriteString("\t\t\treturn \"\", err\n")
		buf.WriteString("\t\t}\n")
		buf.WriteString("\t\treturn publicID, nil\n")
		buf.WriteString("\t}\n\n")

		buf.WriteString("\t// Postgres/SQLite: Use RETURNING\n")
		buf.WriteString("\tvar returnedID string\n")
		buf.WriteString(fmt.Sprintf("\terr := r.db.QueryRowContext(ctx, r.insert%sSQL, args...).Scan(&returnedID)\n", singularPascal))
		buf.WriteString("\tif err != nil {\n")
		buf.WriteString("\t\treturn \"\", err\n")
		buf.WriteString("\t}\n")
		buf.WriteString("\treturn returnedID, nil\n")
	} else {
		buf.WriteString(fmt.Sprintf("\t_, err := r.db.ExecContext(ctx, r.insert%sSQL, args...)\n", singularPascal))
		buf.WriteString("\treturn err\n")
	}

	buf.WriteString("}\n\n")
}

func generateUpdateMethod(buf *bytes.Buffer, table ddl.Table, analysis TableAnalysis, singularPascal string, opts CRUDOptions) {
	buf.WriteString(fmt.Sprintf("// Update%s updates an existing %s.\n", singularPascal, toSingular(table.Name)))
	buf.WriteString(fmt.Sprintf("func (r *QueryRunner) Update%s(ctx context.Context, params Update%sParams) error {\n",
		singularPascal, singularPascal))

	// Build args list: SET values first, then WHERE values
	buf.WriteString("\tvar args []any\n")

	// SET clause args (user columns, excluding scope)
	for _, col := range analysis.UserColumns {
		if opts.ScopeColumn != "" && col.Name == opts.ScopeColumn {
			continue
		}
		buf.WriteString(fmt.Sprintf("\targs = append(args, params.%s)\n", toPascalCase(col.Name)))
	}

	// WHERE clause args
	if analysis.HasPublicID {
		buf.WriteString("\targs = append(args, params.PublicID)\n")
	} else {
		buf.WriteString("\targs = append(args, params.ID)\n")
	}
	if opts.ScopeColumn != "" {
		buf.WriteString(fmt.Sprintf("\targs = append(args, params.%s)\n", toPascalCase(opts.ScopeColumn)))
	}

	buf.WriteString(fmt.Sprintf("\n\t_, err := r.db.ExecContext(ctx, r.update%sSQL, args...)\n", singularPascal))
	buf.WriteString("\treturn err\n")
	buf.WriteString("}\n\n")
}

func generateDeleteMethod(buf *bytes.Buffer, table ddl.Table, analysis TableAnalysis, singularPascal string) {
	buf.WriteString(fmt.Sprintf("// Delete%s soft-deletes a %s (or hard-deletes if no deleted_at column).\n", singularPascal, toSingular(table.Name)))
	buf.WriteString(fmt.Sprintf("func (r *QueryRunner) Delete%s(ctx context.Context, params Delete%sParams) error {\n",
		singularPascal, singularPascal))

	if analysis.HasPublicID {
		buf.WriteString(fmt.Sprintf("\t_, err := r.db.ExecContext(ctx, r.delete%sSQL, params.PublicID)\n", singularPascal))
	} else {
		buf.WriteString(fmt.Sprintf("\t_, err := r.db.ExecContext(ctx, r.delete%sSQL, params.ID)\n", singularPascal))
	}
	buf.WriteString("\treturn err\n")
	buf.WriteString("}\n\n")
}

func generateHardDeleteMethod(buf *bytes.Buffer, table ddl.Table, analysis TableAnalysis, singularPascal string) {
	buf.WriteString(fmt.Sprintf("// HardDelete%s permanently deletes a %s.\n", singularPascal, toSingular(table.Name)))
	buf.WriteString(fmt.Sprintf("func (r *QueryRunner) HardDelete%s(ctx context.Context, params HardDelete%sParams) error {\n",
		singularPascal, singularPascal))

	if analysis.HasPublicID {
		buf.WriteString(fmt.Sprintf("\t_, err := r.db.ExecContext(ctx, r.hardDelete%sSQL, params.PublicID)\n", singularPascal))
	} else {
		buf.WriteString(fmt.Sprintf("\t_, err := r.db.ExecContext(ctx, r.hardDelete%sSQL, params.ID)\n", singularPascal))
	}
	buf.WriteString("\treturn err\n")
	buf.WriteString("}\n\n")
}

// toLowerCamel converts a snake_case string to lowerCamelCase.
func toLowerCamel(s string) string {
	pascal := toPascalCase(s)
	if len(pascal) == 0 {
		return pascal
	}
	return strings.ToLower(pascal[:1]) + pascal[1:]
}
