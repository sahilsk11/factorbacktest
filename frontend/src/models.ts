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

export interface BacktestResponse {
  factorName: string;
  backtestSnapshots: Record<string, BacktestSnapshot>;
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
  savedStrategyID: string;
  strategyName: string;
  rebalanceInterval: string;
  bookmarked: boolean;
  createdAt: string;
  // modifiedAt?: Date; // Uncomment if needed
}

export interface InvestInStrategyRequest {
  savedStrategyID: string;
  amountDollars: number;
}

export interface GetInvestmentsResponse {
  strategyInvestmentID: string;
  amountDollars: number;
  startDate: string; // Using string to represent ISO 8601 date format
  savedStrategyID: string;
  userAccountID: string;
  createdAt: string; // Using string to represent ISO 8601 date format
}