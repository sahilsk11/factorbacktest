import { useNavigate } from 'react-router';

import type { BuilderState } from './types';
import { Button } from '@/components/ui/button';
import { formatNumber } from '@/lib/format';

const DAYS_PER_REBALANCE: Record<BuilderState['rebalanceInterval'], number> = {
  daily: 1,
  weekly: 7,
  monthly: 30,
  yearly: 365,
};

// The "what's about to happen" panel. Sticky on desktop. We avoid
// running the actual backtest here on every keystroke — instead we
// surface the *cost* (compute count + estimated wall time) and a
// human-readable sentence summary, which together make the inputs
// feel coupled without the network thrash. The compute formula
// matches V1's Form.tsx so the projection lines up with what the
// engine actually does.
export function SummaryRail({
  state,
  universeSize,
  canRun,
}: {
  state: BuilderState;
  universeSize: number;
  canRun: boolean;
}): React.ReactNode {
  const navigate = useNavigate();
  const days = daysBetween(state.backtestStart, state.backtestEnd);
  const rebalanceDays = DAYS_PER_REBALANCE[state.rebalanceInterval];
  const computations =
    days > 0 && universeSize > 0 ? Math.floor((universeSize * days) / rebalanceDays / 4) : 0;
  // Same heuristic V1 used for its warning copy: ~10k computations per
  // ~10s of wall time. Floor to whole seconds, min 1s.
  const seconds = Math.max(1, Math.floor(computations / 1000));
  const rebalances = days > 0 ? Math.floor(days / rebalanceDays) : 0;

  return (
    <aside className="sticky top-20 flex flex-col gap-4 rounded-lg border border-border bg-card p-6">
      <header>
        <p className="text-xs font-medium tracking-wide text-subtle-foreground uppercase">
          Your run
        </p>
        <h3 className="mt-1 text-base font-semibold text-foreground">{state.factorName || 'Untitled strategy'}</h3>
      </header>

      <p className="text-sm text-muted-foreground">
        Hold the top{' '}
        <span className="font-mono text-foreground">{state.numAssets}</span> names from{' '}
        <span className="font-mono text-foreground">{state.assetUniverse || '—'}</span>, scored
        by{' '}
        <span className="font-mono text-foreground">{shortExpression(state.factorExpression)}</span>
        , rebalancing{' '}
        <span className="font-mono text-foreground">{state.rebalanceInterval}</span> over{' '}
        <span className="font-mono text-foreground">
          {(days / 365).toFixed(1)} years
        </span>
        .
      </p>

      <dl className="grid grid-cols-2 gap-3 border-t border-border pt-4">
        <Stat label="Rebalances" value={formatNumber(rebalances)} />
        <Stat label="Computations" value={formatNumber(computations)} />
        <Stat
          label="Est. runtime"
          value={seconds < 60 ? `${seconds}s` : `~${Math.ceil(seconds / 60)}m`}
        />
        <Stat label="Universe" value={formatNumber(universeSize)} />
      </dl>

      <Button
        size="lg"
        disabled={!canRun}
        onClick={() => {
          // The /backtest page is a Coming Soon stub today. We pass
          // the builder state in router state so the future result
          // page can pick it up without a re-render trip through the
          // URL. (Future: encode in the URL for shareable runs.)
          void navigate('/backtest', { state: { from: 'builder', request: state } });
        }}
      >
        Run backtest
      </Button>

      {!canRun && (
        <p className="text-xs text-muted-foreground">
          Fill out the steps above to run.
        </p>
      )}
    </aside>
  );
}

function Stat({ label, value }: { label: string; value: string }): React.ReactNode {
  return (
    <div>
      <dt className="text-xs text-subtle-foreground">{label}</dt>
      <dd className="mt-0.5 font-mono text-sm text-foreground">{value}</dd>
    </div>
  );
}

function shortExpression(expr: string): string {
  const trimmed = expr.trim();
  if (trimmed.length <= 32) return trimmed || '—';
  return trimmed.slice(0, 30) + '…';
}

function daysBetween(start: string, end: string): number {
  const a = new Date(start).getTime();
  const b = new Date(end).getTime();
  if (!Number.isFinite(a) || !Number.isFinite(b) || b < a) return 0;
  return Math.floor((b - a) / (1000 * 60 * 60 * 24));
}
