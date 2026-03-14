package query

import (
	"errors"
	"sync"
)

// QueryReturnType specifies how a query returns results.
type QueryReturnType string

const (
	// ReturnOne indicates a query returns 0 or 1 row (*Result or nil).
	ReturnOne QueryReturnType = "one"
	// ReturnMany indicates a query returns 0 to N rows ([]Result).
	ReturnMany QueryReturnType = "many"
	// ReturnExec indicates a query executes without returning rows (sql.Result).
	ReturnExec QueryReturnType = "exec"
	// ReturnBulkExec indicates a bulk insert/update that executes without
	// returning rows. The generated method accepts a slice of param structs.
	ReturnBulkExec QueryReturnType = "bulk_exec"
	// ReturnPaginated indicates a query returns cursor-paginated results.
	// The runner generates cursor types, encode/decode helpers, and a method
	// that handles LIMIT+1, cursor WHERE injection, and NextCursor computation.
	ReturnPaginated QueryReturnType = "paginated"
)

// RegisteredQuery holds a query AST and its return type.
type RegisteredQuery struct {
	AST        *AST
	ReturnType QueryReturnType
	// CursorColumns specifies the columns and sort directions used for cursor pagination.
	// Only set when ReturnType is ReturnPaginated.
	// The first entry is the primary sort key (e.g., created_at),
	// the second is the tiebreaker (e.g., id).
	// Each OrderByExpr carries both the column and the sort direction (Desc bool).
	CursorColumns []OrderByExpr
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

// MustDefinePaginated registers a query with cursor-based pagination.
// The runner generates:
//   - Two SQL variants (base and with-cursor) at compile time
//   - Cursor types, encode/decode helpers
//   - A method that handles LIMIT+1, cursor WHERE injection, and NextCursor
//
// cursorCols specifies the columns and sort directions used for cursor ordering
// and comparison. Typically two OrderByExpr values: a timestamp (primary sort)
// and an ID (tiebreaker). Use .Desc() for newest-first or .Asc() for oldest-first.
//
// MustDefinePaginated panics if:
//
//   - name is empty
//
//   - ast is nil
//
//   - no cursor columns are provided
//
//   - a query with the same name is already registered
//
//     func init() {
//     query.MustDefinePaginated("ListPosts",
//     query.From(schema.Posts).
//     Select(schema.Posts.PublicId(), schema.Posts.Title()).
//     Where(schema.Posts.DeletedAt().IsNull()).
//     Build(),
//     schema.Posts.CreatedAt().Desc(),
//     schema.Posts.Id().Desc(),
//     )
//     }
//
// The generated method will return (*{Name}Result, error) with Items and NextCursor.
func MustDefinePaginated(name string, ast *AST, cursorCols ...OrderByExpr) *AST {
	if name == "" {
		panic("query name cannot be empty")
	}
	if ast == nil {
		panic("query AST cannot be nil")
	}
	if len(cursorCols) == 0 {
		panic("MustDefinePaginated requires at least one cursor column")
	}
	rq := RegisteredQuery{
		AST:           ast,
		ReturnType:    ReturnPaginated,
		CursorColumns: cursorCols,
	}
	if _, loaded := registry.LoadOrStore(name, rq); loaded {
		panic("duplicate query name: " + name)
	}
	return ast
}

// MustDefinePaginatedDesc is a convenience wrapper around MustDefinePaginated
// that accepts bare Column values and sorts them all descending (newest first).
// This preserves backward compatibility for callers that don't need to specify direction.
//
// Deprecated: Use MustDefinePaginated with explicit .Desc() or .Asc() calls instead.
func MustDefinePaginatedDesc(name string, ast *AST, cursorCols ...Column) *AST {
	exprs := make([]OrderByExpr, len(cursorCols))
	for i, col := range cursorCols {
		exprs[i] = OrderByExpr{Expr: ColumnExpr{Column: col}, Desc: true}
	}
	return MustDefinePaginated(name, ast, exprs...)
}

// MustDefineBulkExec registers a bulk insert query.
// The generated method accepts []Params and executes a multi-row INSERT.
//
// MustDefineBulkExec panics if:
//   - name is empty
//   - ast is nil
//   - a query with the same name is already registered
//
// The AST should contain a single "template" row in InsertRows. At code
// generation time, this template is used to derive the per-row parameter
// shape. At runtime the generated method accepts a []Params slice and
// builds the multi-row INSERT dynamically.
//
//	func init() {
//	    query.MustDefineBulkExec("BulkInsertAuthors",
//	        query.InsertInto(schema.Authors).
//	            Columns(schema.Authors.Name(), schema.Authors.Email()).
//	            AddRow(query.Param[string]("name"), query.Param[string]("email")).
//	            Build(),
//	    )
//	}
//
// The generated method will accept a []BulkInsertAuthorsParams slice and
// build the multi-row INSERT at runtime.
func MustDefineBulkExec(name string, ast *AST) *AST {
	return mustDefineQuery(name, ast, ReturnBulkExec)
}

// TryDefineBulkExec registers a bulk insert query.
// Unlike MustDefineBulkExec, this returns an error instead of panicking.
// Use this in tools or tests where you want to handle registration errors gracefully.
func TryDefineBulkExec(name string, ast *AST) (*AST, error) {
	return tryDefineQuery(name, ast, ReturnBulkExec)
}

// =============================================================================
// Non-panicking registration functions
// =============================================================================

// tryDefineQuery is the internal non-panicking registration function.
// Returns an error instead of panicking on invalid input.
func tryDefineQuery(name string, ast *AST, returnType QueryReturnType) (*AST, error) {
	if name == "" {
		return nil, errors.New("query name cannot be empty")
	}
	if ast == nil {
		return nil, errors.New("query AST cannot be nil")
	}
	rq := RegisteredQuery{
		AST:        ast,
		ReturnType: returnType,
	}
	if _, loaded := registry.LoadOrStore(name, rq); loaded {
		return nil, errors.New("duplicate query name: " + name)
	}
	return ast, nil
}

// TryDefineOne registers a query that returns 0 or 1 row.
// Unlike MustDefineOne, this returns an error instead of panicking.
// Use this in tools or tests where you want to handle registration errors gracefully.
func TryDefineOne(name string, ast *AST) (*AST, error) {
	return tryDefineQuery(name, ast, ReturnOne)
}

// TryDefineMany registers a query that returns 0 to N rows.
// Unlike MustDefineMany, this returns an error instead of panicking.
// Use this in tools or tests where you want to handle registration errors gracefully.
func TryDefineMany(name string, ast *AST) (*AST, error) {
	return tryDefineQuery(name, ast, ReturnMany)
}

// TryDefineExec registers a query that executes without returning rows.
// Unlike MustDefineExec, this returns an error instead of panicking.
// Use this in tools or tests where you want to handle registration errors gracefully.
func TryDefineExec(name string, ast *AST) (*AST, error) {
	return tryDefineQuery(name, ast, ReturnExec)
}

// TryDefinePaginated registers a cursor-paginated query.
// Unlike MustDefinePaginated, this returns an error instead of panicking.
func TryDefinePaginated(name string, ast *AST, cursorCols ...OrderByExpr) (*AST, error) {
	if name == "" {
		return nil, errors.New("query name cannot be empty")
	}
	if ast == nil {
		return nil, errors.New("query AST cannot be nil")
	}
	if len(cursorCols) == 0 {
		return nil, errors.New("TryDefinePaginated requires at least one cursor column")
	}
	rq := RegisteredQuery{
		AST:           ast,
		ReturnType:    ReturnPaginated,
		CursorColumns: cursorCols,
	}
	if _, loaded := registry.LoadOrStore(name, rq); loaded {
		return nil, errors.New("duplicate query name: " + name)
	}
	return ast, nil
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
