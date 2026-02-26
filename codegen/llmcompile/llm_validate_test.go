package llmcompile

import (
	"strings"
	"testing"

	"github.com/shipq/shipq/inifile"
)

func parseTestINI(t *testing.T, content string) *inifile.File {
	t.Helper()
	f, err := inifile.Parse(strings.NewReader(content))
	if err != nil {
		t.Fatalf("failed to parse INI: %v", err)
	}
	return f
}

func TestValidatePrerequisites_AllPresent(t *testing.T) {
	ini := parseTestINI(t, `
[db]
database_url = postgres://localhost:5432/myapp

[workers]
redis_url = redis://localhost:6379
centrifugo_api_url = http://localhost:8000/api
centrifugo_api_key = secret
`)

	err := ValidatePrerequisites(ini)
	if err != nil {
		t.Fatalf("expected no error when all prerequisites met, got: %v", err)
	}
}

func TestValidatePrerequisites_MissingWorkers(t *testing.T) {
	ini := parseTestINI(t, `
[db]
database_url = postgres://localhost:5432/myapp
`)

	err := ValidatePrerequisites(ini)
	if err == nil {
		t.Fatal("expected error when [workers] section is missing")
	}
	if !strings.Contains(err.Error(), "channel workers") {
		t.Errorf("expected error to mention channel workers, got: %v", err)
	}
	if !strings.Contains(err.Error(), "shipq workers") {
		t.Errorf("expected error to mention 'shipq workers' command, got: %v", err)
	}
}

func TestValidatePrerequisites_MissingRedis(t *testing.T) {
	ini := parseTestINI(t, `
[db]
database_url = postgres://localhost:5432/myapp

[workers]
centrifugo_api_url = http://localhost:8000/api
`)

	err := ValidatePrerequisites(ini)
	if err == nil {
		t.Fatal("expected error when redis_url is missing")
	}
	if !strings.Contains(err.Error(), "Redis") {
		t.Errorf("expected error to mention Redis, got: %v", err)
	}
}

func TestValidatePrerequisites_MissingCentrifugo(t *testing.T) {
	ini := parseTestINI(t, `
[db]
database_url = postgres://localhost:5432/myapp

[workers]
redis_url = redis://localhost:6379
`)

	err := ValidatePrerequisites(ini)
	if err == nil {
		t.Fatal("expected error when centrifugo_api_url is missing")
	}
	if !strings.Contains(err.Error(), "Centrifugo") {
		t.Errorf("expected error to mention Centrifugo, got: %v", err)
	}
}

func TestValidatePrerequisites_MissingDatabase(t *testing.T) {
	ini := parseTestINI(t, `
[workers]
redis_url = redis://localhost:6379
centrifugo_api_url = http://localhost:8000/api
`)

	err := ValidatePrerequisites(ini)
	if err == nil {
		t.Fatal("expected error when database_url is missing")
	}
	if !strings.Contains(err.Error(), "database") {
		t.Errorf("expected error to mention database, got: %v", err)
	}
	if !strings.Contains(err.Error(), "shipq db setup") {
		t.Errorf("expected error to mention 'shipq db setup' command, got: %v", err)
	}
}

func TestValidatePrerequisites_EmptyRedisURL(t *testing.T) {
	ini := parseTestINI(t, `
[db]
database_url = postgres://localhost:5432/myapp

[workers]
redis_url =
centrifugo_api_url = http://localhost:8000/api
`)

	err := ValidatePrerequisites(ini)
	if err == nil {
		t.Fatal("expected error when redis_url is empty")
	}
	if !strings.Contains(err.Error(), "Redis") {
		t.Errorf("expected error to mention Redis, got: %v", err)
	}
}

func TestValidatePrerequisites_EmptyCentrifugoURL(t *testing.T) {
	ini := parseTestINI(t, `
[db]
database_url = postgres://localhost:5432/myapp

[workers]
redis_url = redis://localhost:6379
centrifugo_api_url =
`)

	err := ValidatePrerequisites(ini)
	if err == nil {
		t.Fatal("expected error when centrifugo_api_url is empty")
	}
	if !strings.Contains(err.Error(), "Centrifugo") {
		t.Errorf("expected error to mention Centrifugo, got: %v", err)
	}
}

func TestValidatePrerequisites_EmptyDatabaseURL(t *testing.T) {
	ini := parseTestINI(t, `
[db]
database_url =

[workers]
redis_url = redis://localhost:6379
centrifugo_api_url = http://localhost:8000/api
`)

	err := ValidatePrerequisites(ini)
	if err == nil {
		t.Fatal("expected error when database_url is empty")
	}
	if !strings.Contains(err.Error(), "database") {
		t.Errorf("expected error to mention database, got: %v", err)
	}
}

func TestValidatePrerequisites_EmptyINI(t *testing.T) {
	ini := parseTestINI(t, ``)

	err := ValidatePrerequisites(ini)
	if err == nil {
		t.Fatal("expected error for completely empty INI")
	}
	// Should fail on the first check — missing [workers].
	if !strings.Contains(err.Error(), "channel workers") {
		t.Errorf("expected error to mention channel workers for empty INI, got: %v", err)
	}
}

func TestValidatePrerequisites_AuthNotRequired(t *testing.T) {
	// Auth is NOT required for LLM — public channels work fine.
	ini := parseTestINI(t, `
[db]
database_url = postgres://localhost:5432/myapp

[workers]
redis_url = redis://localhost:6379
centrifugo_api_url = http://localhost:8000/api
`)

	// No [auth] section — should still pass.
	err := ValidatePrerequisites(ini)
	if err != nil {
		t.Fatalf("expected no error when auth is absent (not required), got: %v", err)
	}
}

func TestValidatePrerequisites_SQLiteDatabase(t *testing.T) {
	ini := parseTestINI(t, `
[db]
database_url = sqlite:myapp.db

[workers]
redis_url = redis://localhost:6379
centrifugo_api_url = http://localhost:8000/api
`)

	err := ValidatePrerequisites(ini)
	if err != nil {
		t.Fatalf("expected no error with SQLite database, got: %v", err)
	}
}

func TestValidatePrerequisites_ChecksInOrder(t *testing.T) {
	// When multiple prerequisites are missing, the first failure is reported.
	// Missing [workers] should be reported before missing database.
	ini := parseTestINI(t, `
[db]
`)

	err := ValidatePrerequisites(ini)
	if err == nil {
		t.Fatal("expected error")
	}
	// First check: [workers] missing.
	if !strings.Contains(err.Error(), "channel workers") {
		t.Errorf("expected first error to be about missing workers, got: %v", err)
	}
}
