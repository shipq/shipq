package portapi

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

// ===============================
// Step 1: Handler signature tests
// ===============================

func TestValidateHandler_ValidShapes(t *testing.T) {
	type Req struct{ Name string }
	type Resp struct{ ID string }

	tests := []struct {
		name    string
		handler any
		shape   HandlerShape
	}{
		{
			name:    "ctx+req -> resp+err",
			handler: func(ctx context.Context, req Req) (Resp, error) { return Resp{}, nil },
			shape:   ShapeCtxReqRespErr,
		},
		{
			name:    "ctx+req -> err",
			handler: func(ctx context.Context, req Req) error { return nil },
			shape:   ShapeCtxReqErr,
		},
		{
			name:    "ctx -> resp+err",
			handler: func(ctx context.Context) (Resp, error) { return Resp{}, nil },
			shape:   ShapeCtxRespErr,
		},
		{
			name:    "ctx -> err",
			handler: func(ctx context.Context) error { return nil },
			shape:   ShapeCtxErr,
		},
		{
			name:    "pointer request",
			handler: func(ctx context.Context, req *Req) (Resp, error) { return Resp{}, nil },
			shape:   ShapeCtxReqRespErr,
		},
		{
			name:    "slice response",
			handler: func(ctx context.Context) ([]Resp, error) { return nil, nil },
			shape:   ShapeCtxRespErr,
		},
		{
			name:    "pointer response",
			handler: func(ctx context.Context) (*Resp, error) { return nil, nil },
			shape:   ShapeCtxRespErr,
		},
		{
			name:    "map response",
			handler: func(ctx context.Context) (map[string]Resp, error) { return nil, nil },
			shape:   ShapeCtxRespErr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := ValidateHandler(tt.handler)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if info.Shape != tt.shape {
				t.Errorf("Shape = %v, want %v", info.Shape, tt.shape)
			}
		})
	}
}

func TestValidateHandler_InvalidShapes(t *testing.T) {
	tests := []struct {
		name        string
		handler     any
		wantErrCode string
	}{
		{
			name:        "not a function",
			handler:     "string",
			wantErrCode: "not_a_function",
		},
		{
			name:        "no args",
			handler:     func() error { return nil },
			wantErrCode: "missing_context",
		},
		{
			name:        "first arg not context",
			handler:     func(s string) error { return nil },
			wantErrCode: "first_arg_not_context",
		},
		{
			name:        "no returns",
			handler:     func(ctx context.Context) {},
			wantErrCode: "missing_error_return",
		},
		{
			name:        "last return not error",
			handler:     func(ctx context.Context) string { return "" },
			wantErrCode: "last_return_not_error",
		},
		{
			name:        "too many args",
			handler:     func(ctx context.Context, a, b string) error { return nil },
			wantErrCode: "too_many_args",
		},
		{
			name:        "too many returns",
			handler:     func(ctx context.Context) (string, int, error) { return "", 0, nil },
			wantErrCode: "too_many_returns",
		},
		{
			name:        "variadic",
			handler:     func(ctx context.Context, args ...string) error { return nil },
			wantErrCode: "variadic_not_supported",
		},
		{
			name:        "nil handler",
			handler:     nil,
			wantErrCode: "nil_handler",
		},
		{
			name:        "int instead of function",
			handler:     42,
			wantErrCode: "not_a_function",
		},
		{
			name:        "struct instead of function",
			handler:     struct{ Name string }{Name: "test"},
			wantErrCode: "not_a_function",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateHandler(tt.handler)
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			var vErr *ValidationError
			if !errors.As(err, &vErr) {
				t.Fatalf("expected *ValidationError, got %T", err)
			}
			if vErr.Code != tt.wantErrCode {
				t.Errorf("Code = %q, want %q", vErr.Code, tt.wantErrCode)
			}
		})
	}
}

func TestValidateHandler_ReturnsCorrectTypes(t *testing.T) {
	type MyReq struct{ Name string }
	type MyResp struct{ ID string }

	t.Run("extracts request type", func(t *testing.T) {
		handler := func(ctx context.Context, req MyReq) error { return nil }
		info, err := ValidateHandler(handler)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if info.ReqType == nil {
			t.Fatal("ReqType should not be nil")
		}
		if info.ReqType.Name() != "MyReq" {
			t.Errorf("ReqType.Name() = %q, want %q", info.ReqType.Name(), "MyReq")
		}
	})

	t.Run("extracts response type", func(t *testing.T) {
		handler := func(ctx context.Context) (MyResp, error) { return MyResp{}, nil }
		info, err := ValidateHandler(handler)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if info.RespType == nil {
			t.Fatal("RespType should not be nil")
		}
		if info.RespType.Name() != "MyResp" {
			t.Errorf("RespType.Name() = %q, want %q", info.RespType.Name(), "MyResp")
		}
	})

	t.Run("extracts both types", func(t *testing.T) {
		handler := func(ctx context.Context, req MyReq) (MyResp, error) { return MyResp{}, nil }
		info, err := ValidateHandler(handler)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if info.ReqType == nil || info.RespType == nil {
			t.Fatal("both types should be set")
		}
	})

	t.Run("no types for ctx -> err", func(t *testing.T) {
		handler := func(ctx context.Context) error { return nil }
		info, err := ValidateHandler(handler)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if info.ReqType != nil {
			t.Error("ReqType should be nil")
		}
		if info.RespType != nil {
			t.Error("RespType should be nil")
		}
	})
}

// ================================
// Step 2: Request type validation
// ================================

func TestValidateRequestType(t *testing.T) {
	t.Run("struct is valid", func(t *testing.T) {
		type Req struct{ Name string }
		err := ValidateRequestType(reflect.TypeOf(Req{}))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("pointer to struct is valid", func(t *testing.T) {
		type Req struct{ Name string }
		err := ValidateRequestType(reflect.TypeOf(&Req{}))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("primitive is invalid", func(t *testing.T) {
		err := ValidateRequestType(reflect.TypeOf("string"))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		var vErr *ValidationError
		if !errors.As(err, &vErr) {
			t.Fatalf("expected *ValidationError, got %T", err)
		}
		if vErr.Code != "request_not_struct" {
			t.Errorf("Code = %q, want %q", vErr.Code, "request_not_struct")
		}
	})

	t.Run("slice is invalid", func(t *testing.T) {
		err := ValidateRequestType(reflect.TypeOf([]string{}))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("map is invalid", func(t *testing.T) {
		err := ValidateRequestType(reflect.TypeOf(map[string]string{}))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("int is invalid", func(t *testing.T) {
		err := ValidateRequestType(reflect.TypeOf(42))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("pointer to primitive is invalid", func(t *testing.T) {
		s := "test"
		err := ValidateRequestType(reflect.TypeOf(&s))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

// ================================
// Step 3: Binding tag validation
// ================================

func TestValidateBindings_PathVariables(t *testing.T) {
	t.Run("path var present in struct", func(t *testing.T) {
		type Req struct {
			ID string `path:"id"`
		}
		err := ValidateBindings("/pets/{id}", reflect.TypeOf(Req{}))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("missing path var in struct", func(t *testing.T) {
		type Req struct {
			Name string `json:"name"`
		}
		err := ValidateBindings("/pets/{id}", reflect.TypeOf(Req{}))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !containsSubstring(err.Error(), "path variable {id}") {
			t.Errorf("error should mention 'path variable {id}', got: %v", err)
		}
	})

	t.Run("extra path tag not in route", func(t *testing.T) {
		type Req struct {
			ID   string `path:"id"`
			Slug string `path:"slug"` // not in route
		}
		err := ValidateBindings("/pets/{id}", reflect.TypeOf(Req{}))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !containsSubstring(err.Error(), "slug") {
			t.Errorf("error should mention 'slug', got: %v", err)
		}
	})

	t.Run("multiple path vars", func(t *testing.T) {
		type Req struct {
			UserID string `path:"user_id"`
			PostID string `path:"post_id"`
		}
		err := ValidateBindings("/users/{user_id}/posts/{post_id}", reflect.TypeOf(Req{}))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("duplicate path tag", func(t *testing.T) {
		type Req struct {
			ID1 string `path:"id"`
			ID2 string `path:"id"` // duplicate
		}
		err := ValidateBindings("/pets/{id}", reflect.TypeOf(Req{}))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !containsSubstring(err.Error(), "duplicate") {
			t.Errorf("error should mention 'duplicate', got: %v", err)
		}
	})

	t.Run("wildcard path var", func(t *testing.T) {
		type Req struct {
			FilePath string `path:"path"`
		}
		err := ValidateBindings("/files/{path...}", reflect.TypeOf(Req{}))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("path var with int type", func(t *testing.T) {
		type Req struct {
			ID int `path:"id"`
		}
		err := ValidateBindings("/pets/{id}", reflect.TypeOf(Req{}))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("path var with int64 type", func(t *testing.T) {
		type Req struct {
			ID int64 `path:"id"`
		}
		err := ValidateBindings("/pets/{id}", reflect.TypeOf(Req{}))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestValidateBindings_QueryParams(t *testing.T) {
	t.Run("scalar query param", func(t *testing.T) {
		type Req struct {
			Limit int `query:"limit"`
		}
		err := ValidateBindings("/pets", reflect.TypeOf(Req{}))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("slice query param", func(t *testing.T) {
		type Req struct {
			Tags []string `query:"tag"`
		}
		err := ValidateBindings("/pets", reflect.TypeOf(Req{}))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("pointer query param (optional)", func(t *testing.T) {
		type Req struct {
			Limit *int `query:"limit"`
		}
		err := ValidateBindings("/pets", reflect.TypeOf(Req{}))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("unsupported query type - map", func(t *testing.T) {
		type Req struct {
			Data map[string]string `query:"data"`
		}
		err := ValidateBindings("/pets", reflect.TypeOf(Req{}))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !containsSubstring(err.Error(), "unsupported") {
			t.Errorf("error should mention 'unsupported', got: %v", err)
		}
	})

	t.Run("unsupported query type - struct", func(t *testing.T) {
		type Inner struct{ X int }
		type Req struct {
			Data Inner `query:"data"`
		}
		err := ValidateBindings("/pets", reflect.TypeOf(Req{}))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("string query param", func(t *testing.T) {
		type Req struct {
			Name string `query:"name"`
		}
		err := ValidateBindings("/pets", reflect.TypeOf(Req{}))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("bool query param", func(t *testing.T) {
		type Req struct {
			Active bool `query:"active"`
		}
		err := ValidateBindings("/pets", reflect.TypeOf(Req{}))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("float64 query param", func(t *testing.T) {
		type Req struct {
			Price float64 `query:"price"`
		}
		err := ValidateBindings("/pets", reflect.TypeOf(Req{}))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("time.Time query param", func(t *testing.T) {
		type Req struct {
			CreatedAt time.Time `query:"created_at"`
		}
		err := ValidateBindings("/pets", reflect.TypeOf(Req{}))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("slice of int query param", func(t *testing.T) {
		type Req struct {
			IDs []int `query:"ids"`
		}
		err := ValidateBindings("/pets", reflect.TypeOf(Req{}))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestValidateBindings_Headers(t *testing.T) {
	t.Run("string header", func(t *testing.T) {
		type Req struct {
			RequestID string `header:"X-Request-Id"`
		}
		err := ValidateBindings("/pets", reflect.TypeOf(Req{}))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("optional header", func(t *testing.T) {
		type Req struct {
			Trace *string `header:"X-Trace-Id"`
		}
		err := ValidateBindings("/pets", reflect.TypeOf(Req{}))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("int header", func(t *testing.T) {
		type Req struct {
			Count int `header:"X-Count"`
		}
		err := ValidateBindings("/pets", reflect.TypeOf(Req{}))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("int64 header", func(t *testing.T) {
		type Req struct {
			Size int64 `header:"X-Size"`
		}
		err := ValidateBindings("/pets", reflect.TypeOf(Req{}))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("bool header", func(t *testing.T) {
		type Req struct {
			Enabled bool `header:"X-Enabled"`
		}
		err := ValidateBindings("/pets", reflect.TypeOf(Req{}))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("float64 header", func(t *testing.T) {
		type Req struct {
			Rate float64 `header:"X-Rate"`
		}
		err := ValidateBindings("/pets", reflect.TypeOf(Req{}))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("time.Time header", func(t *testing.T) {
		type Req struct {
			Timestamp time.Time `header:"X-Timestamp"`
		}
		err := ValidateBindings("/pets", reflect.TypeOf(Req{}))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("optional int header", func(t *testing.T) {
		type Req struct {
			Count *int `header:"X-Count"`
		}
		err := ValidateBindings("/pets", reflect.TypeOf(Req{}))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("unsupported header type - map", func(t *testing.T) {
		type Req struct {
			Data map[string]string `header:"X-Data"`
		}
		err := ValidateBindings("/pets", reflect.TypeOf(Req{}))
		if err == nil {
			t.Fatal("expected error for unsupported map header type")
		}
	})

	t.Run("unsupported header type - struct", func(t *testing.T) {
		type Inner struct{ V int }
		type Req struct {
			Data Inner `header:"X-Data"`
		}
		err := ValidateBindings("/pets", reflect.TypeOf(Req{}))
		if err == nil {
			t.Fatal("expected error for unsupported struct header type")
		}
	})
}

func TestValidateBindings_JSONBody(t *testing.T) {
	t.Run("has json fields means body expected", func(t *testing.T) {
		type Req struct {
			Name string `json:"name"`
			Age  int    `json:"age"`
		}
		info, err := AnalyzeBindings("/pets", reflect.TypeOf(Req{}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !info.HasJSONBody {
			t.Error("HasJSONBody should be true")
		}
	})

	t.Run("no json fields means no body", func(t *testing.T) {
		type Req struct {
			ID string `path:"id"`
		}
		info, err := AnalyzeBindings("/pets/{id}", reflect.TypeOf(Req{}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if info.HasJSONBody {
			t.Error("HasJSONBody should be false")
		}
	})

	t.Run("mixed tags", func(t *testing.T) {
		type Req struct {
			ID    string `path:"id"`
			Limit int    `query:"limit"`
			Name  string `json:"name"`
		}
		info, err := AnalyzeBindings("/pets/{id}", reflect.TypeOf(Req{}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !info.HasJSONBody {
			t.Error("HasJSONBody should be true")
		}
		if len(info.PathBindings) != 1 {
			t.Errorf("PathBindings = %d, want 1", len(info.PathBindings))
		}
		if len(info.QueryBindings) != 1 {
			t.Errorf("QueryBindings = %d, want 1", len(info.QueryBindings))
		}
		if len(info.JSONBindings) != 1 {
			t.Errorf("JSONBindings = %d, want 1", len(info.JSONBindings))
		}
	})

	t.Run("json:- is ignored", func(t *testing.T) {
		type Req struct {
			Internal string `json:"-"`
			Name     string `json:"name"`
		}
		info, err := AnalyzeBindings("/pets", reflect.TypeOf(Req{}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(info.JSONBindings) != 1 {
			t.Errorf("JSONBindings = %d, want 1", len(info.JSONBindings))
		}
	})
}

func TestAnalyzeBindings_FieldBindingDetails(t *testing.T) {
	type Req struct {
		ID    string `path:"id"`
		Name  string `json:"name"`
		Limit int    `query:"limit"`
		Auth  string `header:"Authorization"`
	}

	info, err := AnalyzeBindings("/pets/{id}", reflect.TypeOf(Req{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Run("path binding details", func(t *testing.T) {
		if len(info.PathBindings) != 1 {
			t.Fatalf("PathBindings = %d, want 1", len(info.PathBindings))
		}
		b := info.PathBindings[0]
		if b.FieldName != "ID" {
			t.Errorf("FieldName = %q, want %q", b.FieldName, "ID")
		}
		if b.TagValue != "id" {
			t.Errorf("TagValue = %q, want %q", b.TagValue, "id")
		}
	})

	t.Run("query binding details", func(t *testing.T) {
		if len(info.QueryBindings) != 1 {
			t.Fatalf("QueryBindings = %d, want 1", len(info.QueryBindings))
		}
		b := info.QueryBindings[0]
		if b.FieldName != "Limit" {
			t.Errorf("FieldName = %q, want %q", b.FieldName, "Limit")
		}
		if b.TagValue != "limit" {
			t.Errorf("TagValue = %q, want %q", b.TagValue, "limit")
		}
	})

	t.Run("header binding details", func(t *testing.T) {
		if len(info.HeaderBindings) != 1 {
			t.Fatalf("HeaderBindings = %d, want 1", len(info.HeaderBindings))
		}
		b := info.HeaderBindings[0]
		if b.FieldName != "Auth" {
			t.Errorf("FieldName = %q, want %q", b.FieldName, "Auth")
		}
		if b.TagValue != "Authorization" {
			t.Errorf("TagValue = %q, want %q", b.TagValue, "Authorization")
		}
	})

	t.Run("json binding details", func(t *testing.T) {
		if len(info.JSONBindings) != 1 {
			t.Fatalf("JSONBindings = %d, want 1", len(info.JSONBindings))
		}
		b := info.JSONBindings[0]
		if b.FieldName != "Name" {
			t.Errorf("FieldName = %q, want %q", b.FieldName, "Name")
		}
		if b.TagValue != "name" {
			t.Errorf("TagValue = %q, want %q", b.TagValue, "name")
		}
	})
}

func TestAnalyzeBindings_PointerToStruct(t *testing.T) {
	type Req struct {
		ID string `path:"id"`
	}
	info, err := AnalyzeBindings("/pets/{id}", reflect.TypeOf(&Req{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(info.PathBindings) != 1 {
		t.Errorf("PathBindings = %d, want 1", len(info.PathBindings))
	}
}

// ================================
// Step 4: Endpoint validation
// ================================

func TestValidateEndpoint(t *testing.T) {
	t.Run("valid endpoint", func(t *testing.T) {
		type Req struct {
			ID string `path:"id"`
		}
		type Resp struct {
			Name string `json:"name"`
		}
		handler := func(ctx context.Context, req Req) (Resp, error) {
			return Resp{}, nil
		}

		ep := Endpoint{Method: "GET", Path: "/pets/{id}", Handler: handler}
		err := ValidateEndpoint(ep)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("valid endpoint without request", func(t *testing.T) {
		type Resp struct {
			Name string `json:"name"`
		}
		handler := func(ctx context.Context) (Resp, error) {
			return Resp{}, nil
		}

		ep := Endpoint{Method: "GET", Path: "/pets", Handler: handler}
		err := ValidateEndpoint(ep)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("valid endpoint without response", func(t *testing.T) {
		type Req struct {
			ID string `path:"id"`
		}
		handler := func(ctx context.Context, req Req) error {
			return nil
		}

		ep := Endpoint{Method: "DELETE", Path: "/pets/{id}", Handler: handler}
		err := ValidateEndpoint(ep)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("invalid handler shape", func(t *testing.T) {
		handler := func() {} // invalid
		ep := Endpoint{Method: "GET", Path: "/pets", Handler: handler}
		err := ValidateEndpoint(ep)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("path var mismatch", func(t *testing.T) {
		type Req struct {
			Slug string `path:"slug"` // route has {id}
		}
		handler := func(ctx context.Context, req Req) error { return nil }
		ep := Endpoint{Method: "GET", Path: "/pets/{id}", Handler: handler}
		err := ValidateEndpoint(ep)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("error includes endpoint context", func(t *testing.T) {
		handler := func() {} // invalid
		ep := Endpoint{Method: "POST", Path: "/users", Handler: handler}
		err := ValidateEndpoint(ep)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		errStr := err.Error()
		if !containsSubstring(errStr, "POST") {
			t.Errorf("error should contain 'POST', got: %v", errStr)
		}
		if !containsSubstring(errStr, "/users") {
			t.Errorf("error should contain '/users', got: %v", errStr)
		}
	})

	t.Run("nil handler", func(t *testing.T) {
		ep := Endpoint{Method: "GET", Path: "/pets", Handler: nil}
		err := ValidateEndpoint(ep)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("primitive request type", func(t *testing.T) {
		handler := func(ctx context.Context, id string) error { return nil }
		ep := Endpoint{Method: "GET", Path: "/pets/{id}", Handler: handler}
		err := ValidateEndpoint(ep)
		if err == nil {
			t.Fatal("expected error for primitive request type, got nil")
		}
	})
}

// ================================
// Step 5: Property-like tests
// ================================

func TestValidationErrors_HaveStableCodes(t *testing.T) {
	// Validate twice for each invalid case and ensure codes are identical
	invalidCases := []struct {
		name    string
		handler any
	}{
		{"nil", nil},
		{"not_func", "string"},
		{"no_ctx", func() error { return nil }},
		{"bad_first_arg", func(s string) error { return nil }},
		{"no_return", func(ctx context.Context) {}},
		{"bad_return", func(ctx context.Context) string { return "" }},
	}

	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err1 := ValidateHandler(tc.handler)
			_, err2 := ValidateHandler(tc.handler)

			if err1 == nil || err2 == nil {
				t.Fatal("expected errors")
			}

			var vErr1, vErr2 *ValidationError
			errors.As(err1, &vErr1)
			errors.As(err2, &vErr2)

			if vErr1.Code != vErr2.Code {
				t.Errorf("codes differ: %q vs %q", vErr1.Code, vErr2.Code)
			}
			if vErr1.Message != vErr2.Message {
				t.Errorf("messages differ: %q vs %q", vErr1.Message, vErr2.Message)
			}
		})
	}
}

func TestSupportedQueryTypes(t *testing.T) {
	// Test that all expected types are supported for query params
	supportedTypes := []struct {
		name  string
		field any
	}{
		{"string", ""},
		{"int", int(0)},
		{"int8", int8(0)},
		{"int16", int16(0)},
		{"int32", int32(0)},
		{"int64", int64(0)},
		{"uint", uint(0)},
		{"uint8", uint8(0)},
		{"uint16", uint16(0)},
		{"uint32", uint32(0)},
		{"uint64", uint64(0)},
		{"float32", float32(0)},
		{"float64", float64(0)},
		{"bool", false},
		{"time.Time", time.Time{}},
		{"[]string", []string{}},
		{"[]int", []int{}},
		{"*string", (*string)(nil)},
		{"*int", (*int)(nil)},
	}

	for _, tc := range supportedTypes {
		t.Run(tc.name, func(t *testing.T) {
			ft := reflect.TypeOf(tc.field)
			if !isSupportedQueryType(ft) {
				t.Errorf("expected %s to be supported for query params", tc.name)
			}
		})
	}
}

// Helper function to check if a string contains a substring
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
