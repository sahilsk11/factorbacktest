import { createPortal } from "react-dom";
import styles from "./BacktestLoadingOverlay.module.css";
import { Step, StreamStatus } from "./types";

interface Props {
  status: StreamStatus;
  steps: Step[];
  error: string | null;
  totalMs: number | null;
  // Called when the user dismisses an error overlay. The hook's reset()
  // is the natural target — clears state and returns the overlay to idle.
  onClose: () => void;
}

// BacktestLoadingOverlay renders a fullscreen progress UI for a streaming
// backtest. Visibility is a *pure function of props* — there's intentionally
// no local timer state here. The hook owns the post-success "finishing"
// hold via its own status, which avoids the flicker class of bug where a
// parent re-render briefly remounts the overlay and resets a local linger.
//
// We render via createPortal so the overlay sits above the rest of the SPA
// without callers needing to lift state or restructure layout.
export function BacktestLoadingOverlay({
  status,
  steps,
  error,
  totalMs,
  onClose,
}: Props): JSX.Element | null {
  const visible =
    status === "streaming" || status === "finishing" || status === "error";
  if (!visible) return null;

  // SSR guard for any future move off CRA.
  if (typeof document === "undefined") return null;

  const isError = status === "error";
  const isFinishing = status === "finishing";

  const titleText = isError
    ? "Backtest failed"
    : isFinishing
      ? "Backtest complete"
      : "Running backtest";

  // Subtitle is only meaningful during the active run and the error case.
  // During the finishing hold we move the completion summary to the
  // bottom of the card, so the top stays clean.
  const subtitleText = isError
    ? "We couldn't finish the run. Adjust your inputs and try again."
    : isFinishing
      ? null
      : "Hang tight while we crunch the numbers.";

  const overlay = (
    <div className={styles.scrim} role="status" aria-live="polite">
      <div className={styles.card}>
        <h3 className={styles.title}>{titleText}</h3>
        {/* Always render the subtitle slot (with a non-breaking space when
            empty) so the reserved min-height in CSS keeps the card from
            jumping when we drop the subtitle text during finishing. */}
        <p className={styles.subtitle}>{subtitleText ?? "\u00A0"}</p>

        <ul className={styles.steps}>
          {steps.map((step) => (
            <li
              key={step.id}
              className={`${styles.step} ${
                step.status === "completed"
                  ? styles.completed
                  : step.status === "error"
                    ? styles.error
                    : ""
              }`}
            >
              <span className={styles.indicator}>
                {step.status === "in_progress" && <span className={styles.dot} />}
                {step.status === "completed" && (
                  <span className={styles.check_circle}>
                    <svg width="11" height="11" viewBox="0 0 12 12">
                      <polyline
                        className={styles.check_glyph}
                        points="2.5,6.5 5,9 9.5,3.5"
                      />
                    </svg>
                  </span>
                )}
                {step.status === "error" && (
                  <span className={styles.error_circle}>
                    <svg width="11" height="11" viewBox="0 0 12 12">
                      <line className={styles.error_glyph} x1="3" y1="3" x2="9" y2="9" />
                      <line className={styles.error_glyph} x1="9" y1="3" x2="3" y2="9" />
                    </svg>
                  </span>
                )}
              </span>
              <span className={styles.label}>{step.label}</span>
              {step.status === "completed" && step.durationMs !== undefined ? (
                <span className={styles.duration}>{formatDuration(step.durationMs)}</span>
              ) : null}
            </li>
          ))}
        </ul>

        {error ? <p className={styles.error_message}>{error}</p> : null}

        {isFinishing ? (
          <p className={styles.completion_summary}>
            All done — completed in {formatDuration(totalMs ?? 0)}.
          </p>
        ) : null}

        {isError ? (
          <div className={styles.actions}>
            <button type="button" className={styles.close_btn} onClick={onClose}>
              Close
            </button>
          </div>
        ) : null}
      </div>
    </div>
  );

  return createPortal(overlay, document.body);
}

// formatDuration renders a millisecond count in a human-friendly unit:
//   < 1s  → "120ms"   (whole milliseconds; sub-second steps are common)
//   ≥ 1s  → "1.24s"   (two decimals; matches typical perf tooling)
// Keeping this internal to the overlay file because it's the only consumer.
function formatDuration(ms: number): string {
  if (ms < 1000) return `${Math.max(0, Math.round(ms))}ms`;
  return `${(ms / 1000).toFixed(2)}s`;
}
