---
title: "Building a Full App: End-to-End Example"
description: Walk through building a complete application with authentication, scoped resources, nested relationships, and tests — from zero to a running server.
---

This guide walks through building a **bookstore API** from scratch. By the end, you'll have:

- Cookie-based authentication with signup
- Organization-scoped multi-tenancy
- Two related resources: `authors` and `books` (with a foreign key)
- Full CRUD endpoints with generated tests
- Tenancy isolation tests proving data can't leak between organizations
- An OpenAPI spec, docs UI, and TypeScript client

This mirrors the patterns used in ShipQ's own end-to-end test suite.

## Step 0: Prerequisites

Make sure you have:

- **Go 1.25+** installed
- **ShipQ CLI** built from source ([Installation](/getting-started/installation/))
- A database engine available (Postgres, MySQL, or SQLite)

For this walkthrough, we'll assume Postgres. If you don't have Postgres, ShipQ will fall back to SQLite automatically.

## Step 1: Initialize the Project

```sh
mkdir bookstore && cd bookstore
shipq init
```

This creates:
- `go.mod` with your module name
- `shipq.ini` with `[db]` and `[typescript]` sections
- `.gitignore` configured to exclude `.shipq/`

## Step 2: Set Up the Database

```sh
# For Postgres:
export DATABASE_URL="postgres://localhost:5432/bookstore_dev?sslmode=disable"
shipq db setup

# Or just let ShipQ auto-detect (falls back to SQLite if no DB server found):
# shipq db setup
```

Verify that `shipq.ini` now has a `database_url`:

```ini
[db]
database_url = postgres://localhost:5432/bookstore_dev?sslmode=disable

[typescript]
framework = react
http_output = .
```

## Step 3: Generate Authentication

```sh
shipq auth
go mod tidy
```

This generates:
- Migrations for `organizations`, `accounts`, and `sessions`
- Login/logout/me handlers in `api/auth/`
- Auth middleware that protects routes by default
- Tests in `api/auth/spec/`

Now add the signup endpoint:

```sh
shipq signup
go mod tidy
```

Verify the auth system works:

```sh
go test ./api/auth/spec/... -v -count=1
```

You should see all auth tests passing — login, logout, session management, and signup.

## Step 4: Configure Multi-Tenancy

Open `shipq.ini` and add the `scope` setting under `[db]`:

```ini
[db]
database_url = postgres://localhost:5432/bookstore_dev?sslmode=disable
scope = organization_id

[auth]
protect_by_default = true

[typescript]
framework = react
http_output = .
```

Setting `scope = organization_id` tells ShipQ to:
- Auto-inject `organization_id:references:organizations` into every new migration
- Generate queries that filter by `organization_id`
- Generate handlers that extract `organization_id` from the authenticated user's context
- Generate tenancy isolation tests

:::caution
Set `scope` now, before creating any business tables. Adding it later requires regenerating everything.
:::

## Step 5: Create the Authors Table

```sh
shipq migrate new authors name:string bio:text
```

Because `scope = organization_id` is set, ShipQ automatically injects `organization_id:references:organizations` into the migration. The effective column list is:

```
name:string bio:text organization_id:references:organizations
```

You don't have to type the scope column — it's injected by the compiler.

## Step 6: Create the Books Table

```sh
shipq migrate new books title:string isbn:string published:bool author_id:references:authors
```

Again, `organization_id` is auto-injected. This migration creates a `books` table with:
- `title` (string)
- `isbn` (string)
- `published` (bool)
- `author_id` (foreign key → `authors`)
- `organization_id` (foreign key → `organizations`, auto-injected)
- Plus the standard `id`, `public_id`, `created_at`, `updated_at`, `deleted_at` columns

## Step 7: Apply All Migrations

```sh
shipq migrate up
```

This runs the schema compiler, which:
1. Discovers all migration files (auth tables + authors + books)
2. Builds a canonical `MigrationPlan`
3. Writes `shipq/db/migrate/schema.json`
4. Generates typed schema bindings in `shipq/db/schema/schema.go`
5. Applies the schema to both dev and test databases

After this step, you have typed table and column references available:
- `schema.Authors.Name()`, `schema.Authors.Bio()`, etc.
- `schema.Books.Title()`, `schema.Books.AuthorId()`, etc.

## Step 8: Generate CRUD Endpoints

Generate all CRUD operations for both resources:

```sh
shipq resource authors all
shipq resource books all
go mod tidy
```

For each resource, this generates:

| File | Method | Route |
|------|--------|-------|
| `api/authors/create.go` | `POST` | `/authors` |
| `api/authors/get_one.go` | `GET` | `/authors/:id` |
| `api/authors/list.go` | `GET` | `/authors` |
| `api/authors/update.go` | `PATCH` | `/authors/:id` |
| `api/authors/soft_delete.go` | `DELETE` | `/authors/:id` |
| `api/books/create.go` | `POST` | `/books` |
| `api/books/get_one.go` | `GET` | `/books/:id` |
| `api/books/list.go` | `GET` | `/books` |
| `api/books/update.go` | `PATCH` | `/books/:id` |
| `api/books/soft_delete.go` | `DELETE` | `/books/:id` |

Plus `register.go`, query definitions, and tests for each.

Since `protect_by_default = true` (from `shipq auth`), **all routes require authentication**. The generated tests include both authenticated CRUD tests and 401 rejection tests.

Since `scope = organization_id` is set, **all routes are tenant-scoped**. The generated tests include tenancy isolation tests.

## What the Generated Code Looks Like

Let's peek inside what `shipq resource books all` actually produced. This is the code you'll find in your project — and it's yours to read, understand, and customize.

### Generated query definitions (`querydefs/books/queries.go`)

ShipQ generates a PortSQL query definition file that registers all five CRUD queries at `init()` time. Here's a simplified view of what the `books` queries look like:

```go
// querydefs/books/queries.go
// Code generated by shipq. You may edit this file to customise your CRUD queries.
package books

import (
	"bookstore/shipq/db/schema"
	"bookstore/shipq/lib/db/portsql/query"
)

func init() {
	// GET — fetch one book by public_id, with author resolved via JOIN
	query.MustDefineOne("GetBookByPublicId",
		query.From(schema.Books).
			LeftJoin(schema.Authors).On(
				schema.Books.AuthorId().EqCol(schema.Authors.Id()),
			).
			Select(
				schema.Books.PublicId(),
				schema.Books.Title(),
				schema.Books.Isbn(),
				schema.Books.Published(),
				schema.Books.CreatedAt(),
				schema.Books.UpdatedAt(),
			).
			SelectAs(schema.Authors.PublicId(), "author_id").
			Where(
				query.And(
					schema.Books.PublicId().Eq(query.Param[string]("publicId")),
					schema.Books.DeletedAt().IsNull(),
					schema.Books.OrganizationId().Eq(query.Param[int64]("organizationId")),
				),
			).
			Build())

	// LIST — paginated, newest first, with author resolved via JOIN
	query.MustDefinePaginated("ListBooks",
		query.From(schema.Books).
			LeftJoin(schema.Authors).On(
				schema.Books.AuthorId().EqCol(schema.Authors.Id()),
			).
			Select(
				schema.Books.PublicId(),
				schema.Books.Title(),
				schema.Books.Isbn(),
				schema.Books.Published(),
				schema.Books.CreatedAt(),
				schema.Books.UpdatedAt(),
			).
			SelectAs(schema.Authors.PublicId(), "author_id").
			Where(
				query.And(
					schema.Books.DeletedAt().IsNull(),
					schema.Books.OrganizationId().Eq(query.Param[int64]("organizationId")),
				),
			).
			Build(),
		schema.Books.CreatedAt().Desc(),
		schema.Books.PublicId().Desc(),
	)

	// CREATE — inserts a book, resolving author_id from public_id via subquery
	query.MustDefineOne("CreateBook",
		query.InsertInto(schema.Books).
			Columns(
				schema.Books.PublicId(),
				schema.Books.Title(),
				schema.Books.Isbn(),
				schema.Books.Published(),
				schema.Books.AuthorId(),
				schema.Books.OrganizationId(),
			).
			Values(
				query.Param[string]("publicId"),
				query.Param[string]("title"),
				query.Param[string]("isbn"),
				query.Param[bool]("published"),
				// FK resolution: accepts author's public_id, resolves to internal id
				query.Subquery(
					query.From(schema.Authors).
						Select(schema.Authors.Id()).
						Where(schema.Authors.PublicId().Eq(query.Param[string]("authorId")))),
				query.Param[int64]("organizationId"),
			).
			Returning(
				schema.Books.Id(),
				schema.Books.PublicId(),
			).
			Build())

	// UPDATE — partial update by public_id
	query.MustDefineExec("UpdateBook",
		query.Update(schema.Books).
			Set(schema.Books.Title(), query.Param[string]("title")).
			Set(schema.Books.Isbn(), query.Param[string]("isbn")).
			Set(schema.Books.Published(), query.Param[bool]("published")).
			Where(
				query.And(
					schema.Books.PublicId().Eq(query.Param[string]("publicId")),
					schema.Books.DeletedAt().IsNull(),
					schema.Books.OrganizationId().Eq(query.Param[int64]("organizationId")),
				),
			).
			Build())

	// DELETE — soft delete (sets deleted_at)
	query.MustDefineExec("SoftDeleteBook",
		query.Update(schema.Books).
			Set(schema.Books.DeletedAt(), query.Literal("CURRENT_TIMESTAMP")).
			Where(
				query.And(
					schema.Books.PublicId().Eq(query.Param[string]("publicId")),
					schema.Books.DeletedAt().IsNull(),
					schema.Books.OrganizationId().Eq(query.Param[int64]("organizationId")),
				),
			).
			Build())
}
```

Key things to notice:

- **`organization_id` is everywhere** — because `scope = organization_id` is set, every query filters by it. This is injected by the code generator, not hand-written.
- **Foreign keys resolve via subquery** — `author_id` in the CREATE query uses `query.Subquery(...)` to look up the internal integer ID from the author's public ID string. API consumers never see internal IDs.
- **JOINs in GET and LIST** — the read queries JOIN to the `authors` table and use `SelectAs` to include the author's public ID in the result. The response shows `"author_id": "abc123"` (a string), not the raw integer FK.
- **Soft delete** — DELETE sets `deleted_at` rather than removing the row. All read queries include `deleted_at IS NULL`.

### Generated query runner (after `shipq db compile`)

Running `shipq db compile` turns those PortSQL definitions into typed Go code. Here's what the generated types look like for the books queries:

```go
// shipq/queries/types.go (generated, books section)

type CreateBookParams struct {
	PublicId       string `json:"publicId"`
	Title          string `json:"title"`
	Isbn           string `json:"isbn"`
	Published      bool   `json:"published"`
	AuthorId       string `json:"authorId"`
	OrganizationId int64  `json:"organizationId"`
}

type CreateBookResult struct {
	Id       int64  `json:"id"`
	PublicId string `json:"public_id"`
}

type GetBookByPublicIdParams struct {
	PublicId       string `json:"publicId"`
	OrganizationId int64  `json:"organizationId"`
}

type GetBookByPublicIdResult struct {
	PublicId  string    `json:"public_id"`
	Title     string    `json:"title"`
	Isbn      string    `json:"isbn"`
	Published bool      `json:"published"`
	AuthorId  *string   `json:"author_id"` // nullable — resolved from JOIN
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
```

And the generated runner method (Postgres dialect) looks like:

```go
// shipq/queries/postgres/runner.go (generated, simplified)

func (r *Runner) GetBookByPublicId(ctx context.Context, params GetBookByPublicIdParams) (*GetBookByPublicIdResult, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT "books"."public_id", "books"."title", "books"."isbn", "books"."published",
		        "authors"."public_id" AS "author_id",
		        "books"."created_at", "books"."updated_at"
		 FROM "books"
		 LEFT JOIN "authors" ON "books"."author_id" = "authors"."id"
		 WHERE "books"."public_id" = $1
		   AND "books"."deleted_at" IS NULL
		   AND "books"."organization_id" = $2`,
		params.PublicId, params.OrganizationId,
	)
	var result GetBookByPublicIdResult
	err := row.Scan(
		&result.PublicId, &result.Title, &result.Isbn, &result.Published,
		&result.AuthorId, &result.CreatedAt, &result.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &result, nil
}
```

If you switch to MySQL, the same PortSQL definition generates `` `backtick` ``-quoted identifiers and `?` placeholders instead — you never change your query definitions.

### Generated handler (`api/books/create.go`)

Here's the generated Create handler that ties the query runner to HTTP:

```go
// api/books/create.go
// Code generated by shipq. You may edit this file to customise your handler.
package books

import (
	"context"
	"time"

	"bookstore/shipq/lib/httperror"
	"bookstore/shipq/lib/httputil"
	"bookstore/shipq/lib/nanoid"
	"bookstore/shipq/queries"
)

type CreateBookRequest struct {
	Title     string `json:"title"`
	Isbn      string `json:"isbn"`
	Published bool   `json:"published"`
	AuthorId  string `json:"author_id"`
}

type CreateBookResponse struct {
	PublicId  string `json:"id"`
	Title     string `json:"title"`
	Isbn      string `json:"isbn"`
	Published bool   `json:"published"`
	AuthorId  *string `json:"author_id"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func CreateBook(ctx context.Context, req *CreateBookRequest) (*CreateBookResponse, error) {
	runner := queries.RunnerFromContext(ctx)

	orgID, ok := httputil.OrganizationIDFromContext(ctx)
	if !ok {
		return nil, httperror.Wrap(403, "organization context missing", nil)
	}

	publicId := nanoid.New()

	_, err := runner.CreateBook(ctx, queries.CreateBookParams{
		PublicId:       publicId,
		Title:          req.Title,
		Isbn:           req.Isbn,
		Published:      req.Published,
		AuthorId:       req.AuthorId,
		OrganizationId: orgID,
	})
	if err != nil {
		return nil, httperror.Wrap(500, "failed to create book", err)
	}

	// Re-fetch via GET query to get resolved JOINs (author public_id)
	result, err := runner.GetBookByPublicId(ctx, queries.GetBookByPublicIdParams{
		PublicId:       publicId,
		OrganizationId: orgID,
	})
	if err != nil {
		return nil, httperror.Wrap(500, "failed to fetch created book", err)
	}

	return &CreateBookResponse{
		PublicId:  result.PublicId,
		Title:     result.Title,
		Isbn:      result.Isbn,
		Published: result.Published,
		AuthorId:  result.AuthorId,
		CreatedAt: result.CreatedAt.Format(time.RFC3339),
		UpdatedAt: result.UpdatedAt.Format(time.RFC3339),
	}, nil
}
```

Key things to notice:

- **`organization_id` comes from context, not from the request** — the scope column is extracted from the authenticated session. API consumers never send it, and it's never in the response.
- **`author_id` in the request is a public ID string** — the consumer sends `"author_id": "abc123"`, and the generated INSERT query's subquery resolves it to the internal integer FK.
- **Re-fetch after create** — the handler inserts the row, then re-fetches it using the GET query. This ensures the response includes the resolved JOIN fields (author's public ID) without duplicating the JOIN logic.
- **`nanoid.New()`** — public IDs are generated in the handler, not the database, so the same ID can be used for both the INSERT and the re-fetch.

### Generated `register.go`

This file wires every handler to its HTTP route:

```go
// api/books/register.go
// Code generated by shipq. You may edit this file to add custom routes.
package books

import "bookstore/shipq/lib/handler"

func Register(app *handler.App) {
	app.Post("/books", CreateBook).Auth()
	app.Get("/books", ListBooks).Auth()
	app.Get("/books/:id", GetBook).Auth()
	app.Patch("/books/:id", UpdateBook).Auth()
	app.Delete("/books/:id", SoftDeleteBook).Auth()

	// Admin routes (GLOBAL_OWNER only, includes soft-deleted records)
	app.Get("/admin/books", AdminListBooks).Auth()
	app.Patch("/admin/books/:id/restore", UndeleteBook).Auth()
}
```

The handler compiler reads this file, uses reflection on the handler functions to extract request/response types, and generates: the `cmd/server/main.go` wiring, the OpenAPI 3.1 spec, the TypeScript HTTP client, and the test harness. **You don't write any of that glue code.**

### How the full flow works

Here's what happens when a client sends `POST /books`:

1. **HTTP request arrives** → generated `cmd/server/main.go` routes it to `books.CreateBook`
2. **Auth middleware** checks the session cookie, extracts account ID and org ID, injects them into `context.Context`
3. **JSON deserialization** — the generated server reads the request body into a `CreateBookRequest` struct
4. **Handler runs** — calls `queries.RunnerFromContext(ctx)` to get the typed query runner, then `runner.CreateBook(ctx, params)`
5. **SQL executes** — the generated runner runs dialect-specific SQL: `INSERT INTO "books" ... VALUES ($1, $2, ..., (SELECT "id" FROM "authors" WHERE "public_id" = $5))` — the FK subquery resolves the author's public ID to its internal ID
6. **Re-fetch** — the handler calls `runner.GetBookByPublicId(ctx, ...)` which runs a LEFT JOIN to get the author's public ID
7. **Response** — the `*CreateBookResponse` is serialized to JSON and written to the HTTP response with status 201

No router setup. No middleware wiring. No JSON marshaling code. No SQL strings.

## Step 9: Run the Full Test Suite

```sh
go test ./... -v -count=1
```

This runs **every generated test**, including:

- **Auth tests**: login, logout, signup, session management
- **Authors CRUD tests**: create, read, update, delete with valid auth
- **Books CRUD tests**: create, read, update, delete with valid auth
- **401 tests**: verifying unauthenticated requests are rejected
- **Tenancy isolation tests**: verifying Organization A's data is invisible to Organization B

The tenancy tests follow this pattern:
1. Create User A in Organization A
2. Create User B in Organization B
3. User A creates a resource (e.g., an author)
4. User B tries to read that resource → gets 404 (not 200)
5. User B lists resources → gets an empty list (not User A's data)

If all tests pass, your data isolation is correct by construction.

## Step 10: Start the Server

```sh
go run ./cmd/server
# or:
shipq start server
```

Your API is now running! In dev mode, visit:

- **`GET /docs`** — Interactive API documentation with all 10+ endpoints
- **`GET /openapi`** — Raw OpenAPI 3.1 JSON spec

## Step 11: Test It Manually

### Sign up a new user

```sh
curl -c cookies.txt -X POST http://localhost:8080/auth/signup \
  -H "Content-Type: application/json" \
  -d '{"email": "alice@example.com", "password": "securepassword123"}'
```

The response sets a signed `session` cookie. The `-c cookies.txt` flag saves it for subsequent requests.

### Log in

```sh
curl -c cookies.txt -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email": "alice@example.com", "password": "securepassword123"}'
```

### Create an author

```sh
curl -b cookies.txt -X POST http://localhost:8080/authors \
  -H "Content-Type: application/json" \
  -d '{"name": "J.R.R. Tolkien", "bio": "Author of The Lord of the Rings"}'
```

### Create a book

```sh
curl -b cookies.txt -X POST http://localhost:8080/books \
  -H "Content-Type: application/json" \
  -d '{"title": "The Hobbit", "isbn": "978-0547928227", "published": true, "author_id": "<author-public-id>"}'
```

### List books

```sh
curl -b cookies.txt http://localhost:8080/books
```

## What You've Built

Let's take stock of everything ShipQ generated from just a handful of commands:

### Commands you typed

```sh
shipq init
shipq db setup
shipq auth
shipq signup
shipq migrate new authors name:string bio:text
shipq migrate new books title:string isbn:string published:bool author_id:references:authors
shipq migrate up
shipq resource authors all
shipq resource books all
shipq handler compile
```

### What was generated

- ✅ **Database schema**: 5 tables (organizations, accounts, sessions, authors, books) with proper foreign keys, indexes, and soft-delete support
- ✅ **Cookie-based authentication**: signup, login, logout, session management
- ✅ **10 CRUD endpoints**: full create/read/update/delete/list for both authors and books
- ✅ **Multi-tenancy**: every query scoped to `organization_id`, enforced at the SQL level
- ✅ **Typed query runners**: compile-time type-safe database access for all operations
- ✅ **Comprehensive tests**: auth tests, CRUD tests, 401 tests, tenancy isolation tests
- ✅ **OpenAPI 3.1 spec**: auto-generated from handler metadata
- ✅ **API docs UI**: interactive documentation at `/docs`
- ✅ **Admin UI**: for manual testing and exploration
- ✅ **TypeScript HTTP client**: fully typed, ready for your frontend
- ✅ **React hooks**: data-fetching hooks for every endpoint
- ✅ **Self-contained Go project**: no runtime dependency on ShipQ

## Going Further

From here, you can:

- **Add file uploads**: `shipq files` → S3-compatible managed file storage
- **Add background jobs**: `shipq workers` → Redis job queue + Centrifugo WebSocket channels
- **Add OAuth**: `shipq auth google` or `shipq auth github` for social login
- **Add email verification**: `shipq email` for email verification and password reset (requires workers)
- **Write custom queries**: add query definitions in `querydefs/` using the [PortSQL DSL](/guides/queries/)
- **Write custom handlers**: add handler packages in `api/` and register them with the handler registry
- **Deploy**: `shipq docker` to generate production Dockerfiles

## Iteration Cheat Sheet

As you develop, here's the command to run when things change:

| What changed | Command |
|---|---|
| New or edited migration | `shipq migrate up` |
| New or edited query definition | `shipq db compile` |
| New or edited handler | `shipq handler compile` |
| New table + full CRUD | `shipq migrate new <table> ...` → `shipq resource <table> all` |
| Channel definitions changed | `shipq workers compile` |
| Everything (nuclear option) | `shipq migrate reset` → regenerate resources → `shipq handler compile` |
