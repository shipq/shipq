package registry

import (
	"fmt"
	"os"

	"github.com/shipq/shipq/codegen"
)

// Run executes the full handler compile pipeline:
//  1. Discover API packages
//  2. Generate and run the compile program
//  3. Call CompileRegistry with the results
//
// This is the function called by CLI commands.
func Run(projectRoot string) error {
	// Get module path
	modulePath, err := codegen.GetModulePath(projectRoot)
	if err != nil {
		return fmt.Errorf("failed to get module path: %w", err)
	}

	// Discover API packages
	apiPkgs, err := codegen.DiscoverAPIPackages(projectRoot, modulePath)
	if err != nil {
		return fmt.Errorf("failed to discover API packages: %w", err)
	}

	// If no API packages found, nothing to compile
	if len(apiPkgs) == 0 {
		fmt.Fprintln(os.Stderr, "No API packages found with Register functions.")
		return nil
	}

	// Build and run the compile program
	cfg := codegen.HandlerCompileProgramConfig{
		ModulePath: modulePath,
		APIPkgs:    apiPkgs,
	}

	handlers, err := codegen.BuildAndRunHandlerCompileProgram(projectRoot, cfg)
	if err != nil {
		return fmt.Errorf("failed to compile handlers: %w", err)
	}

	// Run the registry compilation (the central codegen hook)
	compileCfg := CompileConfig{
		ProjectRoot: projectRoot,
		ModulePath:  modulePath,
		Handlers:    handlers,
	}

	return CompileRegistry(compileCfg)
}
