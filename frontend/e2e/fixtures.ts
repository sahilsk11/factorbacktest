import fs from 'fs';
import path from 'path';
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
  beLogOffset: number;
};

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

function readBackendLogSlice(startOffset: number): string {
  const logPath = process.env.FB_TEST_BE_LOG;
  if (!logPath || !fs.existsSync(logPath)) return '';
  try {
    const fd = fs.openSync(logPath, 'r');
    const stat = fs.fstatSync(fd);
    const len = Math.max(0, stat.size - startOffset);
    if (len === 0) {
      fs.closeSync(fd);
      return '';
    }
    const buf = Buffer.alloc(len);
    fs.readSync(fd, buf, 0, len, startOffset);
    fs.closeSync(fd);
    return buf.toString('utf8');
  } catch {
    return '';
  }
}

function currentBackendLogSize(): number {
  const logPath = process.env.FB_TEST_BE_LOG;
  if (!logPath || !fs.existsSync(logPath)) return 0;
  try {
    return fs.statSync(logPath).size;
  } catch {
    return 0;
  }
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

export const test = base.extend<{ diagnostics: Diagnostics }>({
  diagnostics: [
    async ({ page }, use, testInfo) => {
      const diag: Diagnostics = {
        console: [],
        pageErrors: [],
        failedResponses: [],
        allRequests: [],
        startTimeMs: Date.now(),
        beLogOffset: currentBackendLogSize(),
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

      if (testInfo.status === testInfo.expectedStatus) return;

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

      const backendLogSlice = readBackendLogSlice(diag.beLogOffset);

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
