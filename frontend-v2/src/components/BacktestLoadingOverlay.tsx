import { createPortal } from 'react-dom';

import type { Step, StreamStatus } from '@/lib/backtest-stream/types';

interface Props {
  status: StreamStatus;
  steps: Step[];
  error: string | null;
  totalMs: number | null;
  onClose: () => void;
}

// BacktestLoadingOverlay renders a fullscreen progress UI for a streaming
// backtest. Visibility is a pure function of props — no local timer state.
// Rendered via createPortal so it floats above the page layout.
export function BacktestLoadingOverlay({
  status,
  steps,
  error,
  totalMs,
  onClose,
}: Props): React.ReactNode {
  const visible = status === 'streaming' || status === 'finishing' || status === 'error';
  if (!visible) return null;
  if (typeof document === 'undefined') return null;

  const isError = status === 'error';
  const isFinishing = status === 'finishing';

  const title = isError
    ? 'Backtest failed'
    : isFinishing
      ? 'Backtest complete'
      : 'Running backtest';
  const subtitle = isError
    ? "We couldn't finish the run. Adjust your inputs and try again."
    : isFinishing
      ? null
      : 'Hang tight while we crunch the numbers.';

  const overlay = (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm">
      <div className="w-full max-w-sm rounded-xl border border-border bg-card p-6 shadow-xl">
        <h3 className="text-base font-semibold text-foreground">{title}</h3>
        {/* Reserve the subtitle slot so the card doesn't jump when we drop it during finishing. */}
        <p className="mt-1 min-h-[1.25rem] text-sm text-muted-foreground">{subtitle ?? ' '}</p>

        <ul className="mt-4 flex flex-col gap-2">
          {steps.map((step) => (
            <li key={step.id} className="flex items-center gap-2 text-sm">
              <StepIndicator status={step.status} />
              <span
                className={
                  step.status === 'completed'
                    ? 'text-foreground'
                    : step.status === 'error'
                      ? 'text-destructive'
                      : 'text-muted-foreground'
                }
              >
                {step.label}
              </span>
              {step.status === 'completed' && step.durationMs !== undefined && (
                <span className="ml-auto font-mono text-xs text-subtle-foreground">
                  {formatDuration(step.durationMs)}
                </span>
              )}
            </li>
          ))}
        </ul>

        {error && <p className="mt-3 text-sm text-destructive">{error}</p>}

        {isFinishing && (
          <p className="mt-4 text-sm font-medium text-foreground">
            All done — completed in {formatDuration(totalMs ?? 0)}.
          </p>
        )}

        {isError && (
          <div className="mt-4">
            <button
              type="button"
              onClick={onClose}
              className="w-full rounded-md border border-border px-4 py-2 text-sm font-medium text-foreground hover:bg-accent"
            >
              Close
            </button>
          </div>
        )}
      </div>
    </div>
  );

  return createPortal(overlay, document.body);
}

function StepIndicator({ status }: { status: Step['status'] }): React.ReactNode {
  if (status === 'in_progress') {
    return (
      <span className="flex h-4 w-4 items-center justify-center">
        <span className="h-2 w-2 animate-pulse rounded-full bg-primary" />
      </span>
    );
  }
  if (status === 'completed') {
    return (
      <span className="flex h-4 w-4 items-center justify-center rounded-full bg-primary/10 text-primary">
        <svg width="10" height="10" viewBox="0 0 12 12" fill="none">
          <polyline
            points="2.5,6.5 5,9 9.5,3.5"
            stroke="currentColor"
            strokeWidth="1.8"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
        </svg>
      </span>
    );
  }
  // error
  return (
    <span className="flex h-4 w-4 items-center justify-center rounded-full bg-destructive/10 text-destructive">
      <svg width="10" height="10" viewBox="0 0 12 12" fill="none">
        <line
          x1="3"
          y1="3"
          x2="9"
          y2="9"
          stroke="currentColor"
          strokeWidth="1.8"
          strokeLinecap="round"
        />
        <line
          x1="9"
          y1="3"
          x2="3"
          y2="9"
          stroke="currentColor"
          strokeWidth="1.8"
          strokeLinecap="round"
        />
      </svg>
    </span>
  );
}

function formatDuration(ms: number): string {
  if (ms < 1000) return `${Math.max(0, Math.round(ms))}ms`;
  return `${(ms / 1000).toFixed(2)}s`;
}
