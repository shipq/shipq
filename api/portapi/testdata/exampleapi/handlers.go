package exampleapi

import "context"

// Shape 1: ctx + req → resp + err
type CreatePetReq struct {
	Name string `json:"name"`
	Age  int    `json:"age,omitempty"`
}
type CreatePetResp struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func CreatePet(ctx context.Context, req CreatePetReq) (CreatePetResp, error) {
	return CreatePetResp{ID: "pet-123", Name: req.Name}, nil
}

// Shape 2: ctx + req → err
type DeletePetReq struct {
	ID string `path:"id"`
}

func DeletePet(ctx context.Context, req DeletePetReq) error {
	return nil
}

// Shape 3: ctx → resp + err
type ListPetsResp struct {
	Pets []string `json:"pets"`
}

func ListPets(ctx context.Context) (ListPetsResp, error) {
	return ListPetsResp{Pets: []string{"fluffy", "spot"}}, nil
}

// Shape 4: ctx → err
func HealthCheck(ctx context.Context) error {
	return nil
}

// Mixed bindings: path + query + header
type GetPetReq struct {
	ID      string `path:"id"`
	Verbose *bool  `query:"verbose"`
	Auth    string `header:"Authorization"`
}
type GetPetResp struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Verbose bool   `json:"verbose,omitempty"`
}

func GetPet(ctx context.Context, req GetPetReq) (GetPetResp, error) {
	verbose := false
	if req.Verbose != nil {
		verbose = *req.Verbose
	}
	return GetPetResp{ID: req.ID, Name: "fluffy", Verbose: verbose}, nil
}

// Advanced bindings: required scalar query + slice query + optional header
type SearchPetsReq struct {
	Limit  int      `query:"limit"`          // required scalar
	Tags   []string `query:"tag"`            // slice multi-value
	Cursor *string  `query:"cursor"`         // optional pointer
	Auth   *string  `header:"Authorization"` // optional header
}
type SearchPetsResp struct {
	Pets   []string `json:"pets"`
	Limit  int      `json:"limit"`
	Tags   []string `json:"tags"`
	Cursor string   `json:"cursor,omitempty"`
	Auth   string   `json:"auth,omitempty"`
}

func SearchPets(ctx context.Context, req SearchPetsReq) (SearchPetsResp, error) {
	cursor := ""
	if req.Cursor != nil {
		cursor = *req.Cursor
	}
	auth := ""
	if req.Auth != nil {
		auth = *req.Auth
	}
	return SearchPetsResp{
		Pets:   []string{"fluffy", "spot"},
		Limit:  req.Limit,
		Tags:   req.Tags,
		Cursor: cursor,
		Auth:   auth,
	}, nil
}

// JSON body with path param
type UpdatePetReq struct {
	ID   string `path:"id"`
	Name string `json:"name"`
	Age  int    `json:"age,omitempty"`
}
type UpdatePetResp struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Age  int    `json:"age,omitempty"`
}

func UpdatePet(ctx context.Context, req UpdatePetReq) (UpdatePetResp, error) {
	return UpdatePetResp{ID: req.ID, Name: req.Name, Age: req.Age}, nil
}
