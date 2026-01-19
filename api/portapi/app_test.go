package portapi

import (
	"testing"
)

func TestApp_Register(t *testing.T) {
	dummyHandler := func() {}

	t.Run("Get registers GET endpoint", func(t *testing.T) {
		app := &App{}
		app.Get("/pets", dummyHandler)

		endpoints := app.Endpoints()
		if len(endpoints) != 1 {
			t.Fatalf("expected 1 endpoint, got %d", len(endpoints))
		}
		if endpoints[0].Method != "GET" {
			t.Errorf("Method = %q, want %q", endpoints[0].Method, "GET")
		}
		if endpoints[0].Path != "/pets" {
			t.Errorf("Path = %q, want %q", endpoints[0].Path, "/pets")
		}
	})

	t.Run("Post registers POST endpoint", func(t *testing.T) {
		app := &App{}
		app.Post("/pets", dummyHandler)

		endpoints := app.Endpoints()
		if len(endpoints) != 1 {
			t.Fatalf("expected 1 endpoint, got %d", len(endpoints))
		}
		if endpoints[0].Method != "POST" {
			t.Errorf("Method = %q, want %q", endpoints[0].Method, "POST")
		}
	})

	t.Run("Put registers PUT endpoint", func(t *testing.T) {
		app := &App{}
		app.Put("/pets/{id}", dummyHandler)

		endpoints := app.Endpoints()
		if len(endpoints) != 1 {
			t.Fatalf("expected 1 endpoint, got %d", len(endpoints))
		}
		if endpoints[0].Method != "PUT" {
			t.Errorf("Method = %q, want %q", endpoints[0].Method, "PUT")
		}
	})

	t.Run("Delete registers DELETE endpoint", func(t *testing.T) {
		app := &App{}
		app.Delete("/pets/{id}", dummyHandler)

		endpoints := app.Endpoints()
		if len(endpoints) != 1 {
			t.Fatalf("expected 1 endpoint, got %d", len(endpoints))
		}
		if endpoints[0].Method != "DELETE" {
			t.Errorf("Method = %q, want %q", endpoints[0].Method, "DELETE")
		}
	})

	t.Run("multiple registrations", func(t *testing.T) {
		app := &App{}
		app.Get("/pets", dummyHandler)
		app.Post("/pets", dummyHandler)
		app.Get("/users", dummyHandler)

		endpoints := app.Endpoints()
		if len(endpoints) != 3 {
			t.Errorf("expected 3 endpoints, got %d", len(endpoints))
		}
	})
}

func TestApp_Register_Errors(t *testing.T) {
	t.Run("panics on empty path", func(t *testing.T) {
		app := &App{}
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic, got none")
			}
		}()
		app.Get("", func() {})
	})

	t.Run("panics on path without leading slash", func(t *testing.T) {
		app := &App{}
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic, got none")
			}
		}()
		app.Get("pets", func() {})
	})

	t.Run("panics on nil handler", func(t *testing.T) {
		app := &App{}
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic, got none")
			}
		}()
		app.Get("/pets", nil)
	})
}

func TestApp_Endpoints_ReturnsCopy(t *testing.T) {
	app := &App{}
	app.Get("/pets", func() {})

	endpoints1 := app.Endpoints()
	endpoints2 := app.Endpoints()

	// Modify first copy
	endpoints1[0].Method = "MODIFIED"

	// Second copy should be unaffected
	if endpoints2[0].Method == "MODIFIED" {
		t.Error("Endpoints() should return a copy, not the original slice")
	}

	// Original should also be unaffected
	endpoints3 := app.Endpoints()
	if endpoints3[0].Method == "MODIFIED" {
		t.Error("Endpoints() should return a copy, not the original slice")
	}
}
