package files

import "fmt"

func generateManagedFilesMigration(timestamp, modulePath string) []byte {
	return []byte(fmt.Sprintf(`package migrations

import (
	"%s/shipq/lib/db/portsql/ddl"
	"%s/shipq/lib/db/portsql/migrate"
)

func Migrate_%s_managed_files(plan *migrate.MigrationPlan) error {
	// author_account_id is auto-added by AddTable when accounts table exists
	_, err := plan.AddTable("managed_files", func(tb *ddl.TableBuilder) error {
		tb.String("file_key").Unique()
		tb.String("original_name")
		tb.String("content_type")
		tb.Bigint("size_bytes")
		tb.String("status")
		tb.String("visibility")
		tb.String("s3_upload_id").Nullable()
		return nil
	})
	return err
}
`, modulePath, modulePath, timestamp))
}

func generateFileAccessMigration(timestamp, modulePath string) []byte {
	return []byte(fmt.Sprintf(`package migrations

import (
	"%s/shipq/lib/db/portsql/ddl"
	"%s/shipq/lib/db/portsql/migrate"
)

func Migrate_%s_file_access(plan *migrate.MigrationPlan) error {
	managedFilesRef, err := plan.Table("managed_files")
	if err != nil {
		return err
	}

	accountsRef, err := plan.Table("accounts")
	if err != nil {
		return err
	}

	_, err = plan.AddTable("file_access", func(tb *ddl.TableBuilder) error {
		fileIDCol := tb.Bigint("managed_file_id").References(managedFilesRef).Col()
		accountIDCol := tb.Bigint("account_id").References(accountsRef).Col()
		tb.String("role")
		tb.AddUniqueIndex(fileIDCol, accountIDCol)
		tb.JunctionTable()
		return nil
	})
	return err
}
`, modulePath, modulePath, timestamp))
}
