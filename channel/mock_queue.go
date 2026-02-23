package channel

import (
	"context"
	"fmt"
	"sync"
)

// MockQueue is a synchronous in-process TaskQueue for generated integration tests.
// No Redis needed -- tasks execute synchronously in SendTask.
type MockQueue struct {
	mu       sync.Mutex
	handlers map[string]func(string) error
}

// Compile-time interface check.
var _ TaskQueue = (*MockQueue)(nil)

// NewMockQueue creates a new MockQueue.
func NewMockQueue() *MockQueue {
	return &MockQueue{
		handlers: make(map[string]func(string) error),
	}
}

// RegisterTask stores the handler function for later synchronous execution.
// Implements TaskQueue.
func (mq *MockQueue) RegisterTask(name string, handler func(string) error) error {
	mq.mu.Lock()
	defer mq.mu.Unlock()
	mq.handlers[name] = handler
	return nil
}

// SendTask looks up the registered handler and calls it synchronously.
// Returns an error if no handler is registered for the given task name.
// Implements TaskQueue.
func (mq *MockQueue) SendTask(name string, payload string, opts TaskOptions) error {
	mq.mu.Lock()
	handler, ok := mq.handlers[name]
	mq.mu.Unlock()

	if !ok {
		return fmt.Errorf("MockQueue: no handler registered for task %q", name)
	}
	return handler(payload)
}

// StartWorker is a no-op for MockQueue. Tasks execute synchronously in SendTask.
// Implements TaskQueue.
func (mq *MockQueue) StartWorker(ctx context.Context, tag string, concurrency int) error {
	// Block until context is cancelled, mimicking real worker behavior.
	<-ctx.Done()
	return ctx.Err()
}

// StopWorker is a no-op for MockQueue.
// Implements TaskQueue.
func (mq *MockQueue) StopWorker() error {
	return nil
}
