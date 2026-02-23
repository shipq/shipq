#!/bin/bash
set -euo pipefail

ENDPOINT="http://localhost:9000"
BUCKET="shipq-dev"
ALIAS="local"

# Wait for MinIO to be ready
for i in $(seq 1 30); do
    if curl -sf "$ENDPOINT/minio/health/live" > /dev/null 2>&1; then
        break
    fi
    if [ "$i" -eq 30 ]; then
        echo "ERROR: MinIO did not become ready in time." >&2
        exit 1
    fi
    sleep 1
done

# Set up mc alias for local MinIO
mc alias set "$ALIAS" "$ENDPOINT" minioadmin minioadmin --api S3v4

# Create bucket (ignore error if it already exists)
mc mb "$ALIAS/$BUCKET" 2>/dev/null || true

# Apply CORS configuration so browser-based uploads work from frontend dev servers
mc admin config set "$ALIAS" api cors_allow_origin="http://localhost:5173,http://localhost:4321"

# Restart is required for config changes to take effect
mc admin service restart "$ALIAS"

echo "MinIO bucket '$BUCKET' ready with CORS configured."
