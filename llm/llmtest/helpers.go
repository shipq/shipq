package llmtest

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/shipq/shipq/llm"

	_ "modernc.org/sqlite"
)

// AssertToolCalled asserts that a tool with the given name was called during
// the conversation and returns its ToolCallLog entry for further inspection.
func AssertToolCalled(t *testing.T, resp *llm.Response, toolName string) llm.ToolCallLog {
	t.Helper()
	for _, tc := range resp.ToolCalls {
		if tc.ToolName == toolName {
			return tc
		}
	}
	var names []string
	for _, tc := range resp.ToolCalls {
		names = append(names, tc.ToolName)
	}
	t.Fatalf("AssertToolCalled: tool %q was not called; called tools: %v", toolName, names)
	return llm.ToolCallLog{} // unreachable
}

// AssertToolNotCalled asserts that no tool with the given name was called.
func AssertToolNotCalled(t *testing.T, resp *llm.Response, toolName string) {
	t.Helper()
	for _, tc := range resp.ToolCalls {
		if tc.ToolName == toolName {
			t.Fatalf("AssertToolNotCalled: tool %q was called but should not have been", toolName)
		}
	}
}

// AssertToolCalledWith asserts that a tool was called with JSON arguments
// matching the expected value (deep equal after JSON unmarshal of both sides).
func AssertToolCalledWith(t *testing.T, resp *llm.Response, toolName string, expectedArgsJSON string) {
	t.Helper()

	log := AssertToolCalled(t, resp, toolName)

	var actual, expected any
	if err := json.Unmarshal(log.Input, &actual); err != nil {
		t.Fatalf("AssertToolCalledWith: failed to unmarshal actual args for tool %q: %v\nraw: %s", toolName, err, log.Input)
	}
	if err := json.Unmarshal([]byte(expectedArgsJSON), &expected); err != nil {
		t.Fatalf("AssertToolCalledWith: failed to unmarshal expected args: %v\nraw: %s", err, expectedArgsJSON)
	}

	if !reflect.DeepEqual(actual, expected) {
		actualPretty, _ := json.MarshalIndent(actual, "", "  ")
		expectedPretty, _ := json.MarshalIndent(expected, "", "  ")
		t.Fatalf("AssertToolCalledWith: tool %q args mismatch\ngot:\n%s\nwant:\n%s", toolName, actualPretty, expectedPretty)
	}
}

// AssertToolCallCount asserts the total number of tool calls made during the conversation.
func AssertToolCallCount(t *testing.T, resp *llm.Response, expected int) {
	t.Helper()
	if got := len(resp.ToolCalls); got != expected {
		t.Fatalf("AssertToolCallCount: got %d tool calls, want %d", got, expected)
	}
}

// AssertResponseContains asserts that the final response text contains the given substring.
func AssertResponseContains(t *testing.T, resp *llm.Response, substring string) {
	t.Helper()
	if !strings.Contains(resp.Text, substring) {
		t.Fatalf("AssertResponseContains: response text does not contain %q\nfull text: %q", substring, resp.Text)
	}
}

// ValidateSchema validates that a JSON string conforms to a tool's JSON Schema.
// This performs a basic structural validation: it checks that the JSON is valid,
// that it is an object, and that all required properties are present.
// Fails the test if validation fails.
func ValidateSchema(t *testing.T, toolDef llm.ToolDef, inputJSON string) {
	t.Helper()

	// Parse the input JSON.
	var inputObj map[string]any
	if err := json.Unmarshal([]byte(inputJSON), &inputObj); err != nil {
		t.Fatalf("ValidateSchema: input JSON is not valid: %v", err)
	}

	// Parse the schema to extract required fields.
	var schema struct {
		Type       string         `json:"type"`
		Required   []string       `json:"required"`
		Properties map[string]any `json:"properties"`
	}
	if err := json.Unmarshal(toolDef.InputSchema, &schema); err != nil {
		t.Fatalf("ValidateSchema: failed to parse tool schema: %v", err)
	}

	// Check that all required fields are present.
	for _, req := range schema.Required {
		if _, ok := inputObj[req]; !ok {
			t.Errorf("ValidateSchema: required property %q is missing from input", req)
		}
	}

	// If additionalProperties is false, check for extra fields.
	var rawSchema map[string]any
	if err := json.Unmarshal(toolDef.InputSchema, &rawSchema); err == nil {
		if ap, ok := rawSchema["additionalProperties"]; ok {
			if apBool, isBool := ap.(bool); isBool && !apBool {
				for key := range inputObj {
					if schema.Properties != nil {
						if _, defined := schema.Properties[key]; !defined {
							t.Errorf("ValidateSchema: unexpected additional property %q (additionalProperties is false)", key)
						}
					}
				}
			}
		}
	}
}

// CallTool calls a tool's ToolFunc directly with the given JSON args
// and returns the raw result bytes. Fails the test on any error.
func CallTool(t *testing.T, toolDef llm.ToolDef, inputJSON string) []byte {
	t.Helper()
	result, err := toolDef.Func(context.Background(), []byte(inputJSON))
	if err != nil {
		t.Fatalf("CallTool(%q): unexpected error: %v", toolDef.Name, err)
	}
	return result
}

// CallToolExpectError calls a tool's ToolFunc and asserts it returns a non-nil error.
// Returns the error for further inspection.
func CallToolExpectError(t *testing.T, toolDef llm.ToolDef, inputJSON string) error {
	t.Helper()
	_, err := toolDef.Func(context.Background(), []byte(inputJSON))
	if err == nil {
		t.Fatalf("CallToolExpectError(%q): expected error but got nil", toolDef.Name)
	}
	return err
}

// NewTestClient creates an llm.Client wired to a MockProvider with no
// channel and no persistence — suitable for pure unit tests of conversation logic.
func NewTestClient(t *testing.T, mock *MockProvider, opts ...llm.Option) *llm.Client {
	t.Helper()
	return llm.NewClient(mock, opts...)
}

// testPersister is a simple Persister backed by a *sql.DB with the
// llm_conversations and llm_messages tables. It uses raw SQL so it does not
// depend on the generated querydef code.
type testPersister struct {
	db *sql.DB
}

func (p *testPersister) InsertConversation(ctx context.Context, params llm.InsertConversationParams) (llm.ConversationRow, error) {
	startedAt := params.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now()
	}

	var accountID *int64
	if params.AccountID != 0 {
		accountID = &params.AccountID
	}

	result, err := p.db.ExecContext(ctx,
		`INSERT INTO llm_conversations (public_id, job_id, channel_name, account_id, provider, model, status, started_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		params.PublicID, params.JobID, params.ChannelName, accountID,
		params.Provider, params.Model, string(params.Status),
		startedAt.Format(time.RFC3339),
	)
	if err != nil {
		return llm.ConversationRow{}, fmt.Errorf("testPersister.InsertConversation: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return llm.ConversationRow{}, fmt.Errorf("testPersister.InsertConversation: last insert id: %w", err)
	}

	return llm.ConversationRow{ID: id, PublicID: params.PublicID}, nil
}

func (p *testPersister) UpdateConversation(ctx context.Context, params llm.UpdateConversationParams) error {
	var completedAt *string
	if !params.CompletedAt.IsZero() {
		s := params.CompletedAt.Format(time.RFC3339)
		completedAt = &s
	}

	var errorMessage *string
	if params.ErrorMessage != "" {
		errorMessage = &params.ErrorMessage
	}

	_, err := p.db.ExecContext(ctx,
		`UPDATE llm_conversations
		 SET status = ?, total_input_tokens = ?, total_output_tokens = ?,
		     tool_call_count = ?, completed_at = ?, error_message = ?
		 WHERE id = ?`,
		string(params.Status), params.InputTokens, params.OutputTokens,
		params.ToolCallCount, completedAt, errorMessage,
		params.ID,
	)
	if err != nil {
		return fmt.Errorf("testPersister.UpdateConversation: %w", err)
	}
	return nil
}

func (p *testPersister) InsertMessage(ctx context.Context, params llm.InsertMessageParams) error {
	// Auto-assign sequence by counting existing messages for this conversation.
	var seq int
	row := p.db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(sequence), -1) + 1 FROM llm_messages WHERE conversation_id = ?`,
		params.ConversationID,
	)
	if err := row.Scan(&seq); err != nil {
		return fmt.Errorf("testPersister.InsertMessage: sequence: %w", err)
	}

	var toolName, toolCallID, content *string
	if params.ToolName != "" {
		toolName = &params.ToolName
	}
	if params.ToolCallID != "" {
		toolCallID = &params.ToolCallID
	}
	if params.Content != "" {
		content = &params.Content
	}

	createdAt := params.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}

	_, err := p.db.ExecContext(ctx,
		`INSERT INTO llm_messages (conversation_id, sequence, role, content, tool_name, tool_call_id, is_error, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		params.ConversationID, seq, string(params.Role), content,
		toolName, toolCallID, params.IsError,
		createdAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("testPersister.InsertMessage: %w", err)
	}
	return nil
}

// createLLMTables creates the llm_conversations and llm_messages tables
// in the given database using raw DDL.
func createLLMTables(db *sql.DB) error {
	const ddl = `
	CREATE TABLE IF NOT EXISTS llm_conversations (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		public_id TEXT UNIQUE NOT NULL,
		job_id TEXT NOT NULL DEFAULT '',
		channel_name TEXT NOT NULL DEFAULT '',
		account_id INTEGER,
		provider TEXT NOT NULL DEFAULT '',
		model TEXT NOT NULL DEFAULT '',
		system_prompt TEXT,
		total_input_tokens INTEGER NOT NULL DEFAULT 0,
		total_output_tokens INTEGER NOT NULL DEFAULT 0,
		tool_call_count INTEGER NOT NULL DEFAULT 0,
		status TEXT NOT NULL DEFAULT 'running',
		error_message TEXT,
		started_at TEXT NOT NULL,
		completed_at TEXT,
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		updated_at TEXT NOT NULL DEFAULT (datetime('now'))
	);

	CREATE TABLE IF NOT EXISTS llm_messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		conversation_id INTEGER NOT NULL,
		sequence INTEGER NOT NULL,
		role TEXT NOT NULL,
		content TEXT,
		tool_name TEXT,
		tool_call_id TEXT,
		tool_input TEXT,
		tool_output TEXT,
		tool_error TEXT,
		tool_duration_ms INTEGER,
		is_error INTEGER NOT NULL DEFAULT 0,
		input_tokens INTEGER NOT NULL DEFAULT 0,
		output_tokens INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		updated_at TEXT NOT NULL DEFAULT (datetime('now'))
	);
	`
	_, err := db.Exec(ddl)
	return err
}

// NewTestClientWithDB creates an llm.Client wired to a MockProvider and
// an in-memory SQLite database with the LLM tables already migrated.
// Useful for testing persistence without a real database server.
// The DB is closed and cleaned up when the test ends via t.Cleanup.
func NewTestClientWithDB(t *testing.T, mock *MockProvider, opts ...llm.Option) (*llm.Client, *sql.DB) {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("NewTestClientWithDB: open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := createLLMTables(db); err != nil {
		t.Fatalf("NewTestClientWithDB: create tables: %v", err)
	}

	persister := &testPersister{db: db}

	// Prepend the persister option so caller-provided opts can override.
	allOpts := make([]llm.Option, 0, len(opts)+1)
	allOpts = append(allOpts, llm.WithPersister(persister))
	allOpts = append(allOpts, opts...)

	client := llm.NewClient(mock, allOpts...)
	return client, db
}

// AssertConversationPersisted verifies that an llm_conversations row with
// the given public_id exists in the database with the expected status.
func AssertConversationPersisted(t *testing.T, db *sql.DB, conversationPublicID string, expectedStatus string) {
	t.Helper()

	var status string
	err := db.QueryRow(
		`SELECT status FROM llm_conversations WHERE public_id = ?`,
		conversationPublicID,
	).Scan(&status)
	if err == sql.ErrNoRows {
		t.Fatalf("AssertConversationPersisted: no conversation found with public_id %q", conversationPublicID)
	}
	if err != nil {
		t.Fatalf("AssertConversationPersisted: query error: %v", err)
	}
	if status != expectedStatus {
		t.Fatalf("AssertConversationPersisted: conversation %q has status %q, want %q", conversationPublicID, status, expectedStatus)
	}
}

// AssertMessageCount verifies the number of llm_messages rows for a given
// conversation (by internal integer id).
func AssertMessageCount(t *testing.T, db *sql.DB, conversationID int64, expectedCount int) {
	t.Helper()

	var count int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM llm_messages WHERE conversation_id = ?`,
		conversationID,
	).Scan(&count)
	if err != nil {
		t.Fatalf("AssertMessageCount: query error: %v", err)
	}
	if count != expectedCount {
		t.Fatalf("AssertMessageCount: conversation %d has %d messages, want %d", conversationID, count, expectedCount)
	}
}

// AssertMessageSequence verifies that messages for a conversation have the
// expected roles in the expected order (by sequence column ascending).
// Example: AssertMessageSequence(t, db, 1, "user", "assistant", "tool_call", "tool_result", "assistant")
func AssertMessageSequence(t *testing.T, db *sql.DB, conversationID int64, expectedRoles ...string) {
	t.Helper()

	rows, err := db.Query(
		`SELECT role FROM llm_messages WHERE conversation_id = ? ORDER BY sequence ASC`,
		conversationID,
	)
	if err != nil {
		t.Fatalf("AssertMessageSequence: query error: %v", err)
	}
	defer rows.Close()

	var actualRoles []string
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			t.Fatalf("AssertMessageSequence: scan error: %v", err)
		}
		actualRoles = append(actualRoles, role)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("AssertMessageSequence: rows error: %v", err)
	}

	if len(actualRoles) != len(expectedRoles) {
		t.Fatalf("AssertMessageSequence: conversation %d has %d messages, want %d\nactual roles:   %v\nexpected roles: %v",
			conversationID, len(actualRoles), len(expectedRoles), actualRoles, expectedRoles)
	}

	for i, expected := range expectedRoles {
		if actualRoles[i] != expected {
			t.Fatalf("AssertMessageSequence: message[%d] has role %q, want %q\nactual roles:   %v\nexpected roles: %v",
				i, actualRoles[i], expected, actualRoles, expectedRoles)
		}
	}
}
