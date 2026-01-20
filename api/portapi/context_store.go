package portapi

import (
	"context"
	"fmt"
)

// contextStoreKey is the unexported key used to store the typed context map.
// Using a single key with a map avoids collisions with other packages.
type contextStoreKey struct{}

// getStore retrieves the internal map from context, or returns nil if not present.
func getStore(ctx context.Context) map[string]any {
	v := ctx.Value(contextStoreKey{})
	if v == nil {
		return nil
	}
	return v.(map[string]any)
}

// WithTyped stores a typed value in the context under the given key.
// This uses copy-on-write semantics: each call returns a new context
// with a new map that doesn't affect the original context.
func WithTyped[T any](ctx context.Context, key string, value T) context.Context {
	existing := getStore(ctx)

	// Create a new map (copy-on-write)
	newMap := make(map[string]any, len(existing)+1)
	for k, v := range existing {
		newMap[k] = v
	}
	newMap[key] = value

	return context.WithValue(ctx, contextStoreKey{}, newMap)
}

// GetTyped retrieves a typed value from the context by key.
// Returns (value, true) if present and the type matches,
// or (zero, false) if not present or type doesn't match.
func GetTyped[T any](ctx context.Context, key string) (T, bool) {
	var zero T

	store := getStore(ctx)
	if store == nil {
		return zero, false
	}

	v, ok := store[key]
	if !ok {
		return zero, false
	}

	typed, ok := v.(T)
	if !ok {
		return zero, false
	}

	return typed, true
}

// MustTyped retrieves a typed value from the context by key.
// Panics if the key is not present or if the type doesn't match.
func MustTyped[T any](ctx context.Context, key string) T {
	v, ok := GetTyped[T](ctx, key)
	if !ok {
		panic(fmt.Sprintf("portapi: context key %q not found or type mismatch", key))
	}
	return v
}
