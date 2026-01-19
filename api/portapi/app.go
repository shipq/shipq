package portapi

// App records endpoint registrations during discovery.
// It is not used at runtime—only during build-time codegen.
type App struct {
	endpoints []Endpoint
}

// Group represents a scoped collection of endpoints with shared middleware.
type Group struct {
	app         *App
	middlewares []MiddlewareRef
}

// Group creates a new middleware group and executes the provided function.
func (a *App) Group(fn func(g *Group)) {
	g := &Group{
		app:         a,
		middlewares: nil,
	}
	fn(g)
}

// Use adds middleware to this group. Middleware is applied in the order of Use calls.
func (g *Group) Use(mw any) {
	ref := newMiddlewareRef(mw)
	g.middlewares = append(g.middlewares, ref)
}

// Group creates a nested group that inherits the parent's middleware.
func (g *Group) Group(fn func(g2 *Group)) {
	// Copy parent middleware to avoid slice aliasing
	childMws := make([]MiddlewareRef, len(g.middlewares))
	copy(childMws, g.middlewares)

	g2 := &Group{
		app:         g.app,
		middlewares: childMws,
	}
	fn(g2)
}

// Get registers a GET endpoint within this group.
func (g *Group) Get(path string, handler any) {
	g.register("GET", path, handler)
}

// Post registers a POST endpoint within this group.
func (g *Group) Post(path string, handler any) {
	g.register("POST", path, handler)
}

// Put registers a PUT endpoint within this group.
func (g *Group) Put(path string, handler any) {
	g.register("PUT", path, handler)
}

// Delete registers a DELETE endpoint within this group.
func (g *Group) Delete(path string, handler any) {
	g.register("DELETE", path, handler)
}

func (g *Group) register(method, path string, handler any) {
	if handler == nil {
		panic("handler cannot be nil")
	}
	ep, err := NewEndpoint(method, path, handler)
	if err != nil {
		panic(err)
	}

	// Copy middlewares to avoid sharing slices between endpoints
	ep.Middlewares = make([]MiddlewareRef, len(g.middlewares))
	copy(ep.Middlewares, g.middlewares)

	g.app.endpoints = append(g.app.endpoints, ep)
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
