export interface FactorOptions {
  expression: string;
  name: string;
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