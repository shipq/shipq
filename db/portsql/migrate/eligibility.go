package migrate

import (
	"github.com/shipq/shipq/db/portsql/ddl"
)

// AutoincrementPKInfo contains information about a table's autoincrement-eligible primary key.
type AutoincrementPKInfo struct {
	ColumnName string // Name of the PK column
	ColumnType string // DDL type of the PK column ("integer" or "bigint")
}

// GetAutoincrementPK determines if a table has an autoincrement-eligible primary key.
// A table is eligible if:
//   - It has exactly one primary key column (PrimaryKey=true)
//   - That PK column is an integer type ("integer" or "bigint")
//   - The table is not a junction/composite-PK table
//
// Returns the PK info and true if eligible, or empty info and false otherwise.
func GetAutoincrementPK(table *ddl.Table) (AutoincrementPKInfo, bool) {
	var pkColumns []ddl.ColumnDefinition

	// Count primary key columns
	for _, col := range table.Columns {
		if col.PrimaryKey {
			pkColumns = append(pkColumns, col)
		}
	}

	// Must have exactly one PK column (no composite PKs)
	if len(pkColumns) != 1 {
		return AutoincrementPKInfo{}, false
	}

	pkCol := pkColumns[0]

	// PK must be an integer type
	if pkCol.Type != ddl.IntegerType && pkCol.Type != ddl.BigintType {
		return AutoincrementPKInfo{}, false
	}

	// Additional guard: junction tables should not get autoincrement
	// (though they typically have composite PKs anyway)
	if table.IsJunctionTable {
		return AutoincrementPKInfo{}, false
	}

	return AutoincrementPKInfo{
		ColumnName: pkCol.Name,
		ColumnType: pkCol.Type,
	}, true
}

// IsAutoincrementEligible is a convenience function that returns true if the table
// has an autoincrement-eligible primary key.
func IsAutoincrementEligible(table *ddl.Table) bool {
	_, ok := GetAutoincrementPK(table)
	return ok
}

// IsAddTableTable returns true if the table was created with AddTable (has public_id AND deleted_at).
// Tables created with AddTable have the standard columns: id, public_id, created_at, updated_at, deleted_at.
// This is used to determine eligibility for CRUD generation and API resource generation.
func IsAddTableTable(table ddl.Table) bool {
	hasPublicID := false
	hasDeletedAt := false

	for _, col := range table.Columns {
		switch col.Name {
		case "public_id":
			hasPublicID = true
		case "deleted_at":
			hasDeletedAt = true
		}
	}

	return hasPublicID && hasDeletedAt
}

// GetCRUDTables returns tables from a MigrationPlan that qualify for CRUD generation.
// These are tables created with AddTable (have both public_id and deleted_at columns).
func GetCRUDTables(plan *MigrationPlan) []ddl.Table {
	var tables []ddl.Table
	for _, table := range plan.Schema.Tables {
		if IsAddTableTable(table) {
			tables = append(tables, table)
		}
	}
	return tables
}

// IsEligibleForResource returns true if a table is eligible for API resource generation.
// This is the same as IsAddTableTable - resources can only be generated for AddTable tables.
func IsEligibleForResource(table ddl.Table) bool {
	return IsAddTableTable(table)
}
