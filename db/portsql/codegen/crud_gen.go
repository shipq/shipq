package codegen

// CRUDOptions configures CRUD generation behavior.
type CRUDOptions struct {
	// ScopeColumn, if set, adds this column to WHERE clauses.
	// The column must exist in the table.
	// Example: "organization_id", "tenant_id", "user_id"
	ScopeColumn string

	// OrderAsc, if true, orders by created_at ASC (oldest first).
	// Default is false (newest first, DESC).
	OrderAsc bool
}
