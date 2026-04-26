import { useEffect } from 'react';
import { useLocation, useNavigate, useSearchParams } from 'react-router';

import { BacktestLoadingOverlay } from '@/components/BacktestLoadingOverlay';
import { apiClient } from '@/lib/api';
import type { BacktestRequest } from '@/lib/backtest-stream/types';
import { useBacktestStream } from '@/lib/backtest-stream/useBacktestStream';
import type { BuilderState } from '@/pages/Builder/types';
import type { PublishedStrategy } from '@/types/api';

interface LocationState {
  from: 'builder';
  request: BuilderState;
}

function isBuilderState(s: unknown): s is LocationState {
  return (
    typeof s === 'object' &&
    s !== null &&
    (s as LocationState).from === 'builder' &&
    typeof (s as LocationState).request === 'object'
  );
}

function builderStateToRequest(state: BuilderState): BacktestRequest {
  return {
    factorOptions: { expression: state.factorExpression, name: state.factorName },
    backtestStart: state.backtestStart,
    backtestEnd: state.backtestEnd,
    samplingIntervalUnit: state.rebalanceInterval,
    startCash: 10_000,
    numSymbols: state.numAssets,
    assetUniverse: state.assetUniverse,
  };
}

function strategyToRequest(s: PublishedStrategy, start: string, end: string): BacktestRequest {
  return {
    factorOptions: { expression: s.factorExpression, name: s.strategyName },
    backtestStart: start,
    backtestEnd: end,
    samplingIntervalUnit: s.rebalanceInterval,
    startCash: 10_000,
    numSymbols: s.numAssets,
    assetUniverse: s.assetUniverse,
  };
}

function threeYearsAgo(): string {
  const d = new Date();
  d.setFullYear(d.getFullYear() - 3);
  return localISODate(d);
}

function today(): string {
  return localISODate(new Date());
}

function localISODate(d: Date): string {
  const mm = String(d.getMonth() + 1).padStart(2, '0');
  const dd = String(d.getDate()).padStart(2, '0');
  return `${d.getFullYear()}-${mm}-${dd}`;
}

const ignoreRunError = () => undefined;

export function BacktestPage(): React.ReactNode {
  const location = useLocation();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const stream = useBacktestStream();

  const strategyId = searchParams.get('id');
  const fromBuilder = isBuilderState(location.state);

  useEffect(() => {
    const startTimer = window.setTimeout(() => {
      if (fromBuilder) {
        const req = builderStateToRequest(location.state.request);
        stream.run(req).catch(ignoreRunError);
        return;
      }

      if (strategyId) {
        // Fetch the published strategy then kick off the backtest.
        const startParam = searchParams.get('start');
        const start =
          startParam && !isNaN(new Date(startParam).getTime()) ? startParam : threeYearsAgo();

        apiClient
          .get<PublishedStrategy[]>('/publishedStrategies')
          .then((strategies) => {
            const s = strategies.find((s) => s.strategyID === strategyId);
            if (!s) {
              void navigate('/builder', { replace: true });
              return;
            }
            stream.run(strategyToRequest(s, start, today())).catch(ignoreRunError);
          })
          .catch(() => {
            void navigate('/builder', { replace: true });
          });
        return;
      }

      // No valid entry point — send back to builder.
      void navigate('/builder', { replace: true });
    }, 0);

    return () => window.clearTimeout(startTimer);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  if (!fromBuilder && !strategyId) return null;

  return (
    <>
      <BacktestLoadingOverlay
        status={stream.status}
        steps={stream.steps}
        error={stream.error}
        totalMs={stream.totalMs}
        onClose={stream.reset}
      />

      {stream.status === 'success' && (
        <div className="flex min-h-[60vh] items-center justify-center">
          <p className="text-lg font-medium text-foreground">Backtest done.</p>
        </div>
      )}
    </>
  );
}
