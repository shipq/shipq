#!/usr/bin/env bash
set -euo pipefail

# ci-test.sh — Start all services and run the full test suite.
# Called from inside a nix-shell by the CI workflow.

export PROJECT_ROOT="$PWD"
export PORT=8080
export GO_ENV=development
export DATABASE_URL=mysql://root@localhost:3306/shipq
export COOKIE_SECRET=supersecret

cd db/databases && goreman start &
GOREMAN_PID=$!
cd "$PROJECT_ROOT"

cleanup() {
  echo "Shutting down services..."
  kill "$GOREMAN_PID" 2>/dev/null || true
  wait "$GOREMAN_PID" 2>/dev/null || true
}
trap cleanup EXIT

wait_for() {
  local name="$1" timeout="$2" cmd="$3"
  echo "Waiting for ${name}..."
  for i in $(seq 1 "$timeout"); do
    if eval "$cmd" 2>/dev/null; then
      echo "${name} is ready."
      return 0
    fi
    sleep 1
  done
  echo "ERROR: ${name} did not become ready within ${timeout}s." >&2
  exit 1
}

wait_for "PostgreSQL" 60 "pg_isready -h /tmp -U postgres"

MYSQL_SOCKET="$PROJECT_ROOT/db/databases/.mysql-data/mysql.sock"
wait_for "MySQL" 60 "mysqladmin ping --socket='$MYSQL_SOCKET' --user=root --silent"

wait_for "Valkey" 30 "valkey-cli ping | grep -q PONG"

echo "Running tests..."
go test -v ./... -tags=integration,property -count=1
