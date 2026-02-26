---
title: llms.txt
description: A comprehensive plain-text reference for LLMs to understand and work with ShipQ projects.
---

This page contains everything an LLM needs to know to work effectively with ShipQ projects. It is designed as a single, self-contained reference that can be consumed in one pass.

## What is ShipQ?

ShipQ is a Go-first backend framework/CLI that generates a production-oriented API stack from your schema and your API surface. It is NOT an ORM — it is a compiler toolchain for backend code.

At a high level:
- You declare schema changes as Go migrations.
- ShipQ compiles migrations into a canonical schema plan (`schema.json`) and generates typed schema bindings.
- You write queries in Go using a typed SQL DSL (PortSQL) against those bindings.
- ShipQ compiles queries into a type-safe query runner (SQL + params/result types) for Postgres/MySQL/SQLite.
- You write handlers (or generate them) and ShipQ compiles the handler registry into a runnable server, OpenAPI docs, a dev admin UI, TypeScript clients, and test tooling.

ShipQ makes generated projects self-contained: it embeds its runtime libraries into your repo under `shipq/lib/...` and rewrites imports so the generated project has zero dependency on a published ShipQ module.

## The Four-Compiler Chain

ShipQ is best understood as four compilers that feed each other:

### Compiler 1: Schema Compiler (`shipq migrate up`)

Input: Migration files in `migrations/*.go` — Go functions that mutate a `MigrationPlan` using a typed DDL builder.

Process:
1. Embeds ShipQ runtime libraries into `shipq/lib/...` and rewrites imports.
2. Discovers all `migrations/*.go` files.
3. Generates a TEMP Go program that imports your migrations package, executes all migration functions in order to build a canonical `MigrationPlan`, and prints the plan JSON to stdout.
4. Writes the canonical plan to `shipq/db/migrate/schema.json`.
5. Generates typed schema bindings in `shipq/db/schema/schema.go`.
6. Applies the plan against both dev and test databases.

Output artifacts:
- `shipq/db/migrate/schema.json` — canonical schema + per-dialect SQL instructions
- `shipq/db/schema/schema.go` — typed tables + typed columns used by the query DSL
- `shipq/lib/...` — embedded runtime libraries

### Compiler 2: Query Compiler (`shipq db compile`)

Input: Query definition files under `querydefs/` — Go packages whose `init()` functions register queries in a global registry using the PortSQL DSL.

Process:
1. Generates a TEMP Go program that imports your querydef packages (triggering their `init()` functions).
2. Serializes the query registry — each query's AST, return type, parameter types, and cursor columns.
3. Compiles each AST to dialect-specific SQL (Postgres, MySQL, SQLite) with correct quoting, placeholders, JSON aggregation, and ILIKE translation.
4. Generates typed query runner code with parameter structs and result types.

Output artifacts:
- `shipq/queries/types.go` — shared parameter and result types
- `shipq/queries/<dialect>/runner.go` — dialect-specific query runner with typed methods

### Compiler 3: Handler Compiler (`shipq handler compile`)

Input: API handler packages under `api/` — Go packages that register HTTP handlers with their method, path, request/response types, and auth requirements.

Process:
1. Discovers all handler registrations across `api/` packages.
2. Extracts full metadata: HTTP method, path (with path params), request/response struct definitions, auth requirements.
3. Generates server wiring, OpenAPI spec, test infrastructure, and client code.

Output artifacts:
- `cmd/server/main.go` — runnable server with all handlers wired up
- OpenAPI 3.1 JSON spec embedded into the server (`GET /openapi` in dev/test)
- API docs UI (`GET /docs` in dev/test)
- Admin UI (OpenAPI-driven, for manual testing)
- HTTP test client + harness used by generated specs and integration tests
- TypeScript HTTP client codegen (and optional framework helpers for React/Svelte)

### Compiler 4: LLM Compiler (`shipq llm compile`)

Input: LLM tool packages listed in `[llm] tool_pkgs` in `shipq.ini` — Go packages that export a `Register(app *llm.App)` function registering plain Go functions as LLM tools.

Process:
1. Reads the `[llm]` section from `shipq.ini` to discover tool packages.
2. Performs static analysis (AST walking) on each tool package to find `Register(app *llm.App)` functions and `app.Tool(...)` calls.
3. Generates a temporary compile program that imports all tool packages, calls their `Register` functions, and uses reflection to extract input/output struct metadata and JSON Schemas.
4. Builds and runs the temporary program, parsing its output as serialized tool definitions.
5. Merges runtime metadata (schemas) with static data (function names, packages) and generates all downstream artifacts.

Output artifacts:
- `tools/<pkg>/zz_generated_registry.go` — per-package `Registry()` function with typed tool dispatchers and JSON Schemas
- `shipq/lib/llmpersist/zz_generated_persister.go` — persister adapter wrapping `queries.Runner` → `llm.Persister`
- `migrations/*_llm_tables.go` — migration for `llm_conversations` + `llm_messages` tables
- `querydefs/` — querydefs for LLM persistence (insert/update/list conversations and messages)
- LLM stream message types (`LLMTextDelta`, `LLMToolCallStart`, `LLMToolCallResult`, `LLMDone`) injected as `FromServer` types on LLM-enabled channels

Key point: Provider and model selection are NOT part of the compiler — they live in the user's hand-written `Setup` function as ordinary Go code. The compiler generates everything that's genuinely mechanical (schemas, dispatchers, migrations, querydefs); the user writes the ~15 lines of Setup that wire providers to tools.

## Project Structure

A typical ShipQ project has this structure:

```
myapp/
├── shipq.ini                    # Project configuration (control plane)
├── go.mod                       # Go module file
├── .gitignore                   # Excludes .shipq/
├── .shipq/                      # Internal state (gitignored)
│   ├── data/                    # SQLite database files (when using SQLite)
│   └── compile/                 # Temporary build artifacts
├── migrations/                  # Go migration files (input to schema compiler)
│   ├── 001_create_organizations.go
│   ├── 002_create_accounts.go
│   └── ...
├── querydefs/                   # Query definition files (input to query compiler)
│   ├── pets/
│   │   └── queries.go
│   └── ...
├── api/                         # Handler packages (input to handler compiler)
│   ├── auth/
│   │   ├── login.go
│   │   ├── logout.go
│   │   ├── register.go
│   │   └── spec/                # Generated tests
│   ├── pets/
│   │   ├── create.go
│   │   ├── get_one.go
│   │   ├── list.go
│   │   ├── update.go
│   │   ├── soft_delete.go
│   │   ├── register.go
│   │   └── spec/                # Generated tests
│   └── ...
├── cmd/
│   ├── server/
│   │   └── main.go              # Generated server entry point
│   └── worker/
│       └── main.go              # Generated worker entry point (if workers enabled)
├── shipq/
│   ├── db/
│   │   ├── migrate/
│   │   │   ├── schema.json      # Canonical schema plan
│   │   │   └── runner.go        # Migration runner
│   │   └── schema/
│   │       └── schema.go        # Typed table/column bindings
│   ├── queries/
│   │   ├── types.go             # Query param/result types
│   │   └── <dialect>/
│   │       └── runner.go        # Typed query runner
│   └── lib/                     # Embedded runtime libraries
│       └── db/
│           └── portsql/
│               ├── ddl/         # DDL builder
│               ├── migrate/     # Migration framework
│               └── query/       # PortSQL query DSL
└── seeds/                       # Optional seed data files
```

## shipq.ini Configuration

`shipq.ini` lives at the project root. It is the control plane for all ShipQ behavior.

```ini
[db]
database_url = postgres://localhost:5432/myapp_dev
scope = organization_id

[auth]
protect_by_default = true

[typescript]
framework = react
http_output = .

[workers]
redis_url = redis://localhost:6379
centrifugo_url = http://localhost:8000
centrifugo_api_key = <auto-generated>
centrifugo_secret = <auto-generated>

[llm]
tool_pkgs = myapp/tools/weather, myapp/tools/calendar

[env]
STRIPE_SECRET_KEY = required
```

### Section details:
- `[db] database_url` — Connection URL. Prefix determines dialect: `postgres://` = Postgres, `mysql://` = MySQL, `sqlite://` = SQLite.
- `[db] scope` — Optional. When set (e.g., `organization_id`), auto-injects a foreign key column into every new migration and generates tenant-scoped queries/tests.
- `[auth] protect_by_default` — When `true`, generated handlers require auth unless `--public` is passed.
- `[typescript] framework` — `react`, `svelte`, or omit for plain TS.
- `[typescript] http_output` — Output directory for generated TS files.
- `[workers]` — Created by `shipq workers`. Redis + Centrifugo connection details.
- `[llm] tool_pkgs` — Comma-separated list of Go import paths for packages that export `Register(app *llm.App)` functions. Provider/model/system prompt are NOT in config — they live in the user's Setup function as Go code.
- `[env]` — Declare required environment variables validated at server startup.

## CLI Commands Reference

### Project Setup
- `shipq init` — Initialize project (creates go.mod, shipq.ini, .gitignore)
- `shipq nix` — Generate shell.nix with latest stable nixpkgs
- `shipq docker` — Generate production Dockerfiles (server + optional worker)

### Database
- `shipq db setup` — Create dev/test databases, write database_url to shipq.ini. Uses DATABASE_URL env var or auto-detects.
- `shipq db compile` — Run the query compiler: querydefs → typed query runners.
- `shipq db reset` — Drop/recreate databases, re-run all migrations (alias for `migrate reset`).

### Migrations
- `shipq migrate new <table> [columns...] [--global]` — Create a migration. Column syntax: `name:type` or `name:references:table`.
- `shipq migrate up` — Run the schema compiler: migrations → schema.json → typed bindings → apply to databases.
- `shipq migrate reset` — Drop/recreate databases, re-run all migrations from scratch.

### Authentication
- `shipq auth` — Generate full auth system (organizations, accounts, sessions tables + handlers + tests). Sets `protect_by_default = true`.
- `shipq auth google` — Add Google OAuth login endpoints. Requires GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET, GOOGLE_REDIRECT_URL env vars.
- `shipq auth github` — Add GitHub OAuth login endpoints. Requires GITHUB_CLIENT_ID, GITHUB_CLIENT_SECRET, GITHUB_REDIRECT_URL env vars.
- `shipq signup` — Generate signup handler (POST /auth/signup). Run after `shipq auth`.
- `shipq email` — Add email verification and password reset. Requires auth + workers.

### Resources & Handlers
- `shipq resource <table> <operation> [--public]` — Generate CRUD handler(s). Operations: `create`, `get_one`, `list`, `update`, `delete`, `all`. Generates querydefs + handlers + tests + runs handler compile.
- `shipq handler generate <table>` — Generate CRUD handlers without running handler compile.
- `shipq handler compile` — Run the handler compiler: discover handlers → generate server main, OpenAPI, tests, TS clients.

### File Uploads
- `shipq files` — Generate S3-compatible file upload system (managed_files table, handlers, TS helpers). Requires auth. Env vars: S3_BUCKET, S3_REGION, S3_ENDPOINT, AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY.

### Workers & Channels
- `shipq workers` — Full bootstrap: Redis/Centrifugo config, job_results migration, channel codegen, worker binary, TS clients.
- `shipq workers compile` — Fast recompile: only codegen steps, no migrations or prerequisite checks.

### LLM
- `shipq llm compile` — Compile LLM tool registrations: static analysis of tool packages, runtime metadata extraction via reflection, generates tool registries, persister adapter, migrations, querydefs, and stream types. Requires `shipq workers` first.

### Services
- `shipq start <service>` — Start a dev service. Services: `postgres`, `mysql`, `sqlite`, `redis`, `minio`, `centrifugo`, `server`, `worker`.

### Utilities
- `shipq seed` — Run all seed files in seeds/ directory.
- `shipq kill-port <port>` — Kill process on a TCP port.
- `shipq kill-defaults` — Kill all default dev-service ports.

## Column Types for `shipq migrate new`

| Type | Description |
|------|-------------|
| `string` | Short text (VARCHAR 255) |
| `text` | Long text |
| `int` | Integer |
| `bigint` | Large integer (64-bit) |
| `bool` | Boolean |
| `float` | Floating point |
| `decimal` | Fixed-precision decimal |
| `datetime` | Date and time |
| `timestamp` | Alias for datetime |
| `binary` | Binary/blob data |
| `json` | JSON data |
| `references` | Foreign key. Syntax: `column_name:references:other_table` |

Every table automatically gets: `id`, `public_id`, `created_at`, `updated_at`, `deleted_at`.

## PortSQL Query DSL and Generated Query Runner

PortSQL is ShipQ's typed SQL DSL. Queries are Go code that compiles to correct SQL for Postgres, MySQL, and SQLite.

### Registration Functions

```go
// Returns 0 or 1 row. Generated: (*Result, error)
query.MustDefineOne("GetPetById", ast)

// Returns 0..N rows. Generated: ([]Result, error)
query.MustDefineMany("ListPets", ast)

// Executes without returning rows. Generated: (sql.Result, error)
query.MustDefineExec("UpdatePetName", ast)

// Cursor-based pagination. Generated: (*Result, error) with Items + NextCursor
query.MustDefinePaginated("ListPosts", ast, cursorCol1.Desc(), cursorCol2.Desc())
```

These all panic on errors (empty name, nil AST, duplicate names). This is intentional — registration happens at `init()` time. Use `TryDefine*` variants for non-panicking alternatives.

### What `shipq db compile` Generates

For every query you define, ShipQ generates a **params struct**, a **result struct**, and a **runner method**. Example for `GetPetByPublicId`:

```go
// shipq/queries/types.go (generated)
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
```

The generated runner method (Postgres dialect):

```go
// shipq/queries/postgres/runner.go (generated)
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
    err := row.Scan(&result.PublicId, &result.Name, &result.Species, &result.Age, &result.CreatedAt, &result.UpdatedAt)
    if err == sql.ErrNoRows {
        return nil, nil // MustDefineOne: nil means "not found"
    }
    if err != nil {
        return nil, err
    }
    return &result, nil
}
```

Switching from Postgres to MySQL regenerates the runner with backtick-quoted identifiers, `?` placeholders, `LOWER(col) LIKE LOWER(?)` instead of `ILIKE`, etc. You never change your query definitions.

For paginated queries, ShipQ also generates cursor encode/decode helpers:

```go
func DecodeListPetsCursor(encoded string) *ListPetsCursor
func EncodeListPetsCursor(cursor *ListPetsCursor) string
```

The `RunnerFromContext(ctx)` pattern lets handlers get the runner without knowing the dialect — the generated `cmd/server/main.go` injects it based on the configured database.

### Building Queries

```go
// SELECT
query.From(schema.Pets).
    Select(schema.Pets.Id(), schema.Pets.Name()).
    Where(schema.Pets.Id().Eq(query.Param[int64]("id"))).
    OrderBy(schema.Pets.Name().Asc()).
    Limit(query.Param[int]("limit")).
    Build()

// JOIN
query.From(schema.Books).
    Select(schema.Books.Title(), schema.Authors.Name()).
    Join(schema.Authors).On(schema.Books.AuthorId().Eq(schema.Authors.Id())).
    Build()

// UPDATE
query.Update(schema.Pets).
    Set(schema.Pets.Name(), query.Param[string]("name")).
    Where(schema.Pets.Id().Eq(query.Param[int64]("id"))).
    Build()

// INSERT
query.InsertInto(schema.Pets).
    Columns(schema.Pets.Name(), schema.Pets.Species()).
    Values(query.Param[string]("name"), query.Param[string]("species")).
    Build()

// DELETE
query.DeleteFrom(schema.Pets).
    Where(schema.Pets.Id().Eq(query.Param[int64]("id"))).
    Build()
```

### Column Operations

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
| `.Like(expr)` | `LIKE ?` (ILIKE on Postgres) |
| `.In(exprs...)` | `IN (?, ...)` |

### Combining Conditions

```go
query.And(expr1, expr2, expr3)
query.Or(expr1, expr2)
```

### Parameters and Literals

```go
query.Param[string]("email")  // typed query parameter
query.Literal(42)              // constant embedded in SQL
```

### Additional Features

- `SelectAs(col, alias)` / `SelectExprAs(expr, alias)` — column aliases
- `SelectJSONAgg("field", cols...)` — cross-dialect JSON aggregation
- `LeftJoin`, `RightJoin`, `FullJoin` — additional join types
- `.As("alias")` on join builders — table aliases
- `Distinct()` — SELECT DISTINCT
- `GroupBy(cols...)` / `Having(expr)` — aggregation
- Set operations: `Union`, `Intersect`, `Except`
- CTEs: `With("name", ast).From("name")...`
- Subqueries: `query.Subquery(ast)` in WHERE clauses

## Handler System

Handlers register themselves with metadata via `handler.HandlerInfo`:
- HTTP method (GET, POST, PUT, PATCH, DELETE)
- Path (e.g., `/pets/:id`) with auto-extracted path params
- RequireAuth / OptionalAuth flags
- Request struct (nil for no-body handlers)
- Response struct

Every handler follows this function signature:

```go
func HandlerName(ctx context.Context, req *RequestType) (*ResponseType, error)
```

Request/response types use standard Go struct tags. Fields without `omitempty` and non-pointer fields are treated as required in OpenAPI. Struct tags: `json` for body fields, `path` for URL params (e.g., `path:"id"`), `query` for query string params (e.g., `query:"limit"`).

### Generated CRUD Handlers from `shipq resource <table> all`

| File | Method | Route |
|------|--------|-------|
| `create.go` | POST | `/<table>` |
| `get_one.go` | GET | `/<table>/:id` |
| `list.go` | GET | `/<table>` |
| `update.go` | PATCH | `/<table>/:id` |
| `soft_delete.go` | DELETE | `/<table>/:id` |
| `register.go` | — | Handler registration |

### Generated register.go example

```go
// api/pets/register.go
package pets

import "myapp/shipq/lib/handler"

func Register(app *handler.App) {
    app.Post("/pets", CreatePet).Auth()
    app.Get("/pets", ListPets).Auth()
    app.Get("/pets/:id", GetPet).Auth()
    app.Patch("/pets/:id", UpdatePet).Auth()
    app.Delete("/pets/:id", SoftDeletePet).Auth()
}
```

Each line tells ShipQ the HTTP method, path, handler function, and auth requirement. The handler compiler uses reflection on the handler function to extract request/response types for OpenAPI generation, TypeScript clients, and test harness code.

### Generated Create handler example

```go
// api/pets/create.go
package pets

import (
    "context"
    "time"

    "myapp/shipq/lib/httperror"
    "myapp/shipq/lib/httputil"
    "myapp/shipq/lib/nanoid"
    "myapp/shipq/queries"
)

type CreatePetRequest struct {
    Name    string `json:"name"`
    Species string `json:"species"`
    Age     int    `json:"age"`
}

type CreatePetResponse struct {
    PublicId  string `json:"id"`
    Name      string `json:"name"`
    Species   string `json:"species"`
    Age       int    `json:"age"`
    CreatedAt string `json:"created_at"`
    UpdatedAt string `json:"updated_at"`
}

func CreatePet(ctx context.Context, req *CreatePetRequest) (*CreatePetResponse, error) {
    runner := queries.RunnerFromContext(ctx)

    orgID, ok := httputil.OrganizationIDFromContext(ctx)
    if !ok {
        return nil, httperror.Wrap(403, "organization context missing", nil)
    }

    publicId := nanoid.New()

    _, err := runner.CreatePet(ctx, queries.CreatePetParams{
        PublicId:       publicId,
        Name:           req.Name,
        Species:        req.Species,
        Age:            req.Age,
        OrganizationId: orgID,
    })
    if err != nil {
        return nil, httperror.Wrap(500, "failed to create pet", err)
    }

    result, err := runner.GetPetByPublicId(ctx, queries.GetPetByPublicIdParams{
        PublicId:       publicId,
        OrganizationId: orgID,
    })
    if err != nil {
        return nil, httperror.Wrap(500, "failed to fetch created pet", err)
    }

    return &CreatePetResponse{
        PublicId:  result.PublicId,
        Name:      result.Name,
        Species:   result.Species,
        Age:       result.Age,
        CreatedAt: result.CreatedAt.Format(time.RFC3339),
        UpdatedAt: result.UpdatedAt.Format(time.RFC3339),
    }, nil
}
```

Key patterns:
- `queries.RunnerFromContext(ctx)` — the query runner is injected into context by the generated server wiring.
- `httputil.OrganizationIDFromContext(ctx)` — when scoped, the org ID comes from the authenticated session, never from the request body.
- `nanoid.New()` — public IDs are generated in the handler so the same ID can be used for both INSERT and re-fetch.
- The handler re-fetches after create to get resolved JOINs (e.g., FK references as public IDs).
- Error handling uses `httperror.Wrap(statusCode, message, err)` which the generated server wiring converts to proper HTTP responses.

### Generated Get handler example

```go
// api/pets/get_one.go
package pets

type GetPetRequest struct {
    ID string `path:"id"` // public ID extracted from /pets/:id
}

func GetPet(ctx context.Context, req *GetPetRequest) (*GetPetResponse, error) {
    runner := queries.RunnerFromContext(ctx)

    orgID, ok := httputil.OrganizationIDFromContext(ctx)
    if !ok {
        return nil, httperror.Wrap(403, "organization context missing", nil)
    }

    result, err := runner.GetPetByPublicId(ctx, queries.GetPetByPublicIdParams{
        PublicId:       req.ID,
        OrganizationId: orgID,
    })
    if err != nil {
        return nil, httperror.Wrap(500, "failed to fetch pet", err)
    }
    if result == nil {
        return nil, httperror.NotFoundf("pet %q not found", req.ID)
    }

    // ... map result to response ...
    return resp, nil
}
```

Key patterns:
- `path:"id"` struct tag extracts the path parameter from `/pets/:id`.
- `MustDefineOne` queries return `nil` when no row matches — the handler converts that to a 404.
- Scope is always checked: a user in Org A can never fetch Org B's data by guessing a public ID.

### Generated List handler example (with cursor pagination)

```go
// api/pets/list.go
package pets

type ListPetsRequest struct {
    Limit  int     `query:"limit"`  // from ?limit=20
    Cursor *string `query:"cursor"` // from ?cursor=base64...
}

type ListPetsResponse struct {
    Items      []PetItem `json:"items"`
    NextCursor *string   `json:"next_cursor,omitempty"`
}

func ListPets(ctx context.Context, req *ListPetsRequest) (*ListPetsResponse, error) {
    runner := queries.RunnerFromContext(ctx)

    orgID, ok := httputil.OrganizationIDFromContext(ctx)
    if !ok {
        return nil, httperror.Wrap(403, "organization context missing", nil)
    }

    limit := req.Limit
    if limit <= 0 || limit > 100 {
        limit = 20
    }

    var cursor *queries.ListPetsCursor
    if req.Cursor != nil {
        cursor = queries.DecodeListPetsCursor(*req.Cursor)
    }

    result, err := runner.ListPets(ctx, queries.ListPetsParams{
        OrganizationId: orgID,
        Limit:          limit,
        Cursor:         cursor,
    })
    if err != nil {
        return nil, httperror.Wrap(500, "failed to list pets", err)
    }

    items := make([]PetItem, len(result.Items))
    for i, item := range result.Items {
        items[i] = PetItem{ /* map fields */ }
    }

    var nextCursor *string
    if result.NextCursor != nil {
        encoded := queries.EncodeListPetsCursor(result.NextCursor)
        nextCursor = &encoded
    }

    return &ListPetsResponse{Items: items, NextCursor: nextCursor}, nil
}
```

Key patterns:
- `query:"limit"` and `query:"cursor"` tags extract query string parameters.
- Cursor-based pagination is automatic for tables with `created_at` + `public_id`.
- The cursor is an opaque base64 string. Internally uses `WHERE (created_at, public_id) < (?, ?)` for efficient seeking.

### How the full HTTP flow works

1. HTTP request arrives → generated `cmd/server/main.go` routes it to the handler
2. Auth middleware checks the session cookie, extracts account ID and org ID, injects into context
3. JSON deserialization — the server reads the request body into the typed request struct
4. Handler runs — calls `queries.RunnerFromContext(ctx)` then `runner.SomeQuery(ctx, params)`
5. Query runner executes dialect-specific SQL and scans into typed result structs
6. Handler returns `(*Response, nil)` → serialized to JSON with correct HTTP status
7. If handler returns `(nil, error)` → `httperror.Wrap` errors set the status code; unexpected errors become 500s

### File Ownership Rules

- Files with `// Code generated by shipq. DO NOT EDIT.` header or `zz_generated_` prefix are overwritten on every compile. NEVER hand-edit these.
- Handler files like `create.go`, `get_one.go`, etc. are yours to customize.
- `cmd/server/main.go` is always regenerated.

## Multi-Tenancy (Scope System)

When `[db] scope = organization_id` is set in shipq.ini:

1. `shipq migrate new` auto-injects `organization_id:references:organizations` (skip with `--global`).
2. Generated queries include `WHERE organization_id = ?` automatically.
3. Generated handlers extract `organization_id` from authenticated user context.
4. Generated tests include tenancy isolation tests (User A in Org A cannot see Org B's data).

The filtering is at the SQL level — impossible to bypass accidentally.

## Authentication System

`shipq auth` generates:
- Migrations: `organizations`, `accounts`, `sessions` tables
- Handlers: `/auth/login`, `/auth/logout`, `/auth/me`
- Cookie-based session management (signed cookies via `COOKIE_SECRET`) + auth middleware
- Tests in `api/auth/spec/`
- Sets `[auth] protect_by_default = true`

`shipq signup` adds `POST /auth/signup`.

OAuth: `shipq auth google` / `shipq auth github` add OAuth login + account linking.

`shipq email` adds email verification + password reset (requires workers).

## File Uploads

`shipq files` generates:
- Migrations: `managed_files`, `file_access` tables
- Handlers: upload, download, delete in `api/managed_files/`
- TS helpers: `shipq-files.ts`
- Tests

Requires auth. Env vars: S3_BUCKET, S3_REGION, S3_ENDPOINT (empty for AWS, set for MinIO/R2/GCS), AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY.

Local dev: `shipq start minio` starts a local MinIO server.

## Workers & Channels

`shipq workers` bootstraps background jobs (Redis + Machinery) and real-time WebSockets (Centrifugo).

Architecture: Server dispatches jobs to Redis → Worker process picks them up → optionally publishes real-time notifications to Centrifugo → Browser clients receive via WebSocket.

Generated: `cmd/worker/main.go`, `centrifugo.json`, `shipq-channels.ts`, typed channel code, React/Svelte hooks.

Running: `shipq start redis`, `shipq start centrifugo`, `shipq start worker`, `shipq start server` (each in separate terminals).

Fast recompile after channel changes: `shipq workers compile`.

### Defining a Channel

Create a package under `channels/` with message types and a `Register` function:

```go
// channels/chatbot/register.go
package chatbot

import "myapp/shipq/lib/channel"

// Client-to-server messages
type StartChat struct {
    Prompt string `json:"prompt"`
}

type ToolCallApproval struct {
    CallId   string `json:"call_id"`
    Approved bool   `json:"approved"`
}

// Server-to-client messages
type BotMessage struct {
    Text string `json:"text"`
}

type StreamingToken struct {
    Token string `json:"token"`
}

type ChatFinished struct {
    Summary string `json:"summary"`
}

func Register(app *channel.App) {
    app.DefineChannel("chatbot",
        // First type in FromClient is the dispatch (trigger) message; rest are mid-stream
        channel.FromClient(StartChat{}, ToolCallApproval{}),
        channel.FromServer(BotMessage{}, StreamingToken{}, ChatFinished{}),
    ).Retries(3).BackoffSeconds(5).TimeoutSeconds(120)
}
```

Key: first type in `FromClient(...)` is the dispatch message (triggers the job). Subsequent types are mid-stream (sent while the worker is running). `FromServer(...)` types are pushed from worker to browser.

### Writing the Channel Handler

The handler runs in the **worker process**. Signature: `func(context.Context, *DispatchType) error`. Name must be `Handle` + dispatch type name:

```go
// channels/chatbot/handler.go
package chatbot

import (
    "context"
    "fmt"
)

func HandleStartChat(ctx context.Context, req *StartChat) error {
    ch := TypedChannelFromContext(ctx) // generated typed wrapper

    // Stream tokens to the browser
    for _, token := range streamLLM(req.Prompt) {
        if err := ch.SendStreamingToken(ctx, &StreamingToken{Token: token}); err != nil {
            return fmt.Errorf("send token: %w", err)
        }
    }

    // Block until the client sends a ToolCallApproval
    approval, err := ch.ReceiveToolCallApproval(ctx)
    if err != nil {
        return fmt.Errorf("receive approval: %w", err)
    }

    if approval.Approved {
        result := executeTool()
        ch.SendBotMessage(ctx, &BotMessage{Text: result})
    }

    return ch.SendChatFinished(ctx, &ChatFinished{Summary: "Done"})
}
```

Key patterns:
- `TypedChannelFromContext(ctx)` is **generated** — gives you `Send<Type>` and `Receive<Type>` methods.
- `ch.ReceiveToolCallApproval(ctx)` blocks until the client publishes that message type via Centrifugo.
- Returning a non-nil error marks the job as "failed" in `job_results` and triggers retry.

### Generated TypeScript Client

`shipq workers compile` generates `shipq-channels.ts`:

```typescript
export interface ChatbotChannel {
  jobId: string;
  onBotMessage(handler: (msg: BotMessage) => void): void;
  onStreamingToken(handler: (msg: StreamingToken) => void): void;
  onChatFinished(handler: (msg: ChatFinished) => void): void;
  sendToolCallApproval(msg: ToolCallApproval): void;
  unsubscribe(): void;
}

export async function dispatchChatbot(request: StartChat): Promise<ChatbotChannel> {
  // 1. POST /channels/chatbot/dispatch → enqueues job, gets job_id
  // 2. GET /channels/chatbot/token?job_id=... → gets JWT tokens
  // 3. Connects to Centrifugo WebSocket with auto token refresh
  // 4. Returns typed channel interface with on<Type> and send<Type> methods
}
```

The client handles: HTTP dispatch, JWT token fetching/refresh, Centrifugo WebSocket connection, publication echo filtering (ignores own `FromClient` messages echoed back), and type-safe message demultiplexing.

### Generated React Hook

When `[typescript] framework = react`:

```typescript
export function useChatbot(options?: useChatbotOptions): useChatbotReturn {
  // Returns: { channel, isConnecting, error, dispatch, sendToolCallApproval, disconnect }
  // Handles cleanup on unmount, stale closure prevention via refs
}

// Usage in a component:
const { dispatch, sendToolCallApproval, isConnecting } = useChatbot({
    onStreamingToken(msg) { setStreaming(prev => prev + msg.token); },
    onBotMessage(msg) { setMessages(prev => [...prev, msg.text]); },
    onChatFinished(msg) { console.log("Done:", msg.summary); },
});

dispatch({ prompt: "Hello" }); // kicks off the whole flow
```

### Public Channels

For unauthenticated channels, use `.Public()` with rate limiting:

```go
app.DefineChannel("assistant",
    channel.FromClient(AssistantQuery{}),
    channel.FromServer(AssistantAnswer{}),
).Public(channel.RateLimitConfig{RequestsPerMinute: 10, BurstSize: 3})
```

### Setup Functions

If your handler needs external deps, export a `Setup(ctx context.Context) context.Context` function. ShipQ detects it via static analysis and calls it before each handler invocation:

```go
func Setup(ctx context.Context) context.Context {
    return context.WithValue(ctx, depsKey{}, &Deps{APIClient: NewClient(config.Settings.API_KEY)})
}
```

Generated worker code: `queue.RegisterTask("chatbot", channel.WrapDispatchHandler(chatbot.HandleStartChat, transport, db, "chatbot", channel.WithSetup(chatbot.Setup)))`.

### Full Sequence (what happens when the user clicks "Send")

1. React calls `dispatch({ prompt: "Hello" })`
2. Generated TS client POSTs to `POST /channels/chatbot/dispatch`
3. Server validates auth, creates `job_results` row, enqueues to Redis
4. Server responds with `{ "job_id": "abc123" }`
5. TS client fetches JWT from `GET /channels/chatbot/token?job_id=abc123`
6. TS client connects to Centrifugo WebSocket
7. Worker dequeues job, subscribes to same Centrifugo channel, calls `HandleStartChat`
8. Handler calls `ch.SendStreamingToken(...)` → Centrifugo publishes → browser receives
9. Handler calls `ch.ReceiveToolCallApproval(ctx)` → blocks until user responds
10. User clicks Approve → React calls `sendToolCallApproval(...)` → Centrifugo → worker receives
11. Handler completes → worker updates `job_results` to "completed"

## Recommended Workflow

```sh
# Create project
mkdir myapp && cd myapp
shipq init
shipq db setup

# Add auth
shipq auth
shipq signup
go mod tidy

# Configure scope (optional)
# Edit shipq.ini: [db] scope = organization_id

# Create tables
shipq migrate new pets name:string species:string age:int
shipq migrate up

# Generate CRUD
shipq resource pets all
go mod tidy

# Test and run
go test ./... -v
go run ./cmd/server
```

### Iteration commands

| What changed | Command |
|---|---|
| Migration files | `shipq migrate up` |
| Query definitions in querydefs/ | `shipq db compile` |
| Handlers or routes | `shipq handler compile` |
| New table + CRUD | `shipq migrate new ... && shipq resource ... all` |
| Channel definitions | `shipq workers compile` |
| LLM tool functions | `shipq llm compile` |

## Key Design Decisions

1. **Compile-time over runtime**: Schema, queries, and handler registries are build targets — errors surface immediately, not in production.
2. **SQL-shaped data access**: PortSQL lets you write SQL-shaped Go code, not ActiveRecord-style object graphs. Query boundaries are explicit.
3. **Multi-database by construction**: Same DSL → Postgres/MySQL/SQLite. Dialect differences handled at compile time.
4. **Self-contained output**: Generated projects embed all runtime code and have zero ShipQ dependency.
5. **Generated tests are real tests**: Auth, CRUD, 401 rejection, RBAC, and tenancy isolation tests are generated and expected to pass.
6. **Scope = multi-tenancy built in**: Not bolted on with middleware — baked into SQL WHERE clauses by the compiler.

## LLM Tool Calling

ShipQ includes an LLM integration layer that lets you expose plain Go functions as LLM tools, wire them to OpenAI or Anthropic (or both), and get automatic streaming over Centrifugo and automatic persistence to your database — all through the existing channel/worker infrastructure.

### Defining a Tool

A tool is a plain Go function with a struct input and a struct output:

```go
// tools/weather/weather.go
package weather

import "context"

type WeatherInput struct {
    City    string `json:"city"    desc:"The city to get weather for"`
    Country string `json:"country" desc:"ISO country code, e.g. US"`
}

type WeatherOutput struct {
    TempC       float64 `json:"temp_c"`
    Description string  `json:"description"`
}

func GetWeather(ctx context.Context, input *WeatherInput) (*WeatherOutput, error) {
    return &WeatherOutput{TempC: 22.5, Description: "Partly cloudy"}, nil
}
```

Struct tags: `json` sets the property name in JSON Schema. `desc` sets the description (helps the model understand the parameter). Tool function signatures must be `func(context.Context, *InputStruct) (*OutputStruct, error)` or `func(*InputStruct) (*OutputStruct, error)`.

### Registering Tools

Each tool package exports a `Register` function:

```go
// tools/weather/register.go
package weather

import "myapp/shipq/lib/llm"

func Register(app *llm.App) {
    app.Tool("get_weather", "Get the current weather for a city", GetWeather)
}
```

### Configuration

Add tool packages to `shipq.ini`:

```ini
[llm]
tool_pkgs = myapp/tools/weather, myapp/tools/calendar
```

Then compile: `shipq llm compile` followed by `shipq migrate up` (first time only, for the LLM tables).

### Writing the Setup Function

In your channel package, write a `Setup` function that wires provider, tools, and persistence. This is user-written Go code (not generated), giving full control over provider choice:

```go
// channels/chatbot/setup.go
package chatbot

import (
    "context"
    "os"
    "myapp/shipq/lib/channel"
    "myapp/shipq/lib/llm"
    "myapp/shipq/lib/llm/anthropic"
    "myapp/shipq/lib/llmpersist"
    "myapp/shipq/lib/db/dbrunner"
    "myapp/tools/weather"
)

func Setup(ctx context.Context) context.Context {
    ch := channel.FromContext(ctx)
    db := channel.DBFromContext(ctx)
    persister := llmpersist.New(dbrunner.NewQueryRunner(db))

    client := llm.NewClient(
        anthropic.New(os.Getenv("ANTHROPIC_API_KEY"), "claude-sonnet-4-20250514"),
        llm.WithTools(weather.Registry()),
        llm.WithChannel(ch),
        llm.WithPersister(persister),
        llm.WithSystem("You are a helpful weather assistant."),
    )
    return llm.WithClient(ctx, client)
}
```

### Writing the Channel Handler

```go
func HandleChatRequest(ctx context.Context, req *ChatRequest) error {
    client := llm.ClientFromContext(ctx)
    resp, err := client.Chat(ctx, req.Message)
    if err != nil {
        return err
    }
    // By this point, every token was streamed to the frontend,
    // and every message was persisted to the database.
    _ = resp
    return nil
}
```

### Client Options

- `WithTools(r)` — tool registry for function calling
- `WithChannel(ch)` — enables real-time streaming over Centrifugo
- `WithPersister(p)` — enables database persistence
- `WithSystem(prompt)` — system prompt
- `WithMaxIterations(n)` — max tool-calling round-trips (default: 10)
- `WithMaxTokens(n)` — max output tokens per provider call
- `WithTemperature(t)` — sampling temperature
- `WithWebSearch(cfg)` — enable web search (provider-dependent)
- `WithErrorStrategy(s)` — `SendErrorToModel` (default, recoverable) or `AbortOnToolError`
- `WithSequentialToolCalls()` — execute parallel tool calls sequentially

### Multiple Providers

A single channel can use multiple providers. Use `llm.WithNamedClient` for additional clients:

```go
ctx = llm.WithClient(ctx, llm.NewClient(
    anthropic.New(os.Getenv("ANTHROPIC_API_KEY"), "claude-sonnet-4-20250514"), ...))
ctx = llm.WithNamedClient(ctx, "summary", llm.NewClient(
    openai.New(os.Getenv("OPENAI_API_KEY"), "gpt-4.1"), ...))
```

Retrieve named clients with `llm.NamedClientFromContext(ctx, "summary")`.

### Streaming

The client publishes these envelope types over Centrifugo (same format as all ShipQ channels):
- `LLMTextDelta` — `{ "text": "..." }` — streamed text chunk
- `LLMToolCallStart` — `{ "tool_call_id": "...", "tool_name": "...", "input": {...} }` — tool invocation
- `LLMToolCallResult` — `{ "tool_call_id": "...", "tool_name": "...", "output": {...}, "duration_ms": 123 }` — tool result
- `LLMDone` — `{ "text": "...", "input_tokens": 100, "output_tokens": 50, "tool_call_count": 2 }` — conversation complete

### Database Persistence

`shipq llm compile` generates two tables:
- `llm_conversations` — one row per `client.Chat()` call. Links to `job_results` via `job_id`. Tracks provider, model, system prompt, token usage, status.
- `llm_messages` — one row per logical message. Roles: `user`, `assistant`, `tool_call`, `tool_result`. Full audit trail with tool inputs/outputs, duration, and token counts.

### Providers

Built-in: `openai.New(apiKey, model)` and `anthropic.New(apiKey, model)`. Both support tool calling, vision (images), and SSE streaming. Anthropic also supports web search. Custom providers implement the `llm.Provider` interface.

### Testing

The `llm/llmtest` package provides mock providers (scripted responses, no API keys needed), recording providers (wrap real providers for snapshot testing), and assertion helpers (`AssertToolCalled`, `AssertResponseContains`, etc.).

### Environment Variables

- `OPENAI_API_KEY` — required if using the OpenAI provider
- `ANTHROPIC_API_KEY` — required if using the Anthropic provider

These are read by the user's Setup function, not by the framework.

## Deployment

`shipq docker` generates multi-stage Dockerfiles. ShipQ is a dev-time tool only — CI/CD only needs Go and Docker.

Key production env vars: DATABASE_URL, COOKIE_SECRET, plus any declared in `[env]`.

Dev-only features disabled in production: `GET /openapi`, `GET /docs`, admin UI, verbose errors.
