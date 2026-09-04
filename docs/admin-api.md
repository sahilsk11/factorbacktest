# Admin API

Operational endpoints for factorbot and on-call use. Separate from cron routes (`CRON_SECRET` / `X-Cron-Secret`).

## Setup (Fly)

```bash
fly secrets set ADMIN_API_KEY="<generate-a-long-random-secret>" -a factorbacktest
```

All admin routes require header `X-Admin-Api-Key: $ADMIN_API_KEY`.

## Reconcile (read-only)

Runs existing `InvestmentService.Reconcile` checks and returns structured issues as JSON. No ledger writes or Alpaca orders.

```bash
curl -sS -X POST "https://api.factor.trade/internal/admin/reconcile" \
  -H "X-Admin-Api-Key: $ADMIN_API_KEY"
```

Example response with broker/ledger drift:

```json
{
  "status": "ISSUES",
  "checkedAt": "2026-09-04T10:00:00Z",
  "issues": [
    {
      "message": "alpaca account holding insufficient INTC: aggregate portfolio 1.197000 vs alpaca 0.764000 (-0.433000)"
    }
  ]
}
```

When no issues are found, `status` is `OK` and `issues` is empty.

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

## Ops triggers

These reuse the same handlers as `/internal/cron/*` (cron routes remain unchanged).

```bash
curl -sS -X POST "https://api.factor.trade/internal/admin/updatePrices" \
  -H "X-Admin-Api-Key: $ADMIN_API_KEY"

curl -sS -X POST "https://api.factor.trade/internal/admin/rebalance" \
  -H "X-Admin-Api-Key: $ADMIN_API_KEY"

curl -sS -X POST "https://api.factor.trade/internal/admin/updateOrders" \
  -H "X-Admin-Api-Key: $ADMIN_API_KEY"
```
