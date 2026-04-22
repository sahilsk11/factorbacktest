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

  return defineConfig({
    testDir: './e2e',
    timeout: 180_000,
    expect: { timeout: 30_000 },
    fullyParallel: false,
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
        command: 'go run ./cmd/test-api',
        cwd: '..',
        env: {
          ALPHA_ENV: 'test',
          PORT: String(bePort),
        },
        url: `http://localhost:${bePort}/`,
        timeout: 120_000,
        reuseExistingServer: false,
      },
      {
        command: `npm run build && npx serve -s build -l ${fePort}`,
        env: {
          REACT_APP_API_PORT: String(bePort),
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
