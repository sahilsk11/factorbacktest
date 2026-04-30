import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';

import { Card } from '@/components/ui/card';
import { apiClient } from '@/lib/api';
import { formatCurrency, formatPercent } from '@/lib/format';
import type { Investment } from '@/types/api';

// Investments page - displays user's investment portfolio
export function InvestmentsPage(): React.ReactNode {
  const { data, isLoading, isError, error } = useQuery({
    queryKey: ['investments'],
    queryFn: () => apiClient.get<Investment[]>('/activeInvestments'),
  });

  return (
    <div className="mx-auto max-w-7xl px-6 py-8">
      <header className="mb-8">
        <h1 className="text-2xl font-semibold tracking-tight text-foreground">My Investments</h1>
        <p className="mt-2 text-sm text-muted-foreground">
          Track your active investments and their performance over time.
        </p>
      </header>

      {isLoading && <InvestmentSkeletonGrid />}
      {isError && (
        <div className="rounded-lg border border-border bg-card p-6">
          <p className="text-sm font-medium text-loss">Failed to load investments</p>
          <p className="mt-1 text-sm text-muted-foreground">
            {error instanceof Error ? error.message : 'Unknown error'}
          </p>
        </div>
      )}
      {data && (
        <>
          {data.length === 0 ? (
            <EmptyState />
          ) : (
            <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
              {data.map((investment) => (
                <InvestmentCard key={investment.investmentID} investment={investment} />
              ))}
            </div>
          )}
        </>
      )}
    </div>
  );
}

function InvestmentCard({ investment }: { investment: Investment }): React.ReactNode {
  const [expanded, setExpanded] = useState(false);
  const profitLoss = investment.currentValue - investment.originalAmountDollars;
  const isProfit = profitLoss >= 0;

  return (
    <Card className="flex flex-col">
      {/* Header */}
      <div className="border-b border-border p-4">
        <div className="flex items-start justify-between">
          <div className="flex-1">
            <h3 className="text-base font-semibold text-foreground">{investment.strategy.strategyName}</h3>
            <p className="mt-1 text-xs text-muted-foreground">
              Started: {new Date(investment.startDate).toLocaleDateString()}
            </p>
          </div>
          {investment.paused && (
            <span className="rounded-full bg-muted px-2 py-1 text-xs font-medium text-muted-foreground">
              Paused
            </span>
          )}
        </div>
      </div>

      {/* Performance Summary */}
      <div className="grid grid-cols-2 gap-4 border-b border-border p-4">
        <div>
          <p className="text-xs text-muted-foreground">Current Value</p>
          <p className="mt-1 text-lg font-semibold text-foreground">
            {formatCurrency(investment.currentValue)}
          </p>
        </div>
        <div>
          <p className="text-xs text-muted-foreground">Return</p>
          <p className={`mt-1 text-lg font-semibold ${isProfit ? 'text-gain' : 'text-loss'}`}>
            {formatPercent(investment.percentReturnFraction)}
          </p>
          <p className={`text-xs ${isProfit ? 'text-gain' : 'text-loss'}`}>
            {isProfit ? '+' : ''}{formatCurrency(profitLoss)}
          </p>
        </div>
      </div>

      {/* Holdings Summary */}
      <div className="p-4">
        <div className="flex items-center justify-between">
          <div>
            <p className="text-xs text-muted-foreground">Holdings</p>
            <p className="mt-1 text-sm text-foreground">
              {investment.holdings.length} assets
            </p>
          </div>
          <button
            onClick={() => setExpanded(!expanded)}
            className="text-sm font-medium text-primary hover:text-primary/90"
          >
            {expanded ? 'Show Less' : 'Show Details'}
          </button>
        </div>

        {/* Expanded Details */}
        {expanded && (
          <div className="mt-4 space-y-4 border-t border-border pt-4">
            {/* Strategy Details */}
            <div>
              <h4 className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
                Strategy
              </h4>
              <dl className="mt-2 grid grid-cols-2 gap-2 text-xs">
                <div>
                  <dt className="text-muted-foreground">Expression</dt>
                  <dd className="font-mono text-foreground">
                    {investment.strategy.factorExpression}
                  </dd>
                </div>
                <div>
                  <dt className="text-muted-foreground">Universe</dt>
                  <dd className="text-foreground">{investment.strategy.assetUniverse}</dd>
                </div>
                <div>
                  <dt className="text-muted-foreground">Assets</dt>
                  <dd className="text-foreground">{investment.strategy.numAssets}</dd>
                </div>
                <div>
                  <dt className="text-muted-foreground">Rebalance</dt>
                  <dd className="text-foreground">{investment.strategy.rebalanceInterval}</dd>
                </div>
              </dl>
            </div>

            {/* Current Holdings */}
            {investment.holdings.length > 0 && (
              <div>
                <h4 className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
                  Current Holdings
                </h4>
                <div className="mt-2 space-y-1">
                  {investment.holdings.map((holding) => (
                    <div
                      key={holding.symbol}
                      className="flex items-center justify-between text-xs"
                    >
                      <span className="font-medium text-foreground">{holding.symbol}</span>
                      <span className="text-muted-foreground">
                        {holding.quantity.toFixed(4)} • {formatCurrency(holding.marketValue)}
                      </span>
                    </div>
                  ))}
                </div>
              </div>
            )}

            {/* Recent Trades */}
            {investment.completedTrades.length > 0 && (
              <div>
                <h4 className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
                  Completed Trades
                </h4>
                <div className="mt-2 space-y-1">
                  {investment.completedTrades.slice(0, 5).map((trade, idx) => (
                    <div
                      key={`${trade.symbol}-${trade.filledAt}-${idx}`}
                      className="flex items-center justify-between text-xs"
                    >
                      <span className="font-medium text-foreground">
                        {trade.quantity > 0 ? 'BUY' : 'SELL'} {trade.symbol}
                      </span>
                      <span className="text-muted-foreground">
                        {Math.abs(trade.quantity).toFixed(4)} @ {formatCurrency(trade.fillPrice)}
                      </span>
                    </div>
                  ))}
                  {investment.completedTrades.length > 5 && (
                    <p className="mt-1 text-xs text-muted-foreground">
                      +{investment.completedTrades.length - 5} more trades
                    </p>
                  )}
                </div>
              </div>
            )}
          </div>
        )}
      </div>
    </Card>
  );
}

function InvestmentSkeletonGrid(): React.ReactNode {
  return (
    <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
      {Array.from({ length: 4 }).map((_, i) => (
        <div
          key={i}
          className="h-64 animate-pulse rounded-lg border border-border bg-card/60"
          aria-hidden
        />
      ))}
    </div>
  );
}

function EmptyState(): React.ReactNode {
  return (
    <Card className="flex flex-col items-center justify-center py-16 text-center">
      <h3 className="text-lg font-semibold text-foreground">No investments yet</h3>
      <p className="mt-2 max-w-sm text-sm text-muted-foreground">
        Start investing in strategies to see your portfolio here. Track your investments and monitor
        their performance over time.
      </p>
    </Card>
  );
}
