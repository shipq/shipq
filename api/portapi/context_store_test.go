package portapi

import (
	"context"
	"testing"
)

// TestContextStore_SetGetRoundTrip verifies basic set/get functionality.
func TestContextStore_SetGetRoundTrip(t *testing.T) {
	ctx := context.Background()

	// Set a string value
	ctx2 := WithTyped(ctx, "request_id", "abc123")

	// Get should return the value
	val, ok := GetTyped[string](ctx2, "request_id")
	if !ok {
		t.Fatal("expected GetTyped to return ok=true")
	}
	if val != "abc123" {
		t.Errorf("expected value %q, got %q", "abc123", val)
	}

	// Original context should not have the value
	_, ok = GetTyped[string](ctx, "request_id")
	if ok {
		t.Error("expected original context to not have value")
	}
}

// TestContextStore_MultipleKeys verifies multiple keys can be stored independently.
func TestContextStore_MultipleKeys(t *testing.T) {
	ctx := context.Background()

	ctx = WithTyped(ctx, "request_id", "req-123")
	ctx = WithTyped(ctx, "tenant_id", "tenant-456")
	ctx = WithTyped(ctx, "user_id", 42)

	// All values should be retrievable
	reqID, ok := GetTyped[string](ctx, "request_id")
	if !ok || reqID != "req-123" {
		t.Errorf("request_id: expected %q, got %q (ok=%v)", "req-123", reqID, ok)
	}

	tenantID, ok := GetTyped[string](ctx, "tenant_id")
	if !ok || tenantID != "tenant-456" {
		t.Errorf("tenant_id: expected %q, got %q (ok=%v)", "tenant-456", tenantID, ok)
	}

	userID, ok := GetTyped[int](ctx, "user_id")
	if !ok || userID != 42 {
		t.Errorf("user_id: expected %d, got %d (ok=%v)", 42, userID, ok)
	}
}

// TestContextStore_TypeMismatchReturnsFalse verifies that getting with wrong type returns false.
func TestContextStore_TypeMismatchReturnsFalse(t *testing.T) {
	ctx := context.Background()

	// Store a string
	ctx = WithTyped(ctx, "request_id", "abc123")

	// Try to get as int - should return (0, false)
	val, ok := GetTyped[int](ctx, "request_id")
	if ok {
		t.Error("expected GetTyped[int] to return ok=false for string value")
	}
	if val != 0 {
		t.Errorf("expected zero value, got %d", val)
	}
}

// TestContextStore_MissingKeyReturnsFalse verifies that getting a missing key returns false.
func TestContextStore_MissingKeyReturnsFalse(t *testing.T) {
	ctx := context.Background()

	val, ok := GetTyped[string](ctx, "nonexistent")
	if ok {
		t.Error("expected GetTyped to return ok=false for missing key")
	}
	if val != "" {
		t.Errorf("expected zero value, got %q", val)
	}
}

// TestContextStore_CopyOnWriteIsolation verifies that modifications don't affect parent contexts.
func TestContextStore_CopyOnWriteIsolation(t *testing.T) {
	ctx0 := context.Background()

	// Create a chain of contexts
	ctx1 := WithTyped(ctx0, "k", "a")
	ctx2 := WithTyped(ctx1, "k", "b")
	ctx3 := WithTyped(ctx1, "k", "c") // Branch from ctx1, not ctx2

	// ctx0 should have nothing
	_, ok := GetTyped[string](ctx0, "k")
	if ok {
		t.Error("ctx0 should not have key 'k'")
	}

	// ctx1 should have "a"
	v1, ok := GetTyped[string](ctx1, "k")
	if !ok || v1 != "a" {
		t.Errorf("ctx1: expected 'a', got %q (ok=%v)", v1, ok)
	}

	// ctx2 should have "b"
	v2, ok := GetTyped[string](ctx2, "k")
	if !ok || v2 != "b" {
		t.Errorf("ctx2: expected 'b', got %q (ok=%v)", v2, ok)
	}

	// ctx3 should have "c"
	v3, ok := GetTyped[string](ctx3, "k")
	if !ok || v3 != "c" {
		t.Errorf("ctx3: expected 'c', got %q (ok=%v)", v3, ok)
	}

	// Verify ctx1 is still "a" after creating ctx2 and ctx3
	v1Again, ok := GetTyped[string](ctx1, "k")
	if !ok || v1Again != "a" {
		t.Errorf("ctx1 (recheck): expected 'a', got %q (ok=%v)", v1Again, ok)
	}
}

// TestMustTyped_ReturnsValue verifies MustTyped returns value when present.
func TestMustTyped_ReturnsValue(t *testing.T) {
	ctx := WithTyped(context.Background(), "request_id", "abc")

	val := MustTyped[string](ctx, "request_id")
	if val != "abc" {
		t.Errorf("expected %q, got %q", "abc", val)
	}
}

// TestMustTyped_PanicsWhenMissing verifies MustTyped panics when key is missing.
func TestMustTyped_PanicsWhenMissing(t *testing.T) {
	ctx := context.Background()

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected MustTyped to panic for missing key")
		}
		// Verify panic message contains useful info
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("expected panic value to be string, got %T", r)
		}
		if msg == "" {
			t.Error("expected non-empty panic message")
		}
	}()

	MustTyped[string](ctx, "nonexistent")
}

// TestMustTyped_PanicsOnTypeMismatch verifies MustTyped panics when type doesn't match.
func TestMustTyped_PanicsOnTypeMismatch(t *testing.T) {
	ctx := WithTyped(context.Background(), "request_id", "abc")

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected MustTyped to panic for type mismatch")
		}
	}()

	MustTyped[int](ctx, "request_id")
}

// TestContextStore_PointerTypes verifies that pointer types work correctly.
func TestContextStore_PointerTypes(t *testing.T) {
	type User struct {
		ID   int
		Name string
	}

	ctx := context.Background()
	user := &User{ID: 1, Name: "Alice"}

	ctx = WithTyped(ctx, "current_user", user)

	retrieved, ok := GetTyped[*User](ctx, "current_user")
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

// TestContextStore_NilPointer verifies that nil pointers can be stored and retrieved.
func TestContextStore_NilPointer(t *testing.T) {
	type User struct{ ID int }

	ctx := context.Background()
	var user *User = nil

	ctx = WithTyped(ctx, "current_user", user)

	retrieved, ok := GetTyped[*User](ctx, "current_user")
	if !ok {
		t.Fatal("expected GetTyped to return ok=true for nil pointer")
	}
	if retrieved != nil {
		t.Errorf("expected nil, got %v", retrieved)
	}
}

// TestContextStore_SliceTypes verifies that slice types work correctly.
func TestContextStore_SliceTypes(t *testing.T) {
	ctx := context.Background()
	roles := []string{"admin", "user"}

	ctx = WithTyped(ctx, "roles", roles)

	retrieved, ok := GetTyped[[]string](ctx, "roles")
	if !ok {
		t.Fatal("expected to retrieve roles")
	}
	if len(retrieved) != 2 || retrieved[0] != "admin" || retrieved[1] != "user" {
		t.Errorf("unexpected roles: %v", retrieved)
	}
}

// TestContextStore_Overwrite verifies that setting the same key twice overwrites the value.
func TestContextStore_Overwrite(t *testing.T) {
	ctx := context.Background()

	ctx = WithTyped(ctx, "counter", 1)
	ctx = WithTyped(ctx, "counter", 2)
	ctx = WithTyped(ctx, "counter", 3)

	val, ok := GetTyped[int](ctx, "counter")
	if !ok || val != 3 {
		t.Errorf("expected 3, got %d (ok=%v)", val, ok)
	}
}

// TestContextStore_EmptyStringKey verifies behavior with empty string key.
// (The registry rejects empty keys, but the store itself should handle them gracefully.)
func TestContextStore_EmptyStringKey(t *testing.T) {
	ctx := context.Background()

	ctx = WithTyped(ctx, "", "value")

	val, ok := GetTyped[string](ctx, "")
	if !ok || val != "value" {
		t.Errorf("expected 'value', got %q (ok=%v)", val, ok)
	}
}

// TestContextStore_PreservesOtherContextValues verifies that the store doesn't interfere
// with other context values set via standard context.WithValue.
func TestContextStore_PreservesOtherContextValues(t *testing.T) {
	type customKey struct{}

	ctx := context.Background()

	// Set a standard context value
	ctx = context.WithValue(ctx, customKey{}, "standard-value")

	// Set a typed store value
	ctx = WithTyped(ctx, "request_id", "typed-value")

	// Both should be retrievable
	standardVal := ctx.Value(customKey{})
	if standardVal != "standard-value" {
		t.Errorf("standard context value: expected 'standard-value', got %v", standardVal)
	}

	typedVal, ok := GetTyped[string](ctx, "request_id")
	if !ok || typedVal != "typed-value" {
		t.Errorf("typed store value: expected 'typed-value', got %q (ok=%v)", typedVal, ok)
	}
}
