# Async Backtest with Progress Tracking

## Problem

The current `POST /backtest` endpoint is synchronous. The Lambda runs for the entire duration of the backtest (potentially minutes) before returning a response. The frontend shows a spinning icon with no feedback on progress.

Users have no visibility into:
- Whether the request is being processed
- Which stage is currently running
- How far along the backtest is
- Whether something has gone wrong

## Goal

Enable a progressive loading UX on the frontend by exposing backtest job state through a job/status polling model.

---

## Backend Changes

### 1. Database Migration

**New table: `backtest_job`** — ephemeral job state for async backtest tracking

```sql
CREATE TYPE backtest_job_status AS ENUM ('pending', 'running', 'completed', 'failed');

CREATE TABLE backtest_job (
    backtest_job_id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    strategy_id uuid NOT NULL REFERENCES strategy(strategy_id),
    status backtest_job_status NOT NULL DEFAULT 'pending',
    current_stage text,
    progress_pct int NOT NULL DEFAULT 0,
    result jsonb,
    error_message text,
    created_at timestamptz NOT NULL DEFAULT NOW(),
    updated_at timestamptz NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_backtest_job_strategy_id ON backtest_job(strategy_id);
CREATE INDEX idx_backtest_job_status ON backtest_job(status);
```

### 2. New API Endpoints

**`POST /backtest/async`** — Start async backtest job
- Request: Same as `POST /backtest`
- Response: `{ backtestJobId: uuid, strategyId: uuid }`
- Behavior: Creates Strategy + BacktestJob (status='running', stage='initializing'), returns immediately, runs backtest in goroutine

**`GET /backtest/:backtestJobId/status`** — Poll job status
- Response:
  ```json
  {
    "backtestJobId": "uuid",
    "status": "running" | "completed" | "failed",
    "currentStage": "calculating_factor_scores",
    "progressPct": 45,
    "errorMessage": null
  }
  ```
- When status='completed': includes `result` object with full backtest response

### 3. Stage Transitions

The backtest handler will update job progress at each major span:

| Stage Key | Human Label | Trigger |
|-----------|-------------|---------|
| `initializing` | "Setting up backtest..." | Job created |
| `loading_price_data` | "Loading price data..." | Inside `calculateRelevantTradingDays` |
| `calculating_factor_scores` | "Calculating factor scores..." | `profile.StartNewSpan("calculating factor scores")` |
| `running_backtest` | "Running backtest..." | `profile.StartNewSpan("daily calcs")` loop |
| `creating_snapshots` | "Building results..." | `profile.StartNewSpan("creating snapshots")` |
| `done` | "Done!" | Backtest complete |

Progress pct mapping:
- `initializing` → 0-5%
- `loading_price_data` → 5-15%
- `calculating_factor_scores` → 15-40%
- `running_backtest` → 40-85% (increments per trading day)
- `creating_snapshots` → 85-95%
- `done` → 100%

### 4. Files to Create/Modify

| File | Change |
|------|--------|
| `migrations/0000XX_async_backtest_job.up.sql` | New migration |
| `api/endpoints.yaml` | Add `POST /backtest/async`, `GET /backtest/:id/status` |
| `api/models/generated.go` | Auto-generated |
| `api/backtest_async.resolver.go` | New — handles async start |
| `api/backtest_status.resolver.go` | New — handles status polling |
| `api/api.go` | Auto-registered |
| `terraform/api_gateway.tf` | Auto-regenerated |
| `internal/domain/backtest_job.go` | New — domain model for job state |
| `internal/service/backtest_async.service.go` | New — async backtest orchestration |
| `internal/repository/backtest_job.repository.go` | New — DB operations for job state |
| `internal/db/models/postgres/public/model/backtest_job.go` | Auto-generated |

---

## Frontend Changes (for later, out of scope for this PR)

| File | Change |
|------|--------|
| `frontend/src/hooks/useBacktest.ts` | Replace sync call with async + polling |
| `frontend/src/components/BacktestProgressScreen.tsx` | New — step tracker UI |
| `frontend/src/pages/BacktestBuilder.tsx` | Route to builder flow |
| `frontend/src/pages/BacktestResults.tsx` | Route to results view |

Frontend changes will be a separate PR.

---

## Deployment Notes

1. Run migration on Postgres before deploying
2. `make generate-api` to generate resolver stubs
3. `terraform apply` in `terraform/` to update API Gateway routes
4. GitHub Action deploys Lambda automatically on merge

---

## Testing

1. `POST /backtest/async` returns `backtestJobId` immediately (<1s)
2. `GET /backtest/:id/status` returns correct stage and progress
3. When backtest completes, status becomes `completed` with full result
4. If backtest fails, status becomes `failed` with error message
5. Job state is cleaned up after 24h (future: cron job)
