package llm

import (
	"context"
	"encoding/json"
	"time"
)

// ToolFunc is the signature for a compiled tool dispatcher.
// It receives raw JSON arguments, deserializes to the input struct,
// calls the user's function, and serializes the output.
type ToolFunc func(ctx context.Context, argsJSON []byte) (resultJSON []byte, err error)

// ToolDef is the provider-agnostic definition of a tool.
type ToolDef struct {
	Name        string          // "get_weather"
	Description string          // "Get the current weather for a city"
	InputSchema json.RawMessage // JSON Schema for the input struct
	Func        ToolFunc        // the actual Go function to call

	// The following fields are populated by App.Tool() via reflection and
	// used by the compile program to extract metadata for code generation.
	// They are not needed at runtime and may be empty when tools are
	// registered directly (without going through App).

	InputType  string // Go type name of the input struct, e.g. "WeatherInput"
	OutputType string // Go type name of the output struct, e.g. "WeatherOutput"
	Package    string // Import path of the package containing the tool function
}

// Registry holds all registered tools.
type Registry struct {
	Tools []ToolDef
}

// FindTool looks up a tool by name. Returns nil if not found.
func (r *Registry) FindTool(name string) *ToolDef {
	if r == nil {
		return nil
	}
	for i := range r.Tools {
		if r.Tools[i].Name == name {
			return &r.Tools[i]
		}
	}
	return nil
}

// Role is the speaker role in a conversation.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// Image is an image attached to a message.
type Image struct {
	URL       string // HTTP/HTTPS URL, or empty if using Base64
	Base64    string // base64-encoded image data, or empty if using URL
	MediaType string // "image/jpeg", "image/png", "image/gif", "image/webp"
}

// Message is the cross-provider message abstraction (user-facing).
// Tool calls and tool results are managed internally by the conversation loop
// and do not appear in the public Message type.
type Message struct {
	Role   Role
	Text   string
	Images []Image
}

// Usage tracks token consumption across a conversation turn.
type Usage struct {
	InputTokens  int
	OutputTokens int
}

// Add returns a new Usage that is the sum of u and other.
func (u Usage) Add(other Usage) Usage {
	return Usage{
		InputTokens:  u.InputTokens + other.InputTokens,
		OutputTokens: u.OutputTokens + other.OutputTokens,
	}
}

// ToolCallLog records a single tool invocation that occurred during a turn.
type ToolCallLog struct {
	ToolName string
	Input    json.RawMessage
	Output   json.RawMessage
	Error    error
	Duration time.Duration
}

// Response is what the Client returns after a complete conversation turn.
type Response struct {
	Text           string        // the model's final text response
	Usage          Usage         // total token counts across all round-trips
	ToolCalls      []ToolCallLog // log of all tool calls made during this turn
	ConversationID string        // public_id of the llm_conversations row (empty if no persister)
}

// ── Provider interface & related types ────────────────────────────────────────

// ProviderRequest is what the conversation loop sends to the provider.
type ProviderRequest struct {
	System      string            // system prompt (top-level for Anthropic, injected as message for OpenAI)
	Messages    []ProviderMessage // full conversation history in provider-agnostic format
	Tools       []ToolDef         // available tools (may be empty)
	WebSearch   *WebSearchConfig
	MaxTokens   int
	Temperature *float64
}

// ProviderMessage is the internal message representation used in conversation
// history sent to providers. Unlike the public Message type (which only carries
// user-facing fields), this includes tool calls and tool results.
type ProviderMessage struct {
	Role        Role
	Text        string
	Images      []Image
	ToolCalls   []ToolCall   // non-empty when the model requested tool invocations
	ToolResults []ToolResult // non-empty when sending tool execution results back
}

// ToolCall represents a single tool invocation request from the model.
type ToolCall struct {
	ID       string // provider-specific call ID (needed for result routing)
	ToolName string
	ArgsJSON json.RawMessage // raw JSON arguments
}

// ToolResult is a tool execution result sent back to the provider.
type ToolResult struct {
	ToolCallID string          // matches ToolCall.ID
	Output     json.RawMessage // result JSON (or error message encoded as JSON string)
	IsError    bool
}

// WebSearchConfig holds cross-provider web search settings.
type WebSearchConfig struct {
	MaxResults     int
	AllowedDomains []string
	BlockedDomains []string
}

// ProviderResponse is the normalised response from the provider after one
// request. The conversation loop may make multiple ProviderRequests per
// user turn when tool calls are involved.
type ProviderResponse struct {
	Text      string     // text content from the model (may be empty if only tool calls)
	ToolCalls []ToolCall // tool calls the model wants to make (empty when done)
	Usage     Usage
	Done      bool // true when the model has no further tool calls to make
}

// StreamEventType enumerates the kinds of events a streaming provider emits.
type StreamEventType int

const (
	StreamTextDelta     StreamEventType = iota // a text chunk arrived
	StreamToolCallStart                        // the model is beginning a tool call
	StreamToolCallDelta                        // additional argument bytes for an in-progress tool call
	StreamDone                                 // the stream is complete
	StreamError                                // an error occurred mid-stream
)

// StreamEvent is a single event from a streaming provider response.
type StreamEvent struct {
	Type     StreamEventType
	Text     string    // non-empty for StreamTextDelta
	ToolCall *ToolCall // non-nil for StreamToolCallStart / StreamToolCallDelta (ArgsJSON may be partial)
	Usage    *Usage    // non-nil for StreamDone
	Done     bool      // true for StreamDone
	Err      error     // non-nil for StreamError
}

// ErrStreamingNotSupported is returned by Provider.SendStream when the provider
// does not support streaming. The client falls back to Provider.Send.
var ErrStreamingNotSupported = errorString("llm: streaming not supported by this provider")

// errorString is a trivial error implementation so we can declare sentinel
// errors without importing errors (avoiding an import cycle in tests).
type errorString string

func (e errorString) Error() string { return string(e) }

// Provider translates between the cross-provider abstraction and a specific
// LLM API's wire format.
type Provider interface {
	// Name returns a human-readable provider name (e.g., "openai", "anthropic").
	Name() string

	// ModelName returns the model identifier (e.g., "claude-sonnet-4-20250514").
	ModelName() string

	// Send sends a conversation request and returns the complete response.
	Send(ctx context.Context, req *ProviderRequest) (*ProviderResponse, error)

	// SendStream sends a conversation request and returns a channel of streaming
	// events. The channel is closed when the stream ends (Done event or error).
	// Implementations that do not support streaming should return
	// ErrStreamingNotSupported; the client will fall back to Send.
	SendStream(ctx context.Context, req *ProviderRequest) (<-chan StreamEvent, error)
}

// ErrorStrategy controls what happens when a tool function returns an error.
type ErrorStrategy int

const (
	// SendErrorToModel sends the error message back to the model as the tool
	// result so it can retry or explain. This is the default — many tool errors
	// are recoverable ("city not found", validation failures, etc.).
	SendErrorToModel ErrorStrategy = iota

	// AbortOnToolError stops the conversation loop immediately and returns the
	// error to the caller.
	AbortOnToolError
)
