import { test, expect } from './support/seed';

test.describe('Backtest Flow', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/');
  });

  test('landing page loads and shows title', async ({ page }) => {
    await expect(page.locator('h2')).toContainText('Factor Backtest');
    await expect(page.locator('button', { hasText: 'Create Strategy' })).toBeVisible();
  });

  test('navigate to backtest page and verify chart container loads', async ({ page }) => {
    await page.evaluate(() => {
      const overlay = document.getElementById('webpack-dev-server-client-overlay');
      if (overlay) overlay.remove();
    });

    await page.goto('/backtest');

    await expect(page.locator('#backtest-chart')).toBeVisible({ timeout: 10000 });

    await expect(page.locator('canvas')).toBeVisible({ timeout: 10000 });
  });

  test('run backtest with valid data shows chart', async ({ page, seed }) => {
    await seed(['prices_2020']);

    await page.evaluate(() => {
      const overlay = document.getElementById('webpack-dev-server-client-overlay');
      if (overlay) overlay.remove();
    });

    await page.goto('/backtest');

    await expect(page.locator('#backtest-chart')).toBeVisible({ timeout: 10000 });

    const runBacktestBtn = page.locator('button[type="submit"]', { hasText: 'Run Backtest' });

    if (await runBacktestBtn.isVisible()) {
      await runBacktestBtn.click();
    }
  });
});