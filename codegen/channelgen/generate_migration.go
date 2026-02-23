package channelgen

import "fmt"

// GenerateJobResultsMigration generates a migration file that creates the job_results table.
// The generated migration follows the same pattern as the files command's migration generation
// (see internal/commands/files/migrations.go).
//
// Parameters:
//   - timestamp: the migration timestamp (e.g., "20260615120000")
//   - modulePath: the user's Go module path (e.g., "myapp")
//   - hasTenancy: if true, adds an organization_id column that references organizations
//
// Returns the generated Go source code for the migration file.
func GenerateJobResultsMigration(timestamp, modulePath string, hasTenancy bool) []byte {
	orgColumn := ""
	if hasTenancy {
		orgColumn = `		tb.Bigint("organization_id").Nullable()
`
	}

	return []byte(fmt.Sprintf(`package migrations

import (
	"%s/shipq/lib/db/portsql/ddl"
	"%s/shipq/lib/db/portsql/migrate"
)

func Migrate_%s_job_results(plan *migrate.MigrationPlan) error {
	_, err := plan.AddTable("job_results", func(tb *ddl.TableBuilder) error {
		tb.String("channel_name")
		tb.Bigint("account_id").Nullable()
%s		tb.String("status").Default("pending")
		tb.JSON("request_payload")
		tb.JSON("result_payload").Nullable()
		tb.Text("error_message").Nullable()
		tb.Datetime("started_at").Nullable()
		tb.Datetime("completed_at").Nullable()
		tb.Integer("retry_count").Default(0)
		return nil
	})
	return err
}
`, modulePath, modulePath, timestamp, orgColumn))
}
