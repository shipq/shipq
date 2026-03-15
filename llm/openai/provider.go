package openai

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

const defaultBaseURL = "https://api.openai.com/v1"

// Provider implements llm.Provider for the OpenAI Chat Completions API.
type Provider struct {
	apiKey     string
	model      string
	baseURL    string
	strictMode bool
	httpClient *http.Client
}

// Option configures an OpenAI Provider.
type Option func(*Provider)

// WithStrictMode enables strict: true on all function definitions (default: true).
func WithStrictMode(enabled bool) Option {
	return func(p *Provider) { p.strictMode = enabled }
}

// WithBaseURL overrides the API base URL (e.g. for Azure OpenAI or proxies).
func WithBaseURL(url string) Option {
	return func(p *Provider) { p.baseURL = strings.TrimRight(url, "/") }
}

// WithHTTPClient replaces the default http.Client (useful in tests with httptest.Server).
func WithHTTPClient(client *http.Client) Option {
	return func(p *Provider) { p.httpClient = client }
}

// New creates a new OpenAI provider.
func New(apiKey, model string, opts ...Option) *Provider {
	p := &Provider{
		apiKey:     apiKey,
		model:      model,
		baseURL:    defaultBaseURL,
		strictMode: true,
		httpClient: http.DefaultClient,
	}
	for _, o := range opts {
		o(p)
	}
	return p
}

// Name returns "openai".
func (p *Provider) Name() string { return "openai" }

// ModelName returns the configured model identifier.
func (p *Provider) ModelName() string { return p.model }

// ── Send (non-streaming) ──────────────────────────────────────────────────────

// Send sends a conversation request and returns the complete response.
func (p *Provider) Send(ctx context.Context, req *llm.ProviderRequest) (*llm.ProviderResponse, error) {
	body := p.buildRequest(req, false)

	respBytes, err := p.post(ctx, "/chat/completions", body)
	if err != nil {
		return nil, err
	}

	var cr chatResponse
	if err := json.Unmarshal(respBytes, &cr); err != nil {
		return nil, fmt.Errorf("openai: unmarshal response: %w", err)
	}
	if cr.Error != nil {
		return nil, fmt.Errorf("openai: %w", cr.Error)
	}
	if len(cr.Choices) == 0 {
		return nil, fmt.Errorf("openai: empty choices in response")
	}

	return p.parseResponse(&cr)
}

// ── SendStream (SSE streaming) ────────────────────────────────────────────────

// SendStream sends a conversation request and streams back events via a channel.
// The caller must drain the channel until it is closed.
func (p *Provider) SendStream(ctx context.Context, req *llm.ProviderRequest) (<-chan llm.StreamEvent, error) {
	body := p.buildRequest(req, true)
	// Request usage on the final chunk so callers get token counts.
	body.StreamOpts = &streamOptions{IncludeUsage: true}

	rc, err := p.postStream(ctx, "/chat/completions", body)
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

// consumeSSE reads SSE data lines from rc and converts them to StreamEvents.
func (p *Provider) consumeSSE(ctx context.Context, rc io.ReadCloser, events chan<- llm.StreamEvent) {
	// Close rc when ctx is cancelled so scanner.Scan() unblocks immediately
	// rather than waiting for the HTTP transport to time out.
	stop := context.AfterFunc(ctx, func() { rc.Close() })
	defer stop()
	// accumulatedToolCalls tracks tool call state across multiple delta chunks.
	type pendingToolCall struct {
		id   string
		name string
		args strings.Builder
	}
	var pending []pendingToolCall

	scanner := bufio.NewScanner(rc)
	for scanner.Scan() {
		// Check for context cancellation between lines.
		select {
		case <-ctx.Done():
			events <- llm.StreamEvent{Type: llm.StreamError, Err: ctx.Err()}
			return
		default:
		}

		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			// Emit accumulated tool calls as StreamToolCallStart events with
			// their complete ArgsJSON, then a Done event.
			for _, tc := range pending {
				events <- llm.StreamEvent{
					Type: llm.StreamToolCallStart,
					ToolCall: &llm.ToolCall{
						ID:       tc.id,
						ToolName: tc.name,
						ArgsJSON: json.RawMessage(tc.args.String()),
					},
				}
			}
			events <- llm.StreamEvent{Type: llm.StreamDone, Done: true}
			return
		}

		var chunk streamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			// Malformed chunk — emit an error and stop.
			events <- llm.StreamEvent{Type: llm.StreamError, Err: fmt.Errorf("openai: parse chunk: %w", err)}
			return
		}

		// Capture usage from the final usage-only chunk.
		if chunk.Usage != nil && len(chunk.Choices) == 0 {
			u := &llm.Usage{
				InputTokens:  chunk.Usage.PromptTokens,
				OutputTokens: chunk.Usage.CompletionTokens,
			}
			events <- llm.StreamEvent{Type: llm.StreamDone, Done: true, Usage: u}
			return
		}

		for _, ch := range chunk.Choices {
			d := ch.Delta
			if d.Content != "" {
				events <- llm.StreamEvent{Type: llm.StreamTextDelta, Text: d.Content}
			}
			for _, stc := range d.ToolCalls {
				// Grow slice as needed.
				for len(pending) <= stc.Index {
					pending = append(pending, pendingToolCall{})
				}
				if stc.ID != "" {
					pending[stc.Index].id = stc.ID
				}
				if stc.Function.Name != "" {
					pending[stc.Index].name = stc.Function.Name
				}
				if stc.Function.Arguments != "" {
					pending[stc.Index].args.WriteString(stc.Function.Arguments)
					// Emit a delta event so the client can show progress.
					events <- llm.StreamEvent{
						Type: llm.StreamToolCallDelta,
						ToolCall: &llm.ToolCall{
							ID:       pending[stc.Index].id,
							ToolName: pending[stc.Index].name,
							// ArgsJSON here is just the partial fragment.
							ArgsJSON: json.RawMessage(stc.Function.Arguments),
						},
					}
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		// Distinguish a real scan error from a close caused by context cancellation.
		if ctx.Err() != nil {
			events <- llm.StreamEvent{Type: llm.StreamError, Err: ctx.Err()}
		} else {
			events <- llm.StreamEvent{Type: llm.StreamError, Err: fmt.Errorf("openai: scan SSE: %w", err)}
		}
	} else if ctx.Err() != nil {
		// Scanner returned EOF because we closed rc on context cancellation.
		events <- llm.StreamEvent{Type: llm.StreamError, Err: ctx.Err()}
	}
}

// ── Request building ──────────────────────────────────────────────────────────

func (p *Provider) buildRequest(req *llm.ProviderRequest, stream bool) chatRequest {
	cr := chatRequest{
		Model:       p.model,
		Messages:    p.convertMessages(req),
		Tools:       p.convertTools(req.Tools),
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Stream:      stream,
	}
	if req.WebSearch != nil {
		cr.WebSearchOptions = &webSearchOptions{}
	}
	if len(cr.Tools) > 0 {
		cr.ToolChoice = "auto"
	}
	return cr
}

// convertMessages translates the provider-agnostic message list into the
// OpenAI messages array, including the system prompt if set.
func (p *Provider) convertMessages(req *llm.ProviderRequest) []chatMessage {
	msgs := make([]chatMessage, 0, len(req.Messages)+1)

	if req.System != "" {
		msgs = append(msgs, chatMessage{Role: "system", Content: req.System})
	}

	for _, m := range req.Messages {
		switch {
		case len(m.ToolResults) > 0:
			// Tool results: each result becomes its own role:"tool" message.
			for _, tr := range m.ToolResults {
				content := string(tr.Output)
				msgs = append(msgs, chatMessage{
					Role:       "tool",
					Content:    content,
					ToolCallID: tr.ToolCallID,
				})
			}

		case len(m.ToolCalls) > 0:
			// Assistant message that requested tool calls.
			tcs := make([]toolCall, len(m.ToolCalls))
			for i, tc := range m.ToolCalls {
				tcs[i] = toolCall{
					ID:   tc.ID,
					Type: "function",
					Function: functionCall{
						Name:      tc.ToolName,
						Arguments: string(tc.ArgsJSON),
					},
				}
			}
			msgs = append(msgs, chatMessage{
				Role:      string(m.Role),
				Content:   nil,
				ToolCalls: tcs,
			})

		default:
			// Normal text (and optional images) message.
			msgs = append(msgs, p.convertTextMessage(m))
		}
	}
	return msgs
}

// convertTextMessage converts a ProviderMessage that has text (and optional
// images) to a chatMessage.
func (p *Provider) convertTextMessage(m llm.ProviderMessage) chatMessage {
	if len(m.Images) == 0 {
		return chatMessage{Role: string(m.Role), Content: m.Text}
	}
	// Multipart: text + image_url parts.
	parts := make([]contentPart, 0, 1+len(m.Images))
	if m.Text != "" {
		parts = append(parts, contentPart{Type: "text", Text: m.Text})
	}
	for _, img := range m.Images {
		url := img.URL
		if url == "" && img.Base64 != "" {
			url = "data:" + img.MediaType + ";base64," + img.Base64
		}
		parts = append(parts, contentPart{
			Type:     "image_url",
			ImageURL: &imageURL{URL: url, Detail: "auto"},
		})
	}
	return chatMessage{Role: string(m.Role), Content: parts}
}

// convertTools translates ToolDefs to the OpenAI tool definition format.
func (p *Provider) convertTools(tools []llm.ToolDef) []toolDef {
	if len(tools) == 0 {
		return nil
	}
	out := make([]toolDef, 0, len(tools))
	for _, t := range tools {
		out = append(out, toolDef{
			Type: "function",
			Function: functionDef{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
				Strict:      p.strictMode,
			},
		})
	}
	return out
}

// ── Response parsing ──────────────────────────────────────────────────────────

func (p *Provider) parseResponse(cr *chatResponse) (*llm.ProviderResponse, error) {
	msg := cr.Choices[0].Message
	finishReason := cr.Choices[0].FinishReason

	// Content is typed as any because it can be a string, a []contentPart, or
	// nil (when the model only returned tool calls). Coerce safely.
	var text string
	if s, ok := msg.Content.(string); ok {
		text = s
	}

	resp := &llm.ProviderResponse{
		Text: text,
		Usage: llm.Usage{
			InputTokens:  cr.Usage.PromptTokens,
			OutputTokens: cr.Usage.CompletionTokens,
		},
		Done: finishReason == "stop" || finishReason == "length",
	}

	for _, tc := range msg.ToolCalls {
		// NOTE: tc.Function.Arguments is a JSON string — must use the value
		// directly as json.RawMessage (it is already valid JSON).
		resp.ToolCalls = append(resp.ToolCalls, llm.ToolCall{
			ID:       tc.ID,
			ToolName: tc.Function.Name,
			ArgsJSON: json.RawMessage(tc.Function.Arguments),
		})
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
		return nil, fmt.Errorf("openai: encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+path, &buf)
	if err != nil {
		return nil, fmt.Errorf("openai: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai: http: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("openai: read body: %w", err)
	}

	if resp.StatusCode >= 400 {
		msg := string(respBytes)
		var apiErr struct {
			Error *apiError `json:"error"`
		}
		if jsonErr := json.Unmarshal(respBytes, &apiErr); jsonErr == nil && apiErr.Error != nil {
			msg = apiErr.Error.Error()
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			return nil, &llm.RateLimitError{
				StatusCode: resp.StatusCode,
				RetryAfter: llm.ParseRetryAfter(resp.Header.Get("Retry-After"), msg),
				Message:    msg,
			}
		}
		if apiErr.Error != nil {
			return nil, fmt.Errorf("openai HTTP %d: %w", resp.StatusCode, apiErr.Error)
		}
		return nil, fmt.Errorf("openai HTTP %d: %s", resp.StatusCode, string(respBytes))
	}

	return respBytes, nil
}

func (p *Provider) postStream(ctx context.Context, path string, body any) (io.ReadCloser, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return nil, fmt.Errorf("openai: encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+path, &buf)
	if err != nil {
		return nil, fmt.Errorf("openai: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai: http: %w", err)
	}

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		msg := string(b)
		var apiErr struct {
			Error *apiError `json:"error"`
		}
		if jsonErr := json.Unmarshal(b, &apiErr); jsonErr == nil && apiErr.Error != nil {
			msg = apiErr.Error.Error()
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			return nil, &llm.RateLimitError{
				StatusCode: resp.StatusCode,
				RetryAfter: llm.ParseRetryAfter(resp.Header.Get("Retry-After"), msg),
				Message:    msg,
			}
		}
		if apiErr.Error != nil {
			return nil, fmt.Errorf("openai HTTP %d: %w", resp.StatusCode, apiErr.Error)
		}
		return nil, fmt.Errorf("openai HTTP %d: %s", resp.StatusCode, string(b))
	}

	return resp.Body, nil
}
