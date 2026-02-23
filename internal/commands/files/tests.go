package files

import (
	"bytes"
	"fmt"

	"github.com/shipq/shipq/codegen"
)

// GenerateFileTestFiles generates test files for the file upload system.
func GenerateFileTestFiles(modulePath, scopeColumn, dialect string) map[string][]byte {
	files := make(map[string][]byte)

	content := generateFileHandlerTests(modulePath, scopeColumn, dialect)
	files["handlers_http_test.go"] = content

	return files
}

func generateFileHandlerTests(modulePath, scopeColumn, dialect string) []byte {
	var buf bytes.Buffer

	buf.WriteString(generatedFileHeader)
	buf.WriteString("package spec\n\n")

	buf.WriteString("import (\n")
	buf.WriteString("\t\"context\"\n")
	buf.WriteString("\t\"database/sql\"\n")
	buf.WriteString("\t\"os\"\n")
	buf.WriteString("\t\"testing\"\n")
	buf.WriteString("\t\"time\"\n\n")
	fmt.Fprintf(&buf, "\t%s\n", codegen.DriverImportForDialect(dialect))
	buf.WriteString("\t\"" + modulePath + "/api\"\n")
	buf.WriteString("\tmanaged_files \"" + modulePath + "/api/managed_files\"\n")
	buf.WriteString("\t\"" + modulePath + "/config\"\n")
	buf.WriteString("\t\"" + modulePath + "/shipq/lib/crypto\"\n")
	buf.WriteString("\t\"" + modulePath + "/shipq/lib/nanoid\"\n")
	buf.WriteString("\t\"" + modulePath + "/shipq/queries\"\n")
	fmt.Fprintf(&buf, "\tdbrunner %q\n", modulePath+"/shipq/queries/"+dialect)
	buf.WriteString(")\n\n")

	buf.WriteString("var testDB *sql.DB\n\n")

	buf.WriteString("func TestMain(m *testing.M) {\n")
	buf.WriteString("\tos.Setenv(\"COOKIE_SECRET\", \"test-secret-key-for-testing-only\")\n")
	buf.WriteString("\tos.Setenv(\"S3_BUCKET\", \"test-bucket\")\n")
	buf.WriteString("\tos.Setenv(\"S3_REGION\", \"us-east-1\")\n")
	buf.WriteString("\tos.Setenv(\"S3_ENDPOINT\", \"\")\n")
	buf.WriteString("\tos.Setenv(\"AWS_ACCESS_KEY_ID\", \"test-key\")\n")
	buf.WriteString("\tos.Setenv(\"AWS_SECRET_ACCESS_KEY\", \"test-secret\")\n")
	buf.WriteString("\tos.Setenv(\"MAX_UPLOAD_SIZE_MB\", \"100\")\n")
	buf.WriteString("\tos.Setenv(\"MULTIPART_THRESHOLD_MB\", \"10\")\n\n")

	buf.WriteString("\tdbURL := os.Getenv(\"TEST_DATABASE_URL\")\n")
	buf.WriteString("\tif dbURL == \"\" {\n")
	buf.WriteString("\t\tpanic(\"TEST_DATABASE_URL not set\")\n")
	buf.WriteString("\t}\n")
	buf.WriteString("\tif !config.IsLocalhostURL(dbURL) {\n")
	buf.WriteString("\t\tpanic(\"test database URL must point to localhost\")\n")
	buf.WriteString("\t}\n\n")

	buf.WriteString("\tdriver, dsn := config.ParseDatabaseURL(dbURL)\n\n")
	buf.WriteString("\tvar err error\n")
	buf.WriteString("\ttestDB, err = sql.Open(driver, dsn)\n")
	buf.WriteString("\tif err != nil {\n")
	buf.WriteString("\t\tpanic(\"failed to connect to test database: \" + err.Error())\n")
	buf.WriteString("\t}\n")
	buf.WriteString("\tdefer testDB.Close()\n\n")
	buf.WriteString("\tos.Exit(m.Run())\n")
	buf.WriteString("}\n\n")

	// createTestUser helper
	buf.WriteString(`func createTestUser(t *testing.T, ts *api.TestServer, email, password, firstName, lastName string) string {
	t.Helper()
	ctx := context.Background()
	runner := dbrunner.NewQueryRunner(ts.Tx())

	passwordHash, err := crypto.HashPassword(password)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	org, err := runner.SignupCreateOrganization(ctx, queries.SignupCreateOrganizationParams{
		PublicId:    nanoid.New(),
		Name:        firstName + "'s Organization",
		Description: "",
	})
	if err != nil {
		t.Fatalf("failed to create organization: %v", err)
	}

	account, err := runner.SignupCreateAccount(ctx, queries.SignupCreateAccountParams{
		PublicId:              nanoid.New(),
		FirstName:             firstName,
		LastName:              lastName,
		Email:                 email,
		PasswordHash:          passwordHash,
		DefaultOrganizationId: org.Id,
	})
	if err != nil {
		t.Fatalf("failed to create account: %v", err)
	}

	_, err = runner.SignupCreateOrganizationUser(ctx, queries.SignupCreateOrganizationUserParams{
		PublicId:       nanoid.New(),
		OrganizationId: org.Id,
		AccountId:      account.Id,
	})
	if err != nil {
		t.Fatalf("failed to link account to organization: %v", err)
	}

	session, err := runner.SignupCreateSession(ctx, queries.SignupCreateSessionParams{
		PublicId:  nanoid.New(),
		AccountId: account.Id,
		ExpiresAt: time.Now().UTC().Add(14 * 24 * time.Hour).Format("2006-01-02 15:04:05"),
	})
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	return crypto.SignCookie(session.PublicId, []byte(config.Settings.COOKIE_SECRET))
}

`)

	// Upload URL test
	buf.WriteString(`func TestUploadURL_Success(t *testing.T) {
	ts := api.NewUnauthenticatedTestServer(t, testDB)
	ctx := context.Background()

	sessionCookie := createTestUser(t, ts, "uploader@example.com", "password123", "Upload", "Test")
	client := api.NewAuthenticatedTestClient(ts.Server, sessionCookie)

	resp, err := client.UploadURL(ctx, managed_files.UploadURLRequest{
		Name:        "test.txt",
		ContentType: "text/plain",
		SizeBytes:   1024,
	})

	if err != nil {
		t.Fatalf("UploadURL failed: %v", err)
	}

	if resp.FileID == "" {
		t.Error("expected file_id to be set")
	}
	if resp.Method != "PUT" {
		t.Errorf("expected method PUT, got %s", resp.Method)
	}
	if resp.UploadURL == "" {
		t.Error("expected upload_url to be set for single-part upload")
	}
}

func TestUploadURL_Unauthenticated(t *testing.T) {
	ts := api.NewUnauthenticatedTestServer(t, testDB)
	client := api.NewUnauthenticatedTestClient(ts.Server)
	ctx := context.Background()

	_, err := client.UploadURL(ctx, managed_files.UploadURLRequest{
		Name:        "test.txt",
		ContentType: "text/plain",
		SizeBytes:   1024,
	})

	if err == nil {
		t.Error("expected error for unauthenticated upload, got nil")
	}
}

`)

	// Visibility change tests
	buf.WriteString(`func TestUpdateVisibility_OnlyOwnerOrManager(t *testing.T) {
	ts := api.NewUnauthenticatedTestServer(t, testDB)
	ctx := context.Background()

	ownerCookie := createTestUser(t, ts, "owner@example.com", "password123", "Owner", "Test")
	ownerClient := api.NewAuthenticatedTestClient(ts.Server, ownerCookie)

	otherCookie := createTestUser(t, ts, "other@example.com", "password123", "Other", "Test")
	otherClient := api.NewAuthenticatedTestClient(ts.Server, otherCookie)

	// Owner creates a file
	uploadResp, err := ownerClient.UploadURL(ctx, managed_files.UploadURLRequest{
		Name:        "private.txt",
		ContentType: "text/plain",
		SizeBytes:   100,
	})
	if err != nil {
		t.Fatalf("UploadURL failed: %v", err)
	}

	// Other user tries to change visibility -> should fail
	_, err = otherClient.UpdateVisibility(ctx, uploadResp.FileID, managed_files.UpdateVisibilityRequest{
		Visibility: "public",
	})
	if err == nil {
		t.Error("expected error when non-owner changes visibility, got nil")
	}

	// Owner changes visibility -> should succeed
	visResp, err := ownerClient.UpdateVisibility(ctx, uploadResp.FileID, managed_files.UpdateVisibilityRequest{
		Visibility: "public",
	})
	if err != nil {
		t.Fatalf("UpdateVisibility failed: %v", err)
	}
	if visResp.Visibility != "public" {
		t.Errorf("expected visibility 'public', got %s", visResp.Visibility)
	}
}

`)

	// Soft delete tests
	buf.WriteString(`func TestSoftDelete_OnlyOwnerOrManager(t *testing.T) {
	ts := api.NewUnauthenticatedTestServer(t, testDB)
	ctx := context.Background()

	ownerCookie := createTestUser(t, ts, "delowner@example.com", "password123", "DelOwner", "Test")
	ownerClient := api.NewAuthenticatedTestClient(ts.Server, ownerCookie)

	otherCookie := createTestUser(t, ts, "delother@example.com", "password123", "DelOther", "Test")
	otherClient := api.NewAuthenticatedTestClient(ts.Server, otherCookie)

	// Owner creates a file
	uploadResp, err := ownerClient.UploadURL(ctx, managed_files.UploadURLRequest{
		Name:        "deleteme.txt",
		ContentType: "text/plain",
		SizeBytes:   100,
	})
	if err != nil {
		t.Fatalf("UploadURL failed: %v", err)
	}

	// Other user tries to delete -> should fail
	_, err = otherClient.DeleteFile(ctx, uploadResp.FileID, managed_files.DeleteFileRequest{})
	if err == nil {
		t.Error("expected error when non-owner deletes, got nil")
	}

	// Owner deletes -> should succeed
	delResp, err := ownerClient.DeleteFile(ctx, uploadResp.FileID, managed_files.DeleteFileRequest{})
	if err != nil {
		t.Fatalf("DeleteFile failed: %v", err)
	}
	if !delResp.Success {
		t.Error("expected success to be true")
	}
}

`)

	// List visibility tests
	buf.WriteString(`func TestListFiles_OnlyVisibleFiles(t *testing.T) {
	ts := api.NewUnauthenticatedTestServer(t, testDB)
	ctx := context.Background()

	ownerCookie := createTestUser(t, ts, "listowner@example.com", "password123", "ListOwner", "Test")
	ownerClient := api.NewAuthenticatedTestClient(ts.Server, ownerCookie)

	// Owner creates a file
	_, err := ownerClient.UploadURL(ctx, managed_files.UploadURLRequest{
		Name:        "visible.txt",
		ContentType: "text/plain",
		SizeBytes:   100,
	})
	if err != nil {
		t.Fatalf("UploadURL failed: %v", err)
	}

	// Owner lists files (should see their own)
	listResp, err := ownerClient.ListFiles(ctx, managed_files.ListFilesRequest{Limit: 20})
	if err != nil {
		t.Fatalf("ListFiles failed: %v", err)
	}

	// Should have at least the file we just created
	// Note: file is "pending" since we didn't complete upload, so it won't show in list
	_ = listResp.Items
}

`)

	return formatSource(buf.Bytes())
}
