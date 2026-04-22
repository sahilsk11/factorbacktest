# Playwright Database Seeding Plan

## Problem

Playwright specs currently hit an empty database and assert only on
empty-state UI. We need seeded data — tickers, asset universes, users,
strategies, investments, price history — without duplicating the
seeding that already lives in `integration-tests/seeds.go`.

Additionally, every Playwright test must own its own BE + DB so it can
choose its own seed and cannot observe another test's writes.

## Principles

These constrain every decision below.

1. **Primitives, not fixtures.** One function per model, takes an
  `Opts` struct, inserts one row, returns the model. No dependency
   graph, no registry, no topological solver.
2. **Test owns its setup.** Go integration tests define their seed as
  an inline closure in the test function, calling primitives
   top-to-bottom. Two tests that need the same setup duplicate the
   closure — no shared helper yet.
3. **Named Playwright seeds live in one file, in `cmd/test-api/`.** A
  plain `map[string]func(*sql.DB)`. Each value composes primitives
   top-to-bottom. This file exists only because Playwright is a
   different process and cannot call Go directly.
4. **No HTTP seeding surface.** The Playwright side chooses a seed via
  CLI flag on the `test-api` binary. Seeding runs once at startup,
   before HTTP serving. No `/__test__/`* admin router, no reset
   endpoint, no per-primitive HTTP exposure.
5. **No reset logic.** Each test gets a fresh database by construction
  (spawns its own `test-api`, which creates its own DB). Nothing to
   reset.
6. **Per-test BE + DB isolation.** Every Playwright test spawns its
  own `test-api` process with a unique port. The FE is shared — it's
   stateless, and browser-context isolation is already per-test. The
   per-test BE port is plumbed into the browser via a `context.route`
   rewrite.

## Layout

```
internal/testseed/
  primitives.go            # one Create<Model> func per model + 1 bulk helper
  data/prices_2020.csv     # moved from integration-tests/sample_prices_2020.csv

cmd/test-api/
  main.go                  # parses -seed flag; if set, runs seeds[name](db) after
                           # DB creation and before StartApi
  seeds.go                 # var seeds = map[string]func(*sql.DB){ ... } + seedXxx funcs

integration-tests/
  backtest_test.go         # inline `seed := func(db) {...}` using testseed primitives
  rebalance_test.go        # same
  (seeds.go + sample_prices_2020.csv deleted)

frontend/e2e/
  fixtures.ts              # extended: adds `backend` fixture + `seedName` option;
                           # diagnostics fixture reads BE log from backend fixture
  smoke.spec.ts            # unchanged; doesn't request seed, gets an empty BE
  backtest_flow.spec.ts    # new; uses backend fixture with seedName = 'investment_basic'

frontend/
  playwright.config.ts     # drops the BE webServer entry, keeps FE webServer,
                           # keeps FB_TEST_BE_PORT for FE-build URL baking,
                           # adds globalSetup to pre-build the test-api binary

frontend/e2e/
  global-setup.ts          # new; compiles ./cmd/test-api to /tmp/fb-test-api once
                           # per run so fixture spawns are fast (~100ms vs ~2s)
```

## `internal/testseed/primitives.go`

One function per model, exhaustive for the tables tests touch. Each
takes an `Opts` struct with sensible defaults and returns the inserted
`model.X` (so the caller has the generated UUID). Panics on SQL error
— this is test-only code and panics give the clearest traceback.

```go
type TickerOpts struct{ Symbol, Name string }
func CreateTicker(db *sql.DB, opts TickerOpts) model.Ticker

type AssetUniverseOpts struct{ Name string }
func CreateAssetUniverse(db *sql.DB, opts AssetUniverseOpts) model.AssetUniverse

func CreateAssetUniverseTicker(db *sql.DB, universeID, tickerID uuid.UUID) model.AssetUniverseTicker

type UserAccountOpts struct {
    Email, FirstName, LastName string
    Provider                   model.UserAccountProviderType
}
func CreateUserAccount(db *sql.DB, opts UserAccountOpts) model.UserAccount

type StrategyOpts struct {
    Name, FactorExpression, AssetUniverse, RebalanceInterval string
    NumAssets                                                int32
    UserAccountID                                            uuid.UUID
}
func CreateStrategy(db *sql.DB, opts StrategyOpts) model.Strategy

type InvestmentOpts struct {
    StrategyID, UserAccountID uuid.UUID
    AmountDollars             int32
    StartDate                 time.Time   // zero value → time.Now()
}
func CreateInvestment(db *sql.DB, opts InvestmentOpts) model.Investment

func CreateInvestmentHoldingsVersion(db *sql.DB, investmentID uuid.UUID) model.InvestmentHoldingsVersion

type InvestmentHoldingOpts struct {
    VersionID, TickerID uuid.UUID
    Quantity            decimal.Decimal
}
func CreateInvestmentHolding(db *sql.DB, opts InvestmentHoldingOpts) model.InvestmentHoldings

func LookupTickerBySymbol(db *sql.DB, symbol string) model.Ticker

//go:embed data/prices_2020.csv
var prices2020CSV []byte
func InsertPrices2020(db *sql.DB)   // one bulk helper; 1013 rows
```

## `cmd/test-api/seeds.go`

```go
package main

import (
    "database/sql"
    "factorbacktest/internal/testseed"
    "github.com/google/uuid"
    "github.com/shopspring/decimal"
)

var seeds = map[string]func(*sql.DB){
    "investment_basic": seedInvestmentBasic,
    "prices_only":      seedPricesOnly,
}

func seedInvestmentBasic(db *sql.DB) {
    aapl := testseed.CreateTicker(db, testseed.TickerOpts{Symbol: "AAPL", Name: "Apple"})
    goog := testseed.CreateTicker(db, testseed.TickerOpts{Symbol: "GOOG", Name: "Google"})
    meta := testseed.CreateTicker(db, testseed.TickerOpts{Symbol: "META", Name: "Meta"})
    universe := testseed.CreateAssetUniverse(db, testseed.AssetUniverseOpts{Name: "SPY_TOP_80"})
    for _, id := range []uuid.UUID{aapl.TickerID, goog.TickerID, meta.TickerID} {
        testseed.CreateAssetUniverseTicker(db, universe.AssetUniverseID, id)
    }
    testseed.InsertPrices2020(db)
    user := testseed.CreateUserAccount(db, testseed.UserAccountOpts{Email: "test@gmail.com"})
    strategy := testseed.CreateStrategy(db, testseed.StrategyOpts{
        Name: "test_strategy", UserAccountID: user.UserAccountID,
        AssetUniverse: "SPY_TOP_80", NumAssets: 3, RebalanceInterval: "MONTHLY",
        FactorExpression: "pricePercentChange(\n  nDaysAgo(7),\n  currentDate\n)",
    })
    inv := testseed.CreateInvestment(db, testseed.InvestmentOpts{
        StrategyID: strategy.StrategyID, UserAccountID: user.UserAccountID, AmountDollars: 100,
    })
    hv := testseed.CreateInvestmentHoldingsVersion(db, inv.InvestmentID)
    cash := testseed.LookupTickerBySymbol(db, ":CASH")
    testseed.CreateInvestmentHolding(db, testseed.InvestmentHoldingOpts{
        VersionID: hv.InvestmentHoldingsVersionID, TickerID: cash.TickerID,
        Quantity:  decimal.NewFromInt(100),
    })
}

func seedPricesOnly(db *sql.DB) {
    testseed.CreateTicker(db, testseed.TickerOpts{Symbol: "AAPL", Name: "Apple"})
    testseed.CreateTicker(db, testseed.TickerOpts{Symbol: "GOOG", Name: "Google"})
    testseed.CreateTicker(db, testseed.TickerOpts{Symbol: "META", Name: "Meta"})
    testseed.InsertPrices2020(db)
}
```

New seed = one map entry + one function. Duplication between the two
functions (the three tickers) is tolerated — same rule as integration
tests.

## `cmd/test-api/main.go` changes

Narrow patch on top of master. Add a `-seed` flag. After
`createTestDbManager` returns, if `-seed` was provided, dispatch to
`seeds[name]`. Then call `StartApi` as today.

```go
var seedName = flag.String("seed", "", "name of seed to apply at startup")

func main() {
    flag.Parse()
    // ... existing PORT parsing, createTestDbManager, secrets, InitializeDependencies ...
    if *seedName != "" {
        fn, ok := seeds[*seedName]
        if !ok {
            log.Fatalf("unknown seed %q; known: %v", *seedName, sortedSeedNames())
        }
        fn(testDbManager.DB())
    }
    // ... existing StartApi call ...
}
```

No admin router, no `/__test__/*` routes.

## Integration tests

Each test defines its seed as an inline closure. Duplication between
tests is fine.

```go
func Test_rebalanceFlow(t *testing.T) {
    manager, err := NewTestDbManager()
    require.NoError(t, err)
    defer manager.Close()

    server, err := NewTestServer(manager)
    require.NoError(t, err)
    defer server.Stop()

    db := manager.DB()

    seed := func(db *sql.DB) {
        aapl := testseed.CreateTicker(db, testseed.TickerOpts{Symbol: "AAPL", Name: "Apple"})
        goog := testseed.CreateTicker(db, testseed.TickerOpts{Symbol: "GOOG", Name: "Google"})
        meta := testseed.CreateTicker(db, testseed.TickerOpts{Symbol: "META", Name: "Meta"})
        universe := testseed.CreateAssetUniverse(db, testseed.AssetUniverseOpts{Name: "SPY_TOP_80"})
        for _, id := range []uuid.UUID{aapl.TickerID, goog.TickerID, meta.TickerID} {
            testseed.CreateAssetUniverseTicker(db, universe.AssetUniverseID, id)
        }
        testseed.InsertPrices2020(db)
        user := testseed.CreateUserAccount(db, testseed.UserAccountOpts{Email: "test@gmail.com"})
        // ... strategy, investment, hv, cash holding ...
    }
    seed(db)

    // ... call /rebalance, assert on trades ...
}
```

`integration-tests/seeds.go` and `sample_prices_2020.csv` are deleted.

## Per-test BE isolation in Playwright

This is the biggest design decision and the reason the existing
`fixtures.ts` gets reshaped.

### Target model

- FE: built ONCE, served on a fixed port picked by `playwright.config.ts`
at startup. This is how master works today. Unchanged.
- BE: NOT launched by `playwright.config.ts`. Each Playwright test
spawns its own `test-api` via a test-scoped fixture.
- Browser-to-BE traffic: the FE is built with a baked-in
`REACT_APP_API_PORT = FB_TEST_BE_PORT` (same as master). The backend
fixture installs `context.route(\`[[[http://localhost:${FB_TEST_BE_PORT}/**\`)`](http://localhost:${FB_TEST_BE_PORT}/**\`)`](http://localhost:${FB_TEST_BE_PORT}/**\`)`](http://localhost:${FB_TEST_BE_PORT}/**\`)`](http://localhost:${FB_TEST_BE_PORT}/**\`)`](http://localhost:${FB_TEST_BE_PORT}/**\`)`](http://localhost:${FB_TEST_BE_PORT}/**\`)`](http://localhost:${FB_TEST_BE_PORT}/**\`)`)))
to rewrite those URLs to the test's actual BE port. Nothing else
about the FE changes.
- Seed: each spec declares `test.use({ seedName: 'investment_basic' })`.
The fixture passes `-seed=<name>` when spawning. Specs that omit
`seedName` get an unseeded BE (smoke tests fall in this bucket).

### Performance

A cold `go run ./cmd/test-api` spawn takes ~2-3s. Across dozens of
tests that adds up. Mitigation: `global-setup.ts` runs once per
Playwright run and compiles the binary to `/tmp/fb-test-api`. The
fixture spawns that binary directly, so per-test startup is
~100–300ms (binary launch + Postgres `CREATE DATABASE` + migrations).

### `frontend/e2e/global-setup.ts` (new)

```ts
import { execFileSync } from 'child_process';
import path from 'path';

export default async function globalSetup() {
  const repoRoot = path.resolve(__dirname, '../..');
  const binOut = process.env.FB_TEST_API_BIN ?? '/tmp/fb-test-api';
  process.env.FB_TEST_API_BIN = binOut;
  execFileSync(
    'go',
    ['build', '-o', binOut, './cmd/test-api'],
    { cwd: repoRoot, stdio: 'inherit' },
  );
}
```

Set `globalSetup: require.resolve('./e2e/global-setup')` in the config.

### `frontend/playwright.config.ts` changes (narrow)

1. Remove the BE entry from `webServer: [...]`. Keep the FE entry.
2. Keep the free-port picking. `FB_TEST_BE_PORT` is still needed because
  the FE is built with it.
3. Drop `FB_TEST_BE_LOG` and the `tee` wiring — each test writes its
  own log file, path derived from its per-test BE port.
4. Add `globalSetup`.
5. Leave `fullyParallel: true` alone — per-test BE makes this safe.

### `frontend/e2e/fixtures.ts` changes

Extend the existing file. The diagnostics fixture stays; it just
reads the per-test BE log from the `backend` fixture instead of from
a shared `FB_TEST_BE_LOG` with byte offsets.

New surface:

```ts
export type SeedName = '' | 'investment_basic' | 'prices_only';

export type BackendFixture = {
  apiUrl: string;
  port: number;
  logPath: string;
};

type FixtureOptions = { seedName: SeedName };

export const test = base.extend<
  FixtureOptions & { backend: BackendFixture; diagnostics: Diagnostics }
>({
  seedName: ['', { option: true }],

  backend: async ({ seedName, context }, use, testInfo) => {
    const builtInPort = Number(process.env.FB_TEST_BE_PORT);
    const fePort      = Number(process.env.FB_TEST_FE_PORT);
    const binPath     = process.env.FB_TEST_API_BIN ?? '/tmp/fb-test-api';

    const port = await getFreePort();
    const logPath = `/tmp/fb-test-be-${port}.log`;
    const logFd = fs.openSync(logPath, 'w');

    const args = seedName ? ['-seed', seedName] : [];
    const child = spawn(binPath, args, {
      cwd: path.resolve(__dirname, '../..'),
      env: {
        ...process.env,
        PORT: String(port),
        ALPHA_ENV: 'test',
        EXTRA_ALLOWED_ORIGINS: `http://localhost:${fePort}`,
      },
      stdio: ['ignore', logFd, logFd],
    });

    await waitForHttp(`http://localhost:${port}/`, { timeoutMs: 30_000 });

    await context.route(
      `http://localhost:${builtInPort}/**`,
      async (route) => {
        const rewritten = route.request().url().replace(
          `localhost:${builtInPort}`,
          `localhost:${port}`,
        );
        await route.continue({ url: rewritten });
      },
    );

    await use({ apiUrl: `http://localhost:${port}`, port, logPath });

    child.kill('SIGTERM');
    await new Promise((r) => child.once('exit', r));
    fs.closeSync(logFd);
    // logPath is cleaned up below by the diagnostics fixture only on
    // passing tests; on failure it's attached to the test report and
    // left for the user.
  },

  // diagnostics depends on backend so teardown order is:
  //   diagnostics.teardown (reads backend.logPath) -> backend.teardown
  diagnostics: [
    async ({ page, backend }, use, testInfo) => {
      // ... existing console/pageerror/request/response listeners ...
      await use(diag);
      if (testInfo.status === testInfo.expectedStatus) {
        try { fs.unlinkSync(backend.logPath); } catch {}
        return;
      }
      const backendLogSlice = fs.readFileSync(backend.logPath, 'utf8');
      // ... existing SUMMARY.md generation ...
    },
    { auto: true },
  ],
});
```

Deleted from `fixtures.ts`: `currentBackendLogSize`,
`readBackendLogSlice`, and the `beLogOffset` field on `Diagnostics`.
Per-test logs make offset tracking obsolete.

### Spec usage

`smoke.spec.ts` doesn't change — it imports `test, expect` from
`./fixtures` as today. It gets a BE per test (unseeded). Cost: ~300ms
per smoke test.

```ts
// frontend/e2e/backtest_flow.spec.ts  (new)
import { test, expect } from './fixtures';

test.use({ seedName: 'investment_basic' });

test('backtest chart renders with seeded price data', async ({ page }) => {
  await page.goto('/backtest');
  await expect(page.locator('#backtest-chart canvas')).toBeVisible();
  // ... assertions that exercise the seeded rows ...
});
```

## File changes

### Create

- `internal/testseed/primitives.go`
- `internal/testseed/data/prices_2020.csv` (move from integration-tests/)
- `cmd/test-api/seeds.go`
- `frontend/e2e/global-setup.ts`
- `frontend/e2e/backtest_flow.spec.ts`

### Modify

- `cmd/test-api/main.go` — add `-seed` flag + dispatch, between DB
creation and `StartApi`. Keep the rest untouched.
- `frontend/playwright.config.ts` — drop BE webServer entry, drop
`FB_TEST_BE_LOG`, add `globalSetup`. Keep free-port picking, FE
webServer, CORS env vars.
- `frontend/e2e/fixtures.ts` — add `seedName` option, add `backend`
fixture, rewire `diagnostics` to depend on `backend` for log
access. Remove dead shared-log helpers.
- `integration-tests/backtest_test.go` — inline `seed := func(db) {...}`
using `testseed` primitives.
- `integration-tests/rebalance_test.go` — same; delete the existing
`seedInvestment` function.

### Delete

- `integration-tests/seeds.go`
- `integration-tests/sample_prices_2020.csv`

## Verification

- `go build ./...` and `go vet ./...` pass.
- `go test ./integration-tests/ -run Test_backtestFlow` passes against
local Postgres on `:5440`.
- `go run ./cmd/test-api -seed=investment_basic` starts, prints the
seeded rows on the BE log, and serves HTTP.
- `go run ./cmd/test-api -seed=bogus` exits non-zero with a message
listing known seeds.
- `cd frontend && npx playwright test`:
  - `smoke.spec.ts` passes (BE spawned per test, unseeded).
  - `backtest_flow.spec.ts` passes (BE spawned with
  `-seed=investment_basic`; FE→BE requests are routed to the
  per-test port).
- CI (`.github/workflows/e2e.yml`) passes unchanged — it runs
`npx playwright test` against the same setup and uploads
`frontend/test-results/` artifacts (SUMMARY.md, backend.log,
screenshots, traces) on failure.

## Notes

- The only spec that touches `smoke.spec.ts` is "import from
fixtures.ts" — which it already does. Nothing in the smoke spec
changes. Same for `fixtures.ts`'s heuristic failure analysis — it
keeps working because the per-test BE log path is still passed in,
just through the fixture instead of an env var.
- `EXTRA_ALLOWED_ORIGINS` mechanism (master, `api/api.go`) keeps
working. The backend fixture sets it per-spawn to the FE origin.
- Run-local `.gitignore` entry for `/tmp/fb-test-*` is unnecessary —
they live outside the repo.

