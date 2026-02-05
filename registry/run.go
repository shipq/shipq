package registry

import (
	"fmt"
	"os"

	"github.com/shipq/shipq/codegen"
	"github.com/shipq/shipq/codegen/discovery"
	"github.com/shipq/shipq/codegen/handlercompile"
)

// Run executes the full handler compile pipeline:
//  1. Discover API packages
//  2. Generate and run the compile program
//  3. Call CompileRegistry with the results
//
// Parameters:
//   - shipqRoot: directory containing shipq.ini (where api/ directory lives)
//   - goModRoot: directory containing go.mod
//
// This is the function called by CLI commands.
func Run(shipqRoot, goModRoot string) error {
	// Get module path
	modulePath, err := codegen.GetModulePath(goModRoot)
	if err != nil {
		return fmt.Errorf("failed to get module path: %w", err)
	}

	// Discover API packages (in shipq root, but import paths relative to go.mod root)
	apiPkgs, err := discovery.DiscoverAPIPackages(goModRoot, shipqRoot, modulePath)
	if err != nil {
		return fmt.Errorf("failed to discover API packages: %w", err)
	}

	// If no API packages found, nothing to compile
	if len(apiPkgs) == 0 {
		fmt.Fprintln(os.Stderr, "No API packages found with Register functions.")
		return nil
	}

	// Build and run the compile program (uses goModRoot for replace directive)
	cfg := handlercompile.HandlerCompileProgramConfig{
		ModulePath: modulePath,
		APIPkgs:    apiPkgs,
	}

	handlers, err := handlercompile.BuildAndRunHandlerCompileProgram(goModRoot, cfg)
	if err != nil {
		return fmt.Errorf("failed to compile handlers: %w", err)
	}

	// Run the registry compilation (the central codegen hook)
	compileCfg := CompileConfig{
		GoModRoot:  goModRoot,
		ShipqRoot:  shipqRoot,
		ModulePath: modulePath,
		Handlers:   handlers,
	}

	return CompileRegistry(compileCfg)
}
