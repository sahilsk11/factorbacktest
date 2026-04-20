# Factor Backtest API

Version: 1.0.0

Description: REST API for the Factor Backtest investment platform

## Endpoints

### GET /

**Operation ID:** root

**Summary:** Root endpoint

**Description:** Returns a welcome message

**Tags:** Health

**Responses:**

- **200:** Success

---

### POST /backtest

**Operation ID:** backtest

**Summary:** Run a factor backtest

**Description:** Execute a backtest using a factor expression over a specified date range and asset universe

**Tags:** Backtest

**Request Body:**

```json
{"$ref": "#/components/schemas/BacktestRequest"}
```

**Responses:**

- **200:** Success

---

### POST /benchmark

**Operation ID:** benchmark

**Summary:** Get benchmark data

**Description:** Retrieve benchmark performance data for a symbol over a date range

**Tags:** Benchmark

**Request Body:**

```json
{"$ref": "#/components/schemas/BenchmarkRequest"}
```

**Responses:**

- **200:** Success

---

### POST /contact

**Operation ID:** contact

**Summary:** Submit a contact message

**Description:** Submit a contact form message from a user

**Tags:** Contact

**Request Body:**

```json
{"$ref": "#/components/schemas/ContactRequest"}
```

**Responses:**

- **200:** Success

---

### POST /constructFactorEquation

**Operation ID:** constructFactorEquation

**Summary:** Construct a factor equation using AI

**Description:** Generate a factor expression equation based on natural language description using GPT

**Tags:** Factor

**Request Body:**

```json
{"$ref": "#/components/schemas/ConstructFactorEquationRequest"}
```

**Responses:**

- **200:** Success

---

### GET /usageStats

**Operation ID:** usageStats

**Summary:** Get API usage statistics

**Description:** Retrieve aggregated API usage statistics

**Tags:** Stats

**Responses:**

- **200:** Success

---

### GET /assetUniverses

**Operation ID:** getAssetUniverses

**Summary:** Get available asset universes

**Description:** Retrieve list of available asset universes with their display names and asset counts

**Tags:** Universe

**Responses:**

- **200:** Success

---

### POST /backtestBondPortfolio

**Operation ID:** backtestBondPortfolio

**Summary:** Run a bond portfolio backtest

**Description:** Execute a backtest on a bond portfolio using duration-based Treasury returns

**Tags:** Backtest

**Request Body:**

```json
{"$ref": "#/components/schemas/BacktestBondPortfolioRequest"}
```

**Responses:**

- **200:** Success

---

### POST /updatePrices

**Operation ID:** updatePrices

**Summary:** Update asset prices

**Description:** Trigger an update of price data for all tracked assets

**Tags:** Prices

**Responses:**

- **200:** Success

---

### POST /addAssetsToUniverse

**Operation ID:** addAssetsToUniverse

**Summary:** Add assets to a universe

**Description:** Add new assets to an existing or new asset universe and ingest their price data

**Tags:** Universe

**Request Body:**

```json
{"$ref": "#/components/schemas/AddAssetsToUniverseRequest"}
```

**Responses:**

- **200:** Success

---

### POST /bookmarkStrategy

**Operation ID:** bookmarkStrategy

**Summary:** Bookmark or unbookmark a strategy

**Description:** Save or remove a strategy from user's bookmarks (requires authentication)

**Tags:** Strategy

**Security:** Requires authentication

**Request Body:**

```json
{"$ref": "#/components/schemas/BookmarkStrategyRequest"}
```

**Responses:**

- **200:** Success

---

### POST /isStrategyBookmarked

**Operation ID:** isStrategyBookmarked

**Summary:** Check if a strategy is bookmarked

**Description:** Check whether a strategy with the given parameters is bookmarked by the current user (requires authentication)

**Tags:** Strategy

**Security:** Requires authentication

**Request Body:**

```json
{"$ref": "#/components/schemas/BookmarkStrategyRequest"}
```

**Responses:**

- **200:** Success

---

### GET /savedStrategies

**Operation ID:** getSavedStrategies

**Summary:** Get user's saved strategies

**Description:** Retrieve all strategies bookmarked by the current user (requires authentication)

**Tags:** Strategy

**Security:** Requires authentication

**Responses:**

- **200:** Success

---

### POST /investInStrategy

**Operation ID:** investInStrategy

**Summary:** Invest in a strategy

**Description:** Create an investment in a published strategy with a specified dollar amount (requires authentication)

**Tags:** Investment

**Security:** Requires authentication

**Request Body:**

```json
{"$ref": "#/components/schemas/InvestInStrategyRequest"}
```

**Responses:**

- **200:** Success

---

### GET /activeInvestments

**Operation ID:** getActiveInvestments

**Summary:** Get user's active investments

**Description:** Retrieve all active investments for the current user (requires authentication)

**Tags:** Investment

**Security:** Requires authentication

**Responses:**

- **200:** Success

---

### GET /publishedStrategies

**Operation ID:** getPublishedStrategies

**Summary:** Get published strategies

**Description:** Retrieve all published strategies available for investment

**Tags:** Strategy

**Responses:**

- **200:** Success

---

### POST /rebalance

**Operation ID:** rebalance

**Summary:** Trigger portfolio rebalancing

**Description:** Trigger rebalancing for all active investments (currently a no-op based on implementation)

**Tags:** Investment

**Responses:**

- **200:** Success

---

### POST /updateOrders

**Operation ID:** updateOrders

**Summary:** Update pending orders

**Description:** Update status of all pending trade orders

**Tags:** Trading

**Responses:**

- **200:** Success

---

### POST /sendSavedStrategySummaryEmails

**Operation ID:** sendSavedStrategySummaryEmails

**Summary:** Send strategy summary emails

**Description:** Send email summaries to users with saved strategies

**Tags:** Email

**Responses:**

- **200:** Success

---

## Schemas

### BacktestRequest

**Type:** object

**Properties:**

| Name | Type | Format | Description |
|------|------|--------|-------------|
| factorOptions | object |  |  |
| backtestStart | string | date | Backtest start date (YYYY-MM-DD) |
| backtestEnd | string | date | Backtest end date (YYYY-MM-DD) |
| samplingIntervalUnit | string |  | Rebalance interval - daily, weekly, monthly, or yearly |
| startCash | number | double | Starting cash amount |
| assetUniverse | string |  | Asset universe code |
| numSymbols | integer |  | Number of top symbols to select |
| userID | string |  | (optional) User ID |

---

### BacktestResponse

**Type:** object

**Properties:**

| Name | Type | Format | Description |
|------|------|--------|-------------|
| factorName | string |  |  |
| strategyID | string | uuid |  |
| backtestSnapshots | object |  |  |
| latestHoldings | object |  |  |
| sharpeRatio | number | double |  |
| annualizedReturn | number | double |  |
| annualizedStandardDeviation | number | double |  |

---

### BacktestSnapshot

**Type:** object

**Properties:**

| Name | Type | Format | Description |
|------|------|--------|-------------|
| valuePercentChange | number | double |  |
| value | number | double |  |
| date | string |  |  |
| assetMetrics | object |  |  |

---

### SnapshotAssetMetrics

**Type:** object

**Properties:**

| Name | Type | Format | Description |
|------|------|--------|-------------|
| assetWeight | number | double |  |
| factorScore | number | double |  |
| priceChangeTilNextResampling | number | double |  |

---

### LatestHoldings

**Type:** object

**Properties:**

| Name | Type | Format | Description |
|------|------|--------|-------------|
| date | string | date-time |  |
| assets | object |  |  |

---

### BenchmarkRequest

**Type:** object

**Properties:**

| Name | Type | Format | Description |
|------|------|--------|-------------|
| symbol | string |  | Ticker symbol |
| start | string | date | Start date (YYYY-MM-DD) |
| end | string | date | End date (YYYY-MM-DD) |
| granularity | string |  | Sampling granularity - daily, weekly, or monthly |

---

### BenchmarkResponse

**Type:** object

**Description:** Map of date strings to benchmark values

---

### ContactRequest

**Type:** object

**Properties:**

| Name | Type | Format | Description |
|------|------|--------|-------------|
| userID | string |  | (optional) User ID |
| replyEmail | string |  | (optional) Reply email address |
| content | string |  | Contact message content (5-2000 characters) |

---

### ContactResponse

**Type:** object

**Properties:**

| Name | Type | Format | Description |
|------|------|--------|-------------|
| message | string |  |  |

---

### ConstructFactorEquationRequest

**Type:** object

**Properties:**

| Name | Type | Format | Description |
|------|------|--------|-------------|
| input | string |  | Natural language description of the factor to construct |

---

### ConstructFactorEquationResponse

**Type:** object

**Properties:**

| Name | Type | Format | Description |
|------|------|--------|-------------|
| factorExpression | string |  | Generated factor expression |
| reason | string |  | Explanation of how the expression was generated |
| error | string |  | Error message if generation failed |
| factorName | string |  | Generated factor name |

---

### BacktestBondPortfolioRequest

**Type:** object

**Properties:**

| Name | Type | Format | Description |
|------|------|--------|-------------|
| backtestStart | string |  | Backtest start date (YYYY-MM) |
| backtestEnd | string |  | Backtest end date (YYYY-MM) |
| durationKey | integer |  | Duration key (0=1-3mo, 2=12-36mo, 3=36-84mo, 4=120-360mo) |
| startCash | number | double | Starting cash amount |
| userID | string |  | (optional) User ID |

---

### AddAssetsToUniverseRequest

**Type:** object

**Properties:**

| Name | Type | Format | Description |
|------|------|--------|-------------|
| universeName | string |  | Asset universe name |
| assets | array |  |  |

---

### MessageResponse

**Type:** object

**Properties:**

| Name | Type | Format | Description |
|------|------|--------|-------------|
| message | string |  |  |

---

### BookmarkStrategyRequest

**Type:** object

**Properties:**

| Name | Type | Format | Description |
|------|------|--------|-------------|
| expression | string |  | Factor expression |
| name | string |  | Strategy name |
| rebalanceInterval | string |  | Rebalance interval |
| numAssets | integer |  | Number of assets |
| assetUniverse | string |  | Asset universe code |
| bookmark | boolean |  | Whether to save (true) or remove (false) the bookmark |

---

### BookmarkStrategyResponse

**Type:** object

**Properties:**

| Name | Type | Format | Description |
|------|------|--------|-------------|
| message | string |  |  |
| savedStrategyID | string | uuid |  |

---

### IsStrategyBookmarkedResponse

**Type:** object

**Properties:**

| Name | Type | Format | Description |
|------|------|--------|-------------|
| name | string |  |  |
| isBookmarked | boolean |  |  |

---

### GetSavedStrategiesResponse

**Type:** object

**Properties:**

| Name | Type | Format | Description |
|------|------|--------|-------------|
| strategyID | string | uuid |  |
| strategyName | string |  |  |
| rebalanceInterval | string |  |  |
| bookmarked | boolean |  |  |
| createdAt | string | date-time |  |
| factorExpression | string |  |  |
| numAssets | integer |  |  |
| assetUniverse | string |  |  |

---

### InvestInStrategyRequest

**Type:** object

**Properties:**

| Name | Type | Format | Description |
|------|------|--------|-------------|
| strategyID | string | uuid | Strategy ID to invest in |
| amount | integer |  | Investment amount in dollars |

---

### InvestInStrategyResponse

**Type:** object

**Properties:**

| Name | Type | Format | Description |
|------|------|--------|-------------|
| success | boolean |  |  |

---

### GetInvestmentsResponse

**Type:** object

**Properties:**

| Name | Type | Format | Description |
|------|------|--------|-------------|
| investmentID | string | uuid |  |
| originalAmountDollars | integer |  |  |
| startDate | string |  |  |
| strategy | object |  |  |
| holdings | array |  |  |
| percentReturnFraction | number | double |  |
| currentValue | number | double |  |
| completedTrades | array |  |  |
| paused | boolean |  |  |

---

### InvestmentStrategy

**Type:** object

**Properties:**

| Name | Type | Format | Description |
|------|------|--------|-------------|
| strategyID | string | uuid |  |
| strategyName | string |  |  |
| factorExpression | string |  |  |
| numAssets | integer |  |  |
| assetUniverse | string |  |  |
| rebalanceInterval | string |  |  |

---

### Holding

**Type:** object

**Properties:**

| Name | Type | Format | Description |
|------|------|--------|-------------|
| symbol | string |  |  |
| quantity | number | double |  |
| marketValue | number | double |  |

---

### FilledTrade

**Type:** object

**Properties:**

| Name | Type | Format | Description |
|------|------|--------|-------------|
| symbol | string |  |  |
| quantity | number | double |  |
| fillPrice | number | double |  |
| filledAt | string | date-time |  |

---

### GetPublishedStrategiesResponse

**Type:** object

**Properties:**

| Name | Type | Format | Description |
|------|------|--------|-------------|
| strategyID | string | uuid |  |
| strategyName | string |  |  |
| rebalanceInterval | string |  |  |
| createdAt | string | date-time |  |
| factorExpression | string |  |  |
| numAssets | integer |  |  |
| assetUniverse | string |  |  |
| sharpeRatio | number | double |  |
| annualizedReturn | number | double |  |
| annualizedStandardDeviation | number | double |  |
| description | string |  |  |

---

### GetAssetUniversesResponse

**Type:** object

**Properties:**

| Name | Type | Format | Description |
|------|------|--------|-------------|
| displayName | string |  |  |
| code | string |  |  |
| numAssets | integer |  |  |

---

### UpdatePricesResponse

**Type:** object

**Properties:**

| Name | Type | Format | Description |
|------|------|--------|-------------|
| numUpdatedAssets | integer |  |  |

---

### UpdateOrdersResponse

**Type:** object

**Properties:**

| Name | Type | Format | Description |
|------|------|--------|-------------|
| success | string |  |  |

---

### SendSavedStrategySummaryEmailsResponse

**Type:** object

**Properties:**

| Name | Type | Format | Description |
|------|------|--------|-------------|
| message | string |  |  |

---
