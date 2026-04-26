import { AnimatePresence, motion } from 'framer-motion';

import type { Step, StreamStatus } from '@/lib/backtest-stream/types';

interface Props {
  status: StreamStatus;
  steps: Step[];
  error: string | null;
  onDismissError: () => void;
}

// Thin top-of-page progress bar + floating step pill, used during
// re-runs (the page already has a result on screen, so the full
// overlay would be obnoxious). Fades out 600ms after `success`.
export function InlineProgress({ status, steps, error, onDismissError }: Props): React.ReactNode {
  const visible = status === 'streaming' || status === 'finishing';
  const showError = status === 'error' && error !== null;
  const currentStep =
    [...steps].reverse().find((s) => s.status === 'in_progress') ??
    [...steps].reverse().find((s) => s.status === 'completed') ??
    null;
  const completed = steps.filter((s) => s.status === 'completed').length;
  // We don't know how many steps will be emitted up front. Use a
  // sliding "estimated 5 steps" denominator that ticks up if we
  // exceed it — the bar still moves left-to-right monotonically.
  const denom = Math.max(5, steps.length);
  const pct = Math.min(100, (completed / denom) * 100);

  return (
    <AnimatePresence>
      {visible && (
        <>
          {/* Slim progress bar fixed to top of viewport. */}
          <motion.div
            key="bar"
            initial={{ opacity: 0, y: -2 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0 }}
            className="pointer-events-none fixed inset-x-0 top-0 z-50 h-0.5 bg-transparent"
          >
            <motion.div
              className="h-full bg-accent shadow-[0_0_8px_var(--color-accent)]"
              initial={{ width: 0 }}
              animate={{ width: `${pct}%` }}
              transition={{ duration: 0.4, ease: 'easeOut' }}
            />
          </motion.div>

          {/* Floating step pill in the bottom-right corner. */}
          <motion.div
            key="pill"
            initial={{ opacity: 0, y: 8 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: 8 }}
            transition={{ duration: 0.22, ease: 'easeOut' }}
            className="fixed bottom-6 right-6 z-50 flex items-center gap-3 rounded-full border border-border bg-elevated/90 px-4 py-2 shadow-xl backdrop-blur"
          >
            <span className="relative flex h-2 w-2">
              <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-accent opacity-60" />
              <span className="relative inline-flex h-2 w-2 rounded-full bg-accent" />
            </span>
            <div className="flex flex-col">
              <span className="font-mono text-[10px] uppercase tracking-[0.16em] text-subtle-foreground">
                Running backtest
              </span>
              <span className="text-sm text-foreground">
                {currentStep?.label ?? 'Starting…'}
              </span>
            </div>
            <span className="font-mono text-xs tabular-nums text-muted-foreground">
              {completed}/{denom}
            </span>
          </motion.div>
        </>
      )}

      {showError && (
        <motion.div
          key="err"
          initial={{ opacity: 0, y: 8 }}
          animate={{ opacity: 1, y: 0 }}
          exit={{ opacity: 0, y: 8 }}
          transition={{ duration: 0.22, ease: 'easeOut' }}
          className="fixed bottom-6 right-6 z-50 flex max-w-sm items-start gap-3 rounded-lg border border-loss/40 bg-elevated px-4 py-3 shadow-xl backdrop-blur"
          role="alert"
        >
          <span className="mt-1 inline-block h-2 w-2 rounded-full bg-loss" />
          <div className="flex flex-col">
            <span className="font-mono text-[10px] uppercase tracking-[0.16em] text-loss">
              Backtest failed
            </span>
            <span className="text-sm text-foreground">{error}</span>
          </div>
          <button
            type="button"
            onClick={onDismissError}
            className="ml-2 rounded-md border border-border px-2 py-1 text-xs text-muted-foreground hover:bg-card"
          >
            Dismiss
          </button>
        </motion.div>
      )}
    </AnimatePresence>
  );
}
