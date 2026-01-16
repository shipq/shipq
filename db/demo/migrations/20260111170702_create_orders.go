package migrations

import (
	"github.com/shipq/shipq/db/portsql/ddl"
	"github.com/shipq/shipq/db/portsql/migrate"
)

func Migrate_20260111170702_create_orders(plan *migrate.MigrationPlan) error {
	_, err := plan.AddEmptyTable("orders", func(tb *ddl.TableBuilder) error {
		tb.Bigint("id").PrimaryKey()
		tb.Bigint("pet_id").Indexed()
		tb.Integer("quantity")
		tb.Datetime("ship_date").Nullable()
		tb.String("status").Nullable()
		tb.Bool("complete").Default(false)
		return nil
	})
	return err
}
