package channel

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// TestRecorder serves double duty: it's both a RealtimeTransport implementation
// (for injection into Channel) and a test assertion helper. Because Channel
// depends on the RealtimeTransport interface (not a concrete client), the test
// recorder slots in naturally.
type TestRecorder struct {
	mu         sync.Mutex
	Sent       []Envelope
	incomingCh chan []byte
}

// Compile-time interface check.
var _ RealtimeTransport = (*TestRecorder)(nil)

// NewTestRecorder creates a new TestRecorder with a buffered incoming channel.
func NewTestRecorder() *TestRecorder {
	return &TestRecorder{
		Sent:       make([]Envelope, 0),
		incomingCh: make(chan []byte, 64),
	}
}

// Publish captures the message in Sent for later assertion.
// Implements RealtimeTransport.
func (tr *TestRecorder) Publish(channel string, data []byte) error {
	var env Envelope
	if err := env.Unmarshal(data); err != nil {
		return fmt.Errorf("TestRecorder.Publish: unmarshal envelope: %w", err)
	}
	tr.mu.Lock()
	tr.Sent = append(tr.Sent, env)
	tr.mu.Unlock()
	return nil
}

// Subscribe returns the internal incomingCh, a no-op cleanup, and nil error.
// Implements RealtimeTransport.
func (tr *TestRecorder) Subscribe(channel string, subscriberID string) (<-chan []byte, func(), error) {
	return tr.incomingCh, func() {}, nil
}

// GenerateConnectionToken returns a deterministic test token.
// Implements RealtimeTransport.
func (tr *TestRecorder) GenerateConnectionToken(sub string, ttl time.Duration) (string, error) {
	return "test-conn-token-" + sub, nil
}

// GenerateSubscriptionToken returns a deterministic test token.
// Implements RealtimeTransport.
func (tr *TestRecorder) GenerateSubscriptionToken(sub string, channel string, ttl time.Duration) (string, error) {
	return "test-sub-token-" + channel, nil
}

// ConnectionURL returns a fixed test URL.
// Implements RealtimeTransport.
func (tr *TestRecorder) ConnectionURL() string {
	return "ws://test/connection/websocket"
}

// EnqueueIncoming marshals the given value into an Envelope and sends it on
// the internal incoming channel. This simulates a client sending a message.
func (tr *TestRecorder) EnqueueIncoming(typeName string, msg any) {
	data, err := json.Marshal(msg)
	if err != nil {
		panic("TestRecorder.EnqueueIncoming: marshal msg: " + err.Error())
	}
	env := Envelope{
		Type: typeName,
		Data: json.RawMessage(data),
	}
	tr.incomingCh <- env.Marshal()
}

// HasSent returns true if at least one message of the given type was sent.
func (tr *TestRecorder) HasSent(typeName string) bool {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	for _, env := range tr.Sent {
		if env.Type == typeName {
			return true
		}
	}
	return false
}

// SentOfType returns all sent envelopes matching the given type name.
func (tr *TestRecorder) SentOfType(typeName string) []Envelope {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	var result []Envelope
	for _, env := range tr.Sent {
		if env.Type == typeName {
			result = append(result, env)
		}
	}
	return result
}

// WithTestChannel creates a Channel backed by the given TestRecorder and injects
// it into the context. This is the primary way to set up a Channel for unit tests.
func WithTestChannel(ctx context.Context, recorder *TestRecorder) context.Context {
	incoming, cleanup, _ := recorder.Subscribe("test-channel", "test-subscriber")
	ch := NewChannel(
		"test",     // name
		"test-job", // jobID
		0,          // accountID
		0,          // orgID
		false,      // isPublic
		recorder,   // transport
		incoming,   // incoming
		cleanup,    // cleanup
	)
	return WithChannel(ctx, ch)
}
