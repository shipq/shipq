package llm

import "context"

// clientMapKey is the context key for the map of named clients.
type clientMapKey struct{}

// WithClient injects an LLM client as the default client in the context.
// Called by the user's Setup function before the channel handler runs.
func WithClient(ctx context.Context, c *Client) context.Context {
	m := getClientMap(ctx)
	m[""] = c
	return context.WithValue(ctx, clientMapKey{}, m)
}

// WithNamedClient injects an LLM client under a name (e.g., "summary").
// This allows a single channel handler to use multiple providers or models.
// Panics if name is empty — use WithClient for the default client.
func WithNamedClient(ctx context.Context, name string, c *Client) context.Context {
	if name == "" {
		panic("llm.WithNamedClient: name must not be empty — use WithClient for the default client")
	}
	c.name = name
	m := getClientMap(ctx)
	m[name] = c
	return context.WithValue(ctx, clientMapKey{}, m)
}

// ClientFromContext retrieves the default LLM client from the context.
// Panics if no default client is present — this indicates the handler is not
// running inside an LLM-enabled channel, or Setup was not called correctly.
func ClientFromContext(ctx context.Context) *Client {
	return NamedClientFromContext(ctx, "")
}

// NamedClientFromContext retrieves a named LLM client from the context.
// Use this when your Setup registered multiple clients for different
// providers or models.
// Panics if the named client is not present.
func NamedClientFromContext(ctx context.Context, name string) *Client {
	m := getClientMap(ctx)
	c, ok := m[name]
	if !ok || c == nil {
		if name == "" {
			panic("llm.ClientFromContext: no default Client in context — did your Setup call llm.WithClient?")
		}
		panic("llm.NamedClientFromContext: no Client named " + name + " in context — did your Setup call llm.WithNamedClient?")
	}
	return c
}

// getClientMap returns a fresh copy of the client map stored in ctx, or a new
// empty map if none is present. Copy-on-read ensures that storing a new value
// into the returned map never mutates a parent context's map.
func getClientMap(ctx context.Context) map[string]*Client {
	if m, ok := ctx.Value(clientMapKey{}).(map[string]*Client); ok {
		cp := make(map[string]*Client, len(m)+1)
		for k, v := range m {
			cp[k] = v
		}
		return cp
	}
	return make(map[string]*Client, 2)
}
