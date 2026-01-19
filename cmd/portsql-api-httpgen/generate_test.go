package main

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// Phase 2 TDD: Golden tests for binder-based code generation
// These tests assert the new code shape without runtime dependency.

func TestGenerate_NoRuntime(t *testing.T) {
	m := Manifest{Endpoints: []ManifestEndpoint{
		{
			Method:      "GET",
			Path:        "/pets/{id}",
			HandlerPkg:  "example.com/app/pets",
			HandlerName: "Get",
			Shape:       "ctx_req_resp_err",
			ReqType:     "example.com/app/pets.GetReq",
			RespType:    "example.com/app/pets.GetResp",
			Bindings: &BindingInfo{
				PathBindings: []FieldBinding{
					{FieldName: "ID", TagValue: "id", TypeKind: "string"},
				},
			},
		},
	}}

	code, err := Generate(m, "api", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Run("does NOT import runtime", func(t *testing.T) {
		if strings.Contains(code, "api/portapi/runtime") {
			t.Error("generated code must NOT import api/portapi/runtime")
		}
	})

	t.Run("does NOT use runtime.Wrap", func(t *testing.T) {
		if strings.Contains(code, "runtime.Wrap") {
			t.Error("generated code must NOT use runtime.Wrap")
		}
	})

	t.Run("contains BindError type", func(t *testing.T) {
		if !strings.Contains(code, "type BindError struct") {
			t.Error("generated code must contain BindError type")
		}
	})

	t.Run("contains writeError helper", func(t *testing.T) {
		if !strings.Contains(code, "func writeError(") {
			t.Error("generated code must contain writeError helper")
		}
	})

	t.Run("contains writeJSON helper", func(t *testing.T) {
		if !strings.Contains(code, "func writeJSON(") {
			t.Error("generated code must contain writeJSON helper")
		}
	})

	t.Run("is valid Go syntax", func(t *testing.T) {
		if _, err := parser.ParseFile(token.NewFileSet(), "", code, 0); err != nil {
			t.Errorf("generated code must be valid Go: %v\n\nCode:\n%s", err, code)
		}
	})
}

func TestGenerate_BinderFunctions(t *testing.T) {
	t.Run("generates path binding code", func(t *testing.T) {
		m := Manifest{Endpoints: []ManifestEndpoint{
			{
				Method:      "GET",
				Path:        "/pets/{id}",
				HandlerPkg:  "example.com/app/pets",
				HandlerName: "Get",
				Shape:       "ctx_req_resp_err",
				ReqType:     "example.com/app/pets.GetReq",
				RespType:    "example.com/app/pets.GetResp",
				Bindings: &BindingInfo{
					PathBindings: []FieldBinding{
						{FieldName: "ID", TagValue: "id", TypeKind: "string"},
					},
				},
			},
		}}

		code, err := Generate(m, "api", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !strings.Contains(code, `r.PathValue("id")`) {
			t.Error("generated code must contain r.PathValue for path bindings")
		}
	})

	t.Run("generates query binding code", func(t *testing.T) {
		m := Manifest{Endpoints: []ManifestEndpoint{
			{
				Method:      "GET",
				Path:        "/pets",
				HandlerPkg:  "example.com/app/pets",
				HandlerName: "List",
				Shape:       "ctx_req_resp_err",
				ReqType:     "example.com/app/pets.ListReq",
				RespType:    "example.com/app/pets.ListResp",
				Bindings: &BindingInfo{
					QueryBindings: []FieldBinding{
						{FieldName: "Limit", TagValue: "limit", TypeKind: "int"},
					},
				},
			},
		}}

		code, err := Generate(m, "api", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !strings.Contains(code, `r.URL.Query()`) {
			t.Error("generated code must contain r.URL.Query() for query bindings")
		}
	})

	t.Run("generates header binding code", func(t *testing.T) {
		m := Manifest{Endpoints: []ManifestEndpoint{
			{
				Method:      "GET",
				Path:        "/pets/{id}",
				HandlerPkg:  "example.com/app/pets",
				HandlerName: "Get",
				Shape:       "ctx_req_resp_err",
				ReqType:     "example.com/app/pets.GetReq",
				RespType:    "example.com/app/pets.GetResp",
				Bindings: &BindingInfo{
					PathBindings: []FieldBinding{
						{FieldName: "ID", TagValue: "id", TypeKind: "string"},
					},
					HeaderBindings: []FieldBinding{
						{FieldName: "Auth", TagValue: "Authorization", TypeKind: "string"},
					},
				},
			},
		}}

		code, err := Generate(m, "api", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !strings.Contains(code, `r.Header.Get("Authorization")`) {
			t.Error("generated code must contain r.Header.Get for header bindings")
		}
	})

	t.Run("generates JSON body binding code", func(t *testing.T) {
		m := Manifest{Endpoints: []ManifestEndpoint{
			{
				Method:      "POST",
				Path:        "/pets",
				HandlerPkg:  "example.com/app/pets",
				HandlerName: "Create",
				Shape:       "ctx_req_resp_err",
				ReqType:     "example.com/app/pets.CreateReq",
				RespType:    "example.com/app/pets.CreateResp",
				Bindings: &BindingInfo{
					HasJSONBody: true,
				},
			},
		}}

		code, err := Generate(m, "api", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !strings.Contains(code, `json.NewDecoder(r.Body)`) {
			t.Error("generated code must contain json.NewDecoder for JSON body bindings")
		}
	})

	t.Run("generates binder function for endpoint with request", func(t *testing.T) {
		m := Manifest{Endpoints: []ManifestEndpoint{
			{
				Method:      "GET",
				Path:        "/pets/{id}",
				HandlerPkg:  "example.com/app/pets",
				HandlerName: "Get",
				Shape:       "ctx_req_resp_err",
				ReqType:     "example.com/app/pets.GetReq",
				RespType:    "example.com/app/pets.GetResp",
				Bindings: &BindingInfo{
					PathBindings: []FieldBinding{
						{FieldName: "ID", TagValue: "id", TypeKind: "string"},
					},
				},
			},
		}}

		code, err := Generate(m, "api", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !strings.Contains(code, "func bindGet(") {
			t.Error("generated code must contain binder function bindGet")
		}
	})

	t.Run("generates handler wrapper function", func(t *testing.T) {
		m := Manifest{Endpoints: []ManifestEndpoint{
			{
				Method:      "GET",
				Path:        "/pets/{id}",
				HandlerPkg:  "example.com/app/pets",
				HandlerName: "Get",
				Shape:       "ctx_req_resp_err",
				ReqType:     "example.com/app/pets.GetReq",
				RespType:    "example.com/app/pets.GetResp",
				Bindings: &BindingInfo{
					PathBindings: []FieldBinding{
						{FieldName: "ID", TagValue: "id", TypeKind: "string"},
					},
				},
			},
		}}

		code, err := Generate(m, "api", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !strings.Contains(code, "func handleGet(") {
			t.Error("generated code must contain handler wrapper handleGet")
		}
	})
}

func TestGenerate_ParseHelpers(t *testing.T) {
	t.Run("generates parseInt for int query params", func(t *testing.T) {
		m := Manifest{Endpoints: []ManifestEndpoint{
			{
				Method:      "GET",
				Path:        "/pets",
				HandlerPkg:  "example.com/app/pets",
				HandlerName: "List",
				Shape:       "ctx_req_resp_err",
				ReqType:     "example.com/app/pets.ListReq",
				RespType:    "example.com/app/pets.ListResp",
				Bindings: &BindingInfo{
					QueryBindings: []FieldBinding{
						{FieldName: "Limit", TagValue: "limit", TypeKind: "int"},
					},
				},
			},
		}}

		code, err := Generate(m, "api", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !strings.Contains(code, "func parseInt(") {
			t.Error("generated code must contain parseInt helper for int bindings")
		}
	})

	t.Run("generates parseBool for bool query params", func(t *testing.T) {
		m := Manifest{Endpoints: []ManifestEndpoint{
			{
				Method:      "GET",
				Path:        "/pets/{id}",
				HandlerPkg:  "example.com/app/pets",
				HandlerName: "Get",
				Shape:       "ctx_req_resp_err",
				ReqType:     "example.com/app/pets.GetReq",
				RespType:    "example.com/app/pets.GetResp",
				Bindings: &BindingInfo{
					PathBindings: []FieldBinding{
						{FieldName: "ID", TagValue: "id", TypeKind: "string"},
					},
					QueryBindings: []FieldBinding{
						{FieldName: "Verbose", TagValue: "verbose", TypeKind: "bool", IsPointer: true},
					},
				},
			},
		}}

		code, err := Generate(m, "api", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !strings.Contains(code, "func parseBool(") {
			t.Error("generated code must contain parseBool helper for bool bindings")
		}
	})
}

func TestGenerate_AllQueryTypes(t *testing.T) {
	// Test all numeric types that validation supports
	types := []struct {
		typeKind  string
		parseFunc string
	}{
		{"int8", "parseInt8"},
		{"int16", "parseInt16"},
		{"int32", "parseInt32"},
		{"uint", "parseUint"},
		{"uint8", "parseUint8"},
		{"uint16", "parseUint16"},
		{"uint32", "parseUint32"},
		{"uint64", "parseUint64"},
		{"float32", "parseFloat32"},
	}

	for _, tt := range types {
		t.Run("required_"+tt.typeKind, func(t *testing.T) {
			m := Manifest{Endpoints: []ManifestEndpoint{{
				Method:      "GET",
				Path:        "/items",
				HandlerPkg:  "example.com/app/items",
				HandlerName: "List",
				Shape:       "ctx_req_resp_err",
				ReqType:     "example.com/app/items.ListReq",
				RespType:    "example.com/app/items.ListResp",
				Bindings: &BindingInfo{
					QueryBindings: []FieldBinding{
						{FieldName: "Value", TagValue: "value", TypeKind: tt.typeKind},
					},
				},
			}}}

			code, err := Generate(m, "api", "")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !strings.Contains(code, "func "+tt.parseFunc+"(") {
				t.Errorf("generated code must contain %s helper", tt.parseFunc)
			}
			// Verify the parse function is actually called in the binder
			if !strings.Contains(code, tt.parseFunc+"(s)") {
				t.Errorf("generated binder must call %s(s) for required query param", tt.parseFunc)
			}
		})

		t.Run("pointer_"+tt.typeKind, func(t *testing.T) {
			m := Manifest{Endpoints: []ManifestEndpoint{{
				Method:      "GET",
				Path:        "/items",
				HandlerPkg:  "example.com/app/items",
				HandlerName: "List",
				Shape:       "ctx_req_resp_err",
				ReqType:     "example.com/app/items.ListReq",
				RespType:    "example.com/app/items.ListResp",
				Bindings: &BindingInfo{
					QueryBindings: []FieldBinding{
						{FieldName: "Value", TagValue: "value", TypeKind: tt.typeKind, IsPointer: true},
					},
				},
			}}}

			code, err := Generate(m, "api", "")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !strings.Contains(code, "func "+tt.parseFunc+"(") {
				t.Errorf("generated code must contain %s helper for pointer type", tt.parseFunc)
			}
			// Verify the parse function is actually called in the binder
			if !strings.Contains(code, tt.parseFunc+"(s)") {
				t.Errorf("generated binder must call %s(s) for pointer query param", tt.parseFunc)
			}
		})
	}

	// Test slice types
	sliceTypes := []struct {
		elemKind  string
		parseFunc string
	}{
		{"int8", "parseInt8"},
		{"int16", "parseInt16"},
		{"int32", "parseInt32"},
		{"int64", "parseInt64"},
		{"uint", "parseUint"},
		{"uint8", "parseUint8"},
		{"uint16", "parseUint16"},
		{"uint32", "parseUint32"},
		{"uint64", "parseUint64"},
		{"float32", "parseFloat32"},
		{"float64", "parseFloat64"},
	}

	for _, tt := range sliceTypes {
		t.Run("slice_"+tt.elemKind, func(t *testing.T) {
			m := Manifest{Endpoints: []ManifestEndpoint{{
				Method:      "GET",
				Path:        "/items",
				HandlerPkg:  "example.com/app/items",
				HandlerName: "List",
				Shape:       "ctx_req_resp_err",
				ReqType:     "example.com/app/items.ListReq",
				RespType:    "example.com/app/items.ListResp",
				Bindings: &BindingInfo{
					QueryBindings: []FieldBinding{
						{FieldName: "Values", TagValue: "value", TypeKind: "[]" + tt.elemKind, IsSlice: true, ElemKind: tt.elemKind},
					},
				},
			}}}

			code, err := Generate(m, "api", "")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !strings.Contains(code, "func "+tt.parseFunc+"(") {
				t.Errorf("generated code must contain %s helper for slice type", tt.parseFunc)
			}
			// Verify the parse function is actually called in the binder for slice elements
			if !strings.Contains(code, tt.parseFunc+"(s)") {
				t.Errorf("generated binder must call %s(s) for slice element parsing", tt.parseFunc)
			}
		})
	}
}

func TestGenerate_HeaderTypes(t *testing.T) {
	// Test numeric header types
	types := []struct {
		typeKind  string
		parseFunc string
	}{
		{"int", "parseInt"},
		{"int64", "parseInt64"},
		{"bool", "parseBool"},
		{"float64", "parseFloat64"},
	}

	for _, tt := range types {
		t.Run("required_"+tt.typeKind, func(t *testing.T) {
			m := Manifest{Endpoints: []ManifestEndpoint{{
				Method:      "GET",
				Path:        "/items",
				HandlerPkg:  "example.com/app/items",
				HandlerName: "List",
				Shape:       "ctx_req_resp_err",
				ReqType:     "example.com/app/items.ListReq",
				RespType:    "example.com/app/items.ListResp",
				Bindings: &BindingInfo{
					HeaderBindings: []FieldBinding{
						{FieldName: "Value", TagValue: "X-Value", TypeKind: tt.typeKind},
					},
				},
			}}}

			code, err := Generate(m, "api", "")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !strings.Contains(code, "func "+tt.parseFunc+"(") {
				t.Errorf("generated code must contain %s helper for header type %s", tt.parseFunc, tt.typeKind)
			}
			if !strings.Contains(code, `r.Header.Get("X-Value")`) {
				t.Error("generated code must read the header")
			}
			// Verify the parse function is actually called in the binder
			if !strings.Contains(code, tt.parseFunc+"(h)") {
				t.Errorf("generated binder must call %s(h) for header parsing", tt.parseFunc)
			}
		})

		t.Run("pointer_"+tt.typeKind, func(t *testing.T) {
			m := Manifest{Endpoints: []ManifestEndpoint{{
				Method:      "GET",
				Path:        "/items",
				HandlerPkg:  "example.com/app/items",
				HandlerName: "List",
				Shape:       "ctx_req_resp_err",
				ReqType:     "example.com/app/items.ListReq",
				RespType:    "example.com/app/items.ListResp",
				Bindings: &BindingInfo{
					HeaderBindings: []FieldBinding{
						{FieldName: "Value", TagValue: "X-Value", TypeKind: tt.typeKind, IsPointer: true},
					},
				},
			}}}

			code, err := Generate(m, "api", "")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !strings.Contains(code, "func "+tt.parseFunc+"(") {
				t.Errorf("generated code must contain %s helper for pointer header type %s", tt.parseFunc, tt.typeKind)
			}
			// Verify the parse function is actually called in the binder
			if !strings.Contains(code, tt.parseFunc+"(h)") {
				t.Errorf("generated binder must call %s(h) for pointer header parsing", tt.parseFunc)
			}
		})
	}
}

func TestGenerate_NilSliceResponse(t *testing.T) {
	t.Run("slice response has nil check", func(t *testing.T) {
		m := Manifest{Endpoints: []ManifestEndpoint{{
			Method:      "GET",
			Path:        "/pets",
			HandlerPkg:  "example.com/app/pets",
			HandlerName: "List",
			Shape:       "ctx_resp_err",
			RespType:    "[]example.com/app/pets.Pet",
		}}}

		code, err := Generate(m, "api", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should have nil check for slice response
		if !strings.Contains(code, "if resp == nil") {
			t.Error("generated code must check for nil slice response")
		}
	})

	t.Run("non-slice response has no nil check", func(t *testing.T) {
		m := Manifest{Endpoints: []ManifestEndpoint{{
			Method:      "GET",
			Path:        "/pets/{id}",
			HandlerPkg:  "example.com/app/pets",
			HandlerName: "Get",
			Shape:       "ctx_req_resp_err",
			ReqType:     "example.com/app/pets.GetReq",
			RespType:    "example.com/app/pets.Pet",
			Bindings: &BindingInfo{
				PathBindings: []FieldBinding{
					{FieldName: "ID", TagValue: "id", TypeKind: "string"},
				},
			},
		}}}

		code, err := Generate(m, "api", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should NOT have nil check for non-slice response
		if strings.Contains(code, "if resp == nil") {
			t.Error("generated code should not check for nil on non-slice response")
		}
	})
}

func TestGenerate_HandlerShapes(t *testing.T) {
	t.Run("ctx_req_resp_err calls handler and writes JSON", func(t *testing.T) {
		m := Manifest{Endpoints: []ManifestEndpoint{
			{
				Method:      "GET",
				Path:        "/pets/{id}",
				HandlerPkg:  "example.com/app/pets",
				HandlerName: "Get",
				Shape:       "ctx_req_resp_err",
				ReqType:     "example.com/app/pets.GetReq",
				RespType:    "example.com/app/pets.GetResp",
				Bindings: &BindingInfo{
					PathBindings: []FieldBinding{
						{FieldName: "ID", TagValue: "id", TypeKind: "string"},
					},
				},
			},
		}}

		code, err := Generate(m, "api", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !strings.Contains(code, "writeJSON(w, http.StatusOK") {
			t.Error("generated code must write JSON response for ctx_req_resp_err")
		}
	})

	t.Run("ctx_req_err writes 204 on success", func(t *testing.T) {
		m := Manifest{Endpoints: []ManifestEndpoint{
			{
				Method:      "DELETE",
				Path:        "/pets/{id}",
				HandlerPkg:  "example.com/app/pets",
				HandlerName: "Delete",
				Shape:       "ctx_req_err",
				ReqType:     "example.com/app/pets.DeleteReq",
				Bindings: &BindingInfo{
					PathBindings: []FieldBinding{
						{FieldName: "ID", TagValue: "id", TypeKind: "string"},
					},
				},
			},
		}}

		code, err := Generate(m, "api", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !strings.Contains(code, "w.WriteHeader(http.StatusNoContent)") {
			t.Error("generated code must write 204 for ctx_req_err success")
		}
	})

	t.Run("ctx_resp_err has no binder but writes JSON", func(t *testing.T) {
		m := Manifest{Endpoints: []ManifestEndpoint{
			{
				Method:      "GET",
				Path:        "/pets",
				HandlerPkg:  "example.com/app/pets",
				HandlerName: "List",
				Shape:       "ctx_resp_err",
				RespType:    "example.com/app/pets.ListResp",
			},
		}}

		code, err := Generate(m, "api", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should NOT have a binder for this endpoint
		if strings.Contains(code, "func bindList(") {
			t.Error("ctx_resp_err should not have a binder function")
		}

		if !strings.Contains(code, "writeJSON(w, http.StatusOK") {
			t.Error("generated code must write JSON response for ctx_resp_err")
		}
	})

	t.Run("ctx_err writes 204 on success with no binder", func(t *testing.T) {
		m := Manifest{Endpoints: []ManifestEndpoint{
			{
				Method:      "GET",
				Path:        "/health",
				HandlerPkg:  "example.com/app",
				HandlerName: "Health",
				Shape:       "ctx_err",
			},
		}}

		code, err := Generate(m, "api", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should NOT have a binder for this endpoint
		if strings.Contains(code, "func bindHealth(") {
			t.Error("ctx_err should not have a binder function")
		}

		if !strings.Contains(code, "w.WriteHeader(http.StatusNoContent)") {
			t.Error("generated code must write 204 for ctx_err success")
		}
	})
}

func TestGenerate(t *testing.T) {
	t.Run("generates valid Go", func(t *testing.T) {
		m := Manifest{Endpoints: []ManifestEndpoint{
			{Method: "GET", Path: "/pets", HandlerPkg: "example.com/app/pets", HandlerName: "List", Shape: "ctx_resp_err", RespType: "[]Pet"},
		}}

		code, err := Generate(m, "api", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Parse with go/parser
		if _, err := parser.ParseFile(token.NewFileSet(), "", code, 0); err != nil {
			t.Fatalf("generated code must parse: %v", err)
		}
	})

	t.Run("includes DO NOT EDIT header", func(t *testing.T) {
		code, err := Generate(Manifest{Endpoints: []ManifestEndpoint{}}, "api", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(code, "DO NOT EDIT") {
			t.Error("expected code to contain DO NOT EDIT header")
		}
	})

	t.Run("registers all endpoints", func(t *testing.T) {
		m := Manifest{Endpoints: []ManifestEndpoint{
			{Method: "GET", Path: "/pets", HandlerPkg: "example.com/app/pets", HandlerName: "List", Shape: "ctx_resp_err", RespType: "[]Pet"},
			{Method: "POST", Path: "/pets", HandlerPkg: "example.com/app/pets", HandlerName: "Create", Shape: "ctx_req_resp_err", ReqType: "CreateReq", RespType: "Pet"},
		}}

		code, err := Generate(m, "api", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(code, `"GET /pets"`) {
			t.Error("expected code to contain GET /pets pattern")
		}
		if !strings.Contains(code, `"POST /pets"`) {
			t.Error("expected code to contain POST /pets pattern")
		}
	})

	t.Run("deterministic output", func(t *testing.T) {
		m := Manifest{Endpoints: []ManifestEndpoint{
			{Method: "POST", Path: "/b", HandlerPkg: "example.com/x", HandlerName: "B", Shape: "ctx_err"},
			{Method: "GET", Path: "/a", HandlerPkg: "example.com/x", HandlerName: "A", Shape: "ctx_err"},
		}}

		code1, err := Generate(m, "api", "")
		if err != nil {
			t.Fatalf("first generate failed: %v", err)
		}

		code2, err := Generate(m, "api", "")
		if err != nil {
			t.Fatalf("second generate failed: %v", err)
		}

		if code1 != code2 {
			t.Error("expected deterministic output")
		}
	})

	// NOTE: runtime.Wrap* tests removed - we no longer use runtime wrappers
	// See TestGenerate_HandlerShapes for the new handler generation tests

	t.Run("imports handler packages", func(t *testing.T) {
		m := Manifest{Endpoints: []ManifestEndpoint{
			{Method: "GET", Path: "/pets", HandlerPkg: "example.com/app/pets", HandlerName: "List", Shape: "ctx_resp_err", RespType: "[]Pet"},
			{Method: "GET", Path: "/users", HandlerPkg: "example.com/app/users", HandlerName: "List", Shape: "ctx_resp_err", RespType: "[]User"},
		}}

		code, err := Generate(m, "api", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(code, `"example.com/app/pets"`) {
			t.Error("expected code to contain pets import")
		}
		if !strings.Contains(code, `"example.com/app/users"`) {
			t.Error("expected code to contain users import")
		}
	})

	// NOTE: runtime import test removed - we no longer use runtime package

	t.Run("handles multiple endpoints from same package", func(t *testing.T) {
		m := Manifest{Endpoints: []ManifestEndpoint{
			{Method: "GET", Path: "/pets", HandlerPkg: "example.com/app/pets", HandlerName: "List", Shape: "ctx_resp_err", RespType: "[]Pet"},
			{Method: "POST", Path: "/pets", HandlerPkg: "example.com/app/pets", HandlerName: "Create", Shape: "ctx_req_resp_err", ReqType: "CreateReq", RespType: "Pet"},
			{Method: "GET", Path: "/pets/{id}", HandlerPkg: "example.com/app/pets", HandlerName: "Get", Shape: "ctx_req_resp_err", ReqType: "GetReq", RespType: "Pet"},
		}}

		code, err := Generate(m, "api", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should parse successfully
		if _, err := parser.ParseFile(token.NewFileSet(), "", code, 0); err != nil {
			t.Fatalf("generated code must parse: %v", err)
		}

		// Should only import the package once
		// Count occurrences of the import
		count := 0
		searchStr := `"example.com/app/pets"`
		for i := 0; i <= len(code)-len(searchStr); i++ {
			if code[i:i+len(searchStr)] == searchStr {
				count++
			}
		}
		if count != 1 {
			t.Errorf("expected package to be imported once, got %d", count)
		}
	})

	t.Run("empty manifest generates valid code", func(t *testing.T) {
		m := Manifest{Endpoints: []ManifestEndpoint{}}

		code, err := Generate(m, "api", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Parse with go/parser
		if _, err := parser.ParseFile(token.NewFileSet(), "", code, 0); err != nil {
			t.Fatalf("generated code must parse: %v", err)
		}

		if !strings.Contains(code, "package api") {
			t.Error("expected code to contain package api")
		}
		if !strings.Contains(code, "func NewMux()") {
			t.Error("expected code to contain func NewMux()")
		}
	})

	t.Run("handles package alias conflicts", func(t *testing.T) {
		m := Manifest{Endpoints: []ManifestEndpoint{
			{Method: "GET", Path: "/a", HandlerPkg: "example.com/app/api", HandlerName: "A", Shape: "ctx_err"},
			{Method: "GET", Path: "/b", HandlerPkg: "example.com/other/api", HandlerName: "B", Shape: "ctx_err"},
		}}

		code, err := Generate(m, "myapi", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should parse successfully (aliases should be unique)
		if _, err := parser.ParseFile(token.NewFileSet(), "", code, 0); err != nil {
			t.Fatalf("generated code must parse: %v", err)
		}
	})

	t.Run("sorts endpoints for determinism", func(t *testing.T) {
		m1 := Manifest{Endpoints: []ManifestEndpoint{
			{Method: "POST", Path: "/z", HandlerPkg: "example.com/x", HandlerName: "Z", Shape: "ctx_err"},
			{Method: "GET", Path: "/a", HandlerPkg: "example.com/x", HandlerName: "A", Shape: "ctx_err"},
			{Method: "DELETE", Path: "/m", HandlerPkg: "example.com/x", HandlerName: "M", Shape: "ctx_err"},
		}}

		m2 := Manifest{Endpoints: []ManifestEndpoint{
			{Method: "GET", Path: "/a", HandlerPkg: "example.com/x", HandlerName: "A", Shape: "ctx_err"},
			{Method: "DELETE", Path: "/m", HandlerPkg: "example.com/x", HandlerName: "M", Shape: "ctx_err"},
			{Method: "POST", Path: "/z", HandlerPkg: "example.com/x", HandlerName: "Z", Shape: "ctx_err"},
		}}

		code1, err := Generate(m1, "api", "")
		if err != nil {
			t.Fatalf("first generate failed: %v", err)
		}

		code2, err := Generate(m2, "api", "")
		if err != nil {
			t.Fatalf("second generate failed: %v", err)
		}

		if code1 != code2 {
			t.Error("different ordering should produce same output")
		}
	})
}
