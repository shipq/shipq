package llmtest

import (
	"context"
	"sync"

	"github.com/shipq/shipq/llm"
)

// RecordingProvider wraps a real llm.Provider and records all interactions.
// Use it to capture real API conversations for snapshot testing or debugging.
//
// Usage:
//
//	real := anthropic.New(apiKey, model)
//	recorder := llmtest.NewRecordingProvider(real)
//	client := llm.NewClient(recorder, ...)
//
//	// After running conversations:
//	for _, entry := range recorder.Entries() {
//	    fmt.Printf("Request messages: %d\n", len(entry.Request.Messages))
//	    fmt.Printf("Response text: %s\n", entry.Response.Text)
//	}
type RecordingProvider struct {
	inner   llm.Provider
	entries []RecordingEntry
	mu      sync.Mutex
}

// RecordingEntry holds a single request/response pair captured by the
// RecordingProvider.
type RecordingEntry struct {
	Request  *llm.ProviderRequest
	Response *llm.ProviderResponse
	Err      error
}

// Compile-time check that RecordingProvider implements llm.Provider.
var _ llm.Provider = (*RecordingProvider)(nil)

// NewRecordingProvider wraps the given provider so that every Send and
// SendStream call is recorded for later inspection.
func NewRecordingProvider(inner llm.Provider) *RecordingProvider {
	return &RecordingProvider{inner: inner}
}

// Entries returns all recorded request/response pairs in call order.
// The returned slice is a copy — safe to iterate without holding a lock.
func (r *RecordingProvider) Entries() []RecordingEntry {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := make([]RecordingEntry, len(r.entries))
	copy(cp, r.entries)
	return cp
}

// Name delegates to the inner provider.
func (r *RecordingProvider) Name() string { return r.inner.Name() }

// ModelName delegates to the inner provider.
func (r *RecordingProvider) ModelName() string { return r.inner.ModelName() }

// Send delegates to the inner provider and records the request/response pair.
func (r *RecordingProvider) Send(ctx context.Context, req *llm.ProviderRequest) (*llm.ProviderResponse, error) {
	resp, err := r.inner.Send(ctx, req)
	r.mu.Lock()
	r.entries = append(r.entries, RecordingEntry{
		Request:  req,
		Response: resp,
		Err:      err,
	})
	r.mu.Unlock()
	return resp, err
}

// SendStream delegates to the inner provider's SendStream. It consumes the
// full stream to build an accumulated ProviderResponse for recording, then
// replays the events on a new channel so the caller still sees proper
// streaming behavior.
func (r *RecordingProvider) SendStream(ctx context.Context, req *llm.ProviderRequest) (<-chan llm.StreamEvent, error) {
	innerCh, err := r.inner.SendStream(ctx, req)
	if err != nil {
		r.mu.Lock()
		r.entries = append(r.entries, RecordingEntry{
			Request: req,
			Err:     err,
		})
		r.mu.Unlock()
		return nil, err
	}

	// We need to consume the entire inner stream so we can record the
	// accumulated response. We buffer all events and then replay them on
	// a new channel for the caller.
	replayCh := make(chan llm.StreamEvent, 256)

	go func() {
		defer close(replayCh)

		var (
			events    []llm.StreamEvent
			textBuf   string
			toolCalls []llm.ToolCall
			usage     llm.Usage
		)

		for evt := range innerCh {
			events = append(events, evt)

			switch evt.Type {
			case llm.StreamTextDelta:
				textBuf += evt.Text
			case llm.StreamToolCallStart:
				if evt.ToolCall != nil {
					toolCalls = append(toolCalls, *evt.ToolCall)
				}
			case llm.StreamDone:
				if evt.Usage != nil {
					usage = *evt.Usage
				}
			}
		}

		// Record the accumulated response.
		accumulated := &llm.ProviderResponse{
			Text:      textBuf,
			ToolCalls: toolCalls,
			Usage:     usage,
			Done:      len(toolCalls) == 0,
		}
		r.mu.Lock()
		r.entries = append(r.entries, RecordingEntry{
			Request:  req,
			Response: accumulated,
		})
		r.mu.Unlock()

		// Replay all collected events to the caller.
		for _, evt := range events {
			replayCh <- evt
		}
	}()

	return replayCh, nil
}
