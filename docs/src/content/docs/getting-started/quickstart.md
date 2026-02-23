---
title: Quickstart
description: Build a complete API with auth, CRUD endpoints, and tests in under 5 minutes.
---

This guide walks you through building a fully functional API from scratch using ShipQ. By the end, you'll have authentication, CRUD endpoints for a `pets` resource, generated tests, an OpenAPI spec, and a running server.

## Prerequisites

Make sure you have:

- **Go 1.21+** installed
- **ShipQ CLI** built ([see Installation](/getting-started/installation/))
- A database engine available (Postgres, MySQL, or none — ShipQ will fall back to SQLite)

## 1. Create a new project

```sh
mkdir myapp && cd myapp
shipq init
```

`shipq init` creates:
- `go.mod` (if one doesn't exist)
- `shipq.ini` — the project configuration file
- Updates `.gitignore` to exclude `.shipq/` working directories

## 2. Set up your database

```sh
shipq db setup
```

This command:
- Uses `DATABASE_URL` from your environment if set
- Otherwise auto-detects: MySQL if `mysqld` is on PATH, Postgres if `postgres` is on PATH, otherwise SQLite
- Creates both **dev** and **test** databases
- Writes the resolved `database_url` into `shipq.ini`

:::tip
For Postgres or MySQL, set `DATABASE_URL` before running setup:
```sh
export DATABASE_URL="postgres://localhost:5432/myapp?sslmode=disable"
shipq db setup
```
For SQLite, just run `shipq db setup` with no environment variable — the database file is stored under `.shipq/data/`.
:::

## 3. Add authentication

```sh
shipq auth
shipq signup
```

`shipq auth` generates:
- Database migrations for `organizations`, `accounts`, and `sessions` tables
- Login/logout/me handlers in `api/auth/`
- Cookie-based session management (signed cookies via `COOKIE_SECRET`)
- Auth middleware
- Generated tests in `api/auth/spec/`

`shipq signup` adds a registration endpoint on top of the auth system.

Run a quick sanity check:

```sh
go mod tidy
go test ./api/auth/spec/... -v -count=1
```

## 4. Define your schema

Create a migration for a `pets` table:

```sh
shipq migrate new pets name:string species:string age:int
```

This generates a Go migration file in `migrations/` using ShipQ's typed DDL builder. The column grammar supports types like `string`, `text`, `int`, `decimal`, `datetime`, `bool`, and foreign keys via `references`.

Apply the migration:

```sh
shipq migrate up
```

This runs the schema compiler:
1. Discovers all migration files in `migrations/`
2. Executes them to build a canonical `MigrationPlan`
3. Writes `shipq/db/migrate/schema.json`
4. Generates typed schema bindings in `shipq/db/schema/schema.go`
5. Applies the schema to both dev and test databases

## 5. Generate CRUD endpoints

```sh
shipq resource pets all
```

This generates a complete set of CRUD handlers for the `pets` table:

| Operation | Method | Route | Handler |
|-----------|--------|-------|---------|
| Create | `POST` | `/pets` | `api/pets/create.go` |
| Get One | `GET` | `/pets/:id` | `api/pets/get_one.go` |
| List | `GET` | `/pets` | `api/pets/list.go` |
| Update | `PATCH` | `/pets/:id` | `api/pets/update.go` |
| Delete | `DELETE` | `/pets/:id` | `api/pets/soft_delete.go` |

It also generates:
- `api/pets/register.go` — handler registration
- Query definitions in `querydefs/`
- Test files in `api/pets/spec/`

Since we ran `shipq auth` first, all routes are **auth-protected by default**. To make public routes instead, use the `--public` flag:

```sh
shipq resource pets all --public
```

Tidy up dependencies:

```sh
go mod tidy
```

## 6. Compile the handler registry

```sh
shipq handler compile
```

This is the final compilation step. It:
- Discovers all registered handlers across your `api/` packages
- Generates `cmd/server/main.go` (the runnable server)
- Generates an OpenAPI 3.1 JSON spec (served at `GET /openapi` in dev/test)
- Generates a docs UI (served at `GET /docs` in dev/test)
- Generates an admin UI
- Generates an HTTP test client and test harness
- Generates TypeScript HTTP client code (and optional framework helpers)

## 7. Run tests

```sh
go test ./... -v
```

ShipQ generates comprehensive tests including:
- Auth endpoint tests (login, logout, session management)
- CRUD endpoint tests (create, read, update, delete)
- 401 tests for auth-protected routes (when auth is enabled)
- Tenancy isolation tests (when scoping is configured)
- RBAC tests (when roles are configured)

## 8. Start the server

```sh
go run ./cmd/server
# or
shipq start server
```

Your API is now running. If you're in dev/test mode, visit:
- `GET /docs` — Interactive API documentation
- `GET /openapi` — Raw OpenAPI 3.1 JSON spec

## What you've built

In just a few commands, you now have:

- ✅ Cookie-based authentication (login, logout, signup, session management)
- ✅ Full CRUD for `pets` with typed query runners
- ✅ Auto-generated tests that actually pass
- ✅ OpenAPI spec + docs UI + admin UI
- ✅ TypeScript HTTP client ready for your frontend
- ✅ A self-contained Go project with no external ShipQ dependency

## Iteration workflow

As you develop your app, the workflow is:

```sh
# Schema changed? Re-run migrations
shipq migrate up

# Query definitions changed? Recompile queries
shipq db compile

# Handlers or routes changed? Recompile the handler registry
shipq handler compile
```

## Next steps

- **[Core Concepts: The Compiler Chain](/concepts/compiler-chain/)** — Understand how ShipQ's three compilers feed each other
- **[Guide: Queries (PortSQL)](/guides/queries/)** — Write custom queries beyond generated CRUD
- **[Guide: Multi-Tenancy](/guides/multi-tenancy/)** — Add organization-scoped data isolation
- **[Guide: Workers & Channels](/guides/workers/)** — Add background jobs and real-time WebSocket channels
- **[E2E Example](/e2e-example/)** — A longer walkthrough building a complete application
