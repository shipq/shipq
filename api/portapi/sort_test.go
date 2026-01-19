package portapi

import (
	"testing"
)

func TestSortEndpoints(t *testing.T) {
	t.Run("sorts by method then path", func(t *testing.T) {
		endpoints := []Endpoint{
			{Method: "POST", Path: "/pets"},
			{Method: "GET", Path: "/users"},
			{Method: "GET", Path: "/pets"},
			{Method: "DELETE", Path: "/pets/{id}"},
		}

		sorted := SortEndpoints(endpoints)

		// Expected order: DELETE /pets/{id}, GET /pets, GET /users, POST /pets
		if sorted[0].Method != "DELETE" || sorted[0].Path != "/pets/{id}" {
			t.Errorf("sorted[0] = %s %s, want DELETE /pets/{id}", sorted[0].Method, sorted[0].Path)
		}
		if sorted[1].Method != "GET" || sorted[1].Path != "/pets" {
			t.Errorf("sorted[1] = %s %s, want GET /pets", sorted[1].Method, sorted[1].Path)
		}
		if sorted[2].Method != "GET" || sorted[2].Path != "/users" {
			t.Errorf("sorted[2] = %s %s, want GET /users", sorted[2].Method, sorted[2].Path)
		}
		if sorted[3].Method != "POST" || sorted[3].Path != "/pets" {
			t.Errorf("sorted[3] = %s %s, want POST /pets", sorted[3].Method, sorted[3].Path)
		}
	})

	t.Run("stable sort with same method/path uses handler identity", func(t *testing.T) {
		endpoints := []Endpoint{
			{Method: "GET", Path: "/pets", HandlerPkg: "b/pkg", HandlerName: "B"},
			{Method: "GET", Path: "/pets", HandlerPkg: "a/pkg", HandlerName: "A"},
		}

		sorted := SortEndpoints(endpoints)

		if sorted[0].HandlerPkg != "a/pkg" {
			t.Errorf("sorted[0].HandlerPkg = %q, want %q", sorted[0].HandlerPkg, "a/pkg")
		}
		if sorted[1].HandlerPkg != "b/pkg" {
			t.Errorf("sorted[1].HandlerPkg = %q, want %q", sorted[1].HandlerPkg, "b/pkg")
		}
	})

	t.Run("does not mutate original slice", func(t *testing.T) {
		original := []Endpoint{
			{Method: "POST", Path: "/b"},
			{Method: "GET", Path: "/a"},
		}
		originalCopy := append([]Endpoint(nil), original...)

		_ = SortEndpoints(original)

		for i := range original {
			if original[i].Method != originalCopy[i].Method || original[i].Path != originalCopy[i].Path {
				t.Errorf("original was mutated at index %d", i)
			}
		}
	})

	t.Run("empty slice", func(t *testing.T) {
		sorted := SortEndpoints([]Endpoint{})
		if len(sorted) != 0 {
			t.Errorf("expected empty slice, got %d elements", len(sorted))
		}
	})

	t.Run("nil slice", func(t *testing.T) {
		sorted := SortEndpoints(nil)
		if sorted == nil {
			t.Error("expected non-nil empty slice, got nil")
		}
		if len(sorted) != 0 {
			t.Errorf("expected empty slice, got %d elements", len(sorted))
		}
	})

	t.Run("single element", func(t *testing.T) {
		endpoints := []Endpoint{{Method: "GET", Path: "/pets"}}
		sorted := SortEndpoints(endpoints)
		if len(sorted) != 1 {
			t.Errorf("expected 1 element, got %d", len(sorted))
		}
		if sorted[0].Method != "GET" || sorted[0].Path != "/pets" {
			t.Errorf("sorted[0] = %s %s, want GET /pets", sorted[0].Method, sorted[0].Path)
		}
	})
}

func TestEndpoint_SortKey(t *testing.T) {
	tests := []struct {
		name     string
		endpoint Endpoint
		want     string
	}{
		{
			name:     "basic endpoint",
			endpoint: Endpoint{Method: "GET", Path: "/pets"},
			want:     "GET /pets .",
		},
		{
			name:     "with handler info",
			endpoint: Endpoint{Method: "POST", Path: "/users", HandlerPkg: "example.com/app", HandlerName: "CreateUser"},
			want:     "POST /users example.com/app.CreateUser",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.endpoint.SortKey()
			if got != tt.want {
				t.Errorf("SortKey() = %q, want %q", got, tt.want)
			}
		})
	}
}
