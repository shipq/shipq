package apiroot_mw_short_circuit_error

import (
	"context"
	"sync/atomic"

	"github.com/shipq/shipq/api/portapi"
	"github.com/shipq/shipq/api/portapi/cli/testdata/apiroot_mw_short_circuit_error/middleware"
)

// handlerCounter tracks how many times handlers were called
var handlerCounter int64

// GetHandlerCount returns the number of times handlers were called.
func GetHandlerCount() int64 {
	return atomic.LoadInt64(&handlerCounter)
}

// ResetHandlerCount resets the handler counter.
func ResetHandlerCount() {
	atomic.StoreInt64(&handlerCounter, 0)
}

// Secret represents a protected resource.
type Secret struct {
	Value string `json:"value"`
}

// Register registers all endpoints with the app.
func Register(app *portapi.App) {
	app.Group(func(g *portapi.Group) {
		// TrackAfter runs first (outermost)
		g.Use(middleware.TrackAfter)
		// RequireAuth runs second (will short-circuit)
		g.Use(middleware.RequireAuth)

		g.Get("/secrets", GetSecrets)
	})
}

// GetSecrets handles GET /secrets - should not run if auth fails.
func GetSecrets(ctx context.Context) ([]Secret, error) {
	atomic.AddInt64(&handlerCounter, 1)
	return []Secret{{Value: "top-secret"}}, nil
}
