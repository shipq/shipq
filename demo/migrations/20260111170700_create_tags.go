package migrations

import (
	"github.com/portsql/portsql/src/ddl"
	"github.com/portsql/portsql/src/migrate"
)

func Migrate_20260111170700_create_tags(plan *migrate.MigrationPlan) error {
	_, err := plan.AddEmptyTable("tags", func(tb *ddl.TableBuilder) error {
		tb.Bigint("id").PrimaryKey()
		tb.String("name")
		return nil
	})
	return err
}
