package registry

import (
	"strings"
	"testing"
)

func TestSetDefaults_DoesNotDefaultDialectToMySQL(t *testing.T) {
	cfg := CompileConfig{}
	setDefaults(&cfg)

	if cfg.DBDialect == "mysql" {
		t.Error("setDefaults should NOT default DBDialect to \"mysql\"; got \"mysql\"")
	}
	if cfg.DBDialect != "" {
		t.Errorf("setDefaults should leave DBDialect empty when not set; got %q", cfg.DBDialect)
	}
}

func TestSetDefaults_PreservesExplicitDialect(t *testing.T) {
	cfg := CompileConfig{DBDialect: "postgres"}
	setDefaults(&cfg)

	if cfg.DBDialect != "postgres" {
		t.Errorf("setDefaults should preserve explicit DBDialect; got %q, want \"postgres\"", cfg.DBDialect)
	}
}

func TestSetDefaults_DefaultsOutputPkgAndDir(t *testing.T) {
	cfg := CompileConfig{}
	setDefaults(&cfg)

	if cfg.OutputPkg != "api" {
		t.Errorf("OutputPkg = %q, want \"api\"", cfg.OutputPkg)
	}
	if cfg.OutputDir != "api" {
		t.Errorf("OutputDir = %q, want \"api\"", cfg.OutputDir)
	}
}

func TestCompileRegistry_ErrorsOnEmptyDialect(t *testing.T) {
	cfg := CompileConfig{
		DBDialect: "",
	}

	err := CompileRegistry(cfg)
	if err == nil {
		t.Fatal("expected CompileRegistry to return an error when DBDialect is empty")
	}

	msg := err.Error()
	if !strings.Contains(msg, "could not determine database dialect") {
		t.Errorf("error message should mention dialect detection failure; got: %s", msg)
	}
	if !strings.Contains(msg, "db.database_url") {
		t.Errorf("error message should mention db.database_url; got: %s", msg)
	}
}
