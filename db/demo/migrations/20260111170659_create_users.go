package migrations

import (
	"github.com/shipq/shipq/db/portsql/ddl"
	"github.com/shipq/shipq/db/portsql/migrate"
)

// Migrate_20260111170659_create_users demonstrates AddTable which includes
// standard columns: id, public_id, created_at, deleted_at, updated_at
func Migrate_20260111170659_create_users(plan *migrate.MigrationPlan) error {
	_, err := plan.AddTable("users", func(tb *ddl.TableBuilder) error {
		tb.String("username").Unique()
		tb.String("first_name")
		tb.String("last_name")
		tb.String("email")
		tb.String("password")
		tb.String("phone").Nullable()
		tb.Integer("user_status")
		return nil
	})
	return err
}
