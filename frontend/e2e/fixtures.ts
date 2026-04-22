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

export const test = base.extend<{ diagnostics: Diagnostics }>({
  diagnostics: [
    async ({ page }, use, testInfo) => {
      const diag: Diagnostics = {
        console: [],
        pageErrors: [],
        failedResponses: [],
        allRequests: [],
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

      // On failure, attach everything we've collected.
      if (diag.failedResponses.length) {
        await testInfo.attach('failed-responses.json', {
          body: JSON.stringify(diag.failedResponses, null, 2),
          contentType: 'application/json',
        });
      }

      if (diag.pageErrors.length) {
        await testInfo.attach('page-errors.log', {
          body: diag.pageErrors.join('\n\n'),
          contentType: 'text/plain',
        });
      }

      if (diag.console.length) {
        await testInfo.attach('console.log', {
          body: diag.console.join('\n'),
          contentType: 'text/plain',
        });
      }

      if (diag.allRequests.length) {
        await testInfo.attach('all-requests.log', {
          body: diag.allRequests.join('\n'),
          contentType: 'text/plain',
        });
      }

      try {
        const url = page.url();
        const title = await page.title().catch(() => '<no title>');
        const bodyText = (
          await page.locator('body').innerText().catch(() => '')
        ).slice(0, BODY_PREVIEW_CHARS);
        await testInfo.attach('page-snapshot.txt', {
          body: `URL:   ${url}\nTITLE: ${title}\n\n--- BODY TEXT ---\n${bodyText}`,
          contentType: 'text/plain',
        });
        const html = await page.content().catch(() => '');
        if (html) {
          await testInfo.attach('page.html', {
            body: html,
            contentType: 'text/html',
          });
        }
      } catch {
        // best-effort; page may already be closed
      }
    },
    { auto: true },
  ],
});

export { expect };
