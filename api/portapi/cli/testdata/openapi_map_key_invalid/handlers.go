package openapi_map_key_invalid

import (
	"context"

	"github.com/shipq/shipq/api/portapi"
)

// IntKeyMap has a map with int keys (not supported in JSON).
type IntKeyMap struct {
	Data map[int]string `json:"data"`
}

// Response uses a map with non-string keys.
type Response struct {
	// Counts is a map with int keys, which is not JSON-compatible.
	Counts map[int]int `json:"counts"`
	// Labels is a valid map with string keys.
	Labels map[string]string `json:"labels,omitempty"`
}

// Request is a simple request type.
type Request struct {
	ID string `path:"id"`
}

// Register registers all endpoints with the app.
func Register(app *portapi.App) {
	app.Get("/things/{id}", GetThing)
}

// GetThing returns a response with non-string map keys.
func GetThing(ctx context.Context, req Request) (Response, error) {
	return Response{}, nil
}
