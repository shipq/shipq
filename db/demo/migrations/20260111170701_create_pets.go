package migrations

import (
	"github.com/shipq/shipq/db/portsql/ddl"
	"github.com/shipq/shipq/db/portsql/migrate"
)

func Migrate_20260111170701_create_pets(plan *migrate.MigrationPlan) error {
	// Get reference to categories table (must exist from previous migration)
	categories, err := plan.Table("categories")
	if err != nil {
		return err
	}

	_, err = plan.AddEmptyTable("pets", func(tb *ddl.TableBuilder) error {
		tb.Bigint("id").PrimaryKey()
		tb.Bigint("category_id").Indexed().References(categories)
		tb.String("name")
		tb.JSON("photo_urls")
		tb.String("status").Nullable()
		return nil
	})
	return err
}
