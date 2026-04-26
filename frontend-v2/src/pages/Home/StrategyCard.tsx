import { motion } from 'framer-motion';
import { useNavigate } from 'react-router';

import { CountUpNumber } from '@/components/motion/CountUpNumber';
import { Card } from '@/components/ui/card';
import { formatDelta, formatPercent, formatRebalanceInterval, formatSharpe } from '@/lib/format';
import { cn } from '@/lib/utils';
import type { PublishedStrategy } from '@/types/api';

// Card composition (top → bottom):
//   1. header: name (large) + description (clamped)
//   2. headline KPI: annualized return — count-up tween, gain/loss color
//   3. delta pill: same value as a tinted +/- chip for at-a-glance scan
//   4. secondary KPIs: Sharpe + volatility, right-aligned, tabular-mono
//   5. meta chips: rebalance / universe / num assets — small uppercase
// Hover: border lifts to border-strong, slight scale. Click → backtest.
export function StrategyCard({
  strategy,
  index,
}: {
  strategy: PublishedStrategy;
  // Used to stagger the entrance animation in the grid. Caller passes
  // the array index; we don't reach for siblings.
  index: number;
}): React.ReactNode {
  const navigate = useNavigate();
  const ret = strategy.annualizedReturn;
  // Color on the headline number tracks sign vs. zero baseline. Null
  // values stay neutral (foreground).
  const isPositive = ret !== null && ret >= 0;
  const isNegative = ret !== null && ret < 0;

  return (
    <motion.div
      initial={{ opacity: 0, y: 8 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{
        // Staggered entrance — 40ms between cards mirrors Linear/Vercel
        // dashboard reveals. Cap to keep late cards from feeling lazy.
        delay: Math.min(index * 0.04, 0.4),
        duration: 0.25,
        ease: 'easeOut',
      }}
    >
      <Card
        role="button"
        tabIndex={0}
        onClick={() => {
          // navigate returns a Promise in react-router v7; we don't
          // care about the result here.
          void navigate(`/backtest?id=${strategy.strategyID}`);
        }}
        onKeyDown={(e) => {
          if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault();
            void navigate(`/backtest?id=${strategy.strategyID}`);
          }
        }}
        className={cn(
          'group flex h-full cursor-pointer flex-col gap-5 p-5',
          'transition-[border-color,transform,background-color] duration-150 ease-out',
          'hover:scale-[1.005] hover:border-border-strong hover:bg-card/60',
        )}
      >
        {/* Header */}
        <div>
          <h3 className="text-base font-semibold tracking-tight text-foreground">
            {strategy.strategyName}
          </h3>
          <p className="mt-1 line-clamp-2 min-h-[2.5rem] text-sm text-muted-foreground">
            {strategy.description ?? 'No description provided.'}
          </p>
        </div>

        {/* Headline KPI: annualized return. Count-up on mount; color
            tracks sign. Tabular-mono for the digits. */}
        <div className="flex items-end justify-between gap-4">
          <div>
            <p className="text-xs uppercase tracking-widest text-subtle-foreground">
              Annualized Return
            </p>
            <p
              className={cn(
                'mt-1 font-mono text-3xl font-semibold tracking-tight',
                isPositive && 'text-gain',
                isNegative && 'text-loss',
                ret === null && 'text-foreground',
              )}
            >
              {ret === null ? (
                'n/a'
              ) : (
                <CountUpNumber value={ret} format={(n) => formatPercent(n)} />
              )}
            </p>
          </div>
          {ret !== null && (
            <span
              className={cn(
                'inline-flex h-7 items-center rounded-full px-2.5 font-mono text-xs font-medium',
                isPositive && 'bg-gain/15 text-gain',
                isNegative && 'bg-loss/15 text-loss',
              )}
            >
              {formatDelta(ret)}
            </span>
          )}
        </div>

        {/* Secondary KPIs. Right-aligned numbers in tabular-mono. */}
        <dl className="grid grid-cols-2 gap-4 border-t border-border pt-4">
          <div>
            <dt className="text-xs uppercase tracking-widest text-subtle-foreground">Sharpe</dt>
            <dd className="mt-1 font-mono text-sm font-medium text-foreground">
              {formatSharpe(strategy.sharpeRatio)}
            </dd>
          </div>
          <div className="text-right">
            <dt className="text-xs uppercase tracking-widest text-subtle-foreground">Volatility</dt>
            <dd className="mt-1 font-mono text-sm font-medium text-foreground">
              {formatPercent(strategy.annualizedStandardDeviation)}
            </dd>
          </div>
        </dl>

        {/* Meta chips — rebalance / universe / num assets. */}
        <div className="flex flex-wrap items-center gap-2">
          <Chip>{formatRebalanceInterval(strategy.rebalanceInterval)}</Chip>
          <Chip>{strategy.assetUniverse}</Chip>
          <Chip>{strategy.numAssets} assets</Chip>
        </div>
      </Card>
    </motion.div>
  );
}

function Chip({ children }: { children: React.ReactNode }): React.ReactNode {
  return (
    <span className="inline-flex items-center rounded-md border border-border bg-elevated px-2 py-0.5 text-[10px] font-medium uppercase tracking-widest text-muted-foreground">
      {children}
    </span>
  );
}
