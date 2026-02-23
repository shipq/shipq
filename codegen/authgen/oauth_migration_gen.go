package authgen

import (
	"fmt"
	"os"
	"strings"
)

// GenerateOAuthAccountsMigration generates a migration that creates the
// oauth_accounts table. This table links OAuth provider identities to
// accounts. Columns: account_id (FK to accounts), provider, provider_user_id,
// email, avatar_url (nullable). A unique index on (provider, provider_user_id)
// prevents duplicate links.
func GenerateOAuthAccountsMigration(timestamp, modulePath string) []byte {
	return []byte(fmt.Sprintf(`package migrations

import (
	"%s/shipq/lib/db/portsql/ddl"
	"%s/shipq/lib/db/portsql/migrate"
)

func Migrate_%s_oauth_accounts(plan *migrate.MigrationPlan) error {
	accountsRef, err := plan.Table("accounts")
	if err != nil {
		return err
	}

	_, err = plan.AddTable("oauth_accounts", func(tb *ddl.TableBuilder) error {
		tb.Bigint("account_id").References(accountsRef)
		providerCol := tb.String("provider").Col()
		providerUserIDCol := tb.String("provider_user_id").Col()
		tb.String("email")
		tb.String("avatar_url").Nullable()
		tb.AddUniqueIndex(providerCol, providerUserIDCol)
		return nil
	})
	return err
}
`, modulePath, modulePath, timestamp))
}

// OAuthMigrationsExist checks whether the oauth_accounts migration file
// already exists in the migrations directory. This prevents duplicate
// migration generation when running `shipq auth <provider>` multiple times.
func OAuthMigrationsExist(migrationsPath string) bool {
	entries, err := os.ReadDir(migrationsPath)
	if err != nil {
		return false
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(entry.Name(), "_oauth_accounts.go") {
			return true
		}
	}
	return false
}
