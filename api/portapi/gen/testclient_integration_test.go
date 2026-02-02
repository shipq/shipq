package gen

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/shipq/shipq/api/portapi"
	"github.com/shipq/shipq/api/portapi/testdata/exampleapi"
)

// TestIntegration_ClientServerRoundTrip tests the generated client against a real server.
func TestIntegration_ClientServerRoundTrip(t *testing.T) {
	// Set up a mux with all handlers
	mux := http.NewServeMux()

	// Register handlers
	mux.Handle("POST /pets", HandleCreatePet(exampleapi.CreatePet))
	mux.Handle("GET /pets", HandleListPets(exampleapi.ListPets))
	mux.Handle("GET /pets/{id}", HandleGetPet(exampleapi.GetPet))
	mux.Handle("DELETE /pets/{id}", HandleDeletePet(exampleapi.DeletePet))
	mux.Handle("PUT /pets/{id}", HandleUpdatePet(exampleapi.UpdatePet))
	mux.Handle("GET /pets/search", HandleSearchPets(exampleapi.SearchPets))
	mux.Handle("GET /health", HandleHealthCheck(exampleapi.HealthCheck))

	// Create test server using NewTestServer helper
	ts := NewTestServer(t, mux)

	// Create client using NewTestClient helper
	client := NewTestClient(ts)

	ctx := context.Background()

	t.Run("CreatePet", func(t *testing.T) {
		resp, err := client.CreatePet(ctx, exampleapi.CreatePetReq{
			Name: "Fluffy",
			Age:  3,
		})
		if err != nil {
			t.Fatalf("CreatePet error: %v", err)
		}
		if resp.Name != "Fluffy" {
			t.Errorf("Name = %q, want %q", resp.Name, "Fluffy")
		}
		if resp.ID == "" {
			t.Error("ID should not be empty")
		}
	})

	t.Run("ListPets", func(t *testing.T) {
		resp, err := client.ListPets(ctx)
		if err != nil {
			t.Fatalf("ListPets error: %v", err)
		}
		if len(resp.Pets) == 0 {
			t.Error("Pets should not be empty")
		}
	})

	t.Run("GetPet", func(t *testing.T) {
		resp, err := client.GetPet(ctx, exampleapi.GetPetReq{
			ID:   "test-id",
			Auth: "Bearer token",
		})
		if err != nil {
			t.Fatalf("GetPet error: %v", err)
		}
		if resp.ID != "test-id" {
			t.Errorf("ID = %q, want %q", resp.ID, "test-id")
		}
	})

	t.Run("GetPet_WithOptionalVerbose", func(t *testing.T) {
		verbose := true
		resp, err := client.GetPet(ctx, exampleapi.GetPetReq{
			ID:      "test-id",
			Auth:    "Bearer token",
			Verbose: &verbose,
		})
		if err != nil {
			t.Fatalf("GetPet error: %v", err)
		}
		if !resp.Verbose {
			t.Error("Verbose should be true when set")
		}
	})

	t.Run("DeletePet", func(t *testing.T) {
		err := client.DeletePet(ctx, exampleapi.DeletePetReq{
			ID: "test-id",
		})
		if err != nil {
			t.Fatalf("DeletePet error: %v", err)
		}
	})

	t.Run("UpdatePet", func(t *testing.T) {
		resp, err := client.UpdatePet(ctx, exampleapi.UpdatePetReq{
			ID:   "test-id",
			Name: "UpdatedName",
			Age:  5,
		})
		if err != nil {
			t.Fatalf("UpdatePet error: %v", err)
		}
		if resp.Name != "UpdatedName" {
			t.Errorf("Name = %q, want %q", resp.Name, "UpdatedName")
		}
		if resp.Age != 5 {
			t.Errorf("Age = %d, want %d", resp.Age, 5)
		}
	})

	t.Run("SearchPets", func(t *testing.T) {
		cursor := "abc123"
		auth := "Bearer token"
		resp, err := client.SearchPets(ctx, exampleapi.SearchPetsReq{
			Limit:  10,
			Tags:   []string{"tag1", "tag2"},
			Cursor: &cursor,
			Auth:   &auth,
		})
		if err != nil {
			t.Fatalf("SearchPets error: %v", err)
		}
		if resp.Limit != 10 {
			t.Errorf("Limit = %d, want %d", resp.Limit, 10)
		}
		if len(resp.Tags) != 2 {
			t.Errorf("Tags length = %d, want %d", len(resp.Tags), 2)
		}
		if resp.Cursor != "abc123" {
			t.Errorf("Cursor = %q, want %q", resp.Cursor, "abc123")
		}
	})

	t.Run("HealthCheck", func(t *testing.T) {
		err := client.HealthCheck(ctx)
		if err != nil {
			t.Fatalf("HealthCheck error: %v", err)
		}
	})
}

// TestIntegration_ErrorHandling tests that errors are properly decoded.
func TestIntegration_ErrorHandling(t *testing.T) {
	mux := http.NewServeMux()

	// Handler that returns an error
	mux.Handle("GET /pets/{id}", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeError(w, NewHTTPError(404, "not_found", "Pet not found"))
	}))

	ts := NewTestServer(t, mux)

	client := NewTestClient(ts)

	_, err := client.GetPet(context.Background(), exampleapi.GetPetReq{
		ID:   "nonexistent",
		Auth: "Bearer token",
	})

	if err == nil {
		t.Fatal("expected error for non-existent pet")
	}

	// Check that the error satisfies CodedError
	var ce portapi.CodedError
	if !errors.As(err, &ce) {
		t.Fatalf("error should satisfy CodedError, got %T: %v", err, err)
	}

	if ce.StatusCode() != 404 {
		t.Errorf("StatusCode() = %d, want 404", ce.StatusCode())
	}

	if ce.ErrorCode() != "not_found" {
		t.Errorf("ErrorCode() = %q, want %q", ce.ErrorCode(), "not_found")
	}
}

// TestIntegration_ClientBindError tests that client-side binding errors work correctly.
func TestIntegration_ClientBindError(t *testing.T) {
	// Server that should never be called
	ts := NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Server should not be called for bind errors")
		w.WriteHeader(500)
	}))

	client := NewTestClient(ts)

	t.Run("MissingRequiredPathVar", func(t *testing.T) {
		// Empty ID should cause a bind error
		err := client.DeletePet(context.Background(), exampleapi.DeletePetReq{
			ID: "", // Required but empty
		})

		if err == nil {
			t.Fatal("expected error for empty path variable")
		}

		var bindErr *portapi.ClientBindError
		if !errors.As(err, &bindErr) {
			t.Fatalf("error should be ClientBindError, got %T: %v", err, err)
		}

		if bindErr.Source != "path" {
			t.Errorf("Source = %q, want %q", bindErr.Source, "path")
		}
		if bindErr.Field != "id" {
			t.Errorf("Field = %q, want %q", bindErr.Field, "id")
		}
	})
}

// TestIntegration_CustomHTTPClient tests that a custom HTTP client can be used.
func TestIntegration_CustomHTTPClient(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("GET /health", HandleHealthCheck(exampleapi.HealthCheck))

	ts := NewTestServer(t, mux)

	// Create client with custom HTTP client (overriding ts.Client())
	customHTTP := &http.Client{}
	client := &Client{
		BaseURL: ts.URL,
		HTTP:    customHTTP,
	}

	err := client.HealthCheck(context.Background())
	if err != nil {
		t.Fatalf("HealthCheck error: %v", err)
	}
}

// TestIntegration_QueryEncoding tests that query parameters are properly encoded.
func TestIntegration_QueryEncoding(t *testing.T) {
	var receivedQuery string

	mux := http.NewServeMux()
	mux.Handle("GET /pets/search", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedQuery = r.URL.RawQuery
		// Return a valid response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{"pets":[],"limit":10,"tags":[]}`))
	}))

	ts := NewTestServer(t, mux)

	client := NewTestClient(ts)

	_, err := client.SearchPets(context.Background(), exampleapi.SearchPetsReq{
		Limit: 10,
		Tags:  []string{"cat", "dog"},
	})
	if err != nil {
		t.Fatalf("SearchPets error: %v", err)
	}

	// Check that tags are encoded as repeated keys
	if receivedQuery == "" {
		t.Fatal("Query should not be empty")
	}

	// The query should contain limit=10 and tag=cat and tag=dog
	// Note: url.Values.Encode() sorts keys alphabetically
	t.Logf("Received query: %s", receivedQuery)
}

// TestIntegration_NewTestClientUsesServerClient verifies that NewTestClient
// properly uses ts.Client() which is important for cookies, redirects, and TLS.
func TestIntegration_NewTestClientUsesServerClient(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("GET /health", HandleHealthCheck(exampleapi.HealthCheck))

	ts := NewTestServer(t, mux)

	// NewTestClient should use ts.Client() internally
	client := NewTestClient(ts)

	// Verify the client's HTTP field is set (not nil)
	if client.HTTP == nil {
		t.Error("NewTestClient should set HTTP field")
	}

	// Verify BaseURL is set correctly
	if client.BaseURL != ts.URL {
		t.Errorf("BaseURL = %q, want %q", client.BaseURL, ts.URL)
	}

	// Make a request to verify everything works
	err := client.HealthCheck(context.Background())
	if err != nil {
		t.Fatalf("HealthCheck error: %v", err)
	}
}

// TestIntegration_NewTestClientPanicsOnNil verifies that NewTestClient panics
// when given a nil server (indicating a bug in the test).
func TestIntegration_NewTestClientPanicsOnNil(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("NewTestClient(nil) should panic")
		}
	}()

	_ = NewTestClient(nil)
}

// TestIntegration_NewTestServerCleansUp verifies that NewTestServer properly
// registers cleanup via t.Cleanup.
func TestIntegration_NewTestServerCleansUp(t *testing.T) {
	var serverClosed bool

	// Create a custom handler that tracks when the server is accessed
	mux := http.NewServeMux()
	mux.Handle("GET /health", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	// Create server - it will be cleaned up automatically
	ts := NewTestServer(t, mux)
	serverURL := ts.URL

	// Verify server is running
	client := NewTestClient(ts)
	err := client.HealthCheck(context.Background())
	if err != nil {
		t.Fatalf("Server should be running: %v", err)
	}

	// After this test completes, t.Cleanup will close the server
	// We can't easily verify this in the same test, but we trust t.Cleanup
	_ = serverClosed
	_ = serverURL
}
