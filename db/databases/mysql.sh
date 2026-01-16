# A tiny script to start a MySQL server for development.
set -euo pipefail

DATA_DIR="$PROJECT_ROOT/db/databases/.mysql-data"

# Create data directory if it doesn't exist
mkdir -p "$DATA_DIR"

# Initialize database if this is a new data directory
if [ ! -d "$DATA_DIR/mysql" ]; then
    echo "Initializing MySQL data directory..."
    mysqld --initialize-insecure --datadir="$DATA_DIR"
fi

# Remove undo transaction files
rm -f "$DATA_DIR"/undo_*

# Start MySQL in foreground
echo "Starting MySQL..."

# Ensure mysqld dies when this script exits
cleanup() {
    if [ -n "${MYSQLD_PID:-}" ] && kill -0 "$MYSQLD_PID" 2>/dev/null; then
        echo "Shutting down MySQL..."
        kill "$MYSQLD_PID" 2>/dev/null || true
        wait "$MYSQLD_PID" 2>/dev/null || true
    fi
}

trap cleanup EXIT TERM INT HUP

if [ -t 0 ]; then
    stty intr undef
    stty quit ^C
fi

mysqld --datadir="$DATA_DIR" \
  --socket="$DATA_DIR/mysql.sock" \
  --mysqlx-socket="$DATA_DIR/mysqlx.sock" \
  --console &

MYSQLD_PID=$!

# Wait for mysqld to exit
wait "$MYSQLD_PID"