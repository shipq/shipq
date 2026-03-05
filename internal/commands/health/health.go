package health

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/shipq/shipq/cli"
	"github.com/shipq/shipq/codegen"
	shipqdag "github.com/shipq/shipq/internal/dag"
	"github.com/shipq/shipq/project"
	"github.com/shipq/shipq/registry"
)

// HealthCmd implements the "shipq health" command.
// It generates the api/health/ endpoint files (register.go and health_check.go)
// and then runs handler compile (registry.Run) so that the health endpoint is
// wired into the generated HTTP server, OpenAPI spec, and TypeScript client.
//
// This is the same scaffolding that "shipq init" performs, but available as a
// standalone command for projects that were initialised before the health
// endpoint existed, or where the files were accidentally deleted.
//
// The endpoint scaffolding is idempotent: if api/health/register.go already
// exists, it is not overwritten. The handler compile step always runs so that
// any new or changed handlers are picked up.
func HealthCmd() {
	cwd, err := os.Getwd()
	if err != nil {
		cli.FatalErr("failed to get current directory", err)
	}

	roots, err := project.FindProjectRootsFrom(cwd)
	if err != nil {
		cli.FatalErr("failed to find project roots", err)
	}

	// DAG prerequisite check — health requires init (same as handler compile).
	if !shipqdag.CheckPrerequisites(shipqdag.CmdHealth, roots.ShipqRoot) {
		os.Exit(1)
	}

	moduleInfo, err := codegen.GetModuleInfo(roots.GoModRoot, roots.ShipqRoot)
	if err != nil {
		cli.FatalErr("failed to read module info", err)
	}

	modulePath := moduleInfo.FullImportPath("")

	created, err := createHealthEndpoint(roots.ShipqRoot, modulePath)
	if err != nil {
		cli.FatalErr("failed to create health endpoint", err)
	}

	if created {
		cli.Success("Created api/health/ (healthcheck endpoint)")
		cli.Info("  Created api/health/register.go")
		cli.Info("  Created api/health/health_check.go")
	} else {
		cli.Info("api/health/register.go already exists — nothing to do")
	}

	// Run handler compile so the health endpoint (and any other handlers) are
	// wired into the generated HTTP server, OpenAPI spec, and TypeScript client.
	fmt.Println("")
	fmt.Println("Running handler compile...")
	if err := registry.Run(roots.ShipqRoot, roots.GoModRoot); err != nil {
		cli.FatalErr("handler compile failed", err)
	}
	cli.Success("Handler compile complete")
}

// createHealthEndpoint scaffolds api/health/register.go and api/health/health_check.go
// using the provided module path (e.g. "com.myproject"). It is idempotent: if
// api/health/register.go already exists the function returns (false, nil).
func createHealthEndpoint(dir, modulePath string) (bool, error) {
	healthDir := filepath.Join(dir, "api", "health")
	registerPath := filepath.Join(healthDir, "register.go")

	// Idempotency: skip if register.go already exists
	if _, err := os.Stat(registerPath); err == nil {
		return false, nil
	}

	if err := os.MkdirAll(healthDir, 0755); err != nil {
		return false, fmt.Errorf("failed to create api/health directory: %w", err)
	}

	registerContent := fmt.Sprintf(`package health

import (
	"%s/shipq/lib/handler"
)

func Register(app *handler.App) {
	app.Get("/health", HealthCheck)
}
`, modulePath)

	if err := os.WriteFile(registerPath, []byte(registerContent), 0644); err != nil {
		return false, fmt.Errorf("failed to write register.go: %w", err)
	}

	healthCheckContent := fmt.Sprintf(`package health

import (
	"context"

	"%s/shipq/lib/httpserver"
)

type HealthCheckRequest struct{}

type HealthCheckResponse struct {
	Healthy bool `+"`"+`json:"healthy"`+"`"+`
}

func HealthCheck(ctx context.Context, req *HealthCheckRequest) (*HealthCheckResponse, error) {
	q := httpserver.GetQuerier(ctx)
	if pinger, ok := q.(httpserver.Pinger); ok {
		if err := pinger.Ping(); err != nil {
			return &HealthCheckResponse{Healthy: false}, nil
		}
	}
	return &HealthCheckResponse{Healthy: true}, nil
}
`, modulePath)

	healthCheckPath := filepath.Join(healthDir, "health_check.go")
	if err := os.WriteFile(healthCheckPath, []byte(healthCheckContent), 0644); err != nil {
		return false, fmt.Errorf("failed to write health_check.go: %w", err)
	}

	return true, nil
}
