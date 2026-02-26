package llmtest

import (
	"context"
	"fmt"
	"sync"

	"github.com/shipq/shipq/llm"
)

// MockProvider implements llm.Provider with scripted responses.
// Each call to Send() or SendStream() pops the next response from the queue.
// If the queue is empty, it returns an error.
//
// Usage:
//
//	mock := llmtest.NewMockProvider("mock", "mock-model").
//	    Respond("Hello! Let me check the weather.").
//	    RespondWithToolCall("tc_1", "get_weather", `{"city":"Tokyo","country":"JP"}`).
//	    Respond("The weather in Tokyo is sunny and 22 degrees.")
//
//	client := llm.NewClient(mock, llm.WithTools(registry))
//	resp, err := client.Chat(ctx, "What's the weather in Tokyo?")
type MockProvider struct {
	mu        sync.Mutex
	name      string
	model     string
	responses []MockResponse
	calls     []CapturedRequest
	callIdx   int
}

// MockResponse is a single scripted provider response.
type MockResponse struct {
	Text      string
	ToolCalls []llm.ToolCall
	Usage     llm.Usage
	Done      bool
	Err       error
}

// CapturedRequest records what was sent to the mock provider.
type CapturedRequest struct {
	System   string
	Messages []llm.ProviderMessage
	Tools    []llm.ToolDef
}

// Compile-time check that MockProvider implements llm.Provider.
var _ llm.Provider = (*MockProvider)(nil)

// NewMockProvider creates a new MockProvider with the given name and model.
func NewMockProvider(name, model string) *MockProvider {
	return &MockProvider{
		name:  name,
		model: model,
	}
}

// Respond adds a text-only response to the queue.
// The response has Done=true (no tool calls).
func (m *MockProvider) Respond(text string) *MockProvider {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses = append(m.responses, MockResponse{
		Text: text,
		Done: true,
	})
	return m
}

// RespondWithToolCall adds a response that requests a single tool invocation.
// Done=false so the conversation loop continues after tool execution.
func (m *MockProvider) RespondWithToolCall(id, toolName, argsJSON string) *MockProvider {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses = append(m.responses, MockResponse{
		ToolCalls: []llm.ToolCall{
			{
				ID:       id,
				ToolName: toolName,
				ArgsJSON: []byte(argsJSON),
			},
		},
		Done: false,
	})
	return m
}

// RespondWithMultipleToolCalls adds a response with parallel tool calls.
// Done=false so the conversation loop continues.
func (m *MockProvider) RespondWithMultipleToolCalls(calls ...llm.ToolCall) *MockProvider {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses = append(m.responses, MockResponse{
		ToolCalls: calls,
		Done:      false,
	})
	return m
}

// RespondWithError adds an error response.
func (m *MockProvider) RespondWithError(err error) *MockProvider {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses = append(m.responses, MockResponse{
		Err: err,
	})
	return m
}

// RespondWithUsage adds a text response with specific token usage counts.
func (m *MockProvider) RespondWithUsage(text string, inputTokens, outputTokens int) *MockProvider {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses = append(m.responses, MockResponse{
		Text: text,
		Usage: llm.Usage{
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
		},
		Done: true,
	})
	return m
}

// Calls returns all captured requests sent to this mock in call order.
func (m *MockProvider) Calls() []CapturedRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]CapturedRequest, len(m.calls))
	copy(cp, m.calls)
	return cp
}

// Name returns the provider name.
func (m *MockProvider) Name() string { return m.name }

// ModelName returns the model name.
func (m *MockProvider) ModelName() string { return m.model }

// Send pops the next scripted response from the queue. Returns an error if
// the queue is empty.
func (m *MockProvider) Send(_ context.Context, req *llm.ProviderRequest) (*llm.ProviderResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Capture the request.
	m.calls = append(m.calls, CapturedRequest{
		System:   req.System,
		Messages: req.Messages,
		Tools:    req.Tools,
	})

	if m.callIdx >= len(m.responses) {
		return nil, fmt.Errorf("llmtest.MockProvider: no more scripted responses (received call #%d but only %d responses were queued)", m.callIdx+1, len(m.responses))
	}

	resp := m.responses[m.callIdx]
	m.callIdx++

	if resp.Err != nil {
		return nil, resp.Err
	}

	return &llm.ProviderResponse{
		Text:      resp.Text,
		ToolCalls: resp.ToolCalls,
		Usage:     resp.Usage,
		Done:      resp.Done,
	}, nil
}

// SendStream simulates streaming by breaking the scripted response into
// character-by-character StreamTextDelta events, followed by a StreamDone
// event. For tool call responses, it emits StreamToolCallStart events followed
// by StreamDone.
func (m *MockProvider) SendStream(_ context.Context, req *llm.ProviderRequest) (<-chan llm.StreamEvent, error) {
	m.mu.Lock()

	// Capture the request.
	m.calls = append(m.calls, CapturedRequest{
		System:   req.System,
		Messages: req.Messages,
		Tools:    req.Tools,
	})

	if m.callIdx >= len(m.responses) {
		m.mu.Unlock()
		return nil, fmt.Errorf("llmtest.MockProvider: no more scripted responses (received call #%d but only %d responses were queued)", m.callIdx+1, len(m.responses))
	}

	resp := m.responses[m.callIdx]
	m.callIdx++
	m.mu.Unlock()

	if resp.Err != nil {
		return nil, resp.Err
	}

	ch := make(chan llm.StreamEvent, len(resp.Text)+len(resp.ToolCalls)+1)

	go func() {
		defer close(ch)

		// Emit text deltas character-by-character.
		for _, r := range resp.Text {
			ch <- llm.StreamEvent{
				Type: llm.StreamTextDelta,
				Text: string(r),
			}
		}

		// Emit tool call start events.
		for _, tc := range resp.ToolCalls {
			tc := tc // capture
			ch <- llm.StreamEvent{
				Type: llm.StreamToolCallStart,
				ToolCall: &llm.ToolCall{
					ID:       tc.ID,
					ToolName: tc.ToolName,
					ArgsJSON: tc.ArgsJSON,
				},
			}
		}

		// Emit done event.
		usage := resp.Usage
		ch <- llm.StreamEvent{
			Type:  llm.StreamDone,
			Done:  true,
			Usage: &usage,
		}
	}()

	return ch, nil
}
