import { apiBaseUrl } from './env';

// The *only* file allowed to touch global fetch. Everything else goes
// through apiClient — enforced by ESLint's no-restricted-syntax rule.
//
// Why a class with three methods instead of a thin wrapper: keeps the
// JSON parsing, error surfacing, and credentials behavior in one
// place; lets us swap transports later (SSE, websockets) without
// rewriting every call site.

export interface ApiError extends Error {
  status: number;
  body: unknown;
}

class ApiErrorImpl extends Error implements ApiError {
  status: number;
  body: unknown;

  constructor(message: string, status: number, body: unknown) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.body = body;
  }
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const url = `${apiBaseUrl}${path}`;
  const init: RequestInit = {
    method,
    // Cookie session is canonical (see internal/auth). Bearer fallback
    // is gone in v2.
    credentials: 'include',
    headers: {
      Accept: 'application/json',
      ...(body !== undefined ? { 'Content-Type': 'application/json' } : {}),
    },
    ...(body !== undefined ? { body: JSON.stringify(body) } : {}),
  };

  const res = await fetch(url, init);

  // 204 No Content is valid for some auth endpoints. Return undefined
  // typed as T — the caller's type contract owns whether that's safe.
  if (res.status === 204) {
    return undefined as T;
  }

  let parsed: unknown = null;
  const text = await res.text();
  if (text) {
    try {
      parsed = JSON.parse(text);
    } catch {
      // Non-JSON body — keep the raw text on the error so we don't
      // silently lose context.
      parsed = text;
    }
  }

  if (!res.ok) {
    const message =
      parsed && typeof parsed === 'object' && 'error' in parsed && typeof parsed.error === 'string'
        ? parsed.error
        : `${res.status} ${res.statusText}`;
    throw new ApiErrorImpl(message, res.status, parsed);
  }

  return parsed as T;
}

export const apiClient = {
  get: <T>(path: string) => request<T>('GET', path),
  post: <T>(path: string, body?: unknown) => request<T>('POST', path, body),
  del: <T>(path: string) => request<T>('DELETE', path),
};
