package query

import "sync"

// QueryReturnType specifies how a query returns results.
type QueryReturnType string

const (
	// ReturnOne indicates a query returns 0 or 1 row (*Result or nil).
	ReturnOne QueryReturnType = "one"
	// ReturnMany indicates a query returns 0 to N rows ([]Result).
	ReturnMany QueryReturnType = "many"
	// ReturnExec indicates a query executes without returning rows (sql.Result).
	ReturnExec QueryReturnType = "exec"
)

// RegisteredQuery holds a query AST and its return type.
type RegisteredQuery struct {
	AST        *AST
	ReturnType QueryReturnType
}

// registry stores all queries registered via MustDefineOne/MustDefineMany/MustDefineExec.
// Uses a sync.Map for thread-safety during init() execution across packages.
var registry sync.Map

// mustDefineQuery is the internal registration function that panics on errors.
// This is intentional - query registration happens at init() time, and errors
// should cause immediate, obvious failures rather than silent runtime issues.
func mustDefineQuery(name string, ast *AST, returnType QueryReturnType) *AST {
	if name == "" {
		panic("query name cannot be empty")
	}
	if ast == nil {
		panic("query AST cannot be nil")
	}
	rq := RegisteredQuery{
		AST:        ast,
		ReturnType: returnType,
	}
	if _, loaded := registry.LoadOrStore(name, rq); loaded {
		panic("duplicate query name: " + name)
	}
	return ast
}

// MustDefineOne registers a query that returns 0 or 1 row.
// Use this for queries like GetUserById, GetOrderByPublicId, etc.
//
// MustDefineOne panics if:
//   - name is empty
//   - ast is nil
//   - a query with the same name is already registered
//
// This follows Go's convention (like regexp.MustCompile) where "Must" functions
// panic on error. These functions are designed for use in init() where panics
// cause clear, immediate failures.
//
//	func init() {
//	    query.MustDefineOne("GetUserByEmail",
//	        query.From(schema.Users).
//	            Select(schema.Users.Id(), schema.Users.Email()).
//	            Where(schema.Users.Email().Eq(query.Param[string]("email"))).
//	            Build(),
//	    )
//	}
//
// The generated method will return (*Result, error) where Result is nil if no row found.
func MustDefineOne(name string, ast *AST) *AST {
	return mustDefineQuery(name, ast, ReturnOne)
}

// MustDefineMany registers a query that returns 0 to N rows.
// Use this for queries like ListUsers, FindPetsByStatus, etc.
//
// MustDefineMany panics if:
//   - name is empty
//   - ast is nil
//   - a query with the same name is already registered
//
// This follows Go's convention (like regexp.MustCompile) where "Must" functions
// panic on error. These functions are designed for use in init() where panics
// cause clear, immediate failures.
//
//	func init() {
//	    query.MustDefineMany("FindPetsByStatus",
//	        query.From(schema.Pets).
//	            Select(schema.Pets.Id(), schema.Pets.Name()).
//	            Where(schema.Pets.Status().Eq(query.Param[string]("status"))).
//	            Build(),
//	    )
//	}
//
// The generated method will return ([]Result, error).
func MustDefineMany(name string, ast *AST) *AST {
	return mustDefineQuery(name, ast, ReturnMany)
}

// MustDefineExec registers a query that executes without returning rows.
// Use this for INSERT, UPDATE, DELETE queries that don't use RETURNING.
//
// MustDefineExec panics if:
//   - name is empty
//   - ast is nil
//   - a query with the same name is already registered
//
// This follows Go's convention (like regexp.MustCompile) where "Must" functions
// panic on error. These functions are designed for use in init() where panics
// cause clear, immediate failures.
//
//	func init() {
//	    query.MustDefineExec("UpdateUserEmail",
//	        query.Update(schema.Users).
//	            Set(schema.Users.Email(), query.Param[string]("email")).
//	            Where(schema.Users.Id().Eq(query.Param[int64]("id"))).
//	            Build(),
//	    )
//	}
//
// The generated method will return (sql.Result, error).
func MustDefineExec(name string, ast *AST) *AST {
	return mustDefineQuery(name, ast, ReturnExec)
}

// DefineOne is an alias for MustDefineOne for backward compatibility.
// Deprecated: Use MustDefineOne instead for clarity about panic behavior.
func DefineOne(name string, ast *AST) *AST {
	return MustDefineOne(name, ast)
}

// DefineMany is an alias for MustDefineMany for backward compatibility.
// Deprecated: Use MustDefineMany instead for clarity about panic behavior.
func DefineMany(name string, ast *AST) *AST {
	return MustDefineMany(name, ast)
}

// DefineExec is an alias for MustDefineExec for backward compatibility.
// Deprecated: Use MustDefineExec instead for clarity about panic behavior.
func DefineExec(name string, ast *AST) *AST {
	return MustDefineExec(name, ast)
}

// DefineQuery is an alias for MustDefineMany for backward compatibility.
// Deprecated: Use MustDefineOne, MustDefineMany, or MustDefineExec instead.
func DefineQuery(name string, ast *AST) *AST {
	return MustDefineMany(name, ast)
}

// GetRegisteredQueries returns a copy of all registered queries.
// The returned map is safe to modify without affecting the registry.
func GetRegisteredQueries() map[string]RegisteredQuery {
	result := make(map[string]RegisteredQuery)
	registry.Range(func(key, value any) bool {
		result[key.(string)] = value.(RegisteredQuery)
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
