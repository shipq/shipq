package authgen

import (
	"bytes"
	"fmt"

	"github.com/shipq/shipq/codegen"
)

// GenerateAuthTestFiles generates auth test files.
// Returns a map of filenames to their contents.
func GenerateAuthTestFiles(cfg AuthGenConfig) (map[string][]byte, error) {
	files := make(map[string][]byte)

	content, err := GenerateAuthHandlerTests(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to generate auth tests: %w", err)
	}
	files["handlers_http_test.go"] = content

	return files, nil
}

// GenerateAuthHandlerTests generates api/auth/spec/handlers_http_test.go
func GenerateAuthHandlerTests(cfg AuthGenConfig) ([]byte, error) {
	var buf bytes.Buffer

	buf.WriteString(generatedFileHeader)
	buf.WriteString("package spec\n\n")

	// Imports
	buf.WriteString("import (\n")
	buf.WriteString("\t\"context\"\n")
	buf.WriteString("\t\"database/sql\"\n")
	buf.WriteString("\t\"os\"\n")
	buf.WriteString("\t\"testing\"\n")
	buf.WriteString("\t\"time\"\n\n")
	fmt.Fprintf(&buf, "\t%s\n", codegen.DriverImportForDialect(cfg.Dialect))
	buf.WriteString("\t\"" + cfg.ModulePath + "/api\"\n")
	buf.WriteString("\tauth \"" + cfg.ModulePath + "/api/auth\"\n")
	buf.WriteString("\t\"" + cfg.ModulePath + "/config\"\n")
	buf.WriteString("\t\"" + cfg.ModulePath + "/shipq/lib/crypto\"\n")
	buf.WriteString("\t\"" + cfg.ModulePath + "/shipq/lib/nanoid\"\n")
	buf.WriteString("\t\"" + cfg.ModulePath + "/shipq/queries\"\n")
	fmt.Fprintf(&buf, "\tdbrunner %q\n", cfg.ModulePath+"/shipq/queries/"+cfg.Dialect)
	buf.WriteString(")\n\n")

	// Setup function — embed the test database URL as a compile-time fallback
	buf.WriteString("var testDB *sql.DB\n\n")
	buf.WriteString("func TestMain(m *testing.M) {\n")
	buf.WriteString("\t// Set required env vars for tests\n")
	buf.WriteString("\tos.Setenv(\"COOKIE_SECRET\", \"test-secret-key-for-testing-only\")\n\n")
	buf.WriteString("\t// Get test database URL: env override, then compile-time constant from shipq.ini\n")
	buf.WriteString("\tdbURL := os.Getenv(\"TEST_DATABASE_URL\")\n")
	buf.WriteString("\tif dbURL == \"\" {\n")
	fmt.Fprintf(&buf, "\t\tdbURL = %q\n", cfg.TestDatabaseURL)
	buf.WriteString("\t}\n")
	buf.WriteString("\tif dbURL == \"\" {\n")
	buf.WriteString("\t\tpanic(\"TEST_DATABASE_URL not set and no fallback configured\")\n")
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

	// createTestUser helper — inserts a user directly via the query runner
	// so that auth tests do not depend on the /signup endpoint.
	buf.WriteString(`// createTestUser inserts a test account with an organization and session
// directly via the query runner, returning a signed session cookie value.
// This avoids depending on the /signup endpoint which is generated separately.
func createTestUser(t *testing.T, ts *api.TestServer, email, password, firstName, lastName string) string {
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

	// Login tests
	buf.WriteString(`func TestLogin_Success(t *testing.T) {
	ts := api.NewUnauthenticatedTestServer(t, testDB)
	client := api.NewUnauthenticatedTestClient(ts.Server)
	ctx := context.Background()

	// Create an account directly via the query runner
	createTestUser(t, ts, "login@example.com", "mypassword", "Login", "Test")

	// Now try to login
	resp, cookies, err := client.LoginWithCookies(ctx, auth.LoginRequest{
		Email:    "login@example.com",
		Password: "mypassword",
	})

	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	if resp.Email != "login@example.com" {
		t.Errorf("expected email login@example.com, got %s", resp.Email)
	}

	// Should have session cookie
	var sessionCookie string
	for _, c := range cookies {
		if c.Name == "session" {
			sessionCookie = c.Value
			break
		}
	}
	if sessionCookie == "" {
		t.Error("expected session cookie to be set")
	}
}

func TestLogin_InvalidPassword(t *testing.T) {
	ts := api.NewUnauthenticatedTestServer(t, testDB)
	client := api.NewUnauthenticatedTestClient(ts.Server)
	ctx := context.Background()

	// Create an account directly via the query runner
	createTestUser(t, ts, "wrongpass@example.com", "correctpassword", "Wrong", "Pass")

	// Try to login with wrong password
	_, _, err := client.LoginWithCookies(ctx, auth.LoginRequest{
		Email:    "wrongpass@example.com",
		Password: "wrongpassword",
	})

	if err == nil {
		t.Error("expected error for wrong password, got nil")
	}
}

func TestLogin_NonexistentEmail(t *testing.T) {
	ts := api.NewUnauthenticatedTestServer(t, testDB)
	client := api.NewUnauthenticatedTestClient(ts.Server)
	ctx := context.Background()

	_, _, err := client.LoginWithCookies(ctx, auth.LoginRequest{
		Email:    "nonexistent@example.com",
		Password: "anypassword",
	})

	if err == nil {
		t.Error("expected error for nonexistent email, got nil")
	}
}

`)

	// Me tests
	buf.WriteString(`func TestMe_Authenticated(t *testing.T) {
	ts := api.NewUnauthenticatedTestServer(t, testDB)
	ctx := context.Background()

	// Create user and get signed session cookie
	sessionCookie := createTestUser(t, ts, "me@example.com", "password123", "Me", "Test")

	// Call /me with session cookie
	authClient := api.NewAuthenticatedTestClient(ts.Server, sessionCookie)
	resp, err := authClient.Me(ctx, auth.MeRequest{})

	if err != nil {
		t.Fatalf("Me failed: %v", err)
	}

	if resp.Email != "me@example.com" {
		t.Errorf("expected email me@example.com, got %s", resp.Email)
	}
	if resp.FirstName != "Me" {
		t.Errorf("expected first_name Me, got %s", resp.FirstName)
	}
}

func TestMe_Unauthenticated(t *testing.T) {
	ts := api.NewUnauthenticatedTestServer(t, testDB)
	client := api.NewUnauthenticatedTestClient(ts.Server)
	ctx := context.Background()

	_, err := client.Me(ctx, auth.MeRequest{})

	if err == nil {
		t.Error("expected error for unauthenticated request, got nil")
	}
}

`)

	// Logout tests
	buf.WriteString(`func TestLogout_Success(t *testing.T) {
	ts := api.NewUnauthenticatedTestServer(t, testDB)
	ctx := context.Background()

	// Create user and get signed session cookie
	sessionCookie := createTestUser(t, ts, "logout@example.com", "password123", "Logout", "Test")

	// Logout
	authClient := api.NewAuthenticatedTestClient(ts.Server, sessionCookie)
	resp, logoutCookies, err := authClient.LogoutWithCookies(ctx, auth.LogoutRequest{})

	if err != nil {
		t.Fatalf("Logout failed: %v", err)
	}

	if !resp.Success {
		t.Error("expected success to be true")
	}

	// Session cookie should be cleared (MaxAge < 0 or expired)
	var clearedCookie bool
	for _, c := range logoutCookies {
		if c.Name == "session" && c.MaxAge < 0 {
			clearedCookie = true
			break
		}
	}
	if !clearedCookie {
		t.Error("expected session cookie to be cleared")
	}
}

func TestLogout_Unauthenticated(t *testing.T) {
	ts := api.NewUnauthenticatedTestServer(t, testDB)
	client := api.NewUnauthenticatedTestClient(ts.Server)
	ctx := context.Background()

	_, logoutCookies, err := client.LogoutWithCookies(ctx, auth.LogoutRequest{})

	if err == nil {
		t.Error("expected error for unauthenticated logout, got nil")
	}
	_ = logoutCookies // unused in error case
}
`)

	return formatSource(buf.Bytes())
}
