package anthropic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/shipq/shipq/llm"
)

// ── test helpers ──────────────────────────────────────────────────────────────

func newTestProvider(t *testing.T, handler http.HandlerFunc) (*Provider, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	p := New("test-key", "claude-sonnet-4-20250514",
		WithBaseURL(srv.URL),
		WithHTTPClient(srv.Client()),
	)
	return p, srv
}

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

// sseBody builds a valid Anthropic SSE body from paired (event, data) strings.
func sseBody(pairs ...string) string {
	if len(pairs)%2 != 0 {
		panic("sseBody: pairs must have even length (event, data, event, data, ...)")
	}
	var sb strings.Builder
	for i := 0; i < len(pairs); i += 2 {
		sb.WriteString("event: ")
		sb.WriteString(pairs[i])
		sb.WriteString("\n")
		sb.WriteString("data: ")
		sb.WriteString(pairs[i+1])
		sb.WriteString("\n\n")
	}
	return sb.String()
}

func mustMarshalStr(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// ── request serialisation ─────────────────────────────────────────────────────

func TestSendRequestSystemPromptTopLevel(t *testing.T) {
	// Anthropic: system prompt is a top-level field, NOT a message in the array.
	var captured []byte
	p, _ := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		captured = mustReadBody(t, r)
		json.NewEncoder(w).Encode(textResponse("Hello!"))
	})

	req := &llm.ProviderRequest{
		System: "You are a helpful assistant.",
		Messages: []llm.ProviderMessage{
			{Role: llm.RoleUser, Text: "Hi"},
		},
	}
	if _, err := p.Send(context.Background(), req); err != nil {
		t.Fatalf("Send: %v", err)
	}

	var body map[string]any
	if err := json.Unmarshal(captured, &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// System must be a top-level string field.
	if body["system"] != "You are a helpful assistant." {
		t.Errorf("system: got %v, want 'You are a helpful assistant.'", body["system"])
	}

	// Messages array must NOT contain a system message.
	msgs, _ := body["messages"].([]any)
	for _, m := range msgs {
		msg := m.(map[string]any)
		if msg["role"] == "system" {
			t.Error("system message must not appear in messages array")
		}
	}

	// Only the user message should be in the messages array.
	if len(msgs) != 1 {
		t.Errorf("messages count: got %d, want 1", len(msgs))
	}
}

func TestSendRequestNoSystemPrompt(t *testing.T) {
	var captured []byte
	p, _ := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		captured = mustReadBody(t, r)
		json.NewEncoder(w).Encode(textResponse("ok"))
	})

	req := &llm.ProviderRequest{
		Messages: []llm.ProviderMessage{{Role: llm.RoleUser, Text: "Hi"}},
	}
	if _, err := p.Send(context.Background(), req); err != nil {
		t.Fatalf("Send: %v", err)
	}

	var body map[string]any
	_ = json.Unmarshal(captured, &body)

	// When no system prompt is set, the "system" key should be absent or empty.
	if s, ok := body["system"].(string); ok && s != "" {
		t.Errorf("system: expected absent or empty, got %q", s)
	}
}

func TestSendRequestMaxTokensDefaulted(t *testing.T) {
	var captured []byte
	p, _ := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		captured = mustReadBody(t, r)
		json.NewEncoder(w).Encode(textResponse("ok"))
	})

	req := &llm.ProviderRequest{
		Messages:  []llm.ProviderMessage{{Role: llm.RoleUser, Text: "hi"}},
		MaxTokens: 0, // zero → should default
	}
	if _, err := p.Send(context.Background(), req); err != nil {
		t.Fatalf("Send: %v", err)
	}

	var body map[string]any
	_ = json.Unmarshal(captured, &body)

	mt, ok := body["max_tokens"].(float64)
	if !ok || mt <= 0 {
		t.Errorf("max_tokens: expected a positive default, got %v", body["max_tokens"])
	}
}

func TestSendRequestToolDefinitionFormat(t *testing.T) {
	// Anthropic uses "input_schema" (not "parameters") and no "function" wrapper.
	var captured []byte
	p, _ := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		captured = mustReadBody(t, r)
		json.NewEncoder(w).Encode(toolUseResponse("toolu_01", "get_weather", map[string]any{"city": "Tokyo"}))
	})

	schema := json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}},"required":["city"],"additionalProperties":false}`)
	req := &llm.ProviderRequest{
		Messages: []llm.ProviderMessage{{Role: llm.RoleUser, Text: "Weather?"}},
		Tools: []llm.ToolDef{
			{Name: "get_weather", Description: "Get current weather", InputSchema: schema},
		},
	}
	if _, err := p.Send(context.Background(), req); err != nil {
		t.Fatalf("Send: %v", err)
	}

	var body map[string]any
	_ = json.Unmarshal(captured, &body)

	tools, ok := body["tools"].([]any)
	if !ok || len(tools) != 1 {
		t.Fatalf("tools: got %v", body["tools"])
	}
	tool := tools[0].(map[string]any)

	// Must have "name", "description", "input_schema" — NOT "parameters".
	if tool["name"] != "get_weather" {
		t.Errorf("tool.name: got %v", tool["name"])
	}
	if tool["description"] != "Get current weather" {
		t.Errorf("tool.description: got %v", tool["description"])
	}
	if _, ok := tool["input_schema"]; !ok {
		t.Error("tool must have 'input_schema' field (not 'parameters')")
	}
	if _, ok := tool["parameters"]; ok {
		t.Error("tool must NOT have 'parameters' field — Anthropic uses 'input_schema'")
	}
	// Must NOT have a "function" wrapper.
	if _, ok := tool["function"]; ok {
		t.Error("tool must NOT be wrapped in a 'function' object")
	}
	// type field should be absent for user-defined tools.
	if tool["type"] != nil {
		t.Errorf("tool.type: expected absent for user-defined tools, got %v", tool["type"])
	}
}

func TestSendRequestWebSearchTool(t *testing.T) {
	var captured []byte
	p, _ := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		captured = mustReadBody(t, r)
		json.NewEncoder(w).Encode(textResponse("Search result."))
	})

	req := &llm.ProviderRequest{
		Messages:  []llm.ProviderMessage{{Role: llm.RoleUser, Text: "Latest Go version?"}},
		WebSearch: &llm.WebSearchConfig{MaxResults: 5},
	}
	if _, err := p.Send(context.Background(), req); err != nil {
		t.Fatalf("Send: %v", err)
	}

	var body map[string]any
	_ = json.Unmarshal(captured, &body)

	tools, ok := body["tools"].([]any)
	if !ok || len(tools) != 1 {
		t.Fatalf("expected 1 web_search tool, got %v", body["tools"])
	}
	tool := tools[0].(map[string]any)
	if tool["type"] != "web_search_20250305" {
		t.Errorf("tool.type: got %v, want web_search_20250305", tool["type"])
	}
	if tool["name"] != "web_search" {
		t.Errorf("tool.name: got %v, want web_search", tool["name"])
	}
}

// ── response parsing ──────────────────────────────────────────────────────────

func TestSendParsesTextResponse(t *testing.T) {
	p, _ := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(textResponse("The sky is blue."))
	})

	resp, err := p.Send(context.Background(), &llm.ProviderRequest{
		Messages: []llm.ProviderMessage{{Role: llm.RoleUser, Text: "Sky color?"}},
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if resp.Text != "The sky is blue." {
		t.Errorf("Text: got %q, want %q", resp.Text, "The sky is blue.")
	}
	if !resp.Done {
		t.Error("Done should be true for end_turn response")
	}
	if len(resp.ToolCalls) != 0 {
		t.Errorf("ToolCalls: expected 0, got %d", len(resp.ToolCalls))
	}
}

func TestSendParsesUsage(t *testing.T) {
	p, _ := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"id":          "msg_01",
			"type":        "message",
			"role":        "assistant",
			"content":     []any{map[string]any{"type": "text", "text": "ok"}},
			"model":       "claude-sonnet-4-20250514",
			"stop_reason": "end_turn",
			"usage": map[string]any{
				"input_tokens":  55,
				"output_tokens": 22,
			},
		})
	})

	resp, err := p.Send(context.Background(), &llm.ProviderRequest{
		Messages: []llm.ProviderMessage{{Role: llm.RoleUser, Text: "hi"}},
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if resp.Usage.InputTokens != 55 {
		t.Errorf("InputTokens: got %d, want 55", resp.Usage.InputTokens)
	}
	if resp.Usage.OutputTokens != 22 {
		t.Errorf("OutputTokens: got %d, want 22", resp.Usage.OutputTokens)
	}
}

func TestSendParsesToolUseBlock(t *testing.T) {
	// Anthropic returns "input" as a PARSED JSON OBJECT (not a string).
	// This is the key difference from OpenAI where "arguments" is a JSON string.
	p, _ := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(toolUseResponse("toolu_abc", "get_weather", map[string]any{"city": "London"}))
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
	if tc.ID != "toolu_abc" {
		t.Errorf("ID: got %q, want toolu_abc", tc.ID)
	}
	if tc.ToolName != "get_weather" {
		t.Errorf("ToolName: got %q, want get_weather", tc.ToolName)
	}

	// ArgsJSON should be directly usable JSON (no second unmarshal required).
	var args map[string]string
	if err := json.Unmarshal(tc.ArgsJSON, &args); err != nil {
		t.Fatalf("ArgsJSON unmarshal: %v — Anthropic input should be a parsed object, not a string", err)
	}
	if args["city"] != "London" {
		t.Errorf("city: got %q, want London", args["city"])
	}
	if resp.Done {
		t.Error("Done should be false when tool calls are present")
	}
}

func TestSendParsesMultipleToolUseBlocks(t *testing.T) {
	p, _ := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"id":   "msg_02",
			"type": "message",
			"role": "assistant",
			"content": []any{
				map[string]any{
					"type":  "tool_use",
					"id":    "toolu_1",
					"name":  "get_weather",
					"input": map[string]any{"city": "Paris"},
				},
				map[string]any{
					"type":  "tool_use",
					"id":    "toolu_2",
					"name":  "get_time",
					"input": map[string]any{"tz": "Europe/Paris"},
				},
			},
			"stop_reason": "tool_use",
			"model":       "claude-sonnet-4-20250514",
			"usage":       map[string]any{"input_tokens": 30, "output_tokens": 40},
		})
	})

	resp, err := p.Send(context.Background(), &llm.ProviderRequest{
		Messages: []llm.ProviderMessage{{Role: llm.RoleUser, Text: "Paris info?"}},
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

func TestSendParsesTextAndToolUseBlocks(t *testing.T) {
	// Model can return both a text block and a tool_use block in the same response.
	p, _ := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"id":   "msg_03",
			"type": "message",
			"role": "assistant",
			"content": []any{
				map[string]any{"type": "text", "text": "Let me check the weather."},
				map[string]any{
					"type":  "tool_use",
					"id":    "toolu_xyz",
					"name":  "get_weather",
					"input": map[string]any{"city": "Tokyo"},
				},
			},
			"stop_reason": "tool_use",
			"model":       "claude-sonnet-4-20250514",
			"usage":       map[string]any{"input_tokens": 20, "output_tokens": 30},
		})
	})

	resp, err := p.Send(context.Background(), &llm.ProviderRequest{
		Messages: []llm.ProviderMessage{{Role: llm.RoleUser, Text: "Weather?"}},
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if resp.Text != "Let me check the weather." {
		t.Errorf("Text: got %q", resp.Text)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("ToolCalls: got %d, want 1", len(resp.ToolCalls))
	}
}

// ── message format conversion ─────────────────────────────────────────────────

func TestSendToolResultFormatUserRoleToolResultBlock(t *testing.T) {
	// Anthropic tool results: role:"user" + content block of type "tool_result"
	// with tool_use_id (NOT role:"tool" + tool_call_id like OpenAI).
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
				Text: "Let me check.",
				ToolCalls: []llm.ToolCall{
					{ID: "toolu_01", ToolName: "get_weather", ArgsJSON: json.RawMessage(`{"city":"Tokyo"}`)},
				},
			},
			{
				Role: llm.RoleUser,
				ToolResults: []llm.ToolResult{
					{ToolCallID: "toolu_01", Output: json.RawMessage(`"sunny and 22C"`), IsError: false},
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
	// Expect: user message, assistant with tool_use, user with tool_result
	if len(msgs) != 3 {
		t.Fatalf("messages count: got %d, want 3", len(msgs))
	}

	// The assistant message must have tool_use content blocks.
	assistantMsg := msgs[1].(map[string]any)
	if assistantMsg["role"] != "assistant" {
		t.Errorf("msgs[1].role: got %v", assistantMsg["role"])
	}
	assistantContent := assistantMsg["content"].([]any)
	var foundToolUse bool
	for _, block := range assistantContent {
		b := block.(map[string]any)
		if b["type"] == "tool_use" {
			foundToolUse = true
			if b["id"] != "toolu_01" {
				t.Errorf("tool_use.id: got %v, want toolu_01", b["id"])
			}
		}
	}
	if !foundToolUse {
		t.Error("assistant message must contain a tool_use content block")
	}

	// The tool result must be in a user message with a tool_result content block.
	toolResultMsg := msgs[2].(map[string]any)
	if toolResultMsg["role"] != "user" {
		t.Errorf("tool result msg role: got %v, want 'user' (Anthropic uses user role, not 'tool')", toolResultMsg["role"])
	}
	content := toolResultMsg["content"].([]any)
	if len(content) != 1 {
		t.Fatalf("tool result content blocks: got %d, want 1", len(content))
	}
	block := content[0].(map[string]any)
	if block["type"] != "tool_result" {
		t.Errorf("content block type: got %v, want tool_result", block["type"])
	}
	if block["tool_use_id"] != "toolu_01" {
		t.Errorf("tool_use_id: got %v, want toolu_01", block["tool_use_id"])
	}
}

func TestSendAssistantMessageToolUseBlockFormat(t *testing.T) {
	// When echoing back an assistant tool_use message, the content blocks must
	// be "clean" — only type/id/name/input — no extra zero-value fields.
	var captured []byte
	p, _ := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		captured = mustReadBody(t, r)
		json.NewEncoder(w).Encode(textResponse("Done."))
	})

	req := &llm.ProviderRequest{
		Messages: []llm.ProviderMessage{
			{Role: llm.RoleUser, Text: "Hi"},
			{
				Role: llm.RoleAssistant,
				ToolCalls: []llm.ToolCall{
					{ID: "toolu_42", ToolName: "search", ArgsJSON: json.RawMessage(`{"q":"go"}`)},
				},
			},
			{
				Role: llm.RoleUser,
				ToolResults: []llm.ToolResult{
					{ToolCallID: "toolu_42", Output: json.RawMessage(`"results"`)},
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
	assistantMsg := msgs[1].(map[string]any)
	content := assistantMsg["content"].([]any)

	// Find the tool_use block.
	var toolUseBlock map[string]any
	for _, b := range content {
		bm := b.(map[string]any)
		if bm["type"] == "tool_use" {
			toolUseBlock = bm
		}
	}
	if toolUseBlock == nil {
		t.Fatal("no tool_use block found in assistant content")
	}

	// The tool_use block must NOT have a "text" key — that's a text block field.
	if _, ok := toolUseBlock["text"]; ok {
		t.Error("tool_use block must not have a 'text' field — Anthropic rejects extra fields")
	}
	// Must have id, name, input.
	if toolUseBlock["id"] != "toolu_42" {
		t.Errorf("tool_use.id: got %v", toolUseBlock["id"])
	}
	if toolUseBlock["name"] != "search" {
		t.Errorf("tool_use.name: got %v", toolUseBlock["name"])
	}
}

func TestSendImageBase64ContentBlock(t *testing.T) {
	var captured []byte
	p, _ := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		captured = mustReadBody(t, r)
		json.NewEncoder(w).Encode(textResponse("An orange square."))
	})

	req := &llm.ProviderRequest{
		Messages: []llm.ProviderMessage{
			{
				Role: llm.RoleUser,
				Text: "Describe this.",
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
	content := msg["content"].([]any)

	// First block should be an image block.
	imgBlock := content[0].(map[string]any)
	if imgBlock["type"] != "image" {
		t.Errorf("content[0].type: got %v, want image", imgBlock["type"])
	}
	source := imgBlock["source"].(map[string]any)
	if source["type"] != "base64" {
		t.Errorf("source.type: got %v, want base64", source["type"])
	}
	if source["media_type"] != "image/png" {
		t.Errorf("source.media_type: got %v, want image/png", source["media_type"])
	}
	if source["data"] != "abc123" {
		t.Errorf("source.data: got %v, want abc123", source["data"])
	}

	// Last block should be the text.
	textBlock := content[len(content)-1].(map[string]any)
	if textBlock["type"] != "text" {
		t.Errorf("last block type: got %v, want text", textBlock["type"])
	}
}

func TestSendImageURLContentBlock(t *testing.T) {
	var captured []byte
	p, _ := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		captured = mustReadBody(t, r)
		json.NewEncoder(w).Encode(textResponse("A cat."))
	})

	req := &llm.ProviderRequest{
		Messages: []llm.ProviderMessage{
			{
				Role:   llm.RoleUser,
				Text:   "What is this?",
				Images: []llm.Image{{URL: "https://example.com/cat.jpg"}},
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
	content := msg["content"].([]any)

	imgBlock := content[0].(map[string]any)
	if imgBlock["type"] != "image" {
		t.Errorf("content[0].type: got %v, want image", imgBlock["type"])
	}
	source := imgBlock["source"].(map[string]any)
	if source["type"] != "url" {
		t.Errorf("source.type: got %v, want url", source["type"])
	}
	if source["url"] != "https://example.com/cat.jpg" {
		t.Errorf("source.url: got %v", source["url"])
	}
}

// ── error handling ────────────────────────────────────────────────────────────

func TestSendAPIError(t *testing.T) {
	p, _ := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{
			"type": "error",
			"error": map[string]any{
				"type":    "authentication_error",
				"message": "Invalid API key",
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
		t.Errorf("error: got %q, want to contain 'Invalid API key'", err.Error())
	}
}

func TestSendRateLimitError(t *testing.T) {
	// Anthropic 429 responses use the format documented at
	// https://docs.anthropic.com/en/api/errors — the error body type is
	// "rate_limit_error" and the message is generic (no "try again in Xs"
	// pattern like OpenAI). The retry-after header is the primary signal.
	p, _ := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("retry-after", "7")
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]any{
			"type": "error",
			"error": map[string]any{
				"type":    "rate_limit_error",
				"message": "Your account has hit a rate limit.",
			},
		})
	})

	_, err := p.Send(context.Background(), &llm.ProviderRequest{
		Messages: []llm.ProviderMessage{{Role: llm.RoleUser, Text: "hi"}},
	})
	if err == nil {
		t.Fatal("expected error for rate limit")
	}

	var rle *llm.RateLimitError
	if !errors.As(err, &rle) {
		t.Fatalf("expected *llm.RateLimitError, got %T: %v", err, err)
	}
	if rle.StatusCode != 429 {
		t.Errorf("StatusCode = %d, want 429", rle.StatusCode)
	}
	if rle.RetryAfter != 7*time.Second {
		t.Errorf("RetryAfter = %v, want 7s", rle.RetryAfter)
	}
	if !strings.Contains(rle.Message, "rate limit") {
		t.Errorf("Message = %q, want to contain 'rate limit'", rle.Message)
	}
}

func TestSendRateLimitError_NoRetryAfterHeader(t *testing.T) {
	// When Anthropic omits the retry-after header and the body has no
	// parseable wait duration, RetryAfter should be 0 (caller uses fallback).
	p, _ := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]any{
			"type": "error",
			"error": map[string]any{
				"type":    "rate_limit_error",
				"message": "Your account has hit a rate limit.",
			},
		})
	})

	_, err := p.Send(context.Background(), &llm.ProviderRequest{
		Messages: []llm.ProviderMessage{{Role: llm.RoleUser, Text: "hi"}},
	})

	var rle *llm.RateLimitError
	if !errors.As(err, &rle) {
		t.Fatalf("expected *llm.RateLimitError, got %T: %v", err, err)
	}
	if rle.RetryAfter != 0 {
		t.Errorf("RetryAfter = %v, want 0 (no header, no body hint)", rle.RetryAfter)
	}
}

func TestSendStreamRateLimitError(t *testing.T) {
	// Streaming endpoint should also return RateLimitError on 429.
	p, _ := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("retry-after", "4")
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]any{
			"type": "error",
			"error": map[string]any{
				"type":    "rate_limit_error",
				"message": "Your account has hit a rate limit.",
			},
		})
	})

	_, err := p.SendStream(context.Background(), &llm.ProviderRequest{
		Messages: []llm.ProviderMessage{{Role: llm.RoleUser, Text: "hi"}},
	})
	if err == nil {
		t.Fatal("expected error for rate limit")
	}

	var rle *llm.RateLimitError
	if !errors.As(err, &rle) {
		t.Fatalf("expected *llm.RateLimitError, got %T: %v", err, err)
	}
	if rle.RetryAfter != 4*time.Second {
		t.Errorf("RetryAfter = %v, want 4s", rle.RetryAfter)
	}
}

// ── version header ────────────────────────────────────────────────────────────

func TestSendAnthropicVersionHeader(t *testing.T) {
	var capturedVersion string
	p, _ := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		capturedVersion = r.Header.Get("anthropic-version")
		json.NewEncoder(w).Encode(textResponse("ok"))
	})

	if _, err := p.Send(context.Background(), &llm.ProviderRequest{
		Messages: []llm.ProviderMessage{{Role: llm.RoleUser, Text: "hi"}},
	}); err != nil {
		t.Fatalf("Send: %v", err)
	}

	if capturedVersion != defaultVersion {
		t.Errorf("anthropic-version header: got %q, want %q", capturedVersion, defaultVersion)
	}
}

func TestSendCustomAnthropicVersion(t *testing.T) {
	var capturedVersion string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedVersion = r.Header.Get("anthropic-version")
		json.NewEncoder(w).Encode(textResponse("ok"))
	}))
	defer srv.Close()

	p := New("key", "claude-sonnet-4-20250514",
		WithBaseURL(srv.URL),
		WithHTTPClient(srv.Client()),
		WithAnthropicVersion("2025-01-01"),
	)

	if _, err := p.Send(context.Background(), &llm.ProviderRequest{
		Messages: []llm.ProviderMessage{{Role: llm.RoleUser, Text: "hi"}},
	}); err != nil {
		t.Fatalf("Send: %v", err)
	}

	if capturedVersion != "2025-01-01" {
		t.Errorf("anthropic-version header: got %q, want 2025-01-01", capturedVersion)
	}
}

// ── SSE streaming ─────────────────────────────────────────────────────────────

func TestSendStreamText(t *testing.T) {
	// Build a minimal Anthropic SSE stream that delivers text deltas.
	body := sseBody(
		"message_start", mustMarshalStr(map[string]any{
			"type":    "message_start",
			"message": map[string]any{"id": "msg_01", "type": "message", "role": "assistant", "content": []any{}, "model": "claude-sonnet-4-20250514", "stop_reason": nil, "usage": map[string]any{"input_tokens": 10, "output_tokens": 1}},
		}),
		"content_block_start", mustMarshalStr(map[string]any{
			"type": "content_block_start", "index": 0,
			"content_block": map[string]any{"type": "text", "text": ""},
		}),
		"content_block_delta", mustMarshalStr(map[string]any{
			"type": "content_block_delta", "index": 0,
			"delta": map[string]any{"type": "text_delta", "text": "The sky "},
		}),
		"content_block_delta", mustMarshalStr(map[string]any{
			"type": "content_block_delta", "index": 0,
			"delta": map[string]any{"type": "text_delta", "text": "is blue."},
		}),
		"content_block_stop", mustMarshalStr(map[string]any{
			"type": "content_block_stop", "index": 0,
		}),
		"message_delta", mustMarshalStr(map[string]any{
			"type":  "message_delta",
			"delta": map[string]any{"stop_reason": "end_turn"},
			"usage": map[string]any{"output_tokens": 5},
		}),
		"message_stop", mustMarshalStr(map[string]any{"type": "message_stop"}),
	)

	p, _ := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, body)
	})

	events, err := p.SendStream(context.Background(), &llm.ProviderRequest{
		Messages: []llm.ProviderMessage{{Role: llm.RoleUser, Text: "Sky color?"}},
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
			t.Fatalf("stream error: %v", evt.Err)
		}
	}

	if got := strings.Join(textParts, ""); got != "The sky is blue." {
		t.Errorf("assembled text: got %q, want %q", got, "The sky is blue.")
	}
	if !sawDone {
		t.Error("expected StreamDone event")
	}
}

func TestSendStreamToolUse(t *testing.T) {
	// Anthropic streams tool call input as input_json_delta fragments.
	// A complete tool_use block is emitted as StreamToolCallStart once
	// content_block_stop arrives.
	body := sseBody(
		"message_start", mustMarshalStr(map[string]any{
			"type":    "message_start",
			"message": map[string]any{"id": "msg_02", "type": "message", "role": "assistant", "content": []any{}, "model": "claude-sonnet-4-20250514", "stop_reason": nil, "usage": map[string]any{"input_tokens": 20, "output_tokens": 1}},
		}),
		"content_block_start", mustMarshalStr(map[string]any{
			"type": "content_block_start", "index": 0,
			"content_block": map[string]any{"type": "text", "text": ""},
		}),
		"content_block_delta", mustMarshalStr(map[string]any{
			"type": "content_block_delta", "index": 0,
			"delta": map[string]any{"type": "text_delta", "text": "I'll check the weather."},
		}),
		"content_block_stop", mustMarshalStr(map[string]any{"type": "content_block_stop", "index": 0}),
		"content_block_start", mustMarshalStr(map[string]any{
			"type": "content_block_start", "index": 1,
			"content_block": map[string]any{"type": "tool_use", "id": "toolu_stream_01", "name": "get_weather", "input": map[string]any{}},
		}),
		"content_block_delta", mustMarshalStr(map[string]any{
			"type": "content_block_delta", "index": 1,
			"delta": map[string]any{"type": "input_json_delta", "partial_json": `{"city": "To`},
		}),
		"content_block_delta", mustMarshalStr(map[string]any{
			"type": "content_block_delta", "index": 1,
			"delta": map[string]any{"type": "input_json_delta", "partial_json": `kyo"}`},
		}),
		"content_block_stop", mustMarshalStr(map[string]any{"type": "content_block_stop", "index": 1}),
		"message_delta", mustMarshalStr(map[string]any{
			"type":  "message_delta",
			"delta": map[string]any{"stop_reason": "tool_use"},
			"usage": map[string]any{"output_tokens": 30},
		}),
		"message_stop", mustMarshalStr(map[string]any{"type": "message_stop"}),
	)

	p, _ := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, body)
	})

	events, err := p.SendStream(context.Background(), &llm.ProviderRequest{
		Messages: []llm.ProviderMessage{{Role: llm.RoleUser, Text: "Weather?"}},
	})
	if err != nil {
		t.Fatalf("SendStream: %v", err)
	}

	var textParts []string
	var toolCallStart *llm.ToolCall
	var deltaCount int
	var sawDone bool

	for evt := range events {
		switch evt.Type {
		case llm.StreamTextDelta:
			textParts = append(textParts, evt.Text)
		case llm.StreamToolCallDelta:
			deltaCount++
		case llm.StreamToolCallStart:
			toolCallStart = evt.ToolCall
		case llm.StreamDone:
			sawDone = true
		case llm.StreamError:
			t.Fatalf("stream error: %v", evt.Err)
		}
	}

	if got := strings.Join(textParts, ""); got != "I'll check the weather." {
		t.Errorf("text: got %q", got)
	}
	if deltaCount < 1 {
		t.Errorf("expected at least 1 StreamToolCallDelta event, got %d", deltaCount)
	}
	if toolCallStart == nil {
		t.Fatal("expected StreamToolCallStart event, got none")
	}
	if toolCallStart.ID != "toolu_stream_01" {
		t.Errorf("ToolCall.ID: got %q, want toolu_stream_01", toolCallStart.ID)
	}
	if toolCallStart.ToolName != "get_weather" {
		t.Errorf("ToolCall.ToolName: got %q, want get_weather", toolCallStart.ToolName)
	}

	// ArgsJSON must be valid JSON after assembling the fragments.
	var args map[string]string
	if err := json.Unmarshal(toolCallStart.ArgsJSON, &args); err != nil {
		t.Fatalf("ArgsJSON unmarshal: %v (assembled: %s)", err, toolCallStart.ArgsJSON)
	}
	if args["city"] != "Tokyo" {
		t.Errorf("city: got %q, want Tokyo", args["city"])
	}
	if !sawDone {
		t.Error("expected StreamDone event")
	}
}

func TestSendStreamError(t *testing.T) {
	body := sseBody(
		"error", mustMarshalStr(map[string]any{
			"type": "error",
			"error": map[string]any{
				"type":    "overloaded_error",
				"message": "Service temporarily overloaded",
			},
		}),
	)

	p, _ := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, body)
	})

	events, err := p.SendStream(context.Background(), &llm.ProviderRequest{
		Messages: []llm.ProviderMessage{{Role: llm.RoleUser, Text: "hi"}},
	})
	if err != nil {
		t.Fatalf("SendStream: %v", err)
	}

	var gotError bool
	for evt := range events {
		if evt.Type == llm.StreamError {
			gotError = true
			if !strings.Contains(evt.Err.Error(), "overloaded") {
				t.Errorf("stream error message: got %q, want 'overloaded'", evt.Err.Error())
			}
		}
	}

	if !gotError {
		t.Error("expected StreamError event")
	}
}

func TestSendStreamContextCancellation(t *testing.T) {
	// Server that sends one text delta then blocks.
	p, _ := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		blockStart := mustMarshalStr(map[string]any{
			"type": "content_block_start", "index": 0,
			"content_block": map[string]any{"type": "text", "text": ""},
		})
		fmt.Fprintf(w, "event: content_block_start\ndata: %s\n\n", blockStart)
		delta := mustMarshalStr(map[string]any{
			"type": "content_block_delta", "index": 0,
			"delta": map[string]any{"type": "text_delta", "text": "hello"},
		})
		fmt.Fprintf(w, "event: content_block_delta\ndata: %s\n\n", delta)
		if flusher != nil {
			flusher.Flush()
		}
		// Block until client cancels.
		<-r.Context().Done()
	})

	ctx, cancel := context.WithCancel(context.Background())

	events, err := p.SendStream(ctx, &llm.ProviderRequest{
		Messages: []llm.ProviderMessage{{Role: llm.RoleUser, Text: "hi"}},
	})
	if err != nil {
		t.Fatalf("SendStream: %v", err)
	}

	// Wait for the first event.
	got := <-events
	if got.Type != llm.StreamTextDelta {
		t.Errorf("first event: got %v, want StreamTextDelta", got.Type)
	}
	cancel()

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
	p := New("key", "claude-sonnet-4-20250514")
	if p.Name() != "anthropic" {
		t.Errorf("Name: got %q, want anthropic", p.Name())
	}
}

func TestProviderModelName(t *testing.T) {
	p := New("key", "claude-opus-4-20250514")
	if p.ModelName() != "claude-opus-4-20250514" {
		t.Errorf("ModelName: got %q, want claude-opus-4-20250514", p.ModelName())
	}
}

// ── fixture helpers ───────────────────────────────────────────────────────────

// textResponse builds a minimal Anthropic non-streaming response with text.
func textResponse(text string) map[string]any {
	return map[string]any{
		"id":   "msg_test",
		"type": "message",
		"role": "assistant",
		"content": []any{
			map[string]any{"type": "text", "text": text},
		},
		"model":       "claude-sonnet-4-20250514",
		"stop_reason": "end_turn",
		"usage": map[string]any{
			"input_tokens":  10,
			"output_tokens": 5,
		},
	}
}

// toolUseResponse builds a minimal Anthropic response that requests a tool call.
// input is a Go map that will be marshalled to JSON — this simulates Anthropic's
// behaviour of returning "input" as a parsed JSON object (not a string).
func toolUseResponse(id, name string, input map[string]any) map[string]any {
	return map[string]any{
		"id":   "msg_tool",
		"type": "message",
		"role": "assistant",
		"content": []any{
			map[string]any{
				"type":  "tool_use",
				"id":    id,
				"name":  name,
				"input": input,
			},
		},
		"model":       "claude-sonnet-4-20250514",
		"stop_reason": "tool_use",
		"usage": map[string]any{
			"input_tokens":  20,
			"output_tokens": 15,
		},
	}
}

// Ensure time is used (imported for potential future use in test fixtures).
var _ = time.Now
