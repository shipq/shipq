package apiroot_mw_happy_path

import (
	"context"

	"github.com/shipq/shipq/api/portapi"
	"github.com/shipq/shipq/cmd/portsql-api-httpgen/testdata/apiroot_mw_happy_path/middleware"
)

// Pet represents a pet resource.
type Pet struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// GetPetRequest is a request with path binding.
type GetPetRequest struct {
	ID string `path:"id"`
}

// CreatePetRequest is a request with JSON binding.
type CreatePetRequest struct {
	Name string `json:"name"`
}

// Register registers all endpoints with the app.
func Register(app *portapi.App) {
	// Create a group with global middleware
	app.Group(func(g *portapi.Group) {
		g.Use(middleware.GlobalA)
		g.Use(middleware.GlobalB)

		// Nested group with GroupA middleware
		g.Group(func(outer *portapi.Group) {
			outer.Use(middleware.GroupA)

			// Nested group with GroupB middleware
			outer.Group(func(inner *portapi.Group) {
				inner.Use(middleware.GroupB)

				// Register endpoints in nested group
				inner.Get("/pets/{id}", GetPet)
				inner.Post("/pets", CreatePet)

				// Also register a simple endpoint to test context-only shape
				inner.Get("/health", Health)
			})
		})
	})
}

// GetPet handles GET /pets/{id} - tests path binding
func GetPet(ctx context.Context, req GetPetRequest) (Pet, error) {
	middleware.GetTracker().Record("GetPet")
	return Pet{ID: req.ID, Name: "Fluffy"}, nil
}

// CreatePet handles POST /pets - tests JSON binding
func CreatePet(ctx context.Context, req CreatePetRequest) (Pet, error) {
	middleware.GetTracker().Record("CreatePet")
	return Pet{ID: "123", Name: req.Name}, nil
}

// Health handles GET /health - tests context-only shape
func Health(ctx context.Context) error {
	middleware.GetTracker().Record("Health")
	return nil
}
