package api

import (
	"context"
	"fmt"

	"github.com/shipq/shipq/api/portapi"
	"github.com/shipq/shipq/api/portapi/demo/middleware"
)

// Pet represents a pet in the store
type Pet struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Species string `json:"species"`
	Age     int    `json:"age"`
	OwnerID string `json:"owner_id,omitempty"`
}

// CreatePetRequest is the request to create a new pet
type CreatePetRequest struct {
	Name    string `json:"name"`
	Species string `json:"species"`
	Age     int    `json:"age"`
}

// GetPetRequest retrieves a pet by ID
type GetPetRequest struct {
	ID string `path:"id"`
}

// UpdatePetRequest updates a pet
type UpdatePetRequest struct {
	ID      string `path:"id"`
	Name    string `json:"name"`
	Species string `json:"species"`
	Age     int    `json:"age"`
}

// DeletePetRequest deletes a pet
type DeletePetRequest struct {
	ID string `path:"id"`
}

// ListPetsRequest lists pets with optional filtering
type ListPetsRequest struct {
	Species string `query:"species"`
	Limit   int    `query:"limit"`
}

// UserProfile represents the current user's profile
type UserProfile struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	PetCount int    `json:"pet_count"`
}

// In-memory storage for demo purposes
var (
	pets = map[string]*Pet{
		"1": {ID: "1", Name: "Fluffy", Species: "cat", Age: 3, OwnerID: "alice"},
		"2": {ID: "2", Name: "Max", Species: "dog", Age: 5, OwnerID: "bob"},
		"3": {ID: "3", Name: "Charlie", Species: "dog", Age: 2, OwnerID: "alice"},
	}
	nextID = 4
)

// Register configures all API endpoints and middleware
func Register(app *portapi.App) {
	// Public endpoints (no auth required)
	app.Group(func(g *portapi.Group) {
		g.Use(middleware.RequestLogger)
		g.Use(middleware.AuthOptional)

		g.Get("/health", HealthCheck)
		g.Get("/pets", ListPets)
		g.Get("/pets/{id}", GetPet)
	})

	// Protected endpoints (auth required)
	app.Group(func(g *portapi.Group) {
		g.Use(middleware.RequestLogger)
		g.Use(middleware.AuthRequired)
		g.Use(middleware.RateLimiter)

		g.Post("/pets", CreatePet)
		g.Put("/pets/{id}", UpdatePet)
		g.Delete("/pets/{id}", DeletePet)
		g.Get("/profile", GetProfile)
	})
}

// HealthCheck returns service health status
func HealthCheck(ctx context.Context) error {
	return nil
}

// ListPets returns all pets, optionally filtered by species
func ListPets(ctx context.Context, req ListPetsRequest) ([]Pet, error) {
	var result []Pet

	for _, pet := range pets {
		if req.Species != "" && pet.Species != req.Species {
			continue
		}
		result = append(result, *pet)
	}

	// Apply limit if specified
	if req.Limit > 0 && len(result) > req.Limit {
		result = result[:req.Limit]
	}

	return result, nil
}

// GetPet retrieves a single pet by ID
func GetPet(ctx context.Context, req GetPetRequest) (Pet, error) {
	pet, ok := pets[req.ID]
	if !ok {
		return Pet{}, portapi.HTTPError{
			Status: 404,
			Code:   "pet_not_found",
			Msg:    fmt.Sprintf("Pet with ID %s not found", req.ID),
		}
	}

	return *pet, nil
}

// CreatePet creates a new pet (requires authentication)
func CreatePet(ctx context.Context, req CreatePetRequest) (Pet, error) {
	user := middleware.MustCurrentUser(ctx)

	id := fmt.Sprintf("%d", nextID)
	nextID++

	pet := &Pet{
		ID:      id,
		Name:    req.Name,
		Species: req.Species,
		Age:     req.Age,
		OwnerID: user.Username,
	}

	pets[id] = pet

	return *pet, nil
}

// UpdatePet updates an existing pet (requires authentication)
func UpdatePet(ctx context.Context, req UpdatePetRequest) (Pet, error) {
	user := middleware.MustCurrentUser(ctx)

	pet, ok := pets[req.ID]
	if !ok {
		return Pet{}, portapi.HTTPError{
			Status: 404,
			Code:   "pet_not_found",
			Msg:    fmt.Sprintf("Pet with ID %s not found", req.ID),
		}
	}

	if pet.OwnerID != user.Username {
		return Pet{}, portapi.HTTPError{
			Status: 403,
			Code:   "forbidden",
			Msg:    "You can only update your own pets",
		}
	}

	pet.Name = req.Name
	pet.Species = req.Species
	pet.Age = req.Age

	return *pet, nil
}

// DeletePet deletes a pet (requires authentication)
func DeletePet(ctx context.Context, req DeletePetRequest) error {
	user := middleware.MustCurrentUser(ctx)

	pet, ok := pets[req.ID]
	if !ok {
		return portapi.HTTPError{
			Status: 404,
			Code:   "pet_not_found",
			Msg:    fmt.Sprintf("Pet with ID %s not found", req.ID),
		}
	}

	if pet.OwnerID != user.Username {
		return portapi.HTTPError{
			Status: 403,
			Code:   "forbidden",
			Msg:    "You can only delete your own pets",
		}
	}

	delete(pets, req.ID)

	return nil
}

// GetProfile returns the current user's profile
func GetProfile(ctx context.Context) (UserProfile, error) {
	user := middleware.MustCurrentUser(ctx)

	petCount := 0
	for _, pet := range pets {
		if pet.OwnerID == user.Username {
			petCount++
		}
	}

	return UserProfile{
		Username: user.Username,
		Email:    user.Email,
		PetCount: petCount,
	}, nil
}
