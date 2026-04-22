import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './e2e',
  timeout: 180000,
  expect: {
    timeout: 30000,
  },
  fullyParallel: true,
  retries: 0,
  workers: undefined,
  reporter: 'html',
  use: {
    baseURL: 'http://localhost:3000',
    trace: 'on-first-retry',
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
      command: 'ALPHA_ENV=test go run ../cmd/test-api/main.go',
      port: 0,
      timeout: 120000,
      reuseExistingServer: false,
    },
    {
      command: 'REACT_APP_API_PORT=$WEBSERVER_PREVIOUS_PORT npm run build && npx serve -s build -l $WEBSERVER_PORT',
      port: 0,
      timeout: 120000,
      reuseExistingServer: false,
    },
  ],
});
