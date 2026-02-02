// Package openapi_docs_happy provides test handlers with comprehensive doc comments
// for testing docstring extraction in OpenAPI generation.
package openapi_docs_happy

import (
	"context"

	"github.com/shipq/shipq/api/portapi"
)

// Pet represents a pet in the system.
// Pets can be dogs, cats, or other animals.
//
// Example:
//
//	pet := Pet{ID: "123", Name: "Fluffy", Species: "cat"}
type Pet struct {
	// ID is the unique identifier for the pet.
	ID string `json:"id"`
	// Name is the pet's display name.
	// This is what the owner calls the pet.
	Name string `json:"name"`
	// Species indicates what kind of animal the pet is.
	Species string `json:"species,omitempty"`
	// Age is the pet's age in years.
	Age *int `json:"age,omitempty"`
}

// Owner represents a pet owner.
type Owner struct {
	// ID is the unique identifier for the owner.
	ID string `json:"id"`
	// Name is the owner's full name.
	Name string `json:"name"`
	// Email is the owner's contact email.
	Email string `json:"email,omitempty"`
}

// GetPetReq is a request to retrieve a pet by ID.
type GetPetReq struct {
	// ID is the pet's unique identifier.
	ID string `path:"id"`
}

// GetPetResp is the response containing pet details.
type GetPetResp struct {
	// Pet is the retrieved pet.
	Pet Pet `json:"pet"`
	// Owner is the pet's owner, if known.
	Owner *Owner `json:"owner,omitempty"`
}

// CreatePetReq is a request to create a new pet.
type CreatePetReq struct {
	// Name is the name for the new pet.
	Name string `json:"name"`
	// Species is what kind of animal the pet is.
	Species string `json:"species"`
	// OwnerID is the ID of the pet's owner.
	OwnerID *string `json:"owner_id,omitempty"`
}

// Register registers all endpoints with the app.
func Register(app *portapi.App) {
	app.Get("/pets/{id}", GetPet)
	app.Post("/pets", CreatePet)
	app.Get("/health", HealthCheck)
}

// GetPet retrieves a pet by its unique identifier.
// This endpoint returns the full pet details including owner information
// if available.
//
// Returns 404 if the pet is not found.
func GetPet(ctx context.Context, req GetPetReq) (GetPetResp, error) {
	return GetPetResp{}, nil
}

// CreatePet creates a new pet in the system.
// The pet will be assigned a unique ID automatically.
//
// If an owner_id is provided, the pet will be associated with that owner.
// Otherwise, the pet will be created without an owner.
func CreatePet(ctx context.Context, req CreatePetReq) (Pet, error) {
	return Pet{}, nil
}

// HealthCheck returns the health status of the service.
// This endpoint is used for liveness and readiness probes.
func HealthCheck(ctx context.Context) error {
	return nil
}
