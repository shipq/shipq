package migrate

import (
	"github.com/shipq/shipq/db/portsql/ddl"
)

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
