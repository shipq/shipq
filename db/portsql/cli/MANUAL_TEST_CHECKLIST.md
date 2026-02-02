# Manual Test Checklist: Database Setup Commands

This document provides a step-by-step checklist for manually verifying the `shipq db start` and `shipq db setup` commands.

## Prerequisites

- [ ] PostgreSQL installed (`brew install postgresql` on macOS)
- [ ] MySQL installed (`brew install mysql` on macOS)
- [ ] A ShipQ project directory with `shipq.ini`

## Test 1: Start Postgres Server

### Setup
```bash
# Navigate to your project directory
cd /path/to/your/project

# Ensure shipq.ini exists with a postgres URL
cat shipq.ini
# Should contain: [db]
#                 url = postgres://localhost/yourdb
```

### Steps
1. [ ] Run `shipq db start postgres`
2. [ ] Verify output shows "Initializing Postgres data directory" (first run only)
3. [ ] Verify output shows "Starting Postgres server..."
4. [ ] Verify data directory path is shown
5. [ ] Verify connection hint is shown
6. [ ] In another terminal, connect: `psql -h localhost -U postgres`
7. [ ] Press Ctrl+C to stop the server
8. [ ] Verify server shuts down gracefully

### Expected Data Directory
```
db/databases/.postgres-data/
```

---

## Test 2: Start MySQL Server

### Setup
```bash
# Ensure shipq.ini exists with a mysql URL
cat shipq.ini
# Should contain: [db]
#                 url = mysql://root@localhost/yourdb
```

### Steps
1. [ ] Run `shipq db start mysql`
2. [ ] Verify output shows "Initializing MySQL data directory" (first run only)
3. [ ] Verify output shows "Starting MySQL server..."
4. [ ] Verify data directory and socket paths are shown
5. [ ] In another terminal, connect: `mysql -u root --socket=db/databases/.mysql-data/mysql.sock`
6. [ ] Press Ctrl+C to stop the server
7. [ ] Verify server shuts down gracefully

### Expected Data Directory
```
db/databases/.mysql-data/
db/databases/.mysql-data/mysql.sock
db/databases/.mysql-data/mysqlx.sock
```

---

## Test 3: Database Setup (Postgres)

### Setup
```bash
# Start Postgres in one terminal
shipq db start postgres
```

### Steps (in another terminal)
1. [ ] Run `shipq db setup`
2. [ ] Verify output shows project name
3. [ ] Verify output shows dev database name (e.g., `yourproject`)
4. [ ] Verify output shows test database name (e.g., `yourproject_test`)
5. [ ] Verify "Created database" or "Database already exists" messages
6. [ ] Verify connection URLs are printed

### Verify Databases Exist
```bash
psql -h localhost -U postgres -c "\l" | grep yourproject
# Should show both yourproject and yourproject_test
```

---

## Test 4: Database Setup (MySQL)

### Setup
```bash
# Start MySQL in one terminal
shipq db start mysql

# Update shipq.ini for MySQL
# [db]
# url = mysql://root@localhost/yourdb
```

### Steps (in another terminal)
1. [ ] Run `shipq db setup`
2. [ ] Verify output shows project name
3. [ ] Verify output shows dev database name
4. [ ] Verify output shows test database name
5. [ ] Verify "Ensured database exists" messages
6. [ ] Verify connection URLs are printed

### Verify Databases Exist
```bash
mysql -u root --socket=db/databases/.mysql-data/mysql.sock -e "SHOW DATABASES" | grep yourproject
# Should show both yourproject and yourproject_test
```

---

## Test 5: Idempotency

1. [ ] Run `shipq db setup` twice in a row
2. [ ] Verify second run shows "already exists" messages
3. [ ] Verify no errors occur
4. [ ] Verify databases are not modified

---

## Test 6: Custom Database Names

### Setup
```bash
# Update shipq.ini with custom names
cat > shipq.ini << EOF
[db]
url = postgres://localhost/myapp
name = custom_base
dev_name = my_dev_db
test_name = my_test_db
EOF
```

### Steps
1. [ ] Run `shipq db setup`
2. [ ] Verify output shows `my_dev_db` as dev database
3. [ ] Verify output shows `my_test_db` as test database
4. [ ] Verify databases are created with custom names

---

## Test 7: Safety Checks

### Non-localhost URL
```bash
# Update shipq.ini with remote URL
cat > shipq.ini << EOF
[db]
url = postgres://db.example.com/myapp
EOF

shipq db setup
# Should fail with "only supports localhost" error
```

1. [ ] Verify command fails for remote hosts
2. [ ] Verify error message mentions localhost requirement

### Server Not Running
```bash
# Stop any running database servers
# Then run:
shipq db setup
# Should fail with connection error
```

1. [ ] Verify command fails when server is not running
2. [ ] Verify error message suggests `shipq db start`

---

## Test 8: SQLite Setup

### Setup
```bash
cat > shipq.ini << EOF
[db]
url = sqlite://myapp.db
EOF
```

### Steps
1. [ ] Run `shipq db setup`
2. [ ] Verify output mentions SQLite databases are created automatically
3. [ ] Verify expected file paths are shown
4. [ ] Verify no errors occur

---

## Test 9: Data Persistence

1. [ ] Start Postgres: `shipq db start postgres`
2. [ ] Create a test table: `psql -h localhost -U postgres -c "CREATE TABLE test(id int);" yourproject`
3. [ ] Stop the server (Ctrl+C)
4. [ ] Start the server again: `shipq db start postgres`
5. [ ] Verify table exists: `psql -h localhost -U postgres -c "\dt" yourproject`

---

## Cleanup

```bash
# Remove data directories if needed
rm -rf db/databases/.postgres-data
rm -rf db/databases/.mysql-data
```

---

## Notes

- These tests should be run on a development machine, not in CI
- Ensure no other database servers are running on the default ports
- The data directories are gitignored by default
