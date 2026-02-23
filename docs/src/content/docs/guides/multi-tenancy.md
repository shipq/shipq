---
title: Multi-Tenancy
description: How to use ShipQ's scope system for automatic organization-level data isolation.
---

ShipQ has first-class support for multi-tenancy through its **scope** system. When configured, scope injection is woven into the entire compiler chain — migrations, queries, handlers, and tests all enforce tenant isolation automatically.

## Configuring Scope

Multi-tenancy is activated by setting the `scope` key in the `[db]` section of `shipq.ini`:

```ini
[db]
database_url = postgres://localhost:5432/myapp_dev
scope = organization_id
```

The value (`organization_id`) is the name of the foreign key column that will be injected into every new table to reference the `organizations` table.

:::caution
Set the `scope` configuration **before** creating your scoped migrations. Adding multi-tenancy retroactively requires re-generating migrations, queries, and handlers.
:::

## Prerequisites

Before configuring scope, you must have the auth system in place, because the `organizations` table is created by `shipq auth`:

```sh
shipq init
shipq db setup
shipq auth
go mod tidy
shipq migrate up
```

Then add the scope to `shipq.ini`:

```ini
[db]
database_url = postgres://localhost:5432/myapp_dev
scope = organization_id
```

## How Scope Affects the Compiler Chain

Once `scope = organization_id` is set, every stage of ShipQ's compiler chain becomes scope-aware.

### 1. Migrations — Automatic Column Injection

When you create a new migration:

```sh
shipq migrate new pets name:string species:string age:int
```

ShipQ automatically injects `organization_id:references:organizations` into the column list. The resulting migration includes an `organization_id` foreign key column without you having to specify it.

This means the generated migration is equivalent to:

```sh
shipq migrate new pets name:string species:string age:int organization_id:references:organizations
```

#### Global Tables

Some tables shouldn't be scoped (e.g., lookup tables, shared configuration). Use the `--global` flag to skip scope injection:

```sh
shipq migrate new countries name:string code:string --global
```

Global tables don't get the `organization_id` column and aren't subject to tenant filtering.

### 2. Queries — Automatic Filtering

Generated query definitions automatically include `organization_id` in their WHERE clauses. When you run `shipq resource pets all`, the generated queries filter by the tenant's organization:

- **List queries** only return rows matching the current user's `organization_id`
- **Get-one queries** include `organization_id` in the lookup condition
- **Create queries** set `organization_id` from the authenticated user's context
- **Update queries** scope the WHERE clause to the correct tenant
- **Delete queries** scope the WHERE clause to the correct tenant

This means a user in Organization A can never accidentally read, update, or delete data belonging to Organization B — the filter is baked into the SQL at compile time.

### 3. Handlers — Context Extraction

Generated handlers automatically extract the `organization_id` from the authenticated user's session context and pass it to query parameters. The flow is:

1. Auth middleware reads and verifies the signed `session` cookie and loads the user's account
2. The account includes the user's `organization_id`
3. The handler extracts `organization_id` from the context
4. The handler passes it as a query parameter
5. The generated query filters by `organization_id`

No manual plumbing is required — the scope value flows from authentication through to the database query automatically.

### 4. Tests — Tenancy Isolation Verification

This is where ShipQ's scope system really shines. When scope is configured, `shipq handler compile` generates **tenancy isolation tests** alongside your CRUD tests.

These tests verify that:

- A user in Organization A **cannot** read Organization B's data
- A user in Organization A **cannot** update Organization B's data
- A user in Organization A **cannot** delete Organization B's data
- List endpoints only return data belonging to the authenticated user's organization

The generated tenancy test file is located at:

```
api/<table>/spec/zz_generated_tenancy_test.go
```

This file is regenerated on every `shipq handler compile`, so you don't need to maintain these tests by hand.

## Full Walkthrough

Here's the complete flow for building a scoped, auth-protected resource:

```sh
# 1. Initialize the project
shipq init
shipq db setup

# 2. Generate auth (creates organizations, accounts, sessions)
shipq auth
go mod tidy

# 3. Configure scope in shipq.ini
# Add under [db]:
#   scope = organization_id

# 4. Create a scoped migration (organization_id is auto-injected)
shipq migrate new pets name:string species:string age:int
shipq migrate up

# 5. Generate scoped, auth-protected resource
shipq resource pets all
go mod tidy

# 6. Run all tests (includes tenancy isolation tests)
go test ./... -v -count=1
```

## Verifying Tenancy Isolation

After running the steps above, your test suite includes tenancy isolation tests. You can run them specifically:

```sh
go test ./api/pets/spec/... -v -count=1
```

Look for test output that shows:

- Creating data as User A (Organization A)
- Attempting to access that data as User B (Organization B)
- Verifying that User B gets a 404 or empty list (not Organization A's data)

## Mixing Scoped and Global Resources

In a real application, you'll likely have both scoped and global tables:

```sh
# Scoped tables (organization_id auto-injected)
shipq migrate new projects name:string description:text
shipq migrate new tasks title:string status:string project_id:references:projects

# Global tables (no organization_id)
shipq migrate new roles name:string --global
shipq migrate new permissions action:string resource:string --global
```

The `--global` flag is the escape hatch — use it for any table that should be shared across all organizations.

## How It Works Under the Hood

ShipQ's scope system is implemented at the **compiler level**, not at runtime:

1. **`shipq migrate new`** reads `[db] scope` from `shipq.ini` and injects the scope column into the migration's column list before generating the Go migration file.

2. **`shipq resource`** (and the generated querydefs) includes the scope column in all WHERE clauses, INSERT column lists, and parameter types.

3. **`shipq handler compile`** generates tenancy isolation tests when it detects that a table has the scope column and auth is configured.

Because the filtering happens at the SQL level (not in application middleware), there's no way to accidentally bypass it. The generated SQL literally includes `WHERE organization_id = ?` — it's impossible to query without providing the scope value.

## Best Practices

- **Set scope early** — ideally right after `shipq auth` and before creating any business tables.
- **Use `--global` intentionally** — only for truly shared data like lookup tables, roles, and system configuration.
- **Trust the generated tests** — the tenancy isolation tests are comprehensive. If they pass, your data isolation is correct.
- **Don't remove `organization_id` from generated queries** — the scope column in queries is there by design. Removing it breaks tenant isolation.
- **Run the full test suite** after any schema or handler changes: `go test ./... -v`

## Next Steps

- **[Authentication](/guides/authentication/)** — The auth system that powers scope extraction
- **[Handlers & Resources](/guides/handlers/)** — How generated handlers use scope
- **[E2E Example](/e2e-example/)** — A full walkthrough that includes multi-tenancy
