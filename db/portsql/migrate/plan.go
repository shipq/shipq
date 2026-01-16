package migrate

import (
	"fmt"

	"github.com/shipq/shipq/db/portsql/ddl"
)

// ValidateMigrationName validates that a migration name follows the TIMESTAMP_name format.
// The name must be at least 16 characters: 14 digit timestamp + underscore + at least 1 char.
func ValidateMigrationName(name string) error {
	// Check minimum length for: 14 digit timestamp + underscore + 1 char name
	if len(name) < 16 {
		// Provide more specific error based on what's wrong
		if len(name) < 14 {
			return fmt.Errorf("migration name must start with 14-digit timestamp, too short: %q", name)
		}
		if len(name) == 14 {
			return fmt.Errorf("migration name must have underscore and name after timestamp, too short: %q", name)
		}
		if len(name) == 15 {
			return fmt.Errorf("migration name empty after timestamp underscore: %q", name)
		}
		return fmt.Errorf("migration name too short: %q", name)
	}

	// First 14 characters must be digits (timestamp)
	for i := 0; i < 14; i++ {
		if name[i] < '0' || name[i] > '9' {
			return fmt.Errorf("migration name must start with 14-digit timestamp: %q", name)
		}
	}

	// Character 15 (index 14) must be underscore
	if name[14] != '_' {
		return fmt.Errorf("migration name must have underscore after timestamp: %q", name)
	}

	return nil
}

type Schema struct {
	Name   string               `json:"name"`
	Tables map[string]ddl.Table `json:"tables"`
}

const (
	Sqlite   = "sqlite"
	Postgres = "postgres"
	MySQL    = "mysql"
)

type MigrationInstructions struct {
	Sqlite   string `json:"sqlite"`
	Postgres string `json:"postgres"`
	MySQL    string `json:"mysql"`
}

type Migration struct {
	Instructions MigrationInstructions `json:"instructions"`
	Name         string                `json:"name"`
}

type MigrationPlan struct {
	Schema     Schema      `json:"schema"`
	Migrations []Migration `json:"migrations"`
}

// AddEmptyTable creates a new table in the schema and passes a TableBuilder for type-safe column definitions.
func (m *MigrationPlan) AddEmptyTable(name string, fn func(*ddl.TableBuilder) error) (*MigrationPlan, error) {
	// Check for duplicate table
	if _, exists := m.Schema.Tables[name]; exists {
		return nil, fmt.Errorf("table %q already exists in schema", name)
	}

	// Initialize tables map if nil
	if m.Schema.Tables == nil {
		m.Schema.Tables = make(map[string]ddl.Table)
	}

	// Build the table using the TableBuilder
	tb := ddl.MakeEmptyTable(name)
	if err := fn(tb); err != nil {
		return nil, err
	}

	// Add the built table to the schema
	table := tb.Build()
	m.Schema.Tables[name] = *table

	// Generate SQL for each database
	m.Migrations = append(m.Migrations, Migration{
		Name: fmt.Sprintf("create_%s_table", name),
		Instructions: MigrationInstructions{
			Postgres: generatePostgresCreateTable(table),
			MySQL:    generateMySQLCreateTable(table),
			Sqlite:   generateSQLiteCreateTable(table),
		},
	})

	return m, nil
}

// AddTable creates a new table with default columns (id, public_id, created_at, deleted_at, updated_at)
// and passes a TableBuilder for adding additional columns.
func (m *MigrationPlan) AddTable(name string, fn func(*ddl.TableBuilder) error) (*MigrationPlan, error) {
	// Check for duplicate table
	if _, exists := m.Schema.Tables[name]; exists {
		return nil, fmt.Errorf("table %q already exists in schema", name)
	}

	// Initialize tables map if nil
	if m.Schema.Tables == nil {
		m.Schema.Tables = make(map[string]ddl.Table)
	}

	// Build the table using MakeTable (with default columns)
	tb := ddl.MakeTable(name)
	if err := fn(tb); err != nil {
		return nil, err
	}

	// Add the built table to the schema
	table := tb.Build()
	m.Schema.Tables[name] = *table

	// Generate SQL for each database
	m.Migrations = append(m.Migrations, Migration{
		Name: fmt.Sprintf("create_%s_table", name),
		Instructions: MigrationInstructions{
			Postgres: generatePostgresCreateTable(table),
			MySQL:    generateMySQLCreateTable(table),
			Sqlite:   generateSQLiteCreateTable(table),
		},
	})

	return m, nil
}

// UpdateTable looks up an existing table from the schema and passes an AlterTableBuilder
// with access to the table's columns for type-safe column references via ExistingColumn.
func (m *MigrationPlan) UpdateTable(tableName string, fn func(*ddl.AlterTableBuilder) error) error {
	table, ok := m.Schema.Tables[tableName]
	if !ok {
		return fmt.Errorf("table %q not found in schema", tableName)
	}
	alt := ddl.AlterTableFrom(&table)
	if err := fn(alt); err != nil {
		return err
	}

	// Apply operations to the schema
	operations := alt.Build()
	for _, op := range operations {
		switch op.Type {
		case ddl.OpAddColumn:
			if op.ColumnDef != nil {
				table.Columns = append(table.Columns, *op.ColumnDef)
			}
		case ddl.OpDropColumn:
			newColumns := make([]ddl.ColumnDefinition, 0, len(table.Columns))
			for _, col := range table.Columns {
				if col.Name != op.Column {
					newColumns = append(newColumns, col)
				}
			}
			table.Columns = newColumns
		case ddl.OpRenameColumn:
			for i, col := range table.Columns {
				if col.Name == op.Column {
					table.Columns[i].Name = op.NewName
					break
				}
			}
		case ddl.OpAddIndex:
			if op.IndexDef != nil {
				table.Indexes = append(table.Indexes, *op.IndexDef)
			}
		case ddl.OpDropIndex:
			newIndexes := make([]ddl.IndexDefinition, 0, len(table.Indexes))
			for _, idx := range table.Indexes {
				if idx.Name != op.IndexName {
					newIndexes = append(newIndexes, idx)
				}
			}
			table.Indexes = newIndexes
		case ddl.OpRenameIndex:
			for i, idx := range table.Indexes {
				if idx.Name == op.IndexName {
					table.Indexes[i].Name = op.NewName
					break
				}
			}
		case ddl.OpChangeType:
			for i, col := range table.Columns {
				if col.Name == op.Column {
					table.Columns[i].Type = op.NewType
					break
				}
			}
		case ddl.OpChangeNullable:
			if op.Nullable != nil {
				for i, col := range table.Columns {
					if col.Name == op.Column {
						table.Columns[i].Nullable = *op.Nullable
						break
					}
				}
			}
		case ddl.OpChangeDefault:
			for i, col := range table.Columns {
				if col.Name == op.Column {
					table.Columns[i].Default = op.Default
					break
				}
			}
		}
	}

	// Update the table in the schema
	m.Schema.Tables[tableName] = table

	// Generate SQL for each database
	// Note: SQLite needs the current table for potential table rebuild operations
	m.Migrations = append(m.Migrations, Migration{
		Name: fmt.Sprintf("alter_%s_table", tableName),
		Instructions: MigrationInstructions{
			Postgres: generatePostgresAlterTable(tableName, operations),
			MySQL:    generateMySQLAlterTable(tableName, operations),
			Sqlite:   generateSQLiteAlterTable(tableName, operations, &table),
		},
	})

	return nil
}

// DropTable removes a table from the schema and returns a new plan with the table removed.
func (m *MigrationPlan) DropTable(name string) (*MigrationPlan, error) {
	// Verify table exists
	if _, ok := m.Schema.Tables[name]; !ok {
		return nil, fmt.Errorf("table %q not found in schema", name)
	}

	// Delete from schema
	delete(m.Schema.Tables, name)

	// Generate SQL for each database
	m.Migrations = append(m.Migrations, Migration{
		Name: fmt.Sprintf("drop_%s_table", name),
		Instructions: MigrationInstructions{
			Postgres: generatePostgresDropTable(name),
			MySQL:    generateMySQLDropTable(name),
			Sqlite:   generateSQLiteDropTable(name),
		},
	})

	return m, nil
}
