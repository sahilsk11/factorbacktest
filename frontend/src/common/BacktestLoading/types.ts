import { BacktestResponse } from "models";

// Wire-format mirror of internal/progress.Event on the backend. If the BE
// renames any field, this file is the one place to update on the FE.
export type BacktestStreamEvent =
  | { type: "step_started"; step: string; label: string }
  | { type: "step_completed"; step: string; durationMs?: number }
  | { type: "result"; result: BacktestResponse }
  | { type: "error"; error: string };

export type StepStatus = "in_progress" | "completed" | "error";

export interface Step {
  id: string;
  label: string;
  status: StepStatus;
  durationMs?: number;
}

// Stream lifecycle:
//   idle       — no run in progress
//   streaming  — request open, step events flowing
//   finishing  — terminal `result` received; we deliberately keep the overlay
//                up briefly so the user actually sees every check turn green
//                and a "completed in Xs" line. This is a real status, not
//                local UI state, so the overlay's visibility stays a pure
//                function of props (no flicker on parent re-renders).
//   success    — finishing window elapsed, overlay should hide
//   error      — terminal `error` received OR pre-stream HTTP error
export type StreamStatus = "idle" | "streaming" | "finishing" | "success" | "error";
