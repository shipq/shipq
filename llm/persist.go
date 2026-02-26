package llm

import (
	"context"
	"time"
)

// Persister abstracts LLM conversation persistence.
// The runtime library defines this interface; a generated adapter (produced in
// Phase 3) provides a concrete implementation backed by SQL querydefs.
//
// All methods are called by the conversation loop automatically when a
// Persister is wired into the Client via WithPersister. If no Persister is
// configured the loop skips all persistence calls silently.
type Persister interface {
	// InsertConversation creates a new conversation row and returns it.
	InsertConversation(ctx context.Context, p InsertConversationParams) (ConversationRow, error)

	// UpdateConversation updates the status, token counts, and tool call count
	// on an existing conversation row once the turn is complete.
	UpdateConversation(ctx context.Context, p UpdateConversationParams) error

	// InsertMessage appends a single message to an existing conversation.
	InsertMessage(ctx context.Context, p InsertMessageParams) error
}

// ConversationStatus mirrors the CHECK constraint on llm_conversations.status.
type ConversationStatus string

const (
	ConversationStatusRunning   ConversationStatus = "running"
	ConversationStatusCompleted ConversationStatus = "completed"
	ConversationStatusFailed    ConversationStatus = "failed"
)

// MessageRole mirrors the CHECK constraint on llm_messages.role.
// Re-using the Role type would create a dependency on the provider abstraction;
// keeping a separate type here lets the persist layer remain self-contained.
type MessageRole string

const (
	MessageRoleUser       MessageRole = "user"
	MessageRoleAssistant  MessageRole = "assistant"
	MessageRoleToolCall   MessageRole = "tool_call"
	MessageRoleToolResult MessageRole = "tool_result"
)

// ── InsertConversation ────────────────────────────────────────────────────────

// InsertConversationParams carries all the data needed to open a new
// conversation row. Callers supply the public_id; the DB assigns the
// surrogate integer primary key.
type InsertConversationParams struct {
	// PublicID is the externally visible identifier (e.g. a nanoid).
	PublicID string

	// AccountID scopes the conversation to a tenant. 0 means unscoped /
	// anonymous (e.g. a public channel).
	AccountID int64

	// OrgID optionally scopes the conversation to an organisation within the
	// account. 0 means unscoped.
	OrgID int64

	// ChannelName is the registered channel name that initiated this
	// conversation (e.g. "chat"). Empty when the client is used outside a
	// channel context.
	ChannelName string

	// JobID is the Machinery / task-queue job ID for the channel invocation.
	// Empty when the client is used outside a channel context.
	JobID string

	// Provider is the human-readable provider name (e.g. "openai",
	// "anthropic").
	Provider string

	// Model is the model identifier (e.g. "gpt-4.1",
	// "claude-sonnet-4-20250514").
	Model string

	// Status is the initial status — callers should always pass
	// ConversationStatusRunning here.
	Status ConversationStatus

	// StartedAt is when the conversation turn began. Defaults to time.Now()
	// in the adapter implementation if the zero value is supplied.
	StartedAt time.Time
}

// ConversationRow is the minimal projection of an llm_conversations row that
// the conversation loop needs after inserting.
type ConversationRow struct {
	// ID is the surrogate integer primary key assigned by the database.
	ID int64

	// PublicID is the externally visible identifier echoed back from the DB.
	PublicID string
}

// ── UpdateConversation ────────────────────────────────────────────────────────

// UpdateConversationParams carries the fields written back once a conversation
// turn finishes (successfully or with an error).
type UpdateConversationParams struct {
	// ID is the surrogate primary key of the row to update.
	ID int64

	// Status is the final status of the conversation.
	Status ConversationStatus

	// InputTokens is the total number of input tokens consumed across all
	// provider round-trips during this turn.
	InputTokens int

	// OutputTokens is the total number of output tokens produced across all
	// provider round-trips during this turn.
	OutputTokens int

	// ToolCallCount is the total number of tool invocations that took place
	// during this turn.
	ToolCallCount int

	// CompletedAt is when the conversation turn finished.
	CompletedAt time.Time

	// ErrorMessage holds the error description when Status is
	// ConversationStatusFailed. Empty on success.
	ErrorMessage string
}

// ── InsertMessage ─────────────────────────────────────────────────────────────

// InsertMessageParams carries all the data for a single message row.
type InsertMessageParams struct {
	// PublicID is the externally visible identifier (e.g. a nanoid).
	PublicID string

	// ConversationID is the surrogate primary key of the parent conversation.
	ConversationID int64

	// Role identifies who sent the message.
	Role MessageRole

	// Content holds the text payload. For tool_call messages this is the raw
	// JSON arguments; for tool_result messages it is the raw JSON result (or
	// an error description).
	Content string

	// ToolName is the name of the tool for tool_call and tool_result rows.
	// Empty for user and assistant messages.
	ToolName string

	// ToolCallID is the provider-assigned call ID for tool_call and
	// tool_result rows. Empty for user and assistant messages.
	ToolCallID string

	// IsError is true when the role is tool_result and the tool returned an
	// error rather than a successful output.
	IsError bool

	// CreatedAt is the wall-clock time when the message was produced.
	CreatedAt time.Time
}
