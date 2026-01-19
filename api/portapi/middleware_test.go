package portapi

import (
	"testing"
)

// Test middleware functions with canonical signature
func middlewareA(next func()) func() {
	return func() {
		next()
	}
}

func middlewareB(next func()) func() {
	return func() {
		next()
	}
}

func middlewareC(next func()) func() {
	return func() {
		next()
	}
}

// TestMiddlewareOrdering_NestedGroups verifies that middleware from outer groups
// precedes middleware from inner groups in the correct order.
func TestMiddlewareOrdering_NestedGroups(t *testing.T) {
	app := &App{}
	dummyHandler := func() {}

	app.Group(func(outer *Group) {
		outer.Use(middlewareA)
		outer.Use(middlewareB)

		outer.Group(func(inner *Group) {
			inner.Use(middlewareC)
			inner.Get("/x", dummyHandler)
		})
	})

	endpoints := app.Endpoints()
	if len(endpoints) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(endpoints))
	}

	ep := endpoints[0]
	if ep.Path != "/x" || ep.Method != "GET" {
		t.Fatalf("expected GET /x, got %s %s", ep.Method, ep.Path)
	}

	if len(ep.Middlewares) != 3 {
		t.Fatalf("expected 3 middlewares, got %d", len(ep.Middlewares))
	}

	// Verify ordering: A, B, C
	expected := []string{"middlewareA", "middlewareB", "middlewareC"}
	for i, mw := range ep.Middlewares {
		if mw.Name != expected[i] {
			t.Errorf("middleware[%d]: expected %s, got %s", i, expected[i], mw.Name)
		}
	}
}

// TestMiddlewareOrdering_WithinGroup verifies that middleware within a single group
// is preserved in the order of Use() calls.
func TestMiddlewareOrdering_WithinGroup(t *testing.T) {
	app := &App{}
	dummyHandler := func() {}

	app.Group(func(g *Group) {
		g.Use(middlewareA)
		g.Use(middlewareB)
		g.Use(middlewareC)
		g.Get("/test", dummyHandler)
	})

	endpoints := app.Endpoints()
	if len(endpoints) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(endpoints))
	}

	ep := endpoints[0]
	if len(ep.Middlewares) != 3 {
		t.Fatalf("expected 3 middlewares, got %d", len(ep.Middlewares))
	}

	// Verify ordering: A, B, C
	expected := []string{"middlewareA", "middlewareB", "middlewareC"}
	for i, mw := range ep.Middlewares {
		if mw.Name != expected[i] {
			t.Errorf("middleware[%d]: expected %s, got %s", i, expected[i], mw.Name)
		}
	}
}

// TestMiddlewareSliceIsolation verifies that child groups do not mutate
// the parent group's middleware slice (no slice aliasing bug).
func TestMiddlewareSliceIsolation(t *testing.T) {
	app := &App{}
	dummyHandler := func() {}

	app.Group(func(outer *Group) {
		outer.Use(middlewareA)

		// Create inner group and add middleware
		outer.Group(func(inner *Group) {
			inner.Use(middlewareB)
			inner.Get("/inner", dummyHandler)
		})

		// Register endpoint in outer group after inner group is defined
		outer.Get("/outer", dummyHandler)
	})

	endpoints := app.Endpoints()
	if len(endpoints) != 2 {
		t.Fatalf("expected 2 endpoints, got %d", len(endpoints))
	}

	// Find /inner and /outer endpoints
	var innerEp, outerEp *Endpoint
	for i := range endpoints {
		if endpoints[i].Path == "/inner" {
			innerEp = &endpoints[i]
		}
		if endpoints[i].Path == "/outer" {
			outerEp = &endpoints[i]
		}
	}

	if innerEp == nil || outerEp == nil {
		t.Fatal("could not find both /inner and /outer endpoints")
	}

	// /inner should have [A, B]
	if len(innerEp.Middlewares) != 2 {
		t.Errorf("/inner: expected 2 middlewares, got %d", len(innerEp.Middlewares))
	} else {
		if innerEp.Middlewares[0].Name != "middlewareA" {
			t.Errorf("/inner middleware[0]: expected middlewareA, got %s", innerEp.Middlewares[0].Name)
		}
		if innerEp.Middlewares[1].Name != "middlewareB" {
			t.Errorf("/inner middleware[1]: expected middlewareB, got %s", innerEp.Middlewares[1].Name)
		}
	}

	// /outer should have [A] (not [A, B])
	if len(outerEp.Middlewares) != 1 {
		t.Errorf("/outer: expected 1 middleware, got %d", len(outerEp.Middlewares))
	} else {
		if outerEp.Middlewares[0].Name != "middlewareA" {
			t.Errorf("/outer middleware[0]: expected middlewareA, got %s", outerEp.Middlewares[0].Name)
		}
	}
}
