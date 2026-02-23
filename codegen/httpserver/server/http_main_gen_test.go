package server

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// ── HasChannels tests ────────────────────────────────────────────────────────

func TestGenerateHTTPMain_HasChannels_ValidGo(t *testing.T) {
	dialects := []string{"mysql", "postgres", "sqlite"}

	for _, dialect := range dialects {
		t.Run(dialect, func(t *testing.T) {
			cfg := HTTPMainGenConfig{
				ModulePath:  "example.com/myapp",
				OutputPkg:   "api",
				DBDialect:   dialect,
				HasChannels: true,
			}

			code, err := GenerateHTTPMain(cfg)
			if err != nil {
				t.Fatalf("GenerateHTTPMain() error = %v", err)
			}

			_, err = parser.ParseFile(token.NewFileSet(), "", code, parser.AllErrors)
			if err != nil {
				t.Errorf("generated code is not valid Go: %v\n%s", err, string(code))
			}
		})
	}
}

func TestGenerateHTTPMain_HasChannels_ImportsChannelLibrary(t *testing.T) {
	cfg := HTTPMainGenConfig{
		ModulePath:  "example.com/myapp",
		OutputPkg:   "api",
		DBDialect:   "mysql",
		HasChannels: true,
	}

	code, err := GenerateHTTPMain(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPMain() error = %v", err)
	}

	codeStr := string(code)

	if !strings.Contains(codeStr, `"example.com/myapp/shipq/lib/channel"`) {
		t.Error("missing channel library import")
	}
	if !strings.Contains(codeStr, `"example.com/myapp/shipq/lib/logging"`) {
		t.Error("missing logging import for manual Decorate call")
	}
	if !strings.Contains(codeStr, `"example.com/myapp/api/auth"`) {
		t.Error("missing auth package import for channel auth wrappers")
	}
	if !strings.Contains(codeStr, `"example.com/myapp/shipq/queries"`) {
		t.Error("missing queries package import for runner context")
	}
}

func TestGenerateHTTPMain_HasChannels_CreatesTransportAndQueue(t *testing.T) {
	cfg := HTTPMainGenConfig{
		ModulePath:  "example.com/myapp",
		OutputPkg:   "api",
		DBDialect:   "mysql",
		HasChannels: true,
	}

	code, err := GenerateHTTPMain(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPMain() error = %v", err)
	}

	codeStr := string(code)

	if !strings.Contains(codeStr, "channel.NewCentrifugoTransport") {
		t.Error("missing CentrifugoTransport creation")
	}
	if !strings.Contains(codeStr, "channel.NewMachineryQueue") {
		t.Error("missing MachineryQueue creation")
	}
	if !strings.Contains(codeStr, "config.Settings.REDIS_URL") {
		t.Error("missing REDIS_URL config usage")
	}
	if !strings.Contains(codeStr, "config.Settings.CENTRIFUGO_API_URL") {
		t.Error("missing CENTRIFUGO_API_URL config usage")
	}
}

func TestGenerateHTTPMain_HasChannels_RegistersChannelRoutes(t *testing.T) {
	cfg := HTTPMainGenConfig{
		ModulePath:  "example.com/myapp",
		OutputPkg:   "api",
		DBDialect:   "mysql",
		HasChannels: true,
	}

	code, err := GenerateHTTPMain(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPMain() error = %v", err)
	}

	codeStr := string(code)

	if !strings.Contains(codeStr, "api.SetupMux(db, runner)") {
		t.Error("missing api.SetupMux call")
	}
	if !strings.Contains(codeStr, "api.RegisterChannelRoutes(") {
		t.Error("missing api.RegisterChannelRoutes call")
	}
	if !strings.Contains(codeStr, "logging.Decorate(") {
		t.Error("missing logging.Decorate call for manual wrapping")
	}
}

func TestGenerateHTTPMain_HasChannels_AuthWrappers(t *testing.T) {
	cfg := HTTPMainGenConfig{
		ModulePath:  "example.com/myapp",
		OutputPkg:   "api",
		DBDialect:   "mysql",
		HasChannels: true,
	}

	code, err := GenerateHTTPMain(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPMain() error = %v", err)
	}

	codeStr := string(code)

	if !strings.Contains(codeStr, "checkAuth") {
		t.Error("missing checkAuth wrapper function")
	}
	if !strings.Contains(codeStr, "checkRBAC") {
		t.Error("missing checkRBAC wrapper function")
	}
	if !strings.Contains(codeStr, "auth.GetCurrentSession") {
		t.Error("missing auth.GetCurrentSession call in checkAuth")
	}
	if !strings.Contains(codeStr, "auth.CheckRBAC") {
		t.Error("missing auth.CheckRBAC call in checkRBAC")
	}
	if !strings.Contains(codeStr, "httpserver.WithRequestCookies") {
		t.Error("missing httpserver.WithRequestCookies call in checkAuth")
	}
	if !strings.Contains(codeStr, "shipq/lib/httpserver") {
		t.Error("missing httpserver import for channel auth wrappers")
	}
}

func TestGenerateHTTPMain_HasChannels_DoesNotCallNewMux(t *testing.T) {
	cfg := HTTPMainGenConfig{
		ModulePath:  "example.com/myapp",
		OutputPkg:   "api",
		DBDialect:   "mysql",
		HasChannels: true,
	}

	code, err := GenerateHTTPMain(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPMain() error = %v", err)
	}

	codeStr := string(code)

	// Should use SetupMux, not NewMux (since we need the raw mux for channel routes)
	if strings.Contains(codeStr, "api.NewMux(") {
		t.Error("HasChannels main should use api.SetupMux, not api.NewMux")
	}
}

func TestGenerateHTTPMain_NoChannels_DoesNotImportChannelLib(t *testing.T) {
	cfg := HTTPMainGenConfig{
		ModulePath:  "example.com/myapp",
		OutputPkg:   "api",
		DBDialect:   "mysql",
		HasChannels: false,
	}

	code, err := GenerateHTTPMain(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPMain() error = %v", err)
	}

	codeStr := string(code)

	if strings.Contains(codeStr, "shipq/lib/channel") {
		t.Error("non-channel main should not import channel library")
	}
	if strings.Contains(codeStr, "RegisterChannelRoutes") {
		t.Error("non-channel main should not reference RegisterChannelRoutes")
	}
	if strings.Contains(codeStr, "SetupMux") {
		t.Error("non-channel main should not reference SetupMux")
	}
}

func TestGenerateHTTPMain_ValidGo(t *testing.T) {
	dialects := []string{"mysql", "postgres", "sqlite"}

	for _, dialect := range dialects {
		t.Run(dialect, func(t *testing.T) {
			cfg := HTTPMainGenConfig{
				ModulePath: "example.com/myapp",
				OutputPkg:  "api",
				DBDialect:  dialect,
			}

			code, err := GenerateHTTPMain(cfg)
			if err != nil {
				t.Fatalf("GenerateHTTPMain() error = %v", err)
			}

			// Verify it's valid Go
			_, err = parser.ParseFile(token.NewFileSet(), "", code, parser.AllErrors)
			if err != nil {
				t.Errorf("generated code is not valid Go: %v\n%s", err, string(code))
			}
		})
	}
}

func TestGenerateHTTPMain_ContainsExpectedElements(t *testing.T) {
	cfg := HTTPMainGenConfig{
		ModulePath: "example.com/myapp",
		OutputPkg:  "api",
		DBDialect:  "mysql",
	}

	code, err := GenerateHTTPMain(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPMain() error = %v", err)
	}

	codeStr := string(code)

	// Should have package main
	if !strings.Contains(codeStr, "package main") {
		t.Error("missing package main")
	}

	// Should have func main
	if !strings.Contains(codeStr, "func main()") {
		t.Error("missing func main()")
	}

	// Should import database/sql
	if !strings.Contains(codeStr, `"database/sql"`) {
		t.Error("missing database/sql import")
	}

	// Should import net/http
	if !strings.Contains(codeStr, `"net/http"`) {
		t.Error("missing net/http import")
	}

	// Should import the api package
	if !strings.Contains(codeStr, `"example.com/myapp/api"`) {
		t.Error("missing api package import")
	}

	// Should import the config package
	if !strings.Contains(codeStr, `"example.com/myapp/config"`) {
		t.Error("missing config package import")
	}

	// Should have sql.Open call
	if !strings.Contains(codeStr, "sql.Open") {
		t.Error("missing sql.Open call")
	}

	// Should have api.NewMux call
	if !strings.Contains(codeStr, "api.NewMux") {
		t.Error("missing api.NewMux call")
	}

	// Should have ListenAndServe
	if !strings.Contains(codeStr, "http.ListenAndServe") {
		t.Error("missing http.ListenAndServe call")
	}
}

func TestGenerateHTTPMain_MySQLDriver(t *testing.T) {
	cfg := HTTPMainGenConfig{
		ModulePath: "example.com/myapp",
		OutputPkg:  "api",
		DBDialect:  "mysql",
	}

	code, err := GenerateHTTPMain(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPMain() error = %v", err)
	}

	codeStr := string(code)

	// Should import mysql driver
	if !strings.Contains(codeStr, `"github.com/go-sql-driver/mysql"`) {
		t.Error("missing mysql driver import")
	}

	// Should use config.ParseDatabaseURL instead of hardcoded driver name
	if !strings.Contains(codeStr, "config.ParseDatabaseURL(config.Settings.DATABASE_URL)") {
		t.Error("missing config.ParseDatabaseURL call")
	}

	// Should NOT have a hardcoded driver name in sql.Open
	if strings.Contains(codeStr, `sql.Open("mysql"`) {
		t.Error("should not hardcode driver name in sql.Open; should use config.ParseDatabaseURL")
	}
}

func TestGenerateHTTPMain_PostgresDriver(t *testing.T) {
	cfg := HTTPMainGenConfig{
		ModulePath: "example.com/myapp",
		OutputPkg:  "api",
		DBDialect:  "postgres",
	}

	code, err := GenerateHTTPMain(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPMain() error = %v", err)
	}

	codeStr := string(code)

	// Should import pgx driver
	if !strings.Contains(codeStr, `"github.com/jackc/pgx/v5/stdlib"`) {
		t.Error("missing pgx driver import")
	}

	// Should use config.ParseDatabaseURL instead of hardcoded driver name
	if !strings.Contains(codeStr, "config.ParseDatabaseURL(config.Settings.DATABASE_URL)") {
		t.Error("missing config.ParseDatabaseURL call")
	}

	// Should NOT have a hardcoded driver name in sql.Open
	if strings.Contains(codeStr, `sql.Open("pgx"`) {
		t.Error("should not hardcode driver name in sql.Open; should use config.ParseDatabaseURL")
	}
}

func TestGenerateHTTPMain_SQLiteDriver(t *testing.T) {
	cfg := HTTPMainGenConfig{
		ModulePath: "example.com/myapp",
		OutputPkg:  "api",
		DBDialect:  "sqlite",
	}

	code, err := GenerateHTTPMain(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPMain() error = %v", err)
	}

	codeStr := string(code)

	// Should import sqlite driver
	if !strings.Contains(codeStr, `"modernc.org/sqlite"`) {
		t.Error("missing sqlite driver import")
	}

	// Should use config.ParseDatabaseURL instead of hardcoded driver name
	if !strings.Contains(codeStr, "config.ParseDatabaseURL(config.Settings.DATABASE_URL)") {
		t.Error("missing config.ParseDatabaseURL call")
	}

	// Should NOT have a hardcoded driver name in sql.Open
	if strings.Contains(codeStr, `sql.Open("sqlite"`) {
		t.Error("should not hardcode driver name in sql.Open; should use config.ParseDatabaseURL")
	}
}

func TestGenerateHTTPMain_UsesConfigSettings(t *testing.T) {
	cfg := HTTPMainGenConfig{
		ModulePath: "example.com/myapp",
		OutputPkg:  "api",
		DBDialect:  "mysql",
	}

	code, err := GenerateHTTPMain(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPMain() error = %v", err)
	}

	codeStr := string(code)

	// Should use config.Settings.DATABASE_URL via config.ParseDatabaseURL
	if !strings.Contains(codeStr, "config.ParseDatabaseURL(config.Settings.DATABASE_URL)") {
		t.Error("missing config.ParseDatabaseURL(config.Settings.DATABASE_URL)")
	}

	// Should use config.Settings.PORT
	if !strings.Contains(codeStr, "config.Settings.PORT") {
		t.Error("missing config.Settings.PORT")
	}

	// Should use config.Logger
	if !strings.Contains(codeStr, "config.Logger") {
		t.Error("missing config.Logger")
	}

	// Should use driver, dsn variables from ParseDatabaseURL
	if !strings.Contains(codeStr, "sql.Open(driver, dsn)") {
		t.Error("missing sql.Open(driver, dsn)")
	}
}

func TestGenerateHTTPMain_PortWithColon(t *testing.T) {
	cfg := HTTPMainGenConfig{
		ModulePath: "example.com/myapp",
		OutputPkg:  "api",
		DBDialect:  "mysql",
	}

	code, err := GenerateHTTPMain(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPMain() error = %v", err)
	}

	codeStr := string(code)

	// Should prepend ":" to port
	if !strings.Contains(codeStr, `":"+config.Settings.PORT`) && !strings.Contains(codeStr, `":" + config.Settings.PORT`) {
		t.Error("missing port with colon prefix")
	}
}

func TestGenerateHTTPMain_DatabasePing(t *testing.T) {
	cfg := HTTPMainGenConfig{
		ModulePath: "example.com/myapp",
		OutputPkg:  "api",
		DBDialect:  "mysql",
	}

	code, err := GenerateHTTPMain(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPMain() error = %v", err)
	}

	codeStr := string(code)

	// Should verify database connection with Ping
	if !strings.Contains(codeStr, "db.Ping()") {
		t.Error("missing db.Ping() verification")
	}
}

func TestGenerateHTTPMain_DeferClose(t *testing.T) {
	cfg := HTTPMainGenConfig{
		ModulePath: "example.com/myapp",
		OutputPkg:  "api",
		DBDialect:  "mysql",
	}

	code, err := GenerateHTTPMain(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPMain() error = %v", err)
	}

	codeStr := string(code)

	// Should have defer db.Close()
	if !strings.Contains(codeStr, "defer db.Close()") {
		t.Error("missing defer db.Close()")
	}
}

func TestGenerateHTTPMain_GeneratedComment(t *testing.T) {
	cfg := HTTPMainGenConfig{
		ModulePath: "example.com/myapp",
		OutputPkg:  "api",
		DBDialect:  "mysql",
	}

	code, err := GenerateHTTPMain(cfg)
	if err != nil {
		t.Fatalf("GenerateHTTPMain() error = %v", err)
	}

	codeStr := string(code)

	// Should have "Code generated by shipq" comment
	if !strings.Contains(codeStr, "Code generated by shipq") {
		t.Error("missing generated code comment")
	}
}

func TestGetDriverImport(t *testing.T) {
	tests := []struct {
		dialect string
		want    string
	}{
		{"mysql", "github.com/go-sql-driver/mysql"},
		{"postgres", "github.com/jackc/pgx/v5/stdlib"},
		{"sqlite", "modernc.org/sqlite"},
		{"unknown", "github.com/go-sql-driver/mysql"}, // defaults to mysql
		{"", "github.com/go-sql-driver/mysql"},        // empty defaults to mysql
	}

	for _, tt := range tests {
		t.Run(tt.dialect, func(t *testing.T) {
			got := getDriverImport(tt.dialect)
			if got != tt.want {
				t.Errorf("getDriverImport(%q) = %q; want %q", tt.dialect, got, tt.want)
			}
		})
	}
}
