import net from 'net';
import { defineConfig, devices } from '@playwright/test';

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

async function resolvePorts(): Promise<{ bePort: number; fePort: number }> {
  // Picked once in the parent process; workers inherit env and reuse these
  // instead of re-picking and drifting out of sync with the built FE.
  if (!process.env.FB_TEST_BE_PORT || !process.env.FB_TEST_FE_PORT) {
    const [be, fe] = await Promise.all([getFreePort(), getFreePort()]);
    process.env.FB_TEST_BE_PORT = String(be);
    process.env.FB_TEST_FE_PORT = String(fe);
  }
  return {
    bePort: Number(process.env.FB_TEST_BE_PORT),
    fePort: Number(process.env.FB_TEST_FE_PORT),
  };
}

export default (async () => {
  const { bePort, fePort } = await resolvePorts();

  // Shared BE log file - diagnostics fixture reads this on failure.
  // Uniquified by port so concurrent runs don't clobber.
  const beLogPath = `/tmp/fb-test-be-${bePort}.log`;
  process.env.FB_TEST_BE_LOG = beLogPath;

  return defineConfig({
    testDir: './e2e',
    timeout: 180_000,
    expect: { timeout: 30_000 },
    fullyParallel: true,
    retries: 0,
    reporter: 'html',
    use: {
      baseURL: `http://localhost:${fePort}`,
      trace: 'retain-on-failure',
      screenshot: 'only-on-failure',
      video: 'retain-on-failure',
    },
    projects: [
      {
        name: 'chromium',
        use: { ...devices['Desktop Chrome'] },
      },
    ],
    webServer: [
      {
        // tee to a file so the diagnostics fixture can attach the BE log to
        // failing tests. Stderr is merged into stdout so logs stay in order.
        command: `sh -c 'go run ./cmd/test-api 2>&1 | tee ${beLogPath}'`,
        cwd: '..',
        env: {
          ALPHA_ENV: 'test',
          PORT: String(bePort),
          // FE is served on a random port; tell the BE to allow that origin
          // so browser-originated calls from the page aren't blocked by CORS.
          EXTRA_ALLOWED_ORIGINS: `http://localhost:${fePort}`,
        },
        url: `http://localhost:${bePort}/`,
        timeout: 120_000,
        reuseExistingServer: false,
      },
      {
        command: `npm run build && npx serve -s build -l ${fePort}`,
        env: {
          REACT_APP_API_PORT: String(bePort),
          // Stubs so client libraries that require non-empty values at boot
          // (e.g. supabase createClient) don't crash the app before mount.
          REACT_APP_SUPABASE_URL: 'http://localhost.test',
          REACT_APP_SUPABASE_ANON_KEY: 'test-anon-key',
          CI: 'false',
          TSC_COMPILE_ON_ERROR: 'true',
          ESLINT_NO_DEV_ERRORS: 'true',
        },
        url: `http://localhost:${fePort}`,
        timeout: 120_000,
        reuseExistingServer: false,
      },
    ],
  });
})();
