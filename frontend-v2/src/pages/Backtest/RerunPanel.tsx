import { AnimatePresence, motion } from 'framer-motion';
import { useMemo, useState } from 'react';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { cn } from '@/lib/utils';
import { FACTOR_PRESETS } from '@/pages/Builder/presets';
import type { AssetUniverse, BuilderState, RebalanceInterval } from '@/pages/Builder/types';

interface Props {
  open: boolean;
  onClose: () => void;
  initial: BuilderState;
  universes: AssetUniverse[] | null;
  // True while the parent's stream is running. The panel disables its
  // run button to prevent stacking parallel runs over the same chart.
  busy: boolean;
  onRun: (next: BuilderState) => void;
}

const REBALANCE_OPTIONS: { id: RebalanceInterval; label: string }[] = [
  { id: 'daily', label: 'Daily' },
  { id: 'weekly', label: 'Weekly' },
  { id: 'monthly', label: 'Monthly' },
  { id: 'yearly', label: 'Yearly' },
];

// Right-side slide-out drawer. Holds the same fields as the full
// builder, condensed for in-place tweaking. Pre-fills with the
// currently-displayed run so "tweak one knob" is one click away.
export function RerunPanel({ open, onClose, initial, universes, busy, onRun }: Props): React.ReactNode {
  const [factorExpression, setFactorExpression] = useState(initial.factorExpression);
  const [factorName, setFactorName] = useState(initial.factorName);
  const [assetUniverse, setAssetUniverse] = useState(initial.assetUniverse);
  const [backtestStart, setBacktestStart] = useState(initial.backtestStart);
  const [backtestEnd, setBacktestEnd] = useState(initial.backtestEnd);
  const [rebalanceInterval, setRebalanceInterval] = useState<RebalanceInterval>(
    initial.rebalanceInterval,
  );
  const [numAssets, setNumAssets] = useState(initial.numAssets);

  // Resetting the form when `initial` changes is handled by the
  // parent remounting this component (key={initial-snapshot}). Doing
  // it here with useEffect would call setState in an effect — flagged
  // by react-hooks/set-state-in-effect — and would also fight the
  // user's in-progress edits on every render.

  const canRun = useMemo(
    () =>
      factorExpression.trim().length > 0 &&
      factorName.trim().length > 0 &&
      assetUniverse.length > 0 &&
      numAssets >= 3 &&
      backtestStart < backtestEnd,
    [factorExpression, factorName, assetUniverse, numAssets, backtestStart, backtestEnd],
  );

  return (
    <AnimatePresence>
      {open && (
        <>
          {/* Backdrop. Faint — the chart stays visible behind it. */}
          <motion.div
            className="fixed inset-0 z-30 bg-black/40 backdrop-blur-[2px]"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.18 }}
            onClick={onClose}
            aria-hidden
          />

          <motion.aside
            className="fixed right-0 top-0 z-40 flex h-full w-full max-w-md flex-col border-l border-border bg-card shadow-2xl"
            initial={{ x: '100%' }}
            animate={{ x: 0 }}
            exit={{ x: '100%' }}
            transition={{ type: 'spring', stiffness: 380, damping: 38, mass: 0.6 }}
            role="dialog"
            aria-label="Configure backtest"
          >
            <header className="flex items-center justify-between border-b border-border px-6 py-5">
              <div>
                <p className="text-[11px] uppercase tracking-[0.18em] text-subtle-foreground">
                  Configure
                </p>
                <h3 className="mt-0.5 text-lg font-semibold tracking-tight text-foreground">
                  Run a new backtest
                </h3>
              </div>
              <button
                type="button"
                onClick={onClose}
                className="rounded-md border border-border px-2 py-1 text-xs text-muted-foreground hover:bg-elevated"
              >
                Close
              </button>
            </header>

            <div className="flex-1 overflow-y-auto px-6 py-5">
              <div className="flex flex-col gap-6">
                {/* Factor presets — compact chips. */}
                <Section label="Strategy">
                  <div className="flex flex-wrap gap-2">
                    {FACTOR_PRESETS.map((preset) => {
                      const active = preset.expression === factorExpression;
                      return (
                        <button
                          key={preset.id}
                          type="button"
                          onClick={() => {
                            setFactorExpression(preset.expression);
                            setFactorName(preset.name);
                          }}
                          className={cn(
                            'rounded-full border px-3 py-1.5 text-xs transition-colors',
                            active
                              ? 'border-accent bg-accent/15 text-foreground'
                              : 'border-border bg-elevated text-muted-foreground hover:border-border-strong',
                          )}
                        >
                          {preset.name}
                        </button>
                      );
                    })}
                  </div>
                  <Input
                    placeholder="Strategy name"
                    value={factorName}
                    onChange={(e) => setFactorName(e.target.value)}
                  />
                  <textarea
                    value={factorExpression}
                    onChange={(e) => setFactorExpression(e.target.value)}
                    rows={4}
                    className="w-full resize-none rounded-md border border-border bg-elevated/70 px-3 py-2 font-mono text-sm text-foreground placeholder:text-subtle-foreground focus:border-border-strong focus:outline-none"
                    placeholder="Factor expression"
                  />
                </Section>

                <Section label="Universe">
                  <select
                    value={assetUniverse}
                    onChange={(e) => setAssetUniverse(e.target.value)}
                    className="h-11 w-full rounded-md border border-border bg-elevated/70 px-3 text-sm text-foreground focus:border-border-strong focus:outline-none"
                  >
                    {(universes ?? []).map((u) => (
                      <option key={u.code} value={u.code} className="bg-elevated">
                        {u.displayName} ({u.numAssets})
                      </option>
                    ))}
                  </select>
                </Section>

                <Section label="Window">
                  <div className="grid grid-cols-2 gap-3">
                    <Input
                      type="date"
                      value={backtestStart}
                      max={backtestEnd}
                      onChange={(e) => setBacktestStart(e.target.value)}
                    />
                    <Input
                      type="date"
                      value={backtestEnd}
                      min={backtestStart}
                      onChange={(e) => setBacktestEnd(e.target.value)}
                    />
                  </div>
                </Section>

                <Section label="Rebalance">
                  <div className="grid grid-cols-4 gap-1 rounded-md border border-border bg-elevated/40 p-1">
                    {REBALANCE_OPTIONS.map((opt) => (
                      <button
                        key={opt.id}
                        type="button"
                        onClick={() => setRebalanceInterval(opt.id)}
                        className={cn(
                          'rounded-sm py-1.5 text-xs font-medium transition-colors',
                          rebalanceInterval === opt.id
                            ? 'bg-accent text-accent-foreground'
                            : 'text-muted-foreground hover:text-foreground',
                        )}
                      >
                        {opt.label}
                      </button>
                    ))}
                  </div>
                </Section>

                <Section label="Holdings">
                  <div className="flex items-center gap-3">
                    <input
                      type="range"
                      min={3}
                      max={50}
                      value={numAssets}
                      onChange={(e) => setNumAssets(Number(e.target.value))}
                      className="flex-1 accent-[var(--color-accent)]"
                    />
                    <span className="w-12 text-right font-mono text-sm text-foreground">
                      {numAssets}
                    </span>
                  </div>
                </Section>
              </div>
            </div>

            <footer className="border-t border-border px-6 py-4">
              <Button
                size="lg"
                className="w-full"
                disabled={!canRun || busy}
                onClick={() =>
                  onRun({
                    factorExpression,
                    factorName,
                    assetUniverse,
                    backtestStart,
                    backtestEnd,
                    rebalanceInterval,
                    numAssets,
                  })
                }
              >
                {busy ? 'Running…' : 'Run backtest'}
              </Button>
            </footer>
          </motion.aside>
        </>
      )}
    </AnimatePresence>
  );
}

function Section({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}): React.ReactNode {
  return (
    <div className="flex flex-col gap-2.5">
      <span className="text-[11px] uppercase tracking-[0.18em] text-subtle-foreground">
        {label}
      </span>
      {children}
    </div>
  );
}
