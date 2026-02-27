#!/usr/bin/env bash
set -euo pipefail

# ci-test.sh — Run the full test suite.
# Services (PostgreSQL, MySQL) are provided by GitHub Actions service containers.
# Connection strings are passed via POSTGRES_TEST_URL and MYSQL_TEST_URL env vars.

mkdir -p test_results

echo "Running tests..."
go test -v ./... -tags=integration,property -count=1 | tee test_results/ci.log
