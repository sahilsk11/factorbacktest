import { useCallback, useEffect, useRef, useState } from 'react';

import { parseSSEStream } from './sse';
import type { BacktestRequest, BacktestResponse, Step, StreamStatus } from './types';
import { apiClient } from '@/lib/api';

export interface UseBacktestStream {
  status: StreamStatus;
  steps: Step[];
  error: string | null;
  result: BacktestResponse | null;
  totalMs: number | null;
  run: (req: BacktestRequest) => Promise<BacktestResponse>;
  reset: () => void;
}

// How long the overlay lingers after the terminal `result` event so the user
// can read the per-step durations and "completed in Xs" line.
const FINISHING_HOLD_MS = 2000;

export function useBacktestStream(): UseBacktestStream {
  const [status, setStatus] = useState<StreamStatus>('idle');
  const [steps, setSteps] = useState<Step[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [result, setResult] = useState<BacktestResponse | null>(null);
  const [totalMs, setTotalMs] = useState<number | null>(null);

  const abortRef = useRef<AbortController | null>(null);
  const finishingTimerRef = useRef<number | null>(null);

  useEffect(() => {
    return () => {
      abortRef.current?.abort();
      abortRef.current = null;
      if (finishingTimerRef.current !== null) {
        window.clearTimeout(finishingTimerRef.current);
        finishingTimerRef.current = null;
      }
    };
  }, []);

  const reset = useCallback(() => {
    abortRef.current?.abort();
    abortRef.current = null;
    if (finishingTimerRef.current !== null) {
      window.clearTimeout(finishingTimerRef.current);
      finishingTimerRef.current = null;
    }
    setStatus('idle');
    setSteps([]);
    setError(null);
    setResult(null);
    setTotalMs(null);
  }, []);

  const run = useCallback(async (req: BacktestRequest): Promise<BacktestResponse> => {
    abortRef.current?.abort();
    if (finishingTimerRef.current !== null) {
      window.clearTimeout(finishingTimerRef.current);
      finishingTimerRef.current = null;
    }
    const ctrl = new AbortController();
    abortRef.current = ctrl;

    const startedAtMs = Date.now();
    setStatus('streaming');
    setSteps([]);
    setError(null);
    setResult(null);
    setTotalMs(null);

    let response: Response;
    try {
      response = await apiClient.postStream('/backtest/stream', req, ctrl.signal);
    } catch (err) {
      const msg = (err as Error).message || 'network error';
      setError(msg);
      setTotalMs(Date.now() - startedAtMs);
      setStatus('error');
      throw err;
    }

    if (!response.ok || !response.body) {
      let msg = `request failed: ${response.status}`;
      try {
        const j = (await response.json()) as { error?: string };
        if (j.error) msg = j.error;
      } catch {
        /* non-JSON body */
      }
      setError(msg);
      setTotalMs(Date.now() - startedAtMs);
      setStatus('error');
      throw new Error(msg);
    }

    try {
      for await (const ev of parseSSEStream(response.body, ctrl.signal)) {
        if (ev.type === 'step_started') {
          setSteps((prev) => [...prev, { id: ev.step, label: ev.label, status: 'in_progress' }]);
        } else if (ev.type === 'step_completed') {
          setSteps((prev) =>
            prev.map((s) =>
              s.id === ev.step && s.status === 'in_progress'
                ? { ...s, status: 'completed', durationMs: ev.durationMs }
                : s,
            ),
          );
        } else if (ev.type === 'result') {
          setResult(ev.result);
          setTotalMs(Date.now() - startedAtMs);
          setStatus('finishing');
          finishingTimerRef.current = window.setTimeout(() => {
            finishingTimerRef.current = null;
            setStatus('success');
          }, FINISHING_HOLD_MS);
          return ev.result;
        } else if (ev.type === 'error') {
          setSteps((prev) =>
            prev.map((s) => (s.status === 'in_progress' ? { ...s, status: 'error' } : s)),
          );
          setError(ev.error);
          setTotalMs(Date.now() - startedAtMs);
          setStatus('error');
          throw new Error(ev.error);
        }
      }
      const msg = 'backtest stream ended without a result';
      setError(msg);
      setTotalMs(Date.now() - startedAtMs);
      setStatus('error');
      throw new Error(msg);
    } finally {
      if (abortRef.current === ctrl) abortRef.current = null;
    }
  }, []);

  return { status, steps, error, result, totalMs, run, reset };
}
