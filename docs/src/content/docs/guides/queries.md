---
title: Queries (PortSQL)
description: Write type-safe, multi-database queries using ShipQ's PortSQL DSL.
---

PortSQL is ShipQ's typed SQL DSL. It lets you write queries in Go that compile to correct SQL for **Postgres**, **MySQL**, and **SQLite** — handling quoting, placeholders, JSON aggregation, ILIKE translation, and other dialect differences automatically.

## How It Works

1. You write **query definitions** under `querydefs/` using the PortSQL builder API.
2. Queries are **registered at `init()` time** in a global registry.
3. `shipq db compile` runs the **query compiler**, which serializes each query's AST and generates typed runner code.
4. You call the generated runner methods from your handlers.

## Writing Query Definitions

Query definitions live in Go files under `querydefs/`. Each file is a package whose `init()` function registers queries.

### A Basic Query

```go
package querydefs

import (
	"myapp/shipq/db/schema"
	"myapp/shipq/lib/db/portsql/query"
)

func init() {
	query.MustDefineOne("GetPetById",
		query.From(schema.Pets).
			Select(
				schema.Pets.Id(),
				schema.Pets.Name(),
				schema.Pets.Species(),
				schema.Pets.Age(),
			).
			Where(schema.Pets.Id().Eq(query.Param[int64]("id"))).
			Build(),
	)
}
```

This registers a query named `"GetPetById"` that:
- Selects from the `pets` table
- Returns `id`, `name`, `species`, and `age` columns
- Filters by `id` using a typed parameter

After running `shipq db compile`, you get a generated method like:

```go
func (r *Runner) GetPetById(ctx context.Context, params GetPetByIdParams) (*GetPetByIdResult, error)
```

## What `shipq db compile` Generates

This is the part most documentation glosses over. Let's look at what actually appears in your project after you run `shipq db compile`.

### Generated param and result types

For every query you define, ShipQ generates a **params struct** (the inputs) and a **result struct** (the outputs) in `shipq/queries/types.go`:

```go
// shipq/queries/types.go (generated)

// --- GetPetByPublicId ---

type GetPetByPublicIdParams struct {
	PublicId       string `json:"publicId"`
	OrganizationId int64  `json:"organizationId"`
}

type GetPetByPublicIdResult struct {
	PublicId  string    `json:"public_id"`
	Name      string    `json:"name"`
	Species   string    `json:"species"`
	Age       int       `json:"age"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// --- CreatePet ---

type CreatePetParams struct {
	PublicId       string `json:"publicId"`
	Name           string `json:"name"`
	Species        string `json:"species"`
	Age            int    `json:"age"`
	OrganizationId int64  `json:"organizationId"`
}

type CreatePetResult struct {
	Id       int64  `json:"id"`
	PublicId string `json:"public_id"`
}

// --- ListPets (paginated) ---

type ListPetsParams struct {
	OrganizationId int64          `json:"organizationId"`
	Limit          int            `json:"limit"`
	Cursor         *ListPetsCursor `json:"cursor"`
}

type ListPetsCursor struct {
	CreatedAt time.Time `json:"created_at"`
	PublicId  string    `json:"public_id"`
}

type ListPetsItem struct {
	PublicId  string    `json:"public_id"`
	Name      string    `json:"name"`
	Species   string    `json:"species"`
	Age       int       `json:"age"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ListPetsResult struct {
	Items      []ListPetsItem  `json:"items"`
	NextCursor *ListPetsCursor `json:"next_cursor"`
}

// --- SearchPets (custom query, MustDefineMany) ---

type SearchPetsParams struct {
	Search         string `json:"search"`
	OrganizationId int64  `json:"organizationId"`
}

type SearchPetsResult struct {
	PublicId string `json:"public_id"`
	Name     string `json:"name"`
	Species  string `json:"species"`
	Age      int    `json:"age"`
}
```

Every field is strongly typed. The Go types are derived from your schema column types — `string` columns become `string`, `int` columns become `int`, `datetime` columns become `time.Time`, nullable columns become pointers.

### Generated runner methods

The runner itself is generated in `shipq/queries/<dialect>/runner.go` with a method per query:

```go
// shipq/queries/postgres/runner.go (generated, simplified)

type Runner struct {
	db Queryer
}

func (r *Runner) GetPetByPublicId(ctx context.Context, params GetPetByPublicIdParams) (*GetPetByPublicIdResult, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT "pets"."public_id", "pets"."name", "pets"."species", "pets"."age",
		        "pets"."created_at", "pets"."updated_at"
		 FROM "pets"
		 WHERE "pets"."public_id" = $1
		   AND "pets"."deleted_at" IS NULL
		   AND "pets"."organization_id" = $2`,
		params.PublicId, params.OrganizationId,
	)
	var result GetPetByPublicIdResult
	err := row.Scan(
		&result.PublicId, &result.Name, &result.Species, &result.Age,
		&result.CreatedAt, &result.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil // MustDefineOne: nil means "not found"
	}
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (r *Runner) CreatePet(ctx context.Context, params CreatePetParams) (*CreatePetResult, error) {
	row := r.db.QueryRowContext(ctx,
		`INSERT INTO "pets" ("public_id", "name", "species", "age", "organization_id")
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING "id", "public_id"`,
		params.PublicId, params.Name, params.Species, params.Age, params.OrganizationId,
	)
	var result CreatePetResult
	err := row.Scan(&result.Id, &result.PublicId)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (r *Runner) SearchPets(ctx context.Context, params SearchPetsParams) ([]SearchPetsResult, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT "pets"."public_id", "pets"."name", "pets"."species", "pets"."age"
		 FROM "pets"
		 WHERE "pets"."name" ILIKE $1
		   AND "pets"."deleted_at" IS NULL
		   AND "pets"."organization_id" = $2
		 ORDER BY "pets"."name" ASC`,
		params.Search, params.OrganizationId,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []SearchPetsResult
	for rows.Next() {
		var item SearchPetsResult
		if err := rows.Scan(&item.PublicId, &item.Name, &item.Species, &item.Age); err != nil {
			return nil, err
		}
		results = append(results, item)
	}
	return results, rows.Err()
}
```

That SQL was generated from your PortSQL definition. If you switch your database from Postgres to MySQL, `shipq db compile` regenerates the runner with backtick-quoted identifiers, `?` placeholders, `LOWER(col) LIKE LOWER(?)` instead of `ILIKE`, and other dialect differences. **You never change your query definitions.**

### Cursor helpers

For paginated queries, ShipQ also generates encode/decode helpers:

```go
// shipq/queries/types.go (generated)

func DecodeListPetsCursor(encoded string) *ListPetsCursor { ... }
func EncodeListPetsCursor(cursor *ListPetsCursor) string  { ... }
```

These use base64-encoded JSON internally. The cursor is opaque to API consumers.

### The RunnerFromContext pattern

The generated code also includes a context-based accessor so handlers can get the runner without knowing the dialect:

```go
// shipq/queries/context.go (generated)

func RunnerFromContext(ctx context.Context) *Runner { ... }
```

The generated `cmd/server/main.go` injects the runner into the request context based on the configured database dialect. Your handler code just calls `queries.RunnerFromContext(ctx)` and gets back a typed runner — no dialect awareness needed.

### Calling generated queries from a handler

Here's the full pattern — from query definition to handler call:

**Step 1: Define the query** in `querydefs/`:

```go
// querydefs/pets/queries.go
func init() {
	query.MustDefineMany("FindPetsBySpecies",
		query.From(schema.Pets).
			Select(
				schema.Pets.PublicId(),
				schema.Pets.Name(),
				schema.Pets.Species(),
				schema.Pets.Age(),
			).
			Where(
				query.And(
					schema.Pets.Species().Eq(query.Param[string]("species")),
					schema.Pets.DeletedAt().IsNull(),
					schema.Pets.OrganizationId().Eq(query.Param[int64]("organizationId")),
				),
			).
			Build(),
	)
}
```

**Step 2: Run `shipq db compile`** — generates `FindPetsBySpeciesParams`, `FindPetsBySpeciesResult`, and the runner method.

**Step 3: Call it from a handler:**

```go
// api/pets/find_by_species.go
func FindPetsBySpecies(ctx context.Context, req *FindBySpeciesRequest) (*FindBySpeciesResponse, error) {
	runner := queries.RunnerFromContext(ctx)

	orgID, ok := httputil.OrganizationIDFromContext(ctx)
	if !ok {
		return nil, httperror.Wrap(403, "organization context missing", nil)
	}

	results, err := runner.FindPetsBySpecies(ctx, queries.FindPetsBySpeciesParams{
		Species:        req.Species,
		OrganizationId: orgID,
	})
	if err != nil {
		return nil, httperror.Wrap(500, "failed to find pets", err)
	}

	items := make([]PetItem, len(results))
	for i, r := range results {
		items[i] = PetItem{
			PublicId: r.PublicId,
			Name:    r.Name,
			Species: r.Species,
			Age:     r.Age,
		}
	}

	return &FindBySpeciesResponse{Items: items}, nil
}
```

**Step 4: Register the route** in `api/pets/register.go`:

```go
app.Get("/pets/by-species", FindPetsBySpecies).Auth()
```

**Step 5: Run `shipq handler compile`** — the new endpoint appears in the OpenAPI spec, TypeScript client, and test harness.

## Registration Functions

ShipQ provides four registration functions, each generating a different return signature:

### `MustDefineOne` — Returns 0 or 1 row

Use for lookups by unique key (get by ID, get by email, etc.).

```go
query.MustDefineOne("GetUserByEmail",
	query.From(schema.Users).
		Select(schema.Users.Id(), schema.Users.Email(), schema.Users.Name()).
		Where(schema.Users.Email().Eq(query.Param[string]("email"))).
		Build(),
)
```

**Generated signature:** `(*Result, error)` — `Result` is `nil` if no row found.

### `MustDefineMany` — Returns 0..N rows

Use for list queries, searches, filtered results.

```go
query.MustDefineMany("FindPetsBySpecies",
	query.From(schema.Pets).
		Select(schema.Pets.Id(), schema.Pets.Name(), schema.Pets.Species()).
		Where(schema.Pets.Species().Eq(query.Param[string]("species"))).
		Build(),
)
```

**Generated signature:** `([]Result, error)`

### `MustDefineExec` — Executes without returning rows

Use for INSERT, UPDATE, DELETE queries that don't use RETURNING.

```go
query.MustDefineExec("UpdatePetName",
	query.Update(schema.Pets).
		Set(schema.Pets.Name(), query.Param[string]("name")).
		Where(schema.Pets.Id().Eq(query.Param[int64]("id"))).
		Build(),
)
```

**Generated signature:** `(sql.Result, error)`

### `MustDefinePaginated` — Cursor-based pagination

Use for paginated list endpoints. ShipQ generates cursor types, encode/decode helpers, and a method that handles `LIMIT+1`, cursor WHERE injection, and `NextCursor` computation.

```go
query.MustDefinePaginated("ListPosts",
	query.From(schema.Posts).
		Select(schema.Posts.PublicId(), schema.Posts.Title(), schema.Posts.CreatedAt()).
		Where(schema.Posts.DeletedAt().IsNull()).
		Build(),
	schema.Posts.CreatedAt().Desc(),
	schema.Posts.Id().Desc(),
)
```

The cursor columns (last two arguments) specify the sort order and tiebreaker. Use `.Desc()` for newest-first or `.Asc()` for oldest-first.

**Generated signature:** `(*ListPostsResult, error)` with `Items` and `NextCursor` fields.

**Generated cursor helpers:**

```go
func DecodeListPostsCursor(encoded string) *ListPostsCursor
func EncodeListPostsCursor(cursor *ListPostsCursor) string
```

**How the handler uses it:**

```go
func ListPosts(ctx context.Context, req *ListPostsRequest) (*ListPostsResponse, error) {
	runner := queries.RunnerFromContext(ctx)

	// Decode cursor from request (nil on first page)
	var cursor *queries.ListPostsCursor
	if req.Cursor != nil {
		cursor = queries.DecodeListPostsCursor(*req.Cursor)
	}

	result, err := runner.ListPosts(ctx, queries.ListPostsParams{
		Limit:  20,
		Cursor: cursor,
	})
	if err != nil {
		return nil, httperror.Wrap(500, "failed to list posts", err)
	}

	// Encode next cursor for the response (nil on last page)
	var nextCursor *string
	if result.NextCursor != nil {
		encoded := queries.EncodeListPostsCursor(result.NextCursor)
		nextCursor = &encoded
	}

	return &ListPostsResponse{
		Items:      mapPostItems(result.Items),
		NextCursor: nextCursor,
	}, nil
}
```

The cursor is an opaque base64 string. The client passes it back as a query parameter (`?cursor=...`) to fetch the next page. Internally, the generated SQL uses `WHERE (created_at, id) < ($cursor_created_at, $cursor_id)` to efficiently seek without OFFSET.

## The Query Builder API

### Starting a Query

| Function | Description |
|----------|-------------|
| `query.From(table)` | Start a SELECT query from a table |
| `query.Update(table)` | Start an UPDATE query |
| `query.InsertInto(table)` | Start an INSERT query |
| `query.DeleteFrom(table)` | Start a DELETE query |

### SELECT Builder

Chain methods on the builder returned by `query.From()`:

```go
query.From(schema.Pets).
	Select(schema.Pets.Id(), schema.Pets.Name()).  // columns to select
	Distinct().                                      // SELECT DISTINCT
	Where(schema.Pets.Age().Gt(query.Literal(3))).  // WHERE clause
	GroupBy(schema.Pets.Species()).                   // GROUP BY
	Having(query.Count(schema.Pets.Id()).Gt(query.Literal(5))). // HAVING
	OrderBy(schema.Pets.Name().Asc()).               // ORDER BY
	Limit(query.Param[int]("limit")).                // LIMIT
	Offset(query.Param[int]("offset")).              // OFFSET
	Build()
```

### Aliases

Use `SelectAs` or `SelectExprAs` to alias columns in the result:

```go
query.From(schema.Pets).
	SelectAs(schema.Pets.Id(), "pet_id").
	SelectAs(schema.Pets.Name(), "pet_name").
	Build()
```

### JOINs

ShipQ supports `Join`, `LeftJoin`, `RightJoin`, and `FullJoin`:

```go
query.From(schema.Books).
	Select(
		schema.Books.Title(),
		schema.Authors.Name(),
	).
	Join(schema.Authors).On(
		schema.Books.AuthorId().Eq(schema.Authors.Id()),
	).
	Build()
```

You can alias joined tables:

```go
query.From(schema.Books).
	Select(schema.Books.Title()).
	LeftJoin(schema.Authors).As("a").On(
		schema.Books.AuthorId().EqCol(schema.Authors.Id()),
	).
	Build()
```

### JSON Aggregation

`SelectJSONAgg` generates cross-dialect JSON aggregation (works on Postgres, MySQL, and SQLite):

```go
query.From(schema.Authors).
	Select(schema.Authors.Id(), schema.Authors.Name()).
	SelectJSONAgg("books",
		schema.Books.Id(),
		schema.Books.Title(),
	).
	LeftJoin(schema.Books).On(
		schema.Books.AuthorId().Eq(schema.Authors.Id()),
	).
	GroupBy(schema.Authors.Id()).
	Build()
```

This produces a result where the `books` field is a JSON array of objects. The generated SQL differs by dialect:

| Dialect | Generated SQL |
|---------|---------------|
| Postgres | `json_agg(json_build_object('id', books.id, 'title', books.title))` |
| MySQL | `JSON_ARRAYAGG(JSON_OBJECT('id', books.id, 'title', books.title))` |
| SQLite | `json_group_array(json_object('id', books.id, 'title', books.title))` |

You write the query once; ShipQ handles the dialect translation.

### UPDATE Builder

```go
query.Update(schema.Pets).
	Set(schema.Pets.Name(), query.Param[string]("name")).
	Set(schema.Pets.Species(), query.Param[string]("species")).
	Where(schema.Pets.Id().Eq(query.Param[int64]("id"))).
	Build()
```

### INSERT Builder

```go
query.InsertInto(schema.Pets).
	Columns(schema.Pets.Name(), schema.Pets.Species(), schema.Pets.Age()).
	Values(
		query.Param[string]("name"),
		query.Param[string]("species"),
		query.Param[int]("age"),
	).
	Build()
```

### DELETE Builder

```go
query.DeleteFrom(schema.Pets).
	Where(schema.Pets.Id().Eq(query.Param[int64]("id"))).
	Build()
```

## Column Operations

Typed columns from the schema provide these comparison operations:

| Method | SQL |
|--------|-----|
| `.Eq(expr)` | `= ?` |
| `.Neq(expr)` | `!= ?` |
| `.Gt(expr)` | `> ?` |
| `.Gte(expr)` | `>= ?` |
| `.Lt(expr)` | `< ?` |
| `.Lte(expr)` | `<= ?` |
| `.IsNull()` | `IS NULL` |
| `.IsNotNull()` | `IS NOT NULL` |
| `.Like(expr)` | `LIKE ?` (translates to `ILIKE` on Postgres) |
| `.In(exprs...)` | `IN (?, ?, ...)` |

### Combining Conditions

Use `query.And(...)` and `query.Or(...)` to combine conditions:

```go
query.From(schema.Pets).
	Select(schema.Pets.Id(), schema.Pets.Name()).
	Where(
		query.And(
			schema.Pets.Species().Eq(query.Param[string]("species")),
			schema.Pets.Age().Gte(query.Param[int]("min_age")),
			query.Or(
				schema.Pets.Name().Like(query.Param[string]("search")),
				schema.Pets.Name().IsNotNull(),
			),
		),
	).
	Build()
```

## Parameters and Literals

### Parameters

`query.Param[T]("name")` defines a typed query parameter. The type parameter `T` determines the generated parameter struct field type:

```go
query.Param[string]("email")   // generates: Email string
query.Param[int64]("id")       // generates: Id int64
query.Param[int]("age")        // generates: Age int
query.Param[bool]("active")    // generates: Active bool
```

### Literals

`query.Literal(value)` embeds a constant value directly in the query:

```go
schema.Pets.Age().Gt(query.Literal(18))
// Generates: WHERE pets.age > 18
```

## Set Operations

Combine queries with UNION, INTERSECT, and EXCEPT:

```go
query.Union(
	query.From(schema.Cats).Select(schema.Cats.Name()).Build(),
	query.From(schema.Dogs).Select(schema.Dogs.Name()).Build(),
)
```

## Common Table Expressions (CTEs)

Use `With` for Common Table Expressions:

```go
query.With("recent_posts",
	query.From(schema.Posts).
		Select(schema.Posts.Id(), schema.Posts.AuthorId()).
		Where(schema.Posts.CreatedAt().Gt(query.Param[string]("since"))).
		Build(),
).From("recent_posts").
	Select(...).
	Build()
```

## Subqueries

Use subqueries in WHERE clauses:

```go
query.From(schema.Authors).
	Select(schema.Authors.Id(), schema.Authors.Name()).
	Where(
		schema.Authors.Id().In(
			query.Subquery(
				query.From(schema.Books).
					Select(schema.Books.AuthorId()).
					Where(schema.Books.Published().Eq(query.Literal(true))).
					Build(),
			),
		),
	).
	Build()
```

ShipQ also uses subqueries internally for **foreign key resolution**. When a CRUD resource has a FK column like `author_id:references:authors`, the generated CREATE query uses a subquery to resolve the public ID to the internal ID:

```go
// Generated by `shipq resource books all`
query.InsertInto(schema.Books).
	Columns(schema.Books.PublicId(), schema.Books.Title(), schema.Books.AuthorId()).
	Values(
		query.Param[string]("publicId"),
		query.Param[string]("title"),
		query.Subquery(
			query.From(schema.Authors).
				Select(schema.Authors.Id()).
				Where(schema.Authors.PublicId().Eq(query.Param[string]("authorId")))),
	).
	Build()
```

This means API consumers always pass public ID strings (`"abc123"`) rather than internal integer IDs. The subquery resolves it at the SQL level.

## Compiling Queries

After writing your query definitions, run:

```sh
shipq db compile
```

This generates:
- `shipq/queries/types.go` — shared parameter and result types
- `shipq/queries/<dialect>/runner.go` — dialect-specific query runner with typed methods

The compilation step:
1. Generates a temporary Go program that imports your `querydefs/` packages (triggering `init()`)
2. Serializes each registered query's AST into JSON
3. Compiles each AST to dialect-specific SQL for your configured database
4. Generates typed Go runner code with `Scan` calls matching the selected columns

:::tip
If you only changed query definitions (not handlers), you only need `shipq db compile`. But if handler signatures also changed, you need `shipq handler compile` to update the server wiring and TypeScript client.
:::

## Panic Behavior

The `MustDefine*` functions **panic** on:
- Empty query names
- Nil ASTs
- Duplicate query names

This is intentional — query registration happens at `init()` time, and errors should cause immediate, obvious failures. If you need non-panicking registration (e.g., in tests or tooling), use the `TryDefine*` variants:

```go
ast, err := query.TryDefineOne("GetUserByEmail", ...)
if err != nil {
	// handle error
}
```

## Multi-Database Support

A single PortSQL query compiles to correct SQL for all three supported databases. The query compiler handles:

| Feature | Postgres | MySQL | SQLite |
|---------|----------|-------|--------|
| Identifier quoting | `"column"` | `` `column` `` | `"column"` |
| Placeholders | `$1, $2, $3` | `?, ?, ?` | `?, ?, ?` |
| ILIKE | Native `ILIKE` | `LOWER(col) LIKE LOWER(?)` | `col LIKE ? (case-insensitive by default)` |
| JSON aggregation | `json_agg(json_build_object(...))` | `JSON_ARRAYAGG(JSON_OBJECT(...))` | `json_group_array(json_object(...))` |
| RETURNING | Native | Emulated | Emulated |

You never think about dialect differences — PortSQL handles them at compile time.

## Summary: The Full Lifecycle

Here's the complete lifecycle of a query in ShipQ:

1. **Define your schema** — `shipq migrate new pets name:string species:string age:int` → `shipq migrate up`
2. **Write a query definition** — create a file in `querydefs/` using the PortSQL DSL with `query.MustDefineOne`, `MustDefineMany`, `MustDefineExec`, or `MustDefinePaginated`
3. **Compile** — `shipq db compile` generates param structs, result structs, and a typed runner method with dialect-specific SQL
4. **Call from a handler** — `runner := queries.RunnerFromContext(ctx)` then `runner.YourQuery(ctx, params)` — fully typed, compile-time checked
5. **Wire to HTTP** — register the handler in `register.go`, then `shipq handler compile` picks it up and generates the server route, OpenAPI spec, TypeScript client, and tests

If you rename a column in your schema, `shipq migrate up` updates the schema bindings, `shipq db compile` updates the query runner, and any handler referencing the old field name fails at **Go compile time** — not at runtime, not in production.
