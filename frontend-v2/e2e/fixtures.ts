import { test as base, expect } from '@playwright/test';
import { spawn, type ChildProcess } from 'child_process';
import fs from 'fs';
import http from 'http';
import path from 'path';
import { fileURLToPath } from 'url';

export type SeedName = '' | 'authenticated_empty' | 'active_investment';
export interface BackendFixture {
  apiUrl: string;
  port: number;
  logPath: string;
}

async function waitForHttp(url: string, timeoutMs = 30_000): Promise<void> {
  const deadline = Date.now() + timeoutMs;
  let lastErr: unknown;
  while (Date.now() < deadline) {
    try {
      await new Promise<void>((resolve, reject) => {
        const req = http.get(url, (res) => {
          res.resume();
          resolve();
        });
        req.on('error', reject);
        req.setTimeout(2_000, () => req.destroy(new Error('timeout')));
      });
      return;
    } catch (err) {
      lastErr = err;
      await new Promise((r) => setTimeout(r, 250));
    }
  }
  throw new Error(`waitForHttp(${url}) timed out: ${String(lastErr)}`);
}

export const test = base.extend<{ seedName: SeedName; backend: BackendFixture }>({
  seedName: ['authenticated_empty', { option: true }],

  backend: async ({ seedName }, use) => {
    const port = Number(process.env.FB_TEST_BE_PORT);
    const fePort = Number(process.env.FB_TEST_FE_PORT);
    const binPath = process.env.FB_TEST_API_BIN ?? '/tmp/fb-test-api';

    if (!port || !fePort) {
      throw new Error('FB_TEST_BE_PORT and FB_TEST_FE_PORT must be set');
    }

    const logPath = `/tmp/fb-test-be-${port}.log`;
    const logFd = fs.openSync(logPath, 'w');

    const args = seedName ? ['-seed', seedName] : [];
    let child: ChildProcess | null = null;
    try {
      child = spawn(binPath, args, {
        cwd: path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..'),
        env: {
          ...process.env,
          PORT: String(port),
          ALPHA_ENV: 'test',
          EXTRA_ALLOWED_ORIGINS: `http://localhost:${fePort}`,
        },
        stdio: ['ignore', logFd, logFd],
      });

      await waitForHttp(`http://localhost:${port}/`);

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
});

export { expect };
