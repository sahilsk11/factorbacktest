import { describe, expect, it } from 'vitest';

import { publishedWindowOrFallback, shouldUsePublishedCache } from './published-cache';
import type { BacktestResponse } from '@/lib/backtest-stream/types';

const result: BacktestResponse = {
  factorName: 'Momentum',
  strategyID: 'strat-1',
  backtestSnapshots: {},
  latestHoldings: {},
};

describe('shouldUsePublishedCache', () => {
  it('rejects a miss with no result payload', () => {
    expect(
      shouldUsePublishedCache(
        { result: null, backtestStart: '2023-01-01', backtestEnd: '2026-01-01' },
        null,
        null,
      ),
    ).toBe(false);
  });

  it('accepts a hit when the landing-card path supplies no dates', () => {
    expect(
      shouldUsePublishedCache(
        { result, backtestStart: '2023-01-01', backtestEnd: '2026-01-01' },
        null,
        null,
      ),
    ).toBe(true);
  });

  it('accepts a hit when supplied dates match the cached window', () => {
    expect(
      shouldUsePublishedCache(
        { result, backtestStart: '2023-01-01', backtestEnd: '2026-01-01' },
        '2023-01-01',
        '2026-01-01',
      ),
    ).toBe(true);
  });

  it('rejects a stale window when the caller asked for different dates', () => {
    expect(
      shouldUsePublishedCache(
        { result, backtestStart: '2023-01-01', backtestEnd: '2026-01-01' },
        '2020-01-01',
        '2026-01-01',
      ),
    ).toBe(false);
  });
});

describe('publishedWindowOrFallback', () => {
  it('uses fallbacks when params are missing or invalid', () => {
    expect(publishedWindowOrFallback(null, 'not-a-date', '2023-01-01', '2026-01-01')).toEqual({
      start: '2023-01-01',
      end: '2026-01-01',
    });
  });

  it('keeps valid caller dates', () => {
    expect(
      publishedWindowOrFallback('2024-02-01', '2025-02-01', '2023-01-01', '2026-01-01'),
    ).toEqual({
      start: '2024-02-01',
      end: '2025-02-01',
    });
  });
});
