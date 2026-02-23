package codegen

import (
	"testing"

	"github.com/shipq/shipq/db/portsql/ddl"
	"github.com/shipq/shipq/db/portsql/migrate"
)

// Helper to wrap a table in a migration plan for testing
func tableToMigrationPlan(table ddl.Table) *migrate.MigrationPlan {
	return &migrate.MigrationPlan{
		Schema: migrate.Schema{
			Name: "test",
			Tables: map[string]ddl.Table{
				table.Name: table,
			},
		},
	}
}

func TestToSingular(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"authors", "author"},
		{"posts", "post"},
		{"categories", "category"},
		{"addresses", "address"}, // "es" suffix removed
		{"users", "user"},
		{"data", "data"}, // No change for words not ending in s/es/ies
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := toSingular(tt.input)
			if got != tt.want {
				t.Errorf("toSingular(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
