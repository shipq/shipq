package main

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// =============================================================================
// OpenAPI Step 3 Tests: BuildOpenAPI function
// =============================================================================

// Test 1: Minimal OpenAPI document emitted
func TestBuildOpenAPI_Minimal(t *testing.T) {
	cfg := &Config{
		OpenAPIEnabled: true,
		OpenAPITitle:   "Test API",
		OpenAPIVersion: "1.0.0",
		OpenAPIOutput:  "openapi.json",
	}

	manifest := &Manifest{
		Endpoints: []ManifestEndpoint{
			{
				Method:      "GET",
				Path:        "/health",
				HandlerPkg:  "example.com/api",
				HandlerName: "HealthCheck",
				Shape:       "ctx_err",
			},
		},
		Types:        []ManifestType{},
		EndpointDocs: map[string]ManifestDoc{},
	}

	jsonBytes, err := BuildOpenAPI(cfg, manifest)
	if err != nil {
		t.Fatalf("BuildOpenAPI failed: %v", err)
	}

	var doc map[string]any
	if err := json.Unmarshal(jsonBytes, &doc); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	// Check openapi version
	if doc["openapi"] != "3.0.3" {
		t.Errorf("expected openapi='3.0.3', got %v", doc["openapi"])
	}

	// Check info
	info, ok := doc["info"].(map[string]any)
	if !ok {
		t.Fatal("expected info object")
	}
	if info["title"] != "Test API" {
		t.Errorf("expected info.title='Test API', got %v", info["title"])
	}
	if info["version"] != "1.0.0" {
		t.Errorf("expected info.version='1.0.0', got %v", info["version"])
	}

	// Check paths
	paths, ok := doc["paths"].(map[string]any)
	if !ok {
		t.Fatal("expected paths object")
	}
	healthPath, ok := paths["/health"].(map[string]any)
	if !ok {
		t.Fatal("expected /health path")
	}
	getOp, ok := healthPath["get"].(map[string]any)
	if !ok {
		t.Fatal("expected get operation on /health")
	}

	// Check operationId is present and deterministic
	opID, ok := getOp["operationId"].(string)
	if !ok || opID == "" {
		t.Error("expected operationId to be present")
	}

	// Check responses include 204 (no content)
	responses, ok := getOp["responses"].(map[string]any)
	if !ok {
		t.Fatal("expected responses object")
	}
	if _, ok := responses["204"]; !ok {
		t.Error("expected 204 response for ctx_err handler")
	}

	// Check no requestBody exists
	if _, ok := getOp["requestBody"]; ok {
		t.Error("expected no requestBody for handler without request")
	}
}

// Test 2: Request parameters mapping (path/query/header)
func TestBuildOpenAPI_Parameters(t *testing.T) {
	cfg := &Config{
		OpenAPIEnabled: true,
		OpenAPITitle:   "Test API",
		OpenAPIVersion: "1.0.0",
	}

	manifest := &Manifest{
		Endpoints: []ManifestEndpoint{
			{
				Method:      "GET",
				Path:        "/pets/{id}",
				HandlerPkg:  "example.com/api",
				HandlerName: "GetPet",
				Shape:       "ctx_req_resp_err",
				ReqType:     "example.com/api.GetPetRequest",
				RespType:    "example.com/api.Pet",
				Bindings: &BindingInfo{
					HasJSONBody: false,
					PathBindings: []FieldBinding{
						{FieldName: "ID", TagValue: "id", TypeKind: "string", IsPointer: false},
					},
					QueryBindings: []FieldBinding{
						{FieldName: "Verbose", TagValue: "verbose", TypeKind: "bool", IsPointer: true},
					},
					HeaderBindings: []FieldBinding{
						{FieldName: "Authorization", TagValue: "Authorization", TypeKind: "string", IsPointer: false},
					},
				},
			},
		},
		Types: []ManifestType{
			{
				ID:     "example.com/api.Pet",
				GoType: "api.Pet",
				Kind:   "struct",
				Fields: []ManifestField{
					{GoName: "ID", JSONName: "id", TypeID: "string", Required: true},
					{GoName: "Name", JSONName: "name", TypeID: "string", Required: true},
				},
			},
		},
		EndpointDocs: map[string]ManifestDoc{},
	}

	jsonBytes, err := BuildOpenAPI(cfg, manifest)
	if err != nil {
		t.Fatalf("BuildOpenAPI failed: %v", err)
	}

	var doc map[string]any
	if err := json.Unmarshal(jsonBytes, &doc); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	paths := doc["paths"].(map[string]any)
	petPath := paths["/pets/{id}"].(map[string]any)
	getOp := petPath["get"].(map[string]any)
	params := getOp["parameters"].([]any)

	if len(params) != 3 {
		t.Fatalf("expected 3 parameters, got %d", len(params))
	}

	// Build a map of parameters by name for easier testing
	paramsByName := make(map[string]map[string]any)
	for _, p := range params {
		param := p.(map[string]any)
		name := param["name"].(string)
		paramsByName[name] = param
	}

	// Check path param: id
	idParam, ok := paramsByName["id"]
	if !ok {
		t.Fatal("expected 'id' parameter")
	}
	if idParam["in"] != "path" {
		t.Errorf("expected id.in='path', got %v", idParam["in"])
	}
	if idParam["required"] != true {
		t.Errorf("expected id.required=true, got %v", idParam["required"])
	}
	idSchema := idParam["schema"].(map[string]any)
	if idSchema["type"] != "string" {
		t.Errorf("expected id.schema.type='string', got %v", idSchema["type"])
	}

	// Check query param: verbose (pointer, optional)
	verboseParam, ok := paramsByName["verbose"]
	if !ok {
		t.Fatal("expected 'verbose' parameter")
	}
	if verboseParam["in"] != "query" {
		t.Errorf("expected verbose.in='query', got %v", verboseParam["in"])
	}
	if verboseParam["required"] != false {
		t.Errorf("expected verbose.required=false (pointer), got %v", verboseParam["required"])
	}
	verboseSchema := verboseParam["schema"].(map[string]any)
	if verboseSchema["type"] != "boolean" {
		t.Errorf("expected verbose.schema.type='boolean', got %v", verboseSchema["type"])
	}

	// Check header param: Authorization (required)
	authParam, ok := paramsByName["Authorization"]
	if !ok {
		t.Fatal("expected 'Authorization' parameter")
	}
	if authParam["in"] != "header" {
		t.Errorf("expected Authorization.in='header', got %v", authParam["in"])
	}
	if authParam["required"] != true {
		t.Errorf("expected Authorization.required=true, got %v", authParam["required"])
	}
}

// Test 3: JSON request body mapping
func TestBuildOpenAPI_RequestBody(t *testing.T) {
	cfg := &Config{
		OpenAPIEnabled: true,
		OpenAPITitle:   "Test API",
		OpenAPIVersion: "1.0.0",
	}

	manifest := &Manifest{
		Endpoints: []ManifestEndpoint{
			{
				Method:      "POST",
				Path:        "/pets",
				HandlerPkg:  "example.com/api",
				HandlerName: "CreatePet",
				Shape:       "ctx_req_resp_err",
				ReqType:     "example.com/api.CreatePetRequest",
				RespType:    "example.com/api.Pet",
				Bindings: &BindingInfo{
					HasJSONBody: true,
					QueryBindings: []FieldBinding{
						{FieldName: "DryRun", TagValue: "dry_run", TypeKind: "bool", IsPointer: true},
					},
				},
			},
		},
		Types: []ManifestType{
			{
				ID:     "example.com/api.CreatePetRequest",
				GoType: "api.CreatePetRequest",
				Kind:   "struct",
				Fields: []ManifestField{
					{GoName: "Name", JSONName: "name", TypeID: "string", Required: true},
					{GoName: "Species", JSONName: "species", TypeID: "string", Required: false},
					{GoName: "DryRun", JSONName: "", TypeID: "bool", Required: false}, // No JSON name - query param only
				},
			},
			{
				ID:     "example.com/api.Pet",
				GoType: "api.Pet",
				Kind:   "struct",
				Fields: []ManifestField{
					{GoName: "ID", JSONName: "id", TypeID: "string", Required: true},
					{GoName: "Name", JSONName: "name", TypeID: "string", Required: true},
				},
			},
		},
		EndpointDocs: map[string]ManifestDoc{},
	}

	jsonBytes, err := BuildOpenAPI(cfg, manifest)
	if err != nil {
		t.Fatalf("BuildOpenAPI failed: %v", err)
	}

	var doc map[string]any
	if err := json.Unmarshal(jsonBytes, &doc); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	paths := doc["paths"].(map[string]any)
	petsPath := paths["/pets"].(map[string]any)
	postOp := petsPath["post"].(map[string]any)

	// Check requestBody exists
	reqBody, ok := postOp["requestBody"].(map[string]any)
	if !ok {
		t.Fatal("expected requestBody")
	}
	if reqBody["required"] != true {
		t.Errorf("expected requestBody.required=true, got %v", reqBody["required"])
	}

	// Check content
	content, ok := reqBody["content"].(map[string]any)
	if !ok {
		t.Fatal("expected requestBody.content")
	}
	jsonContent, ok := content["application/json"].(map[string]any)
	if !ok {
		t.Fatal("expected application/json content")
	}
	schema, ok := jsonContent["schema"].(map[string]any)
	if !ok {
		t.Fatal("expected schema in json content")
	}
	// Schema should be a $ref
	if _, ok := schema["$ref"]; !ok {
		t.Error("expected schema to be a $ref")
	}

	// Check parameters include query param but NOT JSON fields
	params, ok := postOp["parameters"].([]any)
	if !ok || len(params) != 1 {
		t.Errorf("expected 1 parameter (dry_run), got %v", len(params))
	}
	if len(params) > 0 {
		param := params[0].(map[string]any)
		if param["name"] != "dry_run" {
			t.Errorf("expected parameter name='dry_run', got %v", param["name"])
		}
	}
}

// Test 4: Response schema mapping
func TestBuildOpenAPI_ResponseSchema(t *testing.T) {
	t.Run("array_response", func(t *testing.T) {
		cfg := &Config{
			OpenAPIEnabled: true,
			OpenAPITitle:   "Test API",
			OpenAPIVersion: "1.0.0",
		}

		manifest := &Manifest{
			Endpoints: []ManifestEndpoint{
				{
					Method:      "GET",
					Path:        "/pets",
					HandlerPkg:  "example.com/api",
					HandlerName: "ListPets",
					Shape:       "ctx_resp_err",
					RespType:    "[]example.com/api.Pet",
				},
			},
			Types: []ManifestType{
				{
					ID:     "[]example.com/api.Pet",
					GoType: "[]api.Pet",
					Kind:   "slice",
					Elem:   "example.com/api.Pet",
				},
				{
					ID:     "example.com/api.Pet",
					GoType: "api.Pet",
					Kind:   "struct",
					Fields: []ManifestField{
						{GoName: "ID", JSONName: "id", TypeID: "string", Required: true},
						{GoName: "Name", JSONName: "name", TypeID: "string", Required: true},
					},
				},
			},
			EndpointDocs: map[string]ManifestDoc{},
		}

		jsonBytes, err := BuildOpenAPI(cfg, manifest)
		if err != nil {
			t.Fatalf("BuildOpenAPI failed: %v", err)
		}

		var doc map[string]any
		if err := json.Unmarshal(jsonBytes, &doc); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}

		paths := doc["paths"].(map[string]any)
		petsPath := paths["/pets"].(map[string]any)
		getOp := petsPath["get"].(map[string]any)
		responses := getOp["responses"].(map[string]any)

		// Check 200 response
		resp200, ok := responses["200"].(map[string]any)
		if !ok {
			t.Fatal("expected 200 response")
		}
		content := resp200["content"].(map[string]any)
		jsonContent := content["application/json"].(map[string]any)
		schema := jsonContent["schema"].(map[string]any)

		// Should be array type
		if schema["type"] != "array" {
			t.Errorf("expected schema.type='array', got %v", schema["type"])
		}
		items, ok := schema["items"].(map[string]any)
		if !ok {
			t.Fatal("expected items in array schema")
		}
		if _, ok := items["$ref"]; !ok {
			t.Error("expected items to have $ref")
		}
	})

	t.Run("no_content_response", func(t *testing.T) {
		cfg := &Config{
			OpenAPIEnabled: true,
			OpenAPITitle:   "Test API",
			OpenAPIVersion: "1.0.0",
		}

		manifest := &Manifest{
			Endpoints: []ManifestEndpoint{
				{
					Method:      "GET",
					Path:        "/health",
					HandlerPkg:  "example.com/api",
					HandlerName: "Health",
					Shape:       "ctx_err",
				},
			},
			Types:        []ManifestType{},
			EndpointDocs: map[string]ManifestDoc{},
		}

		jsonBytes, err := BuildOpenAPI(cfg, manifest)
		if err != nil {
			t.Fatalf("BuildOpenAPI failed: %v", err)
		}

		var doc map[string]any
		if err := json.Unmarshal(jsonBytes, &doc); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}

		paths := doc["paths"].(map[string]any)
		healthPath := paths["/health"].(map[string]any)
		getOp := healthPath["get"].(map[string]any)
		responses := getOp["responses"].(map[string]any)

		// Check 204 response
		resp204, ok := responses["204"].(map[string]any)
		if !ok {
			t.Fatal("expected 204 response")
		}
		if resp204["description"] == "" {
			t.Error("expected description on 204 response")
		}
		// 204 should have no content
		if _, ok := resp204["content"]; ok {
			t.Error("expected no content on 204 response")
		}
	})
}

// Test 5: Middleware MayReturn adds responses
func TestBuildOpenAPI_MiddlewareResponses(t *testing.T) {
	cfg := &Config{
		OpenAPIEnabled: true,
		OpenAPITitle:   "Test API",
		OpenAPIVersion: "1.0.0",
	}

	manifest := &Manifest{
		Endpoints: []ManifestEndpoint{
			{
				Method:      "GET",
				Path:        "/protected",
				HandlerPkg:  "example.com/api",
				HandlerName: "ProtectedEndpoint",
				Shape:       "ctx_err",
				Middlewares: []ManifestMiddleware{
					{Pkg: "example.com/middleware", Name: "Auth"},
				},
			},
		},
		MiddlewareMetadata: map[string]*ManifestMiddlewareMetadata{
			"example.com/middleware.Auth": {
				MayReturnStatuses: []ManifestMayReturnStatus{
					{Status: 401, Description: "Unauthorized"},
					{Status: 403, Description: "Forbidden"},
				},
			},
		},
		Types:        []ManifestType{},
		EndpointDocs: map[string]ManifestDoc{},
	}

	jsonBytes, err := BuildOpenAPI(cfg, manifest)
	if err != nil {
		t.Fatalf("BuildOpenAPI failed: %v", err)
	}

	var doc map[string]any
	if err := json.Unmarshal(jsonBytes, &doc); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	paths := doc["paths"].(map[string]any)
	protectedPath := paths["/protected"].(map[string]any)
	getOp := protectedPath["get"].(map[string]any)
	responses := getOp["responses"].(map[string]any)

	// Check 401 response
	resp401, ok := responses["401"].(map[string]any)
	if !ok {
		t.Fatal("expected 401 response from middleware")
	}
	if resp401["description"] != "Unauthorized" {
		t.Errorf("expected 401 description='Unauthorized', got %v", resp401["description"])
	}

	// Check 403 response
	resp403, ok := responses["403"].(map[string]any)
	if !ok {
		t.Fatal("expected 403 response from middleware")
	}
	if resp403["description"] != "Forbidden" {
		t.Errorf("expected 403 description='Forbidden', got %v", resp403["description"])
	}

	// Check 500 response is included
	if _, ok := responses["500"]; !ok {
		t.Error("expected 500 response to be included")
	}
}

// Test 6: Shared error schema shape
func TestBuildOpenAPI_ErrorSchema(t *testing.T) {
	cfg := &Config{
		OpenAPIEnabled: true,
		OpenAPITitle:   "Test API",
		OpenAPIVersion: "1.0.0",
	}

	manifest := &Manifest{
		Endpoints: []ManifestEndpoint{
			{
				Method:      "GET",
				Path:        "/test",
				HandlerPkg:  "example.com/api",
				HandlerName: "Test",
				Shape:       "ctx_err",
			},
		},
		Types:        []ManifestType{},
		EndpointDocs: map[string]ManifestDoc{},
	}

	jsonBytes, err := BuildOpenAPI(cfg, manifest)
	if err != nil {
		t.Fatalf("BuildOpenAPI failed: %v", err)
	}

	var doc map[string]any
	if err := json.Unmarshal(jsonBytes, &doc); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	components, ok := doc["components"].(map[string]any)
	if !ok {
		t.Fatal("expected components")
	}
	schemas, ok := components["schemas"].(map[string]any)
	if !ok {
		t.Fatal("expected components.schemas")
	}

	// Check ErrorResponse schema
	errorResp, ok := schemas["ErrorResponse"].(map[string]any)
	if !ok {
		t.Fatal("expected ErrorResponse schema")
	}

	if errorResp["type"] != "object" {
		t.Errorf("expected ErrorResponse.type='object', got %v", errorResp["type"])
	}

	props, ok := errorResp["properties"].(map[string]any)
	if !ok {
		t.Fatal("expected properties")
	}

	errorProp, ok := props["error"].(map[string]any)
	if !ok {
		t.Fatal("expected error property")
	}

	errorProps, ok := errorProp["properties"].(map[string]any)
	if !ok {
		t.Fatal("expected error.properties")
	}

	// Check code and message properties
	if _, ok := errorProps["code"]; !ok {
		t.Error("expected error.properties.code")
	}
	if _, ok := errorProps["message"]; !ok {
		t.Error("expected error.properties.message")
	}
}

// Test 7: Docstrings are applied
func TestBuildOpenAPI_Docstrings(t *testing.T) {
	t.Run("operation_docs", func(t *testing.T) {
		cfg := &Config{
			OpenAPIEnabled: true,
			OpenAPITitle:   "Test API",
			OpenAPIVersion: "1.0.0",
		}

		manifest := &Manifest{
			Endpoints: []ManifestEndpoint{
				{
					Method:      "GET",
					Path:        "/pets",
					HandlerPkg:  "example.com/api",
					HandlerName: "ListPets",
					Shape:       "ctx_resp_err",
					RespType:    "[]example.com/api.Pet",
				},
			},
			Types: []ManifestType{
				{
					ID:     "[]example.com/api.Pet",
					GoType: "[]api.Pet",
					Kind:   "slice",
					Elem:   "example.com/api.Pet",
				},
				{
					ID:     "example.com/api.Pet",
					GoType: "api.Pet",
					Kind:   "struct",
					Doc:    "Pet represents a pet in the system.",
					Fields: []ManifestField{
						{GoName: "ID", JSONName: "id", TypeID: "string", Required: true, Doc: "ID is the unique identifier."},
						{GoName: "Name", JSONName: "name", TypeID: "string", Required: true, Doc: "Name is the pet's name."},
					},
				},
			},
			EndpointDocs: map[string]ManifestDoc{
				"example.com/api.ListPets": {
					Summary:     "List all pets",
					Description: "Returns a list of all pets in the system.\n\nSupports pagination.",
				},
			},
		}

		jsonBytes, err := BuildOpenAPI(cfg, manifest)
		if err != nil {
			t.Fatalf("BuildOpenAPI failed: %v", err)
		}

		var doc map[string]any
		if err := json.Unmarshal(jsonBytes, &doc); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}

		// Check operation docs
		paths := doc["paths"].(map[string]any)
		petsPath := paths["/pets"].(map[string]any)
		getOp := petsPath["get"].(map[string]any)

		if getOp["summary"] != "List all pets" {
			t.Errorf("expected summary='List all pets', got %v", getOp["summary"])
		}
		if !strings.Contains(getOp["description"].(string), "Returns a list") {
			t.Errorf("expected description to contain 'Returns a list', got %v", getOp["description"])
		}

		// Check type docs
		components := doc["components"].(map[string]any)
		schemas := components["schemas"].(map[string]any)

		// Find Pet schema (key might be sanitized)
		var petSchema map[string]any
		for k, v := range schemas {
			if strings.Contains(k, "Pet") && !strings.Contains(k, "Error") {
				petSchema = v.(map[string]any)
				break
			}
		}
		if petSchema == nil {
			t.Fatal("expected Pet schema")
		}

		if !strings.Contains(petSchema["description"].(string), "represents a pet") {
			t.Errorf("expected schema description to contain 'represents a pet', got %v", petSchema["description"])
		}

		// Check field docs
		props := petSchema["properties"].(map[string]any)
		idProp := props["id"].(map[string]any)
		if !strings.Contains(idProp["description"].(string), "unique identifier") {
			t.Errorf("expected field description to contain 'unique identifier', got %v", idProp["description"])
		}
	})
}

// Test 8: Deterministic output (byte-for-byte)
func TestBuildOpenAPI_Deterministic(t *testing.T) {
	cfg := &Config{
		OpenAPIEnabled: true,
		OpenAPITitle:   "Test API",
		OpenAPIVersion: "1.0.0",
	}

	manifest := &Manifest{
		Endpoints: []ManifestEndpoint{
			{
				Method:      "GET",
				Path:        "/pets",
				HandlerPkg:  "example.com/api",
				HandlerName: "ListPets",
				Shape:       "ctx_resp_err",
				RespType:    "[]example.com/api.Pet",
			},
			{
				Method:      "POST",
				Path:        "/pets",
				HandlerPkg:  "example.com/api",
				HandlerName: "CreatePet",
				Shape:       "ctx_req_resp_err",
				ReqType:     "example.com/api.CreatePetRequest",
				RespType:    "example.com/api.Pet",
				Bindings:    &BindingInfo{HasJSONBody: true},
			},
			{
				Method:      "GET",
				Path:        "/pets/{id}",
				HandlerPkg:  "example.com/api",
				HandlerName: "GetPet",
				Shape:       "ctx_req_resp_err",
				ReqType:     "example.com/api.GetPetRequest",
				RespType:    "example.com/api.Pet",
				Bindings: &BindingInfo{
					PathBindings: []FieldBinding{
						{FieldName: "ID", TagValue: "id", TypeKind: "string"},
					},
				},
			},
			{
				Method:      "DELETE",
				Path:        "/pets/{id}",
				HandlerPkg:  "example.com/api",
				HandlerName: "DeletePet",
				Shape:       "ctx_req_err",
				ReqType:     "example.com/api.DeletePetRequest",
				Bindings: &BindingInfo{
					PathBindings: []FieldBinding{
						{FieldName: "ID", TagValue: "id", TypeKind: "string"},
					},
				},
			},
		},
		Types: []ManifestType{
			{
				ID:     "[]example.com/api.Pet",
				GoType: "[]api.Pet",
				Kind:   "slice",
				Elem:   "example.com/api.Pet",
			},
			{
				ID:     "example.com/api.Pet",
				GoType: "api.Pet",
				Kind:   "struct",
				Fields: []ManifestField{
					{GoName: "ID", JSONName: "id", TypeID: "string", Required: true},
					{GoName: "Name", JSONName: "name", TypeID: "string", Required: true},
					{GoName: "Species", JSONName: "species", TypeID: "string", Required: false},
				},
			},
			{
				ID:     "example.com/api.CreatePetRequest",
				GoType: "api.CreatePetRequest",
				Kind:   "struct",
				Fields: []ManifestField{
					{GoName: "Name", JSONName: "name", TypeID: "string", Required: true},
					{GoName: "Species", JSONName: "species", TypeID: "string", Required: false},
				},
			},
		},
		EndpointDocs: map[string]ManifestDoc{},
	}

	// Build twice
	bytes1, err := BuildOpenAPI(cfg, manifest)
	if err != nil {
		t.Fatalf("first BuildOpenAPI failed: %v", err)
	}

	bytes2, err := BuildOpenAPI(cfg, manifest)
	if err != nil {
		t.Fatalf("second BuildOpenAPI failed: %v", err)
	}

	// Compare byte-for-byte
	if !reflect.DeepEqual(bytes1, bytes2) {
		t.Error("BuildOpenAPI output is not deterministic")
		t.Logf("First:\n%s", string(bytes1))
		t.Logf("Second:\n%s", string(bytes2))
	}
}

// Test: Config with servers
func TestBuildOpenAPI_Servers(t *testing.T) {
	cfg := &Config{
		OpenAPIEnabled: true,
		OpenAPITitle:   "Test API",
		OpenAPIVersion: "1.0.0",
		OpenAPIServers: []string{"http://localhost:8080", "https://api.example.com"},
	}

	manifest := &Manifest{
		Endpoints: []ManifestEndpoint{
			{
				Method:      "GET",
				Path:        "/health",
				HandlerPkg:  "example.com/api",
				HandlerName: "Health",
				Shape:       "ctx_err",
			},
		},
		Types:        []ManifestType{},
		EndpointDocs: map[string]ManifestDoc{},
	}

	jsonBytes, err := BuildOpenAPI(cfg, manifest)
	if err != nil {
		t.Fatalf("BuildOpenAPI failed: %v", err)
	}

	var doc map[string]any
	if err := json.Unmarshal(jsonBytes, &doc); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	servers, ok := doc["servers"].([]any)
	if !ok {
		t.Fatal("expected servers array")
	}

	if len(servers) != 2 {
		t.Fatalf("expected 2 servers, got %d", len(servers))
	}

	server1 := servers[0].(map[string]any)
	if server1["url"] != "http://localhost:8080" {
		t.Errorf("expected first server url='http://localhost:8080', got %v", server1["url"])
	}

	server2 := servers[1].(map[string]any)
	if server2["url"] != "https://api.example.com" {
		t.Errorf("expected second server url='https://api.example.com', got %v", server2["url"])
	}
}

// Test: Config with description
func TestBuildOpenAPI_Description(t *testing.T) {
	cfg := &Config{
		OpenAPIEnabled:     true,
		OpenAPITitle:       "Test API",
		OpenAPIVersion:     "1.0.0",
		OpenAPIDescription: "This is a test API for pets.",
	}

	manifest := &Manifest{
		Endpoints: []ManifestEndpoint{
			{
				Method:      "GET",
				Path:        "/health",
				HandlerPkg:  "example.com/api",
				HandlerName: "Health",
				Shape:       "ctx_err",
			},
		},
		Types:        []ManifestType{},
		EndpointDocs: map[string]ManifestDoc{},
	}

	jsonBytes, err := BuildOpenAPI(cfg, manifest)
	if err != nil {
		t.Fatalf("BuildOpenAPI failed: %v", err)
	}

	var doc map[string]any
	if err := json.Unmarshal(jsonBytes, &doc); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	info := doc["info"].(map[string]any)
	if info["description"] != "This is a test API for pets." {
		t.Errorf("expected info.description, got %v", info["description"])
	}
}

// Test: OperationId generation
func TestBuildOpenAPI_OperationId(t *testing.T) {
	cfg := &Config{
		OpenAPIEnabled: true,
		OpenAPITitle:   "Test API",
		OpenAPIVersion: "1.0.0",
	}

	manifest := &Manifest{
		Endpoints: []ManifestEndpoint{
			{
				Method:      "GET",
				Path:        "/pets/{id}",
				HandlerPkg:  "github.com/example/api",
				HandlerName: "GetPet",
				Shape:       "ctx_req_resp_err",
			},
		},
		Types:        []ManifestType{},
		EndpointDocs: map[string]ManifestDoc{},
	}

	jsonBytes, err := BuildOpenAPI(cfg, manifest)
	if err != nil {
		t.Fatalf("BuildOpenAPI failed: %v", err)
	}

	var doc map[string]any
	if err := json.Unmarshal(jsonBytes, &doc); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	paths := doc["paths"].(map[string]any)
	petPath := paths["/pets/{id}"].(map[string]any)
	getOp := petPath["get"].(map[string]any)

	opID := getOp["operationId"].(string)
	// operationId should be deterministic and sanitized
	if opID == "" {
		t.Error("expected non-empty operationId")
	}
	// Should not contain slashes or special chars
	if strings.Contains(opID, "/") {
		t.Errorf("operationId should not contain '/', got %q", opID)
	}
}

// Test: Tags generation from path
func TestBuildOpenAPI_Tags(t *testing.T) {
	cfg := &Config{
		OpenAPIEnabled: true,
		OpenAPITitle:   "Test API",
		OpenAPIVersion: "1.0.0",
	}

	manifest := &Manifest{
		Endpoints: []ManifestEndpoint{
			{
				Method:      "GET",
				Path:        "/pets/{id}",
				HandlerPkg:  "example.com/api",
				HandlerName: "GetPet",
				Shape:       "ctx_req_resp_err",
			},
			{
				Method:      "GET",
				Path:        "/users/{id}",
				HandlerPkg:  "example.com/api",
				HandlerName: "GetUser",
				Shape:       "ctx_req_resp_err",
			},
		},
		Types:        []ManifestType{},
		EndpointDocs: map[string]ManifestDoc{},
	}

	jsonBytes, err := BuildOpenAPI(cfg, manifest)
	if err != nil {
		t.Fatalf("BuildOpenAPI failed: %v", err)
	}

	var doc map[string]any
	if err := json.Unmarshal(jsonBytes, &doc); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	paths := doc["paths"].(map[string]any)

	// Check pets endpoint has "pets" tag
	petPath := paths["/pets/{id}"].(map[string]any)
	getOp := petPath["get"].(map[string]any)
	tags := getOp["tags"].([]any)
	if len(tags) == 0 || tags[0] != "pets" {
		t.Errorf("expected tag 'pets', got %v", tags)
	}

	// Check users endpoint has "users" tag
	userPath := paths["/users/{id}"].(map[string]any)
	getUserOp := userPath["get"].(map[string]any)
	userTags := getUserOp["tags"].([]any)
	if len(userTags) == 0 || userTags[0] != "users" {
		t.Errorf("expected tag 'users', got %v", userTags)
	}
}

// Test: 400 Bad Request for endpoints with bindings
func TestBuildOpenAPI_BadRequestResponse(t *testing.T) {
	cfg := &Config{
		OpenAPIEnabled: true,
		OpenAPITitle:   "Test API",
		OpenAPIVersion: "1.0.0",
	}

	manifest := &Manifest{
		Endpoints: []ManifestEndpoint{
			{
				Method:      "GET",
				Path:        "/pets/{id}",
				HandlerPkg:  "example.com/api",
				HandlerName: "GetPet",
				Shape:       "ctx_req_resp_err",
				ReqType:     "example.com/api.GetPetRequest",
				Bindings: &BindingInfo{
					PathBindings: []FieldBinding{
						{FieldName: "ID", TagValue: "id", TypeKind: "string"},
					},
				},
			},
		},
		Types:        []ManifestType{},
		EndpointDocs: map[string]ManifestDoc{},
	}

	jsonBytes, err := BuildOpenAPI(cfg, manifest)
	if err != nil {
		t.Fatalf("BuildOpenAPI failed: %v", err)
	}

	var doc map[string]any
	if err := json.Unmarshal(jsonBytes, &doc); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	paths := doc["paths"].(map[string]any)
	petPath := paths["/pets/{id}"].(map[string]any)
	getOp := petPath["get"].(map[string]any)
	responses := getOp["responses"].(map[string]any)

	// Check 400 response exists for endpoints with bindings
	resp400, ok := responses["400"].(map[string]any)
	if !ok {
		t.Fatal("expected 400 response for endpoint with bindings")
	}
	if resp400["description"] == "" {
		t.Error("expected description on 400 response")
	}
}
