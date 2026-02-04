package migrations

import (
	"github.com/shipq/shipq/db/portsql/ddl"
	"github.com/shipq/shipq/db/portsql/migrate"
)

func Migrate_20260204134211_accounts(plan *migrate.MigrationPlan) error {
	_, err := plan.AddTable("accounts", func(tb *ddl.TableBuilder) error {
		tb.String("first_name")
		tb.String("last_name")
		tb.String("email").Unique()
		return nil
	})
	return err
}
