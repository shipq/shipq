package middleware

import (
	"context"
	"fmt"
	"sync"

	"github.com/shipq/shipq/api/portapi"
)

// ObservationResult tracks what the observer middleware saw.
type ObservationResult struct {
	DecodedPresent bool
	DecodedType    string
	Method         string
	Pattern        string
}

// observer holds the last observation
var observer struct {
	mu     sync.Mutex
	result *ObservationResult
}

// GetObservation returns the last observation result.
func GetObservation() *ObservationResult {
	observer.mu.Lock()
	defer observer.mu.Unlock()
	if observer.result == nil {
		return nil
	}
	// Return a copy
	r := *observer.result
	return &r
}

// ResetObservation clears the observation.
func ResetObservation() {
	observer.mu.Lock()
	defer observer.mu.Unlock()
	observer.result = nil
}

// RegisterMiddleware registers all middleware functions.
func RegisterMiddleware(reg *portapi.MiddlewareRegistry) {
	reg.Use(Observer)
}

// Observer is a middleware that calls next and then observes the decoded request.
func Observer(ctx context.Context, req *portapi.Request, next portapi.Next) (portapi.HandlerResult, error) {
	// Call next first (this will perform binding)
	result, err := next(ctx)

	// After next returns, try to observe the decoded request
	decoded, present := req.DecodedReqValue()

	obs := &ObservationResult{
		DecodedPresent: present,
		Method:         req.Method,
		Pattern:        req.Pattern,
	}

	if present && decoded != nil {
		obs.DecodedType = fmt.Sprintf("%T", decoded)
	}

	observer.mu.Lock()
	observer.result = obs
	observer.mu.Unlock()

	return result, err
}
