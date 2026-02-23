---
title: Column Types
description: Complete reference for column types supported by shipq migrate new.
---

When creating migrations with `shipq migrate new`, you specify columns using the `name:type` grammar. This page documents all supported column types, the `references` syntax for foreign keys, and how types map to each supported database dialect.

## Column Grammar

```sh
shipq migrate new <table> [columns...] [--global]
```

Each column follows one of two patterns:

- **`name:type`** — A regular column with a data type
- **`name:references:table`** — A foreign key column referencing another table

## Supported Types

| Type | Description | Postgres | MySQL | SQLite |
|------|-------------|----------|-------|--------|
| `string` | Short text, up to 255 characters | `VARCHAR(255)` | `VARCHAR(255)` | `TEXT` |
| `text` | Long-form text, unlimited length | `TEXT` | `TEXT` | `TEXT` |
| `int` | Standard integer | `INTEGER` | `INT` | `INTEGER` |
| `bigint` | Large integer (64-bit) | `BIGINT` | `BIGINT` | `INTEGER` |
| `bool` | Boolean true/false | `BOOLEAN` | `TINYINT(1)` | `INTEGER` |
| `float` | Floating-point number | `DOUBLE PRECISION` | `DOUBLE` | `REAL` |
| `decimal` | Fixed-precision decimal | `NUMERIC` | `DECIMAL` | `NUMERIC` |
| `datetime` | Date and time with timezone | `TIMESTAMP` | `DATETIME` | `TEXT` |
| `timestamp` | Alias for `datetime` | `TIMESTAMP` | `DATETIME` | `TEXT` |
| `binary` | Binary/blob data | `BYTEA` | `BLOB` | `BLOB` |
| `json` | JSON data | `JSONB` | `JSON` | `TEXT` |

## Foreign Key References

Use the `references` type to create a foreign key column:

```sh
name:references:table
```

This creates:
- A column named `name` with the appropriate integer type for the referenced table's primary key
- A foreign key constraint referencing the `id` column of the target table

### Examples

```sh
# Single foreign key
shipq migrate new books title:string author_id:references:authors

# Multiple foreign keys
shipq migrate new order_items quantity:int price:decimal order_id:references:orders product_id:references:products

# Self-referential foreign key
shipq migrate new categories name:string parent_id:references:categories
```

## Automatic Columns

Every table created by ShipQ automatically includes the following columns — you never need to specify them:

| Column | Type | Description |
|--------|------|-------------|
| `id` | Auto-incrementing integer | Primary key |
| `public_id` | String (nanoid) | URL-safe unique public identifier |
| `created_at` | Timestamp | Set automatically on row creation |
| `updated_at` | Timestamp | Updated automatically on row modification |
| `deleted_at` | Nullable timestamp | Used for soft deletes (null = not deleted) |

These columns are always present on every table and are used by generated queries, handlers, and tests.

## Scope-Injected Columns

When `[db] scope = organization_id` is configured in `shipq.ini`, an additional column is automatically injected into every new migration (unless `--global` is passed):

| Column | Type | Description |
|--------|------|-------------|
| `organization_id` | `references:organizations` | Foreign key for multi-tenant data isolation |

See the [Multi-Tenancy guide](/guides/multi-tenancy/) for details.

## Examples

### Basic table

```sh
shipq migrate new users name:string email:string
```

Resulting columns: `id`, `public_id`, `name`, `email`, `created_at`, `updated_at`, `deleted_at`

### Table with various types

```sh
shipq migrate new products name:string description:text price:decimal in_stock:bool weight:float metadata:json
```

### Table with a foreign key

```sh
shipq migrate new comments body:text post_id:references:posts
```

### Table with timestamps

```sh
shipq migrate new events name:string starts_at:datetime ends_at:datetime
```

### Global table (no scope injection)

When multi-tenancy is configured, use `--global` for shared lookup tables:

```sh
shipq migrate new countries name:string code:string --global
```

## Type Selection Guide

| Use case | Recommended type |
|----------|-----------------|
| Names, titles, short labels | `string` |
| Descriptions, blog content, bios | `text` |
| Counts, quantities, ages | `int` |
| Very large numbers, row counts | `bigint` |
| Prices, monetary values | `decimal` |
| Weights, measurements, percentages | `float` |
| Flags, toggles, on/off states | `bool` |
| Dates, times, scheduling | `datetime` |
| Flexible/schemaless data | `json` |
| File contents, encoded data | `binary` |
| Foreign key to another table | `references` |
