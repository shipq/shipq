PROJECT_DIR := "$HOME/Documents/GitHub/portsql/portsql"

start-cursor:  # Launch a Terminal on MacOS and open cursor.
    osascript -e 'tell application "Terminal"' \
        -e '    activate' \
        -e '    do script "cd \"{{PROJECT_DIR}}\" && /nix/var/nix/profiles/default/bin/nix-shell --keep TMPDIR --keep TMP --keep TEMP --run \"TMPDIR=/tmp TMP=/tmp TEMP=/tmp /usr/local/bin/cursor .\""' \
        -e 'end tell'

start-dbs:  # Start all databases (MySQL, PostgreSQL, SQLite)
    cd databases && goreman start
