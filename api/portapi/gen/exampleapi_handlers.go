package gen

import (
	"context"
	"net/http"

	"github.com/shipq/shipq/api/portapi/testdata/exampleapi"
)

// HandleCreatePet returns an http.Handler for POST /pets
func HandleCreatePet(handler func(context.Context, exampleapi.CreatePetReq) (exampleapi.CreatePetResp, error)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req, err := bindCreatePet(r)
		if err != nil {
			writeError(w, err)
			return
		}

		resp, err := handler(r.Context(), req)
		if err != nil {
			writeError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, resp)
	})
}

// HandleDeletePet returns an http.Handler for DELETE /pets/{id}
func HandleDeletePet(handler func(context.Context, exampleapi.DeletePetReq) error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req, err := bindDeletePet(r)
		if err != nil {
			writeError(w, err)
			return
		}

		if err := handler(r.Context(), req); err != nil {
			writeError(w, err)
			return
		}

		writeNoContent(w)
	})
}

// HandleListPets returns an http.Handler for GET /pets
func HandleListPets(handler func(context.Context) (exampleapi.ListPetsResp, error)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp, err := handler(r.Context())
		if err != nil {
			writeError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, resp)
	})
}

// HandleHealthCheck returns an http.Handler for GET /health
func HandleHealthCheck(handler func(context.Context) error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := handler(r.Context()); err != nil {
			writeError(w, err)
			return
		}

		writeNoContent(w)
	})
}

// HandleGetPet returns an http.Handler for GET /pets/{id}
func HandleGetPet(handler func(context.Context, exampleapi.GetPetReq) (exampleapi.GetPetResp, error)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req, err := bindGetPet(r)
		if err != nil {
			writeError(w, err)
			return
		}

		resp, err := handler(r.Context(), req)
		if err != nil {
			writeError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, resp)
	})
}

// HandleSearchPets returns an http.Handler for GET /pets/search
func HandleSearchPets(handler func(context.Context, exampleapi.SearchPetsReq) (exampleapi.SearchPetsResp, error)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req, err := bindSearchPets(r)
		if err != nil {
			writeError(w, err)
			return
		}

		resp, err := handler(r.Context(), req)
		if err != nil {
			writeError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, resp)
	})
}

// HandleUpdatePet returns an http.Handler for PUT /pets/{id}
func HandleUpdatePet(handler func(context.Context, exampleapi.UpdatePetReq) (exampleapi.UpdatePetResp, error)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req, err := bindUpdatePet(r)
		if err != nil {
			writeError(w, err)
			return
		}

		resp, err := handler(r.Context(), req)
		if err != nil {
			writeError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, resp)
	})
}
