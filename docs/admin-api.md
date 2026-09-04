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
