package authgen

import (
	"fmt"
	"os"
	"strings"
)

// GenerateSentEmailsMigration generates a migration that creates the
// sent_emails table. This table logs every email the system sends, with
// sensitive tokens redacted in the stored html_body. Columns: to_email,
// subject, html_body (redacted copy), status ("sent" or "failed"),
// error_message (nullable).
func GenerateSentEmailsMigration(timestamp, modulePath string) []byte {
	return []byte(fmt.Sprintf(`package migrations

import (
	"%s/shipq/lib/db/portsql/ddl"
	"%s/shipq/lib/db/portsql/migrate"
)

func Migrate_%s_sent_emails(plan *migrate.MigrationPlan) error {
	_, err := plan.AddTable("sent_emails", func(tb *ddl.TableBuilder) error {
		tb.String("to_email")
		tb.String("subject")
		tb.Text("html_body")
		tb.String("status")
		tb.Text("error_message").Nullable()
		return nil
	})
	return err
}
`, modulePath, modulePath, timestamp))
}

// GenerateAccountsVerifiedMigration generates a migration that adds a
// "verified" boolean column to the accounts table. Default is false.
// Existing accounts will need to be marked verified manually or via a
// data migration.
func GenerateAccountsVerifiedMigration(timestamp, modulePath string) []byte {
	return []byte(fmt.Sprintf(`package migrations

import (
	"%s/shipq/lib/db/portsql/ddl"
	"%s/shipq/lib/db/portsql/migrate"
)

func Migrate_%s_accounts_verified(plan *migrate.MigrationPlan) error {
	return plan.UpdateTable("accounts", func(ab *ddl.AlterTableBuilder) error {
		ab.AddBoolean("verified").Default(false)
		return nil
	})
}
`, modulePath, modulePath, timestamp))
}

// GeneratePasswordResetTokensMigration generates a migration that creates the
// password_reset_tokens table. Stores SHA-256 hashed tokens (not raw tokens)
// with an expiry timestamp and a used flag. References accounts via account_id.
func GeneratePasswordResetTokensMigration(timestamp, modulePath string) []byte {
	return []byte(fmt.Sprintf(`package migrations

import (
	"%s/shipq/lib/db/portsql/ddl"
	"%s/shipq/lib/db/portsql/migrate"
)

func Migrate_%s_password_reset_tokens(plan *migrate.MigrationPlan) error {
	accountsRef, err := plan.Table("accounts")
	if err != nil {
		return err
	}

	_, err = plan.AddTable("password_reset_tokens", func(tb *ddl.TableBuilder) error {
		tb.Bigint("account_id").References(accountsRef)
		tb.String("token_hash")
		tb.Datetime("expires_at")
		tb.Boolean("used").Default(false)
		return nil
	})
	return err
}
`, modulePath, modulePath, timestamp))
}

// GenerateEmailVerificationTokensMigration generates a migration that creates
// the email_verification_tokens table. Same pattern as password reset tokens:
// stores a SHA-256 hash of the token, not the raw token.
func GenerateEmailVerificationTokensMigration(timestamp, modulePath string) []byte {
	return []byte(fmt.Sprintf(`package migrations

import (
	"%s/shipq/lib/db/portsql/ddl"
	"%s/shipq/lib/db/portsql/migrate"
)

func Migrate_%s_email_verification_tokens(plan *migrate.MigrationPlan) error {
	accountsRef, err := plan.Table("accounts")
	if err != nil {
		return err
	}

	_, err = plan.AddTable("email_verification_tokens", func(tb *ddl.TableBuilder) error {
		tb.Bigint("account_id").References(accountsRef)
		tb.String("token_hash")
		tb.Datetime("expires_at")
		tb.Boolean("used").Default(false)
		return nil
	})
	return err
}
`, modulePath, modulePath, timestamp))
}

// emailMigrationSuffixes are the file suffixes used to detect existing email migrations.
var emailMigrationSuffixes = []string{
	"_sent_emails.go",
	"_accounts_verified.go",
	"_password_reset_tokens.go",
	"_email_verification_tokens.go",
}

// EmailMigrationsExist checks whether any email migration files already exist
// in the migrations directory. This prevents duplicate migration generation
// when running `shipq email` multiple times.
func EmailMigrationsExist(migrationsPath string) bool {
	entries, err := os.ReadDir(migrationsPath)
	if err != nil {
		return false
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		for _, suffix := range emailMigrationSuffixes {
			if strings.HasSuffix(name, suffix) {
				return true
			}
		}
	}
	return false
}
