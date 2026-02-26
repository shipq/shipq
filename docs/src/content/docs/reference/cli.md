---
title: CLI Commands
description: Complete reference for all ShipQ CLI commands and their options.
---

This is the full reference for every command available in the `shipq` CLI.

## Project Setup

### `shipq init`

Initialize a new ShipQ project in the current directory.

```sh
shipq init
```

**What it does:**
- Creates `go.mod` if one doesn't exist
- Creates `shipq.ini` with default `[db]` and `[typescript]` sections
- Updates `.gitignore` to exclude `.shipq/`

---

### `shipq nix`

Generate a `shell.nix` file pinned to the latest stable nixpkgs.

```sh
shipq nix
```

Provides a reproducible development environment with all required tooling via Nix.

---

### `shipq docker`

Generate production Dockerfiles for your application.

```sh
shipq docker
```

**What it generates:**
- `Dockerfile` (or `Dockerfile.server`) — multi-stage build for the HTTP server
- `Dockerfile.worker` — multi-stage build for the background worker (if workers are configured)

---

## Database

### `shipq db setup`

Set up the database and write the connection URL into `shipq.ini`.

```sh
shipq db setup
```

**Behavior:**
- Uses `DATABASE_URL` from your environment if set
- Otherwise auto-detects: MySQL if `mysqld` is on PATH, Postgres if `postgres` is on PATH, otherwise SQLite
- Creates both dev and test databases
- Writes `[db] database_url` into `shipq.ini`

**Safety guard:** For Postgres and MySQL, `DATABASE_URL` must point to `localhost`.

---

### `shipq db compile`

Generate type-safe query runner code from user-defined query definitions.

```sh
shipq db compile
```

**What it does:**
1. Generates a temporary Go program that imports your `querydefs/` packages
2. Serializes the query registry (each query's AST, return type, parameters)
3. Compiles each AST to dialect-specific SQL (Postgres, MySQL, SQLite)
4. Generates typed query runner code in `shipq/queries/`

**Output:**
- `shipq/queries/types.go` — shared parameter and result types
- `shipq/queries/<dialect>/runner.go` — dialect-specific query runner

---

### `shipq db reset`

Drop and recreate dev and test databases, then re-run all migrations.

```sh
shipq db reset
```

This is an alias for `shipq migrate reset`.

---

## Migrations

### `shipq migrate new`

Create a new migration file.

```sh
shipq migrate new <table_name> [columns...] [--global]
```

**Column syntax:** `name:type` or `name:references:table`

**Supported types:**

| Type | Description |
|------|-------------|
| `string` | Short text (VARCHAR) |
| `text` | Long text (TEXT) |
| `int` | Integer |
| `bigint` | Large integer |
| `bool` | Boolean |
| `float` | Floating point |
| `decimal` | Fixed-precision decimal |
| `datetime` | Date and time |
| `timestamp` | Alias for datetime |
| `binary` | Binary data |
| `json` | JSON data |

**References:** `column_name:references:other_table` creates a foreign key.

**Flags:**

| Flag | Description |
|------|-------------|
| `--global` | Skip scope column injection (when `[db] scope` is configured) |

**Auto-injected columns:** Every table automatically gets `id`, `public_id`, `created_at`, `updated_at`, and `deleted_at`.

**Scope injection:** If `[db] scope = organization_id` is set in `shipq.ini`, the scope column is auto-injected unless `--global` is passed.

**Examples:**

```sh
shipq migrate new users name:string email:string
shipq migrate new posts title:string body:text published:bool
shipq migrate new comments body:text post_id:references:posts
shipq migrate new countries name:string code:string --global
```

---

### `shipq migrate up`

Run all pending migrations (triggers the schema compiler).

```sh
shipq migrate up
```

**What it does:**
1. Embeds ShipQ runtime libraries into `shipq/lib/...` and rewrites imports
2. Discovers all `migrations/*.go` files
3. Generates a temporary Go program to execute migrations and build a canonical `MigrationPlan`
4. Writes the plan to `shipq/db/migrate/schema.json`
5. Generates typed schema bindings in `shipq/db/schema/schema.go`
6. Applies the plan against both dev and test databases

---

### `shipq migrate reset`

Drop and recreate dev and test databases, then re-run all migrations from scratch.

```sh
shipq migrate reset
```

---

## Authentication

### `shipq auth`

Generate the base authentication system.

```sh
shipq auth
```

**What it generates:**
- Migrations for `organizations`, `accounts`, and `sessions` tables
- Login, logout, and session management handlers in `api/auth/`
- Cookie-based session management (signed cookies via `COOKIE_SECRET`) and auth middleware
- Tests in `api/auth/spec/`
- Sets `[auth] protect_by_default = true` in `shipq.ini`

---

### `shipq auth google`

Add Google OAuth login to an existing auth system.

```sh
shipq auth google
```

Generates OAuth endpoints at `/auth/google` and `/auth/google/callback`.

**Required environment variables:** `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `GOOGLE_REDIRECT_URL`

---

### `shipq auth github`

Add GitHub OAuth login to an existing auth system.

```sh
shipq auth github
```

Generates OAuth endpoints at `/auth/github` and `/auth/github/callback`.

**Required environment variables:** `GITHUB_CLIENT_ID`, `GITHUB_CLIENT_SECRET`, `GITHUB_REDIRECT_URL`

---

### `shipq signup`

Generate a signup/registration handler. Must be run after `shipq auth`.

```sh
shipq signup
```

Generates a `POST /auth/signup` handler that creates an organization and account in a single transaction.

---

### `shipq email`

Add email verification and password reset flows. Requires `shipq auth` and `shipq workers` to have been run first.

```sh
shipq email
```

Generates email verification and password reset handlers that send emails asynchronously via the worker queue.

---

## Resources & Handlers

### `shipq resource`

Generate CRUD handler(s) for a database table.

```sh
shipq resource <table> <operation> [--public]
```

**Operations:**

| Operation | Method | Route | Description |
|-----------|--------|-------|-------------|
| `create` | `POST` | `/<table>` | Create handler + test |
| `get_one` | `GET` | `/<table>/:id` | Get-one handler + test |
| `list` | `GET` | `/<table>` | List handler + test (with pagination) |
| `update` | `PATCH` | `/<table>/:id` | Update handler + test |
| `delete` | `DELETE` | `/<table>/:id` | Soft-delete handler + test |
| `all` | All of the above | All of the above | All 5 CRUD handlers + tests + `register.go` |

**Flags:**

| Flag | Description |
|------|-------------|
| `--public` | Skip auth protection for generated routes |

**What it generates:**
- Handler files in `api/<table>/`
- Query definitions in `querydefs/`
- Test files in `api/<table>/spec/`
- Runs `shipq handler compile` automatically

**Examples:**

```sh
shipq resource pets all
shipq resource pets create
shipq resource books all --public
```

---

### `shipq handler generate`

Generate CRUD handlers for a table (without running `handler compile`).

```sh
shipq handler generate <table>
```

Generates handler files in `api/<table>/` including:
- `create.go` — POST handler
- `get_one.go` — GET handler
- `list.go` — GET (list) handler
- `update.go` — PATCH handler
- `soft_delete.go` — DELETE handler
- `register.go` — handler registration function

---

### `shipq handler compile`

Compile the handler registry and run all code generation.

```sh
shipq handler compile
```

**What it does:**
1. Discovers all handler registrations across `api/` packages
2. Extracts full metadata (HTTP method, path, request/response types, auth requirements)
3. Generates all downstream artifacts

**Output:**
- `cmd/server/main.go` — runnable HTTP server
- OpenAPI 3.1 JSON spec (served at `GET /openapi` in dev/test)
- Docs UI (served at `GET /docs` in dev/test)
- Admin UI (OpenAPI-driven)
- HTTP test client and harness
- Integration tests (RBAC, tenancy, 401 tests)
- TypeScript HTTP client (and optional framework helpers)

---

## File Uploads

### `shipq files`

Generate the S3-compatible file upload system. Requires `shipq auth` to have been run first.

```sh
shipq files
```

**What it generates:**
- Migrations for `managed_files` and `file_access` tables
- Upload/download/delete handlers in `api/managed_files/`
- Query definitions for file operations
- TypeScript helpers (`shipq-files.ts`)
- Tests for file endpoints

**Required environment variables:** `S3_BUCKET`, `S3_REGION`, `S3_ENDPOINT`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`

---

## Workers & Channels

### `shipq workers`

Bootstrap the full workers/channels system. Requires `shipq auth`, `redis-server`, and `centrifugo` on PATH.

```sh
shipq workers
```

**What it does:**
1. Verifies prerequisites
2. Writes `[workers]` section into `shipq.ini`
3. Generates and applies a `job_results` migration
4. Compiles querydefs and handler registry
5. Generates typed channel Go code
6. Generates `cmd/worker/main.go`
7. Generates `centrifugo.json`
8. Generates TypeScript channel clients (`shipq-channels.ts`)
9. Generates tests

---

### `shipq workers compile`

Recompile channel codegen without the full bootstrap.

```sh
shipq workers compile
```

Performs only codegen steps: channel discovery, typed channels, worker main, Centrifugo config, TypeScript client, querydefs compilation, and handler registry compilation. Skips migrations, prerequisite checks, and library embedding.

---

## LLM

### `shipq llm compile`

Compile LLM tool registrations and generate all LLM artifacts.

```sh
shipq llm compile
```

**Prerequisites:** `shipq workers` must have been run first (LLM tools build on the channel/worker infrastructure).

**What it does:**
1. Reads the `[llm]` section from `shipq.ini` to discover tool packages
2. Performs static analysis on each tool package to find `Register(app *llm.App)` functions and `app.Tool(...)` calls
3. Generates and runs a temporary compile program to extract runtime metadata (JSON Schemas, function signatures) via reflection
4. Generates all downstream artifacts

**Output:**
- `tools/<pkg>/zz_generated_registry.go` — typed tool dispatchers + `Registry()` function per package
- `shipq/lib/llmpersist/zz_generated_persister.go` — persister adapter wrapping `queries.Runner` → `llm.Persister`
- `migrations/*_llm_tables.go` — migration for `llm_conversations` + `llm_messages` tables
- `querydefs/` — querydefs for LLM persistence (insert/update/list conversations and messages)
- LLM stream message types (`LLMTextDelta`, `LLMToolCallStart`, `LLMToolCallResult`, `LLMDone`) injected as `FromServer` types on LLM-enabled channels

**Note:** After the first compile, run `shipq migrate up` to apply the generated migration.

---

## Services

### `shipq start`

Start a development service.

```sh
shipq start <service>
```

**Available services:**

| Service | Description |
|---------|-------------|
| `postgres` | Start a PostgreSQL server |
| `mysql` | Start a MySQL server |
| `sqlite` | Initialize the SQLite database file |
| `redis` | Start a Redis server |
| `minio` | Start a MinIO S3-compatible object store |
| `centrifugo` | Start Centrifugo (WebSocket hub) |
| `server` | Run the application server (`go run ./cmd/server`) |
| `worker` | Run the background worker (`go run ./cmd/worker`) |

---

## Utilities

### `shipq seed`

Run all seed files in the `seeds/` directory.

```sh
shipq seed
```

---

### `shipq kill-port`

Kill the process bound to a specific TCP port.

```sh
shipq kill-port <port>
```

Sends `SIGTERM` to the process on the specified port. Sends `SIGKILL` if the process doesn't exit within 3 seconds.

---

### `shipq kill-defaults`

Kill all processes on default ShipQ development service ports.

```sh
shipq kill-defaults
```
