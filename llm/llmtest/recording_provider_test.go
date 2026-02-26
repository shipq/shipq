package llmtest

import (
	"context"
	"sync"
	"testing"

	"github.com/shipq/shipq/llm"
)

func TestRecordingProvider_DelegatesSend(t *testing.T) {
	inner := NewMockProvider("inner", "inner-model").
		Respond("Hello from inner!")

	recorder := NewRecordingProvider(inner)

	if recorder.Name() != "inner" {
		t.Errorf("Name() = %q, want %q", recorder.Name(), "inner")
	}
	if recorder.ModelName() != "inner-model" {
		t.Errorf("ModelName() = %q, want %q", recorder.ModelName(), "inner-model")
	}

	resp, err := recorder.Send(context.Background(), &llm.ProviderRequest{
		System:   "Be helpful.",
		Messages: []llm.ProviderMessage{{Role: llm.RoleUser, Text: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Text != "Hello from inner!" {
		t.Errorf("got text %q, want %q", resp.Text, "Hello from inner!")
	}
	if !resp.Done {
		t.Error("expected Done=true")
	}
}

func TestRecordingProvider_RecordsSendEntries(t *testing.T) {
	inner := NewMockProvider("inner", "inner-model").
		Respond("Response 1").
		RespondWithToolCall("tc_1", "get_weather", `{"city":"Tokyo"}`).
		Respond("Response 2")

	recorder := NewRecordingProvider(inner)
	ctx := context.Background()

	// Call 1: text response
	_, err := recorder.Send(ctx, &llm.ProviderRequest{
		System:   "System 1",
		Messages: []llm.ProviderMessage{{Role: llm.RoleUser, Text: "Hello"}},
	})
	if err != nil {
		t.Fatalf("call 1: %v", err)
	}

	// Call 2: tool call response
	_, err = recorder.Send(ctx, &llm.ProviderRequest{
		System:   "System 2",
		Messages: []llm.ProviderMessage{{Role: llm.RoleUser, Text: "Weather?"}},
	})
	if err != nil {
		t.Fatalf("call 2: %v", err)
	}

	// Call 3: text response
	_, err = recorder.Send(ctx, &llm.ProviderRequest{
		Messages: []llm.ProviderMessage{{Role: llm.RoleUser, Text: "Thanks"}},
	})
	if err != nil {
		t.Fatalf("call 3: %v", err)
	}

	entries := recorder.Entries()
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	// Entry 0: text response
	if entries[0].Request.System != "System 1" {
		t.Errorf("entry 0: system = %q, want %q", entries[0].Request.System, "System 1")
	}
	if entries[0].Response.Text != "Response 1" {
		t.Errorf("entry 0: response text = %q, want %q", entries[0].Response.Text, "Response 1")
	}
	if entries[0].Err != nil {
		t.Errorf("entry 0: unexpected error: %v", entries[0].Err)
	}

	// Entry 1: tool call
	if len(entries[1].Response.ToolCalls) != 1 {
		t.Fatalf("entry 1: expected 1 tool call, got %d", len(entries[1].Response.ToolCalls))
	}
	if entries[1].Response.ToolCalls[0].ToolName != "get_weather" {
		t.Errorf("entry 1: tool name = %q, want %q", entries[1].Response.ToolCalls[0].ToolName, "get_weather")
	}

	// Entry 2: text response
	if entries[2].Response.Text != "Response 2" {
		t.Errorf("entry 2: response text = %q, want %q", entries[2].Response.Text, "Response 2")
	}
}

func TestRecordingProvider_RecordsSendErrors(t *testing.T) {
	// No responses queued → inner will return an error
	inner := NewMockProvider("inner", "inner-model")
	recorder := NewRecordingProvider(inner)

	_, err := recorder.Send(context.Background(), &llm.ProviderRequest{
		Messages: []llm.ProviderMessage{{Role: llm.RoleUser, Text: "Hi"}},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	entries := recorder.Entries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry even on error, got %d", len(entries))
	}
	if entries[0].Err == nil {
		t.Error("expected entry to have non-nil error")
	}
	if entries[0].Response != nil {
		t.Error("expected nil response on error")
	}
}

func TestRecordingProvider_SendStreamRecordsAccumulated(t *testing.T) {
	inner := NewMockProvider("inner", "inner-model").
		Respond("Hello!")

	recorder := NewRecordingProvider(inner)

	ch, err := recorder.SendStream(context.Background(), &llm.ProviderRequest{
		System:   "Be helpful.",
		Messages: []llm.ProviderMessage{{Role: llm.RoleUser, Text: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Consume all events from the replayed channel.
	var textChunks []string
	var gotDone bool
	for evt := range ch {
		switch evt.Type {
		case llm.StreamTextDelta:
			textChunks = append(textChunks, evt.Text)
		case llm.StreamDone:
			gotDone = true
		}
	}

	// Verify the caller still sees the streamed events.
	if len(textChunks) != 6 { // H, e, l, l, o, !
		t.Errorf("expected 6 text delta events, got %d: %v", len(textChunks), textChunks)
	}
	if !gotDone {
		t.Error("expected StreamDone event from replayed stream")
	}

	// Verify the accumulated response was recorded.
	entries := recorder.Entries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 recorded entry, got %d", len(entries))
	}
	if entries[0].Response == nil {
		t.Fatal("expected non-nil accumulated response")
	}
	if entries[0].Response.Text != "Hello!" {
		t.Errorf("accumulated text = %q, want %q", entries[0].Response.Text, "Hello!")
	}
	if entries[0].Err != nil {
		t.Errorf("unexpected error in entry: %v", entries[0].Err)
	}
	if entries[0].Request.System != "Be helpful." {
		t.Errorf("request system = %q, want %q", entries[0].Request.System, "Be helpful.")
	}
}

func TestRecordingProvider_SendStreamToolCall(t *testing.T) {
	inner := NewMockProvider("inner", "inner-model").
		RespondWithToolCall("tc_1", "calculate", `{"expression":"1 + 1"}`)

	recorder := NewRecordingProvider(inner)

	ch, err := recorder.SendStream(context.Background(), &llm.ProviderRequest{
		Messages: []llm.ProviderMessage{{Role: llm.RoleUser, Text: "Calculate"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Consume all events.
	var toolCallStarts int
	for evt := range ch {
		if evt.Type == llm.StreamToolCallStart {
			toolCallStarts++
		}
	}
	if toolCallStarts != 1 {
		t.Errorf("expected 1 ToolCallStart event, got %d", toolCallStarts)
	}

	// Verify the accumulated response has the tool call recorded.
	entries := recorder.Entries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 recorded entry, got %d", len(entries))
	}
	if len(entries[0].Response.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call in accumulated response, got %d", len(entries[0].Response.ToolCalls))
	}
	if entries[0].Response.ToolCalls[0].ToolName != "calculate" {
		t.Errorf("tool name = %q, want %q", entries[0].Response.ToolCalls[0].ToolName, "calculate")
	}
}

func TestRecordingProvider_SendStreamError(t *testing.T) {
	// No responses → SendStream will fail.
	inner := NewMockProvider("inner", "inner-model")
	recorder := NewRecordingProvider(inner)

	_, err := recorder.SendStream(context.Background(), &llm.ProviderRequest{
		Messages: []llm.ProviderMessage{{Role: llm.RoleUser, Text: "Hi"}},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	entries := recorder.Entries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry even on stream error, got %d", len(entries))
	}
	if entries[0].Err == nil {
		t.Error("expected non-nil error in recorded entry")
	}
}

func TestRecordingProvider_ThreadSafe(t *testing.T) {
	// Create a mock with enough responses for concurrent calls.
	const numGoroutines = 20
	inner := NewMockProvider("inner", "inner-model")
	for i := 0; i < numGoroutines; i++ {
		inner.Respond("response")
	}

	recorder := NewRecordingProvider(inner)

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			_, _ = recorder.Send(context.Background(), &llm.ProviderRequest{
				Messages: []llm.ProviderMessage{{Role: llm.RoleUser, Text: "Hi"}},
			})
		}()
	}

	wg.Wait()

	entries := recorder.Entries()
	if len(entries) != numGoroutines {
		t.Errorf("expected %d entries, got %d", numGoroutines, len(entries))
	}
}

func TestRecordingProvider_EntriesReturnsCopy(t *testing.T) {
	inner := NewMockProvider("inner", "inner-model").
		Respond("Hello")
	recorder := NewRecordingProvider(inner)

	_, err := recorder.Send(context.Background(), &llm.ProviderRequest{
		Messages: []llm.ProviderMessage{{Role: llm.RoleUser, Text: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries1 := recorder.Entries()
	entries2 := recorder.Entries()

	if len(entries1) != 1 || len(entries2) != 1 {
		t.Fatalf("expected 1 entry in each copy")
	}

	// Modifying one copy shouldn't affect the other.
	entries1[0].Err = context.Canceled
	if entries2[0].Err != nil {
		t.Error("modifying entries1 should not affect entries2")
	}
}
