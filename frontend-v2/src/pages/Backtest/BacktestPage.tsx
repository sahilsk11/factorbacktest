import { useQuery } from '@tanstack/react-query';
import { motion } from 'framer-motion';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { useLocation, useNavigate, useSearchParams } from 'react-router';

import type { ChartSeries } from './chart-types';
import { DayInspector } from './DayInspector';
import { EquityChart } from './EquityChart';
import { InlineProgress } from './InlineProgress';
import { MetricsBar } from './MetricsBar';
import { RerunPanel } from './RerunPanel';
import { snapshotsToStrategyPoints, snapshotToHoldings } from './transform';
import { useBenchmarkSeries } from './useBenchmarkSeries';
import { BacktestLoadingOverlay } from '@/components/BacktestLoadingOverlay';
import { Button } from '@/components/ui/button';
import { apiClient } from '@/lib/api';
import type { BacktestRequest, BacktestResponse } from '@/lib/backtest-stream/types';
import { useBacktestStream } from '@/lib/backtest-stream/useBacktestStream';
import type { AssetUniverse, BuilderState, RebalanceInterval } from '@/pages/Builder/types';
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

function strategyToBuilderState(s: PublishedStrategy, start: string, end: string): BuilderState {
  return {
    factorExpression: s.factorExpression,
    factorName: s.strategyName,
    assetUniverse: s.assetUniverse,
    backtestStart: start,
    backtestEnd: end,
    rebalanceInterval: s.rebalanceInterval as RebalanceInterval,
    numAssets: s.numAssets,
  };
}

function threeYearsAgo(): string {
  const d = new Date();
  d.setFullYear(d.getFullYear() - 3);
  return localISODate(d);
}

function todayISO(): string {
  return localISODate(new Date());
}

function localISODate(d: Date): string {
  const mm = String(d.getMonth() + 1).padStart(2, '0');
  const dd = String(d.getDate()).padStart(2, '0');
  return `${d.getFullYear()}-${mm}-${dd}`;
}

const ignoreRunError = () => undefined;

const VALID_INTERVALS: ReadonlySet<RebalanceInterval> = new Set([
  'daily',
  'weekly',
  'monthly',
  'yearly',
]);

// Parse the URL params we write back on every successful run. Returns
// null unless every required field is present + parseable. Partial
// matches don't fall through to a half-built state — we'd rather kick
// to the builder than run with invented defaults.
function parseDirectInputs(params: URLSearchParams): BuilderState | null {
  const expr = params.get('expr');
  const name = params.get('name');
  const start = params.get('start');
  const end = params.get('end');
  const interval = params.get('interval');
  const num = params.get('num');
  const universe = params.get('universe');

  if (!expr || !name || !start || !end || !interval || !num || !universe) return null;
  if (!VALID_INTERVALS.has(interval as RebalanceInterval)) return null;
  const numParsed = Number(num);
  if (!Number.isFinite(numParsed) || numParsed < 3) return null;
  if (start >= end) return null;

  return {
    factorExpression: expr,
    factorName: name,
    assetUniverse: universe,
    backtestStart: start,
    backtestEnd: end,
    rebalanceInterval: interval as RebalanceInterval,
    numAssets: Math.floor(numParsed),
  };
}

// Page composition (top → bottom):
//   1. Hero chart card — full-width, ~70vh, animated draw-on
//   2. Metrics bar — sharpe / annualized return / volatility tiles
//   3. Day inspector — clicked-date holdings + factor scores
//
// Floating affordances: a "Configure" button (top-right of the chart
// card) opens the RerunPanel slide-out for tweaking inputs without
// leaving the page. Re-runs swap the on-screen data with a fresh
// draw-on animation; an inline progress bar replaces the full-page
// overlay for second-and-subsequent runs.
export function BacktestPage(): React.ReactNode {
  const location = useLocation();
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const stream = useBacktestStream();

  const strategyId = searchParams.get('id');
  const fromBuilder = isBuilderState(location.state);
  const directInputs = parseDirectInputs(searchParams);

  // The current builder state — pre-fills the rerun panel and feeds
  // the rerun's request. Initialized from whichever entry point the
  // user came in through.
  const [builderState, setBuilderState] = useState<BuilderState | null>(() => {
    if (isBuilderState(location.state)) return location.state.request;
    if (directInputs) return directInputs;
    return null;
  });

  // The most-recently-completed result. We hold onto this even while
  // a new run is streaming so the chart keeps showing the prior data
  // (the inline progress bar tells the user something is happening).
  const [lastResult, setLastResult] = useState<BacktestResponse | null>(null);
  const [hoveredDate, setHoveredDate] = useState<string | null>(null);
  const [selectedDate, setSelectedDate] = useState<string | null>(null);
  const [panelOpen, setPanelOpen] = useState(false);

  // Increments per successful run so the chart can remount and play
  // its draw-on animation fresh. Keying on the strategy ID isn't
  // enough — re-running the same strategy with a new window should
  // still feel alive. The first-vs-subsequent run distinction
  // (overlay vs inline progress) is derived from `lastResult` instead
  // of a separate ref, so render is a pure function of state.
  const [runCounter, setRunCounter] = useState(0);

  // Asset universes — needed to populate the rerun panel selector.
  const { data: universes } = useQuery({
    queryKey: ['assetUniverses'],
    queryFn: () => apiClient.get<AssetUniverse[]>('/assetUniverses'),
  });

  // Mount-time bootstrap. Three entry points, in priority order:
  //   1. Router state from /builder      (just clicked Run)
  //   2. Direct inputs in URL search     (deep-link to a custom run)
  //   3. Published strategy id in URL    (deep-link to a published run)
  // Anything else bounces back to /builder.
  useEffect(() => {
    const startTimer = window.setTimeout(() => {
      if (fromBuilder && isBuilderState(location.state)) {
        const req = builderStateToRequest(location.state.request);
        stream.run(req).catch(ignoreRunError);
        return;
      }

      if (directInputs) {
        stream.run(builderStateToRequest(directInputs)).catch(ignoreRunError);
        return;
      }

      if (strategyId) {
        const startParam = searchParams.get('start');
        const endParam = searchParams.get('end');
        const start =
          startParam && !isNaN(new Date(startParam).getTime()) ? startParam : threeYearsAgo();
        const end = endParam && !isNaN(new Date(endParam).getTime()) ? endParam : todayISO();

        apiClient
          .get<PublishedStrategy[]>('/publishedStrategies')
          .then((strategies) => {
            const s = strategies.find((s) => s.strategyID === strategyId);
            if (!s) {
              void navigate('/builder', { replace: true });
              return;
            }
            const next = strategyToBuilderState(s, start, end);
            setBuilderState(next);
            stream.run(builderStateToRequest(next)).catch(ignoreRunError);
          })
          .catch(() => {
            void navigate('/builder', { replace: true });
          });
        return;
      }

      void navigate('/builder', { replace: true });
    }, 0);

    return () => window.clearTimeout(startTimer);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // When a stream result lands, take a snapshot for the chart and
  // bump the run counter. The state writes are deferred to a
  // microtask so we satisfy `react-hooks/set-state-in-effect` (the
  // rule is a guard against cascading sync renders, not a ban on
  // ever updating React state from a stream lifecycle).
  useEffect(() => {
    if (stream.status !== 'finishing' && stream.status !== 'success') return;
    const fresh = stream.result;
    if (!fresh) return;
    if (fresh === lastResult) return;

    const id = window.setTimeout(() => {
      setLastResult(fresh);
      setSelectedDate(null);
      setHoveredDate(null);
      setRunCounter((c) => c + 1);

      // Write current inputs into the URL so the page is shareable
      // and reload-stable. We don't use the server-returned
      // strategyID because there's no GET-by-id endpoint to look up
      // a one-off (non-published) strategy.
      if (builderState) {
        const next = new URLSearchParams();
        next.set('expr', builderState.factorExpression);
        next.set('name', builderState.factorName);
        next.set('start', builderState.backtestStart);
        next.set('end', builderState.backtestEnd);
        next.set('interval', builderState.rebalanceInterval);
        next.set('num', String(builderState.numAssets));
        next.set('universe', builderState.assetUniverse);
        setSearchParams(next, { replace: true });
      }
    }, 0);
    return () => window.clearTimeout(id);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [stream.status, stream.result]);

  // Strategy chart series — derived from the current result.
  const strategyPoints = useMemo(
    () => (lastResult ? snapshotsToStrategyPoints(lastResult) : []),
    [lastResult],
  );

  // Pull the SPY benchmark for the current window. Always SPY;
  // benchmark selection isn't a knob in this iteration.
  const benchmarkReq = useMemo(() => {
    if (!builderState) return null;
    return {
      symbol: 'SPY',
      start: builderState.backtestStart,
      end: builderState.backtestEnd,
      granularity: builderState.rebalanceInterval,
    };
  }, [builderState]);
  const { points: benchmarkPoints } = useBenchmarkSeries(benchmarkReq);

  const series = useMemo<ChartSeries[]>(() => {
    const out: ChartSeries[] = [];
    if (strategyPoints.length > 0) {
      out.push({
        id: 'strategy',
        label: builderState?.factorName ?? 'Strategy',
        colorVar: 'var(--color-gain)',
        points: strategyPoints,
      });
    }
    if (benchmarkPoints && benchmarkPoints.length > 0) {
      out.push({
        id: 'benchmark',
        label: 'SPY',
        colorVar: 'var(--color-accent)',
        points: benchmarkPoints,
        dashed: true,
      });
    }
    return out;
  }, [strategyPoints, benchmarkPoints, builderState?.factorName]);

  const totalReturn = useMemo(() => {
    if (strategyPoints.length === 0) return null;
    return strategyPoints[strategyPoints.length - 1]?.pctReturn ?? null;
  }, [strategyPoints]);

  // Day inspector — sourced from the locked or hovered date if any,
  // else the latest snapshot.
  const inspectorDate = selectedDate ?? hoveredDate ?? null;
  const inspector = useMemo(() => {
    if (!lastResult) return null;
    if (!inspectorDate) return null;
    return snapshotToHoldings(lastResult, inspectorDate);
  }, [lastResult, inspectorDate]);

  // Trigger a rerun from the panel.
  const handleRerun = useCallback(
    (next: BuilderState) => {
      setBuilderState(next);
      setPanelOpen(false);
      stream.run(builderStateToRequest(next)).catch(ignoreRunError);
    },
    [stream],
  );

  // Show the full overlay only when the page hasn't yet rendered a
  // result. Subsequent runs (lastResult is set) get the inline
  // progress bar so the chart stays visible.
  const showFullOverlay = lastResult === null;
  const isRunning = stream.status === 'streaming' || stream.status === 'finishing';

  if (!fromBuilder && !strategyId && !directInputs) return null;

  return (
    <>
      {showFullOverlay && (
        <BacktestLoadingOverlay
          status={stream.status}
          steps={stream.steps}
          error={stream.error}
          totalMs={stream.totalMs}
          onClose={stream.reset}
        />
      )}

      {!showFullOverlay && (
        <InlineProgress
          status={stream.status}
          steps={stream.steps}
          error={stream.error}
          onDismissError={stream.reset}
        />
      )}

      <div className="mx-auto flex max-w-[1400px] flex-col gap-6 px-6 pb-16 pt-6">
        {/* Hero chart card */}
        <motion.section
          initial={{ opacity: 0, y: 6 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.4, ease: 'easeOut' }}
          className="relative h-[68vh] min-h-[480px] overflow-hidden rounded-xl border border-border bg-card"
        >
          {/* Configure / re-run — bottom-left corner of the chart
              card. Off the right side to avoid the y-axis labels +
              right-edge price pill, off the top to avoid the
              headline KPI. */}
          {builderState && (
            <div className="absolute bottom-5 left-5 z-20">
              <Button
                variant="outline"
                size="sm"
                onClick={() => setPanelOpen(true)}
                disabled={isRunning}
              >
                Configure & re-run
              </Button>
            </div>
          )}

          {lastResult ? (
            <EquityChart
              series={series}
              hoveredDate={hoveredDate}
              onHover={setHoveredDate}
              selectedDate={selectedDate}
              onSelectDate={setSelectedDate}
              runKey={String(runCounter)}
            />
          ) : (
            <div className="flex h-full w-full items-center justify-center">
              <div className="flex flex-col items-center gap-3 text-center">
                <div className="h-2 w-2 animate-pulse rounded-full bg-accent" />
                <p className="text-sm text-muted-foreground">Booting up backtest…</p>
              </div>
            </div>
          )}
        </motion.section>

        {/* Metrics row */}
        {lastResult && (
          <MetricsBar
            sharpe={lastResult.sharpeRatio ?? null}
            annualizedReturn={lastResult.annualizedReturn ?? null}
            annualizedStdev={lastResult.annualizedStandardDeviation ?? null}
            totalReturn={totalReturn}
          />
        )}

        {/* Day inspector */}
        {lastResult && (
          <DayInspector
            date={inspectorDate}
            holdings={inspector?.holdings ?? []}
            pctChange={inspector?.pctChange ?? null}
            onClose={() => {
              setSelectedDate(null);
              setHoveredDate(null);
            }}
          />
        )}
      </div>

      {builderState && (
        <RerunPanel
          // Remount on every successful run so the form's local
          // state re-derives from the fresh `initial`. Prevents the
          // form from showing stale values after a deep-link load
          // resolves async.
          key={`${runCounter}`}
          open={panelOpen}
          onClose={() => setPanelOpen(false)}
          initial={builderState}
          universes={universes ?? null}
          busy={isRunning}
          onRun={handleRerun}
        />
      )}
    </>
  );
}
