import { defineConfig, devices } from '@playwright/test';

const isCI = process.env.CI === 'true';

export default defineConfig({
  testDir: './e2e',
  timeout: 180000,
  expect: {
    timeout: 30000,
  },
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: 'html',
  use: {
    baseURL: 'http://localhost:3000',
    trace: 'on-first-retry',
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
  webServer: isCI ? undefined : [
    {
      command: 'ALPHA_ENV=test go run ../cmd/api/main.go',
      port: 3009,
      timeout: 120000,
      reuseExistingServer: true,
    },
    {
      command: 'npm start',
      port: 3000,
      timeout: 120000,
      reuseExistingServer: !isCI,
    },
  ],
});
