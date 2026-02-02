PROJECT_DIR := "$HOME/Documents/GitHub/shipq/shipq"

start-cursor:  # Launch a Terminal on MacOS and open cursor.
    osascript -e 'tell application "Terminal"' \
        -e '    activate' \
        -e '    do script "cd \"{{PROJECT_DIR}}\" && /nix/var/nix/profiles/default/bin/nix-shell --keep TMPDIR --keep TMP --keep TEMP --run \"TMPDIR=/tmp TMP=/tmp TEMP=/tmp /usr/local/bin/cursor .\""' \
        -e 'end tell'

start-zed:  # Launch a Terminal on MacOS and open Zed.
    osascript -e 'tell application "Terminal"' \
        -e '    activate' \
        -e '    do script "cd \"{{PROJECT_DIR}}\" && /nix/var/nix/profiles/default/bin/nix-shell --keep TMPDIR --keep TMP --keep TEMP --run \"TMPDIR=/tmp TMP=/tmp TEMP=/tmp /usr/local/bin/zed .\""' \
        -e 'end tell'

start-dbs:  # Start all databases (MySQL, PostgreSQL, SQLite)
    cd db/databases && goreman start

test-all:  # Run all tests
    go test -v ./... -tags=integration
