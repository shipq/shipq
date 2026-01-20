package portapi

import (
	"context"
	"testing"
)

// TestNewContextKey_CreatesKey verifies that NewContextKey creates a key with the correct name.
func TestNewContextKey_CreatesKey(t *testing.T) {
	key := NewContextKey[string]("request_id")

	if key.Name() != "request_id" {
		t.Errorf("expected name %q, got %q", "request_id", key.Name())
	}
}

// TestContextKeyProvide_DelegatesValidation verifies that Provide delegates validation to the registry.
func TestContextKeyProvide_DelegatesValidation(t *testing.T) {
	reg := &MiddlewareRegistry{}

	// Invalid key should fail
	key := NewContextKey[string]("Bad-Key")
	_, err := key.Provide(reg)

	if err == nil {
		t.Fatal("expected error for invalid key, got nil")
	}
	if err.Code != "invalid_context_key" {
		t.Errorf("expected error code 'invalid_context_key', got %q", err.Code)
	}
}

// TestContextKeyProvide_DelegatesValidation_ConsecutiveUnderscores verifies stricter validation.
func TestContextKeyProvide_DelegatesValidation_ConsecutiveUnderscores(t *testing.T) {
	reg := &MiddlewareRegistry{}

	key := NewContextKey[string]("user__id")
	_, err := key.Provide(reg)

	if err == nil {
		t.Fatal("expected error for consecutive underscores, got nil")
	}
	if err.Code != "invalid_context_key" {
		t.Errorf("expected error code 'invalid_context_key', got %q", err.Code)
	}
}

// TestContextKeyProvide_DelegatesValidation_TrailingUnderscore verifies stricter validation.
func TestContextKeyProvide_DelegatesValidation_TrailingUnderscore(t *testing.T) {
	reg := &MiddlewareRegistry{}

	key := NewContextKey[string]("user_")
	_, err := key.Provide(reg)

	if err == nil {
		t.Fatal("expected error for trailing underscore, got nil")
	}
	if err.Code != "invalid_context_key" {
		t.Errorf("expected error code 'invalid_context_key', got %q", err.Code)
	}
}

// TestContextKeyProvide_Success verifies that Provide returns a capability token on success.
func TestContextKeyProvide_Success(t *testing.T) {
	reg := &MiddlewareRegistry{}

	key := NewContextKey[string]("request_id")
	cap, err := key.Provide(reg)

	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	// Verify the capability token has the right key
	if cap.Key() != "request_id" {
		t.Errorf("expected cap key %q, got %q", "request_id", cap.Key())
	}
}

// TestContextKeyProvide_RegistersWithRegistry verifies that Provide registers the key with the registry.
func TestContextKeyProvide_RegistersWithRegistry(t *testing.T) {
	reg := &MiddlewareRegistry{}

	key := NewContextKey[string]("request_id")
	_, err := key.Provide(reg)

	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	// Verify the registry knows about the key
	keys := reg.ProvidedKeys()
	if len(keys) != 1 {
		t.Fatalf("expected 1 provided key, got %d", len(keys))
	}
	if keys[0].Key != "request_id" {
		t.Errorf("expected key %q, got %q", "request_id", keys[0].Key)
	}
	if keys[0].Type != "string" {
		t.Errorf("expected type %q, got %q", "string", keys[0].Type)
	}
}

// TestContextKeyProvide_DuplicateTypeMismatch verifies duplicate key with different type fails.
func TestContextKeyProvide_DuplicateTypeMismatch(t *testing.T) {
	reg := &MiddlewareRegistry{}

	k1 := NewContextKey[string]("k")
	_, err := k1.Provide(reg)
	if err != nil {
		t.Fatalf("first Provide should succeed, got: %v", err)
	}

	k2 := NewContextKey[int]("k")
	_, err = k2.Provide(reg)

	if err == nil {
		t.Fatal("expected error for duplicate key with different type, got nil")
	}
	if err.Code != "duplicate_context_key_type_mismatch" {
		t.Errorf("expected error code 'duplicate_context_key_type_mismatch', got %q", err.Code)
	}
}

// TestContextKeyProvide_DuplicateSameType verifies duplicate key with same type fails.
func TestContextKeyProvide_DuplicateSameType(t *testing.T) {
	reg := &MiddlewareRegistry{}

	k1 := NewContextKey[string]("k")
	_, err := k1.Provide(reg)
	if err != nil {
		t.Fatalf("first Provide should succeed, got: %v", err)
	}

	k2 := NewContextKey[string]("k")
	_, err = k2.Provide(reg)

	if err == nil {
		t.Fatal("expected error for duplicate key, got nil")
	}
	if err.Code != "duplicate_context_key" {
		t.Errorf("expected error code 'duplicate_context_key', got %q", err.Code)
	}
}

// TestCap_WithGetMust verifies that Cap methods work correctly.
func TestCap_WithGetMust(t *testing.T) {
	reg := &MiddlewareRegistry{}

	key := NewContextKey[string]("request_id")
	cap, err := key.Provide(reg)
	if err != nil {
		t.Fatalf("Provide failed: %v", err)
	}

	ctx := context.Background()

	// With
	ctx = cap.With(ctx, "abc123")

	// Get
	val, ok := cap.Get(ctx)
	if !ok {
		t.Fatal("expected Get to return ok=true")
	}
	if val != "abc123" {
		t.Errorf("expected %q, got %q", "abc123", val)
	}

	// Must
	mustVal := cap.Must(ctx)
	if mustVal != "abc123" {
		t.Errorf("expected %q, got %q", "abc123", mustVal)
	}
}

// TestCap_GetMissingReturnsFalse verifies Get returns false for missing key.
func TestCap_GetMissingReturnsFalse(t *testing.T) {
	reg := &MiddlewareRegistry{}

	key := NewContextKey[string]("request_id")
	cap, _ := key.Provide(reg)

	ctx := context.Background()

	val, ok := cap.Get(ctx)
	if ok {
		t.Error("expected Get to return ok=false for missing key")
	}
	if val != "" {
		t.Errorf("expected zero value, got %q", val)
	}
}

// TestCap_MustPanicsWhenMissing verifies Must panics when key is missing.
func TestCap_MustPanicsWhenMissing(t *testing.T) {
	reg := &MiddlewareRegistry{}

	key := NewContextKey[string]("request_id")
	cap, _ := key.Provide(reg)

	ctx := context.Background()

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected Must to panic for missing key")
		}
	}()

	cap.Must(ctx)
}

// TestCap_InteropsWithPortapiWithTyped verifies that Cap and WithTyped/GetTyped share the same store.
func TestCap_InteropsWithPortapiWithTyped(t *testing.T) {
	reg := &MiddlewareRegistry{}

	key := NewContextKey[string]("request_id")
	cap, _ := key.Provide(reg)

	ctx := context.Background()

	// Set via Cap, read via WithTyped/GetTyped
	ctx = cap.With(ctx, "from-cap")
	val, ok := GetTyped[string](ctx, "request_id")
	if !ok || val != "from-cap" {
		t.Errorf("expected 'from-cap', got %q (ok=%v)", val, ok)
	}

	// Set via WithTyped, read via Cap
	ctx2 := WithTyped(context.Background(), "request_id", "from-typed")
	val2, ok := cap.Get(ctx2)
	if !ok || val2 != "from-typed" {
		t.Errorf("expected 'from-typed', got %q (ok=%v)", val2, ok)
	}
}

// TestCap_PointerType verifies that Cap works with pointer types.
func TestCap_PointerType(t *testing.T) {
	type User struct {
		ID   int
		Name string
	}

	reg := &MiddlewareRegistry{}

	key := NewContextKey[*User]("current_user")
	cap, err := key.Provide(reg)
	if err != nil {
		t.Fatalf("Provide failed: %v", err)
	}

	user := &User{ID: 1, Name: "Alice"}
	ctx := cap.With(context.Background(), user)

	retrieved, ok := cap.Get(ctx)
	if !ok {
		t.Fatal("expected to retrieve user")
	}
	if retrieved != user {
		t.Error("expected same pointer")
	}
	if retrieved.ID != 1 || retrieved.Name != "Alice" {
		t.Errorf("unexpected user data: %+v", retrieved)
	}
}

// TestCap_NilPointer verifies that Cap works with nil pointers.
func TestCap_NilPointer(t *testing.T) {
	type User struct{ ID int }

	reg := &MiddlewareRegistry{}

	key := NewContextKey[*User]("current_user")
	cap, _ := key.Provide(reg)

	var user *User = nil
	ctx := cap.With(context.Background(), user)

	retrieved, ok := cap.Get(ctx)
	if !ok {
		t.Fatal("expected Get to return ok=true for nil pointer")
	}
	if retrieved != nil {
		t.Errorf("expected nil, got %v", retrieved)
	}
}

// TestMultipleCaps_Independent verifies that multiple caps work independently.
func TestMultipleCaps_Independent(t *testing.T) {
	reg := &MiddlewareRegistry{}

	reqIDKey := NewContextKey[string]("request_id")
	tenantKey := NewContextKey[string]("tenant_id")
	userIDKey := NewContextKey[int]("user_id")

	reqIDCap, _ := reqIDKey.Provide(reg)
	tenantCap, _ := tenantKey.Provide(reg)
	userIDCap, _ := userIDKey.Provide(reg)

	ctx := context.Background()
	ctx = reqIDCap.With(ctx, "req-123")
	ctx = tenantCap.With(ctx, "tenant-456")
	ctx = userIDCap.With(ctx, 42)

	// All values should be retrievable
	reqID, ok := reqIDCap.Get(ctx)
	if !ok || reqID != "req-123" {
		t.Errorf("request_id: expected 'req-123', got %q (ok=%v)", reqID, ok)
	}

	tenant, ok := tenantCap.Get(ctx)
	if !ok || tenant != "tenant-456" {
		t.Errorf("tenant_id: expected 'tenant-456', got %q (ok=%v)", tenant, ok)
	}

	userID, ok := userIDCap.Get(ctx)
	if !ok || userID != 42 {
		t.Errorf("user_id: expected 42, got %d (ok=%v)", userID, ok)
	}
}
