package cli

import (
	"strings"
	"testing"
)

func TestTestClientGeneratorFromManifest_Generate(t *testing.T) {
	tests := []struct {
		name           string
		gen            *TestClientGeneratorFromManifest
		wantErr        bool
		wantContain    []string
		wantNotContain []string
	}{
		{
			name: "basic endpoint with request and response",
			gen: &TestClientGeneratorFromManifest{
				PackageName: "api",
				Endpoints: []ManifestEndpoint{
					{
						Method:      "POST",
						Path:        "/pets",
						HandlerName: "CreatePet",
						Shape:       "ctx_req_resp_err",
						ReqType:     "github.com/example/api.CreatePetReq",
						RespType:    "github.com/example/api.CreatePetResp",
						Bindings: &BindingInfo{
							HasJSONBody: true,
						},
					},
				},
			},
			wantContain: []string{
				"package api",
				"func (c *Client) CreatePet(ctx context.Context, req CreatePetReq) (CreatePetResp, error)",
				"portapi.EncodeJSON(req)",
				"type Client struct",
				"func NewClient(baseURL string) *Client",
			},
		},
		{
			name: "endpoint with path variable",
			gen: &TestClientGeneratorFromManifest{
				PackageName: "api",
				Endpoints: []ManifestEndpoint{
					{
						Method:      "GET",
						Path:        "/pets/{id}",
						HandlerName: "GetPet",
						Shape:       "ctx_req_resp_err",
						ReqType:     "github.com/example/api.GetPetReq",
						RespType:    "github.com/example/api.GetPetResp",
						Bindings: &BindingInfo{
							PathBindings: []FieldBinding{
								{FieldName: "ID", TagValue: "id", TypeKind: "string"},
							},
						},
					},
				},
			},
			wantContain: []string{
				`"id": req.ID`,
				`portapi.InterpolatePath("/pets/{id}", pathVars)`,
			},
		},
		{
			name: "endpoint with query parameters",
			gen: &TestClientGeneratorFromManifest{
				PackageName: "api",
				Endpoints: []ManifestEndpoint{
					{
						Method:      "GET",
						Path:        "/pets/search",
						HandlerName: "SearchPets",
						Shape:       "ctx_req_resp_err",
						ReqType:     "github.com/example/api.SearchPetsReq",
						RespType:    "github.com/example/api.SearchPetsResp",
						Bindings: &BindingInfo{
							QueryBindings: []FieldBinding{
								{FieldName: "Limit", TagValue: "limit", TypeKind: "int"},
								{FieldName: "Tags", TagValue: "tag", TypeKind: "string", IsSlice: true, ElemKind: "string"},
							},
						},
					},
				},
			},
			wantContain: []string{
				`portapi.AddQuery(query, "limit"`,
				`portapi.FormatInt(req.Limit)`,
				`for _, v := range req.Tags`,
			},
		},
		{
			name: "endpoint with optional pointer field",
			gen: &TestClientGeneratorFromManifest{
				PackageName: "api",
				Endpoints: []ManifestEndpoint{
					{
						Method:      "GET",
						Path:        "/pets/search",
						HandlerName: "SearchPets",
						Shape:       "ctx_req_resp_err",
						ReqType:     "github.com/example/api.SearchPetsReq",
						RespType:    "github.com/example/api.SearchPetsResp",
						Bindings: &BindingInfo{
							QueryBindings: []FieldBinding{
								{FieldName: "Cursor", TagValue: "cursor", TypeKind: "string", IsPointer: true},
							},
						},
					},
				},
			},
			wantContain: []string{
				`if req.Cursor != nil`,
				`*req.Cursor`,
			},
		},
		{
			name: "endpoint with header binding",
			gen: &TestClientGeneratorFromManifest{
				PackageName: "api",
				Endpoints: []ManifestEndpoint{
					{
						Method:      "GET",
						Path:        "/pets/{id}",
						HandlerName: "GetPet",
						Shape:       "ctx_req_resp_err",
						ReqType:     "github.com/example/api.GetPetReq",
						RespType:    "github.com/example/api.GetPetResp",
						Bindings: &BindingInfo{
							HeaderBindings: []FieldBinding{
								{FieldName: "Auth", TagValue: "Authorization", TypeKind: "string"},
							},
						},
					},
				},
			},
			wantContain: []string{
				`headers.Set("Authorization", req.Auth)`,
			},
		},
		{
			name: "endpoint without request (ctx_resp_err)",
			gen: &TestClientGeneratorFromManifest{
				PackageName: "api",
				Endpoints: []ManifestEndpoint{
					{
						Method:      "GET",
						Path:        "/pets",
						HandlerName: "ListPets",
						Shape:       "ctx_resp_err",
						RespType:    "github.com/example/api.ListPetsResp",
					},
				},
			},
			wantContain: []string{
				"func (c *Client) ListPets(ctx context.Context) (ListPetsResp, error)",
			},
			wantNotContain: []string{
				"req ListPetsReq",
			},
		},
		{
			name: "endpoint without response (ctx_req_err)",
			gen: &TestClientGeneratorFromManifest{
				PackageName: "api",
				Endpoints: []ManifestEndpoint{
					{
						Method:      "DELETE",
						Path:        "/pets/{id}",
						HandlerName: "DeletePet",
						Shape:       "ctx_req_err",
						ReqType:     "github.com/example/api.DeletePetReq",
						Bindings: &BindingInfo{
							PathBindings: []FieldBinding{
								{FieldName: "ID", TagValue: "id", TypeKind: "string"},
							},
						},
					},
				},
			},
			wantContain: []string{
				"func (c *Client) DeletePet(ctx context.Context, req DeletePetReq) error",
			},
			wantNotContain: []string{
				"DeletePetResp",
			},
		},
		{
			name: "endpoint without request or response (ctx_err)",
			gen: &TestClientGeneratorFromManifest{
				PackageName: "api",
				Endpoints: []ManifestEndpoint{
					{
						Method:      "GET",
						Path:        "/health",
						HandlerName: "HealthCheck",
						Shape:       "ctx_err",
					},
				},
			},
			wantContain: []string{
				"func (c *Client) HealthCheck(ctx context.Context) error",
			},
		},
		{
			name: "missing package name",
			gen: &TestClientGeneratorFromManifest{
				PackageName: "",
				Endpoints:   []ManifestEndpoint{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := tt.gen.Generate()

			if tt.wantErr {
				if err == nil {
					t.Error("Generate() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Generate() error: %v", err)
			}

			outputStr := string(output)

			for _, want := range tt.wantContain {
				if !strings.Contains(outputStr, want) {
					t.Errorf("Output should contain %q, but doesn't.\n\nOutput:\n%s", want, outputStr)
				}
			}

			for _, notWant := range tt.wantNotContain {
				if strings.Contains(outputStr, notWant) {
					t.Errorf("Output should NOT contain %q, but does.\n\nOutput:\n%s", notWant, outputStr)
				}
			}
		})
	}
}

func TestTestClientGeneratorFromManifest_Determinism(t *testing.T) {
	gen := &TestClientGeneratorFromManifest{
		PackageName: "api",
		Endpoints: []ManifestEndpoint{
			{Method: "GET", Path: "/z", HandlerName: "Z", Shape: "ctx_err"},
			{Method: "GET", Path: "/a", HandlerName: "A", Shape: "ctx_err"},
			{Method: "POST", Path: "/m", HandlerName: "M", Shape: "ctx_err"},
			{Method: "DELETE", Path: "/b", HandlerName: "B", Shape: "ctx_err"},
		},
	}

	output1, err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate() first call error: %v", err)
	}

	output2, err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate() second call error: %v", err)
	}

	if string(output1) != string(output2) {
		t.Error("Generate() is not deterministic - outputs differ between calls")
	}
}

func TestTestClientGeneratorFromManifest_EndpointsSortedByMethodThenPath(t *testing.T) {
	gen := &TestClientGeneratorFromManifest{
		PackageName: "api",
		Endpoints: []ManifestEndpoint{
			{Method: "POST", Path: "/pets", HandlerName: "CreatePet", Shape: "ctx_err"},
			{Method: "GET", Path: "/pets", HandlerName: "ListPets", Shape: "ctx_err"},
			{Method: "DELETE", Path: "/pets/{id}", HandlerName: "DeletePet", Shape: "ctx_err"},
			{Method: "GET", Path: "/health", HandlerName: "Health", Shape: "ctx_err"},
		},
	}

	output, err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	outputStr := string(output)

	// Check that DELETE comes before GET, and GET comes before POST (alphabetically)
	deleteIdx := strings.Index(outputStr, "func (c *Client) DeletePet")
	healthIdx := strings.Index(outputStr, "func (c *Client) Health")
	listIdx := strings.Index(outputStr, "func (c *Client) ListPets")
	createIdx := strings.Index(outputStr, "func (c *Client) CreatePet")

	if deleteIdx == -1 || healthIdx == -1 || listIdx == -1 || createIdx == -1 {
		t.Fatal("Not all methods found in output")
	}

	// DELETE < GET < POST (method sort first)
	// Within GET: /health < /pets (path sort second)
	if !(deleteIdx < healthIdx && healthIdx < listIdx && listIdx < createIdx) {
		t.Errorf("Endpoints not sorted correctly by method then path.\nOrder found: DELETE=%d, Health=%d, ListPets=%d, CreatePet=%d",
			deleteIdx, healthIdx, listIdx, createIdx)
	}
}

func TestTestHarnessGeneratorFromManifest_Generate(t *testing.T) {
	tests := []struct {
		name        string
		gen         *TestHarnessGeneratorFromManifest
		wantErr     bool
		wantContain []string
	}{
		{
			name: "with NewMux",
			gen: &TestHarnessGeneratorFromManifest{
				PackageName: "api",
				HasNewMux:   true,
			},
			wantContain: []string{
				"package api",
				"func NewTestClient(ts *httptest.Server) *Client",
				"func NewTestServer(t *testing.T) *httptest.Server",
				"mux := NewMux()",
				"ts.Client()",
				"ts.URL",
				"t.Helper()",
				"t.Cleanup(ts.Close)",
			},
		},
		{
			name: "without NewMux",
			gen: &TestHarnessGeneratorFromManifest{
				PackageName: "api",
				HasNewMux:   false,
			},
			wantContain: []string{
				"package api",
				"func NewTestClient(ts *httptest.Server) *Client",
				"func NewTestServer(t *testing.T, handler http.Handler) *httptest.Server",
				"t.Helper()",
				"t.Cleanup(ts.Close)",
			},
		},
		{
			name: "missing package name",
			gen: &TestHarnessGeneratorFromManifest{
				PackageName: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := tt.gen.Generate()

			if tt.wantErr {
				if err == nil {
					t.Error("Generate() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Generate() error: %v", err)
			}

			outputStr := string(output)

			for _, want := range tt.wantContain {
				if !strings.Contains(outputStr, want) {
					t.Errorf("Output should contain %q, but doesn't.\n\nOutput:\n%s", want, outputStr)
				}
			}
		})
	}
}

func TestExtractTypeName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"github.com/example/api.CreatePetReq", "CreatePetReq"},
		{"CreatePetReq", "CreatePetReq"},
		{"*github.com/example/api.Pet", "*Pet"},
		{"[]github.com/example/api.Pet", "[]Pet"},
		{"*[]github.com/example/api.Pet", "*[]Pet"},
		{"string", "string"},
		{"int", "int"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := extractTypeName(tt.input)
			if got != tt.want {
				t.Errorf("extractTypeName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestExtractPathVars(t *testing.T) {
	tests := []struct {
		path string
		want []string
	}{
		{"/pets", nil},
		{"/pets/{id}", []string{"id"}},
		{"/users/{userId}/pets/{petId}", []string{"userId", "petId"}},
		{"/files/{path...}", []string{"path"}},
		{"/health", nil},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := extractPathVars(tt.path)
			if len(got) != len(tt.want) {
				t.Errorf("extractPathVars(%q) = %v, want %v", tt.path, got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("extractPathVars(%q)[%d] = %q, want %q", tt.path, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestFormatManifestValue(t *testing.T) {
	tests := []struct {
		varName  string
		typeKind string
		want     string
	}{
		{"v", "string", "v"},
		{"v", "bool", "portapi.FormatBool(v)"},
		{"v", "int", "portapi.FormatInt(v)"},
		{"v", "int64", "portapi.FormatInt64(v)"},
		{"v", "int8", "portapi.FormatInt64(int64(v))"},
		{"v", "uint", "portapi.FormatUint(v)"},
		{"v", "uint64", "portapi.FormatUint64(v)"},
		{"v", "float32", "portapi.FormatFloat32(v)"},
		{"v", "float64", "portapi.FormatFloat64(v)"},
		{"v", "time.Time", "portapi.FormatTime(v)"},
		{"v", "unknown", "fmt.Sprint(v)"},
	}

	for _, tt := range tests {
		t.Run(tt.typeKind, func(t *testing.T) {
			got := formatManifestValue(tt.varName, tt.typeKind)
			if got != tt.want {
				t.Errorf("formatManifestValue(%q, %q) = %q, want %q", tt.varName, tt.typeKind, got, tt.want)
			}
		})
	}
}
