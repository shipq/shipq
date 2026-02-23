package channelgen

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestGenerateJobResultsMigration_WithoutTenancy(t *testing.T) {
	code := GenerateJobResultsMigration("20260615120000", "example.com/myapp", false)
	codeStr := string(code)

	// Should have package declaration
	if !strings.Contains(codeStr, "package migrations") {
		t.Error("expected 'package migrations' in generated code")
	}

	// Should have correct function name with timestamp
	if !strings.Contains(codeStr, "func Migrate_20260615120000_job_results") {
		t.Error("expected function name with timestamp")
	}

	// Should have channel_name column
	if !strings.Contains(codeStr, `"channel_name"`) {
		t.Error("expected channel_name column")
	}

	// Should have account_id nullable column
	if !strings.Contains(codeStr, `"account_id"`) {
		t.Error("expected account_id column")
	}
	if !strings.Contains(codeStr, "Nullable()") {
		t.Error("expected Nullable() call for account_id")
	}

	// Should NOT have organization_id column
	if strings.Contains(codeStr, `"organization_id"`) {
		t.Error("expected NO organization_id column when hasTenancy is false")
	}

	// Should have status column with default
	if !strings.Contains(codeStr, `"status"`) {
		t.Error("expected status column")
	}
	if !strings.Contains(codeStr, `Default("pending")`) {
		t.Error("expected status default 'pending'")
	}

	// Should have request_payload JSON column
	if !strings.Contains(codeStr, `"request_payload"`) {
		t.Error("expected request_payload column")
	}
	if !strings.Contains(codeStr, "JSON(") {
		t.Error("expected JSON column type for request_payload")
	}

	// Should have result_payload nullable JSON column
	if !strings.Contains(codeStr, `"result_payload"`) {
		t.Error("expected result_payload column")
	}

	// Should have error_message text column
	if !strings.Contains(codeStr, `"error_message"`) {
		t.Error("expected error_message column")
	}
	if !strings.Contains(codeStr, "Text(") {
		t.Error("expected Text column type for error_message")
	}

	// Should have started_at and completed_at datetime columns
	if !strings.Contains(codeStr, `"started_at"`) {
		t.Error("expected started_at column")
	}
	if !strings.Contains(codeStr, `"completed_at"`) {
		t.Error("expected completed_at column")
	}
	if !strings.Contains(codeStr, "Datetime(") {
		t.Error("expected Datetime column type")
	}

	// Should have retry_count int column with default
	if !strings.Contains(codeStr, `"retry_count"`) {
		t.Error("expected retry_count column")
	}
	if !strings.Contains(codeStr, `Default(0)`) {
		t.Error("expected retry_count default 0")
	}

	// Should import correct module paths
	if !strings.Contains(codeStr, `"example.com/myapp/shipq/lib/db/portsql/ddl"`) {
		t.Error("expected ddl import with correct module path")
	}
	if !strings.Contains(codeStr, `"example.com/myapp/shipq/lib/db/portsql/migrate"`) {
		t.Error("expected migrate import with correct module path")
	}
}

func TestGenerateJobResultsMigration_WithTenancy(t *testing.T) {
	code := GenerateJobResultsMigration("20260615120000", "example.com/myapp", true)
	codeStr := string(code)

	// Should have organization_id column
	if !strings.Contains(codeStr, `"organization_id"`) {
		t.Error("expected organization_id column when hasTenancy is true")
	}

	// Should have account_id column
	if !strings.Contains(codeStr, `"account_id"`) {
		t.Error("expected account_id column")
	}

	// Both should be nullable
	// Count Nullable() calls - should be at least 2 (account_id and organization_id)
	nullableCount := strings.Count(codeStr, "Nullable()")
	if nullableCount < 2 {
		t.Errorf("expected at least 2 Nullable() calls (account_id + organization_id), got %d", nullableCount)
	}
}

func TestGenerateJobResultsMigration_ValidGo(t *testing.T) {
	tests := []struct {
		name       string
		hasTenancy bool
	}{
		{"without tenancy", false},
		{"with tenancy", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := GenerateJobResultsMigration("20260615120000", "example.com/myapp", tt.hasTenancy)

			fset := token.NewFileSet()
			_, err := parser.ParseFile(fset, "migration.go", code, parser.AllErrors)
			if err != nil {
				t.Errorf("generated migration code is not valid Go: %v\ncode:\n%s", err, string(code))
			}
		})
	}
}

func TestGenerateJobResultsMigration_DifferentModulePaths(t *testing.T) {
	modulePaths := []string{
		"myapp",
		"github.com/company/project",
		"github.com/company/monorepo/services/api",
	}

	for _, mp := range modulePaths {
		t.Run(mp, func(t *testing.T) {
			code := GenerateJobResultsMigration("20260615120000", mp, false)
			codeStr := string(code)

			expectedDDL := mp + "/shipq/lib/db/portsql/ddl"
			expectedMigrate := mp + "/shipq/lib/db/portsql/migrate"

			if !strings.Contains(codeStr, expectedDDL) {
				t.Errorf("expected ddl import %q in generated code", expectedDDL)
			}
			if !strings.Contains(codeStr, expectedMigrate) {
				t.Errorf("expected migrate import %q in generated code", expectedMigrate)
			}
		})
	}
}

func TestGenerateJobResultsMigration_DifferentTimestamps(t *testing.T) {
	timestamps := []string{
		"20260101000000",
		"20261231235959",
		"20270615120000",
	}

	for _, ts := range timestamps {
		t.Run(ts, func(t *testing.T) {
			code := GenerateJobResultsMigration(ts, "example.com/myapp", false)
			codeStr := string(code)

			expectedFunc := "func Migrate_" + ts + "_job_results"
			if !strings.Contains(codeStr, expectedFunc) {
				t.Errorf("expected function %q in generated code", expectedFunc)
			}
		})
	}
}

func TestGenerateJobResultsMigration_UsesAddTable(t *testing.T) {
	code := GenerateJobResultsMigration("20260615120000", "example.com/myapp", false)
	codeStr := string(code)

	if !strings.Contains(codeStr, `plan.AddTable("job_results"`) {
		t.Error("expected plan.AddTable(\"job_results\", ...) call")
	}
}
