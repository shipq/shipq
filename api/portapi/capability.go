package portapi

import "context"

// ContextKey represents a typed context key definition.
// It is used to declare context keys in middleware packages and obtain
// capability tokens via Provide().
type ContextKey[T any] struct {
	name string
}

// NewContextKey creates a new typed context key with the given name.
// The name should follow the pattern [a-z][a-z0-9_]* with no consecutive
// underscores and no trailing underscore.
func NewContextKey[T any](name string) ContextKey[T] {
	return ContextKey[T]{name: name}
}

// Name returns the string name of the context key.
func (k ContextKey[T]) Name() string {
	return k.name
}

// Provide registers the context key with the middleware registry and returns
// a capability token that can be used to read/write values of type T.
//
// This method delegates all validation to the registry's Provide method,
// ensuring consistent validation rules.
//
// The returned Cap[T] can be used by middleware implementations to
// store and retrieve typed values without depending on generated helpers.
func (k ContextKey[T]) Provide(reg *MiddlewareRegistry) (Cap[T], *RegistryError) {
	err := reg.Provide(k.name, TypeOf[T]())
	if err != nil {
		return Cap[T]{}, err
	}
	return Cap[T]{key: k.name}, nil
}

// Cap is a capability token that proves (at compile time) that a particular
// context key exists and has type T. It provides typed access to context values
// using the same backing store as WithTyped/GetTyped.
//
// Middleware implementations should use Cap methods instead of generated helpers,
// allowing the middleware package to compile without any generated code.
type Cap[T any] struct {
	key string
}

// Key returns the string key name for debugging purposes.
func (c Cap[T]) Key() string {
	return c.key
}

// With stores a value of type T in the context under this capability's key.
// Returns a new context containing the value.
func (c Cap[T]) With(ctx context.Context, value T) context.Context {
	return WithTyped(ctx, c.key, value)
}

// Get retrieves the value of type T from the context.
// Returns (value, true) if present and the type matches,
// or (zero, false) if not present or type doesn't match.
func (c Cap[T]) Get(ctx context.Context) (T, bool) {
	return GetTyped[T](ctx, c.key)
}

// Must retrieves the value of type T from the context.
// Panics if the value is not present or if the type doesn't match.
func (c Cap[T]) Must(ctx context.Context) T {
	return MustTyped[T](ctx, c.key)
}
