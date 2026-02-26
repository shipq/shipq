---
title: Configuration (shipq.ini)
description: Understanding the shipq.ini control plane that drives all ShipQ behavior.
---

`shipq.ini` is the control plane for your ShipQ project. It lives at your project root and is where ShipQ reads and writes all configuration. Every ShipQ command consults this file.

## File Format

`shipq.ini` uses standard INI format with `[section]` headers and `key = value` pairs:

```ini
[db]
database_url = postgres://localhost:5432/myapp_dev

[auth]
protect_by_default = true

[typescript]
framework = react
http_output = .
```

Comments use `#` or `;` prefixes. Keys and section names are case-insensitive.

## Sections Overview

### `[db]` — Database

The most important section. Controls database connectivity and multi-tenant scoping.

| Key | Description | Example |
|-----|-------------|---------|
| `database_url` | Connection URL for the dev database. Written by `shipq db setup`. | `postgres://localhost:5432/myapp_dev` |
| `scope` | Optional global scope column for multi-tenancy. When set, `shipq migrate new` auto-injects this column as a foreign key reference. | `organization_id` |

The `database_url` determines which SQL dialect ShipQ uses for code generation:

- **Postgres**: `postgres://` or `postgresql://` URLs
- **MySQL**: `mysql://` URLs
- **SQLite**: `sqlite://` URLs (stored under `.shipq/data/`)

### `[auth]` — Authentication

Created by `shipq auth`. Controls authentication behavior.

| Key | Description | Example |
|-----|-------------|---------|
| `protect_by_default` | When `true`, generated handlers require authentication unless `--public` is passed. | `true` |

### `[typescript]` — TypeScript Client Codegen

Controls TypeScript client generation during `shipq handler compile`.

| Key | Description | Example |
|-----|-------------|---------|
| `framework` | Which framework helpers to generate alongside the base HTTP client. Options: `react`, `svelte`, or omit for plain TS. | `react` |
| `http_output` | Output directory for generated TypeScript files, relative to project root. | `.` |

### `[files]` — File Uploads

Created by `shipq files`. Configures S3-compatible file upload storage.

The actual credentials (`S3_BUCKET`, `S3_REGION`, `S3_ENDPOINT`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`) are read from environment variables — not stored in the INI file.

### `[workers]` — Workers & Channels

Created by `shipq workers`. Configures the background job queue and realtime WebSocket system.

| Key | Description | Example |
|-----|-------------|---------|
| `redis_url` | Redis connection URL for the job queue. | `redis://localhost:6379` |
| `centrifugo_url` | Centrifugo API URL. | `http://localhost:8000` |
| `centrifugo_api_key` | API key for Centrifugo. | (auto-generated) |
| `centrifugo_secret` | HMAC secret for signing Centrifugo WebSocket connection/subscription tokens. | (auto-generated) |

### `[llm]` — LLM Tool Calling

Added manually by the user. Tells `shipq llm compile` which Go packages contain LLM tool registrations.

| Key | Description | Example |
|-----|-------------|---------|
| `tool_pkgs` | Comma-separated list of Go import paths for packages that export `Register(app *llm.App)` functions. | `myapp/tools/weather, myapp/tools/calendar` |

```ini
[llm]
tool_pkgs = myapp/tools/weather, myapp/tools/calendar
```

Provider, model, and system prompt are **not** in `shipq.ini` — they live in the user's hand-written `Setup` function as ordinary Go code. This means provider choice, model selection, and multi-provider mixing are runtime decisions, not build-time config.

The related environment variables (`OPENAI_API_KEY`, `ANTHROPIC_API_KEY`) are read by the user's `Setup` function at runtime and are never stored in `shipq.ini`.

### `[env]` — Environment Variable Validation

Optionally declare additional environment variables that must be present when running in production. ShipQ's generated config loader will validate these at startup.

```ini
[env]
STRIPE_SECRET_KEY = required
SENDGRID_API_KEY = required
```

## How Commands Use shipq.ini

Different commands read and write different sections:

| Command | Reads | Writes |
|---------|-------|--------|
| `shipq init` | — | Creates `shipq.ini` with `[db]` and `[typescript]` |
| `shipq db setup` | `DATABASE_URL` env var | `[db] database_url` |
| `shipq auth` | `[db]` | `[auth]` |
| `shipq migrate new` | `[db] scope` | — |
| `shipq migrate up` | `[db] database_url` | — |
| `shipq db compile` | `[db]` | — |
| `shipq handler compile` | `[auth]`, `[typescript]` | — |
| `shipq files` | `[db]` | `[files]` |
| `shipq workers` | `[db]`, `[auth]` | `[workers]` |
| `shipq workers compile` | `[db]`, `[auth]`, `[workers]` | — |
| `shipq llm compile` | `[db]`, `[workers]`, `[llm]` | — |
| `shipq resource` | `[db]`, `[auth]` | — |

## The `.shipq/` Directory

Alongside `shipq.ini`, ShipQ maintains a `.shipq/` directory at your project root for internal state:

- `.shipq/data/` — SQLite database files (when using SQLite)
- `.shipq/compile/` — Temporary build artifacts used during query compilation

This directory is added to `.gitignore` by `shipq init`. You should never need to edit anything inside it.

## Scope Configuration Deep Dive

The `[db] scope` setting deserves special attention because it affects code generation across the entire stack.

When you set `scope = organization_id`:

1. **`shipq migrate new`** auto-injects `organization_id:references:organizations` into new tables (unless you pass `--global`).
2. **Generated queries** automatically filter by the scope column.
3. **Generated handlers** extract the scope value from the authenticated user's context.
4. **Generated tests** include tenancy isolation tests that verify users in one organization cannot access another organization's data.

```ini
[db]
database_url = postgres://localhost:5432/myapp_dev
scope = organization_id
```

This is one of ShipQ's most powerful features — multi-tenancy is woven into the compiler chain, not bolted on at runtime.

## Best Practices

- **Commit `shipq.ini`** to version control. It's part of your project's build configuration.
- **Never store secrets** in `shipq.ini`. Use environment variables for credentials.
- **Set `scope` early** if you need multi-tenancy. Adding it later requires re-generating migrations and handlers.
- **Don't hand-edit generated values** like `database_url` unless you know what you're doing — prefer `shipq db setup` to write it correctly.
