// Package ref provides shared reference types used across the portsql packages.
// It exists to avoid circular imports between ddl and migrate packages.
package ref

// TableRef is a validated reference to a table in the schema.
// It is used to establish relationships between tables without
// creating actual foreign key constraints.
type TableRef struct {
	Name string // The table name
}

// TableName returns the referenced table's name.
func (t *TableRef) TableName() string {
	return t.Name
}
