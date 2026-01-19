package portapi

import (
	"testing"
)

func TestEndpoint_NewEndpoint(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		wantErr    bool
		wantMethod string // normalized
		wantPath   string // normalized
	}{
		// Valid cases
		{name: "GET lowercase", method: "get", path: "/pets", wantMethod: "GET", wantPath: "/pets"},
		{name: "POST mixed case", method: "Post", path: "/pets", wantMethod: "POST", wantPath: "/pets"},
		{name: "PUT uppercase", method: "PUT", path: "/pets/{id}", wantMethod: "PUT", wantPath: "/pets/{id}"},
		{name: "DELETE", method: "DELETE", path: "/pets/{id}", wantMethod: "DELETE", wantPath: "/pets/{id}"},

		// Path normalization
		{name: "trailing slash removed", method: "GET", path: "/pets/", wantMethod: "GET", wantPath: "/pets"},
		{name: "root path stays", method: "GET", path: "/", wantMethod: "GET", wantPath: "/"},

		// Invalid cases
		{name: "empty method", method: "", path: "/pets", wantErr: true},
		{name: "unsupported method", method: "PATCH", path: "/pets", wantErr: true},
		{name: "empty path", method: "GET", path: "", wantErr: true},
		{name: "path without leading slash", method: "GET", path: "pets", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ep, err := NewEndpoint(tt.method, tt.path, func() {})

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if ep.Method != tt.wantMethod {
				t.Errorf("Method = %q, want %q", ep.Method, tt.wantMethod)
			}
			if ep.Path != tt.wantPath {
				t.Errorf("Path = %q, want %q", ep.Path, tt.wantPath)
			}
		})
	}
}

func TestEndpoint_NewEndpoint_PreservesHandler(t *testing.T) {
	handler := func() string { return "test" }
	ep, err := NewEndpoint("GET", "/test", handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ep.Handler == nil {
		t.Error("Handler should not be nil")
	}
}

func TestEndpoint_NewEndpoint_WhitespaceHandling(t *testing.T) {
	ep, err := NewEndpoint("  GET  ", "  /pets  ", func() {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ep.Method != "GET" {
		t.Errorf("Method = %q, want %q", ep.Method, "GET")
	}
	if ep.Path != "/pets" {
		t.Errorf("Path = %q, want %q", ep.Path, "/pets")
	}
}

func TestEndpoint_PathVariables(t *testing.T) {
	tests := []struct {
		path string
		want []string
	}{
		{"/pets", nil},
		{"/pets/{id}", []string{"id"}},
		{"/users/{user_id}/posts/{post_id}", []string{"user_id", "post_id"}},
		{"/files/{path...}", []string{"path"}}, // wildcard suffix
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			ep := Endpoint{Path: tt.path}
			got := ep.PathVariables()

			if tt.want == nil {
				if got != nil {
					t.Errorf("PathVariables() = %v, want nil", got)
				}
				return
			}

			if len(got) != len(tt.want) {
				t.Errorf("PathVariables() = %v, want %v", got, tt.want)
				return
			}

			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("PathVariables()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestEndpoint_MuxPattern(t *testing.T) {
	tests := []struct {
		method string
		path   string
		want   string
	}{
		{"GET", "/pets", "GET /pets"},
		{"POST", "/pets", "POST /pets"},
		{"GET", "/pets/{id}", "GET /pets/{id}"},
		{"DELETE", "/users/{user_id}/posts/{post_id}", "DELETE /users/{user_id}/posts/{post_id}"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			ep := Endpoint{Method: tt.method, Path: tt.path}
			got := ep.MuxPattern()
			if got != tt.want {
				t.Errorf("MuxPattern() = %q, want %q", got, tt.want)
			}
		})
	}
}
