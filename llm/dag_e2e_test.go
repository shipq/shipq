package llm_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/shipq/shipq/channel"
	"github.com/shipq/shipq/dag"
	"github.com/shipq/shipq/llm"
	"github.com/shipq/shipq/llm/anthropic"

	_ "modernc.org/sqlite"
)

// ── End-to-end DAG persistence test with real Anthropic API + SQLite ──────────
//
// This test:
//  1. Creates a real SQLite database with llm_conversations and llm_messages tables.
//  2. Uses real Anthropic API keys from the environment.
//  3. Defines tools with a DAG (lookup_info → write_report).
//  4. Runs a first conversation turn — the model should call lookup_info.
//  5. Verifies persistence: tool_call rows are written to SQLite.
//  6. Runs a second conversation turn (simulating a follow-up message on the
//     same channel job) — the persister hydrates completedTools from prior
//     conversations, so write_report is now available.
//  7. Verifies that the DAG progressed across turns automatically.

func TestE2E_DAGPersistence_RealAnthropicSQLite(t *testing.T) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Skip("ANTHROPIC_API_KEY not set — skipping end-to-end DAG persistence test")
	}

	// ── 1. Create SQLite database with LLM tables ─────────────────────────
	db := createLLMDatabase(t)

	// ── 2. Build the persister backed by SQLite ───────────────────────────
	persister := &sqlitePersister{db: db}

	// ── 3. Define tools + DAG ─────────────────────────────────────────────
	//
	//   lookup_info  (root — always available)
	//       │
	//       ▼
	//   write_report (requires lookup_info)
	//
	g, err := dag.New([]dag.Node[string]{
		{ID: "lookup_info", Description: "Look up background information on a topic"},
		{ID: "write_report", Description: "Write a short report", HardDeps: []string{"lookup_info"}},
	})
	if err != nil {
		t.Fatalf("dag.New: %v", err)
	}

	registry := &llm.Registry{
		Tools: []llm.ToolDef{
			{
				Name:        "lookup_info",
				Description: "Look up background information on a topic. Returns a short factoid.",
				InputSchema: json.RawMessage(`{"type":"object","properties":{"topic":{"type":"string","description":"The topic to look up"}},"required":["topic"],"additionalProperties":false}`),
				Func: func(_ context.Context, argsJSON []byte) ([]byte, error) {
					var args struct {
						Topic string `json:"topic"`
					}
					if err := json.Unmarshal(argsJSON, &args); err != nil {
						return nil, err
					}
					return json.Marshal(map[string]string{
						"fact": fmt.Sprintf("Interesting fact about %s: it is widely studied.", args.Topic),
					})
				},
			},
			{
				Name:        "write_report",
				Description: "Write a short report given prior research. Returns the report text.",
				InputSchema: json.RawMessage(`{"type":"object","properties":{"title":{"type":"string","description":"Report title"},"body":{"type":"string","description":"Report body"}},"required":["title","body"],"additionalProperties":false}`),
				Func: func(_ context.Context, argsJSON []byte) ([]byte, error) {
					var args struct {
						Title string `json:"title"`
						Body  string `json:"body"`
					}
					if err := json.Unmarshal(argsJSON, &args); err != nil {
						return nil, err
					}
					return json.Marshal(map[string]string{
						"report": fmt.Sprintf("# %s\n\n%s", args.Title, args.Body),
					})
				},
			},
		},
	}

	// ── 4. Create provider ────────────────────────────────────────────────
	provider := anthropic.New(apiKey, "claude-sonnet-4-20250514")

	// ── 5. Shared job ID (simulates same channel session) ─────────────────
	jobID := fmt.Sprintf("e2e-dag-test-%d", time.Now().UnixNano())

	// ── 6. First turn: model should call lookup_info ──────────────────────
	t.Log("=== Turn 1: expecting lookup_info to be called ===")

	ch1 := createE2EChannel(t, jobID)
	client1 := llm.NewClient(provider,
		llm.WithTools(registry),
		llm.WithTaskDAG(g),
		llm.WithPersister(persister),
		llm.WithChannel(ch1),
		llm.WithSystem("You are a research assistant. Use the available tools to complete tasks. "+
			"On this turn, look up information about 'photosynthesis' using the lookup_info tool. "+
			"You MUST call the lookup_info tool. Do not write a report yet."),
		llm.WithMaxIterations(5),
	)

	resp1, err := client1.Chat(context.Background(), "Please research photosynthesis and then write a report about it.")
	if err != nil {
		t.Fatalf("Turn 1 Chat: %v", err)
	}

	t.Logf("Turn 1 response: %s", truncate(resp1.Text, 200))
	t.Logf("Turn 1 tool calls: %d", len(resp1.ToolCalls))

	// Verify lookup_info was called.
	lookupCalled := false
	for _, tc := range resp1.ToolCalls {
		t.Logf("  tool call: %s (error: %v)", tc.ToolName, tc.Error)
		if tc.ToolName == "lookup_info" && tc.Error == nil {
			lookupCalled = true
		}
	}
	if !lookupCalled {
		t.Fatal("Turn 1: expected lookup_info to be called successfully")
	}

	// Verify write_report was NOT available (it should have been filtered out
	// because lookup_info hadn't completed yet at the START of turn 1).
	// However, if the model called lookup_info and then write_report became
	// available in the same turn, that's also fine — the DAG unlocks within a turn.
	// What we really care about is that persistence recorded the tool calls.

	// ── 7. Verify SQLite has tool_call records ────────────────────────────
	completedTools := queryCompletedTools(t, db, jobID)
	t.Logf("Completed tools in DB after turn 1: %v", completedTools)

	if !contains(completedTools, "lookup_info") {
		t.Fatal("Expected 'lookup_info' to be recorded in SQLite after turn 1")
	}

	// ── 8. Second turn: write_report should now be available via hydration ─
	t.Log("=== Turn 2: expecting write_report to be available (hydrated from DB) ===")

	ch2 := createE2EChannel(t, jobID) // same jobID!
	client2 := llm.NewClient(provider,
		llm.WithTools(registry),
		llm.WithTaskDAG(g),
		llm.WithPersister(persister),
		llm.WithChannel(ch2),
		llm.WithSystem("You are a research assistant. The lookup_info tool has already been used "+
			"in a prior turn. Now you MUST use the write_report tool to write a short report "+
			"about photosynthesis. Call write_report with a title and body."),
		llm.WithMaxIterations(5),
	)

	resp2, err := client2.Chat(context.Background(), "Now write the report about photosynthesis based on your earlier research.")
	if err != nil {
		t.Fatalf("Turn 2 Chat: %v", err)
	}

	t.Logf("Turn 2 response: %s", truncate(resp2.Text, 200))
	t.Logf("Turn 2 tool calls: %d", len(resp2.ToolCalls))

	reportCalled := false
	for _, tc := range resp2.ToolCalls {
		t.Logf("  tool call: %s (error: %v)", tc.ToolName, tc.Error)
		if tc.ToolName == "write_report" && tc.Error == nil {
			reportCalled = true
		}
	}
	if !reportCalled {
		t.Fatal("Turn 2: expected write_report to be called successfully (hydrated from prior turn)")
	}

	// ── 9. Verify final DB state ──────────────────────────────────────────
	finalTools := queryCompletedTools(t, db, jobID)
	t.Logf("Completed tools in DB after turn 2: %v", finalTools)

	if !contains(finalTools, "lookup_info") {
		t.Error("Expected 'lookup_info' in final completed tools")
	}
	if !contains(finalTools, "write_report") {
		t.Error("Expected 'write_report' in final completed tools")
	}

	// Verify we have at least 2 conversations for this job.
	convCount := countConversations(t, db, jobID)
	t.Logf("Conversations for job %s: %d", jobID, convCount)
	if convCount < 2 {
		t.Errorf("Expected at least 2 conversations, got %d", convCount)
	}

	t.Log("=== E2E DAG persistence test PASSED ===")
}

// ── SQLite helpers ────────────────────────────────────────────────────────────

func createLLMDatabase(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	// Enable WAL mode for better concurrency.
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		t.Fatalf("pragma WAL: %v", err)
	}

	// Create llm_conversations table.
	_, err = db.Exec(`
		CREATE TABLE llm_conversations (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			public_id       TEXT    NOT NULL UNIQUE,
			job_id          TEXT    NOT NULL DEFAULT '',
			channel_name    TEXT    NOT NULL DEFAULT '',
			account_id      INTEGER DEFAULT 0,
			provider        TEXT    NOT NULL DEFAULT '',
			model           TEXT    NOT NULL DEFAULT '',
			system_prompt   TEXT,
			total_input_tokens  INTEGER DEFAULT 0,
			total_output_tokens INTEGER DEFAULT 0,
			tool_call_count INTEGER DEFAULT 0,
			status          TEXT    NOT NULL DEFAULT 'running',
			error_message   TEXT,
			started_at      TEXT    NOT NULL DEFAULT '',
			completed_at    TEXT,
			created_at      TEXT    NOT NULL DEFAULT (datetime('now')),
			updated_at      TEXT    NOT NULL DEFAULT (datetime('now'))
		)
	`)
	if err != nil {
		t.Fatalf("create llm_conversations: %v", err)
	}

	// Create llm_messages table.
	_, err = db.Exec(`
		CREATE TABLE llm_messages (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			public_id       TEXT    NOT NULL DEFAULT '',
			conversation_id INTEGER NOT NULL REFERENCES llm_conversations(id),
			role            TEXT    NOT NULL,
			content         TEXT,
			tool_name       TEXT,
			tool_call_id    TEXT,
			tool_input      TEXT,
			tool_output     TEXT,
			tool_error      TEXT,
			tool_duration_ms INTEGER,
			input_tokens    INTEGER DEFAULT 0,
			output_tokens   INTEGER DEFAULT 0,
			is_error        INTEGER DEFAULT 0,
			created_at      TEXT    NOT NULL DEFAULT (datetime('now'))
		)
	`)
	if err != nil {
		t.Fatalf("create llm_messages: %v", err)
	}

	return db
}

// sqlitePersister is a minimal Persister implementation backed by real SQLite.
// It uses direct SQL (not the querydefs system) because this is a test-only
// helper — the real generated persister uses querydefs.
type sqlitePersister struct {
	db *sql.DB
}

func (p *sqlitePersister) InsertConversation(ctx context.Context, params llm.InsertConversationParams) (llm.ConversationRow, error) {
	result, err := p.db.ExecContext(ctx, `
		INSERT INTO llm_conversations (public_id, job_id, channel_name, account_id, provider, model, status, started_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, params.PublicID, params.JobID, params.ChannelName, params.AccountID,
		params.Provider, params.Model, string(params.Status),
		params.StartedAt.Format(time.RFC3339Nano))
	if err != nil {
		return llm.ConversationRow{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return llm.ConversationRow{}, err
	}
	return llm.ConversationRow{ID: id, PublicID: params.PublicID}, nil
}

func (p *sqlitePersister) UpdateConversation(ctx context.Context, params llm.UpdateConversationParams) error {
	var completedAt *string
	if !params.CompletedAt.IsZero() {
		s := params.CompletedAt.Format(time.RFC3339Nano)
		completedAt = &s
	}
	var errorMessage *string
	if params.ErrorMessage != "" {
		errorMessage = &params.ErrorMessage
	}
	_, err := p.db.ExecContext(ctx, `
		UPDATE llm_conversations
		SET status = ?, total_input_tokens = ?, total_output_tokens = ?,
		    tool_call_count = ?, error_message = ?, completed_at = ?
		WHERE id = ?
	`, string(params.Status), params.InputTokens, params.OutputTokens,
		params.ToolCallCount, errorMessage, completedAt, params.ID)
	return err
}

func (p *sqlitePersister) InsertMessage(ctx context.Context, params llm.InsertMessageParams) error {
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
	_, err := p.db.ExecContext(ctx, `
		INSERT INTO llm_messages (public_id, conversation_id, role, content, tool_name, tool_call_id, is_error)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, params.PublicID, params.ConversationID, string(params.Role),
		content, toolName, toolCallID, params.IsError)
	return err
}

func (p *sqlitePersister) ListCompletedTools(ctx context.Context, jobID string) ([]string, error) {
	rows, err := p.db.QueryContext(ctx, `
		SELECT DISTINCT m.tool_name
		FROM llm_messages m
		JOIN llm_conversations c ON c.id = m.conversation_id
		WHERE c.job_id = ?
		  AND m.role = 'tool_call'
		  AND m.tool_name IS NOT NULL
	`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

// Compile-time check.
var _ llm.Persister = (*sqlitePersister)(nil)

// ── Query helpers ─────────────────────────────────────────────────────────────

func queryCompletedTools(t *testing.T, db *sql.DB, jobID string) []string {
	t.Helper()
	rows, err := db.Query(`
		SELECT DISTINCT m.tool_name
		FROM llm_messages m
		JOIN llm_conversations c ON c.id = m.conversation_id
		WHERE c.job_id = ?
		  AND m.role = 'tool_call'
		  AND m.tool_name IS NOT NULL
	`, jobID)
	if err != nil {
		t.Fatalf("queryCompletedTools: %v", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("queryCompletedTools scan: %v", err)
		}
		names = append(names, name)
	}
	return names
}

func countConversations(t *testing.T, db *sql.DB, jobID string) int {
	t.Helper()
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM llm_conversations WHERE job_id = ?`, jobID).Scan(&count)
	if err != nil {
		t.Fatalf("countConversations: %v", err)
	}
	return count
}

// ── Utility helpers ───────────────────────────────────────────────────────────

func createE2EChannel(t *testing.T, jobID string) *channel.Channel {
	t.Helper()
	tr := e2eNoopTransport{}
	incoming, cleanup, _ := tr.Subscribe("e2e-test", "sub")
	return channel.NewChannel("e2e-test", jobID, 0, 0, false, tr, incoming, cleanup)
}

type e2eNoopTransport struct{}

func (e2eNoopTransport) Publish(string, []byte) error { return nil }
func (e2eNoopTransport) Subscribe(string, string) (<-chan []byte, func(), error) {
	return make(chan []byte), func() {}, nil
}
func (e2eNoopTransport) GenerateConnectionToken(_ string, _ time.Duration) (string, error) {
	return "", nil
}
func (e2eNoopTransport) GenerateSubscriptionToken(_ string, _ string, _ time.Duration) (string, error) {
	return "", nil
}
func (e2eNoopTransport) ConnectionURL() string { return "" }

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, item) {
			return true
		}
	}
	return false
}
