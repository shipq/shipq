package files

import (
	"strings"
	"testing"
)

const testModulePath = "example.com/myapp"

// TestGenerateFileRegister_UsesColonParamSyntax verifies that the generated
// Register function uses :param syntax for all path parameters, not {param}.
// Regression: {param} syntax was silently ignored by extractPathParams, meaning
// path parameters were never bound and req.Id was always "".
func TestGenerateFileRegister_UsesColonParamSyntax(t *testing.T) {
	output := string(generateFileRegister(testModulePath))

	// Every route with a path parameter must use :param syntax
	if strings.Contains(output, `"/files/{id}`) {
		t.Error("Register function uses {id} syntax; must use :id instead")
	}
	if strings.Contains(output, `{account_id}`) {
		t.Error("Register function uses {account_id} syntax; must use :account_id instead")
	}

	// Verify the expected :param routes are present
	expectedRoutes := []string{
		`"/files/:id/download"`,
		`"/files/:id"`,
		`"/files/:id/visibility"`,
		`"/files/:id/access"`,
		`"/files/:id/access/:account_id"`,
	}
	for _, route := range expectedRoutes {
		if !strings.Contains(output, route) {
			t.Errorf("expected route %s not found in generated Register function", route)
		}
	}
}

// TestGenerateDownloadHandler_UsesPathTag verifies that DownloadRequest uses
// path:"id" tag for the Id field, not json:"id".
func TestGenerateDownloadHandler_UsesPathTag(t *testing.T) {
	output := string(generateDownloadHandler(testModulePath))

	if !strings.Contains(output, `path:"id"`) {
		t.Error("DownloadRequest.Id should use path:\"id\" tag")
	}
	// Must NOT have json:"id" on the Id field of the request struct
	// (response structs may legitimately use json:"id")
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Id ") && strings.Contains(line, "struct") {
			continue
		}
		if strings.Contains(line, "\tId ") && strings.Contains(line, `json:"id"`) {
			t.Error("DownloadRequest.Id should NOT use json:\"id\" tag for a path parameter")
		}
	}
}

// TestGenerateSoftDeleteHandler_UsesPathTag verifies that DeleteFileRequest
// uses path:"id" tag.
func TestGenerateSoftDeleteHandler_UsesPathTag(t *testing.T) {
	output := string(generateSoftDeleteHandler(testModulePath))

	if !strings.Contains(output, `path:"id"`) {
		t.Error("DeleteFileRequest.Id should use path:\"id\" tag")
	}
}

// TestGenerateVisibilityHandler_UsesPathTag verifies that UpdateVisibilityRequest
// uses path:"id" for the Id field and json:"visibility" for the body field.
func TestGenerateVisibilityHandler_UsesPathTag(t *testing.T) {
	output := string(generateVisibilityHandler(testModulePath))

	if !strings.Contains(output, `path:"id"`) {
		t.Error("UpdateVisibilityRequest.Id should use path:\"id\" tag")
	}
	if !strings.Contains(output, `json:"visibility"`) {
		t.Error("UpdateVisibilityRequest.Visibility should use json:\"visibility\" tag")
	}
}

// TestGenerateAccessHandlers_UsesPathTags verifies that all access-related
// request structs use path tags for path parameters.
func TestGenerateAccessHandlers_UsesPathTags(t *testing.T) {
	output := string(generateAccessHandlers(testModulePath))

	// GrantAccessRequest: Id is path param, AccountID and Role are body
	if !strings.Contains(output, "GrantAccessRequest") {
		t.Fatal("expected GrantAccessRequest in output")
	}

	// RevokeAccessRequest: both Id and AccountId are path params
	if !strings.Contains(output, "RevokeAccessRequest") {
		t.Fatal("expected RevokeAccessRequest in output")
	}

	// ListAccessRequest: Id is path param
	if !strings.Contains(output, "ListAccessRequest") {
		t.Fatal("expected ListAccessRequest in output")
	}

	// Count path:"id" occurrences — should appear in Grant, Revoke, and List
	pathIdCount := strings.Count(output, `path:"id"`)
	if pathIdCount < 3 {
		t.Errorf("expected at least 3 path:\"id\" tags (Grant, Revoke, List), got %d", pathIdCount)
	}

	// RevokeAccessRequest should have path:"account_id"
	if !strings.Contains(output, `path:"account_id"`) {
		t.Error("RevokeAccessRequest.AccountId should use path:\"account_id\" tag")
	}
}

// TestGenerateFileRegister_DownloadUsesOptionalAuth verifies that the Download
// route uses .OptionalAuth() so that sessions are parsed but not required.
// Regression: Download was registered with no auth, so SessionAccountIDFromContext
// always returned (0, false) and non-anonymous files failed with "unauthorized".
func TestGenerateFileRegister_DownloadUsesOptionalAuth(t *testing.T) {
	output := string(generateFileRegister(testModulePath))

	if !strings.Contains(output, `OptionalAuth()`) {
		t.Error("Download route should use .OptionalAuth()")
	}
	// Specifically verify it's on the download route line
	lines := strings.Split(output, "\n")
	found := false
	for _, line := range lines {
		if strings.Contains(line, "/download") && strings.Contains(line, "OptionalAuth") {
			found = true
			break
		}
	}
	if !found {
		t.Error("OptionalAuth() should be chained on the /files/:id/download route")
	}
}

// TestGenerateListHandler_UsesItemsKey verifies that the list response uses
// json:"items" (not json:"files") to match the standard CRUD pattern.
func TestGenerateListHandler_UsesItemsKey(t *testing.T) {
	output := string(generateListHandler(testModulePath))

	if !strings.Contains(output, `json:"items"`) {
		t.Error("ListFilesResponse should use json:\"items\" to match CRUD pattern")
	}
	if strings.Contains(output, `json:"files"`) {
		t.Error("ListFilesResponse should NOT use json:\"files\" — must match CRUD pattern")
	}
	if !strings.Contains(output, `json:"next_cursor,omitempty"`) {
		t.Error("ListFilesResponse should have NextCursor with json:\"next_cursor,omitempty\"")
	}
}
