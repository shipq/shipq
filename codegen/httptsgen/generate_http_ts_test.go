package httptsgen

import (
	"strings"
	"testing"

	"github.com/shipq/shipq/codegen"
)

// ─── Test helpers ───

func makePostsHandlers() []codegen.SerializedHandlerInfo {
	return []codegen.SerializedHandlerInfo{
		{
			Method:      "POST",
			Path:        "/posts",
			FuncName:    "CreatePost",
			PackagePath: "myapp/api/posts",
			RequireAuth: true,
			Request: &codegen.SerializedStructInfo{
				Name: "CreatePostRequest",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "Title", Type: "string", JSONName: "title", Required: true},
					{Name: "Body", Type: "string", JSONName: "body", Required: true},
				},
			},
			Response: &codegen.SerializedStructInfo{
				Name: "CreatePostResponse",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "ID", Type: "string", JSONName: "id", Required: true},
					{Name: "Title", Type: "string", JSONName: "title", Required: true},
					{Name: "Body", Type: "string", JSONName: "body", Required: true},
					{Name: "CreatedAt", Type: "string", JSONName: "created_at", Required: true},
				},
			},
		},
		{
			Method:      "GET",
			Path:        "/posts/:id",
			FuncName:    "GetPost",
			PackagePath: "myapp/api/posts",
			RequireAuth: false,
			PathParams:  []codegen.SerializedPathParam{{Name: "id", Position: 0}},
			Request: &codegen.SerializedStructInfo{
				Name: "GetPostRequest",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "ID", Type: "string", JSONName: "id", Required: true, Tags: map[string]string{"path": "id"}},
				},
			},
			Response: &codegen.SerializedStructInfo{
				Name: "GetPostResponse",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "ID", Type: "string", JSONName: "id", Required: true},
					{Name: "Title", Type: "string", JSONName: "title", Required: true},
					{Name: "Body", Type: "string", JSONName: "body", Required: true},
					{Name: "CreatedAt", Type: "string", JSONName: "created_at", Required: true},
				},
			},
		},
		{
			Method:      "GET",
			Path:        "/posts",
			FuncName:    "ListPosts",
			PackagePath: "myapp/api/posts",
			RequireAuth: false,
			Response: &codegen.SerializedStructInfo{
				Name: "ListPostsResponse",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "Items", Type: "[]Post", JSONName: "items", Required: true},
					{Name: "NextCursor", Type: "string", JSONName: "next_cursor", Required: false},
				},
			},
		},
		{
			Method:      "PATCH",
			Path:        "/posts/:id",
			FuncName:    "UpdatePost",
			PackagePath: "myapp/api/posts",
			RequireAuth: true,
			PathParams:  []codegen.SerializedPathParam{{Name: "id", Position: 0}},
			Request: &codegen.SerializedStructInfo{
				Name: "UpdatePostRequest",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "ID", Type: "string", JSONName: "id", Required: true, Tags: map[string]string{"path": "id"}},
					{Name: "Title", Type: "string", JSONName: "title", Required: false},
					{Name: "Body", Type: "string", JSONName: "body", Required: false},
				},
			},
			Response: &codegen.SerializedStructInfo{
				Name: "UpdatePostResponse",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "ID", Type: "string", JSONName: "id", Required: true},
					{Name: "Title", Type: "string", JSONName: "title", Required: true},
					{Name: "Body", Type: "string", JSONName: "body", Required: true},
				},
			},
		},
		{
			Method:      "DELETE",
			Path:        "/posts/:id",
			FuncName:    "SoftDeletePost",
			PackagePath: "myapp/api/posts",
			RequireAuth: true,
			PathParams:  []codegen.SerializedPathParam{{Name: "id", Position: 0}},
		},
	}
}

func makeAdminHandlers() []codegen.SerializedHandlerInfo {
	return []codegen.SerializedHandlerInfo{
		{
			Method:      "GET",
			Path:        "/admin/posts",
			FuncName:    "AdminListPosts",
			PackagePath: "myapp/api/posts",
			RequireAuth: true,
			Response: &codegen.SerializedStructInfo{
				Name: "AdminListPostsResponse",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "Items", Type: "[]Post", JSONName: "items", Required: true},
					{Name: "NextCursor", Type: "string", JSONName: "next_cursor", Required: false},
				},
			},
		},
		{
			Method:      "POST",
			Path:        "/admin/posts/:id/undelete",
			FuncName:    "UndeletePost",
			PackagePath: "myapp/api/posts",
			RequireAuth: true,
			PathParams:  []codegen.SerializedPathParam{{Name: "id", Position: 0}},
			Response: &codegen.SerializedStructInfo{
				Name: "UndeletePostResponse",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "ID", Type: "string", JSONName: "id", Required: true},
				},
			},
		},
	}
}

func makeCustomHandler() codegen.SerializedHandlerInfo {
	return codegen.SerializedHandlerInfo{
		Method:      "POST",
		Path:        "/posts/:id/publish",
		FuncName:    "PublishPost",
		PackagePath: "myapp/api/posts",
		RequireAuth: true,
		PathParams:  []codegen.SerializedPathParam{{Name: "id", Position: 0}},
		Request: &codegen.SerializedStructInfo{
			Name: "PublishPostRequest",
			Fields: []codegen.SerializedFieldInfo{
				{Name: "ID", Type: "string", JSONName: "id", Required: true, Tags: map[string]string{"path": "id"}},
			},
		},
		Response: &codegen.SerializedStructInfo{
			Name: "PublishPostResponse",
			Fields: []codegen.SerializedFieldInfo{
				{Name: "ID", Type: "string", JSONName: "id", Required: true},
				{Name: "PublishedAt", Type: "string", JSONName: "published_at", Required: true},
			},
		},
	}
}

func makeCustomGetHandler() codegen.SerializedHandlerInfo {
	return codegen.SerializedHandlerInfo{
		Method:      "GET",
		Path:        "/posts/:id/comments",
		FuncName:    "ListPostComments",
		PackagePath: "myapp/api/posts",
		RequireAuth: false,
		PathParams:  []codegen.SerializedPathParam{{Name: "id", Position: 0}},
		Response: &codegen.SerializedStructInfo{
			Name: "ListPostCommentsResponse",
			Fields: []codegen.SerializedFieldInfo{
				{Name: "Items", Type: "[]Comment", JSONName: "items", Required: true},
			},
		},
	}
}

// ─── DetectCRUDRole tests ───

func TestDetectCRUDRole_Create(t *testing.T) {
	h := makePostsHandlers()[0] // CreatePost
	role := DetectCRUDRole(h)
	if role != CRUDRoleCreate {
		t.Errorf("expected CRUDRoleCreate, got %d", role)
	}
}

func TestDetectCRUDRole_GetOne(t *testing.T) {
	h := makePostsHandlers()[1] // GetPost
	role := DetectCRUDRole(h)
	if role != CRUDRoleGetOne {
		t.Errorf("expected CRUDRoleGetOne, got %d", role)
	}
}

func TestDetectCRUDRole_List(t *testing.T) {
	h := makePostsHandlers()[2] // ListPosts
	role := DetectCRUDRole(h)
	if role != CRUDRoleList {
		t.Errorf("expected CRUDRoleList, got %d", role)
	}
}

func TestDetectCRUDRole_Update(t *testing.T) {
	h := makePostsHandlers()[3] // UpdatePost
	role := DetectCRUDRole(h)
	if role != CRUDRoleUpdate {
		t.Errorf("expected CRUDRoleUpdate, got %d", role)
	}
}

func TestDetectCRUDRole_Delete(t *testing.T) {
	h := makePostsHandlers()[4] // SoftDeletePost
	role := DetectCRUDRole(h)
	if role != CRUDRoleDelete {
		t.Errorf("expected CRUDRoleDelete, got %d", role)
	}
}

func TestDetectCRUDRole_AdminList(t *testing.T) {
	h := makeAdminHandlers()[0] // AdminListPosts
	role := DetectCRUDRole(h)
	if role != CRUDRoleAdminList {
		t.Errorf("expected CRUDRoleAdminList, got %d", role)
	}
}

func TestDetectCRUDRole_Undelete(t *testing.T) {
	h := makeAdminHandlers()[1] // UndeletePost
	role := DetectCRUDRole(h)
	if role != CRUDRoleUndelete {
		t.Errorf("expected CRUDRoleUndelete, got %d", role)
	}
}

func TestDetectCRUDRole_CustomMutation(t *testing.T) {
	h := makeCustomHandler() // PublishPost
	role := DetectCRUDRole(h)
	if role != CRUDRoleNone {
		t.Errorf("expected CRUDRoleNone, got %d", role)
	}
}

func TestDetectCRUDRole_CustomGet(t *testing.T) {
	h := makeCustomGetHandler() // ListPostComments
	role := DetectCRUDRole(h)
	if role != CRUDRoleNone {
		t.Errorf("expected CRUDRoleNone, got %d", role)
	}
}

// ─── GenerateHTTPTypeScriptClient tests ───

func TestGenerateHTTPTS_EmptyHandlers(t *testing.T) {
	result, err := GenerateHTTPTypeScriptClient(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(result), "No handlers defined") {
		t.Errorf("expected 'No handlers defined' message, got: %s", string(result))
	}
}

func TestGenerateHTTPTS_Header(t *testing.T) {
	result, err := GenerateHTTPTypeScriptClient(makePostsHandlers())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := string(result)
	if !strings.HasPrefix(output, "// Code generated by shipq. DO NOT EDIT.\n") {
		t.Error("output should start with generated file header")
	}
}

func TestGenerateHTTPTS_ConfigSection(t *testing.T) {
	result, err := GenerateHTTPTypeScriptClient(makePostsHandlers())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := string(result)

	mustContain := []string{
		"export interface ApiConfig {",
		"baseURL: string;",
		"getHeaders?:",
		"onUnauthorized?:",
		"export function configureApi(",
		"function getConfig(): ApiConfig {",
		"let _config: ApiConfig | null = null;",
	}
	for _, s := range mustContain {
		if !strings.Contains(output, s) {
			t.Errorf("config section should contain %q", s)
		}
	}
}

func TestGenerateHTTPTS_QueryHelper(t *testing.T) {
	result, err := GenerateHTTPTypeScriptClient(makePostsHandlers())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := string(result)

	if !strings.Contains(output, "function buildQuery(") {
		t.Error("should contain buildQuery function")
	}
	if !strings.Contains(output, "encodeURIComponent") {
		t.Error("buildQuery should use encodeURIComponent")
	}
}

func TestGenerateHTTPTS_FetchWrapper(t *testing.T) {
	result, err := GenerateHTTPTypeScriptClient(makePostsHandlers())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := string(result)

	mustContain := []string{
		"async function request<T>(",
		"JSON.stringify(body)",
		`credentials: "include"`,
		"res.status === 401",
		"res.status === 204",
	}
	for _, s := range mustContain {
		if !strings.Contains(output, s) {
			t.Errorf("fetch wrapper should contain %q", s)
		}
	}
}

func TestGenerateHTTPTS_ApiError(t *testing.T) {
	result, err := GenerateHTTPTypeScriptClient(makePostsHandlers())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := string(result)

	if !strings.Contains(output, "export class ApiError extends Error {") {
		t.Error("should contain ApiError class")
	}
	if !strings.Contains(output, "public status: number") {
		t.Error("ApiError should have status field")
	}
	if !strings.Contains(output, "public body: string") {
		t.Error("ApiError should have body field")
	}
}

func TestGenerateHTTPTS_CreatePostFunction(t *testing.T) {
	result, err := GenerateHTTPTypeScriptClient(makePostsHandlers())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := string(result)

	if !strings.Contains(output, "export interface CreatePostRequest {") {
		t.Error("should generate CreatePostRequest interface")
	}
	if !strings.Contains(output, "export interface CreatePostResponse {") {
		t.Error("should generate CreatePostResponse interface")
	}
	if !strings.Contains(output, "export async function createPost(req: CreatePostRequest): Promise<CreatePostResponse>") {
		t.Error("should generate createPost function with correct signature")
	}
	if !strings.Contains(output, "\"POST\", `/posts`") {
		t.Error("createPost should call request with POST /posts")
	}
}

func TestGenerateHTTPTS_GetPostFunction(t *testing.T) {
	result, err := GenerateHTTPTypeScriptClient(makePostsHandlers())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := string(result)

	if !strings.Contains(output, "export interface GetPostResponse {") {
		t.Error("should generate GetPostResponse interface")
	}
	if !strings.Contains(output, "export async function getPost(id: string): Promise<GetPostResponse>") {
		t.Error("should generate getPost function with path param")
	}
	if !strings.Contains(output, "${encodeURIComponent(id)}") {
		t.Error("getPost should encode the id path param")
	}
}

func TestGenerateHTTPTS_ListPostsFunction(t *testing.T) {
	result, err := GenerateHTTPTypeScriptClient(makePostsHandlers())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := string(result)

	if !strings.Contains(output, "export interface ListPostsResponse {") {
		t.Error("should generate ListPostsResponse interface")
	}
	if !strings.Contains(output, "export async function listPosts(params?: { cursor?: string; limit?: number }): Promise<ListPostsResponse>") {
		t.Error("should generate listPosts function with optional params")
	}
	if !strings.Contains(output, "buildQuery(params") {
		t.Error("listPosts should call buildQuery")
	}
}

func TestGenerateHTTPTS_UpdatePostFunction(t *testing.T) {
	result, err := GenerateHTTPTypeScriptClient(makePostsHandlers())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := string(result)

	if !strings.Contains(output, "export interface UpdatePostRequest {") {
		t.Error("should generate UpdatePostRequest interface")
	}
	// UpdatePostRequest should NOT include the 'id' field (it's a path param)
	lines := strings.Split(output, "\n")
	inUpdateReq := false
	for _, line := range lines {
		if strings.Contains(line, "export interface UpdatePostRequest {") {
			inUpdateReq = true
			continue
		}
		if inUpdateReq {
			if strings.Contains(line, "}") {
				break
			}
			if strings.Contains(line, "  id") {
				t.Error("UpdatePostRequest should not include id (path param)")
			}
		}
	}

	if !strings.Contains(output, "export async function updatePost(id: string, req: UpdatePostRequest): Promise<UpdatePostResponse>") {
		t.Error("should generate updatePost function with id param and request body")
	}
}

func TestGenerateHTTPTS_SoftDeleteFunction(t *testing.T) {
	result, err := GenerateHTTPTypeScriptClient(makePostsHandlers())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := string(result)

	if !strings.Contains(output, "export async function softDeletePost(id: string): Promise<void>") {
		t.Error("should generate softDeletePost returning void")
	}
}

func TestGenerateHTTPTS_AdminListFunction(t *testing.T) {
	result, err := GenerateHTTPTypeScriptClient(makeAdminHandlers())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := string(result)

	if !strings.Contains(output, "export async function adminListPosts(params?: { cursor?: string; limit?: number }): Promise<AdminListPostsResponse>") {
		t.Error("should generate adminListPosts with optional params")
	}
	if !strings.Contains(output, "buildQuery(params") {
		t.Error("adminListPosts should call buildQuery")
	}
}

func TestGenerateHTTPTS_UndeleteFunction(t *testing.T) {
	result, err := GenerateHTTPTypeScriptClient(makeAdminHandlers())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := string(result)

	if !strings.Contains(output, "export async function undeletePost(id: string): Promise<UndeletePostResponse>") {
		t.Error("should generate undeletePost function")
	}
}

func TestGenerateHTTPTS_CustomHandler(t *testing.T) {
	handlers := []codegen.SerializedHandlerInfo{makeCustomHandler()}
	result, err := GenerateHTTPTypeScriptClient(handlers)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := string(result)

	if !strings.Contains(output, "export interface PublishPostResponse {") {
		t.Error("should generate PublishPostResponse interface")
	}
	// The request only has the id field which is a path param, so no request type should be generated
	if strings.Contains(output, "export interface PublishPostRequest {") {
		t.Error("should not generate PublishPostRequest (only has path param fields)")
	}
	if !strings.Contains(output, "export async function publishPost(id: string): Promise<PublishPostResponse>") {
		t.Error("should generate publishPost function with path param only")
	}
}

func TestGenerateHTTPTS_CustomGetHandler(t *testing.T) {
	handlers := []codegen.SerializedHandlerInfo{makeCustomGetHandler()}
	result, err := GenerateHTTPTypeScriptClient(handlers)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := string(result)

	if !strings.Contains(output, "export async function listPostComments(id: string): Promise<ListPostCommentsResponse>") {
		t.Error("should generate listPostComments with path param")
	}
}

func TestGenerateHTTPTS_JSDocComments(t *testing.T) {
	result, err := GenerateHTTPTypeScriptClient(makePostsHandlers())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := string(result)

	if !strings.Contains(output, "/** POST /posts */") {
		t.Error("should have JSDoc for POST /posts")
	}
	if !strings.Contains(output, "/** GET /posts/:id */") {
		t.Error("should have JSDoc for GET /posts/:id")
	}
	if !strings.Contains(output, "/** GET /posts */") {
		t.Error("should have JSDoc for GET /posts")
	}
	if !strings.Contains(output, "/** PATCH /posts/:id */") {
		t.Error("should have JSDoc for PATCH /posts/:id")
	}
	if !strings.Contains(output, "/** DELETE /posts/:id */") {
		t.Error("should have JSDoc for DELETE /posts/:id")
	}
}

func TestGenerateHTTPTS_PackageSeparation(t *testing.T) {
	handlers := append(makePostsHandlers(), codegen.SerializedHandlerInfo{
		Method:      "GET",
		Path:        "/comments",
		FuncName:    "ListComments",
		PackagePath: "myapp/api/comments",
		Response: &codegen.SerializedStructInfo{
			Name: "ListCommentsResponse",
			Fields: []codegen.SerializedFieldInfo{
				{Name: "Items", Type: "[]Comment", JSONName: "items", Required: true},
			},
		},
	})

	result, err := GenerateHTTPTypeScriptClient(handlers)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := string(result)

	if !strings.Contains(output, "// ─── comments ───") {
		t.Error("should have comments package section")
	}
	if !strings.Contains(output, "// ─── posts ───") {
		t.Error("should have posts package section")
	}
}

func TestGenerateHTTPTS_TypeFieldTypes(t *testing.T) {
	handlers := []codegen.SerializedHandlerInfo{
		{
			Method:      "POST",
			Path:        "/items",
			FuncName:    "CreateItem",
			PackagePath: "myapp/api/items",
			Request: &codegen.SerializedStructInfo{
				Name: "CreateItemRequest",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "Name", Type: "string", JSONName: "name", Required: true},
					{Name: "Count", Type: "int", JSONName: "count", Required: true},
					{Name: "Active", Type: "bool", JSONName: "active", Required: true},
					{Name: "Tags", Type: "[]string", JSONName: "tags", Required: false},
					{Name: "Meta", Type: "map[string]any", JSONName: "meta", Required: false},
					{Name: "Score", Type: "float64", JSONName: "score", Required: false},
					{Name: "NullableField", Type: "*string", JSONName: "nullable_field", Required: false},
				},
			},
			Response: &codegen.SerializedStructInfo{
				Name: "CreateItemResponse",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "ID", Type: "string", JSONName: "id", Required: true},
				},
			},
		},
	}

	result, err := GenerateHTTPTypeScriptClient(handlers)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := string(result)

	if !strings.Contains(output, "  name: string;") {
		t.Error("should map string to string")
	}
	if !strings.Contains(output, "  count: number;") {
		t.Error("should map int to number")
	}
	if !strings.Contains(output, "  active: boolean;") {
		t.Error("should map bool to boolean")
	}
	if !strings.Contains(output, "  tags?: string[];") {
		t.Error("should map []string to string[] (optional)")
	}
	if !strings.Contains(output, "  meta?: Record<string, any>;") {
		t.Error("should map map[string]any to Record<string, any> (optional)")
	}
	if !strings.Contains(output, "  score?: number;") {
		t.Error("should map float64 to number (optional)")
	}
	if !strings.Contains(output, "  nullable_field?: string;") {
		t.Error("should map *string to string (optional)")
	}
}

func TestGenerateHTTPTS_OmittedFields(t *testing.T) {
	handlers := []codegen.SerializedHandlerInfo{
		{
			Method:      "POST",
			Path:        "/things",
			FuncName:    "CreateThing",
			PackagePath: "myapp/api/things",
			Request: &codegen.SerializedStructInfo{
				Name: "CreateThingRequest",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "Name", Type: "string", JSONName: "name", Required: true},
					{Name: "Internal", Type: "string", JSONName: "", JSONOmit: true},
				},
			},
			Response: &codegen.SerializedStructInfo{
				Name: "CreateThingResponse",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "ID", Type: "string", JSONName: "id", Required: true},
				},
			},
		},
	}

	result, err := GenerateHTTPTypeScriptClient(handlers)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := string(result)

	if strings.Contains(output, "Internal") {
		t.Error("should skip fields with json:\"-\" (JSONOmit=true, JSONName empty)")
	}
}

func TestGenerateHTTPTS_CamelCaseFunctionNames(t *testing.T) {
	result, err := GenerateHTTPTypeScriptClient(makePostsHandlers())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := string(result)

	camelFuncs := []string{
		"createPost(",
		"getPost(",
		"listPosts(",
		"updatePost(",
		"softDeletePost(",
	}
	for _, f := range camelFuncs {
		if !strings.Contains(output, f) {
			t.Errorf("should contain camelCase function %q", f)
		}
	}

	// PascalCase function names should NOT appear as function declarations
	pascalFuncs := []string{
		"function CreatePost(",
		"function GetPost(",
		"function ListPosts(",
		"function UpdatePost(",
		"function SoftDeletePost(",
	}
	for _, f := range pascalFuncs {
		if strings.Contains(output, f) {
			t.Errorf("should NOT contain PascalCase function declaration %q", f)
		}
	}
}

func TestGenerateHTTPTS_NoRequestTypeForGET(t *testing.T) {
	handlers := []codegen.SerializedHandlerInfo{
		{
			Method:      "GET",
			Path:        "/posts",
			FuncName:    "ListPosts",
			PackagePath: "myapp/api/posts",
			Request: &codegen.SerializedStructInfo{
				Name: "ListPostsRequest",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "Cursor", Type: "string", JSONName: "cursor", Required: false},
				},
			},
			Response: &codegen.SerializedStructInfo{
				Name: "ListPostsResponse",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "Items", Type: "[]Post", JSONName: "items", Required: true},
				},
			},
		},
	}

	result, err := GenerateHTTPTypeScriptClient(handlers)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := string(result)

	// GET methods don't have a body, so no request interface should be generated
	if strings.Contains(output, "export interface ListPostsRequest {") {
		t.Error("should not generate request interface for GET handlers")
	}
}

func TestGenerateHTTPTS_PathParamEncoding(t *testing.T) {
	handlers := []codegen.SerializedHandlerInfo{
		{
			Method:      "GET",
			Path:        "/orgs/:org_id/posts/:id",
			FuncName:    "GetOrgPost",
			PackagePath: "myapp/api/posts",
			PathParams: []codegen.SerializedPathParam{
				{Name: "org_id", Position: 0},
				{Name: "id", Position: 1},
			},
			Response: &codegen.SerializedStructInfo{
				Name: "GetOrgPostResponse",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "ID", Type: "string", JSONName: "id", Required: true},
				},
			},
		},
	}

	result, err := GenerateHTTPTypeScriptClient(handlers)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := string(result)

	if !strings.Contains(output, "org_id: string, id: string") {
		t.Error("should have both path params as function arguments")
	}
	if !strings.Contains(output, "${encodeURIComponent(org_id)}") {
		t.Error("should encode org_id path param")
	}
	if !strings.Contains(output, "${encodeURIComponent(id)}") {
		t.Error("should encode id path param")
	}
}

// ─── filterBodyFields tests ───

func TestFilterBodyFields_ExcludesPathParams(t *testing.T) {
	h := codegen.SerializedHandlerInfo{
		Method:     "PATCH",
		Path:       "/posts/:id",
		PathParams: []codegen.SerializedPathParam{{Name: "id", Position: 0}},
		Request: &codegen.SerializedStructInfo{
			Name: "UpdatePostRequest",
			Fields: []codegen.SerializedFieldInfo{
				{Name: "ID", Type: "string", JSONName: "id", Required: true, Tags: map[string]string{"path": "id"}},
				{Name: "Title", Type: "string", JSONName: "title", Required: false},
				{Name: "Body", Type: "string", JSONName: "body", Required: false},
			},
		},
	}

	bodyFields := filterBodyFields(h)
	if len(bodyFields) != 2 {
		t.Fatalf("expected 2 body fields, got %d", len(bodyFields))
	}
	for _, f := range bodyFields {
		if f.Name == "ID" {
			t.Error("body fields should not include the ID path param")
		}
	}
}

func TestFilterBodyFields_NilRequest(t *testing.T) {
	h := codegen.SerializedHandlerInfo{
		Method: "DELETE",
		Path:   "/posts/:id",
	}
	bodyFields := filterBodyFields(h)
	if bodyFields != nil {
		t.Errorf("expected nil body fields for nil request, got %v", bodyFields)
	}
}

func TestFilterBodyFields_NoPathParams(t *testing.T) {
	h := codegen.SerializedHandlerInfo{
		Method: "POST",
		Path:   "/posts",
		Request: &codegen.SerializedStructInfo{
			Fields: []codegen.SerializedFieldInfo{
				{Name: "Title", Type: "string", JSONName: "title", Required: true},
				{Name: "Body", Type: "string", JSONName: "body", Required: true},
			},
		},
	}
	bodyFields := filterBodyFields(h)
	if len(bodyFields) != 2 {
		t.Errorf("expected 2 body fields, got %d", len(bodyFields))
	}
}

// ─── buildPathExpression tests ───

func TestBuildPathExpression_NoParams(t *testing.T) {
	result := buildPathExpression("/posts", nil)
	if result != "/posts" {
		t.Errorf("expected /posts, got %q", result)
	}
}

func TestBuildPathExpression_SingleParam(t *testing.T) {
	result := buildPathExpression("/posts/:id", []codegen.SerializedPathParam{{Name: "id", Position: 0}})
	if result != "/posts/${encodeURIComponent(id)}" {
		t.Errorf("expected /posts/${encodeURIComponent(id)}, got %q", result)
	}
}

func TestBuildPathExpression_MultipleParams(t *testing.T) {
	result := buildPathExpression("/orgs/:org_id/posts/:id", []codegen.SerializedPathParam{
		{Name: "org_id", Position: 0},
		{Name: "id", Position: 1},
	})
	expected := "/orgs/${encodeURIComponent(org_id)}/posts/${encodeURIComponent(id)}"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

// ─── groupHandlersByPackage tests ───

func TestGroupHandlersByPackage(t *testing.T) {
	handlers := []codegen.SerializedHandlerInfo{
		{FuncName: "A", PackagePath: "myapp/api/posts"},
		{FuncName: "B", PackagePath: "myapp/api/posts"},
		{FuncName: "C", PackagePath: "myapp/api/comments"},
	}
	groups := groupHandlersByPackage(handlers)
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}
	if len(groups["posts"]) != 2 {
		t.Errorf("expected 2 posts handlers, got %d", len(groups["posts"]))
	}
	if len(groups["comments"]) != 1 {
		t.Errorf("expected 1 comments handler, got %d", len(groups["comments"]))
	}
}

func TestGroupHandlersByPackage_EmptyPackagePath(t *testing.T) {
	handlers := []codegen.SerializedHandlerInfo{
		{FuncName: "A", PackagePath: ""},
	}
	groups := groupHandlersByPackage(handlers)
	if len(groups["api"]) != 1 {
		t.Errorf("expected 1 handler in 'api' group for empty package path, got %d", len(groups["api"]))
	}
}

// ─── Nested struct type test ───

func TestGenerateHTTPTS_NestedStructField(t *testing.T) {
	handlers := []codegen.SerializedHandlerInfo{
		{
			Method:      "POST",
			Path:        "/orders",
			FuncName:    "CreateOrder",
			PackagePath: "myapp/api/orders",
			Request: &codegen.SerializedStructInfo{
				Name: "CreateOrderRequest",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "Name", Type: "string", JSONName: "name", Required: true},
					{
						Name:     "Address",
						Type:     "Address",
						JSONName: "address",
						Required: true,
						StructFields: &codegen.SerializedStructInfo{
							Name: "Address",
							Fields: []codegen.SerializedFieldInfo{
								{Name: "Street", Type: "string", JSONName: "street", Required: true},
								{Name: "City", Type: "string", JSONName: "city", Required: true},
							},
						},
					},
				},
			},
			Response: &codegen.SerializedStructInfo{
				Name: "CreateOrderResponse",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "ID", Type: "string", JSONName: "id", Required: true},
				},
			},
		},
	}

	result, err := GenerateHTTPTypeScriptClient(handlers)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := string(result)

	if !strings.Contains(output, "address: { street: string; city: string; }") {
		t.Errorf("should generate inline interface for nested struct, got:\n%s", output)
	}
}

// ─── DetectCRUDRole edge cases ───

func TestDetectCRUDRole_MismatchedFuncName(t *testing.T) {
	// A handler on the POST /posts route but with a non-standard func name
	h := codegen.SerializedHandlerInfo{
		Method:      "POST",
		Path:        "/posts",
		FuncName:    "ImportPosts",
		PackagePath: "myapp/api/posts",
	}
	role := DetectCRUDRole(h)
	if role != CRUDRoleNone {
		t.Errorf("expected CRUDRoleNone for mismatched func name, got %d", role)
	}
}

func TestDetectCRUDRole_EmptyPath(t *testing.T) {
	h := codegen.SerializedHandlerInfo{
		Method:   "GET",
		Path:     "/",
		FuncName: "Root",
	}
	role := DetectCRUDRole(h)
	if role != CRUDRoleNone {
		t.Errorf("expected CRUDRoleNone for root path, got %d", role)
	}
}

// ─── singularPascalFromTable tests ───

func TestSingularPascalFromTable(t *testing.T) {
	tests := []struct {
		table    string
		expected string
	}{
		{"posts", "Post"},
		{"comments", "Comment"},
		{"categories", "Categorie"},
		{"user", "User"},
		{"a", "A"},
	}
	for _, tt := range tests {
		t.Run(tt.table, func(t *testing.T) {
			result := singularPascalFromTable(tt.table)
			if result != tt.expected {
				t.Errorf("singularPascalFromTable(%q) = %q, want %q", tt.table, result, tt.expected)
			}
		})
	}
}
