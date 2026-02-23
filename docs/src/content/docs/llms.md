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

## The Three-Compiler Chain

ShipQ is best understood as three compilers that feed each other:

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

## PortSQL Query DSL

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

Request/response types use standard Go struct tags. Fields without `omitempty` and non-pointer fields are treated as required in OpenAPI.

### Generated CRUD Handlers from `shipq resource <table> all`

| File | Method | Route |
|------|--------|-------|
| `create.go` | POST | `/<table>` |
| `get_one.go` | GET | `/<table>/:id` |
| `list.go` | GET | `/<table>` |
| `update.go` | PATCH | `/<table>/:id` |
| `soft_delete.go` | DELETE | `/<table>/:id` |
| `register.go` | — | Handler registration |

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

Generated: `cmd/worker/main.go`, `centrifugo.json`, `shipq-channels.ts`, typed channel code.

Running: `shipq start redis`, `shipq start centrifugo`, `shipq start worker`, `shipq start server` (each in separate terminals).

Fast recompile after channel changes: `shipq workers compile`.

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

## Key Design Decisions

1. **Compile-time over runtime**: Schema, queries, and handler registries are build targets — errors surface immediately, not in production.
2. **SQL-shaped data access**: PortSQL lets you write SQL-shaped Go code, not ActiveRecord-style object graphs. Query boundaries are explicit.
3. **Multi-database by construction**: Same DSL → Postgres/MySQL/SQLite. Dialect differences handled at compile time.
4. **Self-contained output**: Generated projects embed all runtime code and have zero ShipQ dependency.
5. **Generated tests are real tests**: Auth, CRUD, 401 rejection, RBAC, and tenancy isolation tests are generated and expected to pass.
6. **Scope = multi-tenancy built in**: Not bolted on with middleware — baked into SQL WHERE clauses by the compiler.

## Deployment

`shipq docker` generates multi-stage Dockerfiles. ShipQ is a dev-time tool only — CI/CD only needs Go and Docker.

Key production env vars: DATABASE_URL, COOKIE_SECRET, plus any declared in `[env]`.

Dev-only features disabled in production: `GET /openapi`, `GET /docs`, admin UI, verbose errors.
