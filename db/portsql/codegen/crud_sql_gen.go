package codegen

import (
	"fmt"
	"strings"

	"github.com/shipq/shipq/db/portsql/ddl"
)

// SQLDialect represents a database dialect for SQL generation.
type SQLDialect string

const (
	SQLDialectPostgres SQLDialect = "postgres"
	SQLDialectMySQL    SQLDialect = "mysql"
	SQLDialectSQLite   SQLDialect = "sqlite"
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
func GenerateCRUDSQL(table ddl.Table, dialect SQLDialect, opts CRUDOptions) CRUDSQLSet {
	analysis := AnalyzeTable(table)
	set := CRUDSQLSet{TableName: table.Name}

	set.GetSQL = generateGetSQL(table, analysis, dialect, opts)
	set.ListSQL = generateListSQL(table, analysis, dialect, opts)
	set.InsertSQL = generateInsertSQL(table, analysis, dialect, opts)
	set.UpdateSQL = generateUpdateSQL(table, analysis, dialect, opts)
	set.DeleteSQL = generateDeleteSQL(table, analysis, dialect, opts)
	if analysis.HasDeletedAt {
		set.HardDeleteSQL = generateHardDeleteSQL(table, analysis, dialect, opts)
	}

	return set
}

// quoteIdentifier quotes an identifier based on dialect.
func quoteIdentifier(name string, dialect SQLDialect) string {
	switch dialect {
	case SQLDialectMySQL:
		return "`" + name + "`"
	default: // Postgres, SQLite use double quotes
		return `"` + name + `"`
	}
}

// placeholder returns the parameter placeholder for the given index (1-based).
func placeholder(index int, dialect SQLDialect) string {
	switch dialect {
	case SQLDialectPostgres:
		return fmt.Sprintf("$%d", index)
	default: // MySQL, SQLite use ?
		return "?"
	}
}

// nowFunc returns the NOW() function for the dialect.
func nowFunc(dialect SQLDialect) string {
	switch dialect {
	case SQLDialectSQLite:
		return "datetime('now')"
	default: // Postgres, MySQL
		return "NOW()"
	}
}

// generateGetSQL generates SELECT ... WHERE public_id = ? (or id = ?)
func generateGetSQL(table ddl.Table, analysis TableAnalysis, dialect SQLDialect, opts CRUDOptions) string {
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

// generateListSQL generates SELECT ... ORDER BY ... LIMIT ?
// For tables with cursor support (created_at + public_id), uses keyset pagination.
// For tables without cursor support, falls back to OFFSET pagination.
func generateListSQL(table ddl.Table, analysis TableAnalysis, dialect SQLDialect, opts CRUDOptions) string {
	// Check if table supports cursor pagination
	supportsCursor := analysis.HasCreatedAt && analysis.HasPublicID

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

	// ORDER BY clause
	orderDir := " DESC"
	if opts.OrderAsc {
		orderDir = " ASC"
	}

	b.WriteString(" ORDER BY ")
	if supportsCursor {
		// Composite ORDER BY for cursor pagination (created_at, id for tiebreaker)
		b.WriteString(quoteIdentifier("created_at", dialect))
		b.WriteString(orderDir)
		b.WriteString(", ")
		b.WriteString(quoteIdentifier("id", dialect))
		b.WriteString(orderDir)
	} else if analysis.HasCreatedAt {
		b.WriteString(quoteIdentifier("created_at", dialect))
		b.WriteString(orderDir)
	} else if analysis.HasPublicID {
		b.WriteString(quoteIdentifier("public_id", dialect))
		b.WriteString(orderDir)
	} else {
		b.WriteString(quoteIdentifier("id", dialect))
		b.WriteString(orderDir)
	}

	// LIMIT clause
	b.WriteString(" LIMIT ")
	b.WriteString(placeholder(paramIdx, dialect))
	paramIdx++

	// OFFSET only for tables without cursor support (fallback)
	if !supportsCursor {
		b.WriteString(" OFFSET ")
		b.WriteString(placeholder(paramIdx, dialect))
	}

	return b.String()
}

// generateInsertSQL generates INSERT with auto-filled columns.
func generateInsertSQL(table ddl.Table, analysis TableAnalysis, dialect SQLDialect, opts CRUDOptions) string {
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
	if analysis.HasPublicID && (dialect == SQLDialectPostgres || dialect == SQLDialectSQLite) {
		b.WriteString(" RETURNING ")
		b.WriteString(quoteIdentifier("public_id", dialect))
	}

	return b.String()
}

// generateUpdateSQL generates UPDATE with auto-filled updated_at.
func generateUpdateSQL(table ddl.Table, analysis TableAnalysis, dialect SQLDialect, opts CRUDOptions) string {
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
func generateDeleteSQL(table ddl.Table, analysis TableAnalysis, dialect SQLDialect, opts CRUDOptions) string {
	var b strings.Builder
	paramIdx := 1

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
	}

	return b.String()
}

// generateHardDeleteSQL generates actual DELETE statement.
func generateHardDeleteSQL(table ddl.Table, analysis TableAnalysis, dialect SQLDialect, opts CRUDOptions) string {
	var b strings.Builder
	paramIdx := 1

	b.WriteString("DELETE FROM ")
	b.WriteString(quoteIdentifier(table.Name, dialect))
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

	return b.String()
}
