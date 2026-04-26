import { motion } from 'framer-motion';

import { CountUpNumber } from '@/components/motion/CountUpNumber';
import { formatPercent, formatSharpe } from '@/lib/format';
import { cn } from '@/lib/utils';

interface Props {
  sharpe: number | null | undefined;
  annualizedReturn: number | null | undefined;
  annualizedStdev: number | null | undefined;
  // Latest cumulative return from the chart, used as a peer KPI in
  // the bar so the user sees both annualized + total-return at a
  // glance. The chart owns the live "scrubbed" value via its
  // headline; this one stays at the run-level total.
  totalReturn: number | null;
}

// Strip of headline metrics displayed under the chart. Five tiles,
// monospace numbers, sign-aware coloring on the return tiles. Cards
// fade in staggered after the chart finishes its draw-on so the eye
// can finish tracking the line first.
export function MetricsBar({ sharpe, annualizedReturn, annualizedStdev, totalReturn }: Props): React.ReactNode {
  const tiles = [
    {
      label: 'Total return',
      value: totalReturn,
      format: (n: number) => formatPercent(n),
      colorize: true,
    },
    {
      label: 'Annualized return',
      value: annualizedReturn ?? null,
      format: (n: number) => formatPercent(n),
      colorize: true,
    },
    {
      label: 'Sharpe ratio',
      value: sharpe ?? null,
      format: (n: number) => formatSharpe(n),
      colorize: false,
    },
    {
      label: 'Volatility',
      value: annualizedStdev ?? null,
      format: (n: number) => formatPercent(n),
      colorize: false,
    },
  ];

  return (
    <div className="grid grid-cols-2 gap-px overflow-hidden rounded-lg border border-border bg-border md:grid-cols-4">
      {tiles.map((t, i) => (
        <motion.div
          key={t.label}
          initial={{ opacity: 0, y: 6 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.9 + i * 0.05, duration: 0.3, ease: 'easeOut' }}
          className="flex flex-col gap-1.5 bg-card p-4"
        >
          <span className="text-[11px] uppercase tracking-[0.18em] text-subtle-foreground">
            {t.label}
          </span>
          <span
            className={cn(
              'font-mono text-2xl font-semibold tracking-tight',
              t.colorize && t.value !== null && t.value >= 0 && 'text-gain',
              t.colorize && t.value !== null && t.value < 0 && 'text-loss',
              (!t.colorize || t.value === null) && 'text-foreground',
            )}
          >
            {t.value === null ? 'n/a' : <CountUpNumber value={t.value} format={t.format} />}
          </span>
        </motion.div>
      ))}
    </div>
  );
}
