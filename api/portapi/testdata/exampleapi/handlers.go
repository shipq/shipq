package exampleapi

import "context"

// Shape 1: ctx + req → resp + err
type CreatePetReq struct {
	Name string `json:"name"`
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
	ID   string `json:"id"`
	Name string `json:"name"`
}

func GetPet(ctx context.Context, req GetPetReq) (GetPetResp, error) {
	return GetPetResp{ID: req.ID, Name: "fluffy"}, nil
}
