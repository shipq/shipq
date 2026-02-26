package openai

import "encoding/json"

// ── Request types ─────────────────────────────────────────────────────────────

// chatRequest is the JSON body sent to /v1/chat/completions.
type chatRequest struct {
	Model       string         `json:"model"`
	Messages    []chatMessage  `json:"messages"`
	Tools       []toolDef      `json:"tools,omitempty"`
	ToolChoice  any            `json:"tool_choice,omitempty"`
	MaxTokens   int            `json:"max_tokens,omitempty"`
	Temperature *float64       `json:"temperature,omitempty"`
	Stream      bool           `json:"stream,omitempty"`
	StreamOpts  *streamOptions `json:"stream_options,omitempty"`
}

type streamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

// chatMessage is a single message in the messages array.
// The Content field is typed as any because it can be:
//   - a plain string (for most messages)
//   - []contentPart (for user messages with images)
//   - nil (for assistant messages that only have tool_calls)
type chatMessage struct {
	Role       string     `json:"role"`
	Content    any        `json:"content"`
	ToolCalls  []toolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
}

// contentPart is a single element in a multipart content array.
type contentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *imageURL `json:"image_url,omitempty"`
}

type imageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

// toolDef is how OpenAI represents a tool in the request.
type toolDef struct {
	Type     string      `json:"type"` // always "function"
	Function functionDef `json:"function"`
}

type functionDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters"`
	Strict      bool            `json:"strict,omitempty"`
}

// ── Tool call types (both request echo and response) ─────────────────────────

// toolCall appears in assistant messages (both in requests when echoing history
// and in responses when the model wants to invoke a tool).
type toolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"` // always "function"
	Function functionCall `json:"function"`
}

type functionCall struct {
	Name string `json:"name"`
	// Arguments is a JSON-encoded string — it requires a second json.Unmarshal
	// to obtain the actual arguments object. This is a key OpenAI wire format
	// quirk that differs from Anthropic (which provides a parsed object).
	Arguments string `json:"arguments"`
}

// ── Response types ────────────────────────────────────────────────────────────

// chatResponse is the JSON body returned by /v1/chat/completions (non-streaming).
type chatResponse struct {
	ID      string    `json:"id"`
	Object  string    `json:"object"`
	Created int64     `json:"created"`
	Model   string    `json:"model"`
	Choices []choice  `json:"choices"`
	Usage   usage     `json:"usage"`
	Error   *apiError `json:"error,omitempty"`
}

type choice struct {
	Index        int         `json:"index"`
	Message      chatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

type usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// apiError is the error body returned by the OpenAI API on 4xx/5xx responses.
type apiError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Param   string `json:"param"`
	Code    string `json:"code"`
}

func (e *apiError) Error() string {
	if e == nil {
		return ""
	}
	return e.Type + ": " + e.Message
}

// ── SSE / streaming types ─────────────────────────────────────────────────────

// streamChunk is a single SSE data payload from a streaming response.
// Each line starts with "data: " and the payload is a JSON object like this.
// The stream ends with the sentinel "data: [DONE]".
type streamChunk struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []streamChoice `json:"choices"`
	Usage   *usage         `json:"usage,omitempty"` // only present on the final chunk when stream_options.include_usage=true
}

type streamChoice struct {
	Index        int         `json:"index"`
	Delta        streamDelta `json:"delta"`
	FinishReason string      `json:"finish_reason"`
}

// streamDelta is the incremental content update within a streaming chunk.
type streamDelta struct {
	Role      string           `json:"role,omitempty"`
	Content   string           `json:"content,omitempty"`
	ToolCalls []streamToolCall `json:"tool_calls,omitempty"`
}

// streamToolCall is the incremental tool call representation in a delta.
// Unlike the non-streaming toolCall, the fields may be empty on most chunks —
// only the first chunk for a given index carries the ID and Name; subsequent
// chunks only carry additional Arguments fragments.
type streamToolCall struct {
	Index    int                 `json:"index"`
	ID       string              `json:"id,omitempty"`
	Type     string              `json:"type,omitempty"`
	Function streamFunctionDelta `json:"function"`
}

type streamFunctionDelta struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}
