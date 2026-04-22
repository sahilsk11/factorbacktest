import { test, expect } from './fixtures';

test.use({ seedName: 'investment_basic' });

test('backtest chart renders with seeded price data', async ({ page }) => {
  await page.goto('/backtest');
  await expect(page.locator('#backtest-chart')).toBeVisible({ timeout: 15_000 });
  await expect(page.locator('#backtest-chart canvas')).toBeVisible({ timeout: 15_000 });
});
