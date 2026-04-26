import { useCallback, useEffect, useRef, useState } from "react";
import { endpoint } from "config";
import { BacktestRequest, BacktestResponse } from "models";
import { parseSSEStream } from "./sse";
import { Step, StreamStatus } from "./types";

export interface UseBacktestStream {
  status: StreamStatus;
  steps: Step[];
  error: string | null;
  result: BacktestResponse | null;
  // totalMs is the wall-clock duration from request open to terminal event,
  // populated on success/error. Used by the overlay to display
  // "completed in X.Xs".
  totalMs: number | null;
  // run kicks off a /backtest/stream request, drives the step state, and
  // resolves with the final BacktestResponse on a `result` event. It rejects
  // if the request fails to start (HTTP non-2xx before the stream begins) or
  // if the stream ends with an `error` event.
  run: (req: BacktestRequest, accessToken?: string | null) => Promise<BacktestResponse>;
  reset: () => void;
}

// FINISHING_HOLD_MS is the grace period the overlay stays up after the
// terminal `result` event so users actually register the completion
// summary. 2s is enough time for the eye to scan the per-step durations
// and the total without dragging.
const FINISHING_HOLD_MS = 2000;

// useBacktestStream owns the fetch + SSE parsing + per-step UI state. It's
// intentionally a thin wrapper so the consumer (Form.tsx) can keep calling
// the same downstream handlers it always has — only the transport changes.
export function useBacktestStream(): UseBacktestStream {
  const [status, setStatus] = useState<StreamStatus>("idle");
  const [steps, setSteps] = useState<Step[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [result, setResult] = useState<BacktestResponse | null>(null);
  const [totalMs, setTotalMs] = useState<number | null>(null);

  // Tracks the AbortController for any in-flight stream so unmount /
  // re-invocation cancels the previous request. We store it in a ref because
  // updates inside async code shouldn't trigger renders.
  const abortRef = useRef<AbortController | null>(null);
  // Tracks the post-success "finishing" timer so a fast double-run cancels
  // the prior hold instead of letting it linger into the next stream's UI.
  const finishingTimerRef = useRef<number | null>(null);

  // Cancel any in-flight request and pending timers on unmount. React
  // StrictMode in dev will mount → unmount → mount, which could otherwise
  // leak a fetch or leave a setTimeout firing into a dead component.
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
    setStatus("idle");
    setSteps([]);
    setError(null);
    setResult(null);
    setTotalMs(null);
  }, []);

  const run = useCallback(
    async (req: BacktestRequest, accessToken?: string | null): Promise<BacktestResponse> => {
      // Cancel any prior stream before starting a new one. This handles the
      // double-click case as well as StrictMode re-runs.
      abortRef.current?.abort();
      if (finishingTimerRef.current !== null) {
        window.clearTimeout(finishingTimerRef.current);
        finishingTimerRef.current = null;
      }
      const ctrl = new AbortController();
      abortRef.current = ctrl;

      const startedAtMs = Date.now();

      setStatus("streaming");
      setSteps([]);
      setError(null);
      setResult(null);
      setTotalMs(null);

      let response: Response;
      try {
        response = await fetch(endpoint + "/backtest/stream", {
          method: "POST",
          credentials: "include",
          headers: {
            "Content-Type": "application/json",
            // Bearer is preserved for older direct integrations; the cookie
            // middleware handles authenticated browser sessions.
            Authorization: accessToken ? "Bearer " + accessToken : "",
          },
          body: JSON.stringify(req),
          signal: ctrl.signal,
        });
      } catch (err) {
        const msg = (err as Error).message || "network error";
        setError(msg);
        setTotalMs(Date.now() - startedAtMs);
        setStatus("error");
        throw err;
      }

      // Pre-stream errors (validation 500 with JSON body) come back the same
      // way today's /backtest does. Surface them via the hook's error state
      // *and* throw so the caller can keep its existing inline error UI.
      if (!response.ok || !response.body) {
        let msg = `request failed: ${response.status}`;
        try {
          const j = (await response.json()) as { error?: string };
          if (j.error) msg = j.error;
        } catch {
          /* response wasn't JSON; fall through with status text */
        }
        setError(msg);
        setTotalMs(Date.now() - startedAtMs);
        setStatus("error");
        throw new Error(msg);
      }

      try {
        for await (const ev of parseSSEStream(response.body, ctrl.signal)) {
          if (ev.type === "step_started") {
            setSteps((prev) => [
              ...prev,
              { id: ev.step, label: ev.label, status: "in_progress" },
            ]);
          } else if (ev.type === "step_completed") {
            setSteps((prev) =>
              prev.map((s) =>
                s.id === ev.step && s.status === "in_progress"
                  ? { ...s, status: "completed", durationMs: ev.durationMs }
                  : s,
              ),
            );
          } else if (ev.type === "result") {
            setResult(ev.result);
            setTotalMs(Date.now() - startedAtMs);
            // Enter the "finishing" hold so the overlay can show every
            // check + the "completed in" line. We resolve the promise
            // immediately so the caller's downstream effects (rendering
            // the chart, etc.) fire in parallel — by the time the hold
            // elapses, the page underneath is already up to date.
            setStatus("finishing");
            finishingTimerRef.current = window.setTimeout(() => {
              finishingTimerRef.current = null;
              setStatus("success");
            }, FINISHING_HOLD_MS);
            return ev.result;
          } else if (ev.type === "error") {
            // Mark the currently-in-progress step (if any) as failed so the
            // overlay can highlight where things went wrong.
            setSteps((prev) =>
              prev.map((s) =>
                s.status === "in_progress" ? { ...s, status: "error" } : s,
              ),
            );
            setError(ev.error);
            setTotalMs(Date.now() - startedAtMs);
            setStatus("error");
            throw new Error(ev.error);
          }
        }
        // Stream ended without a terminal event. This shouldn't happen
        // unless the server crashes mid-response, but be defensive.
        const msg = "backtest stream ended without a result";
        setError(msg);
        setTotalMs(Date.now() - startedAtMs);
        setStatus("error");
        throw new Error(msg);
      } finally {
        if (abortRef.current === ctrl) abortRef.current = null;
      }
    },
    [],
  );

  return { status, steps, error, result, totalMs, run, reset };
}
