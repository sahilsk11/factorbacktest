# Admin API

Operational endpoints for factorbot and on-call use. Separate from cron routes (`CRON_SECRET` / `X-Cron-Secret`).

## Setup (Fly)

```bash
fly secrets set ADMIN_API_KEY="<generate-a-long-random-secret>" -a factorbacktest
```

All admin routes require header `X-Admin-Api-Key: $ADMIN_API_KEY`.

## Reconcile (read-only)

Runs `InvestmentService.RunReconcile` and returns structured issues as JSON. No ledger writes, holdings mutations, or Alpaca orders.

Aggregate comparison uses the **full union** of symbols: latest ledger holdings summed across **all** investments (including paused and closed) versus Alpaca positions, plus cash totals.

```bash
curl -sS -X POST "https://api.factor.trade/internal/admin/reconcile" \
  -H "X-Admin-Api-Key: $ADMIN_API_KEY"
```

`status` is one of:

- `OK` — no issues
- `DUST_ONLY` — every issue has `severity: "DUST"` (absolute dollar impact strictly below $50)
- `DRIFT` — at least one `severity: "DRIFT"` issue (including per-investment checks)

Clients that treat any non-`OK` status as "look at this" stay valid. Known leftover fractional dust (for example closed investments `947b6eff…` and `2bf80abe…`) is reported as `DUST` instead of passing silently.

Issue types:

| `type` | Meaning |
| --- | --- |
| `PHANTOM_POSITION` | Ledger has a nonzero qty; Alpaca does not |
| `UNTRACKED_POSITION` | Alpaca has a nonzero qty; ledger does not |
| `QUANTITY_MISMATCH` | Both sides hold the symbol and `abs(diff) > 1e-6` shares |
| `CASH_MISMATCH` | `abs(alpaca cash - ledger cash) > $0.50` |
| `STALE_PRICE` | A held symbol's latest adjusted price is older than 5 trading days (or missing) |

Each aggregate issue includes `symbol`, `ledgerQty`, `alpacaQty`, `diff` (Alpaca − ledger), and `dollarValue` when a price is available. `severity` is `DUST` when the absolute dollar impact is below $50, otherwise `DRIFT`. Missing prices on a qty mismatch are treated as `DRIFT`. `STALE_PRICE` severity uses the position's approximate market value.

Example response with broker/ledger drift:

```json
{
  "status": "DRIFT",
  "checkedAt": "2026-09-04T10:00:00Z",
  "issues": [
    {
      "type": "QUANTITY_MISMATCH",
      "severity": "DRIFT",
      "symbol": "INTC",
      "ledgerQty": 1.197,
      "alpacaQty": 0.764,
      "diff": -0.433,
      "dollarValue": -86.6,
      "message": "QUANTITY_MISMATCH INTC: ledger 1.197 vs alpaca 0.764 (diff -0.433) (~$86.60)"
    }
  ]
}
```

When no issues are found, `status` is `OK` and `issues` is empty.

The daily `/internal/cron/rebalance` job (and the admin rebalance trigger) runs this reconcile first. `DRIFT` issues are logged and returned on the rebalance response as `driftIssues`. Reconcile never blocks or changes rebalancing, and never mutates holdings.

## Replace holdings (admin override)

Appends a new holdings version with a manual baseline note. Writes a full cash + positions snapshot for that version. Does not mutate prior versions, create trades, or call Alpaca.

```bash
curl -sS -X POST "https://api.factor.trade/internal/admin/investments/<investment-id>/holdings" \
  -H "X-Admin-Api-Key: $ADMIN_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "note": "Broker ledger correction for INTC",
    "actor": "sahil",
    "cash": 1.22,
    "positions": [{ "symbol": "INTC", "quantity": 0.764168 }]
  }'
```

Request body:

- `note` (required, non-empty string): stored on the new holdings version; used by reconcile as a baseline reset marker
- `actor` (optional; defaults to `admin`)
- `cash` (required number, may be `0`)
- `positions` (array): each item has `symbol` (must resolve to a ticker) and `quantity` (>= 0)

The response includes `before` and `holdings` (the after snapshot), plus `auditId`. Every successful replace also inserts an `admin_mutation_audit` row (`kind=ledger_adjust`).

When any holdings version for an investment has a non-empty `note`, reconcile uses the latest such version as the trade-replay baseline and only applies completed trades created strictly after that version.

## Write off ledger dust (zero-out convenience)

Same mechanism as replace: appends a noted holdings version, no Alpaca call, no fake trades. Starts from the current snapshot, sets the listed symbols to zero, and keeps cash unless `cash` is sent.

Use this for **PHANTOM_POSITION** leftover on ended investments (shares the ledger still shows, Alpaca does not). Do **not** credit cash — those shares were never at the broker, so adding sale proceeds would create a cash mismatch.

```bash
curl -sS -X POST "https://api.factor.trade/internal/admin/investments/<investment-id>/holdings/adjust" \
  -H "X-Admin-Api-Key: $ADMIN_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "note": "write off phantom fractional dust after liquidation",
    "actor": "sahil",
    "zeroOutSymbols": ["VST", "GEV", "NSC"]
  }'
```

- `note` required
- `zeroOutSymbols` required (explicit list)
- `cash` optional; omit to keep the current cash row
- Symbols not currently held are returned in `skippedSymbols` (the call still succeeds if at least one symbol was actually removed)

The stored version note is prefixed with `[manual-adjustment] actor=... source=admin-api`.

## Sell untracked Alpaca dust (optional, dry-run by default)

Sells **explicit** Alpaca symbols that the ledger does not track (`UNTRACKED_POSITION`). Does not create `investment_trade` / `trade_order` rows. Uses the app's existing Alpaca client (`PlaceOrder` limit sell at 95% of current price, same as rebalance sells).

**Dry-run is the default.** Execution requires all of: `dryRun=false`, `confirm=true`, and a non-empty `note`.

```bash
# preview (default)
curl -sS -X POST "https://api.factor.trade/internal/admin/broker/dust-sells" \
  -H "X-Admin-Api-Key: $ADMIN_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "symbols": ["XYZ", "ABC"],
    "note": "preview untracked dust"
  }'

# execute (only after reviewing the dry-run)
curl -sS -X POST "https://api.factor.trade/internal/admin/broker/dust-sells" \
  -H "X-Admin-Api-Key: $ADMIN_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "symbols": ["XYZ", "ABC"],
    "note": "sell untracked alpaca dust",
    "actor": "sahil",
    "dryRun": false,
    "confirm": true,
    "maxDollarsPerPosition": 50
  }'
```

Safety rails (any failure refuses the whole request, nothing is sold):

- Symbols must be listed explicitly; there is no sell-everything default
- Symbol must currently be held at Alpaca
- Symbol must **not** be held in any non-ended investment ledger
- Symbol must **not** appear on any investment ledger, including ended ones (otherwise it is not UNTRACKED; write off the ledger first)
- Per-position estimated value must be **below** the cap (default and hard ceiling **$50**; request may set a lower cap, never higher)
- Execute also refuses when the market is closed

This tool does not liquidate live investments, does not sell DRIFT-sized positions, and does not invent ledger fills.

## What this does not do

- Does not call Alpaca during ledger replace/adjust
- Does not fabricate `trade_order` or `investment_trade` rows
- Does not automatically sell all untracked positions
- Does not touch production by itself; these are authenticated admin POSTs

Audit trail: ledger mutations reuse `investment_holdings_version.note` (reconcile baseline) and write `admin_mutation_audit` (actor, kind, note, before/after JSON). Executed broker dust sells write `admin_mutation_audit` only (`kind=broker_dust_sell`). Dry-runs are not persisted.

## Ops triggers

These reuse the same handlers as `/internal/cron/*` (cron routes remain unchanged).

```bash
curl -sS -X POST "https://api.factor.trade/internal/admin/updatePrices" \
  -H "X-Admin-Api-Key: $ADMIN_API_KEY"

curl -sS -X POST "https://api.factor.trade/internal/admin/rebalance" \
  -H "X-Admin-Api-Key: $ADMIN_API_KEY"

Rebalance responses include `"success": "true"` and a `driftIssues` array (possibly empty) from the pre-rebalance reconcile. Rebalance still proceeds when that array is non-empty.

curl -sS -X POST "https://api.factor.trade/internal/admin/updateOrders" \
  -H "X-Admin-Api-Key: $ADMIN_API_KEY"
```
