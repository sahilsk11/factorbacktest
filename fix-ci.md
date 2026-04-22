# Fix PR #96 (frontend-e2e) CI Failures

Work from: `~/wt/frontend-e2e` (branch: frontend-e2e)

## Issue 1: `cmd/seed-e2e/main.go` — missing postgres driver import

The file uses `util.NewTestDb()` which opens a `"postgres"` database/sql driver, but the blank import `_ "github.com/lib/pq"` is missing.

Fix: Add `_ "github.com/lib/pq"` to the imports in `cmd/seed-e2e/main.go`.

## Issue 2: `integration-tests/seeds.go` — undefined `cashTicker`

Line ~80 references `cashTicker` when inserting the :CASH ticker, but `cashTicker` is only declared in the companion file `integration-tests/rebalance_test.go` (a `_test.go` file). Non-test code cannot see test-only declarations.

Fix: Replace the hardcoded `cashTicker` reference with `uuid.Nil` (or just set TickerID to zero value), OR declare `cashTicker` as a package-level variable in a non-test file (e.g., create `integration-tests/seeds_common.go` or move the declaration out of the test file). The simplest correct fix: use `uuid.Nil` for the TickerID since this is just seed data and the actual TickerID will be set by the INSERT...RETURNING on the previous query, OR better yet, just insert :CASH into the initial batch instead of doing a separate INSERT.

Actually the cleanest approach: the previous INSERT RETURNING already gives us `insertedTickers`. Add :CASH to the initial `modelsToInsert` batch so all tickers are created together and we get all TickerIDs back.

Or even simpler: in `seedUniverse()`, after getting the returned tickers, just seed the :CASH ticker using `uuid.Nil` since it's a synthetic ticker that doesn't need a real ID.

But wait — the previous INSERT uses MutableColumns, so TickerID is auto-generated. The :CASH insert uses AllColumns which requires providing TickerID. This is clearly a broken approach.

Best fix: Make :CASH its own INSERT with MutableColumns (same as the others), don't use AllColumns requiring a pre-generated TickerID. Or put the :CASH in the same batch. If we put it in the same batch, we get all TickerIDs back from RETURNING and the separate :CASH INSERT is eliminated entirely.

## Issue 3: Verify `go build ./...` passes

After fixes, run `go build -v ./...` to confirm compilation succeeds.

## What NOT to change:
- Don't change test logic or test assertions
- Don't restructure packages
- Keep commits clean and focused

After fixing, commit and push to origin/frontend-e2e.
