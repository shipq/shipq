package openapi_invalid_bindings

import (
	"context"

	"github.com/shipq/shipq/api/portapi"
)

// BadRequest has a field with multiple binding sources (invalid).
type BadRequest struct {
	// ID has both path and json tags, which is not allowed.
	ID string `path:"id" json:"id"`
}

// GoodResponse is a valid response type.
type GoodResponse struct {
	Message string `json:"message"`
}

// Register registers all endpoints with the app.
func Register(app *portapi.App) {
	app.Get("/things/{id}", GetThing)
}

// GetThing uses a request with conflicting bindings.
func GetThing(ctx context.Context, req BadRequest) (GoodResponse, error) {
	return GoodResponse{}, nil
}
