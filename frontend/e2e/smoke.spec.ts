import { test, expect } from './fixtures';

// These are smoke checks that the FE bundle builds, mounts, and renders the
// expected top-level DOM on key routes. They do NOT exercise any FE->BE flow;
// real integration tests (with seeded data) are a follow-up.
test.describe('Frontend smoke', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/');
  });

  test('landing page renders the title and create-strategy button', async ({ page }) => {
    await expect(page.locator('h2')).toContainText('Factor Backtest');
    await expect(page.locator('button', { hasText: 'Create Strategy' })).toBeVisible();
  });

  test('/backtest route renders the chart container shell', async ({ page }) => {
    await page.goto('/backtest');
    await expect(page.locator('#backtest-chart')).toBeVisible({ timeout: 10_000 });
    await expect(page.locator('canvas')).toBeVisible({ timeout: 10_000 });
  });
});
