package migrations

import (
	"github.com/portsql/portsql/src/ddl"
	"github.com/portsql/portsql/src/migrate"
)

func Migrate_20260111170701_create_pets(plan *migrate.MigrationPlan) error {
	_, err := plan.AddEmptyTable("pets", func(tb *ddl.TableBuilder) error {
		tb.Bigint("id").PrimaryKey()
		tb.Bigint("category_id").Indexed()
		tb.String("name")
		tb.JSON("photo_urls")
		tb.String("status").Nullable()
		return nil
	})
	return err
}
