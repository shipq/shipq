---
title: Deployment
description: Deploy your ShipQ application to production using Docker and best practices.
---

ShipQ generates self-contained Go projects that can be deployed anywhere Go runs. The `shipq docker` command generates production-ready Dockerfiles to make containerized deployment straightforward.

## Generating Dockerfiles

```sh
shipq docker
```

This generates production Dockerfiles for your application:

- **`Dockerfile`** (or `Dockerfile.server`) — Multi-stage build for the HTTP server
- **`Dockerfile.worker`** — Multi-stage build for the background worker (if you've set up `shipq workers`)

The generated Dockerfiles use multi-stage builds to keep the final image small: the first stage compiles your Go binary, and the second stage copies only the binary into a minimal base image.

## Building and Running

### Server only

```sh
docker build -t myapp-server .
docker run -p 8080:8080 \
  -e DATABASE_URL="postgres://user:pass@db-host:5432/myapp" \
  -e COOKIE_SECRET="your-secret-here" \
  myapp-server
```

### With a background worker

```sh
# Build both images
docker build -t myapp-server -f Dockerfile.server .
docker build -t myapp-worker -f Dockerfile.worker .

# Run server
docker run -p 8080:8080 \
  -e DATABASE_URL="postgres://user:pass@db-host:5432/myapp" \
  -e COOKIE_SECRET="your-secret-here" \
  myapp-server

# Run worker
docker run \
  -e DATABASE_URL="postgres://user:pass@db-host:5432/myapp" \
  -e REDIS_URL="redis://redis-host:6379" \
  myapp-worker
```

## Environment Variables

ShipQ-generated applications read configuration from environment variables in production. Here are the key variables you need to configure:

### Required

| Variable | Description |
|----------|-------------|
| `DATABASE_URL` | Production database connection URL (Postgres, MySQL, or SQLite) |
| `COOKIE_SECRET` | Secret key used to sign session cookies (must be kept secret) |

### Authentication (if `shipq auth` was used)

| Variable | Description |
|----------|-------------|
| `COOKIE_SECRET` | HMAC secret for signing session cookies |

### OAuth (if configured)

| Variable | Description |
|----------|-------------|
| `GOOGLE_CLIENT_ID` | Google OAuth client ID |
| `GOOGLE_CLIENT_SECRET` | Google OAuth client secret |
| `GOOGLE_REDIRECT_URL` | Google OAuth callback URL (production URL) |
| `GITHUB_CLIENT_ID` | GitHub OAuth app client ID |
| `GITHUB_CLIENT_SECRET` | GitHub OAuth app client secret |
| `GITHUB_REDIRECT_URL` | GitHub OAuth callback URL (production URL) |

### File Uploads (if `shipq files` was used)

| Variable | Description |
|----------|-------------|
| `S3_BUCKET` | S3 bucket name |
| `S3_REGION` | AWS region (e.g., `us-east-1`) |
| `S3_ENDPOINT` | S3 endpoint URL (empty for AWS; set for MinIO, GCS, R2) |
| `AWS_ACCESS_KEY_ID` | AWS access key |
| `AWS_SECRET_ACCESS_KEY` | AWS secret key |

### Workers & Channels (if `shipq workers` was used)

| Variable | Description |
|----------|-------------|
| `REDIS_URL` | Redis connection URL for the job queue |
| `CENTRIFUGO_URL` | Centrifugo API URL |
| `CENTRIFUGO_API_KEY` | Centrifugo API key |
| `CENTRIFUGO_SECRET` | Centrifugo HMAC secret for Centrifugo connection JWTs (separate from session cookies) |

### Custom Environment Variables

If you've declared additional required environment variables in `shipq.ini`:

```ini
[env]
STRIPE_SECRET_KEY = required
SENDGRID_API_KEY = required
```

ShipQ's generated config loader validates these at startup. The application will refuse to start if any required variable is missing, giving you a clear error message instead of a silent runtime failure.

## Database Considerations

### Postgres (Recommended for Production)

Postgres is the most battle-tested database for ShipQ applications:

```sh
DATABASE_URL="postgres://user:password@db-host:5432/myapp?sslmode=require"
```

Use `sslmode=require` (or `sslmode=verify-full`) for production connections.

### MySQL

```sh
DATABASE_URL="mysql://user:password@tcp(db-host:3306)/myapp?tls=true"
```

### SQLite

SQLite works well for single-instance deployments or edge computing scenarios:

```sh
DATABASE_URL="sqlite:///data/myapp.db"
```

:::caution
SQLite is not recommended for multi-instance production deployments. If you're running multiple server replicas behind a load balancer, use Postgres or MySQL.
:::

## Dev vs. Production Behavior

ShipQ-generated servers behave differently in production:

| Feature | Dev/Test | Production |
|---------|----------|------------|
| `GET /openapi` | ✅ Serves OpenAPI 3.1 JSON spec | ❌ Disabled |
| `GET /docs` | ✅ Interactive API docs UI | ❌ Disabled |
| Admin UI | ✅ Available | ❌ Disabled |
| Error details | ✅ Verbose error messages | ❌ Generic error responses |

The environment is typically determined by a `GO_ENV` or equivalent environment variable. Check your generated `cmd/server/main.go` for the specific mechanism.

## Auto-Migrate on Startup

For simpler deployments, ShipQ can run all pending migrations automatically when the server or worker starts. Set `auto_migrate = true` in the `[db]` section of `shipq.ini`:

```ini
[db]
database_url = postgres://localhost:5432/myapp_dev
auto_migrate = true
```

After recompiling (`shipq handler compile` or any codegen-triggering command), the generated `cmd/server/main.go` and `cmd/worker/main.go` will call `dbmigrate.RunWithDB()` on startup — after connecting to the database but before serving any traffic. The migrator is idempotent and only applies unapplied migrations, so it's safe to call on every boot.

This eliminates the need for a separate migration step (init container, release-phase command, wrapper script) in environments like Docker Compose, single-binary VPS deploys, and PaaS platforms (Fly.io, Railway, Render).

:::caution
For multi-replica deployments (e.g., Kubernetes with horizontal scaling), running migrations from every replica simultaneously can cause race conditions. In those environments, prefer running migrations as a separate Kubernetes Job or init container instead.
:::

## Docker Compose Example

For local development that mirrors production, you can use Docker Compose:

```yaml
version: "3.8"
services:
  db:
    image: postgres:16
    environment:
      POSTGRES_USER: myapp
      POSTGRES_PASSWORD: secret
      POSTGRES_DB: myapp
    ports:
      - "5432:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"

  server:
    build: .
    ports:
      - "8080:8080"
    environment:
      DATABASE_URL: "postgres://myapp:secret@db:5432/myapp?sslmode=disable"
      COOKIE_SECRET: "dev-secret-change-in-production"
      REDIS_URL: "redis://redis:6379"
    depends_on:
      - db
      - redis
    # If auto_migrate = true in shipq.ini, migrations run automatically on boot.
    # No separate migration container needed.

  worker:
    build:
      context: .
      dockerfile: Dockerfile.worker
    environment:
      DATABASE_URL: "postgres://myapp:secret@db:5432/myapp?sslmode=disable"
      REDIS_URL: "redis://redis:6379"
    depends_on:
      - db
      - redis

volumes:
  pgdata:
```

## Nix-Based Deployment

If you use Nix, ShipQ can generate a `shell.nix` for reproducible builds:

```sh
shipq nix
```

This pins your development environment to a specific nixpkgs revision, ensuring that all team members and CI systems use the exact same toolchain versions.

## Pre-Deployment Checklist

Before deploying to production, make sure you have:

- [ ] All tests passing: `go test ./... -v`
- [ ] `DATABASE_URL` pointing to your production database
- [ ] `COOKIE_SECRET` set to a strong, random value (not the dev default)
- [ ] OAuth redirect URLs updated to production domains (if using OAuth)
- [ ] S3 credentials configured (if using file uploads)
- [ ] Redis and Centrifugo accessible (if using workers/channels)
- [ ] All `[env]` required variables present
- [ ] TLS/SSL enabled for database connections
- [ ] A reverse proxy (nginx, Caddy, or a cloud load balancer) in front of the Go server
- [ ] Migration strategy decided: either `auto_migrate = true` in `shipq.ini` for single-instance deploys, or a separate migration step for multi-replica environments

## CI/CD

Since ShipQ generates self-contained Go projects, your CI/CD pipeline doesn't need ShipQ installed. A typical pipeline looks like:

```sh
# Install Go dependencies
go mod download

# Run tests (needs a test database)
go test ./... -v

# Build the binary
go build -o server ./cmd/server

# Build Docker image
docker build -t myapp-server .

# Deploy
docker push myapp-server:latest
```

The key insight is that **ShipQ is a development-time tool**. Once your code is generated and committed, the build and deployment pipeline only needs Go and Docker.
