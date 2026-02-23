---
title: The Compiler Chain
description: How ShipQ's three compilers feed each other to produce a complete, typed backend stack.
---

ShipQ is best understood as **three compilers that feed each other**. Each compiler takes a specific kind of input, produces typed artifacts, and hands them off to the next stage.

## Overview

```
Migrations (Go) ──→ Schema Compiler ──→ Typed Schema Bindings
                                              │
Query Definitions (Go DSL) ──────────────────→ Query Compiler ──→ Typed Query Runners
                                                                        │
API Handler Packages ──────────────────────────────────────────→ Handler Compiler
                                                                        │
                                                                        ▼
                                                    ┌─────────────────────────────────┐
                                                    │ cmd/server/main.go              │
                                                    │ OpenAPI 3.1 spec + docs UI      │
                                                    │ Admin UI                        │
                                                    │ HTTP test client + harness      │
                                                    │ Integration tests (RBAC/tenancy)│
                                                    │ TypeScript HTTP clients         │
                                                    └─────────────────────────────────┘
```

## Compiler 1: Schema Compiler

**Trigger:** `shipq migrate up`

**Input:** Migration files in `migrations/*.go` — Go functions that mutate a `MigrationPlan` using a typed DDL builder.

**Process:**
1. Embeds ShipQ runtime libraries into `shipq/lib/...` and rewrites imports so your project is self-contained.
2. Discovers all `migrations/*.go` files.
3. Generates a temporary Go program that imports your migrations, executes them in order to build a canonical `MigrationPlan`, and prints the plan as JSON to stdout.
4. Writes the canonical plan to `shipq/db/migrate/schema.json`.
5. Generates typed schema bindings in `shipq/db/schema/schema.go`.
6. Applies the plan against both dev and test databases.

**Output artifacts:**
- `shipq/db/migrate/schema.json` — canonical schema + per-dialect SQL instructions
- `shipq/db/schema/schema.go` — typed tables and typed columns used by the query DSL
- `shipq/lib/...` — embedded runtime libraries (query DSL, migrator, HTTP helpers, etc.)

## Compiler 2: Query Compiler

**Trigger:** `shipq db compile`

**Input:** Query definition files under `querydefs/` — Go packages whose `init()` functions register queries in a global registry using the PortSQL DSL.

**Process:**
1. Generates a temporary Go program that imports your querydef packages (triggering their `init()` functions).
2. Serializes the query registry — each query's AST, return type, parameter types, and cursor columns.
3. Compiles each AST to dialect-specific SQL (Postgres, MySQL, SQLite) with correct quoting, placeholders, JSON aggregation, and ILIKE translation.
4. Generates typed query runner code with parameter structs and result types.

**Output artifacts:**
- `shipq/queries/types.go` — shared parameter and result types
- `shipq/queries/<dialect>/runner.go` — dialect-specific query runner with typed methods

**Key point:** Queries are registered at `init()` time using functions like `query.MustDefineOne`, `query.MustDefineMany`, `query.MustDefineExec`, and `query.MustDefinePaginated`. These panic on invalid definitions or duplicate names — failures are immediate and obvious, not silent runtime issues.

## Compiler 3: Handler Compiler

**Trigger:** `shipq handler compile`

**Input:** API handler packages under `api/` — Go packages that register HTTP handlers with their method, path, request/response types, and auth requirements.

**Process:**
1. Discovers all handler registrations across your `api/` packages.
2. Extracts full metadata: HTTP method, path (with path params), request/response struct definitions, auth requirements.
3. Generates server wiring, OpenAPI spec, test infrastructure, and client code.

**Output artifacts:**
- `cmd/server/main.go` — runnable server with all handlers wired up
- OpenAPI 3.1 JSON spec embedded into the server (`GET /openapi` in dev/test)
- API docs UI (`GET /docs` in dev/test)
- Admin UI (OpenAPI-driven, for manual testing)
- HTTP test client + harness used by generated specs and integration tests
- TypeScript HTTP client codegen (and optional framework helpers for React/Svelte)

## How They Connect

The compilers form a directed pipeline:

1. **Schema → Queries:** The schema compiler generates typed column references (e.g., `schema.Pets.Name()`) that the query DSL uses. You can't write queries until the schema compiler has run.

2. **Queries → Handlers:** The query compiler generates typed runner methods (e.g., `queries.ListPets(ctx, db, params)`) that handlers call. You can't write handlers that use generated queries until the query compiler has run.

3. **Handlers → Everything Else:** The handler compiler reads your handler registrations and generates the full serving stack. It must run last because it needs to know about all your routes.

## When to Re-Run Each Compiler

| What changed?                     | Command to run          |
|-----------------------------------|-------------------------|
| Migration files added or modified | `shipq migrate up`      |
| Query definitions added or modified | `shipq db compile`    |
| Handlers or routes changed        | `shipq handler compile` |

:::tip
`shipq resource <table> all` is a convenience command that generates querydefs, handlers, tests, and runs `handler compile` for you — touching all three stages at once.
:::

## Self-Contained Projects

A key design decision: ShipQ embeds its runtime libraries into your repo under `shipq/lib/...` and rewrites imports so the generated project doesn't depend on a published ShipQ module. Your compiled project is a standalone Go application with no external ShipQ dependency.

This means:
- You can vendor and audit every line of generated code.
- Your CI/CD doesn't need ShipQ installed — it just needs Go.
- You can always inspect what ShipQ generated and understand (or customize) it.
