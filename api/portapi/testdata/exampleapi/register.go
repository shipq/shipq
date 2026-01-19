package exampleapi

import "github.com/shipq/shipq/api/portapi"

func Register(app *portapi.App) {
	app.Post("/pets", CreatePet)
	app.Delete("/pets/{id}", DeletePet)
	app.Get("/pets", ListPets)
	app.Get("/health", HealthCheck)
	app.Get("/pets/{id}", GetPet)
}
