package portapi

// App records endpoint registrations during discovery.
// It is not used at runtime—only during build-time codegen.
type App struct {
	endpoints []Endpoint
}

// Get registers a GET endpoint.
func (a *App) Get(path string, handler any) { a.register("GET", path, handler) }

// Post registers a POST endpoint.
func (a *App) Post(path string, handler any) { a.register("POST", path, handler) }

// Put registers a PUT endpoint.
func (a *App) Put(path string, handler any) { a.register("PUT", path, handler) }

// Delete registers a DELETE endpoint.
func (a *App) Delete(path string, handler any) { a.register("DELETE", path, handler) }

func (a *App) register(method, path string, handler any) {
	if handler == nil {
		panic("handler cannot be nil")
	}
	ep, err := NewEndpoint(method, path, handler)
	if err != nil {
		panic(err)
	}
	a.endpoints = append(a.endpoints, ep)
}

// Endpoints returns a copy of all registered endpoints.
func (a *App) Endpoints() []Endpoint {
	return append([]Endpoint(nil), a.endpoints...)
}
