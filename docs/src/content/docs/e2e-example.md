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
