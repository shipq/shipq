package migrate

import "github.com/portsql/portsql/src/ddl"

func Migrate(plan MigrationPlan) error {
	plan.AddEmptyTable("users", func(tb *ddl.TableBuilder) error {
		tb.Bigint("id").PrimaryKey()
		tb.String("name")
		tb.String("email").Unique()
		tb.String("password")
		tb.Datetime("created_at").Indexed()
		tb.Datetime("updated_at")
		tb.Datetime("deleted_at")
		return nil
	})
	return nil
}
