package workers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shipq/shipq/cli"
	"github.com/shipq/shipq/codegen"
	"github.com/shipq/shipq/codegen/channelcompile"
	"github.com/shipq/shipq/codegen/channelgen"
	"github.com/shipq/shipq/codegen/llmgen"
	"github.com/shipq/shipq/dburl"
	"github.com/shipq/shipq/inifile"
	"github.com/shipq/shipq/internal/commands/db"
	"github.com/shipq/shipq/internal/commands/shared"
	shipqdag "github.com/shipq/shipq/internal/dag"
	"github.com/shipq/shipq/project"
	registrypkg "github.com/shipq/shipq/registry"
)

// WorkersCompileCmd implements the "shipq workers compile" subcommand.
// It performs only the codegen-related steps of the workers pipeline:
//
//  1. Channel discovery (static analysis of channels/*/register.go)
//  2. Querydef generation for job_results
//  3. Query compilation (shipq handler compile equivalent)
//  4. Channel registry compilation
//  5. Typed channel code generation
//  6. Worker main generation
//  7. Centrifugo config generation
//  8. Handler registry compilation (regenerates HTTP server + channel routes)
//  9. TypeScript client generation (if frontend channels exist)
//  10. Integration and E2E test generation
//
// It intentionally skips: prerequisite checks, migration execution, go mod tidy,
// embedding, and example channel scaffolding. This makes it much faster for
// iterating on channel definitions after the initial `shipq workers` setup.
func WorkersCompileCmd() {
	roots, err := project.FindProjectRoots()
	if err != nil {
		cli.FatalErr("failed to find project roots", err)
	}

	// DAG prerequisite check (alongside existing checks)
	if !shipqdag.CheckPrerequisites(shipqdag.CmdWorkersCompile, roots.ShipqRoot) {
		os.Exit(1)
	}

	shipqIniPath := filepath.Join(roots.ShipqRoot, project.ShipqIniFile)
	ini, err := inifile.ParseFile(shipqIniPath)
	if err != nil {
		cli.FatalErr("failed to parse shipq.ini", err)
	}

	// Verify [workers] section exists (initial setup must have been run)
	if ini.Section("workers") == nil {
		cli.Fatal("shipq workers compile requires an existing [workers] section in shipq.ini — run 'shipq workers' first")
	}

	moduleInfo, err := codegen.GetModuleInfo(roots.GoModRoot, roots.ShipqRoot)
	if err != nil {
		cli.FatalErr("failed to determine Go module info", err)
	}
	importPrefix := moduleInfo.FullImportPath("")

	scopeColumn := ini.Get("db", "scope")
	hasTenancy := scopeColumn != ""
	hasAuth := shared.IsFeatureEnabled(ini, "auth")

	databaseURL := ini.Get("db", "database_url")
	dialect := ""
	if databaseURL != "" {
		if d, err := dburl.InferDialectFromDBUrl(databaseURL); err == nil {
			dialect = d
		}
	}

	redisURL := ini.Get("workers", "redis_url")
	centrifugoAPIURL := ini.Get("workers", "centrifugo_api_url")
	centrifugoAPIKey := ini.Get("workers", "centrifugo_api_key")
	centrifugoHMACSecret := ini.Get("workers", "centrifugo_hmac_secret")
	centrifugoWSURL := ini.Get("workers", "centrifugo_ws_url")

	cli.Infof("Project: %s", importPrefix)

	// ── Step 1: Generate job_results querydefs ───────────────────────

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

	// ── Step 2: Compile queries ──────────────────────────────────────

	fmt.Println("")
	fmt.Println("Compiling queries...")
	db.DBCompileCmd()

	// ── Step 3: Compile channel registry ─────────────────────────────

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

	// ── Step 4: Generate typed channel code ──────────────────────────

	fmt.Println("")
	fmt.Println("Generating typed channel code...")

	if err := channelgen.GenerateAllTypedChannels(channels, roots.GoModRoot, roots.ShipqRoot, importPrefix); err != nil {
		cli.FatalErr("failed to generate typed channels", err)
	}
	fmt.Println("  Generated typed channel files")

	// ── Step 5: Generate worker server ───────────────────────────────

	fmt.Println("")
	fmt.Println("Generating worker server...")

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

	// ── Step 6: Generate Centrifugo config ───────────────────────────

	fmt.Println("")
	fmt.Println("Generating Centrifugo config...")

	if err := channelgen.WriteCentrifugoConfig(channels, roots.ShipqRoot, centrifugoAPIKey, centrifugoHMACSecret); err != nil {
		cli.FatalErr("failed to generate centrifugo config", err)
	}
	fmt.Println("  Generated centrifugo.json")

	// ── Step 7: Compile handler registry ─────────────────────────────

	fmt.Println("")
	fmt.Println("Compiling handler registry...")

	if err := registrypkg.Run(roots.ShipqRoot, roots.GoModRoot); err != nil {
		cli.FatalErr("failed to compile registry", err)
	}
	fmt.Println("  Handler registry compiled")

	// ── Step 8: Generate TypeScript client ────────────────────────────

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
		tsFrameworks := registrypkg.ParseFrameworks(ini.Get("typescript", "framework"))

		if registrypkg.HasFramework(tsFrameworks, "react") {
			if err := channelgen.WriteReactChannelHooks(channels, roots.ShipqRoot, tsOutputDir, llmCfg); err != nil {
				cli.FatalErr("failed to generate React channel hooks", err)
			}
			fmt.Printf("  Generated %s/react/shipq-channels.ts\n", tsOutputDir)
		}

		if registrypkg.HasFramework(tsFrameworks, "svelte") {
			if err := channelgen.WriteSvelteChannelHooks(channels, roots.ShipqRoot, tsOutputDir, llmCfg); err != nil {
				cli.FatalErr("failed to generate Svelte channel hooks", err)
			}
			fmt.Printf("  Generated %s/svelte/shipq-channels.ts\n", tsOutputDir)
		}
	}

	// ── Step 9: Generate tests ───────────────────────────────────────

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
	cli.Success("Workers compile completed successfully!")
}
