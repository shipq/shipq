package query

import "sync"

// registry stores all queries registered via DefineQuery.
// Uses a sync.Map for thread-safety during init() execution across packages.
var registry sync.Map

// DefineQuery registers a query with a name and returns it.
// This is intended to be used at package initialization time:
//
//	func init() {
//	    query.DefineQuery("GetUserByEmail",
//	        query.From(schema.Users).
//	            Select(schema.Users.Id(), schema.Users.Email()).
//	            Where(schema.Users.Email().Eq(query.Param[string]("email"))).
//	            Build(),
//	    )
//	}
//
// The CLI can then call GetRegisteredQueries() to retrieve all registered queries.
func DefineQuery(name string, ast *AST) *AST {
	if name == "" {
		panic("query name cannot be empty")
	}
	if ast == nil {
		panic("query AST cannot be nil")
	}
	if _, loaded := registry.LoadOrStore(name, ast); loaded {
		panic("duplicate query name: " + name)
	}
	return ast
}

// GetRegisteredQueries returns a copy of all registered queries.
// The returned map is safe to modify without affecting the registry.
func GetRegisteredQueries() map[string]*AST {
	result := make(map[string]*AST)
	registry.Range(func(key, value any) bool {
		result[key.(string)] = value.(*AST)
		return true
	})
	return result
}

// ClearRegistry removes all registered queries.
// This is primarily useful for testing.
func ClearRegistry() {
	registry.Range(func(key, _ any) bool {
		registry.Delete(key)
		return true
	})
}

// QueryCount returns the number of registered queries.
// This is primarily useful for testing.
func QueryCount() int {
	count := 0
	registry.Range(func(_, _ any) bool {
		count++
		return true
	})
	return count
}
