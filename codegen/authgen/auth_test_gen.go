package authgen

import (
	"bytes"
	"fmt"
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

// GenerateAuthHandlerTests generates api/auth_test/handlers_http_test.go
func GenerateAuthHandlerTests(cfg AuthGenConfig) ([]byte, error) {
	var buf bytes.Buffer

	buf.WriteString(generatedFileHeader)
	buf.WriteString("package auth_test\n\n")

	// Imports
	buf.WriteString("import (\n")
	buf.WriteString("\t\"context\"\n")
	buf.WriteString("\t\"database/sql\"\n")
	buf.WriteString("\t\"os\"\n")
	buf.WriteString("\t\"testing\"\n\n")
	buf.WriteString("\t_ \"github.com/go-sql-driver/mysql\"\n")
	buf.WriteString("\t_ \"github.com/lib/pq\"\n")
	buf.WriteString("\t_ \"github.com/mattn/go-sqlite3\"\n")
	buf.WriteString("\t\"" + cfg.ModulePath + "/api\"\n")
	buf.WriteString("\tauth \"" + cfg.ModulePath + "/api/auth\"\n")
	buf.WriteString(")\n\n")

	// Setup function
	buf.WriteString(`var testDB *sql.DB

func TestMain(m *testing.M) {
	// Set required env vars for tests
	os.Setenv("COOKIE_SECRET", "test-secret-key-for-testing-only")

	// Get database URL from environment
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = os.Getenv("TEST_DATABASE_URL")
	}
	if dbURL == "" {
		// Default to SQLite for testing
		dbURL = "file::memory:?cache=shared"
	}

	var err error
	testDB, err = sql.Open(getDriverName(dbURL), dbURL)
	if err != nil {
		panic("failed to connect to test database: " + err.Error())
	}
	defer testDB.Close()

	os.Exit(m.Run())
}

func getDriverName(url string) string {
	if len(url) >= 8 && url[:8] == "postgres" {
		return "postgres"
	}
	if len(url) >= 5 && url[:5] == "mysql" {
		return "mysql"
	}
	return "sqlite3"
}

`)

	// Signup tests
	buf.WriteString(`func TestSignup_Success(t *testing.T) {
	ts := api.NewUnauthenticatedTestServer(t, testDB)
	client := api.NewUnauthenticatedTestClient(ts.Server)
	ctx := context.Background()

	resp, cookies, err := client.SignupWithCookies(ctx, auth.SignupRequest{
		Email:     "newuser@example.com",
		Password:  "securepassword123",
		FirstName: "Test",
		LastName:  "User",
	})

	if err != nil {
		t.Fatalf("Signup failed: %v", err)
	}

	if resp.Email != "newuser@example.com" {
		t.Errorf("expected email newuser@example.com, got %s", resp.Email)
	}
	if resp.FirstName != "Test" {
		t.Errorf("expected first_name Test, got %s", resp.FirstName)
	}
	if resp.ID == "" {
		t.Error("expected non-empty ID")
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

func TestSignup_DuplicateEmail(t *testing.T) {
	ts := api.NewUnauthenticatedTestServer(t, testDB)
	client := api.NewUnauthenticatedTestClient(ts.Server)
	ctx := context.Background()

	// First signup
	_, _, err := client.SignupWithCookies(ctx, auth.SignupRequest{
		Email:     "duplicate@example.com",
		Password:  "password123",
		FirstName: "First",
		LastName:  "User",
	})
	if err != nil {
		t.Fatalf("First signup failed: %v", err)
	}

	// Second signup with same email should fail
	_, _, err = client.SignupWithCookies(ctx, auth.SignupRequest{
		Email:     "duplicate@example.com",
		Password:  "password456",
		FirstName: "Second",
		LastName:  "User",
	})
	if err == nil {
		t.Error("expected error for duplicate email, got nil")
	}
}

`)

	// Login tests
	buf.WriteString(`func TestLogin_Success(t *testing.T) {
	ts := api.NewUnauthenticatedTestServer(t, testDB)
	client := api.NewUnauthenticatedTestClient(ts.Server)
	ctx := context.Background()

	// First create an account
	_, _, _ = client.SignupWithCookies(ctx, auth.SignupRequest{
		Email:     "login@example.com",
		Password:  "mypassword",
		FirstName: "Login",
		LastName:  "Test",
	})

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

	// First create an account
	_, _, _ = client.SignupWithCookies(ctx, auth.SignupRequest{
		Email:     "wrongpass@example.com",
		Password:  "correctpassword",
		FirstName: "Wrong",
		LastName:  "Pass",
	})

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
	client := api.NewUnauthenticatedTestClient(ts.Server)
	ctx := context.Background()

	// Signup and get session cookie
	_, cookies, err := client.SignupWithCookies(ctx, auth.SignupRequest{
		Email:     "me@example.com",
		Password:  "password123",
		FirstName: "Me",
		LastName:  "Test",
	})
	if err != nil {
		t.Fatalf("Signup failed: %v", err)
	}

	// Get session cookie
	var sessionCookie string
	for _, c := range cookies {
		if c.Name == "session" {
			sessionCookie = c.Value
			break
		}
	}

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
	client := api.NewUnauthenticatedTestClient(ts.Server)
	ctx := context.Background()

	// Signup and get session cookie
	_, cookies, err := client.SignupWithCookies(ctx, auth.SignupRequest{
		Email:     "logout@example.com",
		Password:  "password123",
		FirstName: "Logout",
		LastName:  "Test",
	})
	if err != nil {
		t.Fatalf("Signup failed: %v", err)
	}

	// Get session cookie
	var sessionCookie string
	for _, c := range cookies {
		if c.Name == "session" {
			sessionCookie = c.Value
			break
		}
	}

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
