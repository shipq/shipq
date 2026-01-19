//go:build property

package portapi

import (
	"context"
	"errors"
	"math/rand"
	"reflect"
	"testing"
	"time"
)

// TestProperty_ValidationErrors_Stable tests that validation errors are
// deterministic - calling ValidateHandler twice with the same invalid input
// produces identical error codes and messages.
func TestProperty_ValidationErrors_Stable(t *testing.T) {
	invalidHandlers := []struct {
		name    string
		handler any
	}{
		{"nil", nil},
		{"string", "not a function"},
		{"int", 42},
		{"struct", struct{ X int }{X: 1}},
		{"no_args", func() error { return nil }},
		{"no_ctx", func(s string) error { return nil }},
		{"no_return", func(ctx context.Context) {}},
		{"bad_return", func(ctx context.Context) string { return "" }},
		{"too_many_args", func(ctx context.Context, a, b string) error { return nil }},
		{"too_many_returns", func(ctx context.Context) (string, int, error) { return "", 0, nil }},
		{"variadic", func(ctx context.Context, args ...string) error { return nil }},
	}

	for _, tc := range invalidHandlers {
		t.Run(tc.name, func(t *testing.T) {
			// Call ValidateHandler multiple times
			for i := 0; i < 10; i++ {
				_, err1 := ValidateHandler(tc.handler)
				_, err2 := ValidateHandler(tc.handler)

				if err1 == nil || err2 == nil {
					t.Fatal("expected errors for invalid handler")
				}

				var vErr1, vErr2 *ValidationError
				if !errors.As(err1, &vErr1) || !errors.As(err2, &vErr2) {
					t.Fatal("expected *ValidationError")
				}

				if vErr1.Code != vErr2.Code {
					t.Errorf("iteration %d: codes differ: %q vs %q", i, vErr1.Code, vErr2.Code)
				}
				if vErr1.Message != vErr2.Message {
					t.Errorf("iteration %d: messages differ: %q vs %q", i, vErr1.Message, vErr2.Message)
				}
			}
		})
	}
}

// TestProperty_SupportedQueryTypes tests that all expected scalar types
// pass validation for query parameter bindings.
func TestProperty_SupportedQueryTypes(t *testing.T) {
	supportedScalarTypes := []reflect.Type{
		reflect.TypeOf(""),
		reflect.TypeOf(int(0)),
		reflect.TypeOf(int8(0)),
		reflect.TypeOf(int16(0)),
		reflect.TypeOf(int32(0)),
		reflect.TypeOf(int64(0)),
		reflect.TypeOf(uint(0)),
		reflect.TypeOf(uint8(0)),
		reflect.TypeOf(uint16(0)),
		reflect.TypeOf(uint32(0)),
		reflect.TypeOf(uint64(0)),
		reflect.TypeOf(float32(0)),
		reflect.TypeOf(float64(0)),
		reflect.TypeOf(false),
		reflect.TypeOf(time.Time{}),
	}

	for _, typ := range supportedScalarTypes {
		t.Run(typ.String(), func(t *testing.T) {
			if !isSupportedQueryType(typ) {
				t.Errorf("expected %s to be supported for query params", typ)
			}
		})
	}
}

// TestProperty_SupportedQueryTypes_Pointers tests that pointers to
// supported scalar types also pass validation (for optional params).
func TestProperty_SupportedQueryTypes_Pointers(t *testing.T) {
	supportedTypes := []any{
		(*string)(nil),
		(*int)(nil),
		(*int64)(nil),
		(*float64)(nil),
		(*bool)(nil),
		(*time.Time)(nil),
	}

	for _, v := range supportedTypes {
		typ := reflect.TypeOf(v)
		t.Run(typ.String(), func(t *testing.T) {
			if !isSupportedQueryType(typ) {
				t.Errorf("expected %s to be supported for query params", typ)
			}
		})
	}
}

// TestProperty_SupportedQueryTypes_Slices tests that slices of
// supported scalar types pass validation (for multi-value params).
func TestProperty_SupportedQueryTypes_Slices(t *testing.T) {
	supportedTypes := []any{
		[]string{},
		[]int{},
		[]int64{},
		[]float64{},
		[]bool{},
	}

	for _, v := range supportedTypes {
		typ := reflect.TypeOf(v)
		t.Run(typ.String(), func(t *testing.T) {
			if !isSupportedQueryType(typ) {
				t.Errorf("expected %s to be supported for query params", typ)
			}
		})
	}
}

// TestProperty_SupportedPathTypes tests that path variable bindings
// accept expected integer and string types.
func TestProperty_SupportedPathTypes(t *testing.T) {
	supportedTypes := []reflect.Type{
		reflect.TypeOf(""),
		reflect.TypeOf(int(0)),
		reflect.TypeOf(int8(0)),
		reflect.TypeOf(int16(0)),
		reflect.TypeOf(int32(0)),
		reflect.TypeOf(int64(0)),
		reflect.TypeOf(uint(0)),
		reflect.TypeOf(uint8(0)),
		reflect.TypeOf(uint16(0)),
		reflect.TypeOf(uint32(0)),
		reflect.TypeOf(uint64(0)),
	}

	for _, typ := range supportedTypes {
		t.Run(typ.String(), func(t *testing.T) {
			if !isSupportedPathType(typ) {
				t.Errorf("expected %s to be supported for path vars", typ)
			}
		})
	}
}

// TestProperty_UnsupportedQueryTypes tests that complex types
// are rejected for query parameter bindings.
func TestProperty_UnsupportedQueryTypes(t *testing.T) {
	unsupportedTypes := []any{
		map[string]string{},
		map[string]int{},
		struct{ X int }{},
		[]struct{ X int }{},
		func() {},
		make(chan int),
	}

	for _, v := range unsupportedTypes {
		typ := reflect.TypeOf(v)
		t.Run(typ.String(), func(t *testing.T) {
			if isSupportedQueryType(typ) {
				t.Errorf("expected %s to NOT be supported for query params", typ)
			}
		})
	}
}

// TestProperty_ValidHandlerShapes_Deterministic tests that valid handlers
// consistently produce the same HandlerInfo across multiple calls.
func TestProperty_ValidHandlerShapes_Deterministic(t *testing.T) {
	type Req struct{ Name string }
	type Resp struct{ ID string }

	validHandlers := []struct {
		name    string
		handler any
		shape   HandlerShape
	}{
		{"ctx_req_resp_err", func(ctx context.Context, req Req) (Resp, error) { return Resp{}, nil }, ShapeCtxReqRespErr},
		{"ctx_req_err", func(ctx context.Context, req Req) error { return nil }, ShapeCtxReqErr},
		{"ctx_resp_err", func(ctx context.Context) (Resp, error) { return Resp{}, nil }, ShapeCtxRespErr},
		{"ctx_err", func(ctx context.Context) error { return nil }, ShapeCtxErr},
		{"ptr_req", func(ctx context.Context, req *Req) (Resp, error) { return Resp{}, nil }, ShapeCtxReqRespErr},
		{"slice_resp", func(ctx context.Context) ([]Resp, error) { return nil, nil }, ShapeCtxRespErr},
	}

	for _, tc := range validHandlers {
		t.Run(tc.name, func(t *testing.T) {
			for i := 0; i < 10; i++ {
				info1, err1 := ValidateHandler(tc.handler)
				info2, err2 := ValidateHandler(tc.handler)

				if err1 != nil || err2 != nil {
					t.Fatalf("unexpected error: %v / %v", err1, err2)
				}

				if info1.Shape != info2.Shape {
					t.Errorf("iteration %d: shapes differ: %v vs %v", i, info1.Shape, info2.Shape)
				}

				if info1.Shape != tc.shape {
					t.Errorf("iteration %d: expected shape %v, got %v", i, tc.shape, info1.Shape)
				}
			}
		})
	}
}

// TestProperty_BindingValidation_Deterministic tests that binding validation
// produces consistent results across multiple calls.
func TestProperty_BindingValidation_Deterministic(t *testing.T) {
	type Req struct {
		ID    string `path:"id"`
		Limit int    `query:"limit"`
		Name  string `json:"name"`
	}

	path := "/items/{id}"
	reqType := reflect.TypeOf(Req{})

	for i := 0; i < 100; i++ {
		info1, err1 := AnalyzeBindings(path, reqType)
		info2, err2 := AnalyzeBindings(path, reqType)

		if (err1 == nil) != (err2 == nil) {
			t.Fatalf("iteration %d: error mismatch: %v vs %v", i, err1, err2)
		}

		if err1 == nil {
			if info1.HasJSONBody != info2.HasJSONBody {
				t.Errorf("iteration %d: HasJSONBody mismatch", i)
			}
			if len(info1.PathBindings) != len(info2.PathBindings) {
				t.Errorf("iteration %d: PathBindings length mismatch", i)
			}
			if len(info1.QueryBindings) != len(info2.QueryBindings) {
				t.Errorf("iteration %d: QueryBindings length mismatch", i)
			}
			if len(info1.JSONBindings) != len(info2.JSONBindings) {
				t.Errorf("iteration %d: JSONBindings length mismatch", i)
			}
		}
	}
}

// TestProperty_RandomPathPatterns tests path variable extraction with
// randomly generated path patterns.
func TestProperty_RandomPathPatterns(t *testing.T) {
	segments := []string{"users", "posts", "comments", "items", "api", "v1", "v2"}
	pathVars := []string{"id", "user_id", "post_id", "slug", "name"}

	for seed := int64(0); seed < 50; seed++ {
		r := rand.New(rand.NewSource(seed))

		// Build a random path
		numSegments := r.Intn(4) + 1
		var path string
		var expectedVars []string

		for i := 0; i < numSegments; i++ {
			if r.Float32() < 0.5 {
				// Static segment
				path += "/" + segments[r.Intn(len(segments))]
			} else {
				// Path variable
				varName := pathVars[r.Intn(len(pathVars))]
				// Avoid duplicates
				found := false
				for _, v := range expectedVars {
					if v == varName {
						found = true
						break
					}
				}
				if !found {
					path += "/{" + varName + "}"
					expectedVars = append(expectedVars, varName)
				} else {
					path += "/" + segments[r.Intn(len(segments))]
				}
			}
		}

		if path == "" {
			path = "/"
		}

		t.Run(path, func(t *testing.T) {
			vars := extractPathVariables(path)

			// Check all expected vars are present
			for _, v := range expectedVars {
				if !vars[v] {
					t.Errorf("expected path var %q in path %q", v, path)
				}
			}

			// Check count matches
			if len(vars) != len(expectedVars) {
				t.Errorf("path %q: expected %d vars, got %d", path, len(expectedVars), len(vars))
			}
		})
	}
}
