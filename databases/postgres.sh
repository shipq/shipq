#!/bin/bash
set -euo pipefail
if [ ! -d "$PROJECT_ROOT/databases/.postgres-data" ]; then
    mkdir -p "$PROJECT_ROOT/databases/.postgres-data"
    initdb -D "$PROJECT_ROOT/databases/.postgres-data" --username=postgres
fi
postgres -D "$PROJECT_ROOT/databases/.postgres-data"