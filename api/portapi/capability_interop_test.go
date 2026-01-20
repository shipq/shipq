package portapi

import (
	"context"
	"testing"
)

// Phase 4 — Interoperability Tests
//
// These tests verify that capability tokens and generated helpers (which both use
// the stable portapi context store) can interoperate correctly. This is critical
// for the bootstrap story: middleware sets values via Cap[T], handlers read via
// generated helpers.

// TestInterop_TokenWriteTypedRead verifies that values set via Cap[T].With
// can be read via GetTyped (simulating what generated helpers do).
func TestInterop_TokenWriteTypedRead(t *testing.T) {
	reg := &MiddlewareRegistry{}

	// Simulate middleware package declaring a key and getting a capability
	userKey := NewContextKey[string]("current_user")
	userCap, err := userKey.Provide(reg)
	if err != nil {
		t.Fatalf("Provide failed: %v", err)
	}

	// Middleware sets the value using the capability token
	ctx := context.Background()
	ctx = userCap.With(ctx, "alice")

	// Handler reads using GetTyped (what generated helpers do internally)
	val, ok := GetTyped[string](ctx, "current_user")
	if !ok {
		t.Fatal("expected GetTyped to return ok=true")
	}
	if val != "alice" {
		t.Errorf("expected 'alice', got %q", val)
	}
}

// TestInterop_TypedWriteTokenRead verifies that values set via WithTyped
// (what generated helpers do) can be read via Cap[T].Get.
func TestInterop_TypedWriteTokenRead(t *testing.T) {
	reg := &MiddlewareRegistry{}

	// Simulate middleware package declaring a key and getting a capability
	reqIDKey := NewContextKey[string]("request_id")
	reqIDCap, err := reqIDKey.Provide(reg)
	if err != nil {
		t.Fatalf("Provide failed: %v", err)
	}

	// Handler sets the value using WithTyped (what generated WithRequestID does)
	ctx := context.Background()
	ctx = WithTyped(ctx, "request_id", "req-123")

	// Middleware reads using the capability token
	val, ok := reqIDCap.Get(ctx)
	if !ok {
		t.Fatal("expected Cap.Get to return ok=true")
	}
	if val != "req-123" {
		t.Errorf("expected 'req-123', got %q", val)
	}
}

// TestInterop_PointerTypes verifies interop works with pointer types.
func TestInterop_PointerTypes(t *testing.T) {
	type User struct {
		ID   int
		Name string
	}

	reg := &MiddlewareRegistry{}

	userKey := NewContextKey[*User]("current_user")
	userCap, err := userKey.Provide(reg)
	if err != nil {
		t.Fatalf("Provide failed: %v", err)
	}

	user := &User{ID: 42, Name: "Bob"}

	// Middleware sets via token
	ctx := userCap.With(context.Background(), user)

	// Handler reads via GetTyped (simulating generated helper)
	retrieved, ok := GetTyped[*User](ctx, "current_user")
	if !ok {
		t.Fatal("expected GetTyped to return ok=true")
	}
	if retrieved != user {
		t.Error("expected same pointer")
	}
	if retrieved.ID != 42 || retrieved.Name != "Bob" {
		t.Errorf("unexpected user: %+v", retrieved)
	}
}

// TestInterop_MustVariants verifies Must functions work consistently.
func TestInterop_MustVariants(t *testing.T) {
	reg := &MiddlewareRegistry{}

	key := NewContextKey[int]("counter")
	cap, _ := key.Provide(reg)

	// Set via token
	ctx := cap.With(context.Background(), 99)

	// Read via MustTyped (what generated MustCounter does)
	val := MustTyped[int](ctx, "counter")
	if val != 99 {
		t.Errorf("expected 99, got %d", val)
	}

	// Also read via Cap.Must
	val2 := cap.Must(ctx)
	if val2 != 99 {
		t.Errorf("expected 99, got %d", val2)
	}
}

// TestInterop_MultipleKeys verifies multiple keys can be set and read via mixed methods.
func TestInterop_MultipleKeys(t *testing.T) {
	reg := &MiddlewareRegistry{}

	reqIDKey := NewContextKey[string]("request_id")
	tenantKey := NewContextKey[string]("tenant_id")
	userIDKey := NewContextKey[int]("user_id")

	reqIDCap, _ := reqIDKey.Provide(reg)
	tenantCap, _ := tenantKey.Provide(reg)
	userIDCap, _ := userIDKey.Provide(reg)

	ctx := context.Background()

	// Set via mixed methods
	ctx = reqIDCap.With(ctx, "req-abc")             // token
	ctx = WithTyped(ctx, "tenant_id", "tenant-xyz") // typed helper
	ctx = userIDCap.With(ctx, 123)                  // token

	// Read via mixed methods
	reqID, ok := GetTyped[string](ctx, "request_id") // typed helper
	if !ok || reqID != "req-abc" {
		t.Errorf("request_id: expected 'req-abc', got %q (ok=%v)", reqID, ok)
	}

	tenant, ok := tenantCap.Get(ctx) // token
	if !ok || tenant != "tenant-xyz" {
		t.Errorf("tenant_id: expected 'tenant-xyz', got %q (ok=%v)", tenant, ok)
	}

	userID := MustTyped[int](ctx, "user_id") // typed helper
	if userID != 123 {
		t.Errorf("user_id: expected 123, got %d", userID)
	}
}

// TestInterop_OverwriteViaToken verifies that overwriting via token is visible to typed helpers.
func TestInterop_OverwriteViaToken(t *testing.T) {
	reg := &MiddlewareRegistry{}

	key := NewContextKey[string]("state")
	cap, _ := key.Provide(reg)

	ctx := context.Background()

	// Set initial value via typed helper
	ctx = WithTyped(ctx, "state", "initial")

	// Overwrite via token
	ctx = cap.With(ctx, "updated")

	// Read via typed helper should see updated value
	val, ok := GetTyped[string](ctx, "state")
	if !ok || val != "updated" {
		t.Errorf("expected 'updated', got %q (ok=%v)", val, ok)
	}
}

// TestInterop_OverwriteViaTyped verifies that overwriting via typed helper is visible to token.
func TestInterop_OverwriteViaTyped(t *testing.T) {
	reg := &MiddlewareRegistry{}

	key := NewContextKey[string]("state")
	cap, _ := key.Provide(reg)

	ctx := context.Background()

	// Set initial value via token
	ctx = cap.With(ctx, "initial")

	// Overwrite via typed helper
	ctx = WithTyped(ctx, "state", "updated")

	// Read via token should see updated value
	val, ok := cap.Get(ctx)
	if !ok || val != "updated" {
		t.Errorf("expected 'updated', got %q (ok=%v)", val, ok)
	}
}
