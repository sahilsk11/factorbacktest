export interface FactorOptions {
  expression: string;
  intensity: number;
  name: string;
}

export interface BacktestRequest {
  factorOptions: FactorOptions;
  backtestStart: string;
  backtestEnd: string;
  samplingIntervalUnit: string;
  assetSelectionMode: string;
  startCash: number;
  anchorPortfolioQuantities: Record<string, number>;
  numSymbols?: number;
  userID?: string | null;
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