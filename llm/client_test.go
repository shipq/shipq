package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/shipq/shipq/channel"
	"github.com/shipq/shipq/dag"
)

// ── mock provider ──────────────────────────────────────────────────────────────

// mockProvider implements Provider for testing the conversation loop.
// Each call to Send pops the next response from the Responses slice.
type mockProvider struct {
	mu        sync.Mutex
	pName     string
	pModel    string
	Responses []*ProviderResponse // popped in order by Send
	Requests  []*ProviderRequest  // recorded by Send
	sendErr   error               // if set, Send always returns this error
	streamFn  func(ctx context.Context, req *ProviderRequest) (<-chan StreamEvent, error)
}

func newMockProvider(responses ...*ProviderResponse) *mockProvider {
	return &mockProvider{
		pName:     "mock",
		pModel:    "mock-model",
		Responses: responses,
	}
}

func (m *mockProvider) Name() string      { return m.pName }
func (m *mockProvider) ModelName() string { return m.pModel }

func (m *mockProvider) Send(_ context.Context, req *ProviderRequest) (*ProviderResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Requests = append(m.Requests, req)
	if m.sendErr != nil {
		return nil, m.sendErr
	}
	if len(m.Responses) == 0 {
		return nil, fmt.Errorf("mock: no more responses")
	}
	resp := m.Responses[0]
	m.Responses = m.Responses[1:]
	return resp, nil
}

func (m *mockProvider) SendStream(ctx context.Context, req *ProviderRequest) (<-chan StreamEvent, error) {
	if m.streamFn != nil {
		return m.streamFn(ctx, req)
	}
	return nil, ErrStreamingNotSupported
}

// Compile-time check.
var _ Provider = (*mockProvider)(nil)

// ── helper tools ───────────────────────────────────────────────────────────────

func weatherTool() ToolDef {
	return ToolDef{
		Name:        "get_weather",
		Description: "Get the weather for a city",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}},"required":["city"],"additionalProperties":false}`),
		Func: func(_ context.Context, argsJSON []byte) ([]byte, error) {
			var args struct {
				City string `json:"city"`
			}
			if err := json.Unmarshal(argsJSON, &args); err != nil {
				return nil, err
			}
			return json.Marshal(map[string]string{"weather": "sunny in " + args.City})
		},
	}
}

func failingTool(errMsg string) ToolDef {
	return ToolDef{
		Name:        "failing_tool",
		Description: "A tool that always fails",
		InputSchema: json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`),
		Func: func(_ context.Context, _ []byte) ([]byte, error) {
			return nil, errors.New(errMsg)
		},
	}
}

// ── B10: Single-turn conversation (no tool calls) → returns text ───────────────

func TestChatSingleTurnNoToolCalls(t *testing.T) {
	mp := newMockProvider(&ProviderResponse{
		Text:  "Hello! How can I help you?",
		Usage: Usage{InputTokens: 10, OutputTokens: 8},
		Done:  true,
	})

	c := NewClient(mp)
	resp, err := c.Chat(context.Background(), "Hi there")
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.Text != "Hello! How can I help you?" {
		t.Errorf("Text: got %q", resp.Text)
	}
	if resp.Usage.InputTokens != 10 {
		t.Errorf("InputTokens: got %d, want 10", resp.Usage.InputTokens)
	}
	if resp.Usage.OutputTokens != 8 {
		t.Errorf("OutputTokens: got %d, want 8", resp.Usage.OutputTokens)
	}
	if len(resp.ToolCalls) != 0 {
		t.Errorf("ToolCalls: got %d, want 0", len(resp.ToolCalls))
	}
}

// ── B10: Single tool call → dispatches tool, feeds result back ────────────────

func TestChatSingleToolCall(t *testing.T) {
	mp := newMockProvider(
		// Round 1: model requests a tool call.
		&ProviderResponse{
			Text: "",
			ToolCalls: []ToolCall{{
				ID:       "call_1",
				ToolName: "get_weather",
				ArgsJSON: json.RawMessage(`{"city":"Tokyo"}`),
			}},
			Usage: Usage{InputTokens: 15, OutputTokens: 20},
			Done:  false,
		},
		// Round 2: model produces final text after receiving tool result.
		&ProviderResponse{
			Text:  "The weather in Tokyo is sunny!",
			Usage: Usage{InputTokens: 40, OutputTokens: 12},
			Done:  true,
		},
	)

	reg := &Registry{Tools: []ToolDef{weatherTool()}}
	c := NewClient(mp, WithTools(reg))

	resp, err := c.Chat(context.Background(), "What's the weather in Tokyo?")
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.Text != "The weather in Tokyo is sunny!" {
		t.Errorf("Text: got %q", resp.Text)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("ToolCalls: got %d, want 1", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].ToolName != "get_weather" {
		t.Errorf("ToolCalls[0].ToolName: got %q", resp.ToolCalls[0].ToolName)
	}

	// Verify total usage is accumulated.
	if resp.Usage.InputTokens != 55 {
		t.Errorf("InputTokens: got %d, want 55", resp.Usage.InputTokens)
	}
	if resp.Usage.OutputTokens != 32 {
		t.Errorf("OutputTokens: got %d, want 32", resp.Usage.OutputTokens)
	}

	// Verify the second request includes the tool result in history.
	mp.mu.Lock()
	defer mp.mu.Unlock()
	if len(mp.Requests) != 2 {
		t.Fatalf("Requests: got %d, want 2", len(mp.Requests))
	}
	secondReq := mp.Requests[1]
	// History: user, assistant (with tool call), user (with tool result).
	if len(secondReq.Messages) < 3 {
		t.Fatalf("second request messages: got %d, want >= 3", len(secondReq.Messages))
	}
	lastMsg := secondReq.Messages[len(secondReq.Messages)-1]
	if len(lastMsg.ToolResults) != 1 {
		t.Fatalf("tool results in last message: got %d, want 1", len(lastMsg.ToolResults))
	}
	if lastMsg.ToolResults[0].ToolCallID != "call_1" {
		t.Errorf("ToolCallID: got %q, want call_1", lastMsg.ToolResults[0].ToolCallID)
	}
}

// ── B10: Multiple sequential tool-call rounds ─────────────────────────────────

func TestChatMultipleToolCallRounds(t *testing.T) {
	mp := newMockProvider(
		// Round 1: first tool call
		&ProviderResponse{
			ToolCalls: []ToolCall{{
				ID: "call_1", ToolName: "get_weather",
				ArgsJSON: json.RawMessage(`{"city":"Tokyo"}`),
			}},
			Usage: Usage{InputTokens: 10, OutputTokens: 5},
		},
		// Round 2: second tool call
		&ProviderResponse{
			ToolCalls: []ToolCall{{
				ID: "call_2", ToolName: "get_weather",
				ArgsJSON: json.RawMessage(`{"city":"London"}`),
			}},
			Usage: Usage{InputTokens: 20, OutputTokens: 5},
		},
		// Round 3: final text
		&ProviderResponse{
			Text:  "Tokyo is sunny, London is rainy.",
			Usage: Usage{InputTokens: 30, OutputTokens: 10},
			Done:  true,
		},
	)

	reg := &Registry{Tools: []ToolDef{weatherTool()}}
	c := NewClient(mp, WithTools(reg))

	resp, err := c.Chat(context.Background(), "Compare weather in Tokyo and London")
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.Text != "Tokyo is sunny, London is rainy." {
		t.Errorf("Text: got %q", resp.Text)
	}
	if len(resp.ToolCalls) != 2 {
		t.Errorf("ToolCalls: got %d, want 2", len(resp.ToolCalls))
	}
	if resp.Usage.InputTokens != 60 {
		t.Errorf("InputTokens: got %d, want 60", resp.Usage.InputTokens)
	}
	if resp.Usage.OutputTokens != 20 {
		t.Errorf("OutputTokens: got %d, want 20", resp.Usage.OutputTokens)
	}

	mp.mu.Lock()
	defer mp.mu.Unlock()
	if len(mp.Requests) != 3 {
		t.Fatalf("Requests: got %d, want 3", len(mp.Requests))
	}

	// Third request should have full history:
	// user, assistant+toolcall, user+toolresult, assistant+toolcall, user+toolresult
	thirdReq := mp.Requests[2]
	if len(thirdReq.Messages) != 5 {
		t.Errorf("third request messages: got %d, want 5", len(thirdReq.Messages))
	}
}

// ── B10: Parallel tool calls → all tools execute, results collected ───────────

func TestChatParallelToolCalls(t *testing.T) {
	var callCount atomic.Int32

	tool := ToolDef{
		Name:        "slow_tool",
		Description: "A slow tool",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"n":{"type":"integer"}},"additionalProperties":false}`),
		Func: func(_ context.Context, argsJSON []byte) ([]byte, error) {
			callCount.Add(1)
			time.Sleep(20 * time.Millisecond)
			return []byte(`{"ok":true}`), nil
		},
	}

	mp := newMockProvider(
		&ProviderResponse{
			ToolCalls: []ToolCall{
				{ID: "c1", ToolName: "slow_tool", ArgsJSON: json.RawMessage(`{"n":1}`)},
				{ID: "c2", ToolName: "slow_tool", ArgsJSON: json.RawMessage(`{"n":2}`)},
				{ID: "c3", ToolName: "slow_tool", ArgsJSON: json.RawMessage(`{"n":3}`)},
			},
		},
		&ProviderResponse{Text: "done", Done: true},
	)

	reg := &Registry{Tools: []ToolDef{tool}}
	c := NewClient(mp, WithTools(reg))

	start := time.Now()
	resp, err := c.Chat(context.Background(), "run three tools")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.Text != "done" {
		t.Errorf("Text: got %q", resp.Text)
	}
	if callCount.Load() != 3 {
		t.Errorf("callCount: got %d, want 3", callCount.Load())
	}

	// Parallel execution: 3 tools × 20ms each should take ~20-40ms total,
	// not ~60ms+ (sequential).
	if elapsed > 200*time.Millisecond {
		t.Errorf("parallel tool execution took %v, expected < 200ms", elapsed)
	}

	// Verify tool results are sent back in the correct order.
	mp.mu.Lock()
	defer mp.mu.Unlock()
	if len(mp.Requests) < 2 {
		t.Fatalf("Requests: got %d, want >= 2", len(mp.Requests))
	}
	secondReq := mp.Requests[1]
	lastMsg := secondReq.Messages[len(secondReq.Messages)-1]
	if len(lastMsg.ToolResults) != 3 {
		t.Fatalf("tool results: got %d, want 3", len(lastMsg.ToolResults))
	}
	for i, wantID := range []string{"c1", "c2", "c3"} {
		if lastMsg.ToolResults[i].ToolCallID != wantID {
			t.Errorf("result[%d].ToolCallID: got %q, want %q", i, lastMsg.ToolResults[i].ToolCallID, wantID)
		}
	}
}

// ── B10: WithSequentialToolCalls → tools execute one at a time ────────────────

func TestChatSequentialToolCalls(t *testing.T) {
	var order []int
	var mu sync.Mutex

	makeTool := func(n int) ToolDef {
		name := fmt.Sprintf("tool_%d", n)
		return ToolDef{
			Name:        name,
			Description: "test",
			InputSchema: json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`),
			Func: func(_ context.Context, _ []byte) ([]byte, error) {
				mu.Lock()
				order = append(order, n)
				mu.Unlock()
				time.Sleep(10 * time.Millisecond)
				return []byte(`{"ok":true}`), nil
			},
		}
	}

	mp := newMockProvider(
		&ProviderResponse{
			ToolCalls: []ToolCall{
				{ID: "c1", ToolName: "tool_1", ArgsJSON: json.RawMessage(`{}`)},
				{ID: "c2", ToolName: "tool_2", ArgsJSON: json.RawMessage(`{}`)},
				{ID: "c3", ToolName: "tool_3", ArgsJSON: json.RawMessage(`{}`)},
			},
		},
		&ProviderResponse{Text: "done", Done: true},
	)

	reg := &Registry{Tools: []ToolDef{makeTool(1), makeTool(2), makeTool(3)}}
	c := NewClient(mp, WithTools(reg), WithSequentialToolCalls())

	resp, err := c.Chat(context.Background(), "go")
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.Text != "done" {
		t.Errorf("Text: got %q", resp.Text)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(order) != 3 {
		t.Fatalf("order: got %v, want [1 2 3]", order)
	}
	for i, want := range []int{1, 2, 3} {
		if order[i] != want {
			t.Errorf("order[%d]: got %d, want %d", i, order[i], want)
		}
	}
}

// ── B10: SendErrorToModel strategy ────────────────────────────────────────────

func TestChatSendErrorToModel(t *testing.T) {
	mp := newMockProvider(
		// Round 1: model calls the failing tool.
		&ProviderResponse{
			ToolCalls: []ToolCall{{
				ID: "call_1", ToolName: "failing_tool",
				ArgsJSON: json.RawMessage(`{}`),
			}},
			Usage: Usage{InputTokens: 10, OutputTokens: 5},
		},
		// Round 2: model sees the error and produces a final response.
		&ProviderResponse{
			Text:  "Sorry, the tool failed.",
			Usage: Usage{InputTokens: 20, OutputTokens: 10},
			Done:  true,
		},
	)

	reg := &Registry{Tools: []ToolDef{failingTool("something went wrong")}}
	c := NewClient(mp, WithTools(reg)) // default is SendErrorToModel

	resp, err := c.Chat(context.Background(), "do something")
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.Text != "Sorry, the tool failed." {
		t.Errorf("Text: got %q", resp.Text)
	}

	// Verify the error was sent back to the model in a tool result.
	mp.mu.Lock()
	defer mp.mu.Unlock()
	if len(mp.Requests) != 2 {
		t.Fatalf("Requests: got %d, want 2", len(mp.Requests))
	}
	secondReq := mp.Requests[1]
	lastMsg := secondReq.Messages[len(secondReq.Messages)-1]
	if len(lastMsg.ToolResults) != 1 {
		t.Fatalf("tool results: got %d, want 1", len(lastMsg.ToolResults))
	}
	tr := lastMsg.ToolResults[0]
	if !tr.IsError {
		t.Error("expected IsError=true on tool result")
	}
	if !strings.Contains(string(tr.Output), "something went wrong") {
		t.Errorf("tool result output should contain error message, got %q", string(tr.Output))
	}
}

// ── B10: AbortOnToolError strategy → error returned to caller ─────────────────

func TestChatAbortOnToolError(t *testing.T) {
	mp := newMockProvider(
		&ProviderResponse{
			ToolCalls: []ToolCall{{
				ID: "call_1", ToolName: "failing_tool",
				ArgsJSON: json.RawMessage(`{}`),
			}},
		},
	)

	reg := &Registry{Tools: []ToolDef{failingTool("fatal error")}}
	c := NewClient(mp, WithTools(reg), WithErrorStrategy(AbortOnToolError))

	_, err := c.Chat(context.Background(), "do something")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "fatal error") {
		t.Errorf("error should contain 'fatal error', got %q", err.Error())
	}

	// Only one request should have been made (no second round).
	mp.mu.Lock()
	defer mp.mu.Unlock()
	if len(mp.Requests) != 1 {
		t.Errorf("Requests: got %d, want 1", len(mp.Requests))
	}
}

// ── B10: Max iterations exceeded → error ──────────────────────────────────────

func TestChatMaxIterationsExceeded(t *testing.T) {
	// Always return a tool call, never finish.
	responses := make([]*ProviderResponse, 5)
	for i := range responses {
		responses[i] = &ProviderResponse{
			ToolCalls: []ToolCall{{
				ID:       fmt.Sprintf("call_%d", i),
				ToolName: "get_weather",
				ArgsJSON: json.RawMessage(`{"city":"Tokyo"}`),
			}},
			Usage: Usage{InputTokens: 5, OutputTokens: 5},
		}
	}
	mp := newMockProvider(responses...)

	reg := &Registry{Tools: []ToolDef{weatherTool()}}
	c := NewClient(mp, WithTools(reg), WithMaxIterations(3))

	_, err := c.Chat(context.Background(), "loop forever")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "max iterations") {
		t.Errorf("error should mention max iterations, got %q", err.Error())
	}
}

// ── B10: Streaming events published to channel in correct order ───────────────

func TestChatStreamingEventsOrder(t *testing.T) {
	// streamCallCount tracks which call this is (0-indexed).
	var streamCallCount atomic.Int32

	mp := newMockProvider()
	mp.streamFn = func(_ context.Context, req *ProviderRequest) (<-chan StreamEvent, error) {
		callNum := streamCallCount.Add(1)
		ch := make(chan StreamEvent, 10)
		go func() {
			defer close(ch)
			if callNum == 1 {
				// First call: text delta + tool call.
				ch <- StreamEvent{Type: StreamTextDelta, Text: "Let me check..."}
				ch <- StreamEvent{
					Type: StreamToolCallStart,
					ToolCall: &ToolCall{
						ID:       "call_1",
						ToolName: "get_weather",
						ArgsJSON: json.RawMessage(`{"city":"Tokyo"}`),
					},
				}
				ch <- StreamEvent{
					Type:  StreamDone,
					Done:  true,
					Usage: &Usage{InputTokens: 10, OutputTokens: 5},
				}
			} else {
				// Second call: final text.
				ch <- StreamEvent{Type: StreamTextDelta, Text: "It's sunny!"}
				ch <- StreamEvent{
					Type:  StreamDone,
					Done:  true,
					Usage: &Usage{InputTokens: 20, OutputTokens: 8},
				}
			}
		}()
		return ch, nil
	}

	// Use the test channel helpers from stream_test.go.
	mockCh, mt := newTestChannel(t)

	reg := &Registry{Tools: []ToolDef{weatherTool()}}
	c := NewClient(mp, WithTools(reg), WithChannel(mockCh))

	resp, err := c.Chat(context.Background(), "weather in Tokyo?")
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.Text != "It's sunny!" {
		t.Errorf("Text: got %q", resp.Text)
	}

	// Decode published envelopes from the mock transport.
	mt.mu.Lock()
	defer mt.mu.Unlock()

	if len(mt.published) < 5 {
		t.Fatalf("channel publishes: got %d, want >= 5", len(mt.published))
	}

	// Expected order:
	// 1. LLMTextDelta ("Let me check...")
	// 2. LLMToolCallStart
	// 3. LLMToolCallResult
	// 4. LLMTextDelta ("It's sunny!")
	// 5. LLMDone
	wantTypes := []string{
		TypeLLMTextDelta,
		TypeLLMToolCallStart,
		TypeLLMToolCallResult,
		TypeLLMTextDelta,
		TypeLLMDone,
	}

	for i, want := range wantTypes {
		if i >= len(mt.published) {
			t.Errorf("publishes[%d]: missing, want %q", i, want)
			continue
		}
		env := decodeEnvelope(t, mt.published[i].data)
		if env.Type != want {
			t.Errorf("publishes[%d].Type: got %q, want %q", i, env.Type, want)
		}
	}
}

// ── B10: Persistence calls in correct order ───────────────────────────────────

func TestChatPersistenceOrder(t *testing.T) {
	mp := newMockProvider(
		&ProviderResponse{
			ToolCalls: []ToolCall{{
				ID: "call_1", ToolName: "get_weather",
				ArgsJSON: json.RawMessage(`{"city":"Tokyo"}`),
			}},
			Usage: Usage{InputTokens: 10, OutputTokens: 5},
		},
		&ProviderResponse{
			Text:  "It's sunny!",
			Usage: Usage{InputTokens: 20, OutputTokens: 8},
			Done:  true,
		},
	)

	reg := &Registry{Tools: []ToolDef{weatherTool()}}
	persist := newMockPersister()
	c := NewClient(mp, WithTools(reg), WithPersister(persist))

	resp, err := c.Chat(context.Background(), "What's the weather?")
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}

	// Conversation was created.
	if len(persist.conversations) != 1 {
		t.Fatalf("conversations: got %d, want 1", len(persist.conversations))
	}
	if persist.conversations[0].Status != ConversationStatusRunning {
		t.Errorf("initial status: got %q, want running", persist.conversations[0].Status)
	}
	if persist.conversations[0].Provider != "mock" {
		t.Errorf("provider: got %q, want mock", persist.conversations[0].Provider)
	}
	if persist.conversations[0].Model != "mock-model" {
		t.Errorf("model: got %q, want mock-model", persist.conversations[0].Model)
	}

	// Conversation was updated on completion.
	if len(persist.updates) != 1 {
		t.Fatalf("updates: got %d, want 1", len(persist.updates))
	}
	if persist.updates[0].Status != ConversationStatusCompleted {
		t.Errorf("final status: got %q, want completed", persist.updates[0].Status)
	}
	if persist.updates[0].InputTokens != 30 {
		t.Errorf("InputTokens: got %d, want 30", persist.updates[0].InputTokens)
	}
	if persist.updates[0].OutputTokens != 13 {
		t.Errorf("OutputTokens: got %d, want 13", persist.updates[0].OutputTokens)
	}
	if persist.updates[0].ToolCallCount != 1 {
		t.Errorf("ToolCallCount: got %d, want 1", persist.updates[0].ToolCallCount)
	}

	// Message ordering:
	// 1. user message
	// 2. assistant message (first round — has tool call)
	// 3. tool_call message
	// 4. tool_result message
	// 5. assistant message (second round — final text)
	if len(persist.messages) != 5 {
		t.Fatalf("messages: got %d, want 5", len(persist.messages))
	}
	expectedRoles := []MessageRole{
		MessageRoleUser,
		MessageRoleAssistant,
		MessageRoleToolCall,
		MessageRoleToolResult,
		MessageRoleAssistant,
	}
	for i, want := range expectedRoles {
		if persist.messages[i].Role != want {
			t.Errorf("messages[%d].Role: got %q, want %q", i, persist.messages[i].Role, want)
		}
	}

	// tool_call message details.
	tcMsg := persist.messages[2]
	if tcMsg.ToolName != "get_weather" {
		t.Errorf("tool_call.ToolName: got %q", tcMsg.ToolName)
	}
	if tcMsg.ToolCallID != "call_1" {
		t.Errorf("tool_call.ToolCallID: got %q", tcMsg.ToolCallID)
	}

	// tool_result message details.
	trMsg := persist.messages[3]
	if trMsg.ToolName != "get_weather" {
		t.Errorf("tool_result.ToolName: got %q", trMsg.ToolName)
	}
	if trMsg.IsError {
		t.Error("tool_result.IsError should be false")
	}

	// ConversationID set on response.
	if resp.ConversationID == "" {
		t.Error("ConversationID should be non-empty when persister is configured")
	}
}

// ── B10: No persistence calls when persister is nil ───────────────────────────

func TestChatNoPersisterNoPanic(t *testing.T) {
	mp := newMockProvider(&ProviderResponse{
		Text: "hi",
		Done: true,
	})

	c := NewClient(mp)
	resp, err := c.Chat(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.Text != "hi" {
		t.Errorf("Text: got %q", resp.Text)
	}
	if resp.ConversationID != "" {
		t.Errorf("ConversationID should be empty without persister, got %q", resp.ConversationID)
	}
}

// ── B10: Persistence marks conversation as failed on error ────────────────────

func TestChatPersistenceFailedOnError(t *testing.T) {
	mp := newMockProvider()
	mp.sendErr = errors.New("provider exploded")

	persist := newMockPersister()
	c := NewClient(mp, WithPersister(persist))

	_, err := c.Chat(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected error")
	}

	if len(persist.conversations) != 1 {
		t.Fatalf("conversations: got %d, want 1", len(persist.conversations))
	}
	if len(persist.updates) != 1 {
		t.Fatalf("updates: got %d, want 1", len(persist.updates))
	}
	if persist.updates[0].Status != ConversationStatusFailed {
		t.Errorf("status: got %q, want failed", persist.updates[0].Status)
	}
	if persist.updates[0].ErrorMessage == "" {
		t.Error("ErrorMessage should be non-empty for failed conversation")
	}
}

// ── B10: No streaming calls when channel is nil ───────────────────────────────

func TestChatNoChannelNoStreamCalls(t *testing.T) {
	streamCalled := false
	mp := newMockProvider(&ProviderResponse{
		Text: "hello",
		Done: true,
	})
	mp.streamFn = func(_ context.Context, _ *ProviderRequest) (<-chan StreamEvent, error) {
		streamCalled = true
		return nil, ErrStreamingNotSupported
	}

	// No channel → callProvider goes straight to Send (no streaming attempt).
	c := NewClient(mp)
	resp, err := c.Chat(context.Background(), "hi")
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.Text != "hello" {
		t.Errorf("Text: got %q", resp.Text)
	}
	if streamCalled {
		t.Error("SendStream should not be called when channel is nil")
	}
}

// ── B10: Context cancellation mid-loop → clean error ──────────────────────────

func TestChatContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	mp := newMockProvider(
		// Round 1: tool call — during tool execution we cancel the context.
		&ProviderResponse{
			ToolCalls: []ToolCall{{
				ID:       "call_1",
				ToolName: "cancelling_tool",
				ArgsJSON: json.RawMessage(`{}`),
			}},
		},
	)

	tool := ToolDef{
		Name:        "cancelling_tool",
		Description: "cancels the context",
		InputSchema: json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`),
		Func: func(_ context.Context, _ []byte) ([]byte, error) {
			cancel() // cancel while the loop is running
			return nil, context.Canceled
		},
	}

	reg := &Registry{Tools: []ToolDef{tool}}
	c := NewClient(mp, WithTools(reg), WithErrorStrategy(AbortOnToolError))

	_, err := c.Chat(ctx, "go")
	if err == nil {
		t.Fatal("expected error after context cancellation")
	}
	// The error should propagate (either context.Canceled or wrapping it).
	if !strings.Contains(err.Error(), "canceled") && !strings.Contains(err.Error(), "cancelled") {
		t.Errorf("error should mention cancellation, got %q", err.Error())
	}
}

// ── B10: ChatWithHistory requires at least one message ────────────────────────

func TestChatWithHistoryEmpty(t *testing.T) {
	mp := newMockProvider()
	c := NewClient(mp)

	_, err := c.ChatWithHistory(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for empty history")
	}
	if !strings.Contains(err.Error(), "at least one message") {
		t.Errorf("error: got %q", err.Error())
	}
}

// ── B10: System prompt is forwarded to provider ───────────────────────────────

func TestChatSystemPromptForwarded(t *testing.T) {
	mp := newMockProvider(&ProviderResponse{
		Text: "I am a helpful assistant.",
		Done: true,
	})

	c := NewClient(mp, WithSystem("You are a helpful assistant."))
	_, err := c.Chat(context.Background(), "hi")
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}

	mp.mu.Lock()
	defer mp.mu.Unlock()
	if len(mp.Requests) != 1 {
		t.Fatalf("Requests: got %d, want 1", len(mp.Requests))
	}
	if mp.Requests[0].System != "You are a helpful assistant." {
		t.Errorf("System: got %q", mp.Requests[0].System)
	}
}

// ── B10: Tool not found in registry → error sent to model ─────────────────────

func TestChatToolNotFoundSendErrorToModel(t *testing.T) {
	mp := newMockProvider(
		&ProviderResponse{
			ToolCalls: []ToolCall{{
				ID:       "call_1",
				ToolName: "nonexistent_tool",
				ArgsJSON: json.RawMessage(`{}`),
			}},
		},
		&ProviderResponse{
			Text: "I couldn't find that tool.",
			Done: true,
		},
	)

	reg := &Registry{Tools: []ToolDef{weatherTool()}} // only has get_weather
	c := NewClient(mp, WithTools(reg))                // default SendErrorToModel

	resp, err := c.Chat(context.Background(), "use nonexistent tool")
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.Text != "I couldn't find that tool." {
		t.Errorf("Text: got %q", resp.Text)
	}

	// Verify the "not found" error was sent as a tool result.
	mp.mu.Lock()
	defer mp.mu.Unlock()
	if len(mp.Requests) < 2 {
		t.Fatalf("Requests: got %d, want >= 2", len(mp.Requests))
	}
	lastMsg := mp.Requests[1].Messages[len(mp.Requests[1].Messages)-1]
	if len(lastMsg.ToolResults) != 1 {
		t.Fatalf("tool results: got %d, want 1", len(lastMsg.ToolResults))
	}
	if !lastMsg.ToolResults[0].IsError {
		t.Error("expected IsError=true for missing tool")
	}
}

// ── helpers for DAG tests ──────────────────────────────────────────────────────

func toolNames(tools []ToolDef) []string {
	names := make([]string, len(tools))
	for i, t := range tools {
		names[i] = t.Name
	}
	sort.Strings(names)
	return names
}

func assertToolList(t *testing.T, label string, got []string, want []string) {
	t.Helper()
	sortedGot := make([]string, len(got))
	copy(sortedGot, got)
	sort.Strings(sortedGot)
	sortedWant := make([]string, len(want))
	copy(sortedWant, want)
	sort.Strings(sortedWant)
	if len(sortedGot) != len(sortedWant) {
		t.Errorf("%s: got %v, want %v", label, sortedGot, sortedWant)
		return
	}
	for i := range sortedGot {
		if sortedGot[i] != sortedWant[i] {
			t.Errorf("%s: got %v, want %v", label, sortedGot, sortedWant)
			return
		}
	}
}

func noopToolFunc() ToolFunc {
	return func(_ context.Context, _ []byte) ([]byte, error) {
		return json.Marshal(map[string]string{"ok": "true"})
	}
}

// ── DAG filtering unit tests ───────────────────────────────────────────────────

func TestAvailableTools_NoDAG_ReturnsAllTools(t *testing.T) {
	c := &Client{
		registry: &Registry{Tools: []ToolDef{
			{Name: "a"}, {Name: "b"}, {Name: "c"},
		}},
	}
	tools := c.availableTools(map[string]bool{})
	if len(tools) != 3 {
		t.Errorf("expected 3 tools, got %d", len(tools))
	}
}

func TestAvailableTools_WithDAG_FiltersBlockedTools(t *testing.T) {
	g, err := dag.New([]dag.Node[string]{
		{ID: "a", Description: "first"},
		{ID: "b", Description: "second", HardDeps: []string{"a"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	c := &Client{
		registry: &Registry{Tools: []ToolDef{
			{Name: "a"}, {Name: "b"},
		}},
		taskDAG: g,
	}

	// Nothing completed — only "a" should be available.
	tools := c.availableTools(map[string]bool{})
	if len(tools) != 1 || tools[0].Name != "a" {
		t.Errorf("expected [a], got %v", toolNames(tools))
	}

	// After "a" completes — both should be available.
	tools = c.availableTools(map[string]bool{"a": true})
	if len(tools) != 2 {
		t.Errorf("expected [a, b], got %v", toolNames(tools))
	}
}

func TestAvailableTools_UngovernedToolsAlwaysAvailable(t *testing.T) {
	g, err := dag.New([]dag.Node[string]{
		{ID: "a", Description: "governed"},
		{ID: "b", Description: "governed", HardDeps: []string{"a"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	c := &Client{
		registry: &Registry{Tools: []ToolDef{
			{Name: "a"}, {Name: "b"}, {Name: "c"}, // "c" not in DAG
		}},
		taskDAG: g,
	}

	tools := c.availableTools(map[string]bool{})
	names := toolNames(tools)
	// "a" and "c" should be available; "b" is blocked.
	if len(tools) != 2 {
		t.Errorf("expected 2 tools, got %d: %v", len(tools), names)
	}
	assertToolList(t, "ungoverned", names, []string{"a", "c"})
}

func TestAvailableTools_CompletedToolsRemainAvailable(t *testing.T) {
	g, err := dag.New([]dag.Node[string]{
		{ID: "a", Description: "first"},
		{ID: "b", Description: "second", HardDeps: []string{"a"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	c := &Client{
		registry: &Registry{Tools: []ToolDef{
			{Name: "a"}, {Name: "b"},
		}},
		taskDAG: g,
	}

	// After "a" completes, it should still be callable (model might retry).
	tools := c.availableTools(map[string]bool{"a": true})
	names := toolNames(tools)
	if len(tools) != 2 {
		t.Errorf("expected [a, b], got %v", names)
	}
}

func TestAvailableTools_DiamondDAG(t *testing.T) {
	// a → b, a → c, b+c → d
	g, err := dag.New([]dag.Node[string]{
		{ID: "a", Description: "root"},
		{ID: "b", Description: "left", HardDeps: []string{"a"}},
		{ID: "c", Description: "right", HardDeps: []string{"a"}},
		{ID: "d", Description: "join", HardDeps: []string{"b", "c"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	c := &Client{
		registry: &Registry{Tools: []ToolDef{
			{Name: "a"}, {Name: "b"}, {Name: "c"}, {Name: "d"},
		}},
		taskDAG: g,
	}

	// Nothing completed: only "a"
	assertToolList(t, "step0", toolNames(c.availableTools(map[string]bool{})), []string{"a"})

	// "a" done: "a", "b", "c" available; "d" blocked
	assertToolList(t, "step1", toolNames(c.availableTools(map[string]bool{"a": true})), []string{"a", "b", "c"})

	// "a", "b" done: "a", "b", "c" available; "d" still blocked (needs c)
	assertToolList(t, "step2", toolNames(c.availableTools(map[string]bool{"a": true, "b": true})), []string{"a", "b", "c"})

	// "a", "b", "c" done: all four available
	assertToolList(t, "step3", toolNames(c.availableTools(map[string]bool{"a": true, "b": true, "c": true})), []string{"a", "b", "c", "d"})
}

// ── DAG validation tests ───────────────────────────────────────────────────────

func TestNewClient_DAGWithUnknownTool_Panics(t *testing.T) {
	g, err := dag.New([]dag.Node[string]{
		{ID: "nonexistent", Description: "tool that doesn't exist"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for DAG referencing unknown tool")
		}
	}()
	NewClient(newMockProvider(), WithTools(&Registry{
		Tools: []ToolDef{{Name: "real_tool"}},
	}), WithTaskDAG(g))
}

func TestNewClient_DAGWithoutRegistry_NoPanic(t *testing.T) {
	// DAG without registry is fine — validation is skipped.
	g, err := dag.New([]dag.Node[string]{
		{ID: "a", Description: "first"},
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = NewClient(newMockProvider(), WithTaskDAG(g))
}

func TestNewClient_DAGMatchesRegistry_NoPanic(t *testing.T) {
	g, err := dag.New([]dag.Node[string]{
		{ID: "a", Description: "first"},
		{ID: "b", Description: "second", HardDeps: []string{"a"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = NewClient(newMockProvider(), WithTools(&Registry{
		Tools: []ToolDef{{Name: "a"}, {Name: "b"}, {Name: "c"}},
	}), WithTaskDAG(g))
}

// ── DAG system prompt suffix tests ─────────────────────────────────────────────

func TestDagSystemPromptSuffix_NoDAG_EmptyString(t *testing.T) {
	c := &Client{}
	if got := c.dagSystemPromptSuffix(nil); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestDagSystemPromptSuffix_WithDAG_DescribesDependencies(t *testing.T) {
	g, err := dag.New([]dag.Node[string]{
		{ID: "a", Description: "first"},
		{ID: "b", Description: "second", HardDeps: []string{"a"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	c := &Client{taskDAG: g}
	suffix := c.dagSystemPromptSuffix(map[string]bool{})
	if !strings.Contains(suffix, "requires") {
		t.Error("expected suffix to mention requirements")
	}
	if !strings.Contains(suffix, "a") {
		t.Error("expected suffix to mention tool 'a'")
	}
}

func TestDagSystemPromptSuffix_ReflectsCompletedState(t *testing.T) {
	g, err := dag.New([]dag.Node[string]{
		{ID: "a", Description: "first"},
		{ID: "b", Description: "second", HardDeps: []string{"a"}},
		{ID: "c", Description: "third", HardDeps: []string{"b"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	c := &Client{taskDAG: g}

	// Before anything is completed, "a" is available.
	suffix := c.dagSystemPromptSuffix(map[string]bool{})
	if !strings.Contains(suffix, "a") {
		t.Error("expected 'a' in available tools")
	}

	// After "a" is completed, "b" becomes available.
	suffix = c.dagSystemPromptSuffix(map[string]bool{"a": true})
	if !strings.Contains(suffix, "b") {
		t.Error("expected 'b' in available tools after 'a' completed")
	}
}

func TestDagSystemPromptSuffix_SoftDeps(t *testing.T) {
	g, err := dag.New([]dag.Node[string]{
		{ID: "a", Description: "first"},
		{ID: "b", Description: "second", SoftDeps: []string{"a"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	c := &Client{taskDAG: g}
	suffix := c.dagSystemPromptSuffix(map[string]bool{})
	if !strings.Contains(suffix, "benefits from") {
		t.Error("expected suffix to mention soft deps")
	}
}

// ── DAG integration tests: full conversation flow ──────────────────────────────

func TestChat_WithTaskDAG_ToolsFilteredByDependencies(t *testing.T) {
	// Setup: 3 tools, linear chain a → b → c.
	g, err := dag.New([]dag.Node[string]{
		{ID: "a", Description: "first"},
		{ID: "b", Description: "second", HardDeps: []string{"a"}},
		{ID: "c", Description: "third", HardDeps: []string{"b"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	mp := newMockProvider(
		// Turn 1: model sees only tool "a", calls it.
		&ProviderResponse{
			Text:      "calling a",
			ToolCalls: []ToolCall{{ID: "tc1", ToolName: "a", ArgsJSON: json.RawMessage(`{}`)}},
		},
		// Turn 2: model sees "a" + "b", calls "b".
		&ProviderResponse{
			Text:      "calling b",
			ToolCalls: []ToolCall{{ID: "tc2", ToolName: "b", ArgsJSON: json.RawMessage(`{}`)}},
		},
		// Turn 3: model sees "a" + "b" + "c", calls "c".
		&ProviderResponse{
			Text:      "calling c",
			ToolCalls: []ToolCall{{ID: "tc3", ToolName: "c", ArgsJSON: json.RawMessage(`{}`)}},
		},
		// Turn 4: final response.
		&ProviderResponse{Text: "all done", Done: true},
	)

	noop := noopToolFunc()

	client := NewClient(mp,
		WithTools(&Registry{Tools: []ToolDef{
			{Name: "a", Func: noop, InputSchema: json.RawMessage(`{}`)},
			{Name: "b", Func: noop, InputSchema: json.RawMessage(`{}`)},
			{Name: "c", Func: noop, InputSchema: json.RawMessage(`{}`)},
		}}),
		WithTaskDAG(g),
	)

	resp, err := client.Chat(context.Background(), "do all the things")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Text != "all done" {
		t.Errorf("expected 'all done', got %q", resp.Text)
	}

	// Verify tool availability progressed correctly.
	mp.mu.Lock()
	defer mp.mu.Unlock()
	if len(mp.Requests) != 4 {
		t.Fatalf("expected 4 requests, got %d", len(mp.Requests))
	}

	// Collect tool names per request.
	var toolsPerRequest [][]string
	for _, req := range mp.Requests {
		var names []string
		for _, tool := range req.Tools {
			names = append(names, tool.Name)
		}
		toolsPerRequest = append(toolsPerRequest, names)
	}

	// Request 1: only "a"
	assertToolList(t, "request 1", toolsPerRequest[0], []string{"a"})
	// Request 2: "a", "b"
	assertToolList(t, "request 2", toolsPerRequest[1], []string{"a", "b"})
	// Request 3: "a", "b", "c"
	assertToolList(t, "request 3", toolsPerRequest[2], []string{"a", "b", "c"})
	// Request 4: "a", "b", "c" (all completed, all available)
	assertToolList(t, "request 4", toolsPerRequest[3], []string{"a", "b", "c"})
}

func TestChat_WithTaskDAG_BlockedToolCallSendsError(t *testing.T) {
	// Model hallucinates a call to "b" before "a" is done.
	g, err := dag.New([]dag.Node[string]{
		{ID: "a", Description: "first"},
		{ID: "b", Description: "second", HardDeps: []string{"a"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	mp := newMockProvider(
		// Turn 1: model calls "b" (blocked).
		&ProviderResponse{
			Text:      "calling b",
			ToolCalls: []ToolCall{{ID: "tc1", ToolName: "b", ArgsJSON: json.RawMessage(`{}`)}},
		},
		// Turn 2: model calls "a" (correcting itself).
		&ProviderResponse{
			Text:      "calling a",
			ToolCalls: []ToolCall{{ID: "tc2", ToolName: "a", ArgsJSON: json.RawMessage(`{}`)}},
		},
		// Turn 3: model calls "b" (now allowed).
		&ProviderResponse{
			Text:      "calling b again",
			ToolCalls: []ToolCall{{ID: "tc3", ToolName: "b", ArgsJSON: json.RawMessage(`{}`)}},
		},
		// Turn 4: final.
		&ProviderResponse{Text: "done", Done: true},
	)

	noop := noopToolFunc()

	client := NewClient(mp,
		WithTools(&Registry{Tools: []ToolDef{
			{Name: "a", Func: noop, InputSchema: json.RawMessage(`{}`)},
			{Name: "b", Func: noop, InputSchema: json.RawMessage(`{}`)},
		}}),
		WithTaskDAG(g),
	)

	resp, err := client.Chat(context.Background(), "do things")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Text != "done" {
		t.Errorf("expected 'done', got %q", resp.Text)
	}
	// The first tool call should have an error (blocked).
	if len(resp.ToolCalls) < 1 {
		t.Fatal("expected at least one tool call log")
	}
	if resp.ToolCalls[0].Error == nil {
		t.Error("expected error for blocked tool call to 'b'")
	}
	if !strings.Contains(resp.ToolCalls[0].Error.Error(), "prerequisites not met") {
		t.Errorf("expected prerequisites error, got %q", resp.ToolCalls[0].Error.Error())
	}
}

func TestChat_WithTaskDAG_SystemPromptContainsDAGInfo(t *testing.T) {
	g, err := dag.New([]dag.Node[string]{
		{ID: "a", Description: "first"},
		{ID: "b", Description: "second", HardDeps: []string{"a"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	mp := newMockProvider(
		&ProviderResponse{Text: "done", Done: true},
	)

	noop := noopToolFunc()

	client := NewClient(mp,
		WithTools(&Registry{Tools: []ToolDef{
			{Name: "a", Func: noop, InputSchema: json.RawMessage(`{}`)},
			{Name: "b", Func: noop, InputSchema: json.RawMessage(`{}`)},
		}}),
		WithTaskDAG(g),
		WithSystem("Base system prompt."),
	)

	_, err = client.Chat(context.Background(), "hi")
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}

	mp.mu.Lock()
	defer mp.mu.Unlock()
	if len(mp.Requests) < 1 {
		t.Fatal("no requests recorded")
	}
	sys := mp.Requests[0].System
	if !strings.Contains(sys, "Base system prompt.") {
		t.Error("expected base system prompt to be preserved")
	}
	if !strings.Contains(sys, "Tool Ordering") {
		t.Error("expected DAG info in system prompt")
	}
	if !strings.Contains(sys, "requires") {
		t.Error("expected dependency info in system prompt")
	}
}

func TestChat_WithTaskDAG_UngovernedToolsAlwaysPresent(t *testing.T) {
	// Only "a" and "b" are in the DAG. "c" is ungoverned.
	g, err := dag.New([]dag.Node[string]{
		{ID: "a", Description: "first"},
		{ID: "b", Description: "second", HardDeps: []string{"a"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	mp := newMockProvider(
		&ProviderResponse{Text: "done", Done: true},
	)

	noop := noopToolFunc()

	client := NewClient(mp,
		WithTools(&Registry{Tools: []ToolDef{
			{Name: "a", Func: noop, InputSchema: json.RawMessage(`{}`)},
			{Name: "b", Func: noop, InputSchema: json.RawMessage(`{}`)},
			{Name: "c", Func: noop, InputSchema: json.RawMessage(`{}`)},
		}}),
		WithTaskDAG(g),
	)

	_, err = client.Chat(context.Background(), "hi")
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}

	mp.mu.Lock()
	defer mp.mu.Unlock()
	// "a" and "c" should be available; "b" is blocked.
	var names []string
	for _, tool := range mp.Requests[0].Tools {
		names = append(names, tool.Name)
	}
	assertToolList(t, "first request", names, []string{"a", "c"})
}

func TestChat_WithoutDAG_AllToolsAvailable(t *testing.T) {
	// Baseline: no DAG, all tools available.
	mp := newMockProvider(
		&ProviderResponse{Text: "done", Done: true},
	)

	noop := noopToolFunc()

	client := NewClient(mp,
		WithTools(&Registry{Tools: []ToolDef{
			{Name: "a", Func: noop, InputSchema: json.RawMessage(`{}`)},
			{Name: "b", Func: noop, InputSchema: json.RawMessage(`{}`)},
			{Name: "c", Func: noop, InputSchema: json.RawMessage(`{}`)},
		}}),
	)

	_, err := client.Chat(context.Background(), "hi")
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}

	mp.mu.Lock()
	defer mp.mu.Unlock()
	var names []string
	for _, tool := range mp.Requests[0].Tools {
		names = append(names, tool.Name)
	}
	assertToolList(t, "no DAG", names, []string{"a", "b", "c"})
}

// ── computeToolSets tests ──────────────────────────────────────────────────────

func TestComputeToolSets(t *testing.T) {
	g, err := dag.New([]dag.Node[string]{
		{ID: "a", Description: "root"},
		{ID: "b", Description: "mid", HardDeps: []string{"a"}},
		{ID: "c", Description: "leaf", HardDeps: []string{"b"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	c := &Client{taskDAG: g}

	// Nothing completed: "a" available, "b"+"c" blocked.
	avail, done, blocked := c.computeToolSets(map[string]bool{})
	assertToolList(t, "avail-0", avail, []string{"a"})
	if len(done) != 0 {
		t.Errorf("expected 0 done, got %v", done)
	}
	assertToolList(t, "blocked-0", blocked, []string{"b", "c"})

	// "a" completed: "a" done, "b" available, "c" blocked.
	avail, done, blocked = c.computeToolSets(map[string]bool{"a": true})
	assertToolList(t, "avail-1", avail, []string{"b"})
	assertToolList(t, "done-1", done, []string{"a"})
	assertToolList(t, "blocked-1", blocked, []string{"c"})

	// "a"+"b" completed: "c" available.
	avail, done, blocked = c.computeToolSets(map[string]bool{"a": true, "b": true})
	assertToolList(t, "avail-2", avail, []string{"c"})
	assertToolList(t, "done-2", done, []string{"a", "b"})
	if len(blocked) != 0 {
		t.Errorf("expected 0 blocked, got %v", blocked)
	}
}

// ── DAG persistence hydration tests ────────────────────────────────────────────

// mockPersisterWithCompletedTools extends mockPersister with configurable
// ListCompletedTools behavior.
type mockPersisterWithCompletedTools struct {
	mockPersister
	completedTools []string
	completedErr   error
	calledJobID    string
}

func (m *mockPersisterWithCompletedTools) ListCompletedTools(_ context.Context, jobID string) ([]string, error) {
	m.calledJobID = jobID
	return m.completedTools, m.completedErr
}

func TestRun_WithDAG_HydratesCompletedToolsFromPersister(t *testing.T) {
	// DAG: a → b. Persister says "a" was already completed.
	g, err := dag.New([]dag.Node[string]{
		{ID: "a", Description: "first"},
		{ID: "b", Description: "second", HardDeps: []string{"a"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	mp := newMockProvider(
		&ProviderResponse{Text: "done", Done: true},
	)

	noop := noopToolFunc()

	persist := &mockPersisterWithCompletedTools{
		mockPersister:  *newMockPersister(),
		completedTools: []string{"a"},
	}

	// Create a no-op channel to provide a job ID.
	ch := createTestChannel(t, "test-job")

	client := NewClient(mp,
		WithTools(&Registry{Tools: []ToolDef{
			{Name: "a", Func: noop, InputSchema: json.RawMessage(`{}`)},
			{Name: "b", Func: noop, InputSchema: json.RawMessage(`{}`)},
		}}),
		WithTaskDAG(g),
		WithPersister(persist),
		WithChannel(ch),
	)

	_, err = client.Chat(context.Background(), "hi")
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}

	// Both "a" and "b" should be available because "a" was hydrated as completed.
	mp.mu.Lock()
	defer mp.mu.Unlock()
	var names []string
	for _, tool := range mp.Requests[0].Tools {
		names = append(names, tool.Name)
	}
	assertToolList(t, "hydrated", names, []string{"a", "b"})
}

func TestRun_WithDAG_NoPersister_StartsEmpty(t *testing.T) {
	g, err := dag.New([]dag.Node[string]{
		{ID: "a", Description: "first"},
		{ID: "b", Description: "second", HardDeps: []string{"a"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	mp := newMockProvider(
		&ProviderResponse{Text: "done", Done: true},
	)

	noop := noopToolFunc()

	client := NewClient(mp,
		WithTools(&Registry{Tools: []ToolDef{
			{Name: "a", Func: noop, InputSchema: json.RawMessage(`{}`)},
			{Name: "b", Func: noop, InputSchema: json.RawMessage(`{}`)},
		}}),
		WithTaskDAG(g),
		// No persister — should start with empty completed set.
	)

	_, err = client.Chat(context.Background(), "hi")
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}

	mp.mu.Lock()
	defer mp.mu.Unlock()
	var names []string
	for _, tool := range mp.Requests[0].Tools {
		names = append(names, tool.Name)
	}
	// Only "a" available since no hydration happened.
	assertToolList(t, "no persister", names, []string{"a"})
}

func TestRun_WithDAG_NoChannel_StartsEmpty(t *testing.T) {
	g, err := dag.New([]dag.Node[string]{
		{ID: "a", Description: "first"},
		{ID: "b", Description: "second", HardDeps: []string{"a"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	mp := newMockProvider(
		&ProviderResponse{Text: "done", Done: true},
	)

	noop := noopToolFunc()

	persist := &mockPersisterWithCompletedTools{
		mockPersister:  *newMockPersister(),
		completedTools: []string{"a"},
	}

	client := NewClient(mp,
		WithTools(&Registry{Tools: []ToolDef{
			{Name: "a", Func: noop, InputSchema: json.RawMessage(`{}`)},
			{Name: "b", Func: noop, InputSchema: json.RawMessage(`{}`)},
		}}),
		WithTaskDAG(g),
		WithPersister(persist),
		// No channel — no job ID.
	)

	_, err = client.Chat(context.Background(), "hi")
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}

	mp.mu.Lock()
	defer mp.mu.Unlock()
	var names []string
	for _, tool := range mp.Requests[0].Tools {
		names = append(names, tool.Name)
	}
	// Only "a" available (no channel means no hydration).
	assertToolList(t, "no channel", names, []string{"a"})
}

func TestRun_WithDAG_PersisterError_StartsEmpty(t *testing.T) {
	g, err := dag.New([]dag.Node[string]{
		{ID: "a", Description: "first"},
		{ID: "b", Description: "second", HardDeps: []string{"a"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	mp := newMockProvider(
		&ProviderResponse{Text: "done", Done: true},
	)

	noop := noopToolFunc()

	persist := &mockPersisterWithCompletedTools{
		mockPersister: *newMockPersister(),
		completedErr:  errors.New("db connection failed"),
	}

	ch := createTestChannel(t, "test-job")

	client := NewClient(mp,
		WithTools(&Registry{Tools: []ToolDef{
			{Name: "a", Func: noop, InputSchema: json.RawMessage(`{}`)},
			{Name: "b", Func: noop, InputSchema: json.RawMessage(`{}`)},
		}}),
		WithTaskDAG(g),
		WithPersister(persist),
		WithChannel(ch),
	)

	_, err = client.Chat(context.Background(), "hi")
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}

	mp.mu.Lock()
	defer mp.mu.Unlock()
	var names []string
	for _, tool := range mp.Requests[0].Tools {
		names = append(names, tool.Name)
	}
	// Only "a" available (persister error means graceful degradation).
	assertToolList(t, "persister error", names, []string{"a"})
}

// createTestChannel builds a minimal *channel.Channel for tests that need
// a job ID for DAG hydration but don't actually publish anything.
func createTestChannel(t *testing.T, jobID string) *channel.Channel {
	t.Helper()
	tr := testNoopTransport{}
	incoming, cleanup, _ := tr.Subscribe("test", "sub")
	return channel.NewChannel("test", jobID, 0, 0, false, tr, incoming, cleanup)
}

type testNoopTransport struct{}

func (testNoopTransport) Publish(string, []byte) error { return nil }
func (testNoopTransport) Subscribe(string, string) (<-chan []byte, func(), error) {
	return make(chan []byte), func() {}, nil
}
func (testNoopTransport) GenerateConnectionToken(_ string, _ time.Duration) (string, error) {
	return "", nil
}
func (testNoopTransport) GenerateSubscriptionToken(_ string, _ string, _ time.Duration) (string, error) {
	return "", nil
}
func (testNoopTransport) ConnectionURL() string { return "" }
