package codegen

import (
	"fmt"
	"strings"
	"testing"
)

// mysqlParseDatabaseURL mirrors the logic embedded in parseDatabaseURLFuncMySQL
// so we can unit-test it directly. Any fix to the constant must be applied here
// too (and vice-versa) — the TestMySQLParseDatabaseURL_MatchesConstant test
// below ensures the constant contains the key code patterns.
func mysqlParseDatabaseURL(rawURL string) (driver, dsn string) {
	rest := rawURL
	if len(rest) >= 8 && rest[:8] == "mysql://" {
		rest = rest[8:]
	}
	atIdx := strings.Index(rest, "@")
	user := ""
	hostAndDB := rest
	if atIdx >= 0 {
		user = rest[:atIdx]
		hostAndDB = rest[atIdx+1:]
	}
	slashIdx := strings.Index(hostAndDB, "/")
	hostPort := hostAndDB
	dbName := ""
	if slashIdx >= 0 {
		hostPort = hostAndDB[:slashIdx]
		dbName = hostAndDB[slashIdx+1:]
	}
	// Separate existing query params from the database name so we can
	// merge parseTime=true without introducing a second '?'.
	queryParams := ""
	if qIdx := strings.Index(dbName, "?"); qIdx >= 0 {
		queryParams = dbName[qIdx+1:]
		dbName = dbName[:qIdx]
	}
	// parseTime=true is required so the driver scans DATETIME columns into time.Time.
	if queryParams == "" {
		queryParams = "parseTime=true"
	} else if !strings.Contains(queryParams, "parseTime=") {
		queryParams += "&parseTime=true"
	}
	return "mysql", fmt.Sprintf("%s@tcp(%s)/%s?%s", user, hostPort, dbName, queryParams)
}

func TestMySQLParseDatabaseURL(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantDSN string
	}{
		{
			name:    "basic URL without query params",
			input:   "mysql://root@localhost:3306/mydb",
			wantDSN: "root@tcp(localhost:3306)/mydb?parseTime=true",
		},
		{
			name:    "URL with user and password",
			input:   "mysql://admin:secret@db.example.com:3306/prod",
			wantDSN: "admin:secret@tcp(db.example.com:3306)/prod?parseTime=true",
		},
		{
			name:    "URL without port",
			input:   "mysql://root@localhost/mydb",
			wantDSN: "root@tcp(localhost)/mydb?parseTime=true",
		},
		{
			name:    "URL without mysql:// prefix",
			input:   "root@localhost:3306/mydb",
			wantDSN: "root@tcp(localhost:3306)/mydb?parseTime=true",
		},
		{
			name:    "regression: ssl-mode query param must not cause double ?",
			input:   "mysql://user:pass@host:3306/dbname?ssl-mode=REQUIRED",
			wantDSN: "user:pass@tcp(host:3306)/dbname?ssl-mode=REQUIRED&parseTime=true",
		},
		{
			name:    "regression: tls query param must not cause double ?",
			input:   "mysql://user@host:3306/db?tls=skip-verify",
			wantDSN: "user@tcp(host:3306)/db?tls=skip-verify&parseTime=true",
		},
		{
			name:    "multiple existing query params",
			input:   "mysql://user@host:3306/db?ssl-mode=REQUIRED&charset=utf8mb4",
			wantDSN: "user@tcp(host:3306)/db?ssl-mode=REQUIRED&charset=utf8mb4&parseTime=true",
		},
		{
			name:    "parseTime already set by user",
			input:   "mysql://user@host:3306/db?parseTime=true",
			wantDSN: "user@tcp(host:3306)/db?parseTime=true",
		},
		{
			name:    "parseTime=false set by user is preserved",
			input:   "mysql://user@host:3306/db?parseTime=false",
			wantDSN: "user@tcp(host:3306)/db?parseTime=false",
		},
		{
			name:    "parseTime among other params",
			input:   "mysql://user@host:3306/db?charset=utf8mb4&parseTime=true&loc=UTC",
			wantDSN: "user@tcp(host:3306)/db?charset=utf8mb4&parseTime=true&loc=UTC",
		},
		{
			name:    "no user",
			input:   "mysql://localhost:3306/mydb",
			wantDSN: "@tcp(localhost:3306)/mydb?parseTime=true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			driver, dsn := mysqlParseDatabaseURL(tt.input)
			if driver != "mysql" {
				t.Errorf("driver = %q, want %q", driver, "mysql")
			}
			if dsn != tt.wantDSN {
				t.Errorf("dsn = %q, want %q", dsn, tt.wantDSN)
			}

			// Verify there is never a double '?' in the DSN.
			if strings.Count(dsn, "?") > 1 {
				t.Errorf("DSN contains multiple '?' characters: %q", dsn)
			}
		})
	}
}

// TestMySQLParseDatabaseURL_NoDoubleQuestionMark is a focused regression test
// for the exact production error:
//
//	Error 1064 (42000): ... 'ssl-mode = REQUIRED?parseTime=true'
//
// The root cause was that existing query parameters were kept as part of the
// database name, and then "?parseTime=true" was appended, producing a second
// '?' in the DSN.
func TestMySQLParseDatabaseURL_NoDoubleQuestionMark(t *testing.T) {
	inputs := []string{
		"mysql://user:pass@host:3306/db?ssl-mode=REQUIRED",
		"mysql://user@host:3306/db?tls=skip-verify",
		"mysql://user@host:3306/db?charset=utf8mb4&timeout=5s",
		"mysql://user@host:3306/db?parseTime=true&ssl-mode=REQUIRED",
	}
	for _, input := range inputs {
		_, dsn := mysqlParseDatabaseURL(input)
		if count := strings.Count(dsn, "?"); count != 1 {
			t.Errorf("mysqlParseDatabaseURL(%q): DSN has %d '?' chars (want exactly 1): %q",
				input, count, dsn)
		}
	}
}

// TestMySQLParseDatabaseURL_MatchesConstant verifies that the string constant
// parseDatabaseURLFuncMySQL (and its exported twin) contain the key code
// patterns from our fix, so the constant and this test's mirror function
// stay in sync.
func TestMySQLParseDatabaseURL_MatchesConstant(t *testing.T) {
	unexported := ParseDatabaseURLFuncForDialect("mysql")
	exported := ExportedParseDatabaseURLFuncForDialect("mysql")

	requiredPatterns := []string{
		// Must strip query params from dbName before formatting.
		`strings.Index(dbName, "?")`,
		// Must use & separator when appending to existing params.
		`"&parseTime=true"`,
		// Must guard against adding parseTime when it already exists.
		`strings.Contains(queryParams, "parseTime=")`,
	}

	for _, pattern := range requiredPatterns {
		if !strings.Contains(unexported, pattern) {
			t.Errorf("parseDatabaseURLFuncMySQL is missing pattern %q", pattern)
		}
		if !strings.Contains(exported, pattern) {
			t.Errorf("exportedParseDatabaseURLFuncMySQL is missing pattern %q", pattern)
		}
	}

	// Must NOT contain the old buggy line that blindly appends ?parseTime=true.
	buggyLine := `/%s?parseTime=true"`
	if strings.Contains(unexported, buggyLine) {
		t.Errorf("parseDatabaseURLFuncMySQL still contains buggy pattern %q", buggyLine)
	}
	if strings.Contains(exported, buggyLine) {
		t.Errorf("exportedParseDatabaseURLFuncMySQL still contains buggy pattern %q", buggyLine)
	}
}
