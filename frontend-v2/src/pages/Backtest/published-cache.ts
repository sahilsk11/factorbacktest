import type { BacktestResponse } from '@/lib/backtest-stream/types';

export interface PublishedStrategyBacktest {
  strategyID: string;
  strategyName: string;
  rebalanceInterval: string;
  createdAt: string;
  factorExpression: string;
  numAssets: number;
  assetUniverse: string;
  description: string | null;
  backtestStart: string;
  backtestEnd: string;
  cachedAt: string | null;
  result: BacktestResponse | null;
}

function isValidDateParam(value: string | null): value is string {
  return Boolean(value && !Number.isNaN(new Date(value).getTime()));
}

// Cache is usable when a result payload exists and any caller-supplied
// start/end match the cached window. Missing start/end (the landing-card
// click path) always accepts the cached window.
export function shouldUsePublishedCache(
  cache: Pick<PublishedStrategyBacktest, 'result' | 'backtestStart' | 'backtestEnd'>,
  startParam: string | null,
  endParam: string | null,
): boolean {
  if (!cache.result) return false;
  if (isValidDateParam(startParam) && startParam !== cache.backtestStart) return false;
  if (isValidDateParam(endParam) && endParam !== cache.backtestEnd) return false;
  return true;
}

export function publishedWindowOrFallback(
  startParam: string | null,
  endParam: string | null,
  fallbackStart: string,
  fallbackEnd: string,
): { start: string; end: string } {
  return {
    start: isValidDateParam(startParam) ? startParam : fallbackStart,
    end: isValidDateParam(endParam) ? endParam : fallbackEnd,
  };
}
