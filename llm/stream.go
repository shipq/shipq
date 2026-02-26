package llm

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/shipq/shipq/channel"
)

// ── Stream envelope types ──────────────────────────────────────────────────────
//
// These are the Go structs that get serialised into channel.Envelope.Data when
// the conversation loop publishes streaming events to a channel subscriber.

// LLMTextDelta is published for every text chunk that arrives from the model.
type LLMTextDelta struct {
	Text string `json:"text"`
}

// LLMToolCallStart is published when the model begins a tool invocation.
type LLMToolCallStart struct {
	ToolCallID string          `json:"tool_call_id"`
	ToolName   string          `json:"tool_name"`
	Input      json.RawMessage `json:"input"`
}

// LLMToolCallResult is published once a tool has finished executing.
type LLMToolCallResult struct {
	ToolCallID string          `json:"tool_call_id"`
	ToolName   string          `json:"tool_name"`
	Output     json.RawMessage `json:"output"`
	Error      string          `json:"error,omitempty"`
	DurationMs int             `json:"duration_ms"`
}

// LLMDone is published when the entire conversation turn is complete.
type LLMDone struct {
	Text          string `json:"text"`
	InputTokens   int    `json:"input_tokens"`
	OutputTokens  int    `json:"output_tokens"`
	ToolCallCount int    `json:"tool_call_count"`
}

// ── Message type name constants ───────────────────────────────────────────────
//
// These are the type names used in the channel.Envelope.Type field.
// Consumers (e.g. frontend clients) key off these strings to demultiplex
// the stream.

const (
	TypeLLMTextDelta      = "LLMTextDelta"
	TypeLLMToolCallStart  = "LLMToolCallStart"
	TypeLLMToolCallResult = "LLMToolCallResult"
	TypeLLMDone           = "LLMDone"
)

// ── Publishing helpers ────────────────────────────────────────────────────────
//
// All helpers are no-ops when ch is nil, which is the normal case when the
// client is used outside a channel handler (e.g. in tests or batch code).

// publishTextDelta sends a text chunk to the channel subscriber.
// It is a no-op when ch is nil.
func publishTextDelta(ctx context.Context, ch *channel.Channel, text string) error {
	if ch == nil {
		return nil
	}
	return publish(ctx, ch, TypeLLMTextDelta, LLMTextDelta{Text: text})
}

// publishToolCallStart notifies the subscriber that a tool invocation is about
// to begin.
// It is a no-op when ch is nil.
func publishToolCallStart(ctx context.Context, ch *channel.Channel, callID, toolName string, input json.RawMessage) error {
	if ch == nil {
		return nil
	}
	return publish(ctx, ch, TypeLLMToolCallStart, LLMToolCallStart{
		ToolCallID: callID,
		ToolName:   toolName,
		Input:      input,
	})
}

// publishToolCallResult notifies the subscriber that a tool invocation has
// finished. errMsg is the error string when the tool returned an error, or
// empty on success.
// It is a no-op when ch is nil.
func publishToolCallResult(ctx context.Context, ch *channel.Channel, callID, toolName string, output json.RawMessage, errMsg string, durationMs int) error {
	if ch == nil {
		return nil
	}
	return publish(ctx, ch, TypeLLMToolCallResult, LLMToolCallResult{
		ToolCallID: callID,
		ToolName:   toolName,
		Output:     output,
		Error:      errMsg,
		DurationMs: durationMs,
	})
}

// publishDone signals that the conversation turn has completed and provides a
// final summary of token usage and tool calls made.
// It is a no-op when ch is nil.
func publishDone(ctx context.Context, ch *channel.Channel, text string, usage Usage, toolCallCount int) error {
	if ch == nil {
		return nil
	}
	return publish(ctx, ch, TypeLLMDone, LLMDone{
		Text:          text,
		InputTokens:   usage.InputTokens,
		OutputTokens:  usage.OutputTokens,
		ToolCallCount: toolCallCount,
	})
}

// publish is the shared low-level helper that marshals a typed payload and
// calls ch.Send.
func publish(ctx context.Context, ch *channel.Channel, msgType string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("llm.publish(%s): marshal: %w", msgType, err)
	}
	return ch.Send(ctx, msgType, data)
}
