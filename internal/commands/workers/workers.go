package workers

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/shipq/shipq/cli"
	"github.com/shipq/shipq/codegen"
	"github.com/shipq/shipq/codegen/channelcompile"
	"github.com/shipq/shipq/codegen/channelgen"
	"github.com/shipq/shipq/codegen/embed"
	configpkg "github.com/shipq/shipq/codegen/httpserver/config"
	"github.com/shipq/shipq/codegen/llmgen"
	codegenMigrate "github.com/shipq/shipq/codegen/migrate"
	"github.com/shipq/shipq/dburl"
	"github.com/shipq/shipq/inifile"
	"github.com/shipq/shipq/internal/commands/db"
	"github.com/shipq/shipq/internal/commands/migrate/up"
	shipqdag "github.com/shipq/shipq/internal/dag"
	"github.com/shipq/shipq/project"
	"github.com/shipq/shipq/registry"
)

const (
	// DefaultMigrationsDir is the default directory for migration files.
	DefaultMigrationsDir = "migrations"
)

// jobResultsMigrationSuffix is used to detect existing job_results migrations.
var jobResultsMigrationSuffixes = []string{
	"_job_results.go",
}

// WorkersCmd implements the "shipq workers" bootstrap command.
// It runs the full pipeline to set up the workers system:
// channels, Centrifugo, task queue, and all generated code.
func WorkersCmd() {
	// ── Step 0: Check prerequisites ──────────────────────────────────

	roots, err := project.FindProjectRoots()
	if err != nil {
		cli.FatalErr("not in a shipq project", err)
	}

	// DAG prerequisite check (alongside existing checks)
	if !shipqdag.CheckPrerequisites(shipqdag.CmdWorkers, roots.ShipqRoot) {
		os.Exit(1)
	}

	shipqIniPath := filepath.Join(roots.ShipqRoot, project.ShipqIniFile)
	ini, err := inifile.ParseFile(shipqIniPath)
	if err != nil {
		cli.FatalErr("failed to parse shipq.ini", err)
	}

	// Verify redis-server on $PATH
	if _, err := exec.LookPath("redis-server"); err != nil {
		cli.Fatal("redis-server not found on $PATH -- add it to your shell.nix")
	}

	// Verify centrifugo on $PATH
	if _, err := exec.LookPath("centrifugo"); err != nil {
		cli.Fatal("centrifugo not found on $PATH -- add it to your shell.nix")
	}

	cli.Success("Prerequisites OK")

	// ── Step 1: Load project config ──────────────────────────────────

	moduleInfo, err := codegen.GetModuleInfo(roots.GoModRoot, roots.ShipqRoot)
	if err != nil {
		cli.FatalErr("failed to get module info", err)
	}
	importPrefix := moduleInfo.FullImportPath("")

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
	hasTenancy := scopeColumn != ""
	hasAuth := ini.Section("auth") != nil

	filesEnabled := ini.Section("files") != nil

	cli.Infof("Project: %s", importPrefix)

	// ── Step 2: Update shipq.ini with [workers] section ──────────────

	fmt.Println("")
	fmt.Println("Updating shipq.ini with workers config...")

	if ini.Section("workers") == nil {
		apiKey, err := generateRandomKey()
		if err != nil {
			cli.FatalErr("failed to generate API key", err)
		}
		hmacSecret, err := generateRandomKey()
		if err != nil {
			cli.FatalErr("failed to generate HMAC secret", err)
		}

		ini.Set("workers", "redis_url", "redis://localhost:6379")
		ini.Set("workers", "centrifugo_api_url", "http://localhost:8000/api")
		ini.Set("workers", "centrifugo_api_key", apiKey)
		ini.Set("workers", "centrifugo_hmac_secret", hmacSecret)
		ini.Set("workers", "centrifugo_ws_url", "ws://localhost:8000/connection/websocket")

		if writeErr := ini.WriteFile(shipqIniPath); writeErr != nil {
			cli.FatalErr("failed to write shipq.ini", writeErr)
		}
		fmt.Println("  Set [workers] config in shipq.ini")
	} else {
		fmt.Println("  [workers] section already exists, skipping")
	}

	// Re-read ini to get the (possibly just-written) values
	ini, err = inifile.ParseFile(shipqIniPath)
	if err != nil {
		cli.FatalErr("failed to re-read shipq.ini", err)
	}

	centrifugoAPIKey := ini.Get("workers", "centrifugo_api_key")
	centrifugoHMACSecret := ini.Get("workers", "centrifugo_hmac_secret")
	centrifugoAPIURL := ini.Get("workers", "centrifugo_api_url")
	centrifugoWSURL := ini.Get("workers", "centrifugo_ws_url")
	redisURL := ini.Get("workers", "redis_url")

	// ── Step 3: Generate job_results migration ───────────────────────

	fmt.Println("")
	fmt.Println("Checking job_results migration...")

	if err := os.MkdirAll(migrationsPath, 0755); err != nil {
		cli.FatalErr("failed to create migrations directory", err)
	}

	if jobResultsMigrationExists(migrationsPath) {
		fmt.Println("  job_results migration already exists, skipping")
		fmt.Println("")
		fmt.Println("Running migrations (in case they haven't been applied)...")
		up.MigrateUpCmd()
	} else {
		fmt.Println("  Generating job_results migration...")

		timestamp := codegenMigrate.NextMigrationBaseTime(migrationsPath).Format("20060102150405")
		code := channelgen.GenerateJobResultsMigration(timestamp, importPrefix, hasTenancy)
		fileName := fmt.Sprintf("%s_job_results.go", timestamp)
		filePath := filepath.Join(migrationsPath, fileName)

		if err := os.WriteFile(filePath, code, 0644); err != nil {
			cli.FatalErr("failed to write migration", err)
		}

		relPath, _ := filepath.Rel(roots.ShipqRoot, filePath)
		fmt.Printf("  Created: %s\n", relPath)

		fmt.Println("")
		fmt.Println("Running migrations...")
		up.MigrateUpCmd()
	}

	// ── Step 4: Generate config package (early) ──────────────────────

	fmt.Println("")
	fmt.Println("Generating config package...")

	// Read OAuth flags from [auth]
	oauthGoogle := strings.ToLower(ini.Get("auth", "oauth_google")) == "true"
	oauthGitHub := strings.ToLower(ini.Get("auth", "oauth_github")) == "true"

	// Read email flag
	emailEnabled := ini.Section("email") != nil

	devDefaults := configpkg.DevDefaults{
		DatabaseURL:          databaseURL,
		Port:                 "8080",
		CookieSecret:         ini.Get("auth", "cookie_secret"),
		RedisURL:             redisURL,
		CentrifugoAPIURL:     centrifugoAPIURL,
		CentrifugoAPIKey:     centrifugoAPIKey,
		CentrifugoHMACSecret: centrifugoHMACSecret,
		CentrifugoWSURL:      centrifugoWSURL,
	}
	if filesEnabled {
		devDefaults.S3Bucket = ini.Get("files", "s3_bucket")
		devDefaults.S3Region = ini.Get("files", "s3_region")
		devDefaults.S3Endpoint = ini.Get("files", "s3_endpoint")
		devDefaults.AWSAccessKeyID = ini.Get("files", "aws_access_key_id")
		devDefaults.AWSSecretAccessKey = ini.Get("files", "aws_secret_access_key")
		devDefaults.MaxUploadSizeMB = ini.Get("files", "max_upload_size_mb")
		devDefaults.MultipartThresholdMB = ini.Get("files", "multipart_threshold_mb")
	}

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
		ShipqRoot:      roots.ShipqRoot,
		GoModRoot:      roots.GoModRoot,
		Dialect:        dialect,
		FilesEnabled:   filesEnabled,
		WorkersEnabled: true,
		OAuthGoogle:    oauthGoogle,
		OAuthGitHub:    oauthGitHub,
		EmailEnabled:   emailEnabled,
		DevDefaults:    devDefaults,
		CustomEnvVars:  registry.ParseCustomEnvVars(ini),
	}); err != nil {
		cli.FatalErr("failed to generate config", err)
	}
	fmt.Println("  Generated config/config.go")

	// ── Step 5: Embed channel runtime library ────────────────────────

	fmt.Println("")
	fmt.Println("Embedding runtime library packages...")

	if err := embed.EmbedAllPackages(roots.ShipqRoot, importPrefix, embed.EmbedOptions{
		FilesEnabled:   filesEnabled,
		WorkersEnabled: true,
		DBDialect:      dialect,
	}); err != nil {
		cli.FatalErr("failed to embed packages", err)
	}
	fmt.Println("  Embedded all library packages")

	// ── Step 6: Generate example channel ─────────────────────────────

	fmt.Println("")
	channelsDir := filepath.Join(roots.ShipqRoot, "channels")
	generatedExample := false
	if _, err := os.Stat(channelsDir); os.IsNotExist(err) {
		fmt.Println("Generating example channel...")

		exampleDir := filepath.Join(channelsDir, "example")
		if err := os.MkdirAll(exampleDir, 0755); err != nil {
			cli.FatalErr("failed to create example channel directory", err)
		}

		exampleCode := generateExampleChannel(importPrefix)
		examplePath := filepath.Join(exampleDir, "register.go")
		if err := os.WriteFile(examplePath, []byte(exampleCode), 0644); err != nil {
			cli.FatalErr("failed to write example channel", err)
		}

		relPath, _ := filepath.Rel(roots.ShipqRoot, examplePath)
		fmt.Printf("  Created: %s\n", relPath)
		generatedExample = true
	} else {
		fmt.Println("channels/ directory already exists, skipping example generation")
	}

	// ── Step 7: Run go mod tidy ──────────────────────────────────────

	fmt.Println("")
	fmt.Println("Running go mod tidy...")

	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = roots.GoModRoot
	if tidyOut, tidyErr := tidyCmd.CombinedOutput(); tidyErr != nil {
		fmt.Fprintf(os.Stderr, "error: go mod tidy failed: %v\n%s\n", tidyErr, tidyOut)
		os.Exit(1)
	}
	fmt.Println("  go mod tidy done")

	// ── Step 8: Generate querydefs and compile queries ───────────────
	// This must happen before compiling the handler registry (which builds
	// the whole project, including api/auth that references query methods).
	// migrate up regenerates shipq/queries with UserQueries: nil, wiping
	// out any queries previously generated by `shipq auth`. We restore
	// them here by generating the job_results querydefs and running a
	// full db compile that re-discovers ALL querydefs packages.

	fmt.Println("")
	fmt.Println("Generating job_results querydefs...")

	querydefsDir := filepath.Join(roots.ShipqRoot, "querydefs", "job_results")
	if err := os.MkdirAll(querydefsDir, 0755); err != nil {
		cli.FatalErr("failed to create querydefs directory", err)
	}

	querydefsCode := channelgen.GenerateJobResultsQuerydefs(importPrefix, hasTenancy, hasAuth)
	querydefsPath := filepath.Join(querydefsDir, "queries.go")
	if err := os.WriteFile(querydefsPath, querydefsCode, 0644); err != nil {
		cli.FatalErr("failed to write querydefs", err)
	}
	fmt.Println("  Generated querydefs/job_results/queries.go")

	fmt.Println("")
	fmt.Println("Compiling queries...")
	db.DBCompileCmd()

	// ── Step 9: Compile channel registry ─────────────────────────────

	fmt.Println("")
	fmt.Println("Compiling channel registry...")

	channels, err := channelcompile.BuildAndRunChannelCompileProgram(roots.GoModRoot, roots.ShipqRoot, moduleInfo)
	if err != nil {
		cli.FatalErr("failed to compile channels", err)
	}

	if err := channelcompile.ValidateChannels(channels); err != nil {
		cli.FatalErr("channel validation failed", err)
	}

	cli.Infof("  Found %d channel(s)", len(channels))

	// ── Step 10: Generate typed channel code ─────────────────────────

	fmt.Println("")
	fmt.Println("Generating typed channel code...")

	if err := channelgen.GenerateAllTypedChannels(channels, roots.GoModRoot, roots.ShipqRoot, importPrefix); err != nil {
		cli.FatalErr("failed to generate typed channels", err)
	}
	fmt.Println("  Generated typed channel files")

	// Write the example handler now that TypedChannelFromContext exists in
	// the generated zz_generated_channel.go.
	if generatedExample {
		exampleDir := filepath.Join(channelsDir, "example")
		handlerCode := generateExampleHandler()
		handlerPath := filepath.Join(exampleDir, "handler.go")
		if err := os.WriteFile(handlerPath, []byte(handlerCode), 0644); err != nil {
			cli.FatalErr("failed to write example handler", err)
		}
		relPath, _ := filepath.Rel(roots.ShipqRoot, handlerPath)
		fmt.Printf("  Created: %s\n", relPath)
	}

	// ── Step 11: Generate worker server ──────────────────────────────

	fmt.Println("")
	fmt.Println("Generating worker server...")

	// Extract redis host:port from URL for worker config
	redisAddr := extractRedisAddr(redisURL)

	// Detect auto_migrate setting from [db] section
	autoMigrate := false
	if strings.ToLower(ini.Get("db", "auto_migrate")) == "true" {
		schemaJSONPath := filepath.Join(roots.ShipqRoot, "shipq", "db", "migrate", "schema.json")
		if _, err := os.Stat(schemaJSONPath); err == nil {
			autoMigrate = true
		}
	}

	workerCfg := channelgen.WorkerGenConfig{
		Channels:             channels,
		ModulePath:           importPrefix,
		DBDialect:            dialect,
		RedisAddr:            redisAddr,
		CentrifugoAPIURL:     centrifugoAPIURL,
		CentrifugoAPIKey:     centrifugoAPIKey,
		CentrifugoHMACSecret: centrifugoHMACSecret,
		CentrifugoWSURL:      centrifugoWSURL,
		AutoMigrate:          autoMigrate,
	}

	if err := channelgen.WriteWorkerMain(workerCfg, roots.ShipqRoot); err != nil {
		cli.FatalErr("failed to generate worker main", err)
	}
	fmt.Println("  Generated cmd/worker/main.go")

	// ── Step 12: Generate Centrifugo config ──────────────────────────

	fmt.Println("")
	fmt.Println("Generating Centrifugo config...")

	if err := channelgen.WriteCentrifugoConfig(channels, roots.ShipqRoot, centrifugoAPIKey, centrifugoHMACSecret); err != nil {
		cli.FatalErr("failed to generate centrifugo config", err)
	}
	fmt.Println("  Generated centrifugo.json")

	// ── Step 13: Compile handler registry ────────────────────────────
	// The generated config init() now uses dev defaults in non-production mode,
	// so we no longer need to set env vars for the handler compile program.

	fmt.Println("")
	fmt.Println("Compiling handler registry...")

	if err := registry.Run(roots.ShipqRoot, roots.GoModRoot); err != nil {
		cli.FatalErr("failed to compile registry", err)
	}
	fmt.Println("  Handler registry compiled")

	// ── Step 14: Generate TypeScript client ──────────────────────────

	// Only generate if there are frontend channels
	hasFrontendChannels := false
	for _, ch := range channels {
		if ch.Visibility == "frontend" {
			hasFrontendChannels = true
			break
		}
	}

	if hasFrontendChannels {
		fmt.Println("")
		fmt.Println("Generating TypeScript channel client...")

		// Determine TypeScript output directory: [workers] typescript_channel_output
		// falls back to [typescript] channel_output, then [files] typescript_output,
		// then "." (project root).
		tsOutputDir := ini.Get("workers", "typescript_channel_output")
		if tsOutputDir == "" {
			tsOutputDir = ini.Get("typescript", "channel_output")
		}
		if tsOutputDir == "" {
			tsOutputDir = ini.Get("files", "typescript_output")
		}
		if tsOutputDir == "" {
			tsOutputDir = "."
		}

		// Build LLM config for TypeScript generation.
		// First try the marker file written by `llm compile`. If it's empty
		// (e.g. because `llm compile` ran before channels were fully generated),
		// fall back to live detection using the channel list we just compiled.
		var llmCfg *channelgen.LLMConfig
		llmChannelPkgs, _ := llmgen.ReadLLMChannelsMarker(roots.ShipqRoot)
		if len(llmChannelPkgs) == 0 {
			// Marker is empty/missing — try live detection from compiled channels.
			var allPkgs []string
			for _, ch := range channels {
				if ch.PackagePath != "" {
					allPkgs = append(allPkgs, ch.PackagePath)
				}
			}
			if len(allPkgs) > 0 {
				detected, detectErr := llmgen.DetectLLMChannels(roots.ShipqRoot, importPrefix, allPkgs)
				if detectErr == nil && len(detected) > 0 {
					llmChannelPkgs = detected
					// Update the marker file so subsequent runs don't need live detection.
					_ = llmgen.WriteLLMChannelsMarker(roots.ShipqRoot, detected)
				}
			}
		}
		llmTools, _ := llmgen.ReadLLMToolsMarker(roots.ShipqRoot)
		if len(llmChannelPkgs) > 0 {
			pkgSet := make(map[string]bool, len(llmChannelPkgs))
			for _, pkg := range llmChannelPkgs {
				pkgSet[pkg] = true
			}
			llmCfg = &channelgen.LLMConfig{
				LLMChannelPkgs: pkgSet,
				Tools:          llmTools,
			}
		}

		if err := channelgen.WriteTypeScriptChannelClient(channels, roots.ShipqRoot, tsOutputDir, llmCfg); err != nil {
			cli.FatalErr("failed to generate TypeScript channel client", err)
		}
		fmt.Printf("  Generated %s/shipq-channels.ts\n", tsOutputDir)

		// Generate framework-specific channel hooks based on [typescript] framework
		tsFrameworks := registry.ParseFrameworks(ini.Get("typescript", "framework"))

		if registry.HasFramework(tsFrameworks, "react") {
			if err := channelgen.WriteReactChannelHooks(channels, roots.ShipqRoot, tsOutputDir, llmCfg); err != nil {
				cli.FatalErr("failed to generate React channel hooks", err)
			}
			fmt.Printf("  Generated %s/react/shipq-channels.ts\n", tsOutputDir)
		}

		if registry.HasFramework(tsFrameworks, "svelte") {
			if err := channelgen.WriteSvelteChannelHooks(channels, roots.ShipqRoot, tsOutputDir, llmCfg); err != nil {
				cli.FatalErr("failed to generate Svelte channel hooks", err)
			}
			fmt.Printf("  Generated %s/svelte/shipq-channels.ts\n", tsOutputDir)
		}
	}

	// ── Step 15: Generate tests ──────────────────────────────────────

	fmt.Println("")
	fmt.Println("Generating channel tests...")

	if err := channelgen.GenerateIntegrationTests(channels, importPrefix, roots.ShipqRoot); err != nil {
		cli.FatalErr("failed to generate integration tests", err)
	}
	fmt.Println("  Generated channels/spec/zz_generated_integration_test.go")

	if err := channelgen.GenerateE2ETest(channels, importPrefix, roots.ShipqRoot); err != nil {
		cli.FatalErr("failed to generate E2E tests", err)
	}
	fmt.Println("  Generated channels/spec/zz_generated_e2e_test.go")

	// ── Done ─────────────────────────────────────────────────────────

	fmt.Println("")
	cli.Success("Workers system bootstrapped successfully!")
	fmt.Println("")
	fmt.Println("Generated:")
	fmt.Println("  channels/example/register.go  - Example channel (delete or modify)")
	fmt.Println("  cmd/worker/main.go            - Worker server entry point")
	fmt.Println("  centrifugo.json               - Centrifugo configuration")
	fmt.Println("")
	fmt.Println("Next steps:")
	fmt.Println("  1. Edit channels/ to define your own channels")
	fmt.Println("  2. Run 'shipq workers' again after adding channels")
	fmt.Println("  3. Start each service in its own terminal:")
	fmt.Println("")
	fmt.Println("Start all services:")
	fmt.Println("  shipq start redis       # in one terminal")
	fmt.Println("  shipq start centrifugo  # in another terminal")
	fmt.Println("  shipq start worker      # in another terminal")
}

// generateRandomKey generates a 32-byte random hex string using crypto/rand.
func generateRandomKey() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// jobResultsMigrationExists checks if the job_results migration already exists.
func jobResultsMigrationExists(migrationsPath string) bool {
	entries, err := os.ReadDir(migrationsPath)
	if err != nil {
		return false
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		for _, suffix := range jobResultsMigrationSuffixes {
			if len(name) > len(suffix) && name[len(name)-len(suffix):] == suffix {
				return true
			}
		}
	}
	return false
}

// extractRedisAddr extracts host:port from a redis URL like "redis://localhost:6379".
// Falls back to "localhost:6379" if parsing fails.
func extractRedisAddr(redisURL string) string {
	// Simple extraction: strip "redis://" prefix
	addr := redisURL
	if len(addr) > 8 && addr[:8] == "redis://" {
		addr = addr[8:]
	}
	if addr == "" {
		return "localhost:6379"
	}
	return addr
}

// generateExampleChannel generates a simple echo channel as documentation/scaffolding.
func generateExampleChannel(modulePath string) string {
	return fmt.Sprintf(`package example

import (
	"%s/shipq/lib/channel"
)

// EchoRequest is the dispatch message sent by the client to start the channel.
type EchoRequest struct {
	Message string `+"`json:\"message\"`"+`
}

// EchoResponse is sent back from the server to the client.
type EchoResponse struct {
	Echo string `+"`json:\"echo\"`"+`
}

// Register registers the example echo channel with the app.
// This is a simple example -- feel free to delete or modify it.
func Register(app *channel.App) {
	app.DefineChannel(
		"example",
		channel.FromClient(EchoRequest{}),
		channel.FromServer(EchoResponse{}),
	).Retries(3).TimeoutSeconds(30)
}
`, modulePath)
}

func generateExampleHandler() string {
	return `package example

import "context"

// HandleEchoRequest is the handler for the echo channel.
// It receives the dispatch message and sends back an echo response.
func HandleEchoRequest(ctx context.Context, req *EchoRequest) error {
	tc := TypedChannelFromContext(ctx)
	return tc.SendEchoResponse(ctx, &EchoResponse{
		Echo: "Echo: " + req.Message,
	})
}
`
}
