import { mkdirSync } from 'fs';
import path from 'path';

import { expect, test } from './fixtures';

const screenshotDir = path.resolve(
  path.dirname(new URL(import.meta.url).pathname),
  '../../docs/screenshots',
);

test.describe('Create investment screenshots', () => {
  test.use({ seedName: 'authenticated_empty' });

  test('capture PR screenshots', async ({ page, backend: _backend }) => {
    mkdirSync(screenshotDir, { recursive: true });

    await page.goto('/investments');
    await expect(page.getByRole('heading', { name: 'My Investments' })).toBeVisible();
    await page.screenshot({
      path: path.join(screenshotDir, 'create-investment-empty-state.png'),
      fullPage: true,
    });

    await page.getByRole('button', { name: 'Create new investment' }).first().click();
    await expect(page.getByRole('dialog')).toBeVisible();

    await page.getByLabel(/strategy name/i).fill('exploding rockets');
    await page.getByLabel(/investment amount/i).fill('2000');
    await page.getByRole('button', { name: 'SPY Top 300' }).click();
    await page.getByRole('button', { name: 'Daily' }).click();
    await page.locator('input[type="number"]').last().fill('3');

    await page.screenshot({
      path: path.join(screenshotDir, 'create-investment-form-filled.png'),
      fullPage: true,
    });

    await page.getByRole('button', { name: 'Create investment' }).click();
    await expect(page.getByText('exploding rockets')).toBeVisible({ timeout: 15_000 });

    await page.screenshot({
      path: path.join(screenshotDir, 'create-investment-after-create.png'),
      fullPage: true,
    });
  });
});
