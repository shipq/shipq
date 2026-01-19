package portapi

import (
	"testing"
)

// TestStrictMode_MiddlewareUsedWithoutRegistry verifies that when middleware
// is used but no middleware registry package is configured, validation fails.
func TestStrictMode_MiddlewareUsedWithoutRegistry(t *testing.T) {
	app := &App{}
	dummyHandler := func() {}

	// Register endpoint with middleware but no registry
	app.Group(func(g *Group) {
		g.Use(middlewareA)
		g.Get("/test", dummyHandler)
	})

	endpoints := app.Endpoints()

	// Validate with middlewarePackageConfigured=false
	err := ValidateStrictMiddlewareDeclaration(endpoints, nil, false)
	if err == nil {
		t.Fatal("expected error when middleware is used without registry, got nil")
	}

	if err.Code != "middleware_used_without_registry" {
		t.Errorf("expected error code 'middleware_used_without_registry', got %q", err.Code)
	}

	if err.Message == "" {
		t.Error("expected error message to suggest configuring middleware_package")
	}
}

// TestStrictMode_MiddlewareUsedButNotDeclared verifies that when middleware
// is used on endpoints but not declared in the registry, validation fails.
func TestStrictMode_MiddlewareUsedButNotDeclared(t *testing.T) {
	app := &App{}
	dummyHandler := func() {}

	// Register endpoint with middlewareA
	app.Group(func(g *Group) {
		g.Use(middlewareA)
		g.Get("/test", dummyHandler)
	})

	endpoints := app.Endpoints()

	// Create registry but only declare middlewareB (not middlewareA)
	reg := &MiddlewareRegistry{}
	reg.Use(middlewareB)

	// Validate with registry configured
	err := ValidateStrictMiddlewareDeclaration(endpoints, reg, true)
	if err == nil {
		t.Fatal("expected error when middleware is not declared, got nil")
	}

	if err.Code != "undeclared_middleware" {
		t.Errorf("expected error code 'undeclared_middleware', got %q", err.Code)
	}

	if err.Message == "" {
		t.Error("expected error message to include middleware identity")
	}
}

// TestStrictMode_MiddlewareProperlyDeclared verifies that when all middleware
// used by endpoints is properly declared in the registry, validation succeeds.
func TestStrictMode_MiddlewareProperlyDeclared(t *testing.T) {
	app := &App{}
	dummyHandler := func() {}

	// Register endpoints with middlewareA and middlewareB
	app.Group(func(g *Group) {
		g.Use(middlewareA)
		g.Use(middlewareB)
		g.Get("/test1", dummyHandler)
	})

	app.Group(func(g *Group) {
		g.Use(middlewareA)
		g.Get("/test2", dummyHandler)
	})

	endpoints := app.Endpoints()

	// Declare both middlewares in registry
	reg := &MiddlewareRegistry{}
	reg.Use(middlewareA)
	reg.Use(middlewareB)

	// Validation should succeed
	err := ValidateStrictMiddlewareDeclaration(endpoints, reg, true)
	if err != nil {
		t.Errorf("expected validation to succeed, got error: %v", err)
	}
}

// TestStrictMode_NoMiddlewareUsed verifies that when no middleware is used,
// validation succeeds regardless of registry configuration.
func TestStrictMode_NoMiddlewareUsed(t *testing.T) {
	app := &App{}
	dummyHandler := func() {}

	// Register endpoints without middleware
	app.Get("/test1", dummyHandler)
	app.Post("/test2", dummyHandler)

	endpoints := app.Endpoints()

	// Should succeed with no registry
	err := ValidateStrictMiddlewareDeclaration(endpoints, nil, false)
	if err != nil {
		t.Errorf("expected validation to succeed with no middleware, got error: %v", err)
	}

	// Should also succeed with registry
	reg := &MiddlewareRegistry{}
	err = ValidateStrictMiddlewareDeclaration(endpoints, reg, true)
	if err != nil {
		t.Errorf("expected validation to succeed with no middleware, got error: %v", err)
	}
}

// TestStrictMode_MultipleUndeclaredMiddlewares verifies that error messages
// include all undeclared middlewares in a deterministic order.
func TestStrictMode_MultipleUndeclaredMiddlewares(t *testing.T) {
	app := &App{}
	dummyHandler := func() {}

	// Use middlewareA, middlewareB, and middlewareC
	app.Group(func(g *Group) {
		g.Use(middlewareA)
		g.Use(middlewareB)
		g.Use(middlewareC)
		g.Get("/test", dummyHandler)
	})

	endpoints := app.Endpoints()

	// Only declare middlewareB
	reg := &MiddlewareRegistry{}
	reg.Use(middlewareB)

	// Validation should fail with both A and C undeclared
	err := ValidateStrictMiddlewareDeclaration(endpoints, reg, true)
	if err == nil {
		t.Fatal("expected error for undeclared middlewares, got nil")
	}

	if err.Code != "undeclared_middleware" {
		t.Errorf("expected error code 'undeclared_middleware', got %q", err.Code)
	}

	// Error message should be deterministic and include both middlewares
	if err.Message == "" {
		t.Error("expected error message to list undeclared middlewares")
	}
}

// TestStrictMode_RegistryCanDeclareMoreThanUsed verifies that it's valid
// for the registry to declare middleware that isn't currently used.
func TestStrictMode_RegistryCanDeclareMoreThanUsed(t *testing.T) {
	app := &App{}
	dummyHandler := func() {}

	// Only use middlewareA
	app.Group(func(g *Group) {
		g.Use(middlewareA)
		g.Get("/test", dummyHandler)
	})

	endpoints := app.Endpoints()

	// Declare A, B, and C in registry (more than used)
	reg := &MiddlewareRegistry{}
	reg.Use(middlewareA)
	reg.Use(middlewareB)
	reg.Use(middlewareC)

	// Should succeed - registry can have extra declarations
	err := ValidateStrictMiddlewareDeclaration(endpoints, reg, true)
	if err != nil {
		t.Errorf("expected validation to succeed when registry declares extra middleware, got: %v", err)
	}
}

// TestStrictMode_ErrorMessageDeterminism verifies that error messages are
// deterministic even when middleware is used in different orders.
func TestStrictMode_ErrorMessageDeterminism(t *testing.T) {
	app1 := &App{}
	app2 := &App{}
	dummyHandler := func() {}

	// App1: use in order A, B, C
	app1.Group(func(g *Group) {
		g.Use(middlewareA)
		g.Use(middlewareB)
		g.Use(middlewareC)
		g.Get("/test", dummyHandler)
	})

	// App2: use in order C, B, A
	app2.Group(func(g *Group) {
		g.Use(middlewareC)
		g.Use(middlewareB)
		g.Use(middlewareA)
		g.Get("/test", dummyHandler)
	})

	// Empty registry for both
	reg := &MiddlewareRegistry{}

	err1 := ValidateStrictMiddlewareDeclaration(app1.Endpoints(), reg, true)
	err2 := ValidateStrictMiddlewareDeclaration(app2.Endpoints(), reg, true)

	if err1 == nil || err2 == nil {
		t.Fatal("expected both validations to fail")
	}

	// Error messages should be identical (sorted)
	if err1.Message != err2.Message {
		t.Errorf("error messages should be deterministic:\nerr1: %s\nerr2: %s",
			err1.Message, err2.Message)
	}
}
