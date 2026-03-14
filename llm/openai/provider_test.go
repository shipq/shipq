package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shipq/shipq/llm"
)

// ── test helpers ──────────────────────────────────────────────────────────────

func newTestProvider(t *testing.T, handler http.HandlerFunc) (*Provider, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	p := New("test-key", "gpt-4.1",
		WithBaseURL(srv.URL),
		WithHTTPClient(srv.Client()),
	)
	return p, srv
}

func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	return b
}

// sseLines builds a valid SSE body from a list of data payloads.
// Each element is written as "data: <payload>\n\n".
// Pass "[DONE]" as the final payload.
func sseLines(payloads ...string) string {
	var sb strings.Builder
	for _, p := range payloads {
		sb.WriteString("data: ")
		sb.WriteString(p)
		sb.WriteString("\n\n")
	}
	return sb.String()
}

// ── request serialisation ─────────────────────────────────────────────────────

func TestSendRequestBody(t *testing.T) {
	var captured []byte
	p, _ := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		captured = mustReadBody(t, r)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{
				map[string]any{
					"finish_reason": "stop",
					"message": map[string]any{
						"role":    "assistant",
						"content": "Hello!",
					},
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     10,
				"completion_tokens": 5,
			},
		})
	})

	req := &llm.ProviderRequest{
		System: "You are helpful.",
		Messages: []llm.ProviderMessage{
			{Role: llm.RoleUser, Text: "Hi"},
		},
	}
	if _, err := p.Send(context.Background(), req); err != nil {
		t.Fatalf("Send: %v", err)
	}

	var body map[string]any
	if err := json.Unmarshal(captured, &body); err != nil {
		t.Fatalf("unmarshal captured body: %v", err)
	}

	// Model should be set.
	if body["model"] != "gpt-4.1" {
		t.Errorf("model: got %v, want gpt-4.1", body["model"])
	}

	// Messages should have system + user.
	msgs, ok := body["messages"].([]any)
	if !ok {
		t.Fatalf("messages: expected []any, got %T", body["messages"])
	}
	if len(msgs) != 2 {
		t.Fatalf("messages: got %d, want 2", len(msgs))
	}
	sysMsg := msgs[0].(map[string]any)
	if sysMsg["role"] != "system" {
		t.Errorf("msgs[0].role: got %v, want system", sysMsg["role"])
	}
	if sysMsg["content"] != "You are helpful." {
		t.Errorf("msgs[0].content: got %v", sysMsg["content"])
	}
	userMsg := msgs[1].(map[string]any)
	if userMsg["role"] != "user" {
		t.Errorf("msgs[1].role: got %v, want user", userMsg["role"])
	}
}

func TestSendRequestWithTools(t *testing.T) {
	var captured []byte
	p, _ := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		captured = mustReadBody(t, r)
		json.NewEncoder(w).Encode(toolCallResponse("call-1", "get_weather", `{"city":"Tokyo"}`))
	})

	schema := json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}},"required":["city"],"additionalProperties":false}`)
	req := &llm.ProviderRequest{
		Messages: []llm.ProviderMessage{{Role: llm.RoleUser, Text: "Weather?"}},
		Tools: []llm.ToolDef{
			{Name: "get_weather", Description: "Get weather", InputSchema: schema},
		},
	}
	if _, err := p.Send(context.Background(), req); err != nil {
		t.Fatalf("Send: %v", err)
	}

	var body map[string]any
	if err := json.Unmarshal(captured, &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	tools, ok := body["tools"].([]any)
	if !ok || len(tools) != 1 {
		t.Fatalf("tools: got %v", body["tools"])
	}
	tool := tools[0].(map[string]any)
	if tool["type"] != "function" {
		t.Errorf("tools[0].type: got %v, want function", tool["type"])
	}
	fn := tool["function"].(map[string]any)
	if fn["name"] != "get_weather" {
		t.Errorf("function.name: got %v", fn["name"])
	}
	if fn["strict"] != true {
		t.Errorf("function.strict: got %v, want true", fn["strict"])
	}
	// tool_choice should be "auto" when tools are present.
	if body["tool_choice"] != "auto" {
		t.Errorf("tool_choice: got %v, want auto", body["tool_choice"])
	}
}

func TestSendNoToolsOmitsToolChoice(t *testing.T) {
	var captured []byte
	p, _ := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		captured = mustReadBody(t, r)
		json.NewEncoder(w).Encode(textResponse("Hello"))
	})

	req := &llm.ProviderRequest{
		Messages: []llm.ProviderMessage{{Role: llm.RoleUser, Text: "Hi"}},
	}
	if _, err := p.Send(context.Background(), req); err != nil {
		t.Fatalf("Send: %v", err)
	}

	var body map[string]any
	_ = json.Unmarshal(captured, &body)

	if _, present := body["tool_choice"]; present {
		t.Error("tool_choice should be absent when no tools are registered")
	}
	if _, present := body["tools"]; present {
		t.Error("tools should be absent when no tools are registered")
	}
}

// ── response parsing ──────────────────────────────────────────────────────────

func TestSendParsesTextResponse(t *testing.T) {
	p, _ := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(textResponse("The sky is blue."))
	})

	resp, err := p.Send(context.Background(), &llm.ProviderRequest{
		Messages: []llm.ProviderMessage{{Role: llm.RoleUser, Text: "What color is the sky?"}},
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if resp.Text != "The sky is blue." {
		t.Errorf("Text: got %q, want %q", resp.Text, "The sky is blue.")
	}
	if !resp.Done {
		t.Error("Done should be true for a stop response")
	}
	if len(resp.ToolCalls) != 0 {
		t.Errorf("ToolCalls: expected 0, got %d", len(resp.ToolCalls))
	}
}

func TestSendParsesUsage(t *testing.T) {
	p, _ := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{
				map[string]any{
					"finish_reason": "stop",
					"message":       map[string]any{"role": "assistant", "content": "ok"},
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     42,
				"completion_tokens": 17,
			},
		})
	})

	resp, err := p.Send(context.Background(), &llm.ProviderRequest{
		Messages: []llm.ProviderMessage{{Role: llm.RoleUser, Text: "hi"}},
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if resp.Usage.InputTokens != 42 {
		t.Errorf("InputTokens: got %d, want 42", resp.Usage.InputTokens)
	}
	if resp.Usage.OutputTokens != 17 {
		t.Errorf("OutputTokens: got %d, want 17", resp.Usage.OutputTokens)
	}
}

func TestSendParsesToolCall(t *testing.T) {
	// OpenAI returns tool call arguments as a JSON string (requires second Unmarshal).
	p, _ := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(toolCallResponse("call-abc", "get_weather", `{"city":"London"}`))
	})

	resp, err := p.Send(context.Background(), &llm.ProviderRequest{
		Messages: []llm.ProviderMessage{{Role: llm.RoleUser, Text: "Weather?"}},
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("ToolCalls: got %d, want 1", len(resp.ToolCalls))
	}
	tc := resp.ToolCalls[0]
	if tc.ID != "call-abc" {
		t.Errorf("ID: got %q, want call-abc", tc.ID)
	}
	if tc.ToolName != "get_weather" {
		t.Errorf("ToolName: got %q, want get_weather", tc.ToolName)
	}

	// ArgsJSON should be valid JSON that we can parse.
	var args map[string]string
	if err := json.Unmarshal(tc.ArgsJSON, &args); err != nil {
		t.Fatalf("ArgsJSON unmarshal: %v", err)
	}
	if args["city"] != "London" {
		t.Errorf("city: got %q, want London", args["city"])
	}
	if resp.Done {
		t.Error("Done should be false when tool calls are present")
	}
}

func TestSendMultipleToolCalls(t *testing.T) {
	p, _ := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{
				map[string]any{
					"finish_reason": "tool_calls",
					"message": map[string]any{
						"role":    "assistant",
						"content": nil,
						"tool_calls": []any{
							map[string]any{
								"id": "call-1", "type": "function",
								"function": map[string]any{"name": "get_weather", "arguments": `{"city":"Paris"}`},
							},
							map[string]any{
								"id": "call-2", "type": "function",
								"function": map[string]any{"name": "get_time", "arguments": `{"tz":"Europe/Paris"}`},
							},
						},
					},
				},
			},
			"usage": map[string]any{"prompt_tokens": 10, "completion_tokens": 20},
		})
	})

	resp, err := p.Send(context.Background(), &llm.ProviderRequest{
		Messages: []llm.ProviderMessage{{Role: llm.RoleUser, Text: "Weather and time in Paris?"}},
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if len(resp.ToolCalls) != 2 {
		t.Fatalf("ToolCalls: got %d, want 2", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].ToolName != "get_weather" {
		t.Errorf("tc[0].ToolName: got %q", resp.ToolCalls[0].ToolName)
	}
	if resp.ToolCalls[1].ToolName != "get_time" {
		t.Errorf("tc[1].ToolName: got %q", resp.ToolCalls[1].ToolName)
	}
}

// ── message format conversion ─────────────────────────────────────────────────

func TestSendToolResultMessages(t *testing.T) {
	var captured []byte
	p, _ := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		captured = mustReadBody(t, r)
		json.NewEncoder(w).Encode(textResponse("The weather is sunny."))
	})

	req := &llm.ProviderRequest{
		Messages: []llm.ProviderMessage{
			{Role: llm.RoleUser, Text: "Weather in Tokyo?"},
			{
				Role: llm.RoleAssistant,
				ToolCalls: []llm.ToolCall{
					{ID: "call-1", ToolName: "get_weather", ArgsJSON: json.RawMessage(`{"city":"Tokyo"}`)},
				},
			},
			{
				Role: llm.RoleUser,
				ToolResults: []llm.ToolResult{
					{ToolCallID: "call-1", Output: json.RawMessage(`"sunny and 22C"`), IsError: false},
				},
			},
		},
	}

	if _, err := p.Send(context.Background(), req); err != nil {
		t.Fatalf("Send: %v", err)
	}

	var body map[string]any
	if err := json.Unmarshal(captured, &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}

	msgs := body["messages"].([]any)
	// Expect: user, assistant (with tool_calls), tool (result)
	if len(msgs) != 3 {
		t.Fatalf("messages count: got %d, want 3", len(msgs))
	}

	// assistant message has tool_calls, no content
	assistantMsg := msgs[1].(map[string]any)
	if assistantMsg["role"] != "assistant" {
		t.Errorf("msgs[1].role: got %v", assistantMsg["role"])
	}
	tcs, ok := assistantMsg["tool_calls"].([]any)
	if !ok || len(tcs) != 1 {
		t.Fatalf("msgs[1].tool_calls: got %v", assistantMsg["tool_calls"])
	}

	// tool result message uses role:"tool" with tool_call_id
	toolMsg := msgs[2].(map[string]any)
	if toolMsg["role"] != "tool" {
		t.Errorf("msgs[2].role: got %v, want tool", toolMsg["role"])
	}
	if toolMsg["tool_call_id"] != "call-1" {
		t.Errorf("msgs[2].tool_call_id: got %v, want call-1", toolMsg["tool_call_id"])
	}
}

func TestSendImageContentPart(t *testing.T) {
	var captured []byte
	p, _ := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		captured = mustReadBody(t, r)
		json.NewEncoder(w).Encode(textResponse("An orange square."))
	})

	req := &llm.ProviderRequest{
		Messages: []llm.ProviderMessage{
			{
				Role: llm.RoleUser,
				Text: "Describe this image.",
				Images: []llm.Image{
					{Base64: "abc123", MediaType: "image/png"},
				},
			},
		},
	}
	if _, err := p.Send(context.Background(), req); err != nil {
		t.Fatalf("Send: %v", err)
	}

	var body map[string]any
	_ = json.Unmarshal(captured, &body)

	msgs := body["messages"].([]any)
	msg := msgs[0].(map[string]any)

	// Content should be a []contentPart, not a plain string.
	parts, ok := msg["content"].([]any)
	if !ok {
		t.Fatalf("content: expected []any (multipart), got %T", msg["content"])
	}
	if len(parts) != 2 {
		t.Fatalf("content parts: got %d, want 2", len(parts))
	}

	textPart := parts[0].(map[string]any)
	if textPart["type"] != "text" {
		t.Errorf("parts[0].type: got %v, want text", textPart["type"])
	}

	imgPart := parts[1].(map[string]any)
	if imgPart["type"] != "image_url" {
		t.Errorf("parts[1].type: got %v, want image_url", imgPart["type"])
	}
	imgURL := imgPart["image_url"].(map[string]any)
	url := imgURL["url"].(string)
	if !strings.HasPrefix(url, "data:image/png;base64,") {
		t.Errorf("image_url.url: expected data URL, got %q", url)
	}
	if !strings.Contains(url, "abc123") {
		t.Errorf("image_url.url: expected base64 content abc123, got %q", url)
	}
}

func TestSendImageURLContentPart(t *testing.T) {
	var captured []byte
	p, _ := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		captured = mustReadBody(t, r)
		json.NewEncoder(w).Encode(textResponse("A cat."))
	})

	req := &llm.ProviderRequest{
		Messages: []llm.ProviderMessage{
			{
				Role: llm.RoleUser,
				Text: "What is this?",
				Images: []llm.Image{
					{URL: "https://example.com/cat.jpg"},
				},
			},
		},
	}
	if _, err := p.Send(context.Background(), req); err != nil {
		t.Fatalf("Send: %v", err)
	}

	var body map[string]any
	_ = json.Unmarshal(captured, &body)
	msgs := body["messages"].([]any)
	msg := msgs[0].(map[string]any)
	parts := msg["content"].([]any)
	imgPart := parts[1].(map[string]any)
	imgURL := imgPart["image_url"].(map[string]any)
	if imgURL["url"] != "https://example.com/cat.jpg" {
		t.Errorf("image_url.url: got %v", imgURL["url"])
	}
}

// ── error handling ────────────────────────────────────────────────────────────

func TestSendAPIError(t *testing.T) {
	p, _ := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"message": "Invalid API key",
				"type":    "invalid_request_error",
				"code":    "invalid_api_key",
			},
		})
	})

	_, err := p.Send(context.Background(), &llm.ProviderRequest{
		Messages: []llm.ProviderMessage{{Role: llm.RoleUser, Text: "hi"}},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Invalid API key") {
		t.Errorf("error message: got %q, want to contain 'Invalid API key'", err.Error())
	}
}

func TestSendRateLimitError(t *testing.T) {
	p, _ := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"message": "Rate limit exceeded",
				"type":    "requests",
				"code":    "rate_limit_exceeded",
			},
		})
	})

	_, err := p.Send(context.Background(), &llm.ProviderRequest{
		Messages: []llm.ProviderMessage{{Role: llm.RoleUser, Text: "hi"}},
	})
	if err == nil {
		t.Fatal("expected error for rate limit")
	}
	if !strings.Contains(err.Error(), "429") {
		t.Errorf("error should mention HTTP 429, got %q", err.Error())
	}
}

// ── SSE streaming ─────────────────────────────────────────────────────────────

func TestSendStreamText(t *testing.T) {
	chunks := []map[string]any{
		streamTextChunk("The sky "),
		streamTextChunk("is blue."),
		{"choices": []any{map[string]any{"finish_reason": "stop", "delta": map[string]any{}}}},
	}

	p, _ := newTestProvider(t, sseHandler(t, chunks))

	events, err := p.SendStream(context.Background(), &llm.ProviderRequest{
		Messages: []llm.ProviderMessage{{Role: llm.RoleUser, Text: "Color of sky?"}},
	})
	if err != nil {
		t.Fatalf("SendStream: %v", err)
	}

	var textParts []string
	var sawDone bool
	for evt := range events {
		switch evt.Type {
		case llm.StreamTextDelta:
			textParts = append(textParts, evt.Text)
		case llm.StreamDone:
			sawDone = true
		case llm.StreamError:
			t.Fatalf("unexpected stream error: %v", evt.Err)
		}
	}

	if got := strings.Join(textParts, ""); got != "The sky is blue." {
		t.Errorf("assembled text: got %q, want %q", got, "The sky is blue.")
	}
	if !sawDone {
		t.Error("expected StreamDone event")
	}
}

func TestSendStreamToolCall(t *testing.T) {
	// OpenAI streams tool call arguments incrementally across multiple chunks.
	chunks := []map[string]any{
		// First chunk: establishes the tool call ID and name.
		{
			"choices": []any{map[string]any{
				"delta": map[string]any{
					"tool_calls": []any{map[string]any{
						"index": 0,
						"id":    "call-stream-1",
						"type":  "function",
						"function": map[string]any{
							"name":      "get_weather",
							"arguments": "",
						},
					}},
				},
				"finish_reason": nil,
			}},
		},
		// Subsequent chunks: arguments arrive as fragments.
		{
			"choices": []any{map[string]any{
				"delta": map[string]any{
					"tool_calls": []any{map[string]any{
						"index":    0,
						"function": map[string]any{"arguments": `{"cit`},
					}},
				},
				"finish_reason": nil,
			}},
		},
		{
			"choices": []any{map[string]any{
				"delta": map[string]any{
					"tool_calls": []any{map[string]any{
						"index":    0,
						"function": map[string]any{"arguments": `y":"Tokyo"}`},
					}},
				},
				"finish_reason": "tool_calls",
			}},
		},
	}

	p, _ := newTestProvider(t, sseHandler(t, chunks))

	events, err := p.SendStream(context.Background(), &llm.ProviderRequest{
		Messages: []llm.ProviderMessage{{Role: llm.RoleUser, Text: "Weather?"}},
	})
	if err != nil {
		t.Fatalf("SendStream: %v", err)
	}

	var toolCallStart *llm.ToolCall
	var sawDone bool
	for evt := range events {
		switch evt.Type {
		case llm.StreamToolCallStart:
			toolCallStart = evt.ToolCall
		case llm.StreamDone:
			sawDone = true
		case llm.StreamError:
			t.Fatalf("stream error: %v", evt.Err)
		}
	}

	if !sawDone {
		t.Error("expected StreamDone event")
	}
	if toolCallStart == nil {
		t.Fatal("expected StreamToolCallStart event, got none")
	}
	if toolCallStart.ID != "call-stream-1" {
		t.Errorf("ToolCall.ID: got %q, want call-stream-1", toolCallStart.ID)
	}
	if toolCallStart.ToolName != "get_weather" {
		t.Errorf("ToolCall.ToolName: got %q, want get_weather", toolCallStart.ToolName)
	}

	var args map[string]string
	if err := json.Unmarshal(toolCallStart.ArgsJSON, &args); err != nil {
		t.Fatalf("ArgsJSON unmarshal: %v", err)
	}
	if args["city"] != "Tokyo" {
		t.Errorf("city: got %q, want Tokyo", args["city"])
	}
}

func TestSendStreamDeltaEvents(t *testing.T) {
	// Verify that StreamToolCallDelta events are emitted during streaming.
	chunks := []map[string]any{
		{
			"choices": []any{map[string]any{
				"delta": map[string]any{
					"tool_calls": []any{map[string]any{
						"index": 0, "id": "call-1", "type": "function",
						"function": map[string]any{"name": "search", "arguments": ""},
					}},
				},
			}},
		},
		{
			"choices": []any{map[string]any{
				"delta": map[string]any{
					"tool_calls": []any{map[string]any{
						"index":    0,
						"function": map[string]any{"arguments": `{"q":"go`},
					}},
				},
			}},
		},
		{
			"choices": []any{map[string]any{
				"delta": map[string]any{
					"tool_calls": []any{map[string]any{
						"index":    0,
						"function": map[string]any{"arguments": `lang"}`},
					}},
				},
				"finish_reason": "tool_calls",
			}},
		},
	}

	p, _ := newTestProvider(t, sseHandler(t, chunks))

	events, err := p.SendStream(context.Background(), &llm.ProviderRequest{
		Messages: []llm.ProviderMessage{{Role: llm.RoleUser, Text: "Search"}},
	})
	if err != nil {
		t.Fatalf("SendStream: %v", err)
	}

	var deltaCount int
	for evt := range events {
		if evt.Type == llm.StreamToolCallDelta {
			deltaCount++
		}
		if evt.Type == llm.StreamError {
			t.Fatalf("stream error: %v", evt.Err)
		}
	}

	// We should have received delta events for each non-empty arguments fragment.
	if deltaCount < 2 {
		t.Errorf("expected at least 2 StreamToolCallDelta events, got %d", deltaCount)
	}
}

func TestSendStreamContextCancellation(t *testing.T) {
	// Server that sends one chunk then blocks.
	p, _ := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		fmt.Fprintf(w, "data: %s\n\n", mustMarshalStr(streamTextChunk("hello ")))
		if flusher != nil {
			flusher.Flush()
		}
		// Block until the client cancels.
		<-r.Context().Done()
	})

	ctx, cancel := context.WithCancel(context.Background())

	events, err := p.SendStream(ctx, &llm.ProviderRequest{
		Messages: []llm.ProviderMessage{{Role: llm.RoleUser, Text: "hi"}},
	})
	if err != nil {
		t.Fatalf("SendStream: %v", err)
	}

	// Read first event (the text delta), then cancel.
	got := <-events
	if got.Type != llm.StreamTextDelta {
		t.Errorf("first event type: got %v, want StreamTextDelta", got.Type)
	}
	cancel()

	// Drain remaining events — should get a StreamError with context.Canceled.
	var gotError bool
	for evt := range events {
		if evt.Type == llm.StreamError {
			gotError = true
		}
	}
	if !gotError {
		t.Error("expected StreamError after context cancellation")
	}
}

// ── provider metadata ─────────────────────────────────────────────────────────

func TestProviderName(t *testing.T) {
	p := New("key", "gpt-4.1")
	if p.Name() != "openai" {
		t.Errorf("Name: got %q, want openai", p.Name())
	}
}

func TestProviderModelName(t *testing.T) {
	p := New("key", "gpt-4o")
	if p.ModelName() != "gpt-4o" {
		t.Errorf("ModelName: got %q, want gpt-4o", p.ModelName())
	}
}

func TestStrictModeFalse(t *testing.T) {
	var captured []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = mustReadBody(t, r)
		json.NewEncoder(w).Encode(textResponse("ok"))
	}))
	defer srv.Close()

	p := New("key", "gpt-4.1",
		WithBaseURL(srv.URL),
		WithHTTPClient(srv.Client()),
		WithStrictMode(false),
	)

	schema := json.RawMessage(`{"type":"object","properties":{},"required":[],"additionalProperties":false}`)
	req := &llm.ProviderRequest{
		Messages: []llm.ProviderMessage{{Role: llm.RoleUser, Text: "hi"}},
		Tools:    []llm.ToolDef{{Name: "noop", InputSchema: schema}},
	}
	if _, err := p.Send(context.Background(), req); err != nil {
		t.Fatalf("Send: %v", err)
	}

	var body map[string]any
	_ = json.Unmarshal(captured, &body)
	tools := body["tools"].([]any)
	fn := tools[0].(map[string]any)["function"].(map[string]any)
	if s, ok := fn["strict"].(bool); ok && s {
		t.Error("strict should be false when WithStrictMode(false)")
	}
}

func TestSendRequestWebSearchOptions(t *testing.T) {
	t.Run("web_search_options present when WebSearch is set", func(t *testing.T) {
		var captured []byte
		p, _ := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
			captured = mustReadBody(t, r)
			json.NewEncoder(w).Encode(textResponse("Search result."))
		})

		req := &llm.ProviderRequest{
			Messages:  []llm.ProviderMessage{{Role: llm.RoleUser, Text: "Latest Go version?"}},
			WebSearch: &llm.WebSearchConfig{},
		}
		if _, err := p.Send(context.Background(), req); err != nil {
			t.Fatalf("Send: %v", err)
		}

		var body map[string]any
		if err := json.Unmarshal(captured, &body); err != nil {
			t.Fatalf("unmarshal request body: %v", err)
		}

		wso, ok := body["web_search_options"]
		if !ok {
			t.Fatal("expected web_search_options key in request body, but it was absent")
		}
		if _, isObj := wso.(map[string]any); !isObj {
			t.Fatalf("web_search_options should be a JSON object, got %T: %v", wso, wso)
		}
	})

	t.Run("web_search_options absent when WebSearch is nil", func(t *testing.T) {
		var captured []byte
		p, _ := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
			captured = mustReadBody(t, r)
			json.NewEncoder(w).Encode(textResponse("No search."))
		})

		req := &llm.ProviderRequest{
			Messages: []llm.ProviderMessage{{Role: llm.RoleUser, Text: "Hello"}},
		}
		if _, err := p.Send(context.Background(), req); err != nil {
			t.Fatalf("Send: %v", err)
		}

		var body map[string]any
		if err := json.Unmarshal(captured, &body); err != nil {
			t.Fatalf("unmarshal request body: %v", err)
		}

		if _, ok := body["web_search_options"]; ok {
			t.Fatal("web_search_options should NOT be present when WebSearch is nil")
		}
	})
}

// ── fixture helpers ───────────────────────────────────────────────────────────

// textResponse builds a minimal non-streaming OpenAI response body.
func textResponse(text string) map[string]any {
	return map[string]any{
		"choices": []any{
			map[string]any{
				"finish_reason": "stop",
				"message": map[string]any{
					"role":    "assistant",
					"content": text,
				},
			},
		},
		"usage": map[string]any{
			"prompt_tokens":     10,
			"completion_tokens": 5,
		},
	}
}

// toolCallResponse builds a minimal response that requests a single tool call.
// OpenAI returns arguments as a JSON *string* — this fixture matches that.
func toolCallResponse(id, name, argsJSON string) map[string]any {
	return map[string]any{
		"choices": []any{
			map[string]any{
				"finish_reason": "tool_calls",
				"message": map[string]any{
					"role":    "assistant",
					"content": nil,
					"tool_calls": []any{
						map[string]any{
							"id":   id,
							"type": "function",
							"function": map[string]any{
								"name":      name,
								"arguments": argsJSON, // JSON-encoded string
							},
						},
					},
				},
			},
		},
		"usage": map[string]any{
			"prompt_tokens":     20,
			"completion_tokens": 15,
		},
	}
}

// streamTextChunk builds a streaming delta chunk containing a text fragment.
func streamTextChunk(text string) map[string]any {
	return map[string]any{
		"choices": []any{
			map[string]any{
				"delta":         map[string]any{"content": text},
				"finish_reason": nil,
			},
		},
	}
}

// sseHandler builds an HTTP handler that streams the given chunks as SSE data
// lines, followed by [DONE].
func sseHandler(t *testing.T, chunks []map[string]any) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		for _, chunk := range chunks {
			b, _ := json.Marshal(chunk)
			fmt.Fprintf(w, "data: %s\n\n", string(b))
		}
		fmt.Fprintf(w, "data: [DONE]\n\n")
		if flusher != nil {
			flusher.Flush()
		}
	}
}

// mustReadBody reads and returns the request body as bytes.
func mustReadBody(t *testing.T, r *http.Request) []byte {
	t.Helper()
	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 512)
	for {
		n, err := r.Body.Read(tmp)
		buf = append(buf, tmp[:n]...)
		if err != nil {
			break
		}
	}
	return buf
}

// mustMarshalStr marshals v to a JSON string (panics on error).
func mustMarshalStr(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
