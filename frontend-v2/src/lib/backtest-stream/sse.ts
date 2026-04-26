import type { BacktestStreamEvent } from './types';

// parseSSEStream turns a fetch ReadableStream into an async iterable of
// decoded SSE events. Only the `data:` line is supported (the BE never emits
// `event:` or `id:`); comment lines (`:` prefix) are ignored.
export async function* parseSSEStream(
  body: ReadableStream<Uint8Array>,
  signal?: AbortSignal,
): AsyncGenerator<BacktestStreamEvent, void, void> {
  const reader = body.getReader();
  const decoder = new TextDecoder('utf-8');
  let buffer = '';

  try {
    while (true) {
      if (signal?.aborted) return;
      const { value, done } = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, { stream: true });

      let sep: number;
      while ((sep = indexOfBlankLine(buffer)) !== -1) {
        const frame = buffer.slice(0, sep);
        buffer = buffer.slice(sep + blankLineLength(buffer, sep));
        const event = decodeFrame(frame);
        if (event) yield event;
      }
    }

    // Drain any trailing frame without a terminal double-newline.
    const tail = buffer.trim();
    if (tail) {
      const event = decodeFrame(tail);
      if (event) yield event;
    }
  } finally {
    try {
      reader.releaseLock();
    } catch {
      /* already released or stream errored */
    }
  }
}

function indexOfBlankLine(s: string): number {
  const a = s.indexOf('\n\n');
  const b = s.indexOf('\r\n\r\n');
  if (a === -1) return b;
  if (b === -1) return a;
  return Math.min(a, b);
}

function blankLineLength(s: string, idx: number): number {
  return s.startsWith('\r\n\r\n', idx) ? 4 : 2;
}

function decodeFrame(frame: string): BacktestStreamEvent | null {
  const dataLines: string[] = [];
  for (const rawLine of frame.split(/\r?\n/)) {
    if (!rawLine || rawLine.startsWith(':')) continue;
    const colon = rawLine.indexOf(':');
    if (colon === -1) continue;
    if (rawLine.slice(0, colon) !== 'data') continue;
    dataLines.push(rawLine.slice(colon + 1).replace(/^ /, ''));
  }
  if (dataLines.length === 0) return null;
  try {
    return JSON.parse(dataLines.join('\n')) as BacktestStreamEvent;
  } catch {
    return null;
  }
}
