package files

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/shipq/shipq/codegen"
	configpkg "github.com/shipq/shipq/codegen/httpserver/config"
	"github.com/shipq/shipq/dburl"
	"github.com/shipq/shipq/inifile"
	"github.com/shipq/shipq/internal/commands/db"
	"github.com/shipq/shipq/internal/commands/migrate/up"
	"github.com/shipq/shipq/project"
	"github.com/shipq/shipq/registry"
)

const (
	// DefaultMigrationsDir is the default directory for migration files.
	DefaultMigrationsDir = "migrations"
)

// ProjectConfig holds the loaded project configuration.
type ProjectConfig struct {
	GoModRoot      string
	ShipqRoot      string
	ModulePath     string
	MigrationsPath string
	DatabaseURL    string
	Dialect        string
	ScopeColumn    string
}

// loadProjectConfig finds project roots and loads configuration.
func loadProjectConfig() (*ProjectConfig, error) {
	roots, err := project.FindProjectRoots()
	if err != nil {
		return nil, err
	}

	moduleInfo, err := codegen.GetModuleInfo(roots.GoModRoot, roots.ShipqRoot)
	if err != nil {
		return nil, err
	}
	modulePath := moduleInfo.FullImportPath("")

	shipqIniPath := filepath.Join(roots.ShipqRoot, project.ShipqIniFile)
	ini, err := inifile.ParseFile(shipqIniPath)
	if err != nil {
		return nil, err
	}

	migrationsDir := ini.Get("db", "migrations")
	if migrationsDir == "" {
		migrationsDir = DefaultMigrationsDir
	}

	migrationsPath := filepath.Join(roots.ShipqRoot, migrationsDir)

	databaseURL := ini.Get("db", "database_url")
	dialect := ""
	if databaseURL != "" {
		if d, err := dburl.InferDialectFromDBUrl(databaseURL); err == nil {
			dialect = d
		}
	}

	scopeColumn := ini.Get("db", "scope")

	return &ProjectConfig{
		GoModRoot:      roots.GoModRoot,
		ShipqRoot:      roots.ShipqRoot,
		ModulePath:     modulePath,
		MigrationsPath: migrationsPath,
		DatabaseURL:    databaseURL,
		Dialect:        dialect,
		ScopeColumn:    scopeColumn,
	}, nil
}

// filesMigrationSuffixes are the file suffixes used to detect existing files migrations.
var filesMigrationSuffixes = []string{
	"_managed_files.go",
	"_file_access.go",
}

// filesMigrationsExist checks if all files migration files already exist.
func filesMigrationsExist(migrationsPath string) bool {
	entries, err := os.ReadDir(migrationsPath)
	if err != nil {
		return false
	}

	found := make(map[string]bool)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		for _, suffix := range filesMigrationSuffixes {
			if len(name) > len(suffix) && name[len(name)-len(suffix):] == suffix {
				found[suffix] = true
			}
		}
	}
	return len(found) == len(filesMigrationSuffixes)
}

// FilesCmd handles "shipq files" - generates file upload system.
func FilesCmd() {
	cfg, err := loadProjectConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: not in a shipq project (%v)\n", err)
		os.Exit(1)
	}

	// Create migrations directory if needed
	if err := os.MkdirAll(cfg.MigrationsPath, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to create migrations directory: %v\n", err)
		os.Exit(1)
	}

	// STEP 1: Update shipq.ini with [files] section
	fmt.Println("Updating shipq.ini with files config...")
	shipqIniPath := filepath.Join(cfg.ShipqRoot, project.ShipqIniFile)
	ini, iniErr := inifile.ParseFile(shipqIniPath)
	if iniErr != nil {
		fmt.Fprintf(os.Stderr, "error: failed to parse shipq.ini: %v\n", iniErr)
		os.Exit(1)
	}

	// Only set defaults if the section doesn't already exist
	if ini.Get("files", "s3_bucket") == "" {
		ini.Set("files", "s3_bucket", "shipq-dev")
	}
	if ini.Get("files", "s3_region") == "" {
		ini.Set("files", "s3_region", "us-east-1")
	}
	if ini.Get("files", "s3_endpoint") == "" {
		ini.Set("files", "s3_endpoint", "http://localhost:9000")
	}
	if ini.Get("files", "aws_access_key_id") == "" {
		ini.Set("files", "aws_access_key_id", "minioadmin")
	}
	if ini.Get("files", "aws_secret_access_key") == "" {
		ini.Set("files", "aws_secret_access_key", "minioadmin")
	}
	if ini.Get("files", "max_upload_size_mb") == "" {
		ini.Set("files", "max_upload_size_mb", "100")
	}
	if ini.Get("files", "multipart_threshold_mb") == "" {
		ini.Set("files", "multipart_threshold_mb", "10")
	}
	if ini.Get("files", "typescript_output") == "" {
		ini.Set("files", "typescript_output", ".")
	}

	if writeErr := ini.WriteFile(shipqIniPath); writeErr != nil {
		fmt.Fprintf(os.Stderr, "error: failed to write shipq.ini: %v\n", writeErr)
		os.Exit(1)
	}
	fmt.Println("  Set [files] config in shipq.ini")

	// STEP 2: Generate migrations
	if filesMigrationsExist(cfg.MigrationsPath) {
		fmt.Println("")
		fmt.Println("Files migrations already exist, skipping migration generation...")
		fmt.Println("")
		fmt.Println("Running migrations (in case they haven't been applied)...")
		up.MigrateUpCmd()
	} else {
		fmt.Println("")
		fmt.Println("Generating files migrations...")
		fmt.Println("")

		baseTime := time.Now().UTC()
		timestamps := make([]string, 2)
		for i := range timestamps {
			timestamps[i] = baseTime.Add(time.Duration(i) * time.Second).Format("20060102150405")
		}

		type migrationDef struct {
			name     string
			generate func(timestamp, modulePath string) []byte
		}
		migrations := []migrationDef{
			{"managed_files", generateManagedFilesMigration},
			{"file_access", generateFileAccessMigration},
		}

		for i, m := range migrations {
			code := m.generate(timestamps[i], cfg.ModulePath)
			fileName := fmt.Sprintf("%s_%s.go", timestamps[i], m.name)
			filePath := filepath.Join(cfg.MigrationsPath, fileName)

			if err := os.WriteFile(filePath, code, 0644); err != nil {
				fmt.Fprintf(os.Stderr, "error: failed to write %s: %v\n", fileName, err)
				os.Exit(1)
			}

			relPath, _ := filepath.Rel(cfg.ShipqRoot, filePath)
			fmt.Printf("  Created: %s\n", relPath)
		}

		fmt.Println("")
		fmt.Println("Running migrations...")
		up.MigrateUpCmd()
	}

	// STEP 3: Generate config package (handlers import config for S3 settings)
	fmt.Println("")
	fmt.Println("Generating config package...")
	// Re-read ini to pick up the [files] values we just wrote
	ini, iniErr = inifile.ParseFile(shipqIniPath)
	if iniErr != nil {
		fmt.Fprintf(os.Stderr, "error: failed to re-read shipq.ini: %v\n", iniErr)
		os.Exit(1)
	}
	workersEnabled := ini.Section("workers") != nil
	devDefaults := configpkg.DevDefaults{
		DatabaseURL:          cfg.DatabaseURL,
		Port:                 "8080",
		CookieSecret:         ini.Get("auth", "cookie_secret"),
		S3Bucket:             ini.Get("files", "s3_bucket"),
		S3Region:             ini.Get("files", "s3_region"),
		S3Endpoint:           ini.Get("files", "s3_endpoint"),
		AWSAccessKeyID:       ini.Get("files", "aws_access_key_id"),
		AWSSecretAccessKey:   ini.Get("files", "aws_secret_access_key"),
		MaxUploadSizeMB:      ini.Get("files", "max_upload_size_mb"),
		MultipartThresholdMB: ini.Get("files", "multipart_threshold_mb"),
	}
	if workersEnabled {
		devDefaults.RedisURL = ini.Get("workers", "redis_url")
		devDefaults.CentrifugoAPIURL = ini.Get("workers", "centrifugo_api_url")
		devDefaults.CentrifugoAPIKey = ini.Get("workers", "centrifugo_api_key")
		devDefaults.CentrifugoHMACSecret = ini.Get("workers", "centrifugo_hmac_secret")
		devDefaults.CentrifugoWSURL = ini.Get("workers", "centrifugo_ws_url")
	}

	// Read OAuth flags from [auth]
	oauthGoogle := strings.ToLower(ini.Get("auth", "oauth_google")) == "true"
	oauthGitHub := strings.ToLower(ini.Get("auth", "oauth_github")) == "true"

	// Read email flag
	emailEnabled := ini.Section("email") != nil

	// Populate OAuth dev defaults
	if oauthGoogle || oauthGitHub {
		devDefaults.OAuthRedirectURL = ini.Get("auth", "oauth_redirect_url")
		devDefaults.OAuthRedirectBaseURL = ini.Get("auth", "oauth_redirect_base_url")
	}

	// Populate email dev defaults
	if emailEnabled {
		devDefaults.SMTPHost = ini.Get("email", "smtp_host")
		devDefaults.SMTPPort = ini.Get("email", "smtp_port")
		devDefaults.SMTPUsername = ini.Get("email", "smtp_username")
		devDefaults.SMTPPassword = ini.Get("email", "smtp_password")
		devDefaults.AppURL = ini.Get("email", "app_url")
	}

	if err := registry.GenerateConfigEarlyWithFullOptions(registry.ConfigEarlyOptions{
		ShipqRoot:      cfg.ShipqRoot,
		GoModRoot:      cfg.GoModRoot,
		Dialect:        cfg.Dialect,
		FilesEnabled:   true,
		WorkersEnabled: workersEnabled,
		OAuthGoogle:    oauthGoogle,
		OAuthGitHub:    oauthGitHub,
		EmailEnabled:   emailEnabled,
		DevDefaults:    devDefaults,
		CustomEnvVars:  registry.ParseCustomEnvVars(ini),
	}); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to generate config: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  Created: config/config.go")

	// STEP 4: Generate file upload handlers
	fmt.Println("")
	fmt.Println("Generating file upload handlers...")
	fmt.Println("")

	handlerFiles := GenerateFileHandlerFiles(cfg.ModulePath, cfg.ScopeColumn)

	// Create api/managed_files directory
	filesDir := filepath.Join(cfg.ShipqRoot, "api", "managed_files")
	if err := os.MkdirAll(filesDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to create api/managed_files directory: %v\n", err)
		os.Exit(1)
	}

	// Write handler files
	for filename, content := range handlerFiles {
		filePath := filepath.Join(filesDir, filename)
		changed, err := codegen.WriteFileIfChanged(filePath, content)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: failed to write %s: %v\n", filePath, err)
			os.Exit(1)
		}
		if changed {
			relPath, _ := filepath.Rel(cfg.ShipqRoot, filePath)
			fmt.Printf("  Created: %s\n", relPath)
		}
	}

	// Write .shipq-no-regen marker
	markerPath := filepath.Join(filesDir, ".shipq-no-regen")
	if _, err := os.Stat(markerPath); os.IsNotExist(err) {
		markerContent := "# This file prevents shipq from regenerating handlers in this directory.\n# File upload handlers are custom and should not be overwritten by CRUD generation.\n# Delete this file if you want shipq to regenerate the handlers.\n"
		if err := os.WriteFile(markerPath, []byte(markerContent), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "error: failed to write %s: %v\n", markerPath, err)
			os.Exit(1)
		}
	}

	// STEP 5: Generate file query definitions
	fmt.Println("")
	fmt.Println("Generating file query definitions...")

	queryDefs := GenerateFileQueryDefs(cfg.ModulePath)
	queryDefsDir := filepath.Join(cfg.ShipqRoot, "querydefs", "managed_files")
	if err := os.MkdirAll(queryDefsDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to create querydefs/managed_files directory: %v\n", err)
		os.Exit(1)
	}

	queryDefsPath := filepath.Join(queryDefsDir, "queries.go")
	if err := os.WriteFile(queryDefsPath, queryDefs, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to write querydefs/managed_files/queries.go: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  Created: querydefs/managed_files/queries.go")

	// STEP 6: Run go mod tidy
	fmt.Println("")
	fmt.Println("Running go mod tidy...")
	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = cfg.GoModRoot
	if tidyOut, tidyErr := tidyCmd.CombinedOutput(); tidyErr != nil {
		fmt.Fprintf(os.Stderr, "error: go mod tidy failed: %v\n%s\n", tidyErr, tidyOut)
		os.Exit(1)
	}
	fmt.Println("  go mod tidy done")

	// STEP 7: Compile queries
	fmt.Println("")
	fmt.Println("Compiling queries...")
	db.DBCompileCmd()

	// STEP 8: Compile handler registry
	fmt.Println("")
	fmt.Println("Compiling handler registry...")
	if err := registry.Run(cfg.ShipqRoot, cfg.GoModRoot); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to compile registry: %v\n", err)
		os.Exit(1)
	}

	// STEP 9: Generate TypeScript upload helpers
	fmt.Println("")
	fmt.Println("Generating TypeScript upload helpers...")
	tsOutput := ini.Get("files", "typescript_output")
	if tsOutput == "" {
		tsOutput = "."
	}
	tsDir := filepath.Join(cfg.ShipqRoot, tsOutput)
	if err := os.MkdirAll(tsDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to create TypeScript output directory: %v\n", err)
		os.Exit(1)
	}
	tsPath := filepath.Join(tsDir, "shipq-files.ts")
	tsContent := GenerateTypeScriptHelpers()
	tsWritten, tsErr := codegen.WriteFileIfChanged(tsPath, tsContent)
	if tsErr != nil {
		fmt.Fprintf(os.Stderr, "error: failed to write shipq-files.ts: %v\n", tsErr)
		os.Exit(1)
	}
	if tsWritten {
		relPath, _ := filepath.Rel(cfg.ShipqRoot, tsPath)
		fmt.Printf("  Created: %s\n", relPath)
	}

	// STEP 10: Generate file upload tests
	fmt.Println("")
	fmt.Println("Generating file upload tests...")

	testFiles := GenerateFileTestFiles(cfg.ModulePath, cfg.ScopeColumn, cfg.Dialect)
	testDir := filepath.Join(cfg.ShipqRoot, "api", "managed_files", "spec")
	if err := os.MkdirAll(testDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to create api/managed_files/spec directory: %v\n", err)
		os.Exit(1)
	}

	for filename, content := range testFiles {
		filePath := filepath.Join(testDir, filename)
		changed, err := codegen.WriteFileIfChanged(filePath, content)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: failed to write %s: %v\n", filePath, err)
			os.Exit(1)
		}
		if changed {
			relPath, _ := filepath.Rel(cfg.ShipqRoot, filePath)
			fmt.Printf("  Created: %s\n", relPath)
		}
	}

	fmt.Println("")
	fmt.Println("File upload system created successfully!")
	fmt.Println("")
	fmt.Println("Generated routes:")
	fmt.Println("  POST   /files/upload-url           - Get presigned upload URL")
	fmt.Println("  POST   /files/complete              - Complete upload")
	fmt.Println("  GET    /files/:id/download           - Download file (302 redirect)")
	fmt.Println("  GET    /files                        - List files (visibility-aware)")
	fmt.Println("  DELETE /files/:id                    - Soft delete file")
	fmt.Println("  PATCH  /files/:id/visibility         - Change file visibility")
	fmt.Println("  POST   /files/:id/access             - Grant file access")
	fmt.Println("  DELETE /files/:id/access/:account_id  - Revoke file access")
	fmt.Println("  GET    /files/:id/access             - List file access")
	fmt.Println("")
	fmt.Println("Environment variables required (production only — dev defaults are inferred from shipq.ini):")
	fmt.Println("  S3_BUCKET            - S3 bucket name (dev: shipq-dev)")
	fmt.Println("  S3_REGION            - S3 region (dev: us-east-1)")
	fmt.Println("  S3_ENDPOINT          - S3 endpoint (dev: http://localhost:9000 for MinIO)")
	fmt.Println("  AWS_ACCESS_KEY_ID    - S3 access key (dev: minioadmin)")
	fmt.Println("  AWS_SECRET_ACCESS_KEY - S3 secret key (dev: minioadmin)")
	fmt.Println("")
	fmt.Println("Local development: run 'shipq start minio' to start MinIO on port 9000.")
	fmt.Println("")
	fmt.Println("TypeScript helpers generated at: " + tsPath)
}
