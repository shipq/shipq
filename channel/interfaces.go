package channel

import (
	"context"
	"time"
)

// RealtimeTransport abstracts the realtime messaging layer (e.g., Centrifugo, Pusher, Ably).
// All generated code and the channel runtime depend only on this interface.
type RealtimeTransport interface {
	// Publish sends a message from the server to all subscribers of the channel.
	Publish(channel string, data []byte) error

	// Subscribe creates a server-side subscription to the channel, returning
	// a channel that receives incoming messages (from clients), a cleanup
	// function that tears down the subscription, and any error.
	// The incoming channel must use non-blocking sends internally.
	Subscribe(channel string, subscriberID string) (incoming <-chan []byte, cleanup func(), err error)

	// GenerateConnectionToken creates a short-lived token that authorizes
	// a client to connect to the realtime transport. The meaning of `sub`
	// is transport-specific (e.g., user ID for Centrifugo JWT `sub` claim).
	GenerateConnectionToken(sub string, ttl time.Duration) (string, error)

	// GenerateSubscriptionToken creates a short-lived token that authorizes
	// a client to subscribe to a specific channel.
	GenerateSubscriptionToken(sub string, channel string, ttl time.Duration) (string, error)

	// ConnectionURL returns the WebSocket (or equivalent) URL clients should
	// connect to. This is returned to the frontend via the token endpoint.
	ConnectionURL() string
}

// TaskQueue abstracts the async job dispatch + worker mechanism (e.g., Machinery, Asynq, SQS).
// The generated worker main and HTTP dispatch routes depend only on this interface.
type TaskQueue interface {
	// RegisterTask registers a named task handler function.
	// The handler receives a JSON payload string and returns an error.
	RegisterTask(name string, handler func(string) error) error

	// SendTask enqueues a task for async execution.
	SendTask(name string, payload string, opts TaskOptions) error

	// StartWorker begins consuming tasks. It blocks until Stop is called or
	// the context is cancelled. The tag identifies this worker instance.
	// Concurrency controls the number of parallel task goroutines.
	StartWorker(ctx context.Context, tag string, concurrency int) error

	// StopWorker signals the worker to finish in-flight tasks and stop consuming.
	StopWorker() error
}

// TaskOptions configures per-task dispatch behavior.
type TaskOptions struct {
	RetryCount    int // max retries; 0 = no retries
	RetryTimeoutS int // initial backoff seed in seconds (implementation-specific growth)
}
