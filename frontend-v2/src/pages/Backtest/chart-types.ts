// Shared types for the Backtest page chart + day inspector. Not in
// types/api.ts because these are post-transform shapes, not wire formats.

export interface ChartPoint {
  // ISO-8601 date string (YYYY-MM-DD). Same shape across strategy and
  // benchmark so the crosshair can join them by date.
  date: string;
  // Cumulative return relative to the first point, as a fraction
  // (0.12 = +12%). Plotting returns instead of dollar values lets
  // strategy + SPY share a y-axis with no normalization weirdness.
  pctReturn: number;
}

export interface ChartSeries {
  // Stable id used for SVG <defs> ids and React keys.
  id: 'strategy' | 'benchmark';
  label: string;
  // Tailwind / CSS color tokens. The chart uses CSS vars so the rest
  // of the app's theme tokens stay the source of truth.
  colorVar: string;
  points: ChartPoint[];
  // Optional dashed style for the benchmark; the strategy is always
  // solid.
  dashed?: boolean;
}

// One entry in the holdings table on the day inspector.
export interface HoldingRow {
  symbol: string;
  weight: number;
  factorScore: number;
  priceChange: number | null;
}
