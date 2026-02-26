package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/shipq/shipq/channel"
	"golang.org/x/sync/errgroup"
)

// Client manages LLM conversations with automatic streaming and persistence.
type Client struct {
	provider    Provider
	registry    *Registry
	channel     *channel.Channel // nil if not running inside a channel handler
	persister   Persister        // nil if persistence is disabled
	system      string
	maxIter     int
	maxTokens   int
	temperature *float64
	webSearch   *WebSearchConfig
	onError     ErrorStrategy
	name        string // optional name for multi-client contexts (empty = default)
	sequential  bool   // if true, tool calls execute one at a time
}

// Option configures a Client.
type Option func(*Client)

// WithTools registers a tool registry with the client.
func WithTools(r *Registry) Option {
	return func(c *Client) { c.registry = r }
}

// WithChannel wires a realtime channel into the client, enabling streaming
// of text deltas and tool call events to subscribers.
func WithChannel(ch *channel.Channel) Option {
	return func(c *Client) { c.channel = ch }
}

// WithPersister wires a Persister into the client, enabling automatic
// persistence of conversations and messages.
func WithPersister(p Persister) Option {
	return func(c *Client) { c.persister = p }
}

// WithSystem sets the system prompt for all conversations.
func WithSystem(prompt string) Option {
	return func(c *Client) { c.system = prompt }
}

// WithMaxIterations sets the maximum number of provider round-trips per turn.
// Defaults to 10.
func WithMaxIterations(n int) Option {
	return func(c *Client) { c.maxIter = n }
}

// WithMaxTokens sets the max_tokens parameter for each provider request.
func WithMaxTokens(n int) Option {
	return func(c *Client) { c.maxTokens = n }
}

// WithTemperature sets the temperature parameter for each provider request.
func WithTemperature(t float64) Option {
	return func(c *Client) { c.temperature = &t }
}

// WithWebSearch enables web search for providers that support it.
func WithWebSearch(cfg WebSearchConfig) Option {
	return func(c *Client) { c.webSearch = &cfg }
}

// WithErrorStrategy sets what happens when a tool returns an error.
// Default is SendErrorToModel.
func WithErrorStrategy(s ErrorStrategy) Option {
	return func(c *Client) { c.onError = s }
}

// WithSequentialToolCalls disables parallel tool execution. By default all
// tool calls in a single round-trip are dispatched concurrently.
func WithSequentialToolCalls() Option {
	return func(c *Client) { c.sequential = true }
}

// NewClient creates a new LLM client with the given provider and options.
func NewClient(provider Provider, opts ...Option) *Client {
	c := &Client{
		provider: provider,
		maxIter:  10,
		onError:  SendErrorToModel,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// ── Public conversation API ───────────────────────────────────────────────────

// Chat starts a new single-turn conversation with the given user message.
func (c *Client) Chat(ctx context.Context, userMessage string) (*Response, error) {
	return c.ChatWithHistory(ctx, []Message{
		{Role: RoleUser, Text: userMessage},
	})
}

// ChatWithHistory runs a conversation turn starting from an existing message
// history. The history must begin with a user message. Returns the model's
// final response after all tool calls have been resolved.
func (c *Client) ChatWithHistory(ctx context.Context, messages []Message) (*Response, error) {
	if len(messages) == 0 {
		return nil, fmt.Errorf("llm: ChatWithHistory requires at least one message")
	}

	// Convert public messages to internal ProviderMessages.
	history := make([]ProviderMessage, len(messages))
	for i, m := range messages {
		history[i] = ProviderMessage{
			Role:   m.Role,
			Text:   m.Text,
			Images: m.Images,
		}
	}

	return c.run(ctx, history)
}

// ── Core conversation loop ────────────────────────────────────────────────────

// run executes the conversation loop against the provider, handling tool calls,
// streaming, and persistence as configured.
func (c *Client) run(ctx context.Context, history []ProviderMessage) (*Response, error) {
	var (
		convID     int64
		publicID   string
		totalUsage Usage
		toolLogs   []ToolCallLog
	)

	// ── 1. Open persistence row ───────────────────────────────────────────────
	if c.persister != nil {
		row, err := c.persister.InsertConversation(ctx, InsertConversationParams{
			PublicID:  generatePublicID(),
			Provider:  c.provider.Name(),
			Model:     c.provider.ModelName(),
			Status:    ConversationStatusRunning,
			StartedAt: time.Now(),
		})
		if err != nil {
			return nil, fmt.Errorf("llm: insert conversation: %w", err)
		}
		convID = row.ID
		publicID = row.PublicID
	}

	// ── 2. Persist the initial user message(s) ────────────────────────────────
	if c.persister != nil && len(history) > 0 {
		last := history[len(history)-1]
		if last.Role == RoleUser {
			if err := c.persister.InsertMessage(ctx, InsertMessageParams{
				ConversationID: convID,
				Role:           MessageRoleUser,
				Content:        last.Text,
				CreatedAt:      time.Now(),
			}); err != nil {
				return nil, fmt.Errorf("llm: persist user message: %w", err)
			}
		}
	}

	var finalText string

	// ── 3. Main loop ──────────────────────────────────────────────────────────
	for iter := 0; iter < c.maxIter; iter++ {
		req := &ProviderRequest{
			System:      c.system,
			Messages:    history,
			MaxTokens:   c.maxTokens,
			Temperature: c.temperature,
			WebSearch:   c.webSearch,
		}
		if c.registry != nil {
			req.Tools = c.registry.Tools
		}

		// ── 3a. Call the provider ─────────────────────────────────────────────
		provResp, err := c.callProvider(ctx, req)
		if err != nil {
			c.failConversation(ctx, convID, totalUsage, len(toolLogs), err)
			return nil, err
		}

		totalUsage = totalUsage.Add(provResp.Usage)
		finalText = provResp.Text

		// ── 3b. Persist assistant message ─────────────────────────────────────
		if c.persister != nil {
			if err := c.persister.InsertMessage(ctx, InsertMessageParams{
				ConversationID: convID,
				Role:           MessageRoleAssistant,
				Content:        provResp.Text,
				CreatedAt:      time.Now(),
			}); err != nil {
				return nil, fmt.Errorf("llm: persist assistant message: %w", err)
			}
		}

		// ── 3c. Done? ─────────────────────────────────────────────────────────
		if len(provResp.ToolCalls) == 0 {
			break
		}

		// ── 3d. Dispatch tool calls ───────────────────────────────────────────
		toolResults, logs, err := c.dispatchToolCalls(ctx, convID, provResp.ToolCalls)
		if err != nil {
			// AbortOnToolError — propagate immediately.
			c.failConversation(ctx, convID, totalUsage, len(toolLogs), err)
			return nil, err
		}
		toolLogs = append(toolLogs, logs...)

		// ── 3e. Append assistant + tool result messages to history ────────────
		history = append(history, ProviderMessage{
			Role:      RoleAssistant,
			Text:      provResp.Text,
			ToolCalls: provResp.ToolCalls,
		})
		history = append(history, ProviderMessage{
			Role:        RoleUser,
			ToolResults: toolResults,
		})

		if iter == c.maxIter-1 {
			err := fmt.Errorf("llm: max iterations (%d) reached without a final response", c.maxIter)
			c.failConversation(ctx, convID, totalUsage, len(toolLogs), err)
			return nil, err
		}
	}

	// ── 4. Finalise persistence ───────────────────────────────────────────────
	if c.persister != nil {
		if err := c.persister.UpdateConversation(ctx, UpdateConversationParams{
			ID:            convID,
			Status:        ConversationStatusCompleted,
			InputTokens:   totalUsage.InputTokens,
			OutputTokens:  totalUsage.OutputTokens,
			ToolCallCount: len(toolLogs),
			CompletedAt:   time.Now(),
		}); err != nil {
			return nil, fmt.Errorf("llm: update conversation: %w", err)
		}
	}

	// ── 5. Publish LLMDone ────────────────────────────────────────────────────
	if err := publishDone(ctx, c.channel, finalText, totalUsage, len(toolLogs)); err != nil {
		return nil, fmt.Errorf("llm: publish done: %w", err)
	}

	return &Response{
		Text:           finalText,
		Usage:          totalUsage,
		ToolCalls:      toolLogs,
		ConversationID: publicID,
	}, nil
}

// ── Provider dispatch ─────────────────────────────────────────────────────────

// callProvider calls the provider, preferring streaming when a channel is
// wired in. Falls back to non-streaming Send when SendStream returns
// ErrStreamingNotSupported.
func (c *Client) callProvider(ctx context.Context, req *ProviderRequest) (*ProviderResponse, error) {
	if c.channel != nil {
		resp, err := c.callProviderStream(ctx, req)
		if err != nil && !errors.Is(err, ErrStreamingNotSupported) {
			return nil, err
		}
		if err == nil {
			return resp, nil
		}
		// Fall through to non-streaming.
	}
	return c.provider.Send(ctx, req)
}

// callProviderStream calls SendStream and drains the event channel, publishing
// text deltas to the channel and accumulating a complete ProviderResponse.
func (c *Client) callProviderStream(ctx context.Context, req *ProviderRequest) (*ProviderResponse, error) {
	events, err := c.provider.SendStream(ctx, req)
	if err != nil {
		return nil, err
	}

	var (
		textBuf   strings.Builder
		toolCalls []ToolCall
		usage     Usage
	)

	// pendingToolCall accumulates streamed tool call fragments by ID.
	type pendingTC struct {
		id   string
		name string
		args strings.Builder
	}
	pendingByID := map[string]*pendingTC{}

	for evt := range events {
		switch evt.Type {
		case StreamTextDelta:
			textBuf.WriteString(evt.Text)
			if err := publishTextDelta(ctx, c.channel, evt.Text); err != nil {
				return nil, fmt.Errorf("llm: publish text delta: %w", err)
			}

		case StreamToolCallDelta:
			if evt.ToolCall != nil {
				id := evt.ToolCall.ID
				if _, ok := pendingByID[id]; !ok {
					pendingByID[id] = &pendingTC{id: id, name: evt.ToolCall.ToolName}
				}
				if evt.ToolCall.ToolName != "" {
					pendingByID[id].name = evt.ToolCall.ToolName
				}
				pendingByID[id].args.Write(evt.ToolCall.ArgsJSON)
			}

		case StreamToolCallStart:
			// Provider has assembled a complete tool call.
			if evt.ToolCall != nil {
				toolCalls = append(toolCalls, ToolCall{
					ID:       evt.ToolCall.ID,
					ToolName: evt.ToolCall.ToolName,
					ArgsJSON: evt.ToolCall.ArgsJSON,
				})
			}

		case StreamDone:
			if evt.Usage != nil {
				usage = *evt.Usage
			}

		case StreamError:
			return nil, fmt.Errorf("llm: stream error: %w", evt.Err)
		}
	}

	// If tool calls were accumulated via deltas but not emitted as StreamToolCallStart,
	// flush them now.
	if len(toolCalls) == 0 && len(pendingByID) > 0 {
		for _, tc := range pendingByID {
			toolCalls = append(toolCalls, ToolCall{
				ID:       tc.id,
				ToolName: tc.name,
				ArgsJSON: json.RawMessage(tc.args.String()),
			})
		}
	}

	resp := &ProviderResponse{
		Text:      textBuf.String(),
		ToolCalls: toolCalls,
		Usage:     usage,
		Done:      len(toolCalls) == 0,
	}
	return resp, nil
}

// ── Tool dispatch ─────────────────────────────────────────────────────────────

// dispatchToolCalls executes all tool calls, either in parallel (default) or
// sequentially (when WithSequentialToolCalls is set).
//
// On AbortOnToolError, returns the first tool error immediately.
// On SendErrorToModel (default), errors are converted to ToolResult.IsError=true
// entries and returned as normal.
func (c *Client) dispatchToolCalls(
	ctx context.Context,
	convID int64,
	calls []ToolCall,
) ([]ToolResult, []ToolCallLog, error) {
	results := make([]ToolResult, len(calls))
	logs := make([]ToolCallLog, len(calls))

	if c.sequential {
		for i, tc := range calls {
			r, l, err := c.executeOne(ctx, convID, tc)
			if err != nil {
				return nil, nil, err
			}
			results[i] = r
			logs[i] = l
		}
		return results, logs, nil
	}

	// Parallel execution with errgroup.
	var mu sync.Mutex
	g, gctx := errgroup.WithContext(ctx)
	for i, tc := range calls {
		i, tc := i, tc // capture loop vars
		g.Go(func() error {
			r, l, err := c.executeOne(gctx, convID, tc)
			if err != nil {
				return err
			}
			mu.Lock()
			results[i] = r
			logs[i] = l
			mu.Unlock()
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, nil, err
	}
	return results, logs, nil
}

// executeOne executes a single tool call, publishes stream events, and
// persists the tool_call + tool_result messages.
func (c *Client) executeOne(ctx context.Context, convID int64, tc ToolCall) (ToolResult, ToolCallLog, error) {
	// Persist tool_call message.
	if c.persister != nil {
		if err := c.persister.InsertMessage(ctx, InsertMessageParams{
			ConversationID: convID,
			Role:           MessageRoleToolCall,
			Content:        string(tc.ArgsJSON),
			ToolName:       tc.ToolName,
			ToolCallID:     tc.ID,
			CreatedAt:      time.Now(),
		}); err != nil {
			return ToolResult{}, ToolCallLog{}, fmt.Errorf("llm: persist tool_call: %w", err)
		}
	}

	// Publish tool call start.
	if err := publishToolCallStart(ctx, c.channel, tc.ID, tc.ToolName, tc.ArgsJSON); err != nil {
		return ToolResult{}, ToolCallLog{}, fmt.Errorf("llm: publish tool call start: %w", err)
	}

	// Look up the tool.
	var toolDef *ToolDef
	if c.registry != nil {
		toolDef = c.registry.FindTool(tc.ToolName)
	}

	start := time.Now()
	var (
		resultJSON []byte
		execErr    error
	)

	if toolDef == nil {
		execErr = fmt.Errorf("tool %q not found in registry", tc.ToolName)
	} else {
		resultJSON, execErr = toolDef.Func(ctx, tc.ArgsJSON)
	}

	duration := time.Since(start)
	durationMs := int(duration.Milliseconds())

	log := ToolCallLog{
		ToolName: tc.ToolName,
		Input:    tc.ArgsJSON,
		Output:   resultJSON,
		Error:    execErr,
		Duration: duration,
	}

	var (
		toolResult ToolResult
		errMsg     string
	)

	if execErr != nil {
		switch c.onError {
		case AbortOnToolError:
			return ToolResult{}, ToolCallLog{}, fmt.Errorf("llm: tool %q: %w", tc.ToolName, execErr)
		default: // SendErrorToModel
			errMsg = execErr.Error()
			errJSON, _ := json.Marshal(errMsg)
			toolResult = ToolResult{
				ToolCallID: tc.ID,
				Output:     errJSON,
				IsError:    true,
			}
		}
	} else {
		toolResult = ToolResult{
			ToolCallID: tc.ID,
			Output:     resultJSON,
			IsError:    false,
		}
	}

	// Publish tool call result.
	if err := publishToolCallResult(ctx, c.channel, tc.ID, tc.ToolName, toolResult.Output, errMsg, durationMs); err != nil {
		return ToolResult{}, ToolCallLog{}, fmt.Errorf("llm: publish tool call result: %w", err)
	}

	// Persist tool_result message.
	if c.persister != nil {
		resultContent := string(toolResult.Output)
		if execErr != nil {
			resultContent = execErr.Error()
		}
		if err := c.persister.InsertMessage(ctx, InsertMessageParams{
			ConversationID: convID,
			Role:           MessageRoleToolResult,
			Content:        resultContent,
			ToolName:       tc.ToolName,
			ToolCallID:     tc.ID,
			IsError:        toolResult.IsError,
			CreatedAt:      time.Now(),
		}); err != nil {
			return ToolResult{}, ToolCallLog{}, fmt.Errorf("llm: persist tool_result: %w", err)
		}
	}

	return toolResult, log, nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// failConversation marks the conversation as failed. Errors from this call are
// swallowed — the original error takes precedence.
func (c *Client) failConversation(ctx context.Context, convID int64, usage Usage, toolCallCount int, cause error) {
	if c.persister == nil || convID == 0 {
		return
	}
	_ = c.persister.UpdateConversation(ctx, UpdateConversationParams{
		ID:            convID,
		Status:        ConversationStatusFailed,
		InputTokens:   usage.InputTokens,
		OutputTokens:  usage.OutputTokens,
		ToolCallCount: toolCallCount,
		CompletedAt:   time.Now(),
		ErrorMessage:  cause.Error(),
	})
}

// generatePublicID generates a simple unique public ID for a conversation.
// In production this would use the nanoid package; here we use a time-based
// fallback to avoid adding a dependency in the library package.
func generatePublicID() string {
	return fmt.Sprintf("conv_%d", time.Now().UnixNano())
}
