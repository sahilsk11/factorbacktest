import fs from 'fs';
import http from 'http';
import net from 'net';
import path from 'path';
import { spawn, ChildProcess } from 'child_process';
import type { TestInfo } from '@playwright/test';
import { test as base, expect } from '@playwright/test';

type FailedResponse = {
  url: string;
  status: number;
  method: string;
  body: string;
};

type Diagnostics = {
  console: string[];
  pageErrors: string[];
  failedResponses: FailedResponse[];
  allRequests: string[];
  startTimeMs: number;
};

export type SeedName = '' | 'investment_basic' | 'prices_only';
export type BackendFixture = { apiUrl: string; port: number; logPath: string };
type FixtureOptions = { seedName: SeedName };

const BODY_PREVIEW_CHARS = 2000;

async function writeDiagnostic(
  testInfo: TestInfo,
  name: string,
  contentType: string,
  body: string,
): Promise<void> {
  const outPath = testInfo.outputPath(name);
  fs.mkdirSync(path.dirname(outPath), { recursive: true });
  fs.writeFileSync(outPath, body);
  await testInfo.attach(name, { path: outPath, contentType });
}

function getFreePort(): Promise<number> {
  return new Promise((resolve, reject) => {
    const server = net.createServer();
    server.unref();
    server.on('error', reject);
    server.listen(0, '127.0.0.1', () => {
      const addr = server.address();
      if (addr && typeof addr === 'object') {
        const { port } = addr;
        server.close(() => resolve(port));
      } else {
        server.close();
        reject(new Error('Failed to obtain free port'));
      }
    });
  });
}

async function waitForHttp(
  url: string,
  opts: { timeoutMs: number; intervalMs?: number } = { timeoutMs: 30_000 },
): Promise<void> {
  const interval = opts.intervalMs ?? 250;
  const deadline = Date.now() + opts.timeoutMs;
  let lastErr: unknown;
  while (Date.now() < deadline) {
    try {
      await new Promise<void>((resolve, reject) => {
        const req = http.get(url, (res) => {
          res.resume();
          resolve();
        });
        req.on('error', reject);
        req.setTimeout(Math.min(interval * 2, 2_000), () => {
          req.destroy(new Error('request timeout'));
        });
      });
      return;
    } catch (err) {
      lastErr = err;
      await new Promise((r) => setTimeout(r, interval));
    }
  }
  throw new Error(
    `waitForHttp(${url}) timed out after ${opts.timeoutMs}ms: ${String(lastErr)}`,
  );
}

function interpretFailure(args: {
  testInfo: TestInfo;
  diag: Diagnostics;
  pageUrl: string;
  pageTitle: string;
  bodyTextPreview: string;
  backendLogSlice: string;
}): string {
  const { testInfo, diag, pageUrl, pageTitle, bodyTextPreview, backendLogSlice } = args;

  const findings: string[] = [];
  const root: string[] = [];

  // Highest-signal check: FE crashed before mount -> body text empty, page errors present.
  if (diag.pageErrors.length > 0) {
    const first = diag.pageErrors[0].split('\n')[0];
    root.push(`Frontend threw an uncaught error: \`${first}\``);
    if (!bodyTextPreview.trim()) {
      root[root.length - 1] += ' — page body is empty, React likely never mounted.';
    }
  }

  // Backend 5xx responses = server bug / crashed handler.
  const serverErrors = diag.failedResponses.filter((r) => r.status >= 500);
  if (serverErrors.length > 0) {
    root.push(
      `Backend returned ${serverErrors.length} 5xx response(s): ` +
        serverErrors
          .slice(0, 5)
          .map((r) => `${r.status} ${r.method} ${r.url}`)
          .join('; '),
    );
  }

  // 4xx responses = likely missing data / auth / bad params.
  const clientErrors = diag.failedResponses.filter((r) => r.status >= 400 && r.status < 500);
  if (clientErrors.length > 0) {
    root.push(
      `Backend returned ${clientErrors.length} 4xx response(s): ` +
        clientErrors
          .slice(0, 5)
          .map((r) => `${r.status} ${r.method} ${r.url}`)
          .join('; '),
    );
  }

  // Request failures have several distinct causes; differentiate them.
  const networkFails = diag.failedResponses.filter((r) => r.status === 0);
  if (networkFails.length > 0) {
    // Chromium logs CORS specifically to the console; Playwright's
    // request.failure() just says net::ERR_FAILED, which is ambiguous.
    const corsMsgs = diag.console.filter((c) =>
      /blocked by CORS policy|CORS header|Cross-Origin Request Blocked/i.test(c),
    );
    const refusedCount = networkFails.filter((r) =>
      /ERR_CONNECTION_REFUSED/i.test(r.body),
    ).length;
    const dnsCount = networkFails.filter((r) =>
      /ERR_NAME_NOT_RESOLVED/i.test(r.body),
    ).length;

    if (corsMsgs.length > 0) {
      root.push(
        `Browser blocked ${networkFails.length} request(s) via CORS policy. ` +
          `The backend needs to include the FE origin in its Access-Control-Allow-Origin list. ` +
          `Example console message: \`${corsMsgs[0].slice(0, 250)}\``,
      );
    } else if (refusedCount === networkFails.length) {
      root.push(
        `Backend is unreachable — ${refusedCount} request(s) got ERR_CONNECTION_REFUSED. ` +
          `The BE process likely crashed or exited. Check \`backend.log\`.`,
      );
    } else if (dnsCount > 0) {
      root.push(
        `${dnsCount} request(s) failed DNS resolution (ERR_NAME_NOT_RESOLVED). ` +
          `The FE is pointing at a host that doesn't exist.`,
      );
    } else {
      root.push(
        `${networkFails.length} request(s) failed at the network layer: ` +
          networkFails
            .slice(0, 5)
            .map((r) => `${r.method} ${r.url} (${r.body})`)
            .join('; '),
      );
    }
  }

  // "Lack of errors" signal: FE loaded but made no API calls.
  const apiCalls = diag.allRequests.filter((line) => {
    const url = line.split(' ')[1] ?? '';
    // API calls go to the BE port; everything on the FE port is static assets.
    const fePort = process.env.FB_TEST_FE_PORT;
    return fePort ? !url.includes(`:${fePort}`) && url.startsWith('http://localhost:') : false;
  });
  if (apiCalls.length === 0 && diag.failedResponses.length === 0 && diag.pageErrors.length === 0) {
    findings.push(
      'Frontend loaded without errors but made **zero API calls** to the backend. ' +
        'The failing locator likely targets a page/component that never reached the fetch path.',
    );
  }

  // Backend log signals.
  const beLower = backendLogSlice.toLowerCase();
  if (beLower.includes('panic:')) {
    root.push('Backend log contains a `panic:` — server crashed during this test.');
  } else if (/\blevel":"error"/.test(beLower) || /\berror\b/.test(beLower)) {
    const errLines = backendLogSlice
      .split('\n')
      .filter((l) => /error|panic|fatal/i.test(l))
      .slice(0, 3);
    if (errLines.length > 0) {
      findings.push('Backend log has error-level entries:\n```\n' + errLines.join('\n') + '\n```');
    }
  }

  // Fall-through: generic interpretation.
  if (root.length === 0 && findings.length === 0) {
    root.push(
      'No obvious root cause. Assertion failed but FE mounted, API calls succeeded, and BE logged no errors. ' +
        'Check the selector against `page.html` — the DOM shape may have drifted from the test expectation.',
    );
  }

  return [
    `# Test failure: ${testInfo.title}`,
    '',
    `**File**: \`${testInfo.file}:${testInfo.line}\`  `,
    `**Duration**: ${testInfo.duration}ms  `,
    `**Page URL at end of test**: ${pageUrl}  `,
    `**Page title**: ${pageTitle}`,
    '',
    '## Likely root cause',
    '',
    ...(root.length ? root.map((r) => `- ${r}`) : ['- (none identified)']),
    '',
    ...(findings.length
      ? ['## Additional findings', '', ...findings.map((f) => `- ${f}`), '']
      : []),
    '## Evidence summary',
    '',
    `- Browser console messages: ${diag.console.length}`,
    `- Uncaught JS errors (pageerror): ${diag.pageErrors.length}`,
    `- Failed HTTP responses (>=400 or network): ${diag.failedResponses.length}`,
    `- Total requests observed: ${diag.allRequests.length}`,
    `- API calls (to backend): ${apiCalls.length}`,
    `- Backend log bytes during this test: ${backendLogSlice.length}`,
    '',
    '## Artifacts in this folder',
    '',
    '- `SUMMARY.md` — this file',
    '- `page-errors.log` — stack traces from FE uncaught errors',
    '- `failed-responses.json` — URL, status, and response body for every >=400',
    '- `console.log` — full browser console transcript',
    '- `all-requests.log` — every request the FE issued',
    '- `page-snapshot.txt` — final URL, title, visible body text',
    '- `page.html` — full rendered DOM',
    '- `backend.log` — BE stdout/stderr captured during just this test',
    '- `test-failed-1.png`, `trace.zip`, `video.webm` — Playwright built-ins',
    '',
    '## Page body text at failure (first 2KB)',
    '',
    '```',
    bodyTextPreview || '(empty)',
    '```',
  ].join('\n');
}

export const test = base.extend<
  FixtureOptions & { backend: BackendFixture; diagnostics: Diagnostics }
>({
  seedName: ['', { option: true }],

  backend: async ({ seedName, context }, use) => {
    const builtInPort = Number(process.env.FB_TEST_BE_PORT);
    const fePort = Number(process.env.FB_TEST_FE_PORT);
    const binPath = process.env.FB_TEST_API_BIN ?? '/tmp/fb-test-api';

    const port = await getFreePort();
    const logPath = `/tmp/fb-test-be-${port}.log`;
    const logFd = fs.openSync(logPath, 'w');

    const args = seedName ? ['-seed', seedName] : [];
    let child: ChildProcess | null = null;
    try {
      child = spawn(binPath, args, {
        cwd: path.resolve(__dirname, '../..'),
        env: {
          ...process.env,
          PORT: String(port),
          ALPHA_ENV: 'test',
          EXTRA_ALLOWED_ORIGINS: `http://localhost:${fePort}`,
        },
        stdio: ['ignore', logFd, logFd],
      });

      await waitForHttp(`http://localhost:${port}/`, { timeoutMs: 30_000 });

      await context.route(
        `http://localhost:${builtInPort}/**`,
        async (route) => {
          const rewritten = route.request().url().replace(
            `localhost:${builtInPort}`,
            `localhost:${port}`,
          );
          await route.continue({ url: rewritten });
        },
      );

      await use({ apiUrl: `http://localhost:${port}`, port, logPath });
    } finally {
      if (child) {
        child.kill('SIGTERM');
        await new Promise<void>((resolve) => {
          if (child!.exitCode !== null || child!.signalCode !== null) {
            resolve();
            return;
          }
          child!.once('exit', () => resolve());
        });
      }
      try {
        fs.closeSync(logFd);
      } catch {
        // fd may already be closed
      }
    }
  },

  diagnostics: [
    async ({ page, backend }, use, testInfo) => {
      const diag: Diagnostics = {
        console: [],
        pageErrors: [],
        failedResponses: [],
        allRequests: [],
        startTimeMs: Date.now(),
      };

      page.on('console', (msg) => {
        diag.console.push(`[${msg.type()}] ${msg.text()}`);
      });

      page.on('pageerror', (err) => {
        diag.pageErrors.push(`${err.name}: ${err.message}\n${err.stack ?? ''}`);
      });

      page.on('request', (req) => {
        diag.allRequests.push(`${req.method()} ${req.url()}`);
      });

      page.on('response', async (res) => {
        if (res.status() >= 400) {
          let body = '';
          try {
            body = (await res.text()).slice(0, BODY_PREVIEW_CHARS);
          } catch {
            body = '<unable to read body>';
          }
          diag.failedResponses.push({
            url: res.url(),
            status: res.status(),
            method: res.request().method(),
            body,
          });
        }
      });

      page.on('requestfailed', (req) => {
        diag.failedResponses.push({
          url: req.url(),
          status: 0,
          method: req.method(),
          body: `request failed: ${req.failure()?.errorText ?? 'unknown'}`,
        });
      });

      await use(diag);

      if (testInfo.status === testInfo.expectedStatus) {
        try {
          fs.unlinkSync(backend.logPath);
        } catch {
          // best-effort cleanup
        }
        return;
      }

      // Collect page state before any attachments.
      let pageUrl = '';
      let pageTitle = '<no title>';
      let bodyTextPreview = '';
      let html = '';
      try {
        pageUrl = page.url();
        pageTitle = await page.title().catch(() => '<no title>');
        bodyTextPreview = (
          await page.locator('body').innerText().catch(() => '')
        ).slice(0, BODY_PREVIEW_CHARS);
        html = await page.content().catch(() => '');
      } catch {
        // best-effort; page may already be closed
      }

      let backendLogSlice = '';
      try {
        backendLogSlice = fs.readFileSync(backend.logPath, 'utf8');
      } catch {
        backendLogSlice = '';
      }

      // SUMMARY.md first so it's the top-of-attachment-list entry.
      await writeDiagnostic(
        testInfo,
        'SUMMARY.md',
        'text/markdown',
        interpretFailure({
          testInfo,
          diag,
          pageUrl,
          pageTitle,
          bodyTextPreview,
          backendLogSlice,
        }),
      );

      if (backendLogSlice) {
        await writeDiagnostic(testInfo, 'backend.log', 'text/plain', backendLogSlice);
      }

      if (diag.failedResponses.length) {
        await writeDiagnostic(
          testInfo,
          'failed-responses.json',
          'application/json',
          JSON.stringify(diag.failedResponses, null, 2),
        );
      }

      if (diag.pageErrors.length) {
        await writeDiagnostic(
          testInfo,
          'page-errors.log',
          'text/plain',
          diag.pageErrors.join('\n\n'),
        );
      }

      if (diag.console.length) {
        await writeDiagnostic(
          testInfo,
          'console.log',
          'text/plain',
          diag.console.join('\n'),
        );
      }

      if (diag.allRequests.length) {
        await writeDiagnostic(
          testInfo,
          'all-requests.log',
          'text/plain',
          diag.allRequests.join('\n'),
        );
      }

      await writeDiagnostic(
        testInfo,
        'page-snapshot.txt',
        'text/plain',
        `URL:   ${pageUrl}\nTITLE: ${pageTitle}\n\n--- BODY TEXT ---\n${bodyTextPreview}`,
      );
      if (html) {
        await writeDiagnostic(testInfo, 'page.html', 'text/html', html);
      }
    },
    { auto: true },
  ],
});

export { expect };
