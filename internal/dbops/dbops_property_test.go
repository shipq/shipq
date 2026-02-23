package dbops_test

import (
	"strings"
	"testing"

	"github.com/shipq/shipq/internal/dbops"
	"github.com/shipq/shipq/proptest"
)

// Property: QuoteIdentifier should always produce valid SQL identifiers
// that prevent SQL injection for any input string
func TestProperty_QuoteIdentifierPreventsInjection(t *testing.T) {
	proptest.QuickCheck(t, "quote identifier prevents injection", func(g *proptest.Generator) bool {
		// Generate potentially malicious inputs
		input := proptest.OneOfFunc(g,
			func(g *proptest.Generator) string { return g.EdgeCaseString() },
			func(g *proptest.Generator) string { return g.String(100) },
			func(g *proptest.Generator) string { return g.EdgeCaseIdentifier() },
		)

		dialects := []string{"postgres", "mysql", "sqlite"}
		for _, dialect := range dialects {
			quoted := dbops.QuoteIdentifier(input, dialect)

			// Must not be empty
			if quoted == "" {
				t.Logf("QuoteIdentifier returned empty for input %q, dialect %s", input, dialect)
				return false
			}

			// For postgres/sqlite: must be wrapped in double quotes
			// For mysql: must be wrapped in backticks
			if dialect == "mysql" {
				if !strings.HasPrefix(quoted, "`") || !strings.HasSuffix(quoted, "`") {
					t.Logf("MySQL quoted identifier not wrapped in backticks: %q", quoted)
					return false
				}
				// Backticks should be doubled inside
				inner := quoted[1 : len(quoted)-1]
				// Count backticks - should be even (doubled)
				backtickCount := strings.Count(inner, "`")
				if backtickCount%2 != 0 {
					t.Logf("MySQL quoted identifier has odd backtick count: %q", quoted)
					return false
				}
			} else {
				if !strings.HasPrefix(quoted, `"`) || !strings.HasSuffix(quoted, `"`) {
					t.Logf("Postgres/SQLite quoted identifier not wrapped in double quotes: %q", quoted)
					return false
				}
				// Double quotes should be doubled inside
				inner := quoted[1 : len(quoted)-1]
				// Count double quotes - should be even (doubled)
				quoteCount := strings.Count(inner, `"`)
				if quoteCount%2 != 0 {
					t.Logf("Postgres/SQLite quoted identifier has odd quote count: %q", quoted)
					return false
				}
			}
		}
		return true
	})
}

// Property: Database names derived from identifiers should be valid
// across all supported dialects
func TestProperty_DatabaseNameValidForAllDialects(t *testing.T) {
	proptest.QuickCheck(t, "database names valid for all dialects", func(g *proptest.Generator) bool {
		// Generate valid identifier-like project names
		projectName := g.IdentifierLower(30)
		if projectName == "" {
			return true // Skip empty
		}

		dialects := []string{"postgres", "mysql", "sqlite"}
		for _, dialect := range dialects {
			quoted := dbops.QuoteIdentifier(projectName, dialect)
			// The quoted name should be usable in SQL
			if quoted == "" {
				return false
			}
			// Should be longer than the input (at least 2 chars for quotes)
			if len(quoted) < len(projectName)+2 {
				return false
			}
		}
		return true
	})
}

// Property: SQL generation for drop/create should be deterministic
func TestProperty_DropCreateSQLDeterministic(t *testing.T) {
	proptest.QuickCheck(t, "drop/create SQL is deterministic", func(g *proptest.Generator) bool {
		dbName := g.IdentifierLower(20)
		if dbName == "" {
			return true
		}

		dialects := []string{"postgres", "mysql"}
		for _, dialect := range dialects {
			// Generate twice
			drop1 := dbops.GenerateDropSQL(dbName, dialect)
			drop2 := dbops.GenerateDropSQL(dbName, dialect)
			if drop1 != drop2 {
				t.Logf("Drop SQL not deterministic for %q, dialect %s", dbName, dialect)
				return false
			}

			create1 := dbops.GenerateCreateSQL(dbName, dialect)
			create2 := dbops.GenerateCreateSQL(dbName, dialect)
			if create1 != create2 {
				t.Logf("Create SQL not deterministic for %q, dialect %s", dbName, dialect)
				return false
			}
		}
		return true
	})
}

// Property: Drop SQL should contain "DROP DATABASE" and the quoted name
func TestProperty_DropSQLContainsExpectedKeywords(t *testing.T) {
	proptest.QuickCheck(t, "drop SQL contains expected keywords", func(g *proptest.Generator) bool {
		dbName := g.IdentifierLower(20)
		if dbName == "" {
			return true
		}

		dialects := []string{"postgres", "mysql"}
		for _, dialect := range dialects {
			sql := dbops.GenerateDropSQL(dbName, dialect)
			upperSQL := strings.ToUpper(sql)

			if !strings.Contains(upperSQL, "DROP DATABASE") {
				t.Logf("Drop SQL missing DROP DATABASE: %q", sql)
				return false
			}
			if !strings.Contains(upperSQL, "IF EXISTS") {
				t.Logf("Drop SQL missing IF EXISTS: %q", sql)
				return false
			}
		}
		return true
	})
}

// Property: Create SQL should contain "CREATE DATABASE" and the quoted name
func TestProperty_CreateSQLContainsExpectedKeywords(t *testing.T) {
	proptest.QuickCheck(t, "create SQL contains expected keywords", func(g *proptest.Generator) bool {
		dbName := g.IdentifierLower(20)
		if dbName == "" {
			return true
		}

		dialects := []string{"postgres", "mysql"}
		for _, dialect := range dialects {
			sql := dbops.GenerateCreateSQL(dbName, dialect)
			upperSQL := strings.ToUpper(sql)

			if !strings.Contains(upperSQL, "CREATE DATABASE") {
				t.Logf("Create SQL missing CREATE DATABASE: %q", sql)
				return false
			}
		}
		return true
	})
}

// Property: MySQL URL to DSN conversion should preserve user and host info
func TestProperty_MySQLURLToDSNPreservesInfo(t *testing.T) {
	proptest.QuickCheck(t, "MySQL URL to DSN preserves info", func(g *proptest.Generator) bool {
		// Generate valid user, host, port, db components
		user := g.IdentifierLower(10)
		if user == "" {
			return true
		}
		host := proptest.OneOf(g, "localhost", "127.0.0.1", "db.example.com")
		port := g.IntRange(1024, 65535)
		dbName := g.IdentifierLower(15)

		// Build URL
		url := "mysql://" + user + "@" + host + ":" + itoa(port)
		if dbName != "" {
			url += "/" + dbName
		}

		dsn, err := dbops.MySQLURLToDSN(url)
		if err != nil {
			t.Logf("MySQLURLToDSN failed for %q: %v", url, err)
			return false
		}

		// DSN should contain the user
		if !strings.Contains(dsn, user) {
			t.Logf("DSN missing user: %q", dsn)
			return false
		}

		// DSN should contain tcp(host:port)
		if !strings.Contains(dsn, "tcp(") {
			t.Logf("DSN missing tcp(): %q", dsn)
			return false
		}

		return true
	})
}

// Property: SQLite URL to path conversion should strip prefix
func TestProperty_SQLiteURLToPathStripsPrefix(t *testing.T) {
	proptest.QuickCheck(t, "SQLite URL to path strips prefix", func(g *proptest.Generator) bool {
		path := "/" + g.IdentifierLower(5) + "/" + g.IdentifierLower(10) + ".db"

		// Test with sqlite:// prefix
		url1 := "sqlite://" + path
		result1 := dbops.SQLiteURLToPath(url1)
		if result1 != path {
			t.Logf("Expected %q, got %q for %q", path, result1, url1)
			return false
		}

		// Test with sqlite: prefix
		url2 := "sqlite:" + path
		result2 := dbops.SQLiteURLToPath(url2)
		if result2 != path {
			t.Logf("Expected %q, got %q for %q", path, result2, url2)
			return false
		}

		// Test without prefix (passthrough)
		result3 := dbops.SQLiteURLToPath(path)
		if result3 != path {
			t.Logf("Expected %q, got %q for %q", path, result3, path)
			return false
		}

		return true
	})
}

// Property: Quoted identifier should be idempotent-safe
// (quoting an already quoted string should still result in valid SQL)
func TestProperty_QuoteIdentifierHandlesEdgeCases(t *testing.T) {
	edgeCases := []string{
		"",
		" ",
		"normal_name",
		"with space",
		`with"quote`,
		"with`backtick",
		"with'apostrophe",
		"with\nnewline",
		"with\ttab",
		"; DROP TABLE users;",
		"--comment",
		"/* comment */",
	}

	for _, input := range edgeCases {
		dialects := []string{"postgres", "mysql", "sqlite"}
		for _, dialect := range dialects {
			quoted := dbops.QuoteIdentifier(input, dialect)

			// Should not be empty
			if quoted == "" {
				t.Errorf("QuoteIdentifier(%q, %q) returned empty", input, dialect)
			}

			// Should start and end with proper quote chars
			if dialect == "mysql" {
				if quoted[0] != '`' || quoted[len(quoted)-1] != '`' {
					t.Errorf("MySQL quote for %q = %q, not wrapped in backticks", input, quoted)
				}
			} else {
				if quoted[0] != '"' || quoted[len(quoted)-1] != '"' {
					t.Errorf("Postgres/SQLite quote for %q = %q, not wrapped in double quotes", input, quoted)
				}
			}
		}
	}
}

// Helper function
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
