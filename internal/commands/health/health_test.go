package health

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateHealthEndpoint_CreatesFiles(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := "com.testproject"

	created, err := createHealthEndpoint(tmpDir, modulePath)
	if err != nil {
		t.Fatalf("createHealthEndpoint returned error: %v", err)
	}
	if !created {
		t.Fatal("expected created=true on first call")
	}

	// Assert register.go exists and has correct content
	registerPath := filepath.Join(tmpDir, "api", "health", "register.go")
	regContent, err := os.ReadFile(registerPath)
	if err != nil {
		t.Fatalf("register.go not found: %v", err)
	}
	regStr := string(regContent)

	if !strings.Contains(regStr, "package health") {
		t.Errorf("register.go missing package declaration.\ngot:\n%s", regStr)
	}
	if !strings.Contains(regStr, modulePath+"/shipq/lib/handler") {
		t.Errorf("register.go missing correct handler import path.\nwant substring: %s/shipq/lib/handler\ngot:\n%s",
			modulePath, regStr)
	}
	if !strings.Contains(regStr, `app.Get("/health", HealthCheck)`) {
		t.Errorf("register.go missing HealthCheck registration.\ngot:\n%s", regStr)
	}

	// Assert health_check.go exists and has correct content
	healthCheckPath := filepath.Join(tmpDir, "api", "health", "health_check.go")
	hcContent, err := os.ReadFile(healthCheckPath)
	if err != nil {
		t.Fatalf("health_check.go not found: %v", err)
	}
	hcStr := string(hcContent)

	if !strings.Contains(hcStr, "package health") {
		t.Errorf("health_check.go missing package declaration.\ngot:\n%s", hcStr)
	}
	if !strings.Contains(hcStr, "HealthCheckRequest") {
		t.Errorf("health_check.go missing HealthCheckRequest type.\ngot:\n%s", hcStr)
	}
	if !strings.Contains(hcStr, "HealthCheckResponse") {
		t.Errorf("health_check.go missing HealthCheckResponse type.\ngot:\n%s", hcStr)
	}
	if !strings.Contains(hcStr, `json:"healthy"`) {
		t.Errorf("health_check.go missing json tag for healthy field.\ngot:\n%s", hcStr)
	}
	if !strings.Contains(hcStr, "func HealthCheck(ctx context.Context") {
		t.Errorf("health_check.go missing HealthCheck function.\ngot:\n%s", hcStr)
	}
	if !strings.Contains(hcStr, "httpserver.GetQuerier") {
		t.Errorf("health_check.go should call httpserver.GetQuerier.\ngot:\n%s", hcStr)
	}
	if !strings.Contains(hcStr, "httpserver.Pinger") {
		t.Errorf("health_check.go should type-assert to httpserver.Pinger.\ngot:\n%s", hcStr)
	}
	if !strings.Contains(hcStr, "pinger.Ping()") {
		t.Errorf("health_check.go should call pinger.Ping().\ngot:\n%s", hcStr)
	}
	if !strings.Contains(hcStr, modulePath+"/shipq/lib/httpserver") {
		t.Errorf("health_check.go missing httpserver import path.\nwant substring: %s/shipq/lib/httpserver\ngot:\n%s",
			modulePath, hcStr)
	}
}

func TestCreateHealthEndpoint_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := "com.testproject"

	// First call: creates files
	created, err := createHealthEndpoint(tmpDir, modulePath)
	if err != nil {
		t.Fatalf("first call returned error: %v", err)
	}
	if !created {
		t.Fatal("expected created=true on first call")
	}

	// Overwrite register.go with a sentinel
	registerPath := filepath.Join(tmpDir, "api", "health", "register.go")
	sentinel := []byte("// sentinel — do not overwrite\n")
	if err := os.WriteFile(registerPath, sentinel, 0644); err != nil {
		t.Fatalf("failed to write sentinel: %v", err)
	}

	// Second call: should skip because register.go exists
	created2, err := createHealthEndpoint(tmpDir, modulePath)
	if err != nil {
		t.Fatalf("second call returned error: %v", err)
	}
	if created2 {
		t.Error("expected created=false on second call (idempotent)")
	}

	// Verify sentinel is intact — file was not overwritten
	afterContent, err := os.ReadFile(registerPath)
	if err != nil {
		t.Fatalf("failed to read register.go after second call: %v", err)
	}
	if string(afterContent) != string(sentinel) {
		t.Error("register.go was overwritten on second call; expected idempotent skip")
	}
}

func TestCreateHealthEndpoint_CorrectModulePath(t *testing.T) {
	tests := []struct {
		name       string
		modulePath string
	}{
		{"simple module", "com.myapp"},
		{"github module", "github.com/company/monorepo/services/api"},
		{"nested module", "example.com/org/project"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			created, err := createHealthEndpoint(tmpDir, tt.modulePath)
			if err != nil {
				t.Fatalf("createHealthEndpoint error: %v", err)
			}
			if !created {
				t.Fatal("expected created=true")
			}

			regContent, err := os.ReadFile(filepath.Join(tmpDir, "api", "health", "register.go"))
			if err != nil {
				t.Fatalf("register.go not found: %v", err)
			}
			expectedHandlerImport := tt.modulePath + "/shipq/lib/handler"
			if !strings.Contains(string(regContent), expectedHandlerImport) {
				t.Errorf("register.go missing expected import %q.\ngot:\n%s", expectedHandlerImport, regContent)
			}

			hcContent, err := os.ReadFile(filepath.Join(tmpDir, "api", "health", "health_check.go"))
			if err != nil {
				t.Fatalf("health_check.go not found: %v", err)
			}
			expectedHTTPServerImport := tt.modulePath + "/shipq/lib/httpserver"
			if !strings.Contains(string(hcContent), expectedHTTPServerImport) {
				t.Errorf("health_check.go missing expected import %q.\ngot:\n%s", expectedHTTPServerImport, hcContent)
			}
		})
	}
}

func TestCreateHealthEndpoint_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := "com.testproject"

	// api/health/ should not exist yet
	healthDir := filepath.Join(tmpDir, "api", "health")
	if _, err := os.Stat(healthDir); err == nil {
		t.Fatal("api/health/ should not exist before createHealthEndpoint")
	}

	created, err := createHealthEndpoint(tmpDir, modulePath)
	if err != nil {
		t.Fatalf("createHealthEndpoint error: %v", err)
	}
	if !created {
		t.Fatal("expected created=true")
	}

	// api/health/ should exist now
	info, err := os.Stat(healthDir)
	if err != nil {
		t.Fatalf("api/health/ should exist after createHealthEndpoint: %v", err)
	}
	if !info.IsDir() {
		t.Error("api/health/ should be a directory")
	}
}
