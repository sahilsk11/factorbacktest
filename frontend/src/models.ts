export interface FactorOptions {
  expression: string;
  name: string;
}

export interface GoogleAuthUser {
  accessToken: string;
}

export interface GetAssetUniversesResponse {
  displayName: string;
  code: string;
  numAssets: number;
};

export interface BacktestRequest {
  factorOptions: FactorOptions;
  backtestStart: string;
  backtestEnd: string;
  samplingIntervalUnit: string;
  startCash: number;
  numSymbols?: number;
  userID?: string | null;
  assetUniverse?: string | null;
}

export interface Trade {
  action: string;
  quantity: number;
  symbol: string;
  price: number;
}

export interface BacktestSnapshot {
  valuePercentChange: number;
  value: number;
  date: string;
  assetMetrics: Record<string, SnapshotAssetMetrics>;
}

export interface SnapshotAssetMetrics {
  assetWeight: number;
  factorScore: number;
  priceChangeTilNextResampling?: number | null;
}

export interface PerformanceMetrics {
  sharpeRatio?: number;
  annualizedReturn?: number;
  annualizedStandardDeviation?: number;
}

export interface BacktestResponse {
  factorName: string;
  strategyID: string;
  backtestSnapshots: Record<string, BacktestSnapshot>;
  latestHoldings: LatestHoldings;
  sharpeRatio?: number;
  annualizedReturn?: number;
  annualizedStandardDeviation?: number;
}

export interface LatestHoldings {
  date: string;
  assets: Record<string, SnapshotAssetMetrics>;
}

export interface ContactRequest {
  userID?: string | null;
  replyEmail?: string | null;
  content: string;
}

export interface BookmarkStrategyRequest {
  expression: string;
  name: string;
  backtestStart: string;
  backtestEnd: string;
  rebalanceInterval: string;
  numAssets: number;
  assetUniverse: string;
  bookmark: boolean;
}

export interface GetSavedStrategiesResponse {
  strategyID: string;
  strategyName: string;
  rebalanceInterval: string;
  bookmarked: boolean;
  createdAt: string;
  factorExpression: string;
  numAssets: number;
  assetUniverse: string;
}

export interface InvestInStrategyRequest {
  strategyID: string;
  amountDollars: number;
}

export interface GetInvestmentsResponse {
  investmentID: string; // UUID
  originalAmountDollars: number;
  startDate: string;
  strategy: Strategy;
  holdings: Holdings[];
  percentReturnFraction: number;
  currentValue: number;
  completedTrades: FilledTrade[];
}

export interface Holdings {
  symbol: string;
  quantity: number;
  marketValue: number;
}

export interface FilledTrade {
  symbol: string;
  quantity: number;
  fillPrice: number;
  filledAt: string;
}

export interface Strategy {
  savedStrategyID: string; // UUID
  strategyName: string;
  factorExpression: string;
  numAssets: number;
  assetUniverse: string;
  rebalanceInterval: string;
}


export interface BacktestInputs {
  factorExpression: string,
  factorName: string,
  backtestStart: string,
  backtestEnd: string,
  rebalanceInterval: string,
  numAssets: number,
  assetUniverse: string,
}

export interface GetPublishedStrategiesResponse {
  strategyID: string;
  strategyName: string;
  rebalanceInterval: string;
  createdAt: Date; // Date objects in TypeScript
  factorExpression: string;
  numAssets: number; // int32 in Go maps to number in TypeScript
  assetUniverse: string;
  sharpeRatio?: number;
  annualizedReturn?: number;
  annualizedStandardDeviation?: number;
}
