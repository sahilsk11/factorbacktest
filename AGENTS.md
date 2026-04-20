# Factor Backtest — Agent Development Guide

## Branching / Worktree Convention

For any **nontrivial code change** (new feature, refactor, or multi-file edit):

1. **Create a worktree** in `~/wt` based off `master`, pulling the latest first:
   ```bash
   cd ~/projects/factorbacktest
   git fetch origin master
   git worktree add ~/wt/<branch-name> origin/master
   cd ~/wt/<branch-name>
   ```
2. Work, commit, and push from that worktree.
3. When done, remove the worktree:
   ```bash
   git worktree remove ~/wt/<branch-name>
   ```

This keeps `master` clean and ensures every change starts from the latest version.

## Adding a New API Endpoint

When creating a new API, follow these steps:

1. **Add the endpoint to `api/endpoints.yaml`** — define path, method, handler, request/response types, and AWS config. Example:
   ```yaml
   - path: /myNewEndpoint
     method: POST
     handler: myNewEndpoint
     requires_auth: false
     request:
       field1: string
     response:
       message: string
     aws:
       integration_type: lambda_proxy
       timeout: 30
   ```

2. **Run `make generate-api`** — this single command:
   - Generates Go request/response structs in `api/models/generated.go`
   - Creates a resolver stub in `api/<handler>.resolver.go` (if one doesn't already exist)
   - Registers the route in `api/api.go`
   - Regenerates Terraform in `terraform/api_gateway.tf` for AWS API Gateway configuration

3. **Implement the resolver** — fill in the handler logic in the generated `api/<handler>.resolver.go` file.

4. **Commit everything together** — the YAML, generated Go code, and Terraform should all be in the same commit/PR.

## Deploying Infrastructure Changes

- **Terraform**: After merging, apply `terraform/api_gateway.tf` to create/update API Gateway resources:
  ```bash
  terraform -chdir=terraform init
  terraform -chdir=terraform plan
  terraform -chdir=terraform apply
  ```
- **Lambda**: Push to `master` triggers the `deploy-lambda.yml` GitHub Action which builds and deploys the Lambda function.

## Trade Lifecycle

```
InvestmentService.Rebalance() → TradeService.ExecuteBlock() → Alpaca → UpdateAllPendingOrders() → Reconcile()
```

## Key Tables

| Table | Purpose |
|-------|---------|
| `rebalancer_run` | Individual run results, per-investment |
| `trade_order` (PENDING/COMPLETED/ERROR/CANCELED) | Individual buy/sell orders |
| `investment_trade` | Trade execution details |
| `investment_holdings` | Current portfolio positions |
| `adjusted_price` | Daily price data for backtesting |

## Monitoring

- Cron jobs check DB health, stuck orders, price freshness, portfolio reconciliation
- Alerting via AWS SES + Discord
- Logging: zap JSON → CloudWatch + DB notes field for failure context
