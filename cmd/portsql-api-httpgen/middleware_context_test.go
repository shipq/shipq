package main

import (
	"strings"
	"testing"
)

// TestContextKeyNameGeneration tests the conversion of snake_case keys to CamelCase identifiers.
func TestContextKeyNameGeneration(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected string
	}{
		{
			name:     "simple lowercase",
			key:      "user",
			expected: "User",
		},
		{
			name:     "snake_case with two parts",
			key:      "request_id",
			expected: "RequestID",
		},
		{
			name:     "snake_case with multiple parts",
			key:      "user_auth_token",
			expected: "UserAuthToken",
		},
		{
			name:     "id initialism",
			key:      "session_id",
			expected: "SessionID",
		},
		{
			name:     "url initialism",
			key:      "redirect_url",
			expected: "RedirectURL",
		},
		{
			name:     "http initialism",
			key:      "http_method",
			expected: "HTTPMethod",
		},
		{
			name:     "ip initialism",
			key:      "client_ip",
			expected: "ClientIP",
		},
		{
			name:     "api initialism",
			key:      "api_key",
			expected: "APIKey",
		},
		{
			name:     "html initialism",
			key:      "html_content",
			expected: "HTMLContent",
		},
		{
			name:     "json initialism",
			key:      "json_data",
			expected: "JSONData",
		},
		{
			name:     "xml initialism",
			key:      "xml_parser",
			expected: "XMLParser",
		},
		{
			name:     "sql initialism",
			key:      "sql_query",
			expected: "SQLQuery",
		},
		{
			name:     "multiple underscores",
			key:      "retry_count",
			expected: "RetryCount",
		},
		{
			name:     "all common initialism",
			key:      "id",
			expected: "ID",
		},
		{
			name:     "all common initialism lowercase",
			key:      "url",
			expected: "URL",
		},
		{
			name:     "mixed case preserved correctly",
			key:      "user_id_list",
			expected: "UserIDList",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := contextKeyToCamelCase(tt.key)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("contextKeyToCamelCase(%q) = %q, want %q", tt.key, result, tt.expected)
			}
		})
	}
}

// TestContextKeyNameGeneration_Invalid tests error cases.
func TestContextKeyNameGeneration_Invalid(t *testing.T) {
	tests := []struct {
		name        string
		key         string
		expectedErr string
	}{
		{
			name:        "empty string",
			key:         "",
			expectedErr: "invalid_context_key",
		},
		{
			name:        "starts with underscore",
			key:         "_user",
			expectedErr: "invalid_context_key",
		},
		{
			name:        "trailing underscore",
			key:         "user_",
			expectedErr: "invalid_context_key",
		},
		{
			name:        "double underscore",
			key:         "user__id",
			expectedErr: "invalid_context_key",
		},
		{
			name:        "uppercase letter",
			key:         "userId",
			expectedErr: "invalid_context_key",
		},
		{
			name:        "starts with number",
			key:         "123user",
			expectedErr: "invalid_context_key",
		},
		{
			name:        "contains hyphen",
			key:         "user-id",
			expectedErr: "invalid_context_key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := contextKeyToCamelCase(tt.key)
			if err == nil {
				t.Fatalf("expected error for key %q, got nil", tt.key)
			}
			gerr, ok := err.(*GeneratorError)
			if !ok {
				t.Fatalf("expected *GeneratorError, got %T", err)
			}
			if gerr.Code != tt.expectedErr {
				t.Errorf("expected error code %q, got %q", tt.expectedErr, gerr.Code)
			}
		})
	}
}

// TestDetectContextKeyCollisions tests collision detection.
func TestDetectContextKeyCollisions(t *testing.T) {
	tests := []struct {
		name        string
		keys        []ManifestContextKey
		expectError bool
		errorCode   string
	}{
		{
			name: "no collisions",
			keys: []ManifestContextKey{
				{Key: "user", Type: "string"},
				{Key: "request_id", Type: "string"},
				{Key: "retry_count", Type: "int"},
			},
			expectError: false,
		},
		{
			name: "collision between user_id and user__id if both were allowed",
			keys: []ManifestContextKey{
				{Key: "user_id", Type: "string"},
			},
			expectError: false, // Only one key, so no collision
		},
		{
			name: "no collision with different valid keys",
			keys: []ManifestContextKey{
				{Key: "user", Type: "string"},
				{Key: "user_id", Type: "string"},
				{Key: "user_name", Type: "string"},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := detectContextKeyCollisions(tt.keys)
			if tt.expectError {
				if err == nil {
					t.Fatal("expected error but got nil")
				}
				gerr, ok := err.(*GeneratorError)
				if !ok {
					t.Fatalf("expected *GeneratorError, got %T", err)
				}
				if gerr.Code != tt.errorCode {
					t.Errorf("expected error code %q, got %q", tt.errorCode, gerr.Code)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// TestGenerateFunctionNames tests the generation of function names from context keys.
// Note: We no longer generate zzCtxKey types; this test verifies CamelCase naming for functions.
func TestGenerateFunctionNames(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		expectedWith string
		expectedGet  string
		expectedMust string
	}{
		{
			name:         "simple key",
			key:          "user",
			expectedWith: "WithUser",
			expectedGet:  "User",
			expectedMust: "MustUser",
		},
		{
			name:         "snake_case key",
			key:          "request_id",
			expectedWith: "WithRequestID",
			expectedGet:  "RequestID",
			expectedMust: "MustRequestID",
		},
		{
			name:         "multiple parts",
			key:          "user_auth_token",
			expectedWith: "WithUserAuthToken",
			expectedGet:  "UserAuthToken",
			expectedMust: "MustUserAuthToken",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			camelCase, err := contextKeyToCamelCase(tt.key)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			withFunc := "With" + camelCase
			getFunc := camelCase
			mustFunc := "Must" + camelCase

			if withFunc != tt.expectedWith {
				t.Errorf("With function for %q = %q, want %q", tt.key, withFunc, tt.expectedWith)
			}
			if getFunc != tt.expectedGet {
				t.Errorf("Get function for %q = %q, want %q", tt.key, getFunc, tt.expectedGet)
			}
			if mustFunc != tt.expectedMust {
				t.Errorf("Must function for %q = %q, want %q", tt.key, mustFunc, tt.expectedMust)
			}
		})
	}
}

// TestContextHelperFunctionNames tests generation of function names.
func TestContextHelperFunctionNames(t *testing.T) {
	tests := []struct {
		name        string
		key         string
		withFunc    string
		getFunc     string
		mustGetFunc string
	}{
		{
			name:        "user",
			key:         "user",
			withFunc:    "WithUser",
			getFunc:     "User",
			mustGetFunc: "MustUser",
		},
		{
			name:        "request_id",
			key:         "request_id",
			withFunc:    "WithRequestID",
			getFunc:     "RequestID",
			mustGetFunc: "MustRequestID",
		},
		{
			name:        "retry_count",
			key:         "retry_count",
			withFunc:    "WithRetryCount",
			getFunc:     "RetryCount",
			mustGetFunc: "MustRetryCount",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			camelCase, err := contextKeyToCamelCase(tt.key)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			withFunc := "With" + camelCase
			getFunc := camelCase
			mustGetFunc := "Must" + camelCase

			if withFunc != tt.withFunc {
				t.Errorf("with function name = %q, want %q", withFunc, tt.withFunc)
			}
			if getFunc != tt.getFunc {
				t.Errorf("get function name = %q, want %q", getFunc, tt.getFunc)
			}
			if mustGetFunc != tt.mustGetFunc {
				t.Errorf("must get function name = %q, want %q", mustGetFunc, tt.mustGetFunc)
			}
		})
	}
}

// TestGenerateMiddlewareContextFile_NoKeys tests that no file content is generated when there are no keys.
func TestGenerateMiddlewareContextFile_NoKeys(t *testing.T) {
	content, err := generateMiddlewareContextFile("middleware", []ManifestContextKey{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "" {
		t.Error("expected empty content when there are no context keys")
	}
}

// TestGenerateMiddlewareContextFile_WithKeys tests file generation with keys.
func TestGenerateMiddlewareContextFile_WithKeys(t *testing.T) {
	keys := []ManifestContextKey{
		{Key: "user", Type: "*User"},
		{Key: "request_id", Type: "string"},
	}

	content, err := generateMiddlewareContextFile("middleware", keys)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if content == "" {
		t.Fatal("expected non-empty content")
	}

	// Verify it contains package declaration
	if !contains(content, "package middleware") {
		t.Error("expected package declaration")
	}

	// Verify it contains context import
	if !contains(content, `"context"`) {
		t.Error("expected context import")
	}

	// Verify it uses portapi (not per-key context types)
	if !contains(content, `"github.com/shipq/shipq/api/portapi"`) {
		t.Error("expected portapi import")
	}

	// Verify it contains functions using portapi
	if !contains(content, "func WithUser(ctx context.Context, v *User) context.Context") {
		t.Error("expected WithUser function")
	}
	if !contains(content, "func User(ctx context.Context) (*User, bool)") {
		t.Error("expected User function")
	}
	if !contains(content, "func MustUser(ctx context.Context) *User") {
		t.Error("expected MustUser function")
	}
	if !contains(content, "portapi.WithTyped") {
		t.Error("expected portapi.WithTyped usage")
	}
	if !contains(content, "portapi.GetTyped") {
		t.Error("expected portapi.GetTyped usage")
	}
}

// Helper function to check if a string contains a substring.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestGenerateMiddlewareContextFile_UsesPortapiStore verifies that generated helpers
// use the stable portapi store (portapi.WithTyped/GetTyped) instead of per-key context types.
// This ensures interoperability between generated helpers and capability tokens.
func TestGenerateMiddlewareContextFile_UsesPortapiStore(t *testing.T) {
	keys := []ManifestContextKey{
		{Key: "request_id", Type: "string"},
	}

	content, err := generateMiddlewareContextFile("middleware", keys)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should import portapi
	if !strings.Contains(content, `"github.com/shipq/shipq/api/portapi"`) {
		t.Error("expected portapi import")
	}

	// Should use portapi.WithTyped for With functions
	if !strings.Contains(content, "portapi.WithTyped") {
		t.Error("expected WithRequestID to use portapi.WithTyped")
	}

	// Should use portapi.GetTyped for Get functions
	if !strings.Contains(content, "portapi.GetTyped") {
		t.Error("expected RequestID to use portapi.GetTyped")
	}

	// Should NOT contain per-key context types (the old pattern)
	if strings.Contains(content, "type zzCtxKey") {
		t.Error("should not generate per-key context types anymore")
	}

	// Should NOT use context.WithValue directly
	if strings.Contains(content, "context.WithValue") {
		t.Error("should not use context.WithValue directly")
	}

	// Should NOT use ctx.Value directly
	if strings.Contains(content, "ctx.Value(") {
		t.Error("should not use ctx.Value directly")
	}
}

// TestGenerateMiddlewareContextFile_PortapiInterop verifies that the generated code
// would interoperate with capability tokens by using the same key strings.
func TestGenerateMiddlewareContextFile_PortapiInterop(t *testing.T) {
	keys := []ManifestContextKey{
		{Key: "current_user", Type: "*User"},
		{Key: "tenant_id", Type: "string"},
	}

	content, err := generateMiddlewareContextFile("middleware", keys)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The generated code should use the exact key strings for portapi calls
	// This ensures that Cap[T].With(ctx, "current_user", ...) and WithCurrentUser(ctx, ...)
	// write to the same location.
	if !strings.Contains(content, `"current_user"`) {
		t.Error("expected generated code to use exact key string \"current_user\"")
	}
	if !strings.Contains(content, `"tenant_id"`) {
		t.Error("expected generated code to use exact key string \"tenant_id\"")
	}
}
