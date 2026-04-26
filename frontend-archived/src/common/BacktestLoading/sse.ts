import { BacktestStreamEvent } from "./types";

// parseSSEStream turns a fetch ReadableStream into an async iterable of
// decoded SSE events. We only support the `data:` line (the BE never emits
// `event:` or `id:`), and we ignore comment lines (`:` prefix).
//
// Note: the WHATWG SSE spec splits frames on a blank line — i.e. \n\n. Some
// proxies normalize line endings to \r\n; we handle both.
export async function* parseSSEStream(
  body: ReadableStream<Uint8Array>,
  signal?: AbortSignal,
): AsyncGenerator<BacktestStreamEvent, void, void> {
  const reader = body.getReader();
  const decoder = new TextDecoder("utf-8");
  let buffer = "";

  try {
    while (true) {
      if (signal?.aborted) {
        // Surface as a clean terminal event rather than throwing — callers
        // already handle "error" events as a normal end-of-stream.
        return;
      }
      const { value, done } = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, { stream: true });

      // Split on either Unix or CRLF blank lines so we work behind both
      // typical reverse proxies and bare Go HTTP servers.
      let sep: number;
      while (
        (sep = indexOfBlankLine(buffer)) !== -1
      ) {
        const frame = buffer.slice(0, sep);
        buffer = buffer.slice(sep + blankLineLength(buffer, sep));
        const event = decodeFrame(frame);
        if (event) yield event;
      }
    }

    // Drain any trailing frame that didn't end with the standard double
    // newline (rare, but cheap to handle).
    const tail = buffer.trim();
    if (tail) {
      const event = decodeFrame(tail);
      if (event) yield event;
    }
  } finally {
    // Releasing the lock is required so the caller can abort or close the
    // body cleanly. The reader itself can't be reused after this.
    try {
      reader.releaseLock();
    } catch {
      /* ignore — already released or stream errored */
    }
  }
}

// indexOfBlankLine returns the index of the first character of a blank-line
// separator (either \n\n or \r\n\r\n) or -1 if not present.
function indexOfBlankLine(s: string): number {
  const a = s.indexOf("\n\n");
  const b = s.indexOf("\r\n\r\n");
  if (a === -1) return b;
  if (b === -1) return a;
  return Math.min(a, b);
}

function blankLineLength(s: string, idx: number): number {
  return s.startsWith("\r\n\r\n", idx) ? 4 : 2;
}

// decodeFrame collapses possibly-multi-line `data:` payloads (per the SSE
// spec joined with \n) and JSON-decodes them. Comment lines and unknown
// field names are ignored.
function decodeFrame(frame: string): BacktestStreamEvent | null {
  const dataLines: string[] = [];
  for (const rawLine of frame.split(/\r?\n/)) {
    if (!rawLine || rawLine.startsWith(":")) continue;
    const colon = rawLine.indexOf(":");
    if (colon === -1) continue;
    const field = rawLine.slice(0, colon);
    if (field !== "data") continue;
    // Per spec, a single leading space after the colon is stripped.
    const value = rawLine.slice(colon + 1).replace(/^ /, "");
    dataLines.push(value);
  }
  if (dataLines.length === 0) return null;
  const payload = dataLines.join("\n");
  try {
    return JSON.parse(payload) as BacktestStreamEvent;
  } catch {
    // A malformed frame from the server shouldn't crash the consumer —
    // skip it and keep reading.
    return null;
  }
}
