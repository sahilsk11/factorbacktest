// Curated factor expression presets. Click-to-fill on the builder.
// Names roughly match the factor families documented in factors.md.

export interface FactorPreset {
  id: string;
  name: string;
  expression: string;
  blurb: string;
}

export const FACTOR_PRESETS: FactorPreset[] = [
  {
    id: 'momentum',
    name: 'Momentum',
    blurb: 'Hold the names that have run hardest over the last six months.',
    expression: 'pricePercentChange(addDate(currentDate, 0, -6, 0), currentDate)',
  },
  {
    id: 'value',
    name: 'Value',
    blurb: 'Tilt toward cheap earnings — low price-to-earnings.',
    expression: '-pe(currentDate)',
  },
  {
    id: 'quality',
    name: 'Quality',
    blurb: 'Favor profitable balance sheets — high return on equity.',
    expression: 'roe(currentDate)',
  },
  {
    id: 'low-vol',
    name: 'Low volatility',
    blurb: 'Lean defensive — minimize trailing 90-day volatility.',
    expression: '-stdev(addDate(currentDate, 0, -3, 0), currentDate)',
  },
];
