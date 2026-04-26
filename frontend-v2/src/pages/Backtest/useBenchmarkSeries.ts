import { useQuery } from '@tanstack/react-query';

import type { ChartPoint } from './chart-types';
import { apiClient } from '@/lib/api';

// /benchmark returns cumulative percent change since the window's
// start, keyed by date and expressed as percentage points (NOT
// fractions): { "2024-01-15": 12.5, "2024-02-15": -3.2, ... } means
// +12.5% on 2024-01-15. Convert to fractions to match the strategy's
// pctReturn (e.g. 0.125).
type BenchmarkResponse = Record<string, number>;

interface BenchmarkRequest {
  symbol: string;
  start: string;
  end: string;
  granularity: string;
}

// Fetches SPY (or any benchmark) and returns it as ChartPoints
// already cumulated into pctReturn so the chart can render it directly
// alongside the strategy series.
export function useBenchmarkSeries(req: BenchmarkRequest | null): {
  points: ChartPoint[] | null;
  isLoading: boolean;
} {
  const { data, isLoading } = useQuery({
    queryKey: ['benchmark', req?.symbol, req?.start, req?.end, req?.granularity],
    enabled: req !== null,
    queryFn: () => apiClient.post<BenchmarkResponse>('/benchmark', req),
    // Benchmarks for a fixed window don't change — keep them around.
    staleTime: 60 * 60 * 1000,
  });

  if (!data) return { points: null, isLoading };

  const dates = Object.keys(data).sort();
  const points: ChartPoint[] = dates.map((d) => ({
    date: d,
    // Server returns percentage points (62.95 means +62.95%). Divide
    // to land in fraction-space alongside the strategy series.
    pctReturn: (data[d] ?? 0) / 100,
  }));

  return { points, isLoading };
}
