package apiroot_mw_post_bind_observer

import (
	"context"

	"github.com/shipq/shipq/api/portapi"
	"github.com/shipq/shipq/cmd/portsql-api-httpgen/testdata/apiroot_mw_post_bind_observer/middleware"
)

// UpdateRequest is a request with multiple binding sources.
type UpdateRequest struct {
	ID     string `path:"id"`
	Action string `query:"action"`
	Name   string `json:"name"`
}

// UpdateResponse is the response from update.
type UpdateResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// Register registers all endpoints with the app.
func Register(app *portapi.App) {
	app.Group(func(g *portapi.Group) {
		g.Use(middleware.Observer)
		g.Post("/items/{id}", UpdateItem)
	})
}

// UpdateItem handles POST /items/{id} - tests post-bind observation with multiple binding sources.
func UpdateItem(ctx context.Context, req UpdateRequest) (UpdateResponse, error) {
	return UpdateResponse{
		Success: true,
		Message: "Item " + req.ID + " updated with name " + req.Name,
	}, nil
}
