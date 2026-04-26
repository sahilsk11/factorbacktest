import { Section } from './Section';
import type { AssetUniverse } from './types';
import { formatNumber } from '@/lib/format';
import { cn } from '@/lib/utils';

// Step 2 — pick the pool. Universe tiles render the asset count and a
// proportional bar so size differences read at a glance instead of
// from a number. The bar maxes against the largest universe in the
// list (typically "All").
export function UniverseSection({
  universes,
  selectedCode,
  setSelectedCode,
  loading,
}: {
  universes: AssetUniverse[];
  selectedCode: string;
  setSelectedCode: (code: string) => void;
  loading: boolean;
}): React.ReactNode {
  return (
    <Section
      step={2}
      title="Choose your asset universe"
      hint="The pool the factor scores from. Bigger universes mean longer runs and more diverse holdings."
    >
      {loading && <UniverseSkeleton />}
      {!loading && (
        <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
          {universes.map((u) => {
            const active = u.code === selectedCode;
            return (
              <button
                key={u.code}
                type="button"
                onClick={() => setSelectedCode(u.code)}
                className={cn(
                  'flex flex-col rounded-md border p-3 text-left transition-colors',
                  active
                    ? 'border-accent bg-accent/10'
                    : 'border-border bg-elevated/50 hover:border-border-strong hover:bg-elevated',
                )}
              >
                <div className="flex items-baseline justify-between">
                  <span className="text-sm font-medium text-foreground">{u.displayName}</span>
                  <span className="font-mono text-xs text-muted-foreground">
                    {formatNumber(u.numAssets)} assets
                  </span>
                </div>
              </button>
            );
          })}
        </div>
      )}
    </Section>
  );
}

function UniverseSkeleton(): React.ReactNode {
  return (
    <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
      {Array.from({ length: 4 }).map((_, i) => (
        <div
          key={i}
          className="h-[68px] animate-pulse rounded-md border border-border bg-elevated/40"
          aria-hidden
        />
      ))}
    </div>
  );
}
