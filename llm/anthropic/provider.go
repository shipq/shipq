package anthropic

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/shipq/shipq/llm"
)

const defaultBaseURL = "https://api.anthropic.com/v1"
const defaultVersion = "2023-06-01"
const defaultMaxTokens = 4096

// Provider implements llm.Provider for the Anthropic Messages API.
type Provider struct {
	apiKey     string
	model      string
	baseURL    string
	version    string
	httpClient *http.Client
}

// Option configures an Anthropic Provider.
type Option func(*Provider)

// WithBaseURL overrides the API base URL (e.g. for proxies).
func WithBaseURL(url string) Option {
	return func(p *Provider) { p.baseURL = strings.TrimRight(url, "/") }
}

// WithHTTPClient replaces the default http.Client (useful in tests with httptest.Server).
func WithHTTPClient(client *http.Client) Option {
	return func(p *Provider) { p.httpClient = client }
}

// WithAnthropicVersion overrides the anthropic-version header (default: "2023-06-01").
func WithAnthropicVersion(v string) Option {
	return func(p *Provider) { p.version = v }
}

// New creates a new Anthropic provider.
func New(apiKey, model string, opts ...Option) *Provider {
	p := &Provider{
		apiKey:     apiKey,
		model:      model,
		baseURL:    defaultBaseURL,
		version:    defaultVersion,
		httpClient: http.DefaultClient,
	}
	for _, o := range opts {
		o(p)
	}
	return p
}

// Name returns "anthropic".
func (p *Provider) Name() string { return "anthropic" }

// ModelName returns the configured model identifier.
func (p *Provider) ModelName() string { return p.model }

// ── Send (non-streaming) ──────────────────────────────────────────────────────

// Send sends a conversation request and returns the complete response.
func (p *Provider) Send(ctx context.Context, req *llm.ProviderRequest) (*llm.ProviderResponse, error) {
	body := p.buildRequest(req, false)

	respBytes, err := p.post(ctx, "/messages", body)
	if err != nil {
		return nil, err
	}

	var mr messagesResponse
	if err := json.Unmarshal(respBytes, &mr); err != nil {
		return nil, fmt.Errorf("anthropic: unmarshal response: %w", err)
	}
	if mr.Error != nil {
		return nil, fmt.Errorf("anthropic: %w", mr.Error)
	}
	// The API can also return an error envelope with type="error" at the top
	// level even with a 200 status in some edge cases.
	if mr.Type == "error" && mr.Error != nil {
		return nil, fmt.Errorf("anthropic: %w", mr.Error)
	}

	return p.parseResponse(&mr)
}

// ── SendStream (SSE streaming) ────────────────────────────────────────────────

// SendStream sends a conversation request and streams back events via a channel.
// The caller must drain the channel until it is closed.
func (p *Provider) SendStream(ctx context.Context, req *llm.ProviderRequest) (<-chan llm.StreamEvent, error) {
	body := p.buildRequest(req, true)

	rc, err := p.postStream(ctx, "/messages", body)
	if err != nil {
		return nil, err
	}

	events := make(chan llm.StreamEvent, 32)
	go func() {
		defer close(events)
		p.consumeSSE(ctx, rc, events)
		rc.Close()
	}()
	return events, nil
}

// consumeSSE reads the Anthropic SSE stream and converts events to StreamEvents.
//
// Anthropic SSE uses paired "event: <type>" + "data: <json>" lines.
// Each content block has a lifecycle: content_block_start → N×content_block_delta → content_block_stop.
func (p *Provider) consumeSSE(ctx context.Context, rc io.ReadCloser, events chan<- llm.StreamEvent) {
	// Close rc when ctx is cancelled so scanner.Scan() unblocks immediately
	// rather than waiting for the HTTP transport to time out.
	stop := context.AfterFunc(ctx, func() { rc.Close() })
	defer stop()

	// pendingBlock tracks an in-progress content block.
	type pendingBlock struct {
		blockType string // "text" or "tool_use"
		id        string
		name      string
		textBuf   strings.Builder
		inputBuf  strings.Builder
	}

	blocks := make(map[int]*pendingBlock)

	scanner := bufio.NewScanner(rc)
	var currentEvent string

	for scanner.Scan() {
		// Check for context cancellation between lines.
		select {
		case <-ctx.Done():
			events <- llm.StreamEvent{Type: llm.StreamError, Err: ctx.Err()}
			return
		default:
		}

		line := scanner.Text()

		if strings.HasPrefix(line, "event: ") {
			currentEvent = strings.TrimPrefix(line, "event: ")
			continue
		}

		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")

		switch currentEvent {
		case "content_block_start":
			var e sseContentBlockStart
			if err := json.Unmarshal([]byte(data), &e); err != nil {
				events <- llm.StreamEvent{Type: llm.StreamError, Err: fmt.Errorf("anthropic: parse content_block_start: %w", err)}
				return
			}
			blocks[e.Index] = &pendingBlock{
				blockType: e.ContentBlock.Type,
				id:        e.ContentBlock.ID,
				name:      e.ContentBlock.Name,
			}
			// If the block starts with text content (rare but possible), capture it.
			if e.ContentBlock.Text != "" {
				blocks[e.Index].textBuf.WriteString(e.ContentBlock.Text)
			}

		case "content_block_delta":
			var e sseContentBlockDelta
			if err := json.Unmarshal([]byte(data), &e); err != nil {
				events <- llm.StreamEvent{Type: llm.StreamError, Err: fmt.Errorf("anthropic: parse content_block_delta: %w", err)}
				return
			}
			b, ok := blocks[e.Index]
			if !ok {
				continue
			}
			switch e.Delta.Type {
			case "text_delta":
				b.textBuf.WriteString(e.Delta.Text)
				events <- llm.StreamEvent{Type: llm.StreamTextDelta, Text: e.Delta.Text}
			case "input_json_delta":
				b.inputBuf.WriteString(e.Delta.PartialJSON)
				// Emit a StreamToolCallDelta so the client can show streaming
				// progress. ArgsJSON here is just the partial fragment.
				events <- llm.StreamEvent{
					Type: llm.StreamToolCallDelta,
					ToolCall: &llm.ToolCall{
						ID:       b.id,
						ToolName: b.name,
						ArgsJSON: json.RawMessage(e.Delta.PartialJSON),
					},
				}
			}

		case "content_block_stop":
			// Find which block just completed and, if it's a tool_use, emit
			// a StreamToolCallStart with the fully assembled ArgsJSON.
			// We need the index — parse it from the data.
			var e struct {
				Index int `json:"index"`
			}
			if err := json.Unmarshal([]byte(data), &e); err != nil {
				continue
			}
			b, ok := blocks[e.Index]
			if !ok {
				continue
			}
			if b.blockType == "tool_use" {
				events <- llm.StreamEvent{
					Type: llm.StreamToolCallStart,
					ToolCall: &llm.ToolCall{
						ID:       b.id,
						ToolName: b.name,
						ArgsJSON: json.RawMessage(b.inputBuf.String()),
					},
				}
			}
			delete(blocks, e.Index)

		case "message_delta":
			var e sseMessageDelta
			if err := json.Unmarshal([]byte(data), &e); err != nil {
				continue
			}
			// message_delta carries the final output token count.
			if e.Usage != nil {
				u := &llm.Usage{
					OutputTokens: e.Usage.OutputTokens,
				}
				events <- llm.StreamEvent{Type: llm.StreamDone, Done: true, Usage: u}
				return
			}

		case "message_stop":
			events <- llm.StreamEvent{Type: llm.StreamDone, Done: true}
			return

		case "error":
			var e sseError
			if err := json.Unmarshal([]byte(data), &e); err != nil {
				events <- llm.StreamEvent{Type: llm.StreamError, Err: fmt.Errorf("anthropic: stream error (unparseable): %s", data)}
				return
			}
			if e.Error != nil {
				events <- llm.StreamEvent{Type: llm.StreamError, Err: fmt.Errorf("anthropic: %w", e.Error)}
			} else {
				events <- llm.StreamEvent{Type: llm.StreamError, Err: fmt.Errorf("anthropic: unknown stream error")}
			}
			return

		case "message_start", "ping":
			// message_start carries input token count which we don't need here
			// (it will be picked up from the non-streaming path or accumulated
			// by the client). ping is a keep-alive; both are silently ignored.
		}
	}

	if err := scanner.Err(); err != nil {
		// Distinguish a real scan error from a close caused by context cancellation.
		if ctx.Err() != nil {
			events <- llm.StreamEvent{Type: llm.StreamError, Err: ctx.Err()}
		} else {
			events <- llm.StreamEvent{Type: llm.StreamError, Err: fmt.Errorf("anthropic: scan SSE: %w", err)}
		}
	} else if ctx.Err() != nil {
		// Scanner returned EOF because we closed rc on context cancellation.
		events <- llm.StreamEvent{Type: llm.StreamError, Err: ctx.Err()}
	}
}

// ── Request building ──────────────────────────────────────────────────────────

func (p *Provider) buildRequest(req *llm.ProviderRequest, stream bool) messagesRequest {
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = defaultMaxTokens
	}
	mr := messagesRequest{
		Model:       p.model,
		MaxTokens:   maxTokens,
		System:      req.System,
		Messages:    p.convertMessages(req.Messages),
		Tools:       p.convertTools(req.Tools, req.WebSearch),
		Temperature: req.Temperature,
		Stream:      stream,
	}
	return mr
}

// convertMessages translates the provider-agnostic message list into the
// Anthropic messages array.
//
// Key differences from OpenAI:
//   - System prompt is a top-level field, NOT a message.
//   - Images use a different content block shape ("image" with a "source" sub-object).
//   - Tool results go as role:"user" messages with "tool_result" content blocks.
//   - Tool calls returned by the model go as role:"assistant" messages with
//     "tool_use" content blocks where Input is already a parsed JSON object.
func (p *Provider) convertMessages(msgs []llm.ProviderMessage) []message {
	out := make([]message, 0, len(msgs))
	for _, m := range msgs {
		switch {
		case len(m.ToolResults) > 0:
			// Tool results: role:"user" with one tool_result block per result.
			blocks := make([]contentBlock, 0, len(m.ToolResults))
			for _, tr := range m.ToolResults {
				blocks = append(blocks, contentBlock{
					Type:      "tool_result",
					ToolUseID: tr.ToolCallID,
					Content:   string(tr.Output),
					IsError:   tr.IsError,
				})
			}
			out = append(out, message{Role: "user", Content: blocks})

		case len(m.ToolCalls) > 0:
			// Assistant message with tool_use blocks.
			blocks := make([]contentBlock, 0, len(m.ToolCalls))
			if m.Text != "" {
				blocks = append(blocks, contentBlock{Type: "text", Text: m.Text})
			}
			for _, tc := range m.ToolCalls {
				blocks = append(blocks, contentBlock{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.ToolName,
					Input: tc.ArgsJSON,
				})
			}
			out = append(out, message{Role: "assistant", Content: blocks})

		default:
			out = append(out, p.convertTextMessage(m))
		}
	}
	return out
}

// convertTextMessage converts a ProviderMessage that has text (and optional
// images) to an Anthropic message.
func (p *Provider) convertTextMessage(m llm.ProviderMessage) message {
	blocks := make([]contentBlock, 0, 1+len(m.Images))

	for _, img := range m.Images {
		b := contentBlock{Type: "image"}
		if img.URL != "" {
			b.Source = &imageSource{Type: "url", URL: img.URL}
		} else if img.Base64 != "" {
			b.Source = &imageSource{
				Type:      "base64",
				MediaType: img.MediaType,
				Data:      img.Base64,
			}
		}
		blocks = append(blocks, b)
	}

	if m.Text != "" {
		blocks = append(blocks, contentBlock{Type: "text", Text: m.Text})
	}

	return message{Role: string(m.Role), Content: blocks}
}

// convertTools translates ToolDefs to the Anthropic tool definition format.
// When WebSearch is configured, the web_search_20250305 server tool is appended.
func (p *Provider) convertTools(tools []llm.ToolDef, ws *llm.WebSearchConfig) []toolDef {
	if len(tools) == 0 && ws == nil {
		return nil
	}
	out := make([]toolDef, 0, len(tools)+1)
	for _, t := range tools {
		out = append(out, toolDef{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		})
	}
	if ws != nil {
		out = append(out, toolDef{
			Type: "web_search_20250305",
			Name: "web_search",
		})
	}
	return out
}

// ── Response parsing ──────────────────────────────────────────────────────────

func (p *Provider) parseResponse(mr *messagesResponse) (*llm.ProviderResponse, error) {
	resp := &llm.ProviderResponse{
		Usage: llm.Usage{
			InputTokens:  mr.Usage.InputTokens,
			OutputTokens: mr.Usage.OutputTokens,
		},
		Done: mr.StopReason == "end_turn" || mr.StopReason == "max_tokens",
	}

	for _, block := range mr.Content {
		switch block.Type {
		case "text":
			resp.Text += block.Text
		case "tool_use":
			// NOTE: block.Input is already a parsed JSON object — no second
			// json.Unmarshal is needed. This is a key difference from OpenAI,
			// where Arguments is a JSON string requiring a second unmarshal.
			resp.ToolCalls = append(resp.ToolCalls, llm.ToolCall{
				ID:       block.ID,
				ToolName: block.Name,
				ArgsJSON: block.Input,
			})
		}
	}

	if len(resp.ToolCalls) > 0 {
		resp.Done = false
	}

	return resp, nil
}

// ── HTTP helpers ──────────────────────────────────────────────────────────────

func (p *Provider) post(ctx context.Context, path string, body any) ([]byte, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return nil, fmt.Errorf("anthropic: encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+path, &buf)
	if err != nil {
		return nil, fmt.Errorf("anthropic: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", p.version)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("anthropic: http: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("anthropic: read body: %w", err)
	}

	if resp.StatusCode >= 400 {
		var errEnv struct {
			Error *apiError `json:"error"`
		}
		if jsonErr := json.Unmarshal(respBytes, &errEnv); jsonErr == nil && errEnv.Error != nil {
			return nil, fmt.Errorf("anthropic HTTP %d: %w", resp.StatusCode, errEnv.Error)
		}
		return nil, fmt.Errorf("anthropic HTTP %d: %s", resp.StatusCode, string(respBytes))
	}

	return respBytes, nil
}

func (p *Provider) postStream(ctx context.Context, path string, body any) (io.ReadCloser, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return nil, fmt.Errorf("anthropic: encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+path, &buf)
	if err != nil {
		return nil, fmt.Errorf("anthropic: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", p.version)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("anthropic: http: %w", err)
	}

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		var errEnv struct {
			Error *apiError `json:"error"`
		}
		if jsonErr := json.Unmarshal(b, &errEnv); jsonErr == nil && errEnv.Error != nil {
			return nil, fmt.Errorf("anthropic HTTP %d: %w", resp.StatusCode, errEnv.Error)
		}
		return nil, fmt.Errorf("anthropic HTTP %d: %s", resp.StatusCode, string(b))
	}

	return resp.Body, nil
}
