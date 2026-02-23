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
func GetPetById(ctx context.Context, db Queryer, params GetPetByIdParams) (*GetPetByIdResult, error)
```

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

This produces a result where the `books` field is a JSON array of objects.

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

## Compiling Queries

After writing your query definitions, run:

```sh
shipq db compile
```

This generates:
- `shipq/queries/types.go` — shared parameter and result types
- `shipq/queries/<dialect>/runner.go` — dialect-specific query runner with typed methods

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
