package migrations

import (
	"github.com/portsql/portsql/src/ddl"
	"github.com/portsql/portsql/src/migrate"
)

func Migrate_20260111170656_create_categories(plan *migrate.MigrationPlan) error {
	_, err := plan.AddEmptyTable("categories", func(tb *ddl.TableBuilder) error {
		tb.Bigint("id").PrimaryKey()
		tb.String("name")
		return nil
	})
	return err
}
