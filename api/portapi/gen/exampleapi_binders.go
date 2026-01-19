package gen

import (
	"encoding/json"
	"net/http"

	"github.com/shipq/shipq/api/portapi/testdata/exampleapi"
)

// bindCreatePet binds the request for POST /pets
func bindCreatePet(r *http.Request) (exampleapi.CreatePetReq, error) {
	var req exampleapi.CreatePetReq

	// JSON body binding
	if r.Body == nil {
		return req, &BindError{Source: "body", Err: errMissing}
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, &BindError{Source: "body", Err: err}
	}

	return req, nil
}

// bindDeletePet binds the request for DELETE /pets/{id}
func bindDeletePet(r *http.Request) (exampleapi.DeletePetReq, error) {
	var req exampleapi.DeletePetReq

	// path: id (required string)
	v := r.PathValue("id")
	if v == "" {
		return req, &BindError{Source: "path", Field: "id", Err: errMissing}
	}
	req.ID = v

	return req, nil
}

// bindListPets binds the request for GET /pets (no binding needed)
func bindListPets(r *http.Request) error {
	// No request type - nothing to bind
	return nil
}

// bindHealthCheck binds the request for GET /health (no binding needed)
func bindHealthCheck(r *http.Request) error {
	// No request type - nothing to bind
	return nil
}

// bindGetPet binds the request for GET /pets/{id}
func bindGetPet(r *http.Request) (exampleapi.GetPetReq, error) {
	var req exampleapi.GetPetReq

	// path: id (required string)
	v := r.PathValue("id")
	if v == "" {
		return req, &BindError{Source: "path", Field: "id", Err: errMissing}
	}
	req.ID = v

	// query: verbose (*bool optional)
	q := r.URL.Query()
	if q.Has("verbose") {
		s := q.Get("verbose")
		b, err := parseBool(s)
		if err != nil {
			return req, &BindError{Source: "query", Field: "verbose", Err: err}
		}
		req.Verbose = &b
	}

	// header: Authorization (required string)
	h := r.Header.Get("Authorization")
	if h == "" {
		return req, &BindError{Source: "header", Field: "Authorization", Err: errMissing}
	}
	req.Auth = h

	return req, nil
}

// bindSearchPets binds the request for GET /pets/search
func bindSearchPets(r *http.Request) (exampleapi.SearchPetsReq, error) {
	var req exampleapi.SearchPetsReq

	q := r.URL.Query()

	// query: limit (required int)
	if !q.Has("limit") {
		return req, &BindError{Source: "query", Field: "limit", Err: errMissing}
	}
	limitStr := q.Get("limit")
	limit, err := parseInt(limitStr)
	if err != nil {
		return req, &BindError{Source: "query", Field: "limit", Err: err}
	}
	req.Limit = limit

	// query: tag ([]string multi-value, optional)
	if vs, ok := q["tag"]; ok && len(vs) > 0 {
		req.Tags = vs
	}

	// query: cursor (*string optional)
	if q.Has("cursor") {
		s := q.Get("cursor")
		req.Cursor = &s
	}

	// header: Authorization (*string optional)
	if h := r.Header.Get("Authorization"); h != "" {
		req.Auth = &h
	}

	return req, nil
}

// bindUpdatePet binds the request for PUT /pets/{id}
func bindUpdatePet(r *http.Request) (exampleapi.UpdatePetReq, error) {
	var req exampleapi.UpdatePetReq

	// path: id (required string)
	v := r.PathValue("id")
	if v == "" {
		return req, &BindError{Source: "path", Field: "id", Err: errMissing}
	}
	req.ID = v

	// JSON body binding
	if r.Body == nil {
		return req, &BindError{Source: "body", Err: errMissing}
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, &BindError{Source: "body", Err: err}
	}

	return req, nil
}
