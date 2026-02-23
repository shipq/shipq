---
title: File Uploads
description: Add S3-compatible file upload and download support to your ShipQ project.
---

ShipQ includes a managed file upload subsystem that generates everything you need for S3-compatible file storage — migrations, handlers, query definitions, tests, and TypeScript helpers.

## Prerequisites

Before generating the file upload system, you need:

- **`shipq auth`** already run (file uploads require authentication)
- **An S3-compatible object store** available at runtime:
  - [AWS S3](https://aws.amazon.com/s3/)
  - [MinIO](https://min.io/) (for local development)
  - [Cloudflare R2](https://www.cloudflare.com/products/r2/)
  - [Google Cloud Storage](https://cloud.google.com/storage) (S3-compatible mode)

## Generating the File Upload System

```sh
shipq files
```

This single command generates the entire file upload subsystem:

### What gets generated

| Artifact | Description |
|----------|-------------|
| **Migrations** | `managed_files` and `file_access` tables |
| **Handlers** | Upload, download, and access control endpoints in `api/managed_files/` |
| **Query definitions** | File operation queries in `querydefs/` |
| **Tests** | Endpoint tests in `api/managed_files/spec/` |
| **TypeScript helpers** | `shipq-files.ts` with upload/download utilities |

After generating, apply the migrations and compile:

```sh
shipq migrate up
go mod tidy
shipq handler compile
```

## Environment Variables

The file upload system reads its configuration from environment variables — credentials are never stored in `shipq.ini`.

| Variable | Description | Required |
|----------|-------------|----------|
| `S3_BUCKET` | The S3 bucket name for file storage | Yes |
| `S3_REGION` | The AWS region (e.g., `us-east-1`) | Yes |
| `S3_ENDPOINT` | Custom S3 endpoint URL. Leave empty for AWS S3. Set for MinIO, R2, or GCS. | No |
| `AWS_ACCESS_KEY_ID` | AWS (or S3-compatible) access key | Yes |
| `AWS_SECRET_ACCESS_KEY` | AWS (or S3-compatible) secret key | Yes |

### Example: AWS S3

```sh
export S3_BUCKET="myapp-uploads"
export S3_REGION="us-east-1"
export S3_ENDPOINT=""
export AWS_ACCESS_KEY_ID="AKIA..."
export AWS_SECRET_ACCESS_KEY="..."
```

### Example: MinIO (local development)

```sh
export S3_BUCKET="myapp-uploads"
export S3_REGION="us-east-1"
export S3_ENDPOINT="http://localhost:9000"
export AWS_ACCESS_KEY_ID="minioadmin"
export AWS_SECRET_ACCESS_KEY="minioadmin"
```

## Using MinIO for Local Development

ShipQ can start a local MinIO server for development:

```sh
shipq start minio
```

This requires `minio` to be available on your `$PATH`. MinIO provides an S3-compatible API at `http://localhost:9000` and a web console at `http://localhost:9001`.

## Generated Endpoints

After `shipq files` and `shipq handler compile`, you get the following endpoints:

| Method | Route | Description |
|--------|-------|-------------|
| `POST` | `/managed_files` | Upload a file (multipart form data) |
| `GET` | `/managed_files/:id` | Get file metadata |
| `GET` | `/managed_files/:id/download` | Download a file (presigned URL or direct) |
| `DELETE` | `/managed_files/:id` | Delete a file (soft delete) |

All file endpoints require authentication (since `shipq files` requires `shipq auth`).

## Database Schema

The file upload system creates two tables:

### `managed_files`

Stores file metadata:

- `id`, `public_id`, `created_at`, `updated_at`, `deleted_at` (standard ShipQ columns)
- `filename` — the original filename
- `content_type` — MIME type (e.g., `image/png`, `application/pdf`)
- `size` — file size in bytes
- `storage_key` — the S3 object key
- `account_id` — reference to the uploading user's account

### `file_access`

Tracks access control for shared files, enabling fine-grained permission management.

## Multi-Tenancy

If you have [multi-tenancy](/guides/multi-tenancy/) configured with `[db] scope = organization_id`, the file upload system respects tenancy boundaries automatically:

- Files are scoped to the uploading user's organization
- Users in one organization cannot access files uploaded by another organization
- Generated tenancy tests verify this isolation

## TypeScript Client

The generated `shipq-files.ts` provides typed upload and download helpers for your frontend:

```ts
import { uploadFile, downloadFile, getFileMetadata } from './shipq-files';

// Upload a file
const file = document.getElementById('file-input').files[0];
const result = await uploadFile(file, { token: authToken });

// Get file metadata
const metadata = await getFileMetadata(result.id, { token: authToken });

// Download a file
const blob = await downloadFile(result.id, { token: authToken });
```

The TypeScript helpers handle multipart form encoding, authentication headers, and error handling for you.

## Testing

ShipQ generates tests for the file upload system that cover:

- **Upload flow**: uploading files with valid auth, verifying metadata is stored correctly
- **Download flow**: retrieving uploaded files
- **Auth enforcement**: verifying 401 for unauthenticated requests
- **Tenancy isolation**: verifying users can't access files across organizations (when scoping is enabled)

Run the file upload tests:

```sh
go test ./api/managed_files/spec/... -v -count=1
```

Or as part of your full test suite:

```sh
go test ./... -v
```

## Storage Backend

Under the hood, ShipQ uses the AWS SDK v2 for Go (`github.com/aws/aws-sdk-go-v2`) to communicate with S3-compatible object stores. The `filestorage` package provides:

- **Presigned URL generation** for secure uploads and downloads
- **Direct upload/download** as a fallback
- **Content-type detection** from file extensions
- **Storage key generation** using unique identifiers to prevent collisions

## Best Practices

- **Use MinIO for local development** — it's lightweight and fully S3-compatible. Start it with `shipq start minio`.
- **Never commit S3 credentials** — use environment variables or a secrets manager.
- **Set `S3_ENDPOINT` correctly** — leave it empty for real AWS S3; set it for MinIO, R2, or GCS.
- **Use presigned URLs for large files** — they offload transfer to the object store and reduce load on your server.
- **Add file type validation** in your handler code — the generated handlers accept any file type. You can customize them to restrict uploads to specific MIME types or file extensions.

## Next Steps

- **[Workers & Channels](/guides/workers/)** — Process uploaded files asynchronously with background jobs
- **[Handlers & Resources](/guides/handlers/)** — Customize generated file handlers
- **[Deployment](/guides/deployment/)** — Configure S3 credentials in production
