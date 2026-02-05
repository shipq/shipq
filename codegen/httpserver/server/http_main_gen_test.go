package server

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

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

	// Should use "mysql" driver name
	if !strings.Contains(codeStr, `sql.Open("mysql"`) {
		t.Error("missing mysql driver name in sql.Open")
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

	// Should use "pgx" driver name
	if !strings.Contains(codeStr, `sql.Open("pgx"`) {
		t.Error("missing pgx driver name in sql.Open")
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

	// Should use "sqlite" driver name
	if !strings.Contains(codeStr, `sql.Open("sqlite"`) {
		t.Error("missing sqlite driver name in sql.Open")
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

	// Should use config.Settings.DATABASE_URL
	if !strings.Contains(codeStr, "config.Settings.DATABASE_URL") {
		t.Error("missing config.Settings.DATABASE_URL")
	}

	// Should use config.Settings.PORT
	if !strings.Contains(codeStr, "config.Settings.PORT") {
		t.Error("missing config.Settings.PORT")
	}

	// Should use config.Logger
	if !strings.Contains(codeStr, "config.Logger") {
		t.Error("missing config.Logger")
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

func TestGetDriverName(t *testing.T) {
	tests := []struct {
		dialect string
		want    string
	}{
		{"mysql", "mysql"},
		{"postgres", "pgx"},
		{"sqlite", "sqlite"},
		{"unknown", "mysql"}, // defaults to mysql
		{"", "mysql"},        // empty defaults to mysql
	}

	for _, tt := range tests {
		t.Run(tt.dialect, func(t *testing.T) {
			got := getDriverName(tt.dialect)
			if got != tt.want {
				t.Errorf("getDriverName(%q) = %q; want %q", tt.dialect, got, tt.want)
			}
		})
	}
}
