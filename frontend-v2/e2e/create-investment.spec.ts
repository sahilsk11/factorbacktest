import { expect, test } from './fixtures';

test.describe('Create investment flow', () => {
  test.use({ seedName: 'authenticated_empty' });

  test('shows create CTA on empty investments page', async ({ page, backend }) => {
    await page.goto('/investments');
    await expect(page.getByRole('heading', { name: 'My Investments' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Create new investment' }).first()).toBeVisible();
    await expect(page.getByText('No investments yet')).toBeVisible();
    expect(backend.apiUrl).toMatch(/^http:\/\/localhost:\d+$/);
  });

  test('validates amount before submit', async ({ page, backend: _backend }) => {
    await page.goto('/investments');
    await page.getByRole('button', { name: 'Create new investment' }).first().click();
    await expect(page.getByRole('dialog')).toBeVisible();

    await page.getByLabel(/investment amount/i).fill('0');
    await page.getByRole('button', { name: 'Create investment' }).click();
    await expect(page.getByText(/greater than/i)).toBeVisible();
  });

  test('creates an investment from the dialog', async ({ page, backend: _backend }) => {
    await page.goto('/investments');
    await page.getByRole('button', { name: 'Create new investment' }).first().click();

    await page.getByLabel(/strategy name/i).fill('exploding rockets');
    await page.getByLabel(/investment amount/i).fill('2000');
    await page.getByRole('button', { name: 'SPY Top 300' }).click();
    await page.getByRole('button', { name: 'Daily' }).click();
    await page.getByRole('button', { name: 'Create investment' }).click();

    await expect(page.getByText('exploding rockets')).toBeVisible({ timeout: 15_000 });
    await expect(page.getByText('No investments yet')).toHaveCount(0);
  });
});
