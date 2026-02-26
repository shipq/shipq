package llm_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/shipq/shipq/channel"
	"github.com/shipq/shipq/llm"
	"github.com/shipq/shipq/llm/anthropic"
	"github.com/shipq/shipq/llm/openai"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func weatherRegistry() *llm.Registry {
	return &llm.Registry{
		Tools: []llm.ToolDef{{
			Name:        "get_weather",
			Description: "Get current weather for a city",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}},"required":["city"],"additionalProperties":false}`),
			Func: func(_ context.Context, argsJSON []byte) ([]byte, error) {
				var args struct {
					City string `json:"city"`
				}
				if err := json.Unmarshal(argsJSON, &args); err != nil {
					return nil, err
				}
				return json.Marshal(map[string]string{
					"temperature": "22°C",
					"condition":   "sunny",
					"city":        args.City,
				})
			},
		}},
	}
}

// ── B11: Anthropic integration — multi-turn tool-call conversation ────────────

func TestIntegrationAnthropicMultiTurnToolCall(t *testing.T) {
	var reqCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := reqCount.Add(1)

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		// Verify headers.
		if got := r.Header.Get("x-api-key"); got != "test-key" {
			t.Errorf("x-api-key: got %q, want test-key", got)
		}
		if got := r.Header.Get("anthropic-version"); got != "2023-06-01" {
			t.Errorf("anthropic-version: got %q", got)
		}

		w.Header().Set("Content-Type", "application/json")

		switch call {
		case 1:
			// First request: model asks to call get_weather.
			// Verify system prompt is top-level.
			if sys, ok := body["system"].(string); !ok || sys == "" {
				t.Errorf("expected system prompt in first request, got %v", body["system"])
			}
			// Verify tools are present.
			tools, _ := body["tools"].([]any)
			if len(tools) == 0 {
				t.Error("expected tools in first request")
			}

			json.NewEncoder(w).Encode(map[string]any{
				"id":   "msg_001",
				"type": "message",
				"role": "assistant",
				"content": []map[string]any{
					{
						"type": "text",
						"text": "Let me check the weather for you.",
					},
					{
						"type":  "tool_use",
						"id":    "toolu_01",
						"name":  "get_weather",
						"input": map[string]string{"city": "Tokyo"},
					},
				},
				"stop_reason": "tool_use",
				"usage": map[string]int{
					"input_tokens":  25,
					"output_tokens": 40,
				},
			})

		case 2:
			// Second request: model has received tool result, returns final text.
			// Verify conversation history includes the tool result.
			msgs, _ := body["messages"].([]any)
			if len(msgs) < 3 {
				t.Errorf("expected >= 3 messages in second request, got %d", len(msgs))
			}
			// The last message should be the tool result (role: "user" with tool_result block).
			if len(msgs) > 0 {
				lastMsg, _ := msgs[len(msgs)-1].(map[string]any)
				if role, _ := lastMsg["role"].(string); role != "user" {
					t.Errorf("last message role: got %q, want user", role)
				}
				content, _ := lastMsg["content"].([]any)
				if len(content) > 0 {
					block, _ := content[0].(map[string]any)
					if btype, _ := block["type"].(string); btype != "tool_result" {
						t.Errorf("last message content type: got %q, want tool_result", btype)
					}
					if toolUseID, _ := block["tool_use_id"].(string); toolUseID != "toolu_01" {
						t.Errorf("tool_use_id: got %q, want toolu_01", toolUseID)
					}
				}
			}

			json.NewEncoder(w).Encode(map[string]any{
				"id":   "msg_002",
				"type": "message",
				"role": "assistant",
				"content": []map[string]any{
					{
						"type": "text",
						"text": "The weather in Tokyo is 22°C and sunny!",
					},
				},
				"stop_reason": "end_turn",
				"usage": map[string]int{
					"input_tokens":  80,
					"output_tokens": 15,
				},
			})

		default:
			t.Errorf("unexpected request #%d", call)
			http.Error(w, "too many requests", 500)
		}
	}))
	t.Cleanup(srv.Close)

	provider := anthropic.New("test-key", "claude-sonnet-4-20250514",
		anthropic.WithBaseURL(srv.URL),
		anthropic.WithHTTPClient(srv.Client()),
	)

	client := llm.NewClient(provider,
		llm.WithTools(weatherRegistry()),
		llm.WithSystem("You are a helpful weather assistant."),
		llm.WithMaxIterations(5),
	)

	resp, err := client.Chat(context.Background(), "What's the weather in Tokyo?")
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}

	// Verify final text.
	if resp.Text != "The weather in Tokyo is 22°C and sunny!" {
		t.Errorf("Text: got %q", resp.Text)
	}

	// Verify token accounting across both rounds.
	if resp.Usage.InputTokens != 105 {
		t.Errorf("InputTokens: got %d, want 105", resp.Usage.InputTokens)
	}
	if resp.Usage.OutputTokens != 55 {
		t.Errorf("OutputTokens: got %d, want 55", resp.Usage.OutputTokens)
	}

	// Verify tool call log.
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("ToolCalls: got %d, want 1", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].ToolName != "get_weather" {
		t.Errorf("ToolName: got %q", resp.ToolCalls[0].ToolName)
	}
	if resp.ToolCalls[0].Error != nil {
		t.Errorf("ToolCall error: %v", resp.ToolCalls[0].Error)
	}

	// Verify the tool was actually called (output should contain city).
	if !strings.Contains(string(resp.ToolCalls[0].Output), "Tokyo") {
		t.Errorf("tool output should mention Tokyo, got %q", string(resp.ToolCalls[0].Output))
	}

	// Two requests should have been made.
	if reqCount.Load() != 2 {
		t.Errorf("request count: got %d, want 2", reqCount.Load())
	}
}

// ── B11: OpenAI integration — multi-turn tool-call conversation ───────────────

func TestIntegrationOpenAIMultiTurnToolCall(t *testing.T) {
	var reqCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := reqCount.Add(1)

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		// Verify auth header.
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-key" {
			t.Errorf("Authorization: got %q", auth)
		}

		w.Header().Set("Content-Type", "application/json")

		switch call {
		case 1:
			// First request: model returns a tool call.
			// Verify system prompt is in messages[0].
			msgs, _ := body["messages"].([]any)
			if len(msgs) < 2 {
				t.Errorf("expected >= 2 messages (system + user), got %d", len(msgs))
			}
			if len(msgs) > 0 {
				sysMsg, _ := msgs[0].(map[string]any)
				if role, _ := sysMsg["role"].(string); role != "system" {
					t.Errorf("first message role: got %q, want system", role)
				}
			}
			// Verify tools are present.
			tools, _ := body["tools"].([]any)
			if len(tools) == 0 {
				t.Error("expected tools in first request")
			}

			// OpenAI returns tool_calls with arguments as a JSON *string*.
			argsStr := `{"city":"Tokyo"}`
			json.NewEncoder(w).Encode(map[string]any{
				"id":     "chatcmpl-001",
				"object": "chat.completion",
				"model":  "gpt-4.1",
				"choices": []map[string]any{{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": nil,
						"tool_calls": []map[string]any{{
							"id":   "call_abc123",
							"type": "function",
							"function": map[string]any{
								"name":      "get_weather",
								"arguments": argsStr,
							},
						}},
					},
					"finish_reason": "tool_calls",
				}},
				"usage": map[string]int{
					"prompt_tokens":     30,
					"completion_tokens": 25,
				},
			})

		case 2:
			// Second request: tool result has been provided, model returns final text.
			// Verify conversation history includes a "tool" role message.
			msgs, _ := body["messages"].([]any)
			foundToolMsg := false
			for _, m := range msgs {
				msg, _ := m.(map[string]any)
				if role, _ := msg["role"].(string); role == "tool" {
					foundToolMsg = true
					if callID, _ := msg["tool_call_id"].(string); callID != "call_abc123" {
						t.Errorf("tool message tool_call_id: got %q, want call_abc123", callID)
					}
				}
			}
			if !foundToolMsg {
				t.Error("expected a 'tool' role message in second request")
			}

			json.NewEncoder(w).Encode(map[string]any{
				"id":     "chatcmpl-002",
				"object": "chat.completion",
				"model":  "gpt-4.1",
				"choices": []map[string]any{{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": "The weather in Tokyo is 22°C and sunny!",
					},
					"finish_reason": "stop",
				}},
				"usage": map[string]int{
					"prompt_tokens":     90,
					"completion_tokens": 18,
				},
			})

		default:
			t.Errorf("unexpected request #%d", call)
			http.Error(w, "too many requests", 500)
		}
	}))
	t.Cleanup(srv.Close)

	provider := openai.New("test-key", "gpt-4.1",
		openai.WithBaseURL(srv.URL),
		openai.WithHTTPClient(srv.Client()),
	)

	client := llm.NewClient(provider,
		llm.WithTools(weatherRegistry()),
		llm.WithSystem("You are a helpful weather assistant."),
		llm.WithMaxIterations(5),
	)

	resp, err := client.Chat(context.Background(), "What's the weather in Tokyo?")
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}

	if resp.Text != "The weather in Tokyo is 22°C and sunny!" {
		t.Errorf("Text: got %q", resp.Text)
	}

	if resp.Usage.InputTokens != 120 {
		t.Errorf("InputTokens: got %d, want 120", resp.Usage.InputTokens)
	}
	if resp.Usage.OutputTokens != 43 {
		t.Errorf("OutputTokens: got %d, want 43", resp.Usage.OutputTokens)
	}

	if len(resp.ToolCalls) != 1 {
		t.Fatalf("ToolCalls: got %d, want 1", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].ToolName != "get_weather" {
		t.Errorf("ToolName: got %q", resp.ToolCalls[0].ToolName)
	}
	if resp.ToolCalls[0].Error != nil {
		t.Errorf("ToolCall error: %v", resp.ToolCalls[0].Error)
	}
	if !strings.Contains(string(resp.ToolCalls[0].Output), "Tokyo") {
		t.Errorf("tool output should mention Tokyo, got %q", string(resp.ToolCalls[0].Output))
	}

	if reqCount.Load() != 2 {
		t.Errorf("request count: got %d, want 2", reqCount.Load())
	}
}

// ── B11: Anthropic streaming integration — multi-turn with SSE ────────────────

func TestIntegrationAnthropicStreamingMultiTurn(t *testing.T) {
	var reqCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := reqCount.Add(1)

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		stream, _ := body["stream"].(bool)
		if !stream {
			t.Error("expected stream=true")
		}

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "flusher not supported", 500)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")

		writeSSE := func(event, data string) {
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
			flusher.Flush()
		}

		mustJSON := func(v any) string {
			b, _ := json.Marshal(v)
			return string(b)
		}

		switch call {
		case 1:
			// Stream: text delta → tool_use block → message_delta with usage.
			writeSSE("message_start", mustJSON(map[string]any{
				"type": "message_start",
				"message": map[string]any{
					"id": "msg_001", "type": "message", "role": "assistant",
				},
			}))

			// Text block.
			writeSSE("content_block_start", mustJSON(map[string]any{
				"type": "content_block_start", "index": 0,
				"content_block": map[string]any{"type": "text", "text": ""},
			}))
			writeSSE("content_block_delta", mustJSON(map[string]any{
				"type": "content_block_delta", "index": 0,
				"delta": map[string]any{"type": "text_delta", "text": "Checking weather"},
			}))
			writeSSE("content_block_stop", mustJSON(map[string]any{
				"type": "content_block_stop", "index": 0,
			}))

			// Tool use block.
			writeSSE("content_block_start", mustJSON(map[string]any{
				"type": "content_block_start", "index": 1,
				"content_block": map[string]any{
					"type": "tool_use", "id": "toolu_stream_01", "name": "get_weather",
				},
			}))
			writeSSE("content_block_delta", mustJSON(map[string]any{
				"type": "content_block_delta", "index": 1,
				"delta": map[string]any{
					"type":         "input_json_delta",
					"partial_json": `{"city":`,
				},
			}))
			writeSSE("content_block_delta", mustJSON(map[string]any{
				"type": "content_block_delta", "index": 1,
				"delta": map[string]any{
					"type":         "input_json_delta",
					"partial_json": `"Tokyo"}`,
				},
			}))
			writeSSE("content_block_stop", mustJSON(map[string]any{
				"type": "content_block_stop", "index": 1,
			}))

			writeSSE("message_delta", mustJSON(map[string]any{
				"type": "message_delta",
				"usage": map[string]int{
					"output_tokens": 30,
				},
			}))

		case 2:
			// Second request: stream final text response.
			writeSSE("message_start", mustJSON(map[string]any{
				"type": "message_start",
				"message": map[string]any{
					"id": "msg_002", "type": "message", "role": "assistant",
				},
			}))
			writeSSE("content_block_start", mustJSON(map[string]any{
				"type": "content_block_start", "index": 0,
				"content_block": map[string]any{"type": "text", "text": ""},
			}))
			writeSSE("content_block_delta", mustJSON(map[string]any{
				"type": "content_block_delta", "index": 0,
				"delta": map[string]any{"type": "text_delta", "text": "It's 22°C and sunny in Tokyo!"},
			}))
			writeSSE("content_block_stop", mustJSON(map[string]any{
				"type": "content_block_stop", "index": 0,
			}))
			writeSSE("message_delta", mustJSON(map[string]any{
				"type": "message_delta",
				"usage": map[string]int{
					"output_tokens": 12,
				},
			}))

		default:
			t.Errorf("unexpected streaming request #%d", call)
		}
	}))
	t.Cleanup(srv.Close)

	provider := anthropic.New("test-key", "claude-sonnet-4-20250514",
		anthropic.WithBaseURL(srv.URL),
		anthropic.WithHTTPClient(srv.Client()),
	)

	// Wire up a channel so the client uses streaming (SendStream path).
	// We use a no-op transport since we're only verifying the response, not the
	// channel publishes (that's covered in the unit tests).
	client := llm.NewClient(provider,
		llm.WithTools(weatherRegistry()),
		llm.WithSystem("You are a weather bot."),
		llm.WithMaxIterations(5),
		llm.WithChannel(noopChannel(t)),
	)

	resp, err := client.Chat(context.Background(), "Weather in Tokyo?")
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}

	if resp.Text != "It's 22°C and sunny in Tokyo!" {
		t.Errorf("Text: got %q", resp.Text)
	}

	if len(resp.ToolCalls) != 1 {
		t.Fatalf("ToolCalls: got %d, want 1", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].ToolName != "get_weather" {
		t.Errorf("ToolName: got %q", resp.ToolCalls[0].ToolName)
	}
	if !strings.Contains(string(resp.ToolCalls[0].Output), "Tokyo") {
		t.Errorf("tool output should mention Tokyo, got %q", string(resp.ToolCalls[0].Output))
	}

	if reqCount.Load() != 2 {
		t.Errorf("request count: got %d, want 2", reqCount.Load())
	}
}

// ── B11: Multi-tool parallel integration ──────────────────────────────────────

func TestIntegrationAnthropicParallelToolCalls(t *testing.T) {
	var reqCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := reqCount.Add(1)

		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)

		w.Header().Set("Content-Type", "application/json")

		switch call {
		case 1:
			// Model requests two tool calls at once.
			json.NewEncoder(w).Encode(map[string]any{
				"id":   "msg_010",
				"type": "message",
				"role": "assistant",
				"content": []map[string]any{
					{
						"type":  "tool_use",
						"id":    "toolu_a",
						"name":  "get_weather",
						"input": map[string]string{"city": "Tokyo"},
					},
					{
						"type":  "tool_use",
						"id":    "toolu_b",
						"name":  "get_weather",
						"input": map[string]string{"city": "London"},
					},
				},
				"stop_reason": "tool_use",
				"usage": map[string]int{
					"input_tokens":  30,
					"output_tokens": 50,
				},
			})

		case 2:
			// Verify both tool results are present.
			msgs, _ := body["messages"].([]any)
			lastMsg, _ := msgs[len(msgs)-1].(map[string]any)
			content, _ := lastMsg["content"].([]any)
			if len(content) != 2 {
				t.Errorf("expected 2 tool_result blocks, got %d", len(content))
			}
			// Verify both tool_use_ids are present.
			ids := map[string]bool{}
			for _, c := range content {
				block, _ := c.(map[string]any)
				id, _ := block["tool_use_id"].(string)
				ids[id] = true
			}
			if !ids["toolu_a"] || !ids["toolu_b"] {
				t.Errorf("expected both toolu_a and toolu_b, got %v", ids)
			}

			json.NewEncoder(w).Encode(map[string]any{
				"id":   "msg_011",
				"type": "message",
				"role": "assistant",
				"content": []map[string]any{{
					"type": "text",
					"text": "Tokyo is 22°C sunny, London is 15°C cloudy.",
				}},
				"stop_reason": "end_turn",
				"usage": map[string]int{
					"input_tokens":  100,
					"output_tokens": 20,
				},
			})

		default:
			t.Errorf("unexpected request #%d", call)
			http.Error(w, "too many requests", 500)
		}
	}))
	t.Cleanup(srv.Close)

	provider := anthropic.New("test-key", "claude-sonnet-4-20250514",
		anthropic.WithBaseURL(srv.URL),
		anthropic.WithHTTPClient(srv.Client()),
	)

	client := llm.NewClient(provider,
		llm.WithTools(weatherRegistry()),
		llm.WithSystem("Compare weather."),
	)

	resp, err := client.Chat(context.Background(), "Compare Tokyo and London weather")
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}

	if !strings.Contains(resp.Text, "Tokyo") || !strings.Contains(resp.Text, "London") {
		t.Errorf("Text: got %q, want mention of both cities", resp.Text)
	}
	if len(resp.ToolCalls) != 2 {
		t.Fatalf("ToolCalls: got %d, want 2", len(resp.ToolCalls))
	}
	if resp.Usage.InputTokens != 130 {
		t.Errorf("InputTokens: got %d, want 130", resp.Usage.InputTokens)
	}
	if resp.Usage.OutputTokens != 70 {
		t.Errorf("OutputTokens: got %d, want 70", resp.Usage.OutputTokens)
	}
	if reqCount.Load() != 2 {
		t.Errorf("request count: got %d, want 2", reqCount.Load())
	}
}

// ── B11: Persistence integration ──────────────────────────────────────────────

// mockIntegrationPersister is a simple in-memory Persister for integration tests.
// (We can't use the internal mockPersister from persist_test.go because it's in
// the llm package's test files, not exported.)
type mockIntegrationPersister struct {
	conversations []llm.InsertConversationParams
	updates       []llm.UpdateConversationParams
	messages      []llm.InsertMessageParams
	nextID        int64
}

func (m *mockIntegrationPersister) InsertConversation(_ context.Context, p llm.InsertConversationParams) (llm.ConversationRow, error) {
	m.nextID++
	m.conversations = append(m.conversations, p)
	return llm.ConversationRow{ID: m.nextID, PublicID: p.PublicID}, nil
}

func (m *mockIntegrationPersister) UpdateConversation(_ context.Context, p llm.UpdateConversationParams) error {
	m.updates = append(m.updates, p)
	return nil
}

func (m *mockIntegrationPersister) InsertMessage(_ context.Context, p llm.InsertMessageParams) error {
	m.messages = append(m.messages, p)
	return nil
}

func TestIntegrationPersistenceWithAnthropicProvider(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id":   "msg_p01",
			"type": "message",
			"role": "assistant",
			"content": []map[string]any{{
				"type": "text",
				"text": "Hello! I'm here to help.",
			}},
			"stop_reason": "end_turn",
			"usage": map[string]int{
				"input_tokens":  10,
				"output_tokens": 8,
			},
		})
	}))
	t.Cleanup(srv.Close)

	provider := anthropic.New("test-key", "claude-sonnet-4-20250514",
		anthropic.WithBaseURL(srv.URL),
		anthropic.WithHTTPClient(srv.Client()),
	)

	persist := &mockIntegrationPersister{nextID: 0}
	client := llm.NewClient(provider,
		llm.WithPersister(persist),
		llm.WithSystem("Be helpful."),
	)

	resp, err := client.Chat(context.Background(), "Hi")
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.Text != "Hello! I'm here to help." {
		t.Errorf("Text: got %q", resp.Text)
	}

	// Conversation was created and completed.
	if len(persist.conversations) != 1 {
		t.Fatalf("conversations: got %d, want 1", len(persist.conversations))
	}
	if persist.conversations[0].Provider != "anthropic" {
		t.Errorf("provider: got %q", persist.conversations[0].Provider)
	}
	if persist.conversations[0].Model != "claude-sonnet-4-20250514" {
		t.Errorf("model: got %q", persist.conversations[0].Model)
	}
	if persist.conversations[0].Status != "running" {
		t.Errorf("initial status: got %q", persist.conversations[0].Status)
	}

	if len(persist.updates) != 1 {
		t.Fatalf("updates: got %d, want 1", len(persist.updates))
	}
	if persist.updates[0].Status != "completed" {
		t.Errorf("final status: got %q", persist.updates[0].Status)
	}

	// Messages: user + assistant.
	if len(persist.messages) != 2 {
		t.Fatalf("messages: got %d, want 2", len(persist.messages))
	}
	if persist.messages[0].Role != "user" {
		t.Errorf("messages[0].Role: got %q", persist.messages[0].Role)
	}
	if persist.messages[1].Role != "assistant" {
		t.Errorf("messages[1].Role: got %q", persist.messages[1].Role)
	}

	if resp.ConversationID == "" {
		t.Error("ConversationID should be non-empty")
	}
}

// ── helpers: no-op channel for forcing streaming path ─────────────────────────

// noopTransport satisfies channel.RealtimeTransport but discards everything.
type noopTransport struct{}

func (noopTransport) Publish(string, []byte) error { return nil }
func (noopTransport) Subscribe(string, string) (<-chan []byte, func(), error) {
	return make(chan []byte), func() {}, nil
}
func (noopTransport) GenerateConnectionToken(_ string, _ time.Duration) (string, error) {
	return "", nil
}
func (noopTransport) GenerateSubscriptionToken(_ string, _ string, _ time.Duration) (string, error) {
	return "", nil
}
func (noopTransport) ConnectionURL() string { return "" }

// noopChannel creates a *channel.Channel backed by a no-op transport.
// This forces the Client into the streaming (SendStream) path without
// actually publishing anything.
func noopChannel(t *testing.T) *channel.Channel {
	t.Helper()
	tr := noopTransport{}
	incoming, cleanup, _ := tr.Subscribe("noop", "noop")
	return channel.NewChannel("noop", "noop-job", 0, 0, false, tr, incoming, cleanup)
}
