import { test, expect } from '@playwright/test';

test('home → click strategy → backtest completes with chart and stats', async ({ page }) => {
  // 1. Navigate to home, wait for strategy cards to load
  await page.goto('/');
  await page.waitForSelector('.card', { timeout: 10000 });

  // 2. Click the first strategy card
  const firstCard = page.locator('.card').first();
  await firstCard.click();

  // 3. Confirm we're on the backtest page
  await page.waitForURL(/\/backtest\?id=/, { timeout: 5000 });

  // 4. Wait for loading to finish (up to 2 minutes for backtest to run)
  await page.waitForFunction(
    () => !document.querySelector('img[src*="loading"]'),
    { timeout: 120000 }
  );

  // 5. Verify the chart rendered
  const chart = page.locator('#backtest-chart');
  await expect(chart).toBeVisible();
  const canvas = chart.locator('canvas');
  await expect(canvas).toBeVisible();

  // 6. Verify stats table has real values (not "n/a")
  const returnRow = page.locator('text=Annualized Return').locator('.. td');
  await expect(returnRow).not.toHaveText('n/a');

  const sharpeRow = page.locator('text=Sharpe Ratio').locator('.. td');
  await expect(sharpeRow).not.toHaveText('n/a');
});
