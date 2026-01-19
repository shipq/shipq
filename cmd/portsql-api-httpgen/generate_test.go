package main

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestGenerate(t *testing.T) {
	t.Run("generates valid Go", func(t *testing.T) {
		m := Manifest{Endpoints: []ManifestEndpoint{
			{Method: "GET", Path: "/pets", HandlerPkg: "example.com/app/pets", HandlerName: "List", Shape: "ctx_resp_err", RespType: "[]Pet"},
		}}

		code, err := Generate(m, "api")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Parse with go/parser
		if _, err := parser.ParseFile(token.NewFileSet(), "", code, 0); err != nil {
			t.Fatalf("generated code must parse: %v", err)
		}
	})

	t.Run("includes DO NOT EDIT header", func(t *testing.T) {
		code, err := Generate(Manifest{Endpoints: []ManifestEndpoint{}}, "api")
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

		code, err := Generate(m, "api")
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

		code1, err := Generate(m, "api")
		if err != nil {
			t.Fatalf("first generate failed: %v", err)
		}

		code2, err := Generate(m, "api")
		if err != nil {
			t.Fatalf("second generate failed: %v", err)
		}

		if code1 != code2 {
			t.Error("expected deterministic output")
		}
	})

	t.Run("uses correct wrapper for ctx_req_resp_err", func(t *testing.T) {
		m := Manifest{Endpoints: []ManifestEndpoint{
			{Method: "POST", Path: "/pets", HandlerPkg: "example.com/app/pets", HandlerName: "Create", Shape: "ctx_req_resp_err", ReqType: "CreateReq", RespType: "Pet"},
		}}

		code, err := Generate(m, "api")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(code, "runtime.WrapCtxReqRespErr") {
			t.Error("expected code to contain runtime.WrapCtxReqRespErr")
		}
	})

	t.Run("uses correct wrapper for ctx_req_err", func(t *testing.T) {
		m := Manifest{Endpoints: []ManifestEndpoint{
			{Method: "DELETE", Path: "/pets/{id}", HandlerPkg: "example.com/app/pets", HandlerName: "Delete", Shape: "ctx_req_err", ReqType: "DeleteReq"},
		}}

		code, err := Generate(m, "api")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(code, "runtime.WrapCtxReqErr") {
			t.Error("expected code to contain runtime.WrapCtxReqErr")
		}
	})

	t.Run("uses correct wrapper for ctx_resp_err", func(t *testing.T) {
		m := Manifest{Endpoints: []ManifestEndpoint{
			{Method: "GET", Path: "/pets", HandlerPkg: "example.com/app/pets", HandlerName: "List", Shape: "ctx_resp_err", RespType: "[]Pet"},
		}}

		code, err := Generate(m, "api")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(code, "runtime.WrapCtxRespErr") {
			t.Error("expected code to contain runtime.WrapCtxRespErr")
		}
	})

	t.Run("uses correct wrapper for ctx_err", func(t *testing.T) {
		m := Manifest{Endpoints: []ManifestEndpoint{
			{Method: "POST", Path: "/health", HandlerPkg: "example.com/app", HandlerName: "Health", Shape: "ctx_err"},
		}}

		code, err := Generate(m, "api")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(code, "runtime.WrapCtxErr") {
			t.Error("expected code to contain runtime.WrapCtxErr")
		}
	})

	t.Run("imports handler packages", func(t *testing.T) {
		m := Manifest{Endpoints: []ManifestEndpoint{
			{Method: "GET", Path: "/pets", HandlerPkg: "example.com/app/pets", HandlerName: "List", Shape: "ctx_resp_err", RespType: "[]Pet"},
			{Method: "GET", Path: "/users", HandlerPkg: "example.com/app/users", HandlerName: "List", Shape: "ctx_resp_err", RespType: "[]User"},
		}}

		code, err := Generate(m, "api")
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

	t.Run("imports runtime package", func(t *testing.T) {
		m := Manifest{Endpoints: []ManifestEndpoint{
			{Method: "GET", Path: "/pets", HandlerPkg: "example.com/app/pets", HandlerName: "List", Shape: "ctx_resp_err", RespType: "[]Pet"},
		}}

		code, err := Generate(m, "api")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(code, `"github.com/shipq/shipq/api/portapi/runtime"`) {
			t.Error("expected code to contain runtime import")
		}
	})

	t.Run("handles multiple endpoints from same package", func(t *testing.T) {
		m := Manifest{Endpoints: []ManifestEndpoint{
			{Method: "GET", Path: "/pets", HandlerPkg: "example.com/app/pets", HandlerName: "List", Shape: "ctx_resp_err", RespType: "[]Pet"},
			{Method: "POST", Path: "/pets", HandlerPkg: "example.com/app/pets", HandlerName: "Create", Shape: "ctx_req_resp_err", ReqType: "CreateReq", RespType: "Pet"},
			{Method: "GET", Path: "/pets/{id}", HandlerPkg: "example.com/app/pets", HandlerName: "Get", Shape: "ctx_req_resp_err", ReqType: "GetReq", RespType: "Pet"},
		}}

		code, err := Generate(m, "api")
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

		code, err := Generate(m, "api")
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

		code, err := Generate(m, "myapi")
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

		code1, err := Generate(m1, "api")
		if err != nil {
			t.Fatalf("first generate failed: %v", err)
		}

		code2, err := Generate(m2, "api")
		if err != nil {
			t.Fatalf("second generate failed: %v", err)
		}

		if code1 != code2 {
			t.Error("different ordering should produce same output")
		}
	})
}
