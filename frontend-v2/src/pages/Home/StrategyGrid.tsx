import { StrategyCard } from './StrategyCard';
import type { PublishedStrategy } from '@/types/api';

// Responsive grid of strategy cards. Stagger animation lives on the
// individual cards (using their array index) so reordering feels
// natural without orchestration here.
export function StrategyGrid({ strategies }: { strategies: PublishedStrategy[] }): React.ReactNode {
  if (strategies.length === 0) {
    return (
      <div className="rounded-lg border border-dashed border-border bg-card/40 p-12 text-center">
        <p className="text-sm text-muted-foreground">
          No published strategies yet. Create the first one.
        </p>
      </div>
    );
  }

  return (
    <div className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3">
      {strategies.map((s, i) => (
        <StrategyCard key={s.strategyID} strategy={s} index={i} />
      ))}
    </div>
  );
}
