---
title: Installation
description: How to install and build ShipQ from source.
---

ShipQ is a Go CLI tool that you build from source. There is no published binary — you clone the repository and compile it yourself.

## Prerequisites

- **Go 1.25+** — ShipQ requires a recent Go toolchain. Install it from [go.dev](https://go.dev/dl/).
- **A local database** (optional at install time) — ShipQ supports **PostgreSQL**, **MySQL**, and **SQLite**. If none are available, ShipQ defaults to SQLite (stored under `.shipq/data/`).

### Optional (for specific subsystems)

- **Redis** — required for the workers/channels system (`shipq workers`)
- **Centrifugo** — required for WebSocket realtime (`shipq workers`)
- **MinIO** (or any S3-compatible store) — required for file uploads (`shipq files`)
- **Nix** — ShipQ ships a `shell.nix` for reproducible dev environments

## Build from Source

Clone the repository and build the `shipq` binary:

```sh
git clone https://github.com/shipq/shipq.git
cd shipq
go build -o shipq ./cmd/shipq
```

This produces a `shipq` binary in the current directory. You can move it to a location on your `$PATH`:

```sh
mv shipq /usr/local/bin/shipq
```

Verify the installation:

```sh
shipq --help
```

You should see the full command listing:

```
shipq - A database migration and code generation tool

Usage:
  shipq <command> [arguments]

Commands:
  init              Initialize a new shipq project
  auth              Generate authentication system
  signup            Generate signup handler
  db setup          Set up the database
  db compile        Generate type-safe query runner code
  migrate new       Create a new migration
  migrate up        Run all pending migrations
  resource          Generate CRUD handler(s) for a table
  handler compile   Compile handler registry and run codegen
  ...
```

## Using Nix

If you use Nix, ShipQ can generate a `shell.nix` for your project:

```sh
shipq nix
```

This creates a `shell.nix` pinned to the latest stable nixpkgs with an empty package list, giving you a reproducible starting point to add your own dependencies.

## Next Steps

Once you have the `shipq` binary, head to the [Quickstart](/getting-started/quickstart/) to create your first project.
