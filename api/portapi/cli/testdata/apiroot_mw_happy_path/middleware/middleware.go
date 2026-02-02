package middleware

import (
	"context"
	"sync"

	"github.com/shipq/shipq/api/portapi"
)

// ExecutionTracker tracks middleware and handler execution order.
type ExecutionTracker struct {
	mu    sync.Mutex
	order []string
}

// NewExecutionTracker creates a new execution tracker.
func NewExecutionTracker() *ExecutionTracker {
	return &ExecutionTracker{
		order: make([]string, 0),
	}
}

// Record adds a name to the execution order.
func (t *ExecutionTracker) Record(name string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.order = append(t.order, name)
}

// Order returns the recorded execution order.
func (t *ExecutionTracker) Order() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	result := make([]string, len(t.order))
	copy(result, t.order)
	return result
}

// Reset clears the execution order.
func (t *ExecutionTracker) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.order = t.order[:0]
}

// Global tracker for tests to inspect
var tracker = NewExecutionTracker()

// GetTracker returns the global execution tracker.
func GetTracker() *ExecutionTracker {
	return tracker
}

// RegisterMiddleware registers all middleware functions.
func RegisterMiddleware(reg *portapi.MiddlewareRegistry) {
	reg.Use(GlobalA)
	reg.Use(GlobalB)
	reg.Use(GroupA)
	reg.Use(GroupB)
}

// GlobalA is a global middleware that records execution.
func GlobalA(ctx context.Context, req *portapi.Request, next portapi.Next) (portapi.HandlerResult, error) {
	tracker.Record("GlobalA")
	return next(ctx)
}

// GlobalB is a global middleware that records execution.
func GlobalB(ctx context.Context, req *portapi.Request, next portapi.Next) (portapi.HandlerResult, error) {
	tracker.Record("GlobalB")
	return next(ctx)
}

// GroupA is a group-level middleware that records execution.
func GroupA(ctx context.Context, req *portapi.Request, next portapi.Next) (portapi.HandlerResult, error) {
	tracker.Record("GroupA")
	return next(ctx)
}

// GroupB is a nested group-level middleware that records execution.
func GroupB(ctx context.Context, req *portapi.Request, next portapi.Next) (portapi.HandlerResult, error) {
	tracker.Record("GroupB")
	return next(ctx)
}
