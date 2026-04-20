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

1. **Add the endpoint to `api/openapi.yaml`** — define path, method, operationId, request/response schemas, and x-aws-* extensions. Example:
   ```yaml
   paths:
     /myNewEndpoint:
       post:
         operationId: myNewEndpoint
         x-aws-integration-type: lambda_proxy
         x-aws-timeout: 30
         requestBody:
           content:
             application/json:
               schema:
                 type: object
                 properties:
                   field1:
                     type: string
         responses:
           '200':
             description: Success
             content:
               application/json:
                 schema:
                   type: object
                   properties:
                     message:
                       type: string
   ```

2. **Run `make generate-api`** — this single command:
   - Generates Go request/response structs in `api/models/generated.go`
   - Creates a resolver stub in `api/<handler>.resolver.go` (if one doesn't already exist)
   - Registers the route in `api/api.go`
   - Regenerates Terraform in `terraform/api_gateway.tf` for AWS API Gateway configuration

3. **Serve API docs locally** — `make serve-docs` (runs ReDoc on port 3002)

4. **Implement the resolver** — fill in the handler logic in the generated `api/<handler>.resolver.go` file.

5. **Commit everything together** — the YAML, generated Go code, and Terraform should all be in the same commit/PR.

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
