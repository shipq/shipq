package apiroot_mw_short_circuit_result

import (
	"context"
	"sync/atomic"

	"github.com/shipq/shipq/api/portapi"
	"github.com/shipq/shipq/cmd/portsql-api-httpgen/testdata/apiroot_mw_short_circuit_result/middleware"
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

// Data represents some response data.
type Data struct {
	Value string `json:"value"`
}

// Register registers all endpoints with the app.
func Register(app *portapi.App) {
	app.Group(func(g *portapi.Group) {
		g.Use(middleware.CacheCheck)
		g.Get("/data", GetData)
	})
}

// GetData handles GET /data - should not run if cache hits.
func GetData(ctx context.Context) (Data, error) {
	atomic.AddInt64(&handlerCounter, 1)
	return Data{Value: "fresh-data"}, nil
}
