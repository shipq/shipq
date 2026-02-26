package llmtest

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/shipq/shipq/llm"

	_ "modernc.org/sqlite"
)

// ── AssertToolCalled ──────────────────────────────────────────────────────────

func TestAssertToolCalled_Found(t *testing.T) {
	resp := &llm.Response{
		Text: "Here is the weather.",
		ToolCalls: []llm.ToolCallLog{
			{ToolName: "get_weather", Input: json.RawMessage(`{"city":"Tokyo"}`)},
			{ToolName: "calculate", Input: json.RawMessage(`{"expression":"1+1"}`)},
		},
	}

	log := AssertToolCalled(t, resp, "get_weather")
	if log.ToolName != "get_weather" {
		t.Errorf("expected tool name %q, got %q", "get_weather", log.ToolName)
	}
	if string(log.Input) != `{"city":"Tokyo"}` {
		t.Errorf("unexpected input: %s", log.Input)
	}

	// Also find the second tool.
	log2 := AssertToolCalled(t, resp, "calculate")
	if log2.ToolName != "calculate" {
		t.Errorf("expected tool name %q, got %q", "calculate", log2.ToolName)
	}
}

func TestAssertToolCalled_ReturnsCorrectEntry(t *testing.T) {
	output := json.RawMessage(`{"weather":"sunny"}`)
	resp := &llm.Response{
		ToolCalls: []llm.ToolCallLog{
			{
				ToolName: "get_weather",
				Input:    json.RawMessage(`{"city":"Tokyo"}`),
				Output:   output,
				Duration: 42 * time.Millisecond,
			},
		},
	}

	log := AssertToolCalled(t, resp, "get_weather")
	if string(log.Output) != `{"weather":"sunny"}` {
		t.Errorf("unexpected output: %s", log.Output)
	}
	if log.Duration != 42*time.Millisecond {
		t.Errorf("unexpected duration: %v", log.Duration)
	}
}

// ── AssertToolNotCalled ───────────────────────────────────────────────────────

func TestAssertToolNotCalled_Pass(t *testing.T) {
	resp := &llm.Response{
		Text: "No weather.",
		ToolCalls: []llm.ToolCallLog{
			{ToolName: "calculate"},
		},
	}
	// Should not fail — get_weather is not in the tool calls.
	AssertToolNotCalled(t, resp, "get_weather")
}

func TestAssertToolNotCalled_EmptyToolCalls(t *testing.T) {
	resp := &llm.Response{Text: "Just text."}
	AssertToolNotCalled(t, resp, "anything")
}

// ── AssertToolCalledWith ──────────────────────────────────────────────────────

func TestAssertToolCalledWith_Match(t *testing.T) {
	resp := &llm.Response{
		ToolCalls: []llm.ToolCallLog{
			{
				ToolName: "get_weather",
				Input:    json.RawMessage(`{"city":"Tokyo","country":"JP"}`),
			},
		},
	}

	// JSON key order shouldn't matter — deep equal after unmarshal.
	AssertToolCalledWith(t, resp, "get_weather", `{"country":"JP","city":"Tokyo"}`)
}

func TestAssertToolCalledWith_ExactMatch(t *testing.T) {
	resp := &llm.Response{
		ToolCalls: []llm.ToolCallLog{
			{
				ToolName: "calculate",
				Input:    json.RawMessage(`{"expression":"1 + 1"}`),
			},
		},
	}
	AssertToolCalledWith(t, resp, "calculate", `{"expression":"1 + 1"}`)
}

// ── AssertToolCallCount ───────────────────────────────────────────────────────

func TestAssertToolCallCount_Correct(t *testing.T) {
	resp := &llm.Response{
		ToolCalls: []llm.ToolCallLog{
			{ToolName: "a"},
			{ToolName: "b"},
			{ToolName: "c"},
		},
	}
	AssertToolCallCount(t, resp, 3)
}

func TestAssertToolCallCount_Zero(t *testing.T) {
	resp := &llm.Response{}
	AssertToolCallCount(t, resp, 0)
}

func TestAssertToolCallCount_One(t *testing.T) {
	resp := &llm.Response{
		ToolCalls: []llm.ToolCallLog{{ToolName: "only_one"}},
	}
	AssertToolCallCount(t, resp, 1)
}

// ── AssertResponseContains ────────────────────────────────────────────────────

func TestAssertResponseContains_Pass(t *testing.T) {
	resp := &llm.Response{Text: "The weather in Tokyo is sunny and 22 degrees."}
	AssertResponseContains(t, resp, "sunny")
	AssertResponseContains(t, resp, "Tokyo")
	AssertResponseContains(t, resp, "22 degrees")
}

func TestAssertResponseContains_FullText(t *testing.T) {
	resp := &llm.Response{Text: "exact match"}
	AssertResponseContains(t, resp, "exact match")
}

func TestAssertResponseContains_EmptySubstring(t *testing.T) {
	resp := &llm.Response{Text: "anything"}
	AssertResponseContains(t, resp, "") // empty substring is always contained
}

// ── ValidateSchema ────────────────────────────────────────────────────────────

func TestValidateSchema_ValidInput(t *testing.T) {
	td := llm.ToolDef{
		Name: "test_tool",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"city": {"type": "string"},
				"country": {"type": "string"}
			},
			"required": ["city", "country"],
			"additionalProperties": false
		}`),
	}

	ValidateSchema(t, td, `{"city":"Tokyo","country":"JP"}`)
}

func TestValidateSchema_OptionalFields(t *testing.T) {
	td := llm.ToolDef{
		Name: "test_tool",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"city": {"type": "string"},
				"country": {"type": "string"}
			},
			"required": ["city"],
			"additionalProperties": false
		}`),
	}

	// country is optional; only city is required.
	ValidateSchema(t, td, `{"city":"Tokyo"}`)
}

func TestValidateSchema_AllFieldsPresent(t *testing.T) {
	td := llm.ToolDef{
		Name: "test_tool",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"x": {"type": "integer"},
				"y": {"type": "integer"}
			},
			"required": ["x", "y"],
			"additionalProperties": false
		}`),
	}

	ValidateSchema(t, td, `{"x":1,"y":2}`)
}

// ── CallTool ──────────────────────────────────────────────────────────────────

func TestCallTool_Valid(t *testing.T) {
	type Input struct {
		X int `json:"x"`
	}
	type Output struct {
		Result int `json:"result"`
	}

	app := llm.NewApp()
	app.Tool("double", "Double a number", func(_ context.Context, in *Input) (*Output, error) {
		return &Output{Result: in.X * 2}, nil
	})

	td := app.Registry().Tools[0]
	result := CallTool(t, td, `{"x":21}`)

	var out Output
	if err := json.Unmarshal(result, &out); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}
	if out.Result != 42 {
		t.Errorf("got %d, want 42", out.Result)
	}
}

func TestCallTool_WithContext(t *testing.T) {
	type Input struct {
		Name string `json:"name"`
	}
	type Output struct {
		Greeting string `json:"greeting"`
	}

	app := llm.NewApp()
	app.Tool("greet", "Greet someone", func(ctx context.Context, in *Input) (*Output, error) {
		return &Output{Greeting: "Hello, " + in.Name + "!"}, nil
	})

	td := app.Registry().Tools[0]
	result := CallTool(t, td, `{"name":"World"}`)

	var out Output
	if err := json.Unmarshal(result, &out); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}
	if out.Greeting != "Hello, World!" {
		t.Errorf("got %q, want %q", out.Greeting, "Hello, World!")
	}
}

func TestCallTool_BadJSON_FailsAtToolLevel(t *testing.T) {
	type Input struct {
		X int `json:"x"`
	}
	type Output struct {
		Result int `json:"result"`
	}

	app := llm.NewApp()
	app.Tool("double", "Double a number", func(_ context.Context, in *Input) (*Output, error) {
		return &Output{Result: in.X * 2}, nil
	})

	td := app.Registry().Tools[0]

	// Verify that the tool func itself returns an error with bad JSON.
	_, err := td.Func(context.Background(), []byte(`{not valid json}`))
	if err == nil {
		t.Error("expected error from tool func with invalid JSON")
	}
}

// ── CallToolExpectError ───────────────────────────────────────────────────────

func TestCallToolExpectError(t *testing.T) {
	type Input struct{}
	type Output struct{}

	app := llm.NewApp()
	app.Tool("broken", "Always fails", func(_ context.Context, in *Input) (*Output, error) {
		return nil, errors.New("tool broke")
	})

	td := app.Registry().Tools[0]
	err := CallToolExpectError(t, td, `{}`)
	if err.Error() != "tool broke" {
		t.Errorf("got error %q, want %q", err.Error(), "tool broke")
	}
}

func TestCallToolExpectError_CustomError(t *testing.T) {
	type Input struct {
		Divisor int `json:"divisor"`
	}
	type Output struct {
		Result float64 `json:"result"`
	}

	app := llm.NewApp()
	app.Tool("divide", "Divide 100 by input", func(_ context.Context, in *Input) (*Output, error) {
		if in.Divisor == 0 {
			return nil, errors.New("division by zero")
		}
		return &Output{Result: 100.0 / float64(in.Divisor)}, nil
	})

	td := app.Registry().Tools[0]
	err := CallToolExpectError(t, td, `{"divisor":0}`)
	if err.Error() != "division by zero" {
		t.Errorf("got error %q, want %q", err.Error(), "division by zero")
	}
}

// ── NewTestClient ─────────────────────────────────────────────────────────────

func TestNewTestClient_Works(t *testing.T) {
	mock := NewMockProvider("mock", "mock-model").
		Respond("Hello!")

	client := NewTestClient(t, mock)

	resp, err := client.Chat(context.Background(), "Hi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Text != "Hello!" {
		t.Errorf("got %q, want %q", resp.Text, "Hello!")
	}
	// No persister → empty conversation ID.
	if resp.ConversationID != "" {
		t.Errorf("expected empty ConversationID without persister, got %q", resp.ConversationID)
	}
}

func TestNewTestClient_WithOptions(t *testing.T) {
	mock := NewMockProvider("mock", "mock-model").
		Respond("System acknowledged!")

	client := NewTestClient(t, mock, llm.WithSystem("You are a test assistant."))

	resp, err := client.Chat(context.Background(), "Hi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Text != "System acknowledged!" {
		t.Errorf("got %q, want %q", resp.Text, "System acknowledged!")
	}

	// Verify the system prompt was forwarded.
	calls := mock.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].System != "You are a test assistant." {
		t.Errorf("system = %q, want %q", calls[0].System, "You are a test assistant.")
	}
}

// ── NewTestClientWithDB ───────────────────────────────────────────────────────

func TestNewTestClientWithDB(t *testing.T) {
	mock := NewMockProvider("mock", "mock-model").
		Respond("Persisted response!")

	client, db := NewTestClientWithDB(t, mock)

	// Verify tables exist by querying them.
	countConvSQL, _ := compileSQL(countConversationsAST)
	countMsgSQL, _ := compileSQL(countAllMessagesAST)

	var count int
	err := db.QueryRow(countConvSQL).Scan(&count)
	if err != nil {
		t.Fatalf("llm_conversations table not accessible: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 conversations initially, got %d", count)
	}

	err = db.QueryRow(countMsgSQL).Scan(&count)
	if err != nil {
		t.Fatalf("llm_messages table not accessible: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 messages initially, got %d", count)
	}

	// Run a conversation and verify persistence.
	resp, err := client.Chat(context.Background(), "Hello")
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.Text != "Persisted response!" {
		t.Errorf("got %q, want %q", resp.Text, "Persisted response!")
	}
	if resp.ConversationID == "" {
		t.Error("expected non-empty ConversationID when persister is wired")
	}

	// Verify conversation row was created.
	err = db.QueryRow(countConvSQL).Scan(&count)
	if err != nil {
		t.Fatalf("query conversations: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 conversation, got %d", count)
	}

	// Verify messages were persisted (user + assistant = 2).
	err = db.QueryRow(countMsgSQL).Scan(&count)
	if err != nil {
		t.Fatalf("query messages: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 messages (user + assistant), got %d", count)
	}
}

func TestNewTestClientWithDB_WithToolCalls(t *testing.T) {
	mock := NewMockProvider("mock", "mock-model").
		RespondWithToolCall("tc_1", "get_weather", `{"city":"Tokyo"}`).
		Respond("The weather is sunny!")

	type WeatherInput struct {
		City string `json:"city" desc:"City"`
	}
	type WeatherOutput struct {
		Weather string `json:"weather"`
	}

	app := llm.NewApp()
	app.Tool("get_weather", "Get weather", func(_ context.Context, in *WeatherInput) (*WeatherOutput, error) {
		return &WeatherOutput{Weather: "sunny in " + in.City}, nil
	})

	client, db := NewTestClientWithDB(t, mock, llm.WithTools(app.Registry()))

	resp, err := client.Chat(context.Background(), "Weather in Tokyo?")
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}

	// Verify conversation completed.
	AssertConversationPersisted(t, db, resp.ConversationID, "completed")

	// Get the conversation's internal ID.
	convIDSQL, convIDParamOrder := compileSQL(selectConversationByPublicIDAST)
	convIDArgs := mapParams(convIDParamOrder, map[string]any{
		"public_id": resp.ConversationID,
	})
	var convID int64
	err = db.QueryRow(convIDSQL, convIDArgs...).Scan(&convID)
	if err != nil {
		t.Fatalf("query conversation id: %v", err)
	}

	// Expect: user, assistant, tool_call, tool_result, assistant
	AssertMessageSequence(t, db, convID, "user", "assistant", "tool_call", "tool_result", "assistant")
	AssertMessageCount(t, db, convID, 5)
}

func TestNewTestClientWithDB_ConversationStatus(t *testing.T) {
	mock := NewMockProvider("mock", "mock-model").
		Respond("All good.")

	client, db := NewTestClientWithDB(t, mock)

	resp, err := client.Chat(context.Background(), "Hi")
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}

	// Verify the final status is "completed".
	statusSQL, statusParamOrder := compileSQL(selectConversationStatusByPublicIDAST)
	statusArgs := mapParams(statusParamOrder, map[string]any{
		"public_id": resp.ConversationID,
	})
	var status string
	err = db.QueryRow(statusSQL, statusArgs...).Scan(&status)
	if err != nil {
		t.Fatalf("query status: %v", err)
	}
	if status != "completed" {
		t.Errorf("got status %q, want %q", status, "completed")
	}
}

func TestNewTestClientWithDB_TokenCounts(t *testing.T) {
	mock := NewMockProvider("mock", "mock-model").
		RespondWithUsage("Answer", 100, 50)

	client, db := NewTestClientWithDB(t, mock)

	resp, err := client.Chat(context.Background(), "Hi")
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}

	tokensSQL, tokensParamOrder := compileSQL(selectConversationTokensByPublicIDAST)
	tokensArgs := mapParams(tokensParamOrder, map[string]any{
		"public_id": resp.ConversationID,
	})
	var inputTokens, outputTokens int
	err = db.QueryRow(tokensSQL, tokensArgs...).Scan(&inputTokens, &outputTokens)
	if err != nil {
		t.Fatalf("query tokens: %v", err)
	}
	if inputTokens != 100 {
		t.Errorf("got input tokens %d, want 100", inputTokens)
	}
	if outputTokens != 50 {
		t.Errorf("got output tokens %d, want 50", outputTokens)
	}
}

// ── AssertConversationPersisted ───────────────────────────────────────────────

func TestAssertConversationPersisted_Found(t *testing.T) {
	db := openTestDB(t)

	// Insert a conversation using compiled AST.
	insertConvTestSQL, insertConvTestParamOrder := compileSQL(insertTestConversationAST)
	_, err := db.Exec(insertConvTestSQL, mapParams(insertConvTestParamOrder, map[string]any{
		"public_id":  "conv_test_123",
		"provider":   "mock",
		"model":      "mock-model",
		"status":     "completed",
		"started_at": time.Now().Format("2006-01-02T15:04:05.000Z07:00"),
	})...)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	AssertConversationPersisted(t, db, "conv_test_123", "completed")
}

func TestAssertConversationPersisted_Running(t *testing.T) {
	db := openTestDB(t)

	insertConvTestSQL, insertConvTestParamOrder := compileSQL(insertTestConversationAST)
	_, err := db.Exec(insertConvTestSQL, mapParams(insertConvTestParamOrder, map[string]any{
		"public_id":  "conv_running",
		"provider":   "mock",
		"model":      "mock-model",
		"status":     "running",
		"started_at": time.Now().Format("2006-01-02T15:04:05.000Z07:00"),
	})...)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	AssertConversationPersisted(t, db, "conv_running", "running")
}

func TestAssertConversationPersisted_Failed(t *testing.T) {
	db := openTestDB(t)

	insertConvErrSQL, insertConvErrParamOrder := compileSQL(insertTestConversationWithErrorAST)
	errMsg := "something went wrong"
	_, err := db.Exec(insertConvErrSQL, mapParams(insertConvErrParamOrder, map[string]any{
		"public_id":     "conv_failed",
		"provider":      "mock",
		"model":         "mock-model",
		"status":        "failed",
		"started_at":    time.Now().Format("2006-01-02T15:04:05.000Z07:00"),
		"error_message": &errMsg,
	})...)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	AssertConversationPersisted(t, db, "conv_failed", "failed")
}

// ── AssertMessageCount ────────────────────────────────────────────────────────

func TestAssertMessageCount_Correct(t *testing.T) {
	db := openTestDB(t)
	convID := insertTestConversation(t, db, "conv_mc")

	insertTestMessage(t, db, convID, "user")
	insertTestMessage(t, db, convID, "assistant")
	insertTestMessage(t, db, convID, "tool_call")

	AssertMessageCount(t, db, convID, 3)
}

func TestAssertMessageCount_Zero(t *testing.T) {
	db := openTestDB(t)
	convID := insertTestConversation(t, db, "conv_mc_zero")

	AssertMessageCount(t, db, convID, 0)
}

func TestAssertMessageCount_Many(t *testing.T) {
	db := openTestDB(t)
	convID := insertTestConversation(t, db, "conv_mc_many")

	for i := 0; i < 10; i++ {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		insertTestMessage(t, db, convID, role)
	}

	AssertMessageCount(t, db, convID, 10)
}

// ── AssertMessageSequence ─────────────────────────────────────────────────────

func TestAssertMessageSequence_Correct(t *testing.T) {
	db := openTestDB(t)
	convID := insertTestConversation(t, db, "conv_ms")

	insertTestMessage(t, db, convID, "user")
	insertTestMessage(t, db, convID, "assistant")
	insertTestMessage(t, db, convID, "tool_call")
	insertTestMessage(t, db, convID, "tool_result")
	insertTestMessage(t, db, convID, "assistant")

	AssertMessageSequence(t, db, convID, "user", "assistant", "tool_call", "tool_result", "assistant")
}

func TestAssertMessageSequence_SimpleConversation(t *testing.T) {
	db := openTestDB(t)
	convID := insertTestConversation(t, db, "conv_ms_simple")

	insertTestMessage(t, db, convID, "user")
	insertTestMessage(t, db, convID, "assistant")

	AssertMessageSequence(t, db, convID, "user", "assistant")
}

func TestAssertMessageSequence_MultipleToolCalls(t *testing.T) {
	db := openTestDB(t)
	convID := insertTestConversation(t, db, "conv_ms_multi")

	insertTestMessage(t, db, convID, "user")
	insertTestMessage(t, db, convID, "assistant")
	insertTestMessage(t, db, convID, "tool_call")
	insertTestMessage(t, db, convID, "tool_result")
	insertTestMessage(t, db, convID, "tool_call")
	insertTestMessage(t, db, convID, "tool_result")
	insertTestMessage(t, db, convID, "assistant")

	AssertMessageSequence(t, db, convID,
		"user", "assistant", "tool_call", "tool_result",
		"tool_call", "tool_result", "assistant")
}

func TestAssertMessageSequence_OrderedByCreatedAt(t *testing.T) {
	db := openTestDB(t)
	convID := insertTestConversation(t, db, "conv_ms_order")

	// Messages are ordered by created_at ASC. Sequential insertions
	// with incrementing timestamps ensure deterministic ordering.
	insertTestMessage(t, db, convID, "user")
	insertTestMessage(t, db, convID, "assistant")
	insertTestMessage(t, db, convID, "assistant")

	AssertMessageSequence(t, db, convID, "user", "assistant", "assistant")
}

// ── Integration: full conversation with DB persistence ────────────────────────

func TestFullConversationWithDBPersistence(t *testing.T) {
	// Simulate: user asks → model calls tool → model responds.
	mock := NewMockProvider("test-provider", "test-model").
		RespondWithToolCall("tc_1", "calculate", `{"expression":"6 * 7"}`).
		RespondWithUsage("The answer is 42.", 200, 100)

	type CalcInput struct {
		Expression string `json:"expression" desc:"Math expression"`
	}
	type CalcOutput struct {
		Result float64 `json:"result"`
	}

	app := llm.NewApp()
	app.Tool("calculate", "Evaluate an expression", func(_ context.Context, in *CalcInput) (*CalcOutput, error) {
		return &CalcOutput{Result: 42}, nil
	})

	client, db := NewTestClientWithDB(t, mock,
		llm.WithTools(app.Registry()),
		llm.WithSystem("You are a calculator."),
	)

	resp, err := client.Chat(context.Background(), "What is 6 * 7?")
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}

	// Verify response.
	AssertResponseContains(t, resp, "42")
	AssertToolCalled(t, resp, "calculate")
	AssertToolCalledWith(t, resp, "calculate", `{"expression":"6 * 7"}`)
	AssertToolCallCount(t, resp, 1)
	AssertToolNotCalled(t, resp, "get_weather")

	// Verify DB persistence.
	AssertConversationPersisted(t, db, resp.ConversationID, "completed")

	convIDSQL, convIDParamOrder := compileSQL(selectConversationByPublicIDAST)
	convIDArgs := mapParams(convIDParamOrder, map[string]any{
		"public_id": resp.ConversationID,
	})
	var convID int64
	err = db.QueryRow(convIDSQL, convIDArgs...).Scan(&convID)
	if err != nil {
		t.Fatalf("query conv id: %v", err)
	}

	// user → assistant (with tool call) → tool_call → tool_result → assistant (final)
	AssertMessageSequence(t, db, convID, "user", "assistant", "tool_call", "tool_result", "assistant")
	AssertMessageCount(t, db, convID, 5)

	// Verify token counts were persisted.
	tokensSQL, tokensParamOrder := compileSQL(selectConversationTokensAST)
	tokensArgs := mapParams(tokensParamOrder, map[string]any{
		"id": convID,
	})
	var inputTokens, outputTokens, toolCallCount int
	err = db.QueryRow(tokensSQL, tokensArgs...).Scan(&inputTokens, &outputTokens, &toolCallCount)
	if err != nil {
		t.Fatalf("query token counts: %v", err)
	}
	if inputTokens != 200 {
		t.Errorf("input tokens = %d, want 200", inputTokens)
	}
	if outputTokens != 100 {
		t.Errorf("output tokens = %d, want 100", outputTokens)
	}
	if toolCallCount != 1 {
		t.Errorf("tool call count = %d, want 1", toolCallCount)
	}

	// Verify provider and model were recorded.
	pmSQL, pmParamOrder := compileSQL(selectConversationProviderModelAST)
	pmArgs := mapParams(pmParamOrder, map[string]any{
		"id": convID,
	})
	var provider, model string
	err = db.QueryRow(pmSQL, pmArgs...).Scan(&provider, &model)
	if err != nil {
		t.Fatalf("query provider/model: %v", err)
	}
	if provider != "test-provider" {
		t.Errorf("provider = %q, want %q", provider, "test-provider")
	}
	if model != "test-model" {
		t.Errorf("model = %q, want %q", model, "test-model")
	}
}

// ── Test infrastructure ───────────────────────────────────────────────────────

// openTestDB opens an in-memory SQLite database with LLM tables created.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := createLLMTables(db); err != nil {
		t.Fatalf("create tables: %v", err)
	}
	return db
}

// insertTestConversation inserts a conversation row and returns its ID.
func insertTestConversation(t *testing.T, db *sql.DB, publicID string) int64 {
	t.Helper()
	sqlStr, paramOrder := compileSQL(insertTestConversationAST)
	result, err := db.Exec(sqlStr, mapParams(paramOrder, map[string]any{
		"public_id":  publicID,
		"provider":   "mock",
		"model":      "mock-model",
		"status":     "running",
		"started_at": time.Now().Format("2006-01-02T15:04:05.000Z07:00"),
	})...)
	if err != nil {
		t.Fatalf("insert conversation: %v", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("last insert id: %v", err)
	}
	return id
}

// testMsgCounter is a package-level counter used to generate unique,
// monotonically increasing timestamps for insertTestMessage. This ensures
// deterministic ordering by created_at even when messages are inserted
// within the same millisecond.
var testMsgCounter int64

// insertTestMessage inserts a message row for testing.
func insertTestMessage(t *testing.T, db *sql.DB, conversationID int64, role string) {
	t.Helper()
	testMsgCounter++
	// Use a monotonically increasing base time so that created_at ordering
	// is deterministic regardless of wall-clock resolution.
	ts := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(testMsgCounter) * time.Millisecond)
	sqlStr, paramOrder := compileSQL(insertTestMessageAST)
	_, err := db.Exec(sqlStr, mapParams(paramOrder, map[string]any{
		"public_id":       fmt.Sprintf("msg_%d_%d", conversationID, testMsgCounter),
		"conversation_id": conversationID,
		"role":            role,
		"content":         "test content",
		"created_at":      ts.Format("2006-01-02T15:04:05.000Z07:00"),
	})...)
	if err != nil {
		t.Fatalf("insert message: %v", err)
	}
}
