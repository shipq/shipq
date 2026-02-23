package channel

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// testDepKey is a context key used in WithSetup tests.
type testDepKey struct{}

// testRequest is a simple struct used as the dispatch type in tests.
type testRequest struct {
	Prompt string `json:"prompt"`
}

// setupTestDB creates an in-memory SQLite database with the job_results table.
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	_, err = db.Exec(`CREATE TABLE job_results (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		public_id TEXT NOT NULL UNIQUE,
		channel_name TEXT NOT NULL,
		account_id INTEGER,
		author_account_id INTEGER NOT NULL DEFAULT 0,
		status TEXT NOT NULL DEFAULT 'pending',
		request_payload TEXT NOT NULL,
		result_payload TEXT,
		error_message TEXT,
		started_at TEXT,
		completed_at TEXT,
		retry_count INTEGER NOT NULL DEFAULT 0,
		created_at TEXT DEFAULT CURRENT_TIMESTAMP,
		updated_at TEXT DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		t.Fatalf("failed to create job_results table: %v", err)
	}

	return db
}

// insertPendingJob inserts a pending job into the test database and returns the job ID.
func insertPendingJob(t *testing.T, db *sql.DB, jobID, channelName string, accountID int64, requestJSON string) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO job_results (public_id, channel_name, account_id, author_account_id, status, request_payload, created_at, updated_at)
		 VALUES (?, ?, ?, ?, 'pending', ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		jobID, channelName, accountID, accountID, requestJSON,
	)
	if err != nil {
		t.Fatalf("failed to insert pending job: %v", err)
	}
}

// getJobStatus reads the job status and related fields from the database.
func getJobStatus(t *testing.T, db *sql.DB, jobID string) (status string, errorMessage *string, startedAt *string, completedAt *string) {
	t.Helper()
	row := db.QueryRow(`SELECT status, error_message, started_at, completed_at FROM job_results WHERE public_id = ?`, jobID)
	if err := row.Scan(&status, &errorMessage, &startedAt, &completedAt); err != nil {
		t.Fatalf("failed to read job status: %v", err)
	}
	return
}

func TestWrapDispatchHandler_CallsHandler(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	recorder := NewTestRecorder()

	handlerCalled := false
	var receivedPrompt string

	handler := func(ctx context.Context, req *testRequest) error {
		handlerCalled = true
		receivedPrompt = req.Prompt

		// Verify the channel is available in context
		ch := FromContext(ctx)
		if ch == nil {
			t.Error("expected Channel in context")
		}
		if ch.Name() != "chatbot" {
			t.Errorf("expected channel name 'chatbot', got %q", ch.Name())
		}
		if ch.JobID() != "job-123" {
			t.Errorf("expected job ID 'job-123', got %q", ch.JobID())
		}

		return nil
	}

	// Insert a pending job
	reqJSON, _ := json.Marshal(testRequest{Prompt: "Hello, chatbot!"})
	insertPendingJob(t, db, "job-123", "chatbot", 42, string(reqJSON))

	// Wrap the handler
	wrappedFn := WrapDispatchHandler(handler, recorder, db, "chatbot")

	// Build the dispatch payload
	dp := DispatchPayload{
		JobID:       "job-123",
		ChannelName: "chatbot",
		AccountID:   42,
		OrgID:       7,
		IsPublic:    false,
		Request:     reqJSON,
	}
	payloadJSON, _ := json.Marshal(dp)

	// Invoke the wrapped function (simulating what the TaskQueue does)
	err := wrappedFn(string(payloadJSON))
	if err != nil {
		t.Fatalf("wrapped handler returned error: %v", err)
	}

	if !handlerCalled {
		t.Error("expected handler to be called")
	}

	if receivedPrompt != "Hello, chatbot!" {
		t.Errorf("expected prompt 'Hello, chatbot!', got %q", receivedPrompt)
	}
}

func TestWrapDispatchHandler_UpdatesJobStatus(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	recorder := NewTestRecorder()

	handler := func(ctx context.Context, req *testRequest) error {
		return nil // success
	}

	reqJSON, _ := json.Marshal(testRequest{Prompt: "test"})
	insertPendingJob(t, db, "job-456", "chatbot", 42, string(reqJSON))

	wrappedFn := WrapDispatchHandler(handler, recorder, db, "chatbot")

	dp := DispatchPayload{
		JobID:       "job-456",
		ChannelName: "chatbot",
		AccountID:   42,
		OrgID:       0,
		IsPublic:    false,
		Request:     reqJSON,
	}
	payloadJSON, _ := json.Marshal(dp)

	err := wrappedFn(string(payloadJSON))
	if err != nil {
		t.Fatalf("wrapped handler returned error: %v", err)
	}

	// Verify the job status was updated to "completed"
	status, errMsg, startedAt, completedAt := getJobStatus(t, db, "job-456")

	if status != "completed" {
		t.Errorf("expected status 'completed', got %q", status)
	}

	if errMsg != nil {
		t.Errorf("expected nil error_message, got %q", *errMsg)
	}

	if startedAt == nil {
		t.Error("expected started_at to be set")
	}

	if completedAt == nil {
		t.Error("expected completed_at to be set")
	}
}

func TestWrapDispatchHandler_HandlerFailure_SetsErrorMessage(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	recorder := NewTestRecorder()

	expectedErr := fmt.Errorf("something went wrong in the handler")

	handler := func(ctx context.Context, req *testRequest) error {
		return expectedErr
	}

	reqJSON, _ := json.Marshal(testRequest{Prompt: "test"})
	insertPendingJob(t, db, "job-789", "chatbot", 42, string(reqJSON))

	wrappedFn := WrapDispatchHandler(handler, recorder, db, "chatbot")

	dp := DispatchPayload{
		JobID:       "job-789",
		ChannelName: "chatbot",
		AccountID:   42,
		OrgID:       0,
		IsPublic:    false,
		Request:     reqJSON,
	}
	payloadJSON, _ := json.Marshal(dp)

	// The wrapped function should return the handler's error
	err := wrappedFn(string(payloadJSON))
	if err == nil {
		t.Fatal("expected error from wrapped handler")
	}

	if err.Error() != expectedErr.Error() {
		t.Errorf("expected error %q, got %q", expectedErr.Error(), err.Error())
	}

	// Verify the job status was updated to "failed" with error message
	status, errMsg, startedAt, completedAt := getJobStatus(t, db, "job-789")

	if status != "failed" {
		t.Errorf("expected status 'failed', got %q", status)
	}

	if errMsg == nil {
		t.Fatal("expected error_message to be set")
	}
	if *errMsg != "something went wrong in the handler" {
		t.Errorf("expected error_message 'something went wrong in the handler', got %q", *errMsg)
	}

	if startedAt == nil {
		t.Error("expected started_at to be set (job started before failing)")
	}

	if completedAt == nil {
		t.Error("expected completed_at to be set on failure")
	}
}

func TestWrapDispatchHandler_InvalidPayload(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	recorder := NewTestRecorder()

	handler := func(ctx context.Context, req *testRequest) error {
		t.Error("handler should not be called with invalid payload")
		return nil
	}

	wrappedFn := WrapDispatchHandler(handler, recorder, db, "chatbot")

	// Send garbage JSON
	err := wrappedFn("not valid json at all")
	if err == nil {
		t.Fatal("expected error for invalid payload")
	}

	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}
}

func TestWrapDispatchHandler_InvalidHandlerSignature_Panics(t *testing.T) {
	recorder := NewTestRecorder()
	db := setupTestDB(t)
	defer db.Close()

	tests := []struct {
		name    string
		handler any
	}{
		{
			"not a function",
			"this is a string",
		},
		{
			"wrong number of args",
			func(ctx context.Context) error { return nil },
		},
		{
			"wrong first arg type",
			func(s string, req *testRequest) error { return nil },
		},
		{
			"second arg not pointer",
			func(ctx context.Context, req testRequest) error { return nil },
		},
		{
			"wrong return type",
			func(ctx context.Context, req *testRequest) string { return "" },
		},
		{
			"too many return values",
			func(ctx context.Context, req *testRequest) (string, error) { return "", nil },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if r == nil {
					t.Error("expected panic for invalid handler signature")
				}
			}()
			WrapDispatchHandler(tt.handler, recorder, db, "test")
		})
	}
}

func TestWrapDispatchHandler_PublicChannel(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	recorder := NewTestRecorder()

	var capturedChannel *Channel

	handler := func(ctx context.Context, req *testRequest) error {
		capturedChannel = FromContext(ctx)
		return nil
	}

	reqJSON, _ := json.Marshal(testRequest{Prompt: "public test"})
	insertPendingJob(t, db, "pub-job-1", "demo", 0, string(reqJSON))

	wrappedFn := WrapDispatchHandler(handler, recorder, db, "demo")

	dp := DispatchPayload{
		JobID:       "pub-job-1",
		ChannelName: "demo",
		AccountID:   0,
		OrgID:       0,
		IsPublic:    true,
		Request:     reqJSON,
	}
	payloadJSON, _ := json.Marshal(dp)

	err := wrappedFn(string(payloadJSON))
	if err != nil {
		t.Fatalf("wrapped handler returned error: %v", err)
	}

	if capturedChannel == nil {
		t.Fatal("expected channel in context")
	}

	if capturedChannel.isPublic != true {
		t.Error("expected channel to be marked as public")
	}

	if capturedChannel.accountID != 0 {
		t.Errorf("expected accountID 0 for public channel, got %d", capturedChannel.accountID)
	}
}

func TestWrapDispatchHandler_SetsRunningStatus(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	recorder := NewTestRecorder()

	// Use a channel to coordinate: the handler will check the status mid-execution
	statusDuringHandler := make(chan string, 1)

	handler := func(ctx context.Context, req *testRequest) error {
		// Check the status while the handler is running
		var s string
		row := db.QueryRow(`SELECT status FROM job_results WHERE public_id = ?`, "running-check-job")
		if err := row.Scan(&s); err != nil {
			return err
		}
		statusDuringHandler <- s
		return nil
	}

	reqJSON, _ := json.Marshal(testRequest{Prompt: "test"})
	insertPendingJob(t, db, "running-check-job", "chatbot", 42, string(reqJSON))

	wrappedFn := WrapDispatchHandler(handler, recorder, db, "chatbot")

	dp := DispatchPayload{
		JobID:       "running-check-job",
		ChannelName: "chatbot",
		AccountID:   42,
		OrgID:       0,
		IsPublic:    false,
		Request:     reqJSON,
	}
	payloadJSON, _ := json.Marshal(dp)

	err := wrappedFn(string(payloadJSON))
	if err != nil {
		t.Fatalf("wrapped handler returned error: %v", err)
	}

	select {
	case s := <-statusDuringHandler:
		if s != "running" {
			t.Errorf("expected status 'running' during handler execution, got %q", s)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for status check")
	}
}

func TestComputeChannelID_Public(t *testing.T) {
	id := ComputeChannelID("demo", 0, "abc123", true)
	expected := "demo_public_abc123"
	if id != expected {
		t.Errorf("expected %q, got %q", expected, id)
	}
}

func TestComputeChannelID_Scoped(t *testing.T) {
	id := ComputeChannelID("chatbot", 42, "abc123", false)
	expected := "chatbot_42_abc123"
	if id != expected {
		t.Errorf("expected %q, got %q", expected, id)
	}
}

func TestComputeChannelID_MatchesChannelContextChannelID(t *testing.T) {
	// Verify that ComputeChannelID produces the same result as Channel.channelID().
	// This is critical for [L3] — the token endpoint must produce the exact same
	// channel name as the worker uses.

	tests := []struct {
		name      string
		chName    string
		accountID int64
		jobID     string
		isPublic  bool
	}{
		{"scoped channel", "chatbot", 42, "job-abc", false},
		{"public channel", "demo", 0, "job-xyz", true},
		{"different account", "assistant", 99, "job-def", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Compute via exported function (used by HTTP token endpoint)
			fromExported := ComputeChannelID(tt.chName, tt.accountID, tt.jobID, tt.isPublic)

			// Compute via Channel struct method (used by worker)
			ch := &Channel{
				name:      tt.chName,
				accountID: tt.accountID,
				jobID:     tt.jobID,
				isPublic:  tt.isPublic,
			}
			fromChannel := ch.channelID()

			if fromExported != fromChannel {
				t.Errorf("channel ID mismatch: ComputeChannelID=%q, Channel.channelID()=%q", fromExported, fromChannel)
			}
		})
	}
}

func TestWrapDispatchHandler_WithSetup(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	recorder := NewTestRecorder()

	var capturedDep string

	handler := func(ctx context.Context, req *testRequest) error {
		// The Setup function should have injected "injected-value" into the context
		val, ok := ctx.Value(testDepKey{}).(string)
		if !ok {
			t.Error("expected testDepKey in context from Setup function")
			return nil
		}
		capturedDep = val
		return nil
	}

	setupFn := func(ctx context.Context) context.Context {
		return context.WithValue(ctx, testDepKey{}, "injected-value")
	}

	reqJSON, _ := json.Marshal(testRequest{Prompt: "setup test"})
	insertPendingJob(t, db, "setup-job-1", "chatbot", 42, string(reqJSON))

	wrappedFn := WrapDispatchHandler(handler, recorder, db, "chatbot", WithSetup(setupFn))

	dp := DispatchPayload{
		JobID:       "setup-job-1",
		ChannelName: "chatbot",
		AccountID:   42,
		OrgID:       0,
		IsPublic:    false,
		Request:     reqJSON,
	}
	payloadJSON, _ := json.Marshal(dp)

	err := wrappedFn(string(payloadJSON))
	if err != nil {
		t.Fatalf("wrapped handler returned error: %v", err)
	}

	if capturedDep != "injected-value" {
		t.Errorf("expected capturedDep 'injected-value', got %q", capturedDep)
	}
}

func TestWrapDispatchHandler_WithSetup_ChannelAvailableInSetup(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	recorder := NewTestRecorder()

	var channelInSetup *Channel

	handler := func(ctx context.Context, req *testRequest) error {
		return nil
	}

	setupFn := func(ctx context.Context) context.Context {
		// The Channel should already be in the context when Setup is called
		channelInSetup = FromContext(ctx)
		return ctx
	}

	reqJSON, _ := json.Marshal(testRequest{Prompt: "setup channel test"})
	insertPendingJob(t, db, "setup-job-2", "chatbot", 42, string(reqJSON))

	wrappedFn := WrapDispatchHandler(handler, recorder, db, "chatbot", WithSetup(setupFn))

	dp := DispatchPayload{
		JobID:       "setup-job-2",
		ChannelName: "chatbot",
		AccountID:   42,
		OrgID:       0,
		IsPublic:    false,
		Request:     reqJSON,
	}
	payloadJSON, _ := json.Marshal(dp)

	err := wrappedFn(string(payloadJSON))
	if err != nil {
		t.Fatalf("wrapped handler returned error: %v", err)
	}

	if channelInSetup == nil {
		t.Error("expected Channel to be available in context during Setup")
	} else if channelInSetup.Name() != "chatbot" {
		t.Errorf("expected channel name 'chatbot' in Setup, got %q", channelInSetup.Name())
	}
}

func TestWrapDispatchHandler_WithoutSetup_StillWorks(t *testing.T) {
	// Verify that WrapDispatchHandler works without any options (backward compat)
	db := setupTestDB(t)
	defer db.Close()

	recorder := NewTestRecorder()

	handlerCalled := false
	handler := func(ctx context.Context, req *testRequest) error {
		handlerCalled = true
		return nil
	}

	reqJSON, _ := json.Marshal(testRequest{Prompt: "no setup"})
	insertPendingJob(t, db, "no-setup-job", "chatbot", 42, string(reqJSON))

	wrappedFn := WrapDispatchHandler(handler, recorder, db, "chatbot")

	dp := DispatchPayload{
		JobID:       "no-setup-job",
		ChannelName: "chatbot",
		AccountID:   42,
		OrgID:       0,
		IsPublic:    false,
		Request:     reqJSON,
	}
	payloadJSON, _ := json.Marshal(dp)

	err := wrappedFn(string(payloadJSON))
	if err != nil {
		t.Fatalf("wrapped handler returned error: %v", err)
	}

	if !handlerCalled {
		t.Error("expected handler to be called without Setup option")
	}
}

func TestDispatchPayload_RoundTrip(t *testing.T) {
	reqJSON := json.RawMessage(`{"prompt":"hello"}`)
	dp := DispatchPayload{
		JobID:       "job-001",
		ChannelName: "chatbot",
		AccountID:   42,
		OrgID:       7,
		IsPublic:    false,
		Request:     reqJSON,
	}

	data, err := json.Marshal(dp)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var dp2 DispatchPayload
	if err := json.Unmarshal(data, &dp2); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if dp2.JobID != dp.JobID {
		t.Errorf("JobID mismatch: %q vs %q", dp2.JobID, dp.JobID)
	}
	if dp2.ChannelName != dp.ChannelName {
		t.Errorf("ChannelName mismatch: %q vs %q", dp2.ChannelName, dp.ChannelName)
	}
	if dp2.AccountID != dp.AccountID {
		t.Errorf("AccountID mismatch: %d vs %d", dp2.AccountID, dp.AccountID)
	}
	if dp2.OrgID != dp.OrgID {
		t.Errorf("OrgID mismatch: %d vs %d", dp2.OrgID, dp.OrgID)
	}
	if dp2.IsPublic != dp.IsPublic {
		t.Errorf("IsPublic mismatch: %v vs %v", dp2.IsPublic, dp.IsPublic)
	}
	if string(dp2.Request) != string(dp.Request) {
		t.Errorf("Request mismatch: %s vs %s", string(dp2.Request), string(dp.Request))
	}
}
