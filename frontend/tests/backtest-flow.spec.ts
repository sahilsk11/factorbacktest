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

  // 4. Wait for the chart canvas to be visible (up to 2 minutes for backtest to run)
  const canvas = page.locator('#backtest-chart canvas');
  await expect(canvas).toBeVisible({ timeout: 120000 });

  // 5. Verify stats table has real values (not "n/a")
  const returnRow = page.getByRole('row', { name: /Annualized Return/i });
  const returnCell = returnRow.locator('td').last();
  await expect(returnCell).not.toHaveText('n/a');
  await expect(returnCell).toMatchText(/^\d+\.?\d*%?$/);

  const sharpeRow = page.getByRole('row', { name: /Sharpe Ratio/i });
  const sharpeCell = sharpeRow.locator('td').last();
  await expect(sharpeCell).not.toHaveText('n/a');
  await expect(sharpeCell).toMatchText(/^\d+\.?\d*%?$/);
});
