package anthropic

import "encoding/json"

// ── Request types ─────────────────────────────────────────────────────────────

// messagesRequest is the JSON body sent to /v1/messages.
type messagesRequest struct {
	Model       string    `json:"model"`
	MaxTokens   int       `json:"max_tokens"`
	System      string    `json:"system,omitempty"`
	Messages    []message `json:"messages"`
	Tools       []toolDef `json:"tools,omitempty"`
	Temperature *float64  `json:"temperature,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
}

// message is a single turn in the conversation.
// Content is typed as []contentBlock (always an array in Anthropic's API —
// even a plain text turn is represented as a one-element array).
type message struct {
	Role    string         `json:"role"`
	Content []contentBlock `json:"content"`
}

// contentBlock is a polymorphic content element. The Type field selects which
// other fields are relevant:
//
//	"text"        → Text
//	"image"       → Source
//	"tool_use"    → ID, Name, Input
//	"tool_result" → ToolUseID, Content (string or []contentBlock), IsError
type contentBlock struct {
	Type string `json:"type"`

	// type = "text"
	Text string `json:"text,omitempty"`

	// type = "image"
	Source *imageSource `json:"source,omitempty"`

	// type = "tool_use"
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`

	// type = "tool_result"
	ToolUseID string `json:"tool_use_id,omitempty"`
	// Content for tool_result can be a plain string or an array of content
	// blocks. We always send a plain string for simplicity.
	Content interface{} `json:"content,omitempty"`
	IsError bool        `json:"is_error,omitempty"`
}

// imageSource describes how an image is provided to the model.
type imageSource struct {
	// Type is either "base64" or "url".
	Type      string `json:"type"`
	MediaType string `json:"media_type,omitempty"`
	Data      string `json:"data,omitempty"`
	URL       string `json:"url,omitempty"`
}

// toolDef is how Anthropic represents a tool in the request.
// Unlike OpenAI, Anthropic uses "input_schema" (not "parameters") and does
// not wrap the definition in a "function" sub-object.
// Server tools (e.g. web_search_20250305) use a different shape with only
// Type and Name — InputSchema is omitted.
type toolDef struct {
	// Type is present only for server tools (e.g. "web_search_20250305").
	// For user-defined tools this field is omitted.
	Type string `json:"type,omitempty"`

	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema,omitempty"`
}

// ── Response types ────────────────────────────────────────────────────────────

// messagesResponse is the JSON body returned by /v1/messages (non-streaming).
type messagesResponse struct {
	ID           string          `json:"id"`
	Type         string          `json:"type"`
	Role         string          `json:"role"`
	Content      []responseBlock `json:"content"`
	Model        string          `json:"model"`
	StopReason   string          `json:"stop_reason"`
	StopSequence *string         `json:"stop_sequence"`
	Usage        responseUsage   `json:"usage"`

	// Error fields — present when type = "error".
	Error *apiError `json:"error,omitempty"`
}

// responseBlock is a content block in the response. The Type field selects
// which fields are meaningful:
//
//	"text"     → Text
//	"tool_use" → ID, Name, Input (already-parsed JSON object, NOT a string)
type responseBlock struct {
	Type string `json:"type"`

	// type = "text"
	Text string `json:"text,omitempty"`

	// type = "tool_use"
	// ID and Name identify the tool call.
	// Input is a parsed JSON object — unlike OpenAI, there is no second
	// json.Unmarshal required.
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

type responseUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// apiError is the error shape returned by Anthropic on 4xx/5xx responses
// and also embedded in streaming error events.
type apiError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

func (e *apiError) Error() string {
	if e == nil {
		return ""
	}
	return e.Type + ": " + e.Message
}

// ── SSE / streaming types ─────────────────────────────────────────────────────
//
// Anthropic SSE uses paired "event:" + "data:" lines. The event type
// determines how to interpret the data payload.
//
// Key event types:
//   message_start         → message metadata + initial usage
//   content_block_start   → a new content block begins (text or tool_use)
//   content_block_delta   → incremental update to the current block
//   content_block_stop    → current block is complete
//   message_delta         → message-level delta (stop_reason, output tokens)
//   message_stop          → the entire message is complete
//   ping                  → keep-alive; ignored
//   error                 → stream-level error

// sseMessageStart carries the initial message metadata.
type sseMessageStart struct {
	Type    string           `json:"type"`
	Message messagesResponse `json:"message"`
}

// sseContentBlockStart signals the beginning of a new content block.
type sseContentBlockStart struct {
	Type         string        `json:"type"`
	Index        int           `json:"index"`
	ContentBlock responseBlock `json:"content_block"`
}

// sseContentBlockDelta carries an incremental delta for the current block.
type sseContentBlockDelta struct {
	Type  string   `json:"type"`
	Index int      `json:"index"`
	Delta sseDelta `json:"delta"`
}

// sseDelta is the discriminated union of delta types.
// Type is one of:
//
//	"text_delta"       → Text contains the new text fragment
//	"input_json_delta" → PartialJSON contains a fragment of the tool input JSON
type sseDelta struct {
	Type        string `json:"type"`
	Text        string `json:"text,omitempty"`
	PartialJSON string `json:"partial_json,omitempty"`
}

// sseMessageDelta carries the final stop_reason and output token count.
type sseMessageDelta struct {
	Type  string           `json:"type"`
	Delta messageDeltaBody `json:"delta"`
	Usage *responseUsage   `json:"usage,omitempty"`
}

type messageDeltaBody struct {
	StopReason   string  `json:"stop_reason"`
	StopSequence *string `json:"stop_sequence"`
}

// sseError carries a stream-level error.
type sseError struct {
	Type  string    `json:"type"`
	Error *apiError `json:"error"`
}
