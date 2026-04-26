import { useQuery } from '@tanstack/react-query';

import { Hero } from './Hero';
import { StrategyGrid } from './StrategyGrid';
import { apiClient } from '@/lib/api';
import type { PublishedStrategy } from '@/types/api';

// Top-level landing page. Owns the data fetch; renders Hero +
// StrategyGrid with explicit loading / error / success states.
export function HomePage(): React.ReactNode {
  const { data, isLoading, isError, error } = useQuery({
    queryKey: ['publishedStrategies'],
    queryFn: () => apiClient.get<PublishedStrategy[]>('/publishedStrategies'),
  });

  return (
    <>
      <Hero />
      <section className="mx-auto max-w-7xl px-6 pb-24">
        <header className="mb-6">
          <h2 className="text-lg font-semibold tracking-tight text-foreground">
            Published strategies
          </h2>
          <p className="mt-1 text-sm text-muted-foreground">
            Browse community strategies, click through to see the full backtest.
          </p>
        </header>

        {isLoading && <SkeletonGrid />}
        {isError && (
          <div className="rounded-lg border border-border bg-card p-6">
            <p className="text-sm font-medium text-loss">Failed to load strategies</p>
            <p className="mt-1 text-sm text-muted-foreground">
              {error instanceof Error ? error.message : 'Unknown error'}
            </p>
          </div>
        )}
        {data && <StrategyGrid strategies={data} />}
      </section>
    </>
  );
}

// Skeleton matches the grid shape so the layout doesn't jump when the
// real data lands. Six placeholders covers two rows on the common
// desktop breakpoint.
function SkeletonGrid(): React.ReactNode {
  return (
    <div className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3">
      {Array.from({ length: 6 }).map((_, i) => (
        <div
          key={i}
          className="h-64 animate-pulse rounded-lg border border-border bg-card/60"
          aria-hidden
        />
      ))}
    </div>
  );
}
