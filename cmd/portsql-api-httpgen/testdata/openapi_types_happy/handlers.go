package openapi_types_happy

import (
	"context"
	"time"

	"github.com/shipq/shipq/api/portapi"
)

// Address represents a physical address.
type Address struct {
	Street  string `json:"street"`
	City    string `json:"city"`
	ZipCode string `json:"zip_code,omitempty"`
}

// Tag represents a tag on a thing.
type Tag struct {
	Name  string `json:"name"`
	Value string `json:"value,omitempty"`
}

// GetThingReq is a request to get a thing by ID.
type GetThingReq struct {
	// ID is the unique identifier of the thing.
	ID string `path:"id"`
	// IncludeDetails controls whether to include extra details.
	IncludeDetails *bool `query:"include_details"`
}

// GetThingResp is the response containing a thing.
type GetThingResp struct {
	// ID is the unique identifier.
	ID string `json:"id"`
	// Name is the display name.
	Name string `json:"name"`
	// Description is an optional description.
	Description *string `json:"description,omitempty"`
	// Count is a required integer field.
	Count int `json:"count"`
	// Score is an optional float field.
	Score *float64 `json:"score,omitempty"`
	// Active indicates if the thing is active.
	Active bool `json:"active"`
	// CreatedAt is when the thing was created.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is when the thing was last updated (optional).
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
	// Tags are labels attached to the thing.
	Tags []Tag `json:"tags,omitempty"`
	// Metadata is arbitrary key-value data.
	Metadata map[string]string `json:"metadata,omitempty"`
	// Address is the thing's location.
	Address Address `json:"address"`
	// SecondaryAddress is an optional secondary location.
	SecondaryAddress *Address `json:"secondary_address,omitempty"`
}

// ListThingsReq is a request to list things.
type ListThingsReq struct {
	Limit  *int    `query:"limit"`
	Offset *int    `query:"offset"`
	Search *string `query:"search"`
}

// ListThingsResp is the response containing a list of things.
type ListThingsResp struct {
	Items      []GetThingResp `json:"items"`
	TotalCount int            `json:"total_count"`
}

// Register registers all endpoints with the app.
func Register(app *portapi.App) {
	app.Get("/things/{id}", GetThing)
	app.Get("/things", ListThings)
}

// GetThing retrieves a single thing by ID.
func GetThing(ctx context.Context, req GetThingReq) (GetThingResp, error) {
	return GetThingResp{}, nil
}

// ListThings retrieves a paginated list of things.
func ListThings(ctx context.Context, req ListThingsReq) (ListThingsResp, error) {
	return ListThingsResp{}, nil
}
