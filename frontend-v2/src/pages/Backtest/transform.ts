import type { ChartPoint, HoldingRow } from './chart-types';
import type { BacktestResponse } from '@/lib/backtest-stream/types';

// Wire format from the Go API. Hand-mirrored from
// internal/service/backtest.service.go::BacktestSnapshot. We don't pull
// these into types/api.ts because they're only used here, and the
// BacktestResponse generic-record shape on the streaming hook is too
// loose for this file to depend on directly.
interface BacktestSnapshot {
  date: string;
  value: number;
  valuePercentChange: number;
  assetMetrics: Record<string, SnapshotAssetMetrics>;
}

interface SnapshotAssetMetrics {
  assetWeight: number;
  factorScore: number;
  priceChangeTilNextResampling: number | null;
}

// snapshots is the `backtestSnapshots` field on the BacktestResponse —
// typed as Record<string, unknown> on the SSE wire type because it
// crosses the streaming boundary. Validate-once-then-trust here.
function asSnapshotMap(raw: unknown): Record<string, BacktestSnapshot> | null {
  if (typeof raw !== 'object' || raw === null) return null;
  // Trust the structure beyond this; the backend is the source of
  // truth and validating every nested field would be paranoid.
  return raw as Record<string, BacktestSnapshot>;
}

// Build the strategy chart series: every snapshot becomes a point,
// pctReturn = (value - startValue) / startValue, sorted by date.
export function snapshotsToStrategyPoints(result: BacktestResponse): ChartPoint[] {
  const map = asSnapshotMap(result.backtestSnapshots);
  if (!map) return [];

  const dates = Object.keys(map).sort();
  if (dates.length === 0) return [];

  const firstDate = dates[0];
  if (firstDate === undefined) return [];
  const firstSnapshot = map[firstDate];
  if (!firstSnapshot) return [];
  const startValue = firstSnapshot.value;
  if (!startValue || !Number.isFinite(startValue)) return [];

  return dates.map((d) => {
    const snap = map[d];
    if (!snap) {
      return { date: d, pctReturn: 0, value: startValue };
    }
    return {
      date: d,
      value: snap.value,
      pctReturn: snap.value / startValue - 1,
    };
  });
}

// Pull a single day's holdings (sorted by descending weight) for the
// day inspector. Returns null if the date is unknown — caller falls
// back to a "no data" empty state.
export function snapshotToHoldings(
  result: BacktestResponse,
  date: string,
): { holdings: HoldingRow[]; value: number; pctChange: number } | null {
  const map = asSnapshotMap(result.backtestSnapshots);
  if (!map) return null;
  const snap = map[date];
  if (!snap) return null;

  const holdings: HoldingRow[] = Object.entries(snap.assetMetrics ?? {})
    .map(([symbol, metrics]) => ({
      symbol,
      weight: metrics.assetWeight,
      factorScore: metrics.factorScore,
      // Backend ships these as percentage points (9.13 means +9.13%).
      // Convert to fractions to match what the UI's percent
      // formatters expect (formatDelta multiplies by 100).
      priceChange:
        metrics.priceChangeTilNextResampling === null
          ? null
          : metrics.priceChangeTilNextResampling / 100,
    }))
    .filter((h) => h.weight > 0)
    .sort((a, b) => b.weight - a.weight);

  // valuePercentChange is also in percentage points on the wire.
  return {
    holdings,
    value: snap.value,
    pctChange: (snap.valuePercentChange ?? 0) / 100,
  };
}
