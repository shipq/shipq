package llm

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/shipq/shipq/channel"
)

// ── mock transport ─────────────────────────────────────────────────────────────

type mockTransport struct {
	mu        sync.Mutex
	published []mockPublish
}

type mockPublish struct {
	ch   string
	data []byte
}

func (m *mockTransport) Publish(ch string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.published = append(m.published, mockPublish{ch: ch, data: data})
	return nil
}

func (m *mockTransport) Subscribe(ch string, subscriberID string) (<-chan []byte, func(), error) {
	c := make(chan []byte)
	return c, func() { close(c) }, nil
}

func (m *mockTransport) GenerateConnectionToken(sub string, ttl time.Duration) (string, error) {
	return "token", nil
}

func (m *mockTransport) GenerateSubscriptionToken(sub string, ch string, ttl time.Duration) (string, error) {
	return "token", nil
}

func (m *mockTransport) ConnectionURL() string { return "ws://localhost" }

// ── helpers ───────────────────────────────────────────────────────────────────

func newTestChannel(t *testing.T) (*channel.Channel, *mockTransport) {
	t.Helper()
	mt := &mockTransport{}
	ch := channel.NewChannel("test", "job-1", 0, 0, true, mt, make(<-chan []byte), func() {})
	return ch, mt
}

func decodeEnvelope(t *testing.T, data []byte) channel.Envelope {
	t.Helper()
	var env channel.Envelope
	if err := env.Unmarshal(data); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	return env
}

// ── nil channel: all helpers are no-ops ───────────────────────────────────────

func TestPublishTextDeltaNilChannel(t *testing.T) {
	ctx := context.Background()
	if err := publishTextDelta(ctx, nil, "hello"); err != nil {
		t.Errorf("expected nil error with nil channel, got %v", err)
	}
}

func TestPublishToolCallStartNilChannel(t *testing.T) {
	ctx := context.Background()
	if err := publishToolCallStart(ctx, nil, "id", "tool", json.RawMessage(`{}`)); err != nil {
		t.Errorf("expected nil error with nil channel, got %v", err)
	}
}

func TestPublishToolCallResultNilChannel(t *testing.T) {
	ctx := context.Background()
	if err := publishToolCallResult(ctx, nil, "id", "tool", json.RawMessage(`{}`), "", 10); err != nil {
		t.Errorf("expected nil error with nil channel, got %v", err)
	}
}

func TestPublishDoneNilChannel(t *testing.T) {
	ctx := context.Background()
	if err := publishDone(ctx, nil, "done", Usage{InputTokens: 10, OutputTokens: 5}, 1); err != nil {
		t.Errorf("expected nil error with nil channel, got %v", err)
	}
}

// ── publishTextDelta ──────────────────────────────────────────────────────────

func TestPublishTextDelta(t *testing.T) {
	ctx := context.Background()
	ch, mt := newTestChannel(t)

	if err := publishTextDelta(ctx, ch, "hello world"); err != nil {
		t.Fatalf("publishTextDelta: %v", err)
	}

	if len(mt.published) != 1 {
		t.Fatalf("expected 1 published message, got %d", len(mt.published))
	}

	env := decodeEnvelope(t, mt.published[0].data)
	if env.Type != TypeLLMTextDelta {
		t.Errorf("type: got %q, want %q", env.Type, TypeLLMTextDelta)
	}

	var payload LLMTextDelta
	if err := json.Unmarshal(env.Data, &payload); err != nil {
		t.Fatalf("unmarshal LLMTextDelta: %v", err)
	}
	if payload.Text != "hello world" {
		t.Errorf("Text: got %q, want %q", payload.Text, "hello world")
	}
}

func TestPublishTextDeltaEmptyString(t *testing.T) {
	ctx := context.Background()
	ch, mt := newTestChannel(t)

	if err := publishTextDelta(ctx, ch, ""); err != nil {
		t.Fatalf("publishTextDelta(empty): %v", err)
	}
	if len(mt.published) != 1 {
		t.Fatalf("expected 1 published message, got %d", len(mt.published))
	}
	env := decodeEnvelope(t, mt.published[0].data)
	var payload LLMTextDelta
	_ = json.Unmarshal(env.Data, &payload)
	if payload.Text != "" {
		t.Errorf("Text: got %q, want empty", payload.Text)
	}
}

// ── publishToolCallStart ──────────────────────────────────────────────────────

func TestPublishToolCallStart(t *testing.T) {
	ctx := context.Background()
	ch, mt := newTestChannel(t)

	input := json.RawMessage(`{"city":"Tokyo"}`)
	if err := publishToolCallStart(ctx, ch, "call-1", "get_weather", input); err != nil {
		t.Fatalf("publishToolCallStart: %v", err)
	}

	if len(mt.published) != 1 {
		t.Fatalf("expected 1 published message, got %d", len(mt.published))
	}

	env := decodeEnvelope(t, mt.published[0].data)
	if env.Type != TypeLLMToolCallStart {
		t.Errorf("type: got %q, want %q", env.Type, TypeLLMToolCallStart)
	}

	var payload LLMToolCallStart
	if err := json.Unmarshal(env.Data, &payload); err != nil {
		t.Fatalf("unmarshal LLMToolCallStart: %v", err)
	}
	if payload.ToolCallID != "call-1" {
		t.Errorf("ToolCallID: got %q, want %q", payload.ToolCallID, "call-1")
	}
	if payload.ToolName != "get_weather" {
		t.Errorf("ToolName: got %q, want %q", payload.ToolName, "get_weather")
	}
	// Verify input is round-tripped correctly.
	var got map[string]string
	if err := json.Unmarshal(payload.Input, &got); err != nil {
		t.Fatalf("unmarshal Input: %v", err)
	}
	if got["city"] != "Tokyo" {
		t.Errorf("Input.city: got %q, want %q", got["city"], "Tokyo")
	}
}

// ── publishToolCallResult ─────────────────────────────────────────────────────

func TestPublishToolCallResultSuccess(t *testing.T) {
	ctx := context.Background()
	ch, mt := newTestChannel(t)

	output := json.RawMessage(`{"weather":"sunny"}`)
	if err := publishToolCallResult(ctx, ch, "call-1", "get_weather", output, "", 42); err != nil {
		t.Fatalf("publishToolCallResult: %v", err)
	}

	env := decodeEnvelope(t, mt.published[0].data)
	if env.Type != TypeLLMToolCallResult {
		t.Errorf("type: got %q, want %q", env.Type, TypeLLMToolCallResult)
	}

	var payload LLMToolCallResult
	if err := json.Unmarshal(env.Data, &payload); err != nil {
		t.Fatalf("unmarshal LLMToolCallResult: %v", err)
	}
	if payload.ToolCallID != "call-1" {
		t.Errorf("ToolCallID: got %q", payload.ToolCallID)
	}
	if payload.ToolName != "get_weather" {
		t.Errorf("ToolName: got %q", payload.ToolName)
	}
	if payload.Error != "" {
		t.Errorf("Error: got %q, want empty", payload.Error)
	}
	if payload.DurationMs != 42 {
		t.Errorf("DurationMs: got %d, want 42", payload.DurationMs)
	}

	var gotOutput map[string]string
	if err := json.Unmarshal(payload.Output, &gotOutput); err != nil {
		t.Fatalf("unmarshal Output: %v", err)
	}
	if gotOutput["weather"] != "sunny" {
		t.Errorf("Output.weather: got %q, want sunny", gotOutput["weather"])
	}
}

func TestPublishToolCallResultError(t *testing.T) {
	ctx := context.Background()
	ch, mt := newTestChannel(t)

	if err := publishToolCallResult(ctx, ch, "call-1", "get_weather", nil, "city not found", 5); err != nil {
		t.Fatalf("publishToolCallResult(error): %v", err)
	}

	env := decodeEnvelope(t, mt.published[0].data)
	var payload LLMToolCallResult
	if err := json.Unmarshal(env.Data, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload.Error != "city not found" {
		t.Errorf("Error: got %q, want %q", payload.Error, "city not found")
	}
	if payload.DurationMs != 5 {
		t.Errorf("DurationMs: got %d, want 5", payload.DurationMs)
	}
}

func TestPublishToolCallResultErrorOmittedFromJSONWhenEmpty(t *testing.T) {
	ctx := context.Background()
	ch, mt := newTestChannel(t)

	output := json.RawMessage(`{}`)
	if err := publishToolCallResult(ctx, ch, "call-1", "tool", output, "", 0); err != nil {
		t.Fatalf("publishToolCallResult: %v", err)
	}

	env := decodeEnvelope(t, mt.published[0].data)

	// When Error is empty, it should be omitted from the JSON (omitempty tag).
	var raw map[string]any
	if err := json.Unmarshal(env.Data, &raw); err != nil {
		t.Fatalf("unmarshal raw: %v", err)
	}
	if _, present := raw["error"]; present {
		t.Error("\"error\" key should be omitted from JSON when empty")
	}
}

// ── publishDone ───────────────────────────────────────────────────────────────

func TestPublishDone(t *testing.T) {
	ctx := context.Background()
	ch, mt := newTestChannel(t)

	usage := Usage{InputTokens: 100, OutputTokens: 50}
	if err := publishDone(ctx, ch, "The weather is sunny.", usage, 2); err != nil {
		t.Fatalf("publishDone: %v", err)
	}

	env := decodeEnvelope(t, mt.published[0].data)
	if env.Type != TypeLLMDone {
		t.Errorf("type: got %q, want %q", env.Type, TypeLLMDone)
	}

	var payload LLMDone
	if err := json.Unmarshal(env.Data, &payload); err != nil {
		t.Fatalf("unmarshal LLMDone: %v", err)
	}
	if payload.Text != "The weather is sunny." {
		t.Errorf("Text: got %q", payload.Text)
	}
	if payload.InputTokens != 100 {
		t.Errorf("InputTokens: got %d, want 100", payload.InputTokens)
	}
	if payload.OutputTokens != 50 {
		t.Errorf("OutputTokens: got %d, want 50", payload.OutputTokens)
	}
	if payload.ToolCallCount != 2 {
		t.Errorf("ToolCallCount: got %d, want 2", payload.ToolCallCount)
	}
}

func TestPublishDoneZeroUsage(t *testing.T) {
	ctx := context.Background()
	ch, mt := newTestChannel(t)

	if err := publishDone(ctx, ch, "hello", Usage{}, 0); err != nil {
		t.Fatalf("publishDone: %v", err)
	}
	env := decodeEnvelope(t, mt.published[0].data)
	var payload LLMDone
	if err := json.Unmarshal(env.Data, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload.InputTokens != 0 || payload.OutputTokens != 0 || payload.ToolCallCount != 0 {
		t.Errorf("expected all zeros: %+v", payload)
	}
}

// ── ordering: multiple publishes arrive in order ──────────────────────────────

func TestPublishOrderPreserved(t *testing.T) {
	ctx := context.Background()
	ch, mt := newTestChannel(t)

	_ = publishTextDelta(ctx, ch, "chunk1")
	_ = publishTextDelta(ctx, ch, "chunk2")
	_ = publishDone(ctx, ch, "chunk1chunk2", Usage{}, 0)

	if len(mt.published) != 3 {
		t.Fatalf("expected 3 published messages, got %d", len(mt.published))
	}

	types := make([]string, 3)
	for i, p := range mt.published {
		env := decodeEnvelope(t, p.data)
		types[i] = env.Type
	}

	if types[0] != TypeLLMTextDelta {
		t.Errorf("msg[0] type: got %q, want %q", types[0], TypeLLMTextDelta)
	}
	if types[1] != TypeLLMTextDelta {
		t.Errorf("msg[1] type: got %q, want %q", types[1], TypeLLMTextDelta)
	}
	if types[2] != TypeLLMDone {
		t.Errorf("msg[2] type: got %q, want %q", types[2], TypeLLMDone)
	}
}
