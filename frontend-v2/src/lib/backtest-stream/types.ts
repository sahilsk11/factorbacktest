// Wire-format mirror of internal/progress.Event on the backend.
// Source of truth: Go server. Update here if the backend renames fields.

export interface BacktestResponse {
  factorName: string;
  strategyID: string;
  backtestSnapshots: Record<string, unknown>;
  latestHoldings: unknown;
  sharpeRatio?: number;
  annualizedReturn?: number;
  annualizedStandardDeviation?: number;
}

export type BacktestStreamEvent =
  | { type: 'step_started'; step: string; label: string }
  | { type: 'step_completed'; step: string; durationMs?: number }
  | { type: 'result'; result: BacktestResponse }
  | { type: 'error'; error: string };

export type StepStatus = 'in_progress' | 'completed' | 'error';

export interface Step {
  id: string;
  label: string;
  status: StepStatus;
  durationMs?: number;
}

// Stream lifecycle:
//   idle       — no run in progress
//   streaming  — request open, step events flowing
//   finishing  — terminal `result` received; overlay stays up briefly so the
//                user sees every check turn green and the "completed in Xs" line
//   success    — finishing window elapsed, overlay hides
//   error      — terminal `error` received OR pre-stream HTTP error
export type StreamStatus = 'idle' | 'streaming' | 'finishing' | 'success' | 'error';

export interface BacktestRequest {
  factorOptions: {
    expression: string;
    name: string;
  };
  backtestStart: string;
  backtestEnd: string;
  samplingIntervalUnit: string;
  startCash: number;
  numSymbols?: number;
  assetUniverse?: string | null;
}
