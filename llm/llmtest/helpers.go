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

	"github.com/shipq/shipq/db/portsql/ddl"
	"github.com/shipq/shipq/db/portsql/migrate"
	"github.com/shipq/shipq/db/portsql/query"
	"github.com/shipq/shipq/db/portsql/query/compile"
	"github.com/shipq/shipq/llm"
)

// ═══════════════════════════════════════════════════════════════════════════════
// Table type and column definitions (local, no dependency on generated code)
// ═══════════════════════════════════════════════════════════════════════════════

type simpleTable string

func (t simpleTable) TableName() string { return string(t) }

var (
	// ── llm_conversations ─────────────────────────────────────────────────
	convTable             = simpleTable("llm_conversations")
	convID                = query.Int64Column{Table: "llm_conversations", Name: "id"}
	convPublicID          = query.StringColumn{Table: "llm_conversations", Name: "public_id"}
	convJobID             = query.StringColumn{Table: "llm_conversations", Name: "job_id"}
	convChannelName       = query.StringColumn{Table: "llm_conversations", Name: "channel_name"}
	convAccountID         = query.NullInt64Column{Table: "llm_conversations", Name: "account_id"}
	convProvider          = query.StringColumn{Table: "llm_conversations", Name: "provider"}
	convModel             = query.StringColumn{Table: "llm_conversations", Name: "model"}
	convStatus            = query.StringColumn{Table: "llm_conversations", Name: "status"}
	convTotalInputTokens  = query.Int64Column{Table: "llm_conversations", Name: "total_input_tokens"}
	convTotalOutputTokens = query.Int64Column{Table: "llm_conversations", Name: "total_output_tokens"}
	convToolCallCount     = query.Int64Column{Table: "llm_conversations", Name: "tool_call_count"}
	convErrorMessage      = query.NullStringColumn{Table: "llm_conversations", Name: "error_message"}
	convStartedAt         = query.StringColumn{Table: "llm_conversations", Name: "started_at"}
	convCompletedAt       = query.NullStringColumn{Table: "llm_conversations", Name: "completed_at"}

	// ── llm_messages ──────────────────────────────────────────────────────
	msgTable          = simpleTable("llm_messages")
	msgPublicID       = query.StringColumn{Table: "llm_messages", Name: "public_id"}
	msgConversationID = query.Int64Column{Table: "llm_messages", Name: "conversation_id"}
	msgRole           = query.StringColumn{Table: "llm_messages", Name: "role"}
	msgContent        = query.NullStringColumn{Table: "llm_messages", Name: "content"}
	msgToolName       = query.NullStringColumn{Table: "llm_messages", Name: "tool_name"}
	msgToolCallID     = query.NullStringColumn{Table: "llm_messages", Name: "tool_call_id"}
	msgIsError        = query.Int64Column{Table: "llm_messages", Name: "is_error"}
	msgCreatedAt      = query.StringColumn{Table: "llm_messages", Name: "created_at"}
)

// ═══════════════════════════════════════════════════════════════════════════════
// Query ASTs (package-level, compiled on demand via compileSQL)
// ═══════════════════════════════════════════════════════════════════════════════

// insertConversationAST inserts a full conversation row (8 columns).
var insertConversationAST = query.InsertInto(convTable).
	Columns(convPublicID, convJobID, convChannelName, convAccountID,
		convProvider, convModel, convStatus, convStartedAt).
	Values(
		query.Param[string]("public_id"),
		query.Param[string]("job_id"),
		query.Param[string]("channel_name"),
		query.Param[*int64]("account_id"),
		query.Param[string]("provider"),
		query.Param[string]("model"),
		query.Param[string]("status"),
		query.Param[string]("started_at"),
	).Build()

// updateConversationAST updates status, tokens, tool_call_count, completed_at, error_message WHERE id = ?.
var updateConversationAST = query.Update(convTable).
	Set(convStatus, query.Param[string]("status")).
	Set(convTotalInputTokens, query.Param[int64]("total_input_tokens")).
	Set(convTotalOutputTokens, query.Param[int64]("total_output_tokens")).
	Set(convToolCallCount, query.Param[int64]("tool_call_count")).
	Set(convCompletedAt, query.Param[*string]("completed_at")).
	Set(convErrorMessage, query.Param[*string]("error_message")).
	Where(convID.Eq(query.Param[int64]("id"))).
	Build()

// insertMessageAST inserts a message row (8 columns).
var insertMessageAST = query.InsertInto(msgTable).
	Columns(msgPublicID, msgConversationID, msgRole, msgContent,
		msgToolName, msgToolCallID, msgIsError, msgCreatedAt).
	Values(
		query.Param[string]("public_id"),
		query.Param[int64]("conversation_id"),
		query.Param[string]("role"),
		query.Param[*string]("content"),
		query.Param[*string]("tool_name"),
		query.Param[*string]("tool_call_id"),
		query.Param[bool]("is_error"),
		query.Param[string]("created_at"),
	).Build()

// selectConversationStatusAST: SELECT status FROM llm_conversations WHERE public_id = ?
var selectConversationStatusAST = query.From(convTable).
	Select(convStatus).
	Where(convPublicID.Eq(query.Param[string]("public_id"))).
	Build()

// countMessagesAST: SELECT COUNT(*) FROM llm_messages WHERE conversation_id = ?
var countMessagesAST = query.From(msgTable).
	SelectCount().
	Where(msgConversationID.Eq(query.Param[int64]("conversation_id"))).
	Build()

// selectMessageRolesAST: SELECT role FROM llm_messages WHERE conversation_id = ? ORDER BY created_at ASC
var selectMessageRolesAST = query.From(msgTable).
	Select(msgRole).
	Where(msgConversationID.Eq(query.Param[int64]("conversation_id"))).
	OrderBy(query.OrderByExpr{Expr: query.ColumnExpr{Column: msgCreatedAt}, Desc: false}).
	Build()

// selectConversationByPublicIDAST: SELECT id FROM llm_conversations WHERE public_id = ?
var selectConversationByPublicIDAST = query.From(convTable).
	Select(convID).
	Where(convPublicID.Eq(query.Param[string]("public_id"))).
	Build()

// selectConversationTokensAST: SELECT total_input_tokens, total_output_tokens, tool_call_count FROM llm_conversations WHERE id = ?
var selectConversationTokensAST = query.From(convTable).
	Select(convTotalInputTokens, convTotalOutputTokens, convToolCallCount).
	Where(convID.Eq(query.Param[int64]("id"))).
	Build()

// selectConversationProviderModelAST: SELECT provider, model FROM llm_conversations WHERE id = ?
var selectConversationProviderModelAST = query.From(convTable).
	Select(convProvider, convModel).
	Where(convID.Eq(query.Param[int64]("id"))).
	Build()

// countConversationsAST: SELECT COUNT(*) FROM llm_conversations
var countConversationsAST = query.From(convTable).
	SelectCount().
	Build()

// countAllMessagesAST: SELECT COUNT(*) FROM llm_messages
var countAllMessagesAST = query.From(msgTable).
	SelectCount().
	Build()

// insertTestConversationAST: minimal INSERT for test infrastructure (5 columns).
var insertTestConversationAST = query.InsertInto(convTable).
	Columns(convPublicID, convProvider, convModel, convStatus, convStartedAt).
	Values(
		query.Param[string]("public_id"),
		query.Param[string]("provider"),
		query.Param[string]("model"),
		query.Param[string]("status"),
		query.Param[string]("started_at"),
	).Build()

// insertTestConversationWithErrorAST: INSERT with error_message for failed conversation tests (6 columns).
var insertTestConversationWithErrorAST = query.InsertInto(convTable).
	Columns(convPublicID, convProvider, convModel, convStatus, convStartedAt, convErrorMessage).
	Values(
		query.Param[string]("public_id"),
		query.Param[string]("provider"),
		query.Param[string]("model"),
		query.Param[string]("status"),
		query.Param[string]("started_at"),
		query.Param[*string]("error_message"),
	).Build()

// insertTestMessageAST: minimal INSERT for test infrastructure (5 columns).
var insertTestMessageAST = query.InsertInto(msgTable).
	Columns(msgPublicID, msgConversationID, msgRole, msgContent, msgCreatedAt).
	Values(
		query.Param[string]("public_id"),
		query.Param[int64]("conversation_id"),
		query.Param[string]("role"),
		query.Param[string]("content"),
		query.Param[string]("created_at"),
	).Build()

// selectConversationStatusByPublicIDAST: SELECT status FROM llm_conversations WHERE public_id = ?
// (same as selectConversationStatusAST — aliased for clarity in test usage)
var selectConversationStatusByPublicIDAST = selectConversationStatusAST

// selectConversationTokensByPublicIDAST: SELECT total_input_tokens, total_output_tokens FROM llm_conversations WHERE public_id = ?
var selectConversationTokensByPublicIDAST = query.From(convTable).
	Select(convTotalInputTokens, convTotalOutputTokens).
	Where(convPublicID.Eq(query.Param[string]("public_id"))).
	Build()

// ═══════════════════════════════════════════════════════════════════════════════
// Compile helper
// ═══════════════════════════════════════════════════════════════════════════════

// compileSQL compiles a query AST to SQL using the SQLite dialect.
// Panics on compile error because these are static, compile-time-known ASTs.
func compileSQL(ast *query.AST) (sqlStr string, paramOrder []string) {
	c := compile.NewCompiler(compile.SQLite)
	s, p, err := c.Compile(ast)
	if err != nil {
		panic("llmtest: failed to compile query: " + err.Error())
	}
	return s, p
}

// ═══════════════════════════════════════════════════════════════════════════════
// DDL: portable table creation
// ═══════════════════════════════════════════════════════════════════════════════

// createLLMTables creates the llm_conversations and llm_messages tables
// in the given database using the DDL builder and migration system.
func createLLMTables(db *sql.DB) error {
	plan := &migrate.MigrationPlan{
		Schema: migrate.Schema{
			Tables: make(map[string]ddl.Table),
		},
	}

	// Define llm_conversations table.
	plan.SetCurrentMigration("00000000000001_create_llm_conversations")
	_, err := plan.AddEmptyTable("llm_conversations", func(tb *ddl.TableBuilder) error {
		tb.Bigint("id").PrimaryKey()
		tb.String("public_id").Unique()
		tb.String("job_id").Default("")
		tb.String("channel_name").Default("")
		tb.Bigint("account_id").Nullable()
		tb.String("provider").Default("")
		tb.String("model").Default("")
		tb.Text("system_prompt").Nullable()
		tb.Integer("total_input_tokens").Default(0)
		tb.Integer("total_output_tokens").Default(0)
		tb.Integer("tool_call_count").Default(0)
		tb.String("status").Default("running")
		tb.Text("error_message").Nullable()
		tb.String("started_at")
		tb.String("completed_at").Nullable()
		tb.Datetime("created_at").Default("CURRENT_TIMESTAMP")
		tb.Datetime("updated_at").Default("CURRENT_TIMESTAMP")
		return nil
	})
	if err != nil {
		return fmt.Errorf("llmtest: define llm_conversations: %w", err)
	}

	// Define llm_messages table.
	plan.SetCurrentMigration("00000000000002_create_llm_messages")
	_, err = plan.AddEmptyTable("llm_messages", func(tb *ddl.TableBuilder) error {
		tb.Bigint("id").PrimaryKey()
		tb.String("public_id").Unique()
		tb.Integer("conversation_id")
		tb.String("role")
		tb.Text("content").Nullable()
		tb.String("tool_name").Nullable()
		tb.String("tool_call_id").Nullable()
		tb.Text("tool_input").Nullable()
		tb.Text("tool_output").Nullable()
		tb.Text("tool_error").Nullable()
		tb.Integer("tool_duration_ms").Nullable()
		tb.Integer("is_error").Default(0)
		tb.Integer("input_tokens").Default(0)
		tb.Integer("output_tokens").Default(0)
		tb.Datetime("created_at").Default("CURRENT_TIMESTAMP")
		tb.Datetime("updated_at").Default("CURRENT_TIMESTAMP")
		return nil
	})
	if err != nil {
		return fmt.Errorf("llmtest: define llm_messages: %w", err)
	}

	// Execute each migration's SQLite DDL.
	for _, m := range plan.Migrations {
		stmts := strings.Split(m.Instructions.Sqlite, ";")
		for _, stmt := range stmts {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			if _, err := db.Exec(stmt); err != nil {
				return fmt.Errorf("llmtest: exec DDL %q: %w", stmt, err)
			}
		}
	}

	return nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// Pure assertion helpers (no DB, no raw SQL)
// ═══════════════════════════════════════════════════════════════════════════════

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

// ═══════════════════════════════════════════════════════════════════════════════
// testPersister — backed by compiled query ASTs
// ═══════════════════════════════════════════════════════════════════════════════

// testPersister is a simple Persister backed by a *sql.DB with the
// llm_conversations and llm_messages tables. It uses compiled query ASTs
// so it does not depend on generated querydef code and is dialect-portable.
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

	sqlStr, paramOrder := compileSQL(insertConversationAST)
	args := mapParams(paramOrder, map[string]any{
		"public_id":    params.PublicID,
		"job_id":       params.JobID,
		"channel_name": params.ChannelName,
		"account_id":   accountID,
		"provider":     params.Provider,
		"model":        params.Model,
		"status":       string(params.Status),
		"started_at":   startedAt.Format("2006-01-02T15:04:05.000Z07:00"),
	})

	result, err := p.db.ExecContext(ctx, sqlStr, args...)
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
		s := params.CompletedAt.Format("2006-01-02T15:04:05.000Z07:00")
		completedAt = &s
	}

	var errorMessage *string
	if params.ErrorMessage != "" {
		errorMessage = &params.ErrorMessage
	}

	sqlStr, paramOrder := compileSQL(updateConversationAST)
	args := mapParams(paramOrder, map[string]any{
		"status":              string(params.Status),
		"total_input_tokens":  params.InputTokens,
		"total_output_tokens": params.OutputTokens,
		"tool_call_count":     params.ToolCallCount,
		"completed_at":        completedAt,
		"error_message":       errorMessage,
		"id":                  params.ID,
	})

	_, err := p.db.ExecContext(ctx, sqlStr, args...)
	if err != nil {
		return fmt.Errorf("testPersister.UpdateConversation: %w", err)
	}
	return nil
}

func (p *testPersister) ListCompletedTools(ctx context.Context, jobID string) ([]string, error) {
	rows, err := p.db.QueryContext(ctx, `
		SELECT DISTINCT m.tool_name
		FROM llm_messages m
		JOIN llm_conversations c ON c.id = m.conversation_id
		WHERE c.job_id = ?
		  AND m.role = 'tool_call'
		  AND m.tool_name IS NOT NULL
	`, jobID)
	if err != nil {
		return nil, fmt.Errorf("testPersister.ListCompletedTools: %w", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("testPersister.ListCompletedTools: scan: %w", err)
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

func (p *testPersister) InsertMessage(ctx context.Context, params llm.InsertMessageParams) error {
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

	publicID := params.PublicID
	if publicID == "" {
		publicID = fmt.Sprintf("msg_%d", time.Now().UnixNano())
	}

	sqlStr, paramOrder := compileSQL(insertMessageAST)
	args := mapParams(paramOrder, map[string]any{
		"public_id":       publicID,
		"conversation_id": params.ConversationID,
		"role":            string(params.Role),
		"content":         content,
		"tool_name":       toolName,
		"tool_call_id":    toolCallID,
		"is_error":        params.IsError,
		"created_at":      createdAt.Format("2006-01-02T15:04:05.000Z07:00"),
	})

	_, err := p.db.ExecContext(ctx, sqlStr, args...)
	if err != nil {
		return fmt.Errorf("testPersister.InsertMessage: %w", err)
	}
	return nil
}

// mapParams maps param names (as returned by the compiler) to values, preserving order.
func mapParams(paramOrder []string, values map[string]any) []any {
	args := make([]any, len(paramOrder))
	for i, name := range paramOrder {
		args[i] = values[name]
	}
	return args
}

// ═══════════════════════════════════════════════════════════════════════════════
// NewTestClientWithDB — wires MockProvider + in-memory SQLite + testPersister
// ═══════════════════════════════════════════════════════════════════════════════

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

// ═══════════════════════════════════════════════════════════════════════════════
// DB assertion helpers — use compiled ASTs
// ═══════════════════════════════════════════════════════════════════════════════

// AssertConversationPersisted verifies that an llm_conversations row with
// the given public_id exists in the database with the expected status.
func AssertConversationPersisted(t *testing.T, db *sql.DB, conversationPublicID string, expectedStatus string) {
	t.Helper()

	sqlStr, paramOrder := compileSQL(selectConversationStatusAST)
	args := mapParams(paramOrder, map[string]any{
		"public_id": conversationPublicID,
	})

	var status string
	err := db.QueryRow(sqlStr, args...).Scan(&status)
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

	sqlStr, paramOrder := compileSQL(countMessagesAST)
	args := mapParams(paramOrder, map[string]any{
		"conversation_id": conversationID,
	})

	var count int
	err := db.QueryRow(sqlStr, args...).Scan(&count)
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

	sqlStr, paramOrder := compileSQL(selectMessageRolesAST)
	args := mapParams(paramOrder, map[string]any{
		"conversation_id": conversationID,
	})

	rows, err := db.Query(sqlStr, args...)
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
