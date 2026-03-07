package shared

import (
	"os"
)

// AuthMigrationSuffixes are the file suffixes used to detect existing auth migrations.
var AuthMigrationSuffixes = []string{
	"_organizations.go",
	"_accounts.go",
	"_organization_users.go",
	"_sessions.go",
	"_roles.go",
	"_account_roles.go",
	"_role_actions.go",
}

// MigrationsExist checks whether migration files matching the given suffixes
// exist in the directory. When requireAll is true, every suffix must be found;
// when false, any single match is sufficient.
func MigrationsExist(dir string, suffixes []string, requireAll bool) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}

	found := make(map[string]bool)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		for _, suffix := range suffixes {
			if len(name) > len(suffix) && name[len(name)-len(suffix):] == suffix {
				found[suffix] = true
				if !requireAll {
					// Any match is enough.
					return true
				}
			}
		}
	}

	if requireAll {
		return len(found) == len(suffixes)
	}
	return false
}

// AuthMigrationsExist checks if all auth migration files already exist in the
// migrations directory. This prevents duplicate migration generation when
// running `shipq auth` multiple times.
func AuthMigrationsExist(migrationsPath string) bool {
	return MigrationsExist(migrationsPath, AuthMigrationSuffixes, true)
}
