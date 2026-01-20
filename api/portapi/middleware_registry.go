package portapi

import (
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"
)

// TypeToken represents a type for use in the middleware registry.
type TypeToken struct {
	t reflect.Type
}

// TypeOf returns a TypeToken for the given type parameter.
func TypeOf[T any]() TypeToken {
	var zero T
	return TypeToken{t: reflect.TypeOf(zero)}
}

// MiddlewareRegistry holds middleware declarations and metadata for discovery-time validation.
type MiddlewareRegistry struct {
	middlewares []MiddlewareRef
	provided    map[string]reflect.Type
	metadata    map[middlewareKey]*MiddlewareMetadata
}

// middlewareKey creates a unique key for a middleware function.
type middlewareKey struct {
	pkg  string
	name string
}

// MiddlewareMetadata holds metadata about a middleware function.
type MiddlewareMetadata struct {
	RequiredHeaders   []string
	RequiredCookies   []string
	SecuritySchemes   []string
	MayReturnStatuses []MayReturnStatus
}

// MayReturnStatus represents a status code and description that middleware may return.
type MayReturnStatus struct {
	Status      int
	Description string
}

// MiddlewareDescriptor provides a builder API for attaching metadata to middleware.
type MiddlewareDescriptor struct {
	reg *MiddlewareRegistry
	key middlewareKey
}

var contextKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// Use registers a middleware function with the registry in declaration order.
func (r *MiddlewareRegistry) Use(mw any) {
	ref := newMiddlewareRef(mw)
	r.middlewares = append(r.middlewares, ref)
}

// Middlewares returns all registered middleware in declaration order.
func (r *MiddlewareRegistry) Middlewares() []MiddlewareRef {
	return append([]MiddlewareRef(nil), r.middlewares...)
}

// Provide declares a context key that middleware may provide to handlers.
// Keys must follow the pattern [a-z][a-z0-9_]*, cannot contain consecutive
// underscores, and cannot end with an underscore. Keys must be unique.
func (r *MiddlewareRegistry) Provide(key string, typ TypeToken) *RegistryError {
	if key == "" {
		return &RegistryError{
			Code:    "invalid_context_key",
			Message: "context key cannot be empty",
		}
	}

	if !contextKeyPattern.MatchString(key) {
		return &RegistryError{
			Code:    "invalid_context_key",
			Message: fmt.Sprintf("context key %q must match pattern [a-z][a-z0-9_]*", key),
		}
	}

	// Reject consecutive underscores (e.g., "user__id")
	if strings.Contains(key, "__") {
		return &RegistryError{
			Code:    "invalid_context_key",
			Message: fmt.Sprintf("context key %q cannot contain consecutive underscores", key),
		}
	}

	// Reject trailing underscore (e.g., "user_")
	if strings.HasSuffix(key, "_") {
		return &RegistryError{
			Code:    "invalid_context_key",
			Message: fmt.Sprintf("context key %q cannot end with underscore", key),
		}
	}

	if r.provided == nil {
		r.provided = make(map[string]reflect.Type)
	}

	if existing, ok := r.provided[key]; ok {
		if existing == typ.t {
			return &RegistryError{
				Code:    "duplicate_context_key",
				Message: fmt.Sprintf("context key %q is already declared", key),
			}
		}
		return &RegistryError{
			Code:    "duplicate_context_key_type_mismatch",
			Message: fmt.Sprintf("context key %q is already declared with a different type", key),
		}
	}

	r.provided[key] = typ.t
	return nil
}

// ProvidedKeys returns all provided context keys in sorted order for determinism.
func (r *MiddlewareRegistry) ProvidedKeys() []ProvidedKey {
	if r.provided == nil {
		return nil
	}

	keys := make([]string, 0, len(r.provided))
	for k := range r.provided {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	result := make([]ProvidedKey, len(keys))
	for i, k := range keys {
		result[i] = ProvidedKey{
			Key:  k,
			Type: r.provided[k].String(),
		}
	}
	return result
}

// ProvidedKey represents a context key and its type.
type ProvidedKey struct {
	Key  string
	Type string
}

// Describe returns a descriptor for attaching metadata to a middleware function.
// The middleware must have been previously declared via Use().
func (r *MiddlewareRegistry) Describe(mw any) (*MiddlewareDescriptor, *RegistryError) {
	ref := newMiddlewareRef(mw)
	key := middlewareKey{pkg: ref.Pkg, name: ref.Name}

	// Check if middleware was declared via Use
	found := false
	for _, m := range r.middlewares {
		if m.Pkg == ref.Pkg && m.Name == ref.Name {
			found = true
			break
		}
	}

	if !found {
		return nil, &RegistryError{
			Code:    "describe_undeclared_middleware",
			Message: fmt.Sprintf("middleware %s/%s must be declared via Use() before calling Describe()", ref.Pkg, ref.Name),
		}
	}

	if r.metadata == nil {
		r.metadata = make(map[middlewareKey]*MiddlewareMetadata)
	}

	if _, ok := r.metadata[key]; !ok {
		r.metadata[key] = &MiddlewareMetadata{}
	}

	return &MiddlewareDescriptor{reg: r, key: key}, nil
}

// GetMetadata returns the metadata for a middleware function, or nil if not found.
func (r *MiddlewareRegistry) GetMetadata(mw any) *MiddlewareMetadata {
	ref := newMiddlewareRef(mw)
	key := middlewareKey{pkg: ref.Pkg, name: ref.Name}
	return r.metadata[key]
}

// RequireHeader adds a required header to the middleware metadata.
func (d *MiddlewareDescriptor) RequireHeader(name string) *MiddlewareDescriptor {
	meta := d.reg.metadata[d.key]
	meta.RequiredHeaders = append(meta.RequiredHeaders, name)
	return d
}

// RequireCookie adds a required cookie to the middleware metadata.
func (d *MiddlewareDescriptor) RequireCookie(name string) *MiddlewareDescriptor {
	meta := d.reg.metadata[d.key]
	meta.RequiredCookies = append(meta.RequiredCookies, name)
	return d
}

// Security adds a security scheme to the middleware metadata.
func (d *MiddlewareDescriptor) Security(scheme string) *MiddlewareDescriptor {
	meta := d.reg.metadata[d.key]
	meta.SecuritySchemes = append(meta.SecuritySchemes, scheme)
	return d
}

// MayReturn adds a status code and description that the middleware may return.
func (d *MiddlewareDescriptor) MayReturn(status int, description string) *MiddlewareDescriptor {
	meta := d.reg.metadata[d.key]
	meta.MayReturnStatuses = append(meta.MayReturnStatuses, MayReturnStatus{
		Status:      status,
		Description: description,
	})
	return d
}

// RegistryError represents a middleware registry validation error.
type RegistryError struct {
	Code    string
	Message string
}

func (e *RegistryError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// ValidateStrictMiddlewareDeclaration validates that all middleware used by endpoints
// is properly declared in the registry (strict mode).
func ValidateStrictMiddlewareDeclaration(endpoints []Endpoint, reg *MiddlewareRegistry, middlewarePackageConfigured bool) *RegistryError {
	// Collect all used middleware
	usedMiddleware := make(map[middlewareKey]bool)
	hasMiddleware := false

	for _, ep := range endpoints {
		if len(ep.Middlewares) > 0 {
			hasMiddleware = true
			for _, mw := range ep.Middlewares {
				key := middlewareKey{pkg: mw.Pkg, name: mw.Name}
				usedMiddleware[key] = true
			}
		}
	}

	// If no middleware is used, validation succeeds
	if !hasMiddleware {
		return nil
	}

	// If middleware is used but package is not configured, fail
	if !middlewarePackageConfigured {
		return &RegistryError{
			Code:    "middleware_used_without_registry",
			Message: "middleware is used by endpoints but no middleware package is configured; add [httpgen] middleware_package to your configuration",
		}
	}

	// Build set of declared middleware
	declaredMiddleware := make(map[middlewareKey]bool)
	if reg != nil {
		for _, mw := range reg.middlewares {
			key := middlewareKey{pkg: mw.Pkg, name: mw.Name}
			declaredMiddleware[key] = true
		}
	}

	// Find undeclared middleware
	var undeclared []middlewareKey
	for key := range usedMiddleware {
		if !declaredMiddleware[key] {
			undeclared = append(undeclared, key)
		}
	}

	if len(undeclared) > 0 {
		// Sort for deterministic error messages
		sort.Slice(undeclared, func(i, j int) bool {
			if undeclared[i].pkg != undeclared[j].pkg {
				return undeclared[i].pkg < undeclared[j].pkg
			}
			return undeclared[i].name < undeclared[j].name
		})

		var names []string
		for _, key := range undeclared {
			if key.pkg != "" {
				names = append(names, key.pkg+"."+key.name)
			} else {
				names = append(names, key.name)
			}
		}

		return &RegistryError{
			Code:    "undeclared_middleware",
			Message: fmt.Sprintf("the following middleware is used but not declared in RegisterMiddleware(): %v", names),
		}
	}

	return nil
}
