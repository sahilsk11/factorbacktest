import { AnimatePresence, motion } from 'framer-motion';

import type { HoldingRow } from './chart-types';
import { Card } from '@/components/ui/card';
import { formatDelta, formatPercent } from '@/lib/format';
import { cn } from '@/lib/utils';

interface Props {
  date: string | null;
  holdings: HoldingRow[];
  value: number | null;
  pctChange: number | null;
  // Bubbled up so consumers can clear the selection (e.g. close button
  // on the panel header).
  onClose: () => void;
}

// Rendered below the chart. Empty state nudges the user to click a
// point on the curve. Filled state shows holdings sorted by weight, an
// at-a-glance bar for each weight, and the asset's price change to
// next rebalance. This is the "what did the strategy do that day"
// affordance the user asked for.
export function DayInspector({
  date,
  holdings,
  value,
  pctChange,
  onClose,
}: Props): React.ReactNode {
  const empty = !date || holdings.length === 0;

  return (
    <Card className="overflow-hidden">
      <header className="flex items-baseline justify-between gap-4 border-b border-border px-5 py-4">
        <div>
          <p className="text-[11px] uppercase tracking-[0.18em] text-subtle-foreground">
            Day inspector
          </p>
          <h3 className="mt-0.5 font-mono text-base font-medium text-foreground">
            {date ? formatDateLong(date) : 'Click a point on the chart'}
          </h3>
        </div>
        {date && (
          <div className="flex items-center gap-3">
            {value !== null && (
              <span className="font-mono text-sm text-muted-foreground">
                ${' '}
                {value.toLocaleString('en-US', {
                  minimumFractionDigits: 2,
                  maximumFractionDigits: 2,
                })}
              </span>
            )}
            {pctChange !== null && (
              <span
                className={cn(
                  'inline-flex h-6 items-center rounded-full px-2 font-mono text-xs font-medium',
                  pctChange >= 0 ? 'bg-gain/15 text-gain' : 'bg-loss/15 text-loss',
                )}
              >
                {formatDelta(pctChange)}
              </span>
            )}
            <button
              type="button"
              onClick={onClose}
              className="rounded-md border border-border px-2 py-1 text-xs text-muted-foreground transition-colors hover:bg-elevated hover:text-foreground"
            >
              Close
            </button>
          </div>
        )}
      </header>

      <AnimatePresence mode="wait">
        {empty ? (
          <motion.div
            key="empty"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.2 }}
            className="px-5 py-10 text-center"
          >
            <p className="text-sm text-muted-foreground">
              Hover the chart to scrub through time. Click any point to lock the day and inspect
              what the strategy held.
            </p>
          </motion.div>
        ) : (
          <motion.div
            key={date}
            initial={{ opacity: 0, y: 4 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.25, ease: 'easeOut' }}
          >
            <HoldingsTable rows={holdings} />
          </motion.div>
        )}
      </AnimatePresence>
    </Card>
  );
}

// Holdings table. Tabular numerals, right-aligned numerics, sticky
// header. Bar visualization per row uses a flex spacer so weight
// renders as a horizontal magnitude without a separate chart lib.
function HoldingsTable({ rows }: { rows: HoldingRow[] }): React.ReactNode {
  const maxWeight = Math.max(...rows.map((r) => r.weight), 0.0001);

  return (
    <div className="max-h-[360px] overflow-auto">
      <table className="w-full text-left text-sm">
        <thead className="sticky top-0 z-10 bg-card text-[11px] uppercase tracking-[0.14em] text-subtle-foreground">
          <tr className="border-b border-border">
            <th className="px-5 py-2 font-medium">Symbol</th>
            <th className="px-5 py-2 font-medium">Weight</th>
            <th className="px-5 py-2 text-right font-medium">Factor score</th>
            <th className="px-5 py-2 text-right font-medium">Period return</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r, i) => (
            <motion.tr
              key={r.symbol}
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              transition={{ delay: Math.min(i * 0.012, 0.18), duration: 0.18 }}
              className="border-b border-border/60 last:border-b-0 hover:bg-elevated/40"
            >
              <td className="px-5 py-2.5 font-mono text-foreground">{r.symbol}</td>
              <td className="px-5 py-2.5">
                <div className="flex items-center gap-3">
                  <div className="h-1.5 w-32 overflow-hidden rounded-full bg-elevated">
                    <motion.div
                      className="h-full bg-accent"
                      initial={{ width: 0 }}
                      animate={{ width: `${(r.weight / maxWeight) * 100}%` }}
                      transition={{ duration: 0.45, ease: 'easeOut' }}
                    />
                  </div>
                  <span className="font-mono text-xs text-muted-foreground">
                    {formatPercent(r.weight)}
                  </span>
                </div>
              </td>
              <td className="px-5 py-2.5 text-right font-mono text-muted-foreground">
                {r.factorScore.toFixed(3)}
              </td>
              <td className="px-5 py-2.5 text-right">
                {r.priceChange === null ? (
                  <span className="font-mono text-subtle-foreground">—</span>
                ) : (
                  <span className={cn('font-mono', r.priceChange >= 0 ? 'text-gain' : 'text-loss')}>
                    {formatDelta(r.priceChange)}
                  </span>
                )}
              </td>
            </motion.tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function formatDateLong(iso: string): string {
  const d = new Date(iso);
  if (isNaN(d.getTime())) return iso;
  return d.toLocaleDateString('en-US', {
    weekday: 'short',
    month: 'long',
    day: 'numeric',
    year: 'numeric',
  });
}
