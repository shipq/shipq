package llmtest

import (
	"context"
	"errors"
	"testing"

	"github.com/shipq/shipq/llm"
)

func TestMockProvider_SingleTextResponse(t *testing.T) {
	mock := NewMockProvider("mock", "mock-model").
		Respond("Hello! How can I help you?")

	resp, err := mock.Send(context.Background(), &llm.ProviderRequest{
		System:   "You are helpful.",
		Messages: []llm.ProviderMessage{{Role: llm.RoleUser, Text: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Text != "Hello! How can I help you?" {
		t.Errorf("got text %q, want %q", resp.Text, "Hello! How can I help you?")
	}
	if !resp.Done {
		t.Error("expected Done=true for text-only response")
	}
	if len(resp.ToolCalls) != 0 {
		t.Errorf("expected no tool calls, got %d", len(resp.ToolCalls))
	}
}

func TestMockProvider_ToolCallResponse(t *testing.T) {
	mock := NewMockProvider("mock", "mock-model").
		RespondWithToolCall("tc_1", "get_weather", `{"city":"Tokyo"}`)

	resp, err := mock.Send(context.Background(), &llm.ProviderRequest{
		Messages: []llm.ProviderMessage{{Role: llm.RoleUser, Text: "Weather?"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Done {
		t.Error("expected Done=false for tool call response")
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	tc := resp.ToolCalls[0]
	if tc.ID != "tc_1" {
		t.Errorf("got tool call ID %q, want %q", tc.ID, "tc_1")
	}
	if tc.ToolName != "get_weather" {
		t.Errorf("got tool name %q, want %q", tc.ToolName, "get_weather")
	}
	if string(tc.ArgsJSON) != `{"city":"Tokyo"}` {
		t.Errorf("got args %s, want %s", tc.ArgsJSON, `{"city":"Tokyo"}`)
	}
}

func TestMockProvider_MultipleResponses(t *testing.T) {
	mock := NewMockProvider("mock", "mock-model").
		Respond("Let me check the weather.").
		RespondWithToolCall("tc_1", "get_weather", `{"city":"Tokyo"}`).
		Respond("The weather in Tokyo is sunny.")

	ctx := context.Background()
	req := &llm.ProviderRequest{
		Messages: []llm.ProviderMessage{{Role: llm.RoleUser, Text: "Weather?"}},
	}

	// First call: text response
	resp1, err := mock.Send(ctx, req)
	if err != nil {
		t.Fatalf("call 1: unexpected error: %v", err)
	}
	if resp1.Text != "Let me check the weather." {
		t.Errorf("call 1: got text %q, want %q", resp1.Text, "Let me check the weather.")
	}
	if !resp1.Done {
		t.Error("call 1: expected Done=true")
	}

	// Second call: tool call response
	resp2, err := mock.Send(ctx, req)
	if err != nil {
		t.Fatalf("call 2: unexpected error: %v", err)
	}
	if len(resp2.ToolCalls) != 1 {
		t.Fatalf("call 2: expected 1 tool call, got %d", len(resp2.ToolCalls))
	}
	if resp2.Done {
		t.Error("call 2: expected Done=false")
	}

	// Third call: text response
	resp3, err := mock.Send(ctx, req)
	if err != nil {
		t.Fatalf("call 3: unexpected error: %v", err)
	}
	if resp3.Text != "The weather in Tokyo is sunny." {
		t.Errorf("call 3: got text %q, want %q", resp3.Text, "The weather in Tokyo is sunny.")
	}
	if !resp3.Done {
		t.Error("call 3: expected Done=true")
	}
}

func TestMockProvider_EmptyQueue(t *testing.T) {
	mock := NewMockProvider("mock", "mock-model")

	_, err := mock.Send(context.Background(), &llm.ProviderRequest{
		Messages: []llm.ProviderMessage{{Role: llm.RoleUser, Text: "Hi"}},
	})
	if err == nil {
		t.Fatal("expected error when queue is empty, got nil")
	}
	if want := "no more scripted responses"; !containsSubstring(err.Error(), want) {
		t.Errorf("error %q does not contain %q", err.Error(), want)
	}
}

func TestMockProvider_CapturesRequests(t *testing.T) {
	mock := NewMockProvider("test-provider", "test-model").
		Respond("Response 1").
		Respond("Response 2")

	if mock.Name() != "test-provider" {
		t.Errorf("Name() = %q, want %q", mock.Name(), "test-provider")
	}
	if mock.ModelName() != "test-model" {
		t.Errorf("ModelName() = %q, want %q", mock.ModelName(), "test-model")
	}

	ctx := context.Background()

	// First request
	_, err := mock.Send(ctx, &llm.ProviderRequest{
		System: "System prompt 1",
		Messages: []llm.ProviderMessage{
			{Role: llm.RoleUser, Text: "Hello"},
		},
		Tools: []llm.ToolDef{{Name: "tool_a"}},
	})
	if err != nil {
		t.Fatalf("call 1: %v", err)
	}

	// Second request
	_, err = mock.Send(ctx, &llm.ProviderRequest{
		System: "System prompt 2",
		Messages: []llm.ProviderMessage{
			{Role: llm.RoleUser, Text: "Goodbye"},
			{Role: llm.RoleAssistant, Text: "See you!"},
		},
	})
	if err != nil {
		t.Fatalf("call 2: %v", err)
	}

	calls := mock.Calls()
	if len(calls) != 2 {
		t.Fatalf("expected 2 captured calls, got %d", len(calls))
	}

	// Verify first call
	if calls[0].System != "System prompt 1" {
		t.Errorf("call 0: system = %q, want %q", calls[0].System, "System prompt 1")
	}
	if len(calls[0].Messages) != 1 {
		t.Fatalf("call 0: expected 1 message, got %d", len(calls[0].Messages))
	}
	if calls[0].Messages[0].Text != "Hello" {
		t.Errorf("call 0: message text = %q, want %q", calls[0].Messages[0].Text, "Hello")
	}
	if len(calls[0].Tools) != 1 {
		t.Fatalf("call 0: expected 1 tool, got %d", len(calls[0].Tools))
	}
	if calls[0].Tools[0].Name != "tool_a" {
		t.Errorf("call 0: tool name = %q, want %q", calls[0].Tools[0].Name, "tool_a")
	}

	// Verify second call
	if calls[1].System != "System prompt 2" {
		t.Errorf("call 1: system = %q, want %q", calls[1].System, "System prompt 2")
	}
	if len(calls[1].Messages) != 2 {
		t.Fatalf("call 1: expected 2 messages, got %d", len(calls[1].Messages))
	}
	if len(calls[1].Tools) != 0 {
		t.Errorf("call 1: expected 0 tools, got %d", len(calls[1].Tools))
	}
}

func TestMockProvider_ErrorResponse(t *testing.T) {
	testErr := errors.New("provider exploded")
	mock := NewMockProvider("mock", "mock-model").
		RespondWithError(testErr)

	_, err := mock.Send(context.Background(), &llm.ProviderRequest{
		Messages: []llm.ProviderMessage{{Role: llm.RoleUser, Text: "Hi"}},
	})
	if !errors.Is(err, testErr) {
		t.Errorf("expected error %v, got %v", testErr, err)
	}

	// The request should still be captured even when an error is returned.
	calls := mock.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 captured call even on error, got %d", len(calls))
	}
}

func TestMockProvider_UsageResponse(t *testing.T) {
	mock := NewMockProvider("mock", "mock-model").
		RespondWithUsage("Answer", 100, 50)

	resp, err := mock.Send(context.Background(), &llm.ProviderRequest{
		Messages: []llm.ProviderMessage{{Role: llm.RoleUser, Text: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Text != "Answer" {
		t.Errorf("got text %q, want %q", resp.Text, "Answer")
	}
	if resp.Usage.InputTokens != 100 {
		t.Errorf("got input tokens %d, want %d", resp.Usage.InputTokens, 100)
	}
	if resp.Usage.OutputTokens != 50 {
		t.Errorf("got output tokens %d, want %d", resp.Usage.OutputTokens, 50)
	}
}

func TestMockProvider_MultipleToolCalls(t *testing.T) {
	calls := []llm.ToolCall{
		{ID: "tc_1", ToolName: "get_weather", ArgsJSON: []byte(`{"city":"Tokyo"}`)},
		{ID: "tc_2", ToolName: "calculate", ArgsJSON: []byte(`{"expression":"1+1"}`)},
	}
	mock := NewMockProvider("mock", "mock-model").
		RespondWithMultipleToolCalls(calls...)

	resp, err := mock.Send(context.Background(), &llm.ProviderRequest{
		Messages: []llm.ProviderMessage{{Role: llm.RoleUser, Text: "Weather and math"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Done {
		t.Error("expected Done=false")
	}
	if len(resp.ToolCalls) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].ToolName != "get_weather" {
		t.Errorf("tool call 0: got %q, want %q", resp.ToolCalls[0].ToolName, "get_weather")
	}
	if resp.ToolCalls[1].ToolName != "calculate" {
		t.Errorf("tool call 1: got %q, want %q", resp.ToolCalls[1].ToolName, "calculate")
	}
}

func TestMockProvider_SendStream(t *testing.T) {
	mock := NewMockProvider("mock", "mock-model").
		Respond("Hi!")

	ch, err := mock.SendStream(context.Background(), &llm.ProviderRequest{
		Messages: []llm.ProviderMessage{{Role: llm.RoleUser, Text: "Hello"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var textChunks []string
	var gotDone bool
	var usage *llm.Usage

	for evt := range ch {
		switch evt.Type {
		case llm.StreamTextDelta:
			textChunks = append(textChunks, evt.Text)
		case llm.StreamDone:
			gotDone = true
			usage = evt.Usage
		}
	}

	// "Hi!" should be streamed character-by-character: "H", "i", "!"
	if len(textChunks) != 3 {
		t.Fatalf("expected 3 text delta events (char-by-char), got %d: %v", len(textChunks), textChunks)
	}
	if textChunks[0] != "H" || textChunks[1] != "i" || textChunks[2] != "!" {
		t.Errorf("unexpected text chunks: %v", textChunks)
	}

	if !gotDone {
		t.Error("expected StreamDone event")
	}
	if usage == nil {
		t.Fatal("expected non-nil usage in StreamDone event")
	}

	// Verify request was captured
	calls := mock.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 captured call, got %d", len(calls))
	}
}

func TestMockProvider_SendStreamToolCall(t *testing.T) {
	mock := NewMockProvider("mock", "mock-model").
		RespondWithToolCall("tc_1", "get_weather", `{"city":"London"}`)

	ch, err := mock.SendStream(context.Background(), &llm.ProviderRequest{
		Messages: []llm.ProviderMessage{{Role: llm.RoleUser, Text: "Weather?"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var toolCallStarts []llm.ToolCall
	var gotDone bool

	for evt := range ch {
		switch evt.Type {
		case llm.StreamToolCallStart:
			if evt.ToolCall != nil {
				toolCallStarts = append(toolCallStarts, *evt.ToolCall)
			}
		case llm.StreamDone:
			gotDone = true
		}
	}

	if len(toolCallStarts) != 1 {
		t.Fatalf("expected 1 ToolCallStart event, got %d", len(toolCallStarts))
	}
	tc := toolCallStarts[0]
	if tc.ID != "tc_1" {
		t.Errorf("tool call ID = %q, want %q", tc.ID, "tc_1")
	}
	if tc.ToolName != "get_weather" {
		t.Errorf("tool name = %q, want %q", tc.ToolName, "get_weather")
	}
	if string(tc.ArgsJSON) != `{"city":"London"}` {
		t.Errorf("args = %s, want %s", tc.ArgsJSON, `{"city":"London"}`)
	}
	if !gotDone {
		t.Error("expected StreamDone event")
	}
}

func TestMockProvider_SendStream_EmptyQueue(t *testing.T) {
	mock := NewMockProvider("mock", "mock-model")

	_, err := mock.SendStream(context.Background(), &llm.ProviderRequest{
		Messages: []llm.ProviderMessage{{Role: llm.RoleUser, Text: "Hi"}},
	})
	if err == nil {
		t.Fatal("expected error when queue is empty, got nil")
	}
}

func TestMockProvider_SendStream_ErrorResponse(t *testing.T) {
	testErr := errors.New("stream exploded")
	mock := NewMockProvider("mock", "mock-model").
		RespondWithError(testErr)

	_, err := mock.SendStream(context.Background(), &llm.ProviderRequest{
		Messages: []llm.ProviderMessage{{Role: llm.RoleUser, Text: "Hi"}},
	})
	if !errors.Is(err, testErr) {
		t.Errorf("expected error %v, got %v", testErr, err)
	}
}

func TestMockProvider_SendStream_FullConversationLoop(t *testing.T) {
	// Simulate: user asks weather → model calls tool → model responds with answer
	mock := NewMockProvider("mock", "mock-model").
		RespondWithToolCall("tc_1", "get_weather", `{"city":"Tokyo"}`).
		Respond("The weather in Tokyo is sunny!")

	// Build a tool registry with a simple weather tool.
	app := llm.NewApp()

	type WeatherInput struct {
		City string `json:"city" desc:"City name"`
	}
	type WeatherOutput struct {
		Weather string `json:"weather"`
	}

	app.Tool("get_weather", "Get weather", func(_ context.Context, in *WeatherInput) (*WeatherOutput, error) {
		return &WeatherOutput{Weather: "sunny in " + in.City}, nil
	})

	client := llm.NewClient(mock,
		llm.WithTools(app.Registry()),
		llm.WithSystem("You are helpful."),
	)

	resp, err := client.Chat(context.Background(), "What's the weather in Tokyo?")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Text != "The weather in Tokyo is sunny!" {
		t.Errorf("got text %q, want %q", resp.Text, "The weather in Tokyo is sunny!")
	}

	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call log, got %d", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].ToolName != "get_weather" {
		t.Errorf("tool call name = %q, want %q", resp.ToolCalls[0].ToolName, "get_weather")
	}

	// Verify two calls were made to the mock.
	calls := mock.Calls()
	if len(calls) != 2 {
		t.Fatalf("expected 2 provider calls (tool call + final), got %d", len(calls))
	}

	// First call should have the system prompt.
	if calls[0].System != "You are helpful." {
		t.Errorf("first call system = %q, want %q", calls[0].System, "You are helpful.")
	}
}

// containsSubstring checks if s contains sub.
func containsSubstring(s, sub string) bool {
	return len(s) >= len(sub) && searchSubstring(s, sub)
}

func searchSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
