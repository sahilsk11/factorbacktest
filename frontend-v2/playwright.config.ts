import { defineConfig, devices } from '@playwright/test';
import net from 'net';

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
    reporter: [['list'], ['html', { open: 'never' }]],
    globalSetup: './e2e/global-setup.ts',
    use: {
      baseURL: `http://localhost:${fePort}`,
      trace: 'retain-on-failure',
      screenshot: 'on',
      video: 'retain-on-failure',
    },
    projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
    webServer: {
      command: `npm run build && npm run preview`,
      env: {
        PORT: String(fePort),
        VITE_API_BASE_URL: `http://localhost:${bePort}`,
      },
      url: `http://localhost:${fePort}`,
      timeout: 120_000,
      reuseExistingServer: false,
    },
  });
})();
