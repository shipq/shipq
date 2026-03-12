package files

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Bug 3: No generated handler should concatenate err.Error() into httperror.New(500, ...)
// ---------------------------------------------------------------------------

// TestGeneratedHandlers_NoErrStringConcatenation scans all generated file
// handler output for the pattern `+err.Error()` within httperror.New calls
// and fails if any are found. This prevents leaking raw Go error strings
// (S3 SDK internals, DB driver details, etc.) to HTTP clients.
func TestGeneratedHandlers_NoErrStringConcatenation(t *testing.T) {
	files := GenerateFileHandlerFiles(testModulePath, "")

	for name, content := range files {
		code := string(content)
		if strings.Contains(code, `+err.Error()`) {
			t.Errorf("%s contains +err.Error() concatenation — raw errors must not be leaked to clients", name)
		}
		if strings.Contains(code, `+ err.Error()`) {
			t.Errorf("%s contains + err.Error() concatenation — raw errors must not be leaked to clients", name)
		}
	}
}

// TestGeneratedHandlers_500sUseWrapOrGenericMessage verifies that every 500
// status code in generated handler code uses httperror.Wrap (not httperror.New)
// with a generic "internal server error" message.
func TestGeneratedHandlers_500sUseGenericMessage(t *testing.T) {
	files := GenerateFileHandlerFiles(testModulePath, "")

	for name, content := range files {
		code := string(content)
		// Skip files that don't have 500 errors
		if !strings.Contains(code, "500") {
			continue
		}
		// Every httperror.Wrap(500, ...) should use "internal server error" or "access check failed"
		lines := strings.Split(code, "\n")
		for i, line := range lines {
			if strings.Contains(line, "httperror.New(500,") {
				// httperror.New(500, ...) should not exist in generated handlers
				// (except possibly in helpers.go for access check which already uses Wrap)
				t.Errorf("%s:%d uses httperror.New(500, ...) — should use httperror.Wrap(500, \"internal server error\", err) instead:\n  %s", name, i+1, strings.TrimSpace(line))
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Bug 4: CompleteRequest, GrantAccessRequest field tags and route patterns
// ---------------------------------------------------------------------------

// TestGenerateCompleteHandler_UsesPathTag verifies that CompleteRequest uses
// path:"id" for the Id field instead of json:"file_id".
func TestGenerateCompleteHandler_UsesPathTag(t *testing.T) {
	output := string(generateCompleteHandler(testModulePath))

	if !strings.Contains(output, `path:"id"`) {
		t.Error("CompleteRequest.Id should use path:\"id\" tag")
	}
	// The request struct should not have json:"file_id" (response struct may)
	lines := strings.Split(output, "\n")
	inRequest := false
	for _, line := range lines {
		if strings.Contains(line, "CompleteRequest struct") {
			inRequest = true
		}
		if inRequest && strings.TrimSpace(line) == "}" {
			inRequest = false
		}
		if inRequest && strings.Contains(line, `json:"file_id"`) {
			t.Error("CompleteRequest should NOT use json:\"file_id\" — file ID comes from path")
		}
	}
}

// TestGenerateCompleteHandler_NoUploadIDInRequest verifies that CompleteRequest
// does not contain an UploadID field — the upload ID should be derived
// server-side from the file record.
func TestGenerateCompleteHandler_NoUploadIDInRequest(t *testing.T) {
	output := string(generateCompleteHandler(testModulePath))

	// The request struct should not have upload_id
	if strings.Contains(output, `json:"upload_id`) {
		t.Error("CompleteRequest should NOT contain upload_id — it should be derived server-side from file.S3UploadId")
	}
}

// TestGenerateCompleteHandler_UsesServerSideUploadID verifies that the
// Complete handler uses file.S3UploadId instead of a client-supplied upload_id.
func TestGenerateCompleteHandler_UsesServerSideUploadID(t *testing.T) {
	output := string(generateCompleteHandler(testModulePath))

	if !strings.Contains(output, "*file.S3UploadId") {
		t.Error("Complete handler should use *file.S3UploadId for multipart completion, not client-supplied value")
	}
}

// TestGenerateFileRegister_CompleteRouteUsesPathParam verifies that the
// Complete route uses /files/:id/complete instead of /files/complete.
func TestGenerateFileRegister_CompleteRouteUsesPathParam(t *testing.T) {
	output := string(generateFileRegister(testModulePath))

	if strings.Contains(output, `"/files/complete"`) {
		t.Error("Complete route should be /files/:id/complete, not /files/complete")
	}
	if !strings.Contains(output, `"/files/:id/complete"`) {
		t.Error("Complete route should be /files/:id/complete")
	}
}

// TestGenerateAccessHandlers_GrantUsesPathForAccountId verifies that
// GrantAccessRequest.AccountId uses path:"account_id" tag, matching
// RevokeAccessRequest for consistency.
func TestGenerateAccessHandlers_GrantUsesPathForAccountId(t *testing.T) {
	output := string(generateAccessHandlers(testModulePath))

	// Count path:"account_id" occurrences — should appear in both Grant and Revoke
	pathAccountIdCount := strings.Count(output, `path:"account_id"`)
	if pathAccountIdCount < 2 {
		t.Errorf("expected at least 2 path:\"account_id\" tags (Grant and Revoke), got %d", pathAccountIdCount)
	}

	// GrantAccessRequest should NOT use json:"account_id" for the AccountId field.
	// (AccessListItem response struct legitimately uses json:"account_id".)
	lines := strings.Split(output, "\n")
	inGrantRequest := false
	for _, line := range lines {
		if strings.Contains(line, "GrantAccessRequest struct") {
			inGrantRequest = true
		}
		if inGrantRequest && strings.TrimSpace(line) == "}" {
			inGrantRequest = false
		}
		if inGrantRequest && strings.Contains(line, `json:"account_id"`) {
			t.Error("GrantAccessRequest.AccountId should use path:\"account_id\", not json:\"account_id\"")
		}
	}
}

// TestGenerateFileRegister_GrantRouteUsesAccountIdParam verifies that the
// GrantAccess route includes :account_id in the path, matching RevokeAccess.
func TestGenerateFileRegister_GrantRouteUsesAccountIdParam(t *testing.T) {
	output := string(generateFileRegister(testModulePath))

	// The POST grant access route should include :account_id
	lines := strings.Split(output, "\n")
	found := false
	for _, line := range lines {
		if strings.Contains(line, "Post") && strings.Contains(line, "/access/:account_id") {
			found = true
			break
		}
	}
	if !found {
		t.Error("GrantAccess route should be POST /files/:id/access/:account_id")
	}
}

// ---------------------------------------------------------------------------
// Bug 2: File access checks must distinguish sql.ErrNoRows from real DB errors
// ---------------------------------------------------------------------------

// TestGenerateFileHelpers_CheckDownloadAccess_DistinguishesSqlErrors verifies
// that the generated checkDownloadAccess function uses errors.Is(err, sql.ErrNoRows)
// to distinguish "no permission row" (403) from real database failures (500).
func TestGenerateFileHelpers_CheckDownloadAccess_DistinguishesSqlErrors(t *testing.T) {
	output := string(generateFileHelpers(testModulePath))

	// Must import database/sql and errors for error classification
	if !strings.Contains(output, `"database/sql"`) {
		t.Error("helpers must import database/sql for sql.ErrNoRows check")
	}
	if !strings.Contains(output, `"errors"`) {
		t.Error("helpers must import errors for errors.Is check")
	}

	// checkDownloadAccess must check for sql.ErrNoRows explicitly
	if !strings.Contains(output, "sql.ErrNoRows") {
		t.Error("checkDownloadAccess must check for sql.ErrNoRows to distinguish no-permission from DB failure")
	}

	// Must return a 500 for real database errors, not a 403
	if !strings.Contains(output, `httperror.Wrap(500, "access check failed"`) {
		t.Error("checkDownloadAccess must return 500 for real database errors, not mask them as 403")
	}
}

// TestGenerateSoftDeleteHandler_DistinguishesSqlErrors verifies that the
// soft-delete handler distinguishes sql.ErrNoRows from real DB errors when
// checking FilesCheckAccess.
func TestGenerateSoftDeleteHandler_DistinguishesSqlErrors(t *testing.T) {
	output := string(generateSoftDeleteHandler(testModulePath))

	if !strings.Contains(output, `"database/sql"`) {
		t.Error("soft-delete handler must import database/sql")
	}
	if !strings.Contains(output, `"errors"`) {
		t.Error("soft-delete handler must import errors")
	}
	if !strings.Contains(output, "sql.ErrNoRows") {
		t.Error("soft-delete handler must check sql.ErrNoRows in access check")
	}
	if !strings.Contains(output, `httperror.Wrap(500, "access check failed"`) {
		t.Error("soft-delete handler must return 500 for real DB errors in access check")
	}
}

// TestGenerateVisibilityHandler_DistinguishesSqlErrors verifies that the
// visibility handler distinguishes sql.ErrNoRows from real DB errors.
func TestGenerateVisibilityHandler_DistinguishesSqlErrors(t *testing.T) {
	output := string(generateVisibilityHandler(testModulePath))

	if !strings.Contains(output, `"database/sql"`) {
		t.Error("visibility handler must import database/sql")
	}
	if !strings.Contains(output, `"errors"`) {
		t.Error("visibility handler must import errors")
	}
	if !strings.Contains(output, "sql.ErrNoRows") {
		t.Error("visibility handler must check sql.ErrNoRows in access check")
	}
	if !strings.Contains(output, `httperror.Wrap(500, "access check failed"`) {
		t.Error("visibility handler must return 500 for real DB errors in access check")
	}
}

// TestGenerateAccessHandlers_DistinguishesSqlErrors verifies that the
// grant, revoke, and list access handlers all distinguish sql.ErrNoRows
// from real DB errors.
func TestGenerateAccessHandlers_DistinguishesSqlErrors(t *testing.T) {
	output := string(generateAccessHandlers(testModulePath))

	if !strings.Contains(output, `"database/sql"`) {
		t.Error("access handlers must import database/sql")
	}
	if !strings.Contains(output, `"errors"`) {
		t.Error("access handlers must import errors")
	}

	// There are 3 FilesCheckAccess call sites (grant, revoke, list) — each
	// must check sql.ErrNoRows independently.
	count := strings.Count(output, "sql.ErrNoRows")
	if count < 3 {
		t.Errorf("expected at least 3 sql.ErrNoRows checks (grant, revoke, list), got %d", count)
	}

	wrapCount := strings.Count(output, `httperror.Wrap(500, "access check failed"`)
	if wrapCount < 3 {
		t.Errorf("expected at least 3 httperror.Wrap(500, ...) calls (grant, revoke, list), got %d", wrapCount)
	}
}

const testModulePath = "example.com/myapp"

// TestGenerateFileHelpers_AccessCheckUsesWrap verifies that helpers.go uses
// httperror.Wrap for 500 errors, not httperror.New with string concatenation.
func TestGenerateFileHelpers_AccessCheckUsesWrap(t *testing.T) {
	output := string(generateFileHelpers(testModulePath))

	if strings.Contains(output, `+err.Error()`) || strings.Contains(output, `+ err.Error()`) {
		t.Error("helpers.go must not concatenate err.Error() into error messages")
	}
}

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
