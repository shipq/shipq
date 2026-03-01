---
title: Schema & Migrations
description: How to define your database schema using ShipQ's typed Go migration system.
---

ShipQ manages your database schema through **Go migrations** — functions that mutate a `MigrationPlan` using a typed DDL builder. Migrations are the input to ShipQ's schema compiler, which produces typed bindings used by every other part of the system.

## Creating Migrations

Use `shipq migrate new` to generate a migration file:

```sh
shipq migrate new pets name:string species:string age:int
```

This creates a timestamped Go file in `migrations/` that imports the embedded migration and DDL libraries:

- `<your module>/shipq/lib/db/portsql/migrate`
- `<your module>/shipq/lib/db/portsql/ddl`

The generated migration uses ShipQ's typed DDL builder to define the table, columns, and any constraints.

## Column Grammar

The column grammar for `shipq migrate new` follows the pattern `name:type` or `name:references:table`:

```sh
shipq migrate new <table> [columns...] [--global]
```

### Supported Column Types

| Type | Description | SQL Equivalent |
|------|-------------|----------------|
| `string` | Short text (VARCHAR) | `VARCHAR(255)` |
| `text` | Long text | `TEXT` |
| `int` | Integer | `INTEGER` |
| `bigint` | Large integer | `BIGINT` |
| `bool` | Boolean | `BOOLEAN` |
| `float` | Floating point | `FLOAT` / `DOUBLE` |
| `decimal` | Fixed-precision decimal | `DECIMAL` / `NUMERIC` |
| `datetime` | Date and time | `TIMESTAMP` / `DATETIME` |
| `timestamp` | Alias for datetime | `TIMESTAMP` |
| `binary` | Binary data | `BLOB` / `BYTEA` |
| `json` | JSON data | `JSON` / `JSONB` |

### Foreign Key References

To create a foreign key column, use the `references` type:

```sh
shipq migrate new books title:string author_id:references:authors
```

This creates an `author_id` column that references the `authors` table's primary key, with appropriate foreign key constraints.

### Examples

```sh
# Simple table
shipq migrate new users name:string email:string

# Table with various types
shipq migrate new posts title:string body:text published:bool view_count:int

# Table with a foreign key
shipq migrate new comments body:text post_id:references:posts

# Table with multiple references
shipq migrate new order_items quantity:int price:decimal order_id:references:orders product_id:references:products
```

## Automatic Columns

ShipQ automatically adds the following columns to every migration:

- `id` — auto-incrementing primary key
- `public_id` — a unique, URL-safe public identifier (nanoid)
- `created_at` — timestamp set on creation
- `updated_at` — timestamp updated on modification
- `deleted_at` — nullable timestamp for soft deletes

You never need to specify these — they're always present.

## Applying Migrations

Run all pending migrations with:

```sh
shipq migrate up
```

This triggers the **schema compiler**, which:

1. **Embeds runtime libraries** into `shipq/lib/...` and rewrites imports so your project is self-contained.
2. **Discovers** all `migrations/*.go` files.
3. **Generates a temporary Go program** that imports your migrations package, executes all migration functions in order to build a canonical `MigrationPlan`, and prints the plan as JSON.
4. **Writes** the canonical plan to `shipq/db/migrate/schema.json`.
5. **Generates typed schema bindings** in `shipq/db/schema/schema.go`.
6. **Applies** the plan against both dev and test databases.

### Generated Artifacts

After `shipq migrate up`, you'll find:

- **`shipq/db/migrate/schema.json`** — The canonical schema representation, including per-dialect SQL instructions for Postgres, MySQL, and SQLite.
- **`shipq/db/schema/schema.go`** — Typed Go code with table and column references used by the PortSQL query DSL.
- **`shipq/lib/...`** — Embedded runtime libraries (query DSL, migrator, HTTP helpers, etc.).

## Scope / Tenancy Injection

If you configure a global scope column in `shipq.ini`:

```ini
[db]
scope = organization_id
```

Then `shipq migrate new` will **automatically inject** `organization_id:references:organizations` into every new table. This ensures multi-tenant data isolation is baked into your schema from the start.

To create a table without the scope column (e.g., for global lookup tables), pass the `--global` flag:

```sh
shipq migrate new countries name:string code:string --global
```

:::tip
Set the `scope` configuration **before** creating your first scoped migration. Adding multi-tenancy after the fact requires re-generating migrations, queries, and handlers.
:::

## Resetting the Database

To drop and recreate both dev and test databases and re-run all migrations from scratch:

```sh
shipq migrate reset
# or equivalently:
shipq db reset
```

This is useful during development when you want a clean slate.

## Editing Migrations

Migrations are Go source files, so you can edit them after generation. However, keep in mind:

- **Migrations are executed in filename order** (the timestamp prefix ensures correct ordering).
- **The schema compiler re-executes all migrations** on every `shipq migrate up` to build the canonical plan. Changing an existing migration changes the schema for all subsequent steps.
- **In production**, you should treat applied migrations as immutable. Create new migrations to alter existing tables.

## Auto-Migrate on Startup

For simpler deployments (single-binary deploys, Docker Compose, small VPS), you can configure ShipQ to run all pending migrations automatically when the server or worker starts. Add `auto_migrate = true` to the `[db]` section of `shipq.ini`:

```ini
[db]
database_url = postgres://localhost:5432/myapp_dev
auto_migrate = true
```

When this is set, the next `shipq handler compile` (or any command that triggers codegen) will generate `cmd/server/main.go` and `cmd/worker/main.go` with an automatic migration block that runs before the application starts serving traffic.

### How it works

1. The generated main calls `dbmigrate.RunWithDB(ctx, db)` **after** `db.Ping()` succeeds and **before** any HTTP handler or worker task is registered.
2. The underlying migrator is idempotent — it checks the `_portsql_migrations` tracking table and only applies unapplied migrations. Calling it on every startup is safe by design.
3. If a migration fails, the process exits immediately with a non-zero status code. The server will not start with a partially-migrated schema.
4. Migration start and completion are logged via the structured logger so operators can see migration activity.

### Gating conditions

Auto-migrate codegen is only emitted when **both** conditions hold:

- `auto_migrate = true` is set in `shipq.ini`
- `shipq/db/migrate/schema.json` exists (i.e., `shipq migrate up` has been run at least once)

If `schema.json` doesn't exist yet, the setting is silently ignored to prevent generating code that imports a package that doesn't exist.

### When to use it

Auto-migrate is ideal for:

- **Docker Compose** setups where you don't want a separate migration container
- **Single-binary deploys** on a VPS or PaaS (Fly.io, Railway, Render)
- **Development** to avoid forgetting `shipq migrate up` after pulling new migrations

For more complex production environments (Kubernetes with multiple replicas, blue-green deployments), you may prefer running migrations as a separate step (e.g., a Kubernetes Job or init container) to avoid race conditions when multiple instances start simultaneously.

:::tip
The decision is made at codegen time, not at runtime. There are no CLI flags or environment variables to check — if `auto_migrate = true` is in your `shipq.ini`, the generated binary unconditionally runs migrations on boot.
:::

## Multi-Database Support

The same migration code works across Postgres, MySQL, and SQLite. The schema compiler generates dialect-specific SQL from the canonical plan in `schema.json`. You don't need to write different DDL for different databases.

The dialect is determined by the `database_url` in your `shipq.ini`:

- `postgres://` → PostgreSQL DDL
- `mysql://` → MySQL DDL
- `sqlite://` → SQLite DDL

## Next Steps

- **[Queries (PortSQL)](/guides/queries/)** — Use the generated schema bindings to write type-safe queries.
- **[Handlers & Resources](/guides/handlers/)** — Generate CRUD endpoints from your schema.
- **[Multi-Tenancy](/guides/multi-tenancy/)** — Deep dive into scope-based data isolation.
