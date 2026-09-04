# Admin API

Read-only operational endpoints for factorbot and on-call checks. These routes are separate from cron jobs (`CRON_SECRET` / `X-Cron-Secret`) and require a dedicated admin key.

## Setup (Fly)

Set the admin API key on the `factorbacktest` app:

```bash
fly secrets set ADMIN_API_KEY="<generate-a-long-random-secret>" -a factorbacktest
```

The key is read from the `ADMIN_API_KEY` environment variable at runtime.

## Quantity mismatch check

Compares aggregate ledger holdings across all investments to the live Alpaca broker account (same logic as `InvestmentService.reconcileAggregatePortfolio`). Read-only — no repairs or orders.

```bash
curl -sS -X POST "https://api.factor.trade/internal/admin/checks/quantity-mismatch" \
  -H "X-Admin-Api-Key: $ADMIN_API_KEY"
```

Example mismatch response:

```json
{
  "status": "MISMATCH",
  "checkedAt": "2026-09-04T10:00:00Z",
  "mismatches": [
    {
      "symbol": "INTC",
      "ledgerQty": 1.197,
      "brokerQty": 0.764,
      "delta": -0.433,
      "kind": "SHORTAGE"
    }
  ]
}
```

When ledger and broker quantities align (within existing reconciliation tolerances), `status` is `OK` and `mismatches` is empty.
