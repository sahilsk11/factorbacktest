# Integration Test Database Isolation Plan

## Problem

Currently, all integration tests share a single `postgres_test` database:
- `util.NewTestDb()` returns a hardcoded connection to `postgres_test`
- Tests use manual `cleanup*` functions to delete rows between runs
- Tests are overly conservative with transactions (begin, seed, commit, cleanup)
- Parallel test execution is impossible — tests step on each other's data
- CI creates ONE database at startup and all tests share it

## Solution

Each integration test run creates its own isolated database with a unique name, runs all tests against it, then drops it.

Database naming convention: `YYYYMMDD-XXXX` where XXXX is 4 random alphanumeric characters.
Example: `20250421-7aB3`

## Phase 1: Test Database Manager (`integration-tests/testdb.go`)

Create a new file that handles:
1. **Create**: `CREATE DATABASE "20250421-7aB3"` via the postgres admin connection
2. **Connect**: Return connection string for the new database
3. **Migrate**: Run `tools/migrations.py up <dbname>` on the new database
4. **Drop**: `DROP DATABASE "20250421-7aB3"` with `WITH (FORCE)` to disconnect sessions
5. **Cleanup**: In `TestMain()`, drop the database after all tests finish

## Phase 2: TestMain Setup (`integration-tests/main_test.go`)

Create `TestMain(m *testing.M)`:
1. Create new unique database
2. Run migrations on it
3. Replace `util.NewTestDb()` to point to this database (via env var or package-level var)
4. Run `m.Run()`
5. Drop the database in cleanup
6. If tests fail, still drop the database (or optionally keep it for debugging)

## Phase 3: CI Workflow Update (`.github/workflows/go.yml`)

Remove hardcoded `POSTGRES_DB: postgres_test` from the service container.
Use `postgres` as the default database instead.
Tests now create their own database dynamically.
Remove the `migrations.py up postgres_test` step (tests do their own migration).

## Phase 4: Remove Boilerplate

1. **Delete manual cleanup functions:**
   - `cleanupUniverse()` - no longer needed
   - `cleanupStrategies()` - no longer needed
   - `cleanupUsers()` - no longer needed
   - `cleanupRebalance()` - no longer needed
   - All `defer cleanup(db)` patterns in test files

2. **Remove conservative transaction patterns:**
   - `tx, err := db.Begin()` / `defer tx.Rollback()` / `tx.Commit()` in integration tests
   - Tests can now insert directly since they own the database

3. **Update `util.NewTestDb()`:**
   - Remove hardcoded connection string
   - Read from environment variable `FACTOR_TEST_DB` or similar
   - Remove `NewTestDb()` from `util.go` entirely and move to `integration-tests/testdb.go`

4. **Update Makefile:**
   - Remove `postgres_test` from migrate target
   - Add `test-integration` target if needed

## Files Modified
- `integration-tests/testdb.go` (new)
- `integration-tests/main_test.go` (new)
- `integration-tests/backtest_test.go`
- `integration-tests/rebalance_test.go`
- `integration-tests/seeds.go`
- `internal/util/util.go`
- `internal/repository/asset_universe.repository_test.go`
- `internal/repository/ses_email.repository_test.go`
- `.github/workflows/go.yml`
- `Makefile`

## Verification
- `go test ./integration-tests/...` runs successfully locally
- CI passes on PR
- No cleanup functions remain in integration tests
- Database is created and dropped per test run
