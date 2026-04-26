import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';

import { DateRangeSection } from './DateRangeSection';
import { FactorExpressionSection } from './FactorExpressionSection';
import { NumAssetsSection } from './NumAssetsSection';
import { FACTOR_PRESETS } from './presets';
import { RebalanceSection } from './RebalanceSection';
import { SummaryRail } from './SummaryRail';
import type { AssetUniverse, RebalanceInterval } from './types';
import { UniverseSection } from './UniverseSection';
import { apiClient } from '@/lib/api';

// Backtest Builder. Five-step input wizard rendered as a single
// scrollable page (no actual scrolling between steps — they're all
// visible at once). The right rail is the live "preview": it
// recomputes cost + a one-line summary as inputs change, but does NOT
// kick off a real backtest. The actual run happens when the user
// clicks Run, which navigates to /backtest with the request in router
// state.
//
// Rationale on what's "interactive": the things we preview are the
// things whose effect on the run is non-obvious in isolation —
// universe size × range × cadence collapses into a single compute
// estimate; num-assets becomes a fraction of the universe. Dates and
// rebalance pick the cheapest possible affordances (native picker,
// segmented control) because there's nothing to preview about them
// individually.
export function BuilderPage(): React.ReactNode {
  const { data: universes, isLoading } = useQuery({
    queryKey: ['assetUniverses'],
    queryFn: () => apiClient.get<AssetUniverse[]>('/assetUniverses'),
  });

  const today = new Date().toISOString().split('T')[0];
  const ninetyDaysAgo = (() => {
    const d = new Date();
    d.setDate(d.getDate() - 90);
    return d.toISOString().split('T')[0];
  })();

  const defaultPreset = FACTOR_PRESETS[0];
  const [factorExpression, setFactorExpression] = useState(defaultPreset.expression);
  const [factorName, setFactorName] = useState(defaultPreset.name);
  const [presetId, setPresetId] = useState<string | null>(defaultPreset.id);
  // null = "user hasn't picked yet, use the first universe once it
  // loads". Storing the unset state explicitly keeps the default
  // derivation pure — no useEffect-driven setState which fights React 19.
  const [pickedUniverse, setPickedUniverse] = useState<string | null>(null);
  const [backtestStart, setBacktestStart] = useState(ninetyDaysAgo);
  const [backtestEnd, setBacktestEnd] = useState(today);
  const [rebalanceInterval, setRebalanceInterval] = useState<RebalanceInterval>('monthly');
  const [numAssets, setNumAssets] = useState(10);

  const assetUniverse = pickedUniverse ?? universes?.[0]?.code ?? '';
  const universeSize =
    universes?.find((u) => u.code === assetUniverse)?.numAssets ?? 0;

  const canRun =
    factorExpression.trim().length > 0 &&
    factorName.trim().length > 0 &&
    assetUniverse.length > 0 &&
    numAssets >= 3 &&
    backtestStart < backtestEnd;

  return (
    <div className="mx-auto max-w-7xl px-6 py-10">
      <header className="mb-8">
        <h1 className="text-2xl font-semibold tracking-tight text-foreground">
          Build your strategy
        </h1>
      </header>

      <div className="grid grid-cols-1 gap-4 lg:grid-cols-[minmax(0,1fr)_340px]">
        <div className="flex flex-col gap-4">
          <FactorExpressionSection
            expression={factorExpression}
            setExpression={setFactorExpression}
            name={factorName}
            setName={setFactorName}
            presetId={presetId}
            setPresetId={setPresetId}
          />
          <UniverseSection
            universes={universes ?? []}
            selectedCode={assetUniverse}
            setSelectedCode={setPickedUniverse}
            loading={isLoading}
          />
          <DateRangeSection
            start={backtestStart}
            setStart={setBacktestStart}
            end={backtestEnd}
            setEnd={setBacktestEnd}
          />
          <RebalanceSection value={rebalanceInterval} setValue={setRebalanceInterval} />
          <NumAssetsSection
            value={numAssets}
            setValue={setNumAssets}
            universeSize={universeSize}
          />
        </div>

        <SummaryRail
          state={{
            factorExpression,
            factorName,
            assetUniverse,
            backtestStart,
            backtestEnd,
            rebalanceInterval,
            numAssets,
          }}
          universeSize={universeSize}
          canRun={canRun}
        />
      </div>
    </div>
  );
}
