package codegen

// CRUDOptions configures CRUD generation behavior.
type CRUDOptions struct {
	// ScopeColumn, if set, adds this column to WHERE clauses.
	// The column must exist in the table.
	// Example: "organization_id", "tenant_id", "user_id"
	ScopeColumn string
}
