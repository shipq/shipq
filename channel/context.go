package channel

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// contextKey is an unexported type used as the key for storing a Channel in context.Value.
type contextKey struct{}

// Channel is the runtime handle that channel handlers use to send and receive
// messages over a realtime transport. It is injected into the handler's context
// by the generated worker code.
//
// Send publishes messages to all subscribers (including the worker's own subscription).
// Receive blocks until a message of the requested type arrives or the context is cancelled.
//
// Echo handling: Because the realtime transport (e.g., Centrifugo) delivers all
// publications to ALL subscribers -- including the publisher -- the worker's own
// Send calls will appear as echoes on the incoming channel. Receive handles this
// via per-type buffering: since server→client types (FromServer) and client→server
// types (FromClient) always have distinct names in our protocol, echoes are naturally
// buffered as non-matching types and never returned in place of genuine client messages.
// Do NOT call Receive(typeName) for a type the worker also Sends, or the echo will
// be returned instead of a genuine client message.
type Channel struct {
	name      string
	jobID     string
	accountID int64 // 0 for public channels
	orgID     int64 // 0 if unscoped or public
	isPublic  bool
	transport RealtimeTransport
	incoming  <-chan []byte
	cleanup   func() // tears down the transport subscription when the job finishes
	mu        sync.Mutex
	buffers   map[string][]Envelope // per-type message buffers
}

// NewChannel creates a new Channel. This is typically called by generated worker code,
// not by user handlers directly.
func NewChannel(
	name string,
	jobID string,
	accountID int64,
	orgID int64,
	isPublic bool,
	transport RealtimeTransport,
	incoming <-chan []byte,
	cleanup func(),
) *Channel {
	return &Channel{
		name:      name,
		jobID:     jobID,
		accountID: accountID,
		orgID:     orgID,
		isPublic:  isPublic,
		transport: transport,
		incoming:  incoming,
		cleanup:   cleanup,
		buffers:   make(map[string][]Envelope),
	}
}

// FromContext retrieves the Channel from the context.
// Panics if no Channel is present -- this indicates a programming error
// (the handler is being called outside the channel runtime).
func FromContext(ctx context.Context) *Channel {
	ch, ok := ctx.Value(contextKey{}).(*Channel)
	if !ok || ch == nil {
		panic("channel.FromContext: no Channel in context -- is this handler running inside the channel runtime?")
	}
	return ch
}

// WithChannel injects a Channel into the context.
func WithChannel(ctx context.Context, ch *Channel) context.Context {
	return context.WithValue(ctx, contextKey{}, ch)
}

// Send publishes a typed message to all subscribers of this channel.
// The message is wrapped in an Envelope with the given type name and
// published via the underlying RealtimeTransport.
func (ch *Channel) Send(ctx context.Context, msgType string, data []byte) error {
	env := Envelope{
		Type: msgType,
		Data: json.RawMessage(data),
	}
	return ch.transport.Publish(ch.channelID(), env.Marshal())
}

// Receive blocks until a message of the specified type arrives on the channel,
// or until the context is cancelled.
//
// Messages of non-matching types that arrive while waiting are buffered in
// per-type buffers so they can be retrieved by subsequent Receive calls for
// those types. This naturally handles echoes of the worker's own Send calls
// (see Channel doc comment).
func (ch *Channel) Receive(ctx context.Context, msgType string) ([]byte, error) {
	// Check the buffer first.
	if env, ok := ch.popFromBuffer(msgType); ok {
		return env.Data, nil
	}

	// Block on the incoming channel until we get a matching type or context is done.
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case raw, ok := <-ch.incoming:
			if !ok {
				return nil, fmt.Errorf("channel.Receive: incoming channel closed")
			}
			var env Envelope
			if err := env.Unmarshal(raw); err != nil {
				return nil, fmt.Errorf("channel.Receive: unmarshal envelope: %w", err)
			}
			if env.Type == msgType {
				return env.Data, nil
			}
			// Buffer non-matching types.
			ch.mu.Lock()
			ch.buffers[env.Type] = append(ch.buffers[env.Type], env)
			ch.mu.Unlock()
		}
	}
}

// ReceiveAny blocks until any message arrives on the channel, regardless of type.
// It checks all per-type buffers first, then waits on the incoming channel.
// Returns the type name, raw data, and any error.
func (ch *Channel) ReceiveAny(ctx context.Context) (typeName string, data []byte, err error) {
	// Check buffers first -- return the first buffered message from any type.
	ch.mu.Lock()
	for t, buf := range ch.buffers {
		if len(buf) > 0 {
			env := buf[0]
			ch.buffers[t] = buf[1:]
			if len(ch.buffers[t]) == 0 {
				delete(ch.buffers, t)
			}
			ch.mu.Unlock()
			return env.Type, env.Data, nil
		}
	}
	ch.mu.Unlock()

	// Block on the incoming channel.
	select {
	case <-ctx.Done():
		return "", nil, ctx.Err()
	case raw, ok := <-ch.incoming:
		if !ok {
			return "", nil, fmt.Errorf("channel.ReceiveAny: incoming channel closed")
		}
		var env Envelope
		if err := env.Unmarshal(raw); err != nil {
			return "", nil, fmt.Errorf("channel.ReceiveAny: unmarshal envelope: %w", err)
		}
		return env.Type, env.Data, nil
	}
}

// Cleanup tears down the transport subscription. Called by the generated worker
// code when the channel handler finishes.
func (ch *Channel) Cleanup() {
	if ch.cleanup != nil {
		ch.cleanup()
	}
}

// Name returns the channel's registered name.
func (ch *Channel) Name() string {
	return ch.name
}

// JobID returns the unique job identifier for this channel instance.
func (ch *Channel) JobID() string {
	return ch.jobID
}

// popFromBuffer removes and returns the first buffered envelope of the given type.
func (ch *Channel) popFromBuffer(msgType string) (Envelope, bool) {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	buf, ok := ch.buffers[msgType]
	if !ok || len(buf) == 0 {
		return Envelope{}, false
	}

	env := buf[0]
	ch.buffers[msgType] = buf[1:]
	if len(ch.buffers[msgType]) == 0 {
		delete(ch.buffers, msgType)
	}
	return env, true
}

// channelID returns the transport-level channel name.
// Uses underscores (not colons) to avoid Centrifugo namespace interpretation.
// Format: name_accountID_jobID (or name_public_jobID for public channels).
func (ch *Channel) channelID() string {
	if ch.isPublic {
		return ch.name + "_public_" + ch.jobID
	}
	return fmt.Sprintf("%s_%d_%s", ch.name, ch.accountID, ch.jobID)
}
