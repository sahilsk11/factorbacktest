// Shared types for the Backtest Builder page.

export interface AssetUniverse {
  displayName: string;
  code: string;
  numAssets: number;
}

export type RebalanceInterval = 'daily' | 'weekly' | 'monthly' | 'yearly';

export interface BuilderState {
  factorExpression: string;
  factorName: string;
  assetUniverse: string;
  backtestStart: string;
  backtestEnd: string;
  rebalanceInterval: RebalanceInterval;
  numAssets: number;
}
