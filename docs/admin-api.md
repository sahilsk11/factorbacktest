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
    "cash": 1.22,
    "positions": [{ "symbol": "INTC", "quantity": 0.764168 }]
  }'
```

Request body:

- `note` (required, non-empty string): stored on the new holdings version; used by reconcile as a baseline reset marker
- `cash` (required number, may be `0`)
- `positions` (array): each item has `symbol` (must resolve to a ticker) and `quantity` (>= 0)

Example response:

```json
{
  "versionId": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "holdings": {
    "cash": 1.22,
    "positions": [{ "symbol": "INTC", "quantity": 0.764168 }]
  }
}
```

When any holdings version for an investment has a non-empty `note`, reconcile uses the latest such version as the trade-replay baseline and only applies completed trades created strictly after that version.

## Unpause / approve investment

Clears `paused_at` so the investment is eligible for the next scheduled rebalance candidate listing. New investments are created paused and stay out of rebalance until this approval. Does **not** place orders, trigger a rebalance, call the broker, or change cash/holdings.

```bash
curl -sS -X POST "https://api.factor.trade/internal/admin/investments/<investment-id>/unpause" \
  -H "X-Admin-Api-Key: $ADMIN_API_KEY"
```

Example response:

```json
{
  "investmentId": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "paused": false,
  "alreadyActive": false
}
```

- `200` with `alreadyActive: true` if the investment was already unpaused (idempotent; no write)
- `404` if the investment does not exist
- `409` if it has ended (`end_date` set) or is in liquidation (`liquidation_requested_at` set)

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
