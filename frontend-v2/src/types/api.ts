// Hand-ported API contract. Source: api/get_published_strategies.resolver.go
// (Go server is the source of truth; we mirror what it actually emits).
//
// Other types port lazily as PRs need them — codegen from the Go API
// is a future concern (see plans/frontend-v2-north-star.md §8).

export interface PublishedStrategy {
  strategyID: string;
  strategyName: string;
  rebalanceInterval: string;
  // ISO-8601 string, NOT a Date. The legacy frontend's `Date` typing
  // here was wrong (Go marshals time.Time → string).
  createdAt: string;
  factorExpression: string;
  numAssets: number;
  assetUniverse: string;
  // Optional because GetLatestPublishedRun can return nil for
  // strategies that have no runs yet.
  sharpeRatio: number | null;
  annualizedReturn: number | null;
  annualizedStandardDeviation: number | null;
  description: string | null;
}

// Investment types. Source: api/get_investments.resolver.go
export interface Investment {
  investmentID: string;
  originalAmountDollars: number;
  startDate: string; // ISO-8601 date string
  strategy: InvestmentStrategy;
  holdings: Holding[];
  percentReturnFraction: number;
  currentValue: number;
  completedTrades: FilledTrade[];
  paused: boolean;
}

export interface InvestmentStrategy {
  strategyID: string;
  strategyName: string;
  factorExpression: string;
  numAssets: number;
  assetUniverse: string;
  rebalanceInterval: string;
}

export interface Holding {
  symbol: string;
  quantity: number;
  marketValue: number;
}

export interface FilledTrade {
  symbol: string;
  quantity: number;
  fillPrice: number;
  filledAt: string; // ISO-8601 datetime string
}
