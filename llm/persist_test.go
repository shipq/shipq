package llm

import (
	"context"
	"testing"
	"time"
)

// ── mock persister ─────────────────────────────────────────────────────────────

type mockPersister struct {
	conversations []InsertConversationParams
	updates       []UpdateConversationParams
	messages      []InsertMessageParams
	nextID        int64
}

func newMockPersister() *mockPersister {
	return &mockPersister{nextID: 1}
}

func (m *mockPersister) InsertConversation(_ context.Context, p InsertConversationParams) (ConversationRow, error) {
	m.conversations = append(m.conversations, p)
	row := ConversationRow{
		ID:       m.nextID,
		PublicID: p.PublicID,
	}
	m.nextID++
	return row, nil
}

func (m *mockPersister) UpdateConversation(_ context.Context, p UpdateConversationParams) error {
	m.updates = append(m.updates, p)
	return nil
}

func (m *mockPersister) InsertMessage(_ context.Context, p InsertMessageParams) error {
	m.messages = append(m.messages, p)
	return nil
}

func (m *mockPersister) ListCompletedTools(_ context.Context, jobID string) ([]string, error) {
	seen := make(map[string]bool)
	var result []string
	for _, msg := range m.messages {
		if msg.Role == MessageRoleToolCall && msg.ToolName != "" {
			if !seen[msg.ToolName] {
				seen[msg.ToolName] = true
				result = append(result, msg.ToolName)
			}
		}
	}
	return result, nil
}

// Compile-time check: mockPersister satisfies Persister.
var _ Persister = (*mockPersister)(nil)

// ── InsertConversationParams field coverage ───────────────────────────────────

func TestInsertConversationParamsFields(t *testing.T) {
	p := InsertConversationParams{
		PublicID:    "abc123",
		AccountID:   42,
		OrgID:       7,
		ChannelName: "chat",
		JobID:       "job-xyz",
		Provider:    "openai",
		Model:       "gpt-4.1",
		Status:      ConversationStatusRunning,
		StartedAt:   time.Now(),
	}

	if p.PublicID != "abc123" {
		t.Errorf("PublicID: got %q", p.PublicID)
	}
	if p.AccountID != 42 {
		t.Errorf("AccountID: got %d", p.AccountID)
	}
	if p.OrgID != 7 {
		t.Errorf("OrgID: got %d", p.OrgID)
	}
	if p.ChannelName != "chat" {
		t.Errorf("ChannelName: got %q", p.ChannelName)
	}
	if p.JobID != "job-xyz" {
		t.Errorf("JobID: got %q", p.JobID)
	}
	if p.Provider != "openai" {
		t.Errorf("Provider: got %q", p.Provider)
	}
	if p.Model != "gpt-4.1" {
		t.Errorf("Model: got %q", p.Model)
	}
	if p.Status != ConversationStatusRunning {
		t.Errorf("Status: got %q", p.Status)
	}
}

// ── UpdateConversationParams field coverage ───────────────────────────────────

func TestUpdateConversationParamsFields(t *testing.T) {
	now := time.Now()
	p := UpdateConversationParams{
		ID:            99,
		Status:        ConversationStatusCompleted,
		InputTokens:   120,
		OutputTokens:  60,
		ToolCallCount: 3,
		CompletedAt:   now,
		ErrorMessage:  "",
	}

	if p.ID != 99 {
		t.Errorf("ID: got %d", p.ID)
	}
	if p.Status != ConversationStatusCompleted {
		t.Errorf("Status: got %q", p.Status)
	}
	if p.InputTokens != 120 {
		t.Errorf("InputTokens: got %d", p.InputTokens)
	}
	if p.OutputTokens != 60 {
		t.Errorf("OutputTokens: got %d", p.OutputTokens)
	}
	if p.ToolCallCount != 3 {
		t.Errorf("ToolCallCount: got %d", p.ToolCallCount)
	}
	if !p.CompletedAt.Equal(now) {
		t.Errorf("CompletedAt: got %v, want %v", p.CompletedAt, now)
	}
}

func TestUpdateConversationParamsFailedStatus(t *testing.T) {
	p := UpdateConversationParams{
		ID:           1,
		Status:       ConversationStatusFailed,
		ErrorMessage: "context deadline exceeded",
	}
	if p.Status != ConversationStatusFailed {
		t.Errorf("Status: got %q", p.Status)
	}
	if p.ErrorMessage == "" {
		t.Error("ErrorMessage should be non-empty for failed status")
	}
}

// ── InsertMessageParams field coverage ───────────────────────────────────────

func TestInsertMessageParamsUserMessage(t *testing.T) {
	p := InsertMessageParams{
		ConversationID: 5,
		Role:           MessageRoleUser,
		Content:        "What's the weather in Tokyo?",
		CreatedAt:      time.Now(),
	}
	if p.ConversationID != 5 {
		t.Errorf("ConversationID: got %d", p.ConversationID)
	}
	if p.Role != MessageRoleUser {
		t.Errorf("Role: got %q", p.Role)
	}
	if p.Content != "What's the weather in Tokyo?" {
		t.Errorf("Content: got %q", p.Content)
	}
	if p.ToolName != "" {
		t.Errorf("ToolName should be empty for user message, got %q", p.ToolName)
	}
	if p.ToolCallID != "" {
		t.Errorf("ToolCallID should be empty for user message, got %q", p.ToolCallID)
	}
	if p.IsError {
		t.Error("IsError should be false for user message")
	}
}

func TestInsertMessageParamsToolCall(t *testing.T) {
	p := InsertMessageParams{
		ConversationID: 5,
		Role:           MessageRoleToolCall,
		Content:        `{"city":"Tokyo"}`,
		ToolName:       "get_weather",
		ToolCallID:     "call_abc123",
		CreatedAt:      time.Now(),
	}
	if p.Role != MessageRoleToolCall {
		t.Errorf("Role: got %q", p.Role)
	}
	if p.ToolName != "get_weather" {
		t.Errorf("ToolName: got %q", p.ToolName)
	}
	if p.ToolCallID != "call_abc123" {
		t.Errorf("ToolCallID: got %q", p.ToolCallID)
	}
	if p.IsError {
		t.Error("IsError should be false for a successful tool call")
	}
}

func TestInsertMessageParamsToolResult(t *testing.T) {
	p := InsertMessageParams{
		ConversationID: 5,
		Role:           MessageRoleToolResult,
		Content:        `{"weather":"sunny"}`,
		ToolName:       "get_weather",
		ToolCallID:     "call_abc123",
		IsError:        false,
		CreatedAt:      time.Now(),
	}
	if p.Role != MessageRoleToolResult {
		t.Errorf("Role: got %q", p.Role)
	}
	if p.IsError {
		t.Error("IsError should be false for a successful result")
	}
}

func TestInsertMessageParamsToolResultError(t *testing.T) {
	p := InsertMessageParams{
		ConversationID: 5,
		Role:           MessageRoleToolResult,
		Content:        "city not found",
		ToolName:       "get_weather",
		ToolCallID:     "call_abc123",
		IsError:        true,
		CreatedAt:      time.Now(),
	}
	if !p.IsError {
		t.Error("IsError should be true for an error result")
	}
}

// ── ConversationStatus constants ──────────────────────────────────────────────

func TestConversationStatusValues(t *testing.T) {
	statuses := []ConversationStatus{
		ConversationStatusRunning,
		ConversationStatusCompleted,
		ConversationStatusFailed,
	}
	want := []string{"running", "completed", "failed"}
	for i, s := range statuses {
		if string(s) != want[i] {
			t.Errorf("status[%d]: got %q, want %q", i, s, want[i])
		}
	}
}

// ── MessageRole constants ─────────────────────────────────────────────────────

func TestMessageRoleValues(t *testing.T) {
	roles := []MessageRole{
		MessageRoleUser,
		MessageRoleAssistant,
		MessageRoleToolCall,
		MessageRoleToolResult,
	}
	want := []string{"user", "assistant", "tool_call", "tool_result"}
	for i, r := range roles {
		if string(r) != want[i] {
			t.Errorf("role[%d]: got %q, want %q", i, r, want[i])
		}
	}
}

// ── mock persister round-trip ─────────────────────────────────────────────────

func TestMockPersisterInsertConversation(t *testing.T) {
	ctx := context.Background()
	mp := newMockPersister()

	row, err := mp.InsertConversation(ctx, InsertConversationParams{
		PublicID:  "pub-001",
		Provider:  "anthropic",
		Model:     "claude-sonnet-4-20250514",
		Status:    ConversationStatusRunning,
		StartedAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("InsertConversation: %v", err)
	}
	if row.ID != 1 {
		t.Errorf("ID: got %d, want 1", row.ID)
	}
	if row.PublicID != "pub-001" {
		t.Errorf("PublicID: got %q, want pub-001", row.PublicID)
	}
	if len(mp.conversations) != 1 {
		t.Errorf("conversations recorded: got %d, want 1", len(mp.conversations))
	}
}

func TestMockPersisterUpdateConversation(t *testing.T) {
	ctx := context.Background()
	mp := newMockPersister()

	err := mp.UpdateConversation(ctx, UpdateConversationParams{
		ID:            1,
		Status:        ConversationStatusCompleted,
		InputTokens:   200,
		OutputTokens:  80,
		ToolCallCount: 1,
		CompletedAt:   time.Now(),
	})
	if err != nil {
		t.Fatalf("UpdateConversation: %v", err)
	}
	if len(mp.updates) != 1 {
		t.Errorf("updates recorded: got %d, want 1", len(mp.updates))
	}
	if mp.updates[0].InputTokens != 200 {
		t.Errorf("InputTokens: got %d, want 200", mp.updates[0].InputTokens)
	}
}

func TestMockPersisterInsertMessage(t *testing.T) {
	ctx := context.Background()
	mp := newMockPersister()

	msgs := []InsertMessageParams{
		{ConversationID: 1, Role: MessageRoleUser, Content: "hello", CreatedAt: time.Now()},
		{ConversationID: 1, Role: MessageRoleAssistant, Content: "hi there", CreatedAt: time.Now()},
	}
	for _, p := range msgs {
		if err := mp.InsertMessage(ctx, p); err != nil {
			t.Fatalf("InsertMessage: %v", err)
		}
	}
	if len(mp.messages) != 2 {
		t.Errorf("messages recorded: got %d, want 2", len(mp.messages))
	}
	if mp.messages[0].Role != MessageRoleUser {
		t.Errorf("messages[0].Role: got %q, want %q", mp.messages[0].Role, MessageRoleUser)
	}
	if mp.messages[1].Role != MessageRoleAssistant {
		t.Errorf("messages[1].Role: got %q, want %q", mp.messages[1].Role, MessageRoleAssistant)
	}
}

func TestMockPersisterIDAutoIncrement(t *testing.T) {
	ctx := context.Background()
	mp := newMockPersister()

	for i := 0; i < 3; i++ {
		row, err := mp.InsertConversation(ctx, InsertConversationParams{
			PublicID:  "pub",
			Status:    ConversationStatusRunning,
			StartedAt: time.Now(),
		})
		if err != nil {
			t.Fatalf("InsertConversation[%d]: %v", i, err)
		}
		if row.ID != int64(i+1) {
			t.Errorf("row[%d].ID: got %d, want %d", i, row.ID, i+1)
		}
	}
}
